package app

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strings"
	"time"
)

const (
	EnvTemplateImportHistoryEnabled = "CMS_TEMPLATE_IMPORT_HISTORY_ENABLED"
	TemplateImportRetentionDays     = 365

	ImportAttemptStatusValidationFailed = "VALIDATION_FAILED"
	ImportAttemptStatusMappingRequired  = "MAPPING_REQUIRED"
	ImportAttemptStatusValidated        = "VALIDATED"
	ImportAttemptStatusConfirmed        = "CONFIRMED"
	ImportAttemptStatusConfirmFailed    = "CONFIRM_FAILED"
	ImportAttemptStatusConfirming       = "CONFIRMING"

	// ImportConfirmLeaseDuration is how long a CONFIRMING claim blocks another request.
	ImportConfirmLeaseDuration = 10 * time.Minute
)

// importConfirmNow is the clock for confirm leases. Tests replace it.
var importConfirmNow = time.Now

// SetImportConfirmClockForTest replaces the confirm lease clock and returns a restore func.
func SetImportConfirmClockForTest(now func() time.Time) func() {
	prev := importConfirmNow
	if now == nil {
		importConfirmNow = time.Now
	} else {
		importConfirmNow = now
	}
	return func() { importConfirmNow = prev }
}

// ImportHistoryEnabled reports whether attempt rows are written and history reads are served.
// Unset and any value other than true, 1, or yes is off.
func ImportHistoryEnabled() bool {
	v := strings.TrimSpace(os.Getenv(EnvTemplateImportHistoryEnabled))
	return strings.EqualFold(v, "true") || v == "1" || strings.EqualFold(v, "yes")
}

func templateImportRetentionCutoff(now time.Time) time.Time {
	return now.UTC().Add(-time.Duration(TemplateImportRetentionDays) * 24 * time.Hour)
}

func importSHA256Hex(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func sha256BytesHex(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
