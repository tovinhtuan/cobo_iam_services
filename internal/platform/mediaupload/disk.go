package mediaupload

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// DiskStorage stores objects under a root directory using POSIX-style object keys.
type DiskStorage struct {
	rootDir string
}

// NewDiskStorage creates the root directory if missing.
func NewDiskStorage(rootDir string) (*DiskStorage, error) {
	root := strings.TrimSpace(rootDir)
	if root == "" {
		root = "./var/media"
	}
	cleanRoot := filepath.Clean(root)
	if err := os.MkdirAll(cleanRoot, 0o755); err != nil {
		return nil, fmt.Errorf("create media storage dir: %w", err)
	}
	return &DiskStorage{rootDir: cleanRoot}, nil
}

// Root returns the resolved storage root.
func (s *DiskStorage) Root() string {
	return s.rootDir
}

// Write stores an object, creating parent directories as needed.
func (s *DiskStorage) Write(objectKey string, body io.Reader) (int64, error) {
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

// Read returns the full object bytes.
func (s *DiskStorage) Read(objectKey string) ([]byte, error) {
	target, err := s.resolvePath(objectKey)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(target)
	if err != nil {
		return nil, fmt.Errorf("read media file: %w", err)
	}
	return data, nil
}

// Exists reports whether the object file is present.
func (s *DiskStorage) Exists(objectKey string) bool {
	target, err := s.resolvePath(objectKey)
	if err != nil {
		return false
	}
	info, err := os.Stat(target)
	return err == nil && !info.IsDir()
}

// Delete removes the object file if present.
func (s *DiskStorage) Delete(objectKey string) error {
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

func (s *DiskStorage) resolvePath(objectKey string) (string, error) {
	cleanKey := filepath.Clean(strings.TrimSpace(objectKey))
	if cleanKey == "" || cleanKey == "." {
		return "", fmt.Errorf("invalid media object_key")
	}
	// filepath.Clean on keys uses OS separators; normalize to forward slashes for keys then join.
	cleanKey = strings.ReplaceAll(cleanKey, "\\", "/")
	cleanKey = strings.TrimPrefix(cleanKey, "/")
	fullPath := filepath.Clean(filepath.Join(s.rootDir, filepath.FromSlash(cleanKey)))
	rel, err := filepath.Rel(s.rootDir, fullPath)
	if err != nil {
		return "", fmt.Errorf("resolve media object_key: %w", err)
	}
	if strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("invalid media object_key path traversal")
	}
	return fullPath, nil
}

// AvatarObjectKey builds the canonical object key for a user avatar file.
func AvatarObjectKey(userID, assetID, ext string) string {
	userID = strings.TrimSpace(userID)
	assetID = strings.TrimSpace(assetID)
	ext = strings.TrimPrefix(strings.TrimSpace(ext), ".")
	if ext == "" {
		ext = "bin"
	}
	return fmt.Sprintf("users/%s/avatar/%s.%s", userID, assetID, ext)
}
