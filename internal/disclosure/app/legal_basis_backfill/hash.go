package backfill

import (
	"crypto/sha256"
	"encoding/hex"
)

// ShortHash matches Phase 12.6A inventory evidence hash (sha256 first 8 bytes hex).
func ShortHash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:8])
}

func ShortHashBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:8])
}

func FileSHA256(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
