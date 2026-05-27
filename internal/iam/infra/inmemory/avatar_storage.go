package inmemory

import (
	"bytes"
	"io"
	"sync"
)

// AvatarStorage is an in-memory object store for avatar tests.
type AvatarStorage struct {
	mu    sync.Mutex
	files map[string][]byte
}

// NewAvatarStorage creates empty avatar storage.
func NewAvatarStorage() *AvatarStorage {
	return &AvatarStorage{files: map[string][]byte{}}
}

func (s *AvatarStorage) Write(objectKey string, body io.Reader) (int64, error) {
	data, err := io.ReadAll(body)
	if err != nil {
		return 0, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.files[objectKey] = append([]byte(nil), data...)
	return int64(len(data)), nil
}

func (s *AvatarStorage) Read(objectKey string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, ok := s.files[objectKey]
	if !ok {
		return nil, io.EOF
	}
	return append([]byte(nil), data...), nil
}

func (s *AvatarStorage) Delete(objectKey string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.files, objectKey)
	return nil
}

func (s *AvatarStorage) Exists(objectKey string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.files[objectKey]
	return ok
}

// Bytes returns a copy of stored bytes (test helper).
func (s *AvatarStorage) Bytes(objectKey string) []byte {
	data, _ := s.Read(objectKey)
	return data
}

// WriteBytes stores bytes directly (test helper).
func (s *AvatarStorage) WriteBytes(objectKey string, data []byte) {
	_, _ = s.Write(objectKey, bytes.NewReader(data))
}
