// Package companystatus defines write-side allowlists for companies.status and
// companies.verification_status. Reads remain tolerant of legacy/unknown values.
package companystatus

import (
	"fmt"
	"strings"
)

const (
	StatusActive    = "active"
	StatusInactive  = "inactive"
	StatusSuspended = "suspended"

	VerificationVerified   = "verified"
	VerificationUnverified = "unverified"
)

// BlocksAccess reports whether a company's operational status shuts out its members. Only
// "inactive" ("Ngừng hoạt động") does; other values, legacy ones included, keep access. A company
// that cannot be found is reported by callers as "" and treated as blocked by them, not here.
func BlocksAccess(status string) bool {
	return strings.EqualFold(strings.TrimSpace(status), StatusInactive)
}

// IsReadOnly reports whether members only keep read access (view and export): "suspended"
// ("Tạm ngưng", e.g. unpaid), because disclosure deadlines still apply to the company.
func IsReadOnly(status string) bool {
	return strings.EqualFold(strings.TrimSpace(status), StatusSuspended)
}

// NormalizeOperationalStatus trims and lowercases, then allowlists active|inactive|suspended.
// Matches existing SetCompanyStatusPlatform normalization (ToLower+TrimSpace).
func NormalizeOperationalStatus(raw string) (string, error) {
	v := strings.ToLower(strings.TrimSpace(raw))
	switch v {
	case StatusActive, StatusInactive, StatusSuspended:
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
