package refreshtoken

import (
	"crypto/sha256"
	"encoding/hex"
)

// Hash returns a stable hex digest for storing opaque refresh tokens (sessions.refresh_token_hash).
func Hash(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
