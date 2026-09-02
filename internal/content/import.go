package content

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	DefaultPSFreeRepo     = "https://github.com/kmeps4/PSFree"
	DefaultPSFreeRevision = "368d82aa40d3017c220757ce315761adb5f06678"
	maxArchiveBytes       = 128 << 20
	maxExtractedBytes     = 256 << 20
	maxArchiveEntries     = 10000
	maxBundleFileBytes    = 64 << 20
)

var githubPart = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

type ImportOptions struct {
	Repository      string
	Revision        string
	DestinationRoot string
	Profile         string
	ArchiveSHA256   string
	SigningKey      ed25519.PrivateKey
	Client          *http.Client
	Now             time.Time
}

type ImportResult struct {
	Directory     string
	ManifestPath  string
	Revision      string
	ArchiveSHA256 string
	Envelope      Envelope
}

func ImportGitHub(ctx context.Context, options ImportOptions) (ImportResult, error) {
	owner, repo, err := ParseGitHubRepository(options.Repository)
	if err != nil {
		return ImportResult{}, err
	}
	if options.Client == nil {
		options.Client = &http.Client{Timeout: 2 * time.Minute}
	}
	if options.Revision == "" {
		options.Revision = DefaultPSFreeRevision
	}
	revision, err := resolveGitHubRevision(ctx, options.Client, owner, repo, options.Revision)
	if err != nil {
		return ImportResult{}, err
	}
	archiveURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/tarball/%s", owner, repo, url.PathEscape(revision))
	return importArchive(ctx, archiveURL, owner+"/"+repo, revision, options)
}

func ParseGitHubRepository(value string) (string, string, error) {
	value = strings.TrimSpace(strings.TrimSuffix(value, ".git"))
	if !strings.Contains(value, "://") {
		parts := strings.Split(strings.Trim(value, "/"), "/")
		if len(parts) == 2 && validGitHubPart(parts[0]) && validGitHubPart(parts[1]) {
			return parts[0], parts[1], nil
		}
		return "", "", errors.New("repository must be owner/name or a github.com URL")
	}
	parsed, err := url.Parse(value)
	if err != nil || !strings.EqualFold(parsed.Hostname(), "github.com") {
		return "", "", errors.New("only github.com repository URLs are accepted")
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) != 2 || !validGitHubPart(parts[0]) || !validGitHubPart(parts[1]) {
		return "", "", errors.New("invalid GitHub repository path")
	}
	return parts[0], parts[1], nil
}

func validGitHubPart(value string) bool {
	return value != "." && value != ".." && githubPart.MatchString(value)
}

