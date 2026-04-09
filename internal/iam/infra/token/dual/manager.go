package dual

import (
	"context"
	"strings"

	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
)

// Manager issues via primary issuer and verifies JWT first then opaque fallback.
// Intended for migration window (dual mode).
type Manager struct {
	primary interface {
		IssueAccessToken(context.Context, iamapp.AccessTokenClaims) (string, int64, error)
	}
	opaque interface {
		iamapp.TokenIssuer
		iamapp.TokenInspector
	}
	jwt iamapp.TokenInspector
}

func NewManager(primary interface {
	IssueAccessToken(context.Context, iamapp.AccessTokenClaims) (string, int64, error)
}, opaque interface {
	iamapp.TokenIssuer
	iamapp.TokenInspector
}, jwt iamapp.TokenInspector) *Manager {
	return &Manager{primary: primary, opaque: opaque, jwt: jwt}
}

func (m *Manager) IssueAccessToken(ctx context.Context, claims iamapp.AccessTokenClaims) (string, int64, error) {
	return m.primary.IssueAccessToken(ctx, claims)
}

func (m *Manager) IssuePreCompanyToken(ctx context.Context, userID, sessionID string) (string, int64, error) {
	return m.opaque.IssuePreCompanyToken(ctx, userID, sessionID)
}

func (m *Manager) IssueRefreshToken(ctx context.Context, sessionID, userID string) (string, error) {
	return m.opaque.IssueRefreshToken(ctx, sessionID, userID)
}

func (m *Manager) InspectAccessToken(ctx context.Context, token string) (*iamapp.AccessTokenClaims, error) {
	if strings.Count(token, ".") == 2 {
		if claims, err := m.jwt.InspectAccessToken(ctx, token); err == nil {
			return claims, nil
		}
	}
	return m.opaque.InspectAccessToken(ctx, token)
}

func (m *Manager) InspectPreCompanyToken(ctx context.Context, token string) (*iamapp.PreCompanyTokenClaims, error) {
	// Pre-company token still uses opaque path in dual mode.
	return m.opaque.InspectPreCompanyToken(ctx, token)
}
