package inmemory

import (
	"context"
	"net/http"
	"sync"

	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

type TokenManager struct {
	mu               sync.RWMutex
	idgen            idgen.Generator
	accessTokens     map[string]iamapp.AccessTokenClaims
	preCompanyTokens map[string]iamapp.PreCompanyTokenClaims
}

func NewTokenManager(idgen idgen.Generator) *TokenManager {
	return &TokenManager{
		idgen:            idgen,
		accessTokens:     map[string]iamapp.AccessTokenClaims{},
		preCompanyTokens: map[string]iamapp.PreCompanyTokenClaims{},
	}
}

func (m *TokenManager) IssueAccessToken(_ context.Context, claims iamapp.AccessTokenClaims) (string, int64, error) {
	t := "atk_" + m.idgen.NewUUID()
	m.mu.Lock()
	m.accessTokens[t] = claims
	m.mu.Unlock()
	return t, 900, nil
}

func (m *TokenManager) IssuePreCompanyToken(_ context.Context, userID, sessionID string) (string, int64, error) {
	t := "ptk_" + m.idgen.NewUUID()
	m.mu.Lock()
	m.preCompanyTokens[t] = iamapp.PreCompanyTokenClaims{Sub: userID, SessionID: sessionID}
	m.mu.Unlock()
	return t, 900, nil
}

func (m *TokenManager) IssueRefreshToken(_ context.Context, sessionID, userID string) (string, error) {
	return "rtk_" + m.idgen.NewUUID(), nil
}

func (m *TokenManager) InspectAccessToken(_ context.Context, token string) (*iamapp.AccessTokenClaims, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	claims, ok := m.accessTokens[token]
	if !ok {
		return nil, perr.NewHTTPError(http.StatusUnauthorized, perr.CodeSessionExpired, "invalid access token", nil)
	}
	cp := claims
	return &cp, nil
}

func (m *TokenManager) InspectPreCompanyToken(_ context.Context, token string) (*iamapp.PreCompanyTokenClaims, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	claims, ok := m.preCompanyTokens[token]
	if !ok {
		return nil, perr.NewHTTPError(http.StatusUnauthorized, perr.CodeSessionExpired, "invalid pre-company token", nil)
	}
	cp := claims
	return &cp, nil
}