func resolveGitHubRevision(ctx context.Context, client *http.Client, owner, repo, revision string) (string, error) {
	endpoint := fmt.Sprintf("https://api.github.com/repos/%s/%s/commits/%s", owner, repo, url.PathEscape(revision))
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "ron1n-content-importer")
	response, err := client.Do(request)
	if err != nil {
		return "", fmt.Errorf("resolve GitHub revision: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("resolve GitHub revision: HTTP %d", response.StatusCode)
	}
	var result struct {
		SHA string `json:"sha"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&result); err != nil {
		return "", fmt.Errorf("decode GitHub revision: %w", err)
	}
	if len(result.SHA) != 40 {
		return "", errors.New("GitHub returned an invalid commit ID")
	}
	return result.SHA, nil
}

func importArchive(ctx context.Context, archiveURL, source, revision string, options ImportOptions) (ImportResult, error) {
	if options.DestinationRoot == "" {
		return ImportResult{}, errors.New("destination root is required")
	}
	if options.Profile == "" {
		options.Profile = "psfree-lapse-900"
	}
	if options.Client == nil {
		options.Client = &http.Client{Timeout: 2 * time.Minute}
	}
	if options.Now.IsZero() {
		options.Now = time.Now()
	}
	if err := os.MkdirAll(options.DestinationRoot, 0o700); err != nil {
		return ImportResult{}, err
	}
	bundles := filepath.Join(options.DestinationRoot, "bundles")
	if err := os.MkdirAll(bundles, 0o700); err != nil {
		return ImportResult{}, err
	}
	finalDir := filepath.Join(bundles, revision)
	if options.ArchiveSHA256 == "" {
		if existing, err := reuseImportedBundle(finalDir, source, revision, options.SigningKey, options.Now); err == nil {
			return existing, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return ImportResult{}, err
		}
	}
	archiveFile, err := os.CreateTemp(options.DestinationRoot, ".archive-*.tar.gz")
	if err != nil {
		return ImportResult{}, err
	}
	archiveName := archiveFile.Name()
	defer os.Remove(archiveName)
	digest, err := download(ctx, options.Client, archiveURL, archiveFile)
	if closeErr := archiveFile.Close(); err == nil && closeErr != nil {
		err = closeErr
	}
	if err != nil {
		return ImportResult{}, err
	}
	if options.ArchiveSHA256 != "" && !strings.EqualFold(options.ArchiveSHA256, digest) {
		return ImportResult{}, errors.New("downloaded archive SHA-256 does not match the requested digest")
	}

	if _, err := os.Stat(finalDir); err == nil {
		return ImportResult{}, fmt.Errorf("bundle revision already exists: %s", revision)
	} else if !errors.Is(err, os.ErrNotExist) {
		return ImportResult{}, err
	}
	staging, err := os.MkdirTemp(bundles, ".staging-*")
	if err != nil {
		return ImportResult{}, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = os.RemoveAll(staging)
		}
	}()
	if err := extractTarGzip(archiveName, staging); err != nil {
		return ImportResult{}, err
	}
	manifest, err := Build(staging, options.Profile, "https://github.com/"+source, revision, options.Now, time.Time{})
	if err != nil {
		return ImportResult{}, err
	}
	envelope := Envelope{Manifest: manifest}
	if len(options.SigningKey) > 0 {
		envelope, err = Sign(manifest, options.SigningKey)
		if err != nil {
			return ImportResult{}, err
		}
	}
	manifestPath := filepath.Join(staging, manifestName)
	if err := Save(manifestPath, envelope); err != nil {
		return ImportResult{}, err
	}
	if err := VerifyFiles(staging, manifest, options.Now); err != nil {
		return ImportResult{}, err
	}
	if err := os.Rename(staging, finalDir); err != nil {
		return ImportResult{}, fmt.Errorf("activate imported bundle: %w", err)
	}
	committed = true
	return ImportResult{
		Directory:     finalDir,
		ManifestPath:  filepath.Join(finalDir, manifestName),
		Revision:      revision,
		ArchiveSHA256: digest,
		Envelope:      envelope,
	}, nil
}

func reuseImportedBundle(directory, source, revision string, signingKey ed25519.PrivateKey, now time.Time) (ImportResult, error) {
	manifestPath := filepath.Join(directory, manifestName)
	envelope, err := Load(manifestPath)
	if err != nil {
		return ImportResult{}, err
	}
	if envelope.Manifest.Source != "https://github.com/"+source || envelope.Manifest.SourceRevision != revision {
		return ImportResult{}, errors.New("existing bundle provenance does not match requested source")
	}
	if err := VerifyFiles(directory, envelope.Manifest, now); err != nil {
		return ImportResult{}, fmt.Errorf("existing bundle verification failed: %w", err)
	}
	if len(signingKey) > 0 {
		public := signingKey.Public().(ed25519.PublicKey)
		if envelope.Signature == "" {
			envelope, err = Sign(envelope.Manifest, signingKey)
			if err != nil {
				return ImportResult{}, err
			}
			if err := Save(manifestPath, envelope); err != nil {
				return ImportResult{}, err
			}
		} else if err := VerifySignature(envelope, public); err != nil {
			return ImportResult{}, errors.New("existing bundle is signed by a different or invalid key")
		}
	}
	return ImportResult{
		Directory:    directory,
		ManifestPath: manifestPath,
		Revision:     revision,
		Envelope:     envelope,
	}, nil
}

func download(ctx context.Context, client *http.Client, archiveURL string, destination *os.File) (string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, archiveURL, nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "ron1n-content-importer")
	response, err := client.Do(request)
	if err != nil {
		return "", fmt.Errorf("download content archive: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download content archive: HTTP %d", response.StatusCode)
	}
	hash := sha256.New()
	limited := &io.LimitedReader{R: response.Body, N: maxArchiveBytes + 1}
	written, err := io.Copy(io.MultiWriter(destination, hash), limited)
	if err != nil {
		return "", err
	}
	if written > maxArchiveBytes {
		return "", errors.New("content archive exceeds 128 MiB limit")
	}
	if err := destination.Sync(); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func extractTarGzip(archiveName, destination string) error {
	file, err := os.Open(archiveName)
	if err != nil {
		return err
	}
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("open gzip archive: %w", err)
	}
	defer gzipReader.Close()
	tarReader := tar.NewReader(gzipReader)
	var rootPrefix string
	var total int64
	entries := 0
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar archive: %w", err)
		}
		if isArchiveMetadata(header) {
			continue
		}
		entries++
		if entries > maxArchiveEntries {
			return errors.New("content archive has too many entries")
		}
		name := strings.ReplaceAll(header.Name, "\\", "/")
		clean := path.Clean(name)
		if clean == "." || path.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, "../") {
			return fmt.Errorf("unsafe archive path %q", header.Name)
		}
		parts := strings.Split(clean, "/")
		if isIgnoredArchiveRoot(parts[0]) {
			continue
		}
		if rootPrefix == "" {
			rootPrefix = parts[0]
		}
		if parts[0] != rootPrefix {
			return errors.New("content archive has multiple roots")
		}
		if len(parts) == 1 {
			continue
		}
		rel := path.Join(parts[1:]...)
		if rel == ".git" || strings.HasPrefix(rel, ".git/") {
			return errors.New("content archive contains git metadata")
		}
		target := filepath.Join(destination, filepath.FromSlash(rel))
		resolved, err := filepath.Rel(destination, target)
		if err != nil || resolved == ".." || strings.HasPrefix(resolved, ".."+string(filepath.Separator)) {
			return fmt.Errorf("archive path escapes destination: %q", header.Name)
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if header.Size < 0 || header.Size > maxBundleFileBytes {
				return fmt.Errorf("archive file exceeds limit: %s", rel)
			}
			total += header.Size
			if total > maxExtractedBytes {
				return errors.New("extracted archive exceeds 256 MiB limit")
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			mode := os.FileMode(0o644)
			if header.Mode&0o111 != 0 {
				mode = 0o755
			}
			output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
			if err != nil {
				return err
			}
			_, copyErr := io.CopyN(output, tarReader, header.Size)
			closeErr := output.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
		default:
			return fmt.Errorf("archive link or special entry is not allowed: %s", rel)
		}
	}
	if rootPrefix == "" || entries == 0 {
		return errors.New("content archive is empty")
	}
	return nil
}

func isArchiveMetadata(header *tar.Header) bool {
	switch header.Typeflag {
	case tar.TypeXHeader, tar.TypeXGlobalHeader, tar.TypeGNULongName, tar.TypeGNULongLink:
		return true
	default:
		return false
	}
}

func isIgnoredArchiveRoot(value string) bool {
	value = strings.ToLower(value)
	return value == "pax_global_header" || value == "@paxheader" || strings.Contains(value, "paxheaders")
}
