package app

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

// ComputeCanonicalTemplatePayloadHash computes the deterministic SHA-256 hex hash
// of the normalized template authoring definition.
func ComputeCanonicalTemplatePayloadHash(template *TemplateImportDefinitionV1) (string, error) {
	if template == nil {
		return "", errors.New("cannot hash nil template definition")
	}
	canonicalJSON, err := json.Marshal(template)
	if err != nil {
		return "", fmt.Errorf("canonical json marshal: %w", err)
	}
	h := sha256.Sum256(canonicalJSON)
	return hex.EncodeToString(h[:]), nil
}

// TemplateImportSigner manages HMAC issuance and verification for stateless validation tokens.
type TemplateImportSigner struct {
	secret []byte
	ttl    time.Duration
}

// ResolveTemplateImportSigningSecret resolves the signing secret from explicit parameter
// or from environment variable CMS_TEMPLATE_IMPORT_SIGNING_SECRET.
// It has NO fallback to media upload secrets or other domains (strict domain separation).
// It returns empty string if not configured (strictly fail closed, no hardcoded fallbacks).
func ResolveTemplateImportSigningSecret(explicitSecret string) string {
	s := strings.TrimSpace(explicitSecret)
	if s == "" {
		s = strings.TrimSpace(os.Getenv("CMS_TEMPLATE_IMPORT_SIGNING_SECRET"))
	}
	return s
}

// NewTemplateImportSigner constructs a signer using secret or environment.
// Fails closed if no secret is available.
func NewTemplateImportSigner(secret string, ttl time.Duration) *TemplateImportSigner {
	s := ResolveTemplateImportSigningSecret(secret)
	if ttl <= 0 {
		ttl = TemplateImportTokenTTLMinutes * time.Minute
	}
	return &TemplateImportSigner{
		secret: []byte(s),
		ttl:    ttl,
	}
}

// IssueToken creates a stateless HMAC-signed token binding the claims.
func (s *TemplateImportSigner) IssueToken(claims TemplateImportTokenClaims) (string, error) {
	if len(s.secret) == 0 {
		return "", errors.New("CMS template import signing secret is not configured: fail closed")
	}
	claims.SchemaVersion = TemplateImportSchemaVersion
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("marshal claims: %w", err)
	}

	claimsB64 := base64.RawURLEncoding.EncodeToString(claimsJSON)
	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write([]byte(TemplateImportTokenPurpose + ":" + claimsB64))
	sigB64 := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return claimsB64 + "." + sigB64, nil
}

// VerifyToken validates the token signature, schema_version, expiration, and actor.
func (s *TemplateImportSigner) VerifyToken(token string, expectedActorID string, now time.Time) (*TemplateImportTokenClaims, error) {
	if len(s.secret) == 0 {
		return nil, errors.New("CMS template import signing secret is not configured: fail closed")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return nil, errors.New("invalid token format: expected claims.signature")
	}
	claimsB64, sigB64 := parts[0], parts[1]

	sig, err := base64.RawURLEncoding.DecodeString(sigB64)
	if err != nil {
		return nil, fmt.Errorf("decode token signature: %w", err)
	}

	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write([]byte(TemplateImportTokenPurpose + ":" + claimsB64))
	expectedSig := mac.Sum(nil)

	if !hmac.Equal(sig, expectedSig) {
		return nil, errors.New("invalid token signature or purpose mismatch")
	}

	claimsJSON, err := base64.RawURLEncoding.DecodeString(claimsB64)
	if err != nil {
		return nil, fmt.Errorf("decode token claims: %w", err)
	}

	var claims TemplateImportTokenClaims
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		return nil, fmt.Errorf("unmarshal token claims: %w", err)
	}

	if claims.SchemaVersion != TemplateImportSchemaVersion {
		return nil, fmt.Errorf("unsupported token schema_version: %q", claims.SchemaVersion)
	}

	if now.Unix() > claims.ExpiresAt {
		return nil, errors.New("validation token has expired")
	}

	if expectedActorID != "" && claims.ActorID != expectedActorID {
		return nil, fmt.Errorf("token actor mismatch: expected %q, got %q", expectedActorID, claims.ActorID)
	}

	return &claims, nil
}
