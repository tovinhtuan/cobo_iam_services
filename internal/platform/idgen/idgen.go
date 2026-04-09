package idgen

import (
	"github.com/google/uuid"
)

// Generator produces opaque identifiers.
type Generator interface {
	NewUUID() string
}

// UUIDv7Generator generates time-ordered UUIDs when supported by the library (falls back to v4).
type UUIDv7Generator struct{}

func (UUIDv7Generator) NewUUID() string {
	id, err := uuid.NewV7()
	if err == nil {
		return id.String()
	}
	return uuid.NewString()
}
