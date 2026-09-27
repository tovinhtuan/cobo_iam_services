package workflowdept

import (
	"encoding/base64"
	"errors"
	"os"
	"strconv"
	"strings"
)

const (
	EnvRecipientKey        = "WORKFLOW_DEPARTMENT_EMAIL_RECIPIENT_KEY"
	EnvRecipientKeyVersion = "WORKFLOW_DEPARTMENT_EMAIL_RECIPIENT_KEY_VERSION"
)

// ErrRecipientKeyMissing is fail-closed: do not send when the key is absent or invalid.
var ErrRecipientKeyMissing = errors.New("recipient key missing")

// RecipientKey is loaded from the process environment, never from the business database.
type RecipientKey struct {
	Key     []byte
	Version uint32
}

// LoadRecipientKey reads a 32-byte base64 key. The returned error is redacted and does not include key bytes.
func LoadRecipientKey() (RecipientKey, error) {
	raw := strings.TrimSpace(os.Getenv(EnvRecipientKey))
	verRaw := strings.TrimSpace(os.Getenv(EnvRecipientKeyVersion))
	if raw == "" || verRaw == "" {
		return RecipientKey{}, ErrRecipientKeyMissing
	}
	ver, err := strconv.ParseUint(verRaw, 10, 32)
	if err != nil || ver == 0 {
		return RecipientKey{}, ErrRecipientKeyMissing
	}
	key, err := base64.StdEncoding.DecodeString(raw)
	if err != nil || len(key) != 32 {
		return RecipientKey{}, ErrRecipientKeyMissing
	}
	return RecipientKey{Key: key, Version: uint32(ver)}, nil
}
