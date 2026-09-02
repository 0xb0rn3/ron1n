package content

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/0xb0rn3/ron1n/internal/version"
)

const (
	ManifestSchema = 1
	manifestName   = "ron1n-manifest.json"
)

var RequiredPSFree900 = []string{
	"index.html",
	"cache.html",
	"psfree_lapse.cache",
	"alert.mjs",
	"psfree.mjs",
	"lapse.mjs",
	"aio_patches.bin",
	"goldhen.bin",
}

type File struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
	MIME   string `json:"mime"`
	Stage  string `json:"stage,omitempty"`
}

type Manifest struct {
	Schema          int    `json:"schema"`
	ProductVersion  string `json:"product_version"`
	BundleID        string `json:"bundle_id"`
	Profile         string `json:"profile"`
	Source          string `json:"source"`
	SourceRevision  string `json:"source_revision"`
	CreatedAt       string `json:"created_at"`
	ExpiresAt       string `json:"expires_at,omitempty"`
	MinimumProtocol int    `json:"minimum_protocol"`
	Files           []File `json:"files"`
}

type Envelope struct {
	Manifest  Manifest `json:"manifest"`
	KeyID     string   `json:"key_id,omitempty"`
	Signature string   `json:"signature,omitempty"`
}

type PrivateKeyFile struct {
	Schema     int    `json:"schema"`
	KeyID      string `json:"key_id"`
	PrivateKey string `json:"private_key"`
}

type PublicKeyFile struct {
	Schema    int    `json:"schema"`
	KeyID     string `json:"key_id"`
	PublicKey string `json:"public_key"`
}

func Build(root, profile, source, revision string, now time.Time, expires time.Time) (Manifest, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return Manifest{}, fmt.Errorf("resolve content root: %w", err)
	}
	info, err := os.Stat(root)
	if err != nil {
		return Manifest{}, fmt.Errorf("stat content root: %w", err)
	}
	if !info.IsDir() {
		return Manifest{}, errors.New("content root is not a directory")
	}

	manifest := Manifest{
		Schema:          ManifestSchema,
		ProductVersion:  version.Version,
		Profile:         profile,
		Source:          source,
		SourceRevision:  revision,
		CreatedAt:       now.UTC().Format(time.RFC3339),
		MinimumProtocol: version.Protocol,
	}
	if !expires.IsZero() {
		manifest.ExpiresAt = expires.UTC().Format(time.RFC3339)
	}

	err = filepath.WalkDir(root, func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(root, name)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		if rel == ".git" || strings.HasPrefix(rel, ".git/") {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if path.Base(rel) == manifestName {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink is not allowed in a content bundle: %s", rel)
		}
		if entry.IsDir() {
			return nil
		}
		fileInfo, err := entry.Info()
		if err != nil {
			return err
		}
		if !fileInfo.Mode().IsRegular() {
			return fmt.Errorf("non-regular content is not allowed: %s", rel)
		}
		digest, err := hashFile(name)
		if err != nil {
			return err
		}
		manifest.Files = append(manifest.Files, File{
			Path:   rel,
			Size:   fileInfo.Size(),
			SHA256: digest,
			MIME:   MIME(rel),
			Stage:  Stage(rel),
		})
		return nil
	})
	if err != nil {
		return Manifest{}, fmt.Errorf("build content inventory: %w", err)
	}
	sort.Slice(manifest.Files, func(i, j int) bool { return manifest.Files[i].Path < manifest.Files[j].Path })
	if err := validateRequired(manifest, profile); err != nil {
		return Manifest{}, err
	}
	canonical, err := canonicalManifest(manifest)
	if err != nil {
		return Manifest{}, err
	}
	digest := sha256.Sum256(canonical)
	manifest.BundleID = hex.EncodeToString(digest[:])
	return manifest, nil
}

