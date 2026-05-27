package mediaupload

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
	"time"
)

// Purpose separates signature domains to prevent cross-feature replay (e.g. CMS vs user avatar).
type Purpose string

const (
	PurposeUserAvatarUpload  Purpose = "user_avatar_upload"
	PurposeUserAvatarContent Purpose = "user_avatar_content"
)

// SignInput binds a signed URL to owner, asset, HTTP method, path, and optional upload metadata.
type SignInput struct {
	Purpose     Purpose
	OwnerID     string
	AssetID     string
	Method      string
	Path        string
	ContentType string
	SizeBytes   int64
	ExpiresAt   time.Time
}

// Signer issues and verifies HMAC-SHA256 signatures for signed upload and content URLs.
type Signer struct {
	secret []byte
	ttl    time.Duration
}

// NewSigner creates a signer. secret must be non-empty for production use.
func NewSigner(secret string, ttl time.Duration) *Signer {
	secret = strings.TrimSpace(secret)
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	return &Signer{
		secret: []byte(secret),
		ttl:    ttl,
	}
}

// TTL returns the configured default signature lifetime.
func (s *Signer) TTL() time.Duration {
	return s.ttl
}

// Sign returns a hex HMAC signature for the input.
func (s *Signer) Sign(in SignInput) string {
	mac := hmac.New(sha256.New, s.secret)
	_, _ = io.WriteString(mac, payload(in))
	return hex.EncodeToString(mac.Sum(nil))
}

// Verify checks a signature against the input.
func (s *Signer) Verify(sig string, in SignInput) bool {
	expected := s.Sign(in)
	return hmac.Equal([]byte(strings.ToLower(strings.TrimSpace(sig))), []byte(expected))
}

// VerifyNotExpired returns false when now is after ExpiresAt.
func VerifyNotExpired(in SignInput, now time.Time) bool {
	if in.ExpiresAt.IsZero() {
		return false
	}
	return !now.After(in.ExpiresAt)
}

func payload(in SignInput) string {
	expUnix := in.ExpiresAt.Unix()
	return fmt.Sprintf(
		"%s|%s|%s|%s|%s|%s|%d|%d",
		string(in.Purpose),
		strings.TrimSpace(in.OwnerID),
		strings.TrimSpace(in.AssetID),
		strings.ToUpper(strings.TrimSpace(in.Method)),
		normalizePath(in.Path),
		strings.ToLower(strings.TrimSpace(in.ContentType)),
		in.SizeBytes,
		expUnix,
	)
}

func normalizePath(path string) string {
	p := strings.TrimSpace(path)
	if p == "" {
		return ""
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return p
}
