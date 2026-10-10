package sessionbound

import (
	"context"
	"net/http"
	"strings"

	"github.com/cobo/cobo_iam_services/internal/companyaccess/companystatus"
	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// Manager wraps a token issuer/inspector and rejects tokens whose session row is missing or revoked.
// Opaque access tokens live in process memory; without this check, APIs that only inspect the token
// can succeed while stateful operations (e.g. switch-company UpdateContext) fail with SESSION_EXPIRED.
type Manager struct {
	inner     tokenBackend
	sessions  iamapp.SessionRepository
	companies iamapp.CompanyStatusReader
}

type tokenBackend interface {
	iamapp.TokenIssuer
	iamapp.TokenInspector
}

// Option configures a Manager.
type Option func(*Manager)

// WithCompanyStatus also rejects access tokens bound to a deactivated or missing company
// (403 COMPANY_INACTIVE), and write requests (iamapp.IsWriteRequest) in a suspended company
// (403 COMPANY_SUSPENDED), on every request.
func WithCompanyStatus(r iamapp.CompanyStatusReader) Option {
	return func(m *Manager) { m.companies = r }
}

func New(inner tokenBackend, sessions iamapp.SessionRepository, opts ...Option) *Manager {
	m := &Manager{inner: inner, sessions: sessions}
	for _, o := range opts {
		o(m)
	}
	return m
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
	if err := m.assertCompanyUsable(ctx, claims.CompanyID); err != nil {
		return nil, err
	}
	return claims, nil
}

func (m *Manager) assertCompanyUsable(ctx context.Context, companyID string) error {
	companyID = strings.TrimSpace(companyID)
	if m.companies == nil || companyID == "" {
		return nil
	}
	status, err := m.companies.CompanyStatus(ctx, companyID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(status) == "" || companystatus.BlocksAccess(status) {
		return perr.NewHTTPError(http.StatusForbidden, perr.CodeCompanyInactive, "the company is no longer active", nil)
	}
	if companystatus.IsReadOnly(status) && iamapp.IsWriteRequest(ctx) {
		return perr.NewHTTPError(http.StatusForbidden, perr.CodeCompanySuspended,
			"the company is suspended: read and export only", nil)
	}
	return nil
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