func validateRequired(manifest Manifest, profile string) error {
	if profile != "psfree-lapse-900" {
		return nil
	}
	available := make(map[string]bool, len(manifest.Files))
	for _, file := range manifest.Files {
		available[file.Path] = true
	}
	var missing []string
	for _, required := range RequiredPSFree900 {
		if !available[required] {
			missing = append(missing, required)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("psfree-lapse-900 bundle is missing: %s", strings.Join(missing, ", "))
	}
	return nil
}

func canonicalManifest(manifest Manifest) ([]byte, error) {
	copyManifest := manifest
	copyManifest.BundleID = ""
	sort.Slice(copyManifest.Files, func(i, j int) bool { return copyManifest.Files[i].Path < copyManifest.Files[j].Path })
	return json.Marshal(copyManifest)
}

func Sign(manifest Manifest, private ed25519.PrivateKey) (Envelope, error) {
	if len(private) != ed25519.PrivateKeySize {
		return Envelope{}, errors.New("invalid Ed25519 private key")
	}
	canonical, err := canonicalManifest(manifest)
	if err != nil {
		return Envelope{}, err
	}
	public := private.Public().(ed25519.PublicKey)
	return Envelope{
		Manifest:  manifest,
		KeyID:     KeyID(public),
		Signature: base64.RawStdEncoding.EncodeToString(ed25519.Sign(private, canonical)),
	}, nil
}

func VerifySignature(envelope Envelope, public ed25519.PublicKey) error {
	if envelope.Signature == "" || envelope.KeyID == "" {
		return errors.New("manifest is unsigned")
	}
	if envelope.KeyID != KeyID(public) {
		return fmt.Errorf("manifest key %q does not match trusted key %q", envelope.KeyID, KeyID(public))
	}
	signature, err := base64.RawStdEncoding.DecodeString(envelope.Signature)
	if err != nil {
		return fmt.Errorf("decode manifest signature: %w", err)
	}
	canonical, err := canonicalManifest(envelope.Manifest)
	if err != nil {
		return err
	}
	if !ed25519.Verify(public, canonical, signature) {
		return errors.New("manifest signature verification failed")
	}
	return nil
}

func VerifyFiles(root string, manifest Manifest, now time.Time) error {
	if err := ValidateManifest(manifest, now); err != nil {
		return err
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	for _, expected := range manifest.Files {
		name, err := Resolve(root, expected.Path)
		if err != nil {
			return err
		}
		info, err := os.Stat(name)
		if err != nil {
			return fmt.Errorf("stat %s: %w", expected.Path, err)
		}
		if !info.Mode().IsRegular() || info.Size() != expected.Size {
			return fmt.Errorf("content size mismatch: %s", expected.Path)
		}
		digest, err := hashFile(name)
		if err != nil {
			return err
		}
		if digest != expected.SHA256 {
			return fmt.Errorf("content hash mismatch: %s", expected.Path)
		}
	}
	return nil
}

func ValidateManifest(manifest Manifest, now time.Time) error {
	if manifest.Schema != ManifestSchema {
		return fmt.Errorf("unsupported manifest schema %d", manifest.Schema)
	}
	if manifest.BundleID == "" || len(manifest.Files) == 0 {
		return errors.New("manifest bundle_id and files are required")
	}
	if manifest.MinimumProtocol > version.Protocol {
		return fmt.Errorf("bundle requires protocol %d", manifest.MinimumProtocol)
	}
	if manifest.ExpiresAt != "" {
		expires, err := time.Parse(time.RFC3339, manifest.ExpiresAt)
		if err != nil {
			return fmt.Errorf("invalid manifest expiry: %w", err)
		}
		if !now.Before(expires) {
			return errors.New("manifest has expired")
		}
	}
	seen := make(map[string]bool, len(manifest.Files))
	last := ""
	for _, file := range manifest.Files {
		normalized, err := Normalize(file.Path)
		if err != nil || normalized != file.Path {
			return fmt.Errorf("invalid manifest path %q", file.Path)
		}
		if seen[file.Path] {
			return fmt.Errorf("duplicate manifest path %q", file.Path)
		}
		seen[file.Path] = true
		if file.Path < last {
			return errors.New("manifest files are not sorted")
		}
		last = file.Path
		if file.Size < 0 || len(file.SHA256) != sha256.Size*2 {
			return fmt.Errorf("invalid metadata for %q", file.Path)
		}
		if _, err := hex.DecodeString(file.SHA256); err != nil {
			return fmt.Errorf("invalid digest for %q", file.Path)
		}
	}
	canonical, err := canonicalManifest(manifest)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(canonical)
	if manifest.BundleID != hex.EncodeToString(digest[:]) {
		return errors.New("bundle_id does not match manifest content")
	}
	return validateRequired(manifest, manifest.Profile)
}

func (manifest Manifest) Lookup(name string) (File, bool) {
	i := sort.Search(len(manifest.Files), func(i int) bool { return manifest.Files[i].Path >= name })
	if i < len(manifest.Files) && manifest.Files[i].Path == name {
		return manifest.Files[i], true
	}
	return File{}, false
}

func Normalize(requestPath string) (string, error) {
	requestPath = strings.ReplaceAll(requestPath, "\\", "/")
	requestPath = strings.TrimPrefix(requestPath, "/")
	if strings.ContainsRune(requestPath, 0) {
		return "", errors.New("NUL is not allowed in a path")
	}
	for _, part := range strings.Split(requestPath, "/") {
		if part == ".." {
			return "", errors.New("parent path segments are not allowed")
		}
	}
	clean := path.Clean(requestPath)
	if clean == "." || clean == "" {
		return "index.html", nil
	}
	if clean == ".." || strings.HasPrefix(clean, "../") || path.IsAbs(clean) {
		return "", errors.New("path escapes the bundle")
	}
	for _, part := range strings.Split(clean, "/") {
		if part == ".git" {
			return "", errors.New("git metadata is not content")
		}
	}
	return clean, nil
}

func Resolve(root, name string) (string, error) {
	normalized, err := Normalize(name)
	if err != nil {
		return "", err
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return "", err
	}
	resolved := filepath.Join(root, filepath.FromSlash(normalized))
	rel, err := filepath.Rel(root, resolved)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("path escapes the bundle")
	}
	current := root
	for _, part := range strings.Split(normalized, "/") {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("symlink is not allowed: %s", normalized)
		}
	}
	return resolved, nil
}

