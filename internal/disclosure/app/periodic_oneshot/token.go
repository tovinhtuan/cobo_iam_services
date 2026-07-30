package periodic_oneshot

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// BuildConfirmToken is deterministic from preview snapshot fields (not a secret).
func BuildConfirmToken(
	environment, database, typeID, companyID, period string,
	activeVersion int,
	dueDate, snapshotChecksum, plannedAction string,
) string {
	payload := strings.Join([]string{
		strings.TrimSpace(environment),
		strings.TrimSpace(database),
		strings.TrimSpace(typeID),
		strings.TrimSpace(companyID),
		strings.TrimSpace(period),
		fmt.Sprintf("%d", activeVersion),
		strings.TrimSpace(dueDate),
		strings.TrimSpace(snapshotChecksum),
		strings.TrimSpace(plannedAction),
		"periodic-oneshot-v1",
	}, "|")
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}

func ConfirmTokenOK(got, want string) error {
	g := strings.TrimSpace(got)
	w := strings.TrimSpace(want)
	if g == "" {
		return fmt.Errorf("confirm token required")
	}
	if len(g) < 16 {
		return fmt.Errorf("confirm token too short")
	}
	if g != w {
		return fmt.Errorf("confirm token mismatch")
	}
	return nil
}

func SnapshotChecksum(parts ...string) string {
	joined := strings.Join(parts, "|")
	sum := sha256.Sum256([]byte(joined))
	return hex.EncodeToString(sum[:8])
}
