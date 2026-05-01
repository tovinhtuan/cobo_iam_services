package http

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type cmsMediaSigner struct {
	secret []byte
	ttl    time.Duration
}

func newCMSMediaSigner(secret string, ttl time.Duration) *cmsMediaSigner {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		secret = "dev-cms-media-secret"
	}
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	return &cmsMediaSigner{
		secret: []byte(secret),
		ttl:    ttl,
	}
}

func (s *cmsMediaSigner) sign(assetID, companyID, method, contentType string, sizeBytes int64, expUnix int64) string {
	mac := hmac.New(sha256.New, s.secret)
	io.WriteString(mac, signingPayload(assetID, companyID, method, contentType, sizeBytes, expUnix))
	return hex.EncodeToString(mac.Sum(nil))
}

func (s *cmsMediaSigner) verify(sig, assetID, companyID, method, contentType string, sizeBytes int64, expUnix int64) bool {
	expected := s.sign(assetID, companyID, method, contentType, sizeBytes, expUnix)
	return hmac.Equal([]byte(strings.ToLower(strings.TrimSpace(sig))), []byte(expected))
}

func signingPayload(assetID, companyID, method, contentType string, sizeBytes int64, expUnix int64) string {
	return fmt.Sprintf("%s|%s|%s|%s|%d|%d", strings.TrimSpace(assetID), strings.TrimSpace(companyID), strings.ToUpper(strings.TrimSpace(method)), strings.ToLower(strings.TrimSpace(contentType)), sizeBytes, expUnix)
}

type cmsMediaDiskStorage struct {
	rootDir string
}

func newCMSMediaDiskStorage(rootDir string) (*cmsMediaDiskStorage, error) {
	root := strings.TrimSpace(rootDir)
	if root == "" {
		root = "./var/cms-media"
	}
	cleanRoot := filepath.Clean(root)
	if err := os.MkdirAll(cleanRoot, 0o755); err != nil {
		return nil, fmt.Errorf("create media storage dir: %w", err)
	}
	return &cmsMediaDiskStorage{rootDir: cleanRoot}, nil
}

func (s *cmsMediaDiskStorage) Write(objectKey string, body io.Reader) (int64, error) {
	target, err := s.resolvePath(objectKey)
	if err != nil {
		return 0, err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return 0, fmt.Errorf("create media dir: %w", err)
	}
	file, err := os.Create(target)
	if err != nil {
		return 0, fmt.Errorf("create media file: %w", err)
	}
	defer file.Close()
	written, err := io.Copy(file, body)
	if err != nil {
		return 0, fmt.Errorf("write media file: %w", err)
	}
	return written, nil
}

func (s *cmsMediaDiskStorage) Exists(objectKey string) bool {
	target, err := s.resolvePath(objectKey)
	if err != nil {
		return false
	}
	info, err := os.Stat(target)
	return err == nil && !info.IsDir()
}

func (s *cmsMediaDiskStorage) Delete(objectKey string) error {
	target, err := s.resolvePath(objectKey)
	if err != nil {
		return err
	}
	err = os.Remove(target)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete media file: %w", err)
	}
	return nil
}

func (s *cmsMediaDiskStorage) resolvePath(objectKey string) (string, error) {
	cleanKey := filepath.Clean(strings.TrimSpace(objectKey))
	if cleanKey == "" || cleanKey == "." {
		return "", fmt.Errorf("invalid media object_key")
	}
	fullPath := filepath.Clean(filepath.Join(s.rootDir, cleanKey))
	rel, err := filepath.Rel(s.rootDir, fullPath)
	if err != nil {
		return "", fmt.Errorf("resolve media object_key: %w", err)
	}
	if strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("invalid media object_key path traversal")
	}
	return fullPath, nil
}
