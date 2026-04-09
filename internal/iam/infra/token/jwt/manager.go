package jwt

import (
	"context"
	"fmt"

	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	"github.com/cobo/cobo_iam_services/internal/platform/config"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

// Manager is a placeholder skeleton for signed JWT access tokens.
// TODO(PR1): implement real JWT sign/verify (iss/aud/exp/skew/kid/alg allowlist).
type Manager struct {
	cfg   config.Config
	idgen idgen.Generator
}

func NewManager(cfg config.Config, idgen idgen.Generator) *Manager {
	return &Manager{cfg: cfg, idgen: idgen}
}

func (m *Manager) IssueAccessToken(_ context.Context, claims iamapp.AccessTokenClaims) (string, int64, error) {
	_ = claims
	return "", 0, fmt.Errorf("jwt access token manager not implemented yet")
}

func (m *Manager) IssuePreCompanyToken(_ context.Context, userID, sessionID string) (string, int64, error) {
	_ = userID
	_ = sessionID
	// Keep pre-company token migration out-of-scope for PR1.
	return "", 0, fmt.Errorf("jwt pre-company token manager not implemented yet")
}

func (m *Manager) IssueRefreshToken(_ context.Context, sessionID, userID string) (string, error) {
	_ = sessionID
	_ = userID
	// Refresh token remains opaque; wired by dual/opaque path in PR1.
	return "", fmt.Errorf("jwt refresh token manager not implemented yet")
}

func (m *Manager) InspectAccessToken(_ context.Context, token string) (*iamapp.AccessTokenClaims, error) {
	_ = token
	return nil, fmt.Errorf("jwt access token inspector not implemented yet")
}

func (m *Manager) InspectPreCompanyToken(_ context.Context, token string) (*iamapp.PreCompanyTokenClaims, error) {
	_ = token
	return nil, fmt.Errorf("jwt pre-company inspector not implemented yet")
}
