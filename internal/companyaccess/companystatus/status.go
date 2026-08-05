// Package companystatus defines write-side allowlists for companies.status and
// companies.verification_status. Reads remain tolerant of legacy/unknown values.
package companystatus

import (
	"fmt"
	"strings"
)

const (
	StatusActive   = "active"
	StatusInactive = "inactive"

	VerificationVerified   = "verified"
	VerificationUnverified = "unverified"
)

// NormalizeOperationalStatus trims and lowercases, then allowlists active|inactive.
// Matches existing SetCompanyStatusPlatform normalization (ToLower+TrimSpace).
func NormalizeOperationalStatus(raw string) (string, error) {
	v := strings.ToLower(strings.TrimSpace(raw))
	switch v {
	case StatusActive, StatusInactive:
		return v, nil
	case "":
		return "", fmt.Errorf("company status is required")
	default:
		return "", fmt.Errorf("invalid company status")
	}
}

// NormalizeVerificationStatus trims and lowercases, then allowlists verified|unverified.
// Write-side casing aligned with operational status + CMS list filter LOWER(TRIM(...)).
func NormalizeVerificationStatus(raw string) (string, error) {
	v := strings.ToLower(strings.TrimSpace(raw))
	switch v {
	case VerificationVerified, VerificationUnverified:
		return v, nil
	case "":
		return "", fmt.Errorf("verification_status is required")
	default:
		return "", fmt.Errorf("invalid verification_status")
	}
}
