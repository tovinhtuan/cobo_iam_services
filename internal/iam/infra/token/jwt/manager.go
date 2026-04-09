package jwt

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"fmt"
	"net/http"
	"strings"
	"time"

	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	"github.com/cobo/cobo_iam_services/internal/platform/config"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
	jwtv5 "github.com/golang-jwt/jwt/v5"
)

type Manager struct {
	cfg      config.Config
	idgen    idgen.Generator
	fallback interface {
		IssuePreCompanyToken(context.Context, string, string) (string, int64, error)
		IssueRefreshToken(context.Context, string, string) (string, error)
		InspectPreCompanyToken(context.Context, string) (*iamapp.PreCompanyTokenClaims, error)
	}
	method  jwtv5.SigningMethod
	signKey interface{}
	initErr error
}

type accessClaims struct {
	SessionID    string `json:"session_id"`
	MembershipID string `json:"membership_id"`
	CompanyID    string `json:"company_id"`
	Type         string `json:"typ"`
	jwtv5.RegisteredClaims
}

func NewManager(cfg config.Config, idgen idgen.Generator, fallback interface {
	IssuePreCompanyToken(context.Context, string, string) (string, int64, error)
	IssueRefreshToken(context.Context, string, string) (string, error)
	InspectPreCompanyToken(context.Context, string) (*iamapp.PreCompanyTokenClaims, error)
}) *Manager {
	m := &Manager{cfg: cfg, idgen: idgen, fallback: fallback}
	m.method, m.signKey, m.initErr = m.resolveSigningMaterial()
	return m
}

func (m *Manager) IssueAccessToken(_ context.Context, claims iamapp.AccessTokenClaims) (string, int64, error) {
	if m.initErr != nil {
		return "", 0, m.initErr
	}
	now := time.Now().UTC()
	ttl := m.cfg.AccessTokenTTL
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	ac := accessClaims{
		SessionID:    claims.SessionID,
		MembershipID: claims.MembershipID,
		CompanyID:    claims.CompanyID,
		Type:         "access",
		RegisteredClaims: jwtv5.RegisteredClaims{
			Subject:   claims.Sub,
			Issuer:    m.cfg.JWTIssuer,
			Audience:  []string{m.cfg.JWTAudience},
			IssuedAt:  jwtv5.NewNumericDate(now),
			ExpiresAt: jwtv5.NewNumericDate(now.Add(ttl)),
			ID:        m.idgen.NewUUID(),
		},
	}
	t := jwtv5.NewWithClaims(m.method, ac)
	signed, err := t.SignedString(m.signKey)
	if err != nil {
		return "", 0, fmt.Errorf("sign jwt: %w", err)
	}
	return signed, int64(ttl.Seconds()), nil
}

func (m *Manager) IssuePreCompanyToken(ctx context.Context, userID, sessionID string) (string, int64, error) {
	// Pre-company token stays opaque in PR1.
	return m.fallback.IssuePreCompanyToken(ctx, userID, sessionID)
}

func (m *Manager) IssueRefreshToken(ctx context.Context, sessionID, userID string) (string, error) {
	// Refresh token remains opaque (hash in DB).
	return m.fallback.IssueRefreshToken(ctx, sessionID, userID)
}

func (m *Manager) InspectAccessToken(_ context.Context, token string) (*iamapp.AccessTokenClaims, error) {
	if m.initErr != nil {
		return nil, perr.NewHTTPError(http.StatusUnauthorized, perr.CodeSessionExpired, "invalid access token", m.initErr)
	}
	claims := &accessClaims{}
	parser := jwtv5.NewParser(
		jwtv5.WithValidMethods([]string{m.method.Alg()}),
		jwtv5.WithIssuer(m.cfg.JWTIssuer),
		jwtv5.WithAudience(m.cfg.JWTAudience),
		jwtv5.WithLeeway(time.Duration(max(m.cfg.JWTClockSkewSec, 0))*time.Second),
	)
	_, err := parser.ParseWithClaims(token, claims, func(t *jwtv5.Token) (interface{}, error) {
		return m.verificationKey(), nil
	})
	if err != nil {
		return nil, perr.NewHTTPError(http.StatusUnauthorized, perr.CodeSessionExpired, "invalid access token", err)
	}
	if claims.Type != "" && claims.Type != "access" {
		return nil, perr.NewHTTPError(http.StatusUnauthorized, perr.CodeSessionExpired, "invalid access token type", nil)
	}
	return &iamapp.AccessTokenClaims{
		Sub:          claims.Subject,
		SessionID:    claims.SessionID,
		MembershipID: claims.MembershipID,
		CompanyID:    claims.CompanyID,
	}, nil
}

func (m *Manager) InspectPreCompanyToken(ctx context.Context, token string) (*iamapp.PreCompanyTokenClaims, error) {
	return m.fallback.InspectPreCompanyToken(ctx, token)
}

func (m *Manager) resolveSigningMaterial() (jwtv5.SigningMethod, interface{}, error) {
	alg := strings.ToUpper(strings.TrimSpace(m.cfg.JWTAlg))
	if alg == "" {
		alg = "EDDSA"
	}
	switch alg {
	case "HS256":
		secret := strings.TrimSpace(m.cfg.JWTSigningPrivateKey)
		if secret == "" {
			return nil, nil, fmt.Errorf("JWT_SIGNING_PRIVATE_KEY_PEM required for HS256")
		}
		return jwtv5.SigningMethodHS256, []byte(secret), nil
	case "EDDSA":
		key, err := jwtv5.ParseEdPrivateKeyFromPEM([]byte(m.cfg.JWTSigningPrivateKey))
		if err != nil {
			return nil, nil, fmt.Errorf("parse ed25519 private key: %w", err)
		}
		return jwtv5.SigningMethodEdDSA, key, nil
	case "ES256":
		key, err := jwtv5.ParseECPrivateKeyFromPEM([]byte(m.cfg.JWTSigningPrivateKey))
		if err != nil {
			return nil, nil, fmt.Errorf("parse ecdsa private key: %w", err)
		}
		return jwtv5.SigningMethodES256, key, nil
	default:
		return nil, nil, fmt.Errorf("unsupported JWT_ALG: %s", m.cfg.JWTAlg)
	}
}

func (m *Manager) verificationKey() interface{} {
	switch k := m.signKey.(type) {
	case ed25519.PrivateKey:
		return k.Public()
	case *ecdsa.PrivateKey:
		return &k.PublicKey
	default:
		return k
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
