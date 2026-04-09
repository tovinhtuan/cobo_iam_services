package app

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	ca "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

type service struct {
	cred        CredentialVerifier
	sessions    SessionRepository
	tokens      TokenIssuer
	memberships ca.MembershipQueryService
	idgen       idgen.Generator
	mfa         MFACheck
	sso         SSOLoginBridge
}

func NewService(cred CredentialVerifier, sessions SessionRepository, tokens TokenIssuer, memberships ca.MembershipQueryService, idgen idgen.Generator, opts ...ServiceOption) Service {
	s := &service{cred: cred, sessions: sessions, tokens: tokens, memberships: memberships, idgen: idgen}
	for _, opt := range opts {
		if opt != nil {
			opt(s)
		}
	}
	return s
}

func (s *service) Login(ctx context.Context, req LoginRequest) (*LoginResponse, error) {
	var user *AuthenticatedUser
	if s.sso != nil {
		u, handled, err := s.sso.TryExternalPrimaryAuth(ctx, req)
		if err != nil {
			return nil, err
		}
		if handled {
			if u == nil {
				return nil, perr.NewHTTPError(http.StatusInternalServerError, perr.CodeInternal, "sso bridge returned no user", nil)
			}
			user = u
		}
	}
	if user == nil {
		if strings.TrimSpace(req.LoginID) == "" || strings.TrimSpace(req.Password) == "" {
			return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "login_id and password are required", nil)
		}
		var err error
		user, err = s.cred.Verify(ctx, req.LoginID, req.Password)
		if err != nil {
			return nil, err
		}
	}
	if strings.ToLower(user.Status) != "active" {
		return nil, perr.NewHTTPError(http.StatusForbidden, perr.CodeAccountLocked, "account is not active", nil)
	}

	if s.mfa != nil {
		if err := s.mfa.VerifyAfterPrimaryAuth(ctx, user, req); err != nil {
			return nil, err
		}
	}

	memberships, err := s.memberships.GetMembershipsByUser(ctx, user.UserID)
	if err != nil {
		return nil, fmt.Errorf("list memberships: %w", err)
	}
	active := make([]ca.MembershipView, 0, len(memberships))
	for _, m := range memberships {
		if strings.EqualFold(m.Status, "active") {
			active = append(active, m)
		}
	}
	if len(active) == 0 {
		return nil, perr.NewHTTPError(http.StatusForbidden, perr.CodeNoActiveCompanyAccess, "User does not have any active company membership.", nil)
	}

	sid := s.idgen.NewUUID()
	refresh, err := s.tokens.IssueRefreshToken(ctx, sid, user.UserID)
	if err != nil {
		return nil, fmt.Errorf("issue refresh token: %w", err)
	}

	resp := &LoginResponse{
		User:    LoginUser{UserID: user.UserID, FullName: user.FullName},
		Session: LoginSession{RefreshToken: refresh},
	}

	if len(active) == 1 {
		m := active[0]
		access, exp, err := s.tokens.IssueAccessToken(ctx, AccessTokenClaims{Sub: user.UserID, SessionID: sid, MembershipID: m.MembershipID, CompanyID: m.CompanyID})
		if err != nil {
			return nil, fmt.Errorf("issue access token: %w", err)
		}
		resp.Session.AccessToken = access
		resp.Session.ExpiresIn = exp
		resp.CurrentContext = &LoginCurrentContext{CompanyID: m.CompanyID, MembershipID: m.MembershipID, AutoSelected: true}
		resp.NextAction = "load_effective_access"
		if err := s.sessions.Create(ctx, CreateSessionParams{SessionID: sid, UserID: user.UserID, MembershipID: m.MembershipID, CompanyID: m.CompanyID, RefreshToken: refresh, IP: req.IP, UserAgent: req.UserAgent}); err != nil {
			return nil, fmt.Errorf("create session: %w", err)
		}
		return resp, nil
	}

	pre, exp, err := s.tokens.IssuePreCompanyToken(ctx, user.UserID, sid)
	if err != nil {
		return nil, fmt.Errorf("issue pre-company token: %w", err)
	}
	resp.Session.PreCompanyToken = pre
	resp.Session.ExpiresIn = exp
	resp.NextAction = "select_company"
	resp.Memberships = make([]LoginMembershipSummary, 0, len(active))
	for _, m := range active {
		resp.Memberships = append(resp.Memberships, LoginMembershipSummary{CompanyID: m.CompanyID, CompanyName: m.CompanyName, MembershipID: m.MembershipID})
	}
	if err := s.sessions.Create(ctx, CreateSessionParams{SessionID: sid, UserID: user.UserID, RefreshToken: refresh, IP: req.IP, UserAgent: req.UserAgent}); err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}
	return resp, nil
}

