package sessionbound

import (
	"context"

	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
)

// Manager wraps a token issuer/inspector and rejects tokens whose session row is missing or revoked.
// Opaque access tokens live in process memory; without this check, APIs that only inspect the token
// can succeed while stateful operations (e.g. switch-company UpdateContext) fail with SESSION_EXPIRED.
type Manager struct {
	inner    tokenBackend
	sessions iamapp.SessionRepository
}

type tokenBackend interface {
	iamapp.TokenIssuer
	iamapp.TokenInspector
}

func New(inner tokenBackend, sessions iamapp.SessionRepository) *Manager {
	return &Manager{inner: inner, sessions: sessions}
}

func (m *Manager) IssueAccessToken(ctx context.Context, claims iamapp.AccessTokenClaims) (string, int64, error) {
	if err := m.sessions.AssertSessionActive(ctx, claims.SessionID); err != nil {
		return "", 0, err
	}
	return m.inner.IssueAccessToken(ctx, claims)
}

func (m *Manager) IssuePreCompanyToken(ctx context.Context, userID, sessionID string) (string, int64, error) {
	if err := m.sessions.AssertSessionActive(ctx, sessionID); err != nil {
		return "", 0, err
	}
	return m.inner.IssuePreCompanyToken(ctx, userID, sessionID)
}

func (m *Manager) IssueRefreshToken(ctx context.Context, sessionID, userID string) (string, error) {
	return m.inner.IssueRefreshToken(ctx, sessionID, userID)
}

func (m *Manager) InspectAccessToken(ctx context.Context, token string) (*iamapp.AccessTokenClaims, error) {
	claims, err := m.inner.InspectAccessToken(ctx, token)
	if err != nil {
		return nil, err
	}
	if err := m.sessions.AssertSessionActive(ctx, claims.SessionID); err != nil {
		return nil, err
	}
	return claims, nil
}

func (m *Manager) InspectPreCompanyToken(ctx context.Context, token string) (*iamapp.PreCompanyTokenClaims, error) {
	claims, err := m.inner.InspectPreCompanyToken(ctx, token)
	if err != nil {
		return nil, err
	}
	if err := m.sessions.AssertSessionActive(ctx, claims.SessionID); err != nil {
		return nil, err
	}
	return claims, nil
}