func MIME(name string) string {
	switch strings.ToLower(path.Ext(name)) {
	case ".cache":
		return "text/cache-manifest; charset=utf-8"
	case ".mjs", ".js":
		return "text/javascript; charset=utf-8"
	case ".bin", ".elf", ".o":
		return "application/octet-stream"
	case ".wasm":
		return "application/wasm"
	}
	if value := mime.TypeByExtension(path.Ext(name)); value != "" {
		return value
	}
	return "application/octet-stream"
}

func Stage(name string) string {
	name, _ = Normalize(name)
	switch name {
	case "cache.html", "psfree_lapse.cache":
		return "offline-cache"
	case "psfree.mjs", "alert.mjs":
		return "webkit-exploit-activity"
	case "lapse.mjs", "aio_patches.bin", "kpatch/900.elf":
		return "kernel-exploit-activity"
	case "goldhen.bin":
		return "goldhen-payload-served"
	default:
		return "browser"
	}
}

func Save(path string, envelope Envelope) error {
	b, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return atomicWrite(path, b, 0o644)
}

func Load(path string) (Envelope, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Envelope{}, err
	}
	var envelope Envelope
	if err := json.Unmarshal(b, &envelope); err != nil {
		return Envelope{}, fmt.Errorf("decode manifest: %w", err)
	}
	return envelope, nil
}

func GenerateKeyPair(privatePath, publicPath string) (string, error) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", err
	}
	id := KeyID(public)
	privateFile := PrivateKeyFile{Schema: 1, KeyID: id, PrivateKey: base64.RawStdEncoding.EncodeToString(private)}
	publicFile := PublicKeyFile{Schema: 1, KeyID: id, PublicKey: base64.RawStdEncoding.EncodeToString(public)}
	privateJSON, _ := json.MarshalIndent(privateFile, "", "  ")
	publicJSON, _ := json.MarshalIndent(publicFile, "", "  ")
	if err := atomicWrite(privatePath, append(privateJSON, '\n'), 0o600); err != nil {
		return "", err
	}
	if err := atomicWrite(publicPath, append(publicJSON, '\n'), 0o644); err != nil {
		return "", err
	}
	return id, nil
}

func LoadPrivateKey(name string) (ed25519.PrivateKey, string, error) {
	b, err := os.ReadFile(name)
	if err != nil {
		return nil, "", err
	}
	var keyFile PrivateKeyFile
	if err := json.Unmarshal(b, &keyFile); err != nil {
		return nil, "", err
	}
	key, err := base64.RawStdEncoding.DecodeString(keyFile.PrivateKey)
	if err != nil || len(key) != ed25519.PrivateKeySize {
		return nil, "", errors.New("invalid private key file")
	}
	private := ed25519.PrivateKey(key)
	if KeyID(private.Public().(ed25519.PublicKey)) != keyFile.KeyID {
		return nil, "", errors.New("private key ID mismatch")
	}
	return private, keyFile.KeyID, nil
}

func LoadPublicKey(name string) (ed25519.PublicKey, string, error) {
	b, err := os.ReadFile(name)
	if err != nil {
		return nil, "", err
	}
	var keyFile PublicKeyFile
	if err := json.Unmarshal(b, &keyFile); err != nil {
		return nil, "", err
	}
	key, err := base64.RawStdEncoding.DecodeString(keyFile.PublicKey)
	if err != nil || len(key) != ed25519.PublicKeySize {
		return nil, "", errors.New("invalid public key file")
	}
	public := ed25519.PublicKey(key)
	if KeyID(public) != keyFile.KeyID {
		return nil, "", errors.New("public key ID mismatch")
	}
	return public, keyFile.KeyID, nil
}

func KeyID(public ed25519.PublicKey) string {
	digest := sha256.Sum256(public)
	return hex.EncodeToString(digest[:8])
}

func hashFile(name string) (string, error) {
	file, err := os.Open(name)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func atomicWrite(name string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(name), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(name), ".ron1n-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, name)
}
