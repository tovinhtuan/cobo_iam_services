package workflowdept

import (
	"crypto/sha256"
	"strings"
)

// ScopeKeyHash is SHA-256 of the canonical UTF-8 scope string.
// Empty disclosure type and step use an empty sentinel, not the word NULL.
func ScopeKeyHash(companyID, templateCode, disclosureTypeID, stepCode string) []byte {
	canon := strings.TrimSpace(companyID) + "|" +
		strings.TrimSpace(templateCode) + "|" +
		strings.TrimSpace(disclosureTypeID) + "|" +
		strings.TrimSpace(stepCode)
	sum := sha256.Sum256([]byte(canon))
	out := make([]byte, sha256.Size)
	copy(out, sum[:])
	return out
}
