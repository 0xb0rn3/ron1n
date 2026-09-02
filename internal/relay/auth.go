package relay

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

var hostIDPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,63}$`)

type Credential struct {
	TokenSHA256 string `json:"token_sha256"`
	Disabled    bool   `json:"disabled,omitempty"`
}

type CredentialFile struct {
	Schema int                   `json:"schema"`
	Hosts  map[string]Credential `json:"hosts"`
}

type Authenticator struct {
	hosts map[string]Credential
}

func LoadAuthenticator(name string) (*Authenticator, error) {
	b, err := os.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("read relay credentials: %w", err)
	}
	var file CredentialFile
	if err := json.Unmarshal(b, &file); err != nil {
		return nil, fmt.Errorf("decode relay credentials: %w", err)
	}
	if file.Schema != 1 || len(file.Hosts) == 0 {
		return nil, errors.New("relay credential file is empty or unsupported")
	}
	for hostID, credential := range file.Hosts {
		if !hostIDPattern.MatchString(hostID) {
			return nil, fmt.Errorf("invalid host ID %q", hostID)
		}
		if len(credential.TokenSHA256) != sha256.Size*2 {
			return nil, fmt.Errorf("invalid token hash for %q", hostID)
		}
		if _, err := hex.DecodeString(credential.TokenSHA256); err != nil {
			return nil, fmt.Errorf("invalid token hash for %q", hostID)
		}
	}
	return &Authenticator{hosts: file.Hosts}, nil
}

func (auth *Authenticator) Authenticate(hostID, authorization string) bool {
	credential, ok := auth.hosts[hostID]
	if !ok || credential.Disabled || !strings.HasPrefix(authorization, "Bearer ") {
		return false
	}
	token := strings.TrimSpace(strings.TrimPrefix(authorization, "Bearer "))
	actual := sha256.Sum256([]byte(token))
	expected, err := hex.DecodeString(credential.TokenSHA256)
	if err != nil || len(expected) != len(actual) {
		return false
	}
	return subtle.ConstantTimeCompare(actual[:], expected) == 1
}

func Provision(credentialsPath, hostID string, rotate bool) (string, error) {
	if !hostIDPattern.MatchString(hostID) {
		return "", errors.New("host ID must use 1-64 letters, numbers, dots, dashes, or underscores")
	}
	file := CredentialFile{Schema: 1, Hosts: make(map[string]Credential)}
	if b, err := os.ReadFile(credentialsPath); err == nil {
		if err := json.Unmarshal(b, &file); err != nil {
			return "", fmt.Errorf("decode existing credentials: %w", err)
		}
		if file.Schema != 1 {
			return "", errors.New("unsupported credential file schema")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if file.Hosts == nil {
		file.Hosts = make(map[string]Credential)
	}
	if _, exists := file.Hosts[hostID]; exists && !rotate {
		return "", fmt.Errorf("host %q already exists; use --rotate to replace its token", hostID)
	}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(secret)
	digest := sha256.Sum256([]byte(token))
	file.Hosts[hostID] = Credential{TokenSHA256: hex.EncodeToString(digest[:])}
	b, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return "", err
	}
	if err := atomicCredentialWrite(credentialsPath, append(b, '\n')); err != nil {
		return "", err
	}
	return token, nil
}

func ReadToken(name string) (string, error) {
	info, err := os.Lstat(name)
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return "", errors.New("relay token must be a regular, non-symlink file")
	}
	if info.Size() > 4096 {
		return "", errors.New("relay token file is too large")
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		return "", errors.New("relay token file permissions must not grant group or other access")
	}
	file, err := os.Open(name)
	if err != nil {
		return "", err
	}
	defer file.Close()
	b, err := io.ReadAll(io.LimitReader(file, 4097))
	if err != nil {
		return "", err
	}
	if len(b) > 4096 {
		return "", errors.New("relay token file is too large")
	}
	token := strings.TrimSpace(string(b))
	if len(token) < 32 {
		return "", errors.New("relay token file is invalid")
	}
	return token, nil
}

func WriteToken(name, token string) error {
	return atomicCredentialWrite(name, []byte(strings.TrimSpace(token)+"\n"))
}

func atomicCredentialWrite(name string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(name), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(name), ".relay-secret-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
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
