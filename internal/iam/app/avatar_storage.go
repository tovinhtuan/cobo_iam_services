package app

import "io"

// AvatarObjectStorage reads and writes avatar binaries by object key.
type AvatarObjectStorage interface {
	Write(objectKey string, body io.Reader) (int64, error)
	Read(objectKey string) ([]byte, error)
	Delete(objectKey string) error
	Exists(objectKey string) bool
}