func (s *service) Refresh(ctx context.Context, req RefreshRequest) (*RefreshResponse, error) {
	if strings.TrimSpace(req.RefreshToken) == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "refresh_token is required", nil)
	}
	ss, err := s.sessions.FindByRefreshToken(ctx, req.RefreshToken)
	if err != nil {
		return nil, err
	}
	if ss.Revoked {
		return nil, perr.NewHTTPError(http.StatusUnauthorized, perr.CodeSessionExpired, "session expired", nil)
	}
	if ss.CompanyID == "" || ss.MembershipID == "" {
		return nil, perr.NewHTTPError(http.StatusUnprocessableEntity, perr.CodeCompanyContextRequired, "company context is required", nil)
	}
	access, exp, err := s.tokens.IssueAccessToken(ctx, AccessTokenClaims{Sub: ss.UserID, SessionID: ss.SessionID, MembershipID: ss.MembershipID, CompanyID: ss.CompanyID})
	if err != nil {
		return nil, fmt.Errorf("issue access token: %w", err)
	}
	newRefresh, err := s.tokens.IssueRefreshToken(ctx, ss.SessionID, ss.UserID)
	if err != nil {
		return nil, fmt.Errorf("issue refresh token: %w", err)
	}
	if err := s.sessions.RotateRefreshToken(ctx, ss.SessionID, newRefresh); err != nil {
		return nil, fmt.Errorf("rotate refresh token: %w", err)
	}
	return &RefreshResponse{
		AccessToken:    access,
		RefreshToken:   newRefresh,
		ExpiresIn:      exp,
		CurrentContext: TokenContext{CompanyID: ss.CompanyID, MembershipID: ss.MembershipID},
	}, nil
}

func (s *service) Logout(ctx context.Context, req LogoutRequest) (*LogoutResponse, error) {
	if strings.TrimSpace(req.RefreshToken) == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "refresh_token is required", nil)
	}
	if err := s.sessions.RevokeByRefreshToken(ctx, req.RefreshToken); err != nil {
		return nil, err
	}
	return &LogoutResponse{Success: true}, nil
}

func (s *service) SelectCompany(ctx context.Context, req SelectCompanyRequest) (*SelectCompanyResponse, error) {
	return s.bindCompany(ctx, req.UserID, req.SessionID, req.CompanyID)
}

func (s *service) SwitchCompany(ctx context.Context, req SwitchCompanyRequest) (*SwitchCompanyResponse, error) {
	bound, err := s.bindCompany(ctx, req.UserID, req.SessionID, req.CompanyID)
	if err != nil {
		return nil, err
	}
	return &SwitchCompanyResponse{AccessToken: bound.AccessToken, ExpiresIn: bound.ExpiresIn, CurrentContext: bound.CurrentContext}, nil
}

func (s *service) bindCompany(ctx context.Context, userID, sessionID, companyID string) (*SelectCompanyResponse, error) {
	if userID == "" || sessionID == "" {
		return nil, perr.NewHTTPError(http.StatusUnauthorized, perr.CodeSessionExpired, "invalid session token", nil)
	}
	if strings.TrimSpace(companyID) == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "company_id is required", nil)
	}
	m, err := s.memberships.GetActiveMembership(ctx, userID, companyID)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, perr.NewHTTPError(http.StatusForbidden, perr.CodeMembershipNotFound, "membership not found in company", nil)
	}
	if err := s.sessions.UpdateContext(ctx, sessionID, m.MembershipID, m.CompanyID); err != nil {
		return nil, fmt.Errorf("update session context: %w", err)
	}
	access, exp, err := s.tokens.IssueAccessToken(ctx, AccessTokenClaims{Sub: userID, SessionID: sessionID, MembershipID: m.MembershipID, CompanyID: m.CompanyID})
	if err != nil {
		return nil, fmt.Errorf("issue access token: %w", err)
	}
	return &SelectCompanyResponse{AccessToken: access, ExpiresIn: exp, CurrentContext: TokenContext{CompanyID: m.CompanyID, MembershipID: m.MembershipID}}, nil
}
