package inmemory

import (
	"context"
	"net/http"
	"sync"

	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

type SessionRepository struct {
	mu             sync.RWMutex
	byRefreshToken map[string]*iamapp.SessionState
	bySessionID    map[string]*iamapp.SessionState
}

func NewSessionRepository() *SessionRepository {
	return &SessionRepository{byRefreshToken: map[string]*iamapp.SessionState{}, bySessionID: map[string]*iamapp.SessionState{}}
}

func (r *SessionRepository) Create(_ context.Context, p iamapp.CreateSessionParams) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	ss := &iamapp.SessionState{SessionID: p.SessionID, UserID: p.UserID, MembershipID: p.MembershipID, CompanyID: p.CompanyID, RefreshToken: p.RefreshToken}
	r.byRefreshToken[p.RefreshToken] = ss
	r.bySessionID[p.SessionID] = ss
	return nil
}

func (r *SessionRepository) FindByRefreshToken(_ context.Context, refreshToken string) (*iamapp.SessionState, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ss, ok := r.byRefreshToken[refreshToken]
	if !ok {
		return nil, perr.NewHTTPError(http.StatusUnauthorized, perr.CodeSessionExpired, "session expired", nil)
	}
	cp := *ss
	return &cp, nil
}

func (r *SessionRepository) RevokeByRefreshToken(_ context.Context, refreshToken string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	ss, ok := r.byRefreshToken[refreshToken]
	if !ok {
		return perr.NewHTTPError(http.StatusUnauthorized, perr.CodeSessionExpired, "session expired", nil)
	}
	ss.Revoked = true
	return nil
}

func (r *SessionRepository) UpdateContext(_ context.Context, sessionID, membershipID, companyID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	ss, ok := r.bySessionID[sessionID]
	if !ok {
		return perr.NewHTTPError(http.StatusUnauthorized, perr.CodeSessionExpired, "session expired", nil)
	}
	ss.MembershipID = membershipID
	ss.CompanyID = companyID
	return nil
}

func (r *SessionRepository) RotateRefreshToken(_ context.Context, sessionID, newRefreshToken string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	ss, ok := r.bySessionID[sessionID]
	if !ok {
		return perr.NewHTTPError(http.StatusUnauthorized, perr.CodeSessionExpired, "session expired", nil)
	}
	delete(r.byRefreshToken, ss.RefreshToken)
	ss.RefreshToken = newRefreshToken
	r.byRefreshToken[newRefreshToken] = ss
	return nil
}
