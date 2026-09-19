package workflowstepcomments

import (
	"crypto/sha256"
	"encoding/hex"
)

// hashRequestBody is the V1 request hash: SHA256(utf8(trimmed_body)).
// Kept for V1 compatibility and empty-mentions equivalence proofs.
func hashRequestBody(body string) string {
	sum := sha256.Sum256([]byte(body))
	return hex.EncodeToString(sum[:])
}

// HashCreateRequest computes V1.1 create RequestHash.
// When canonicalMentionsJSON is empty or "[]", result MUST equal hashRequestBody(trimmedBody).
func HashCreateRequest(trimmedBody, canonicalMentionsJSON string) string {
	if canonicalMentionsJSON == "" || canonicalMentionsJSON == "[]" {
		return hashRequestBody(trimmedBody)
	}
	payload := trimmedBody + "\n" + canonicalMentionsJSON
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}
