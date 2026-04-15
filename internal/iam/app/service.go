package app

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	ca "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/events"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
	"github.com/cobo/cobo_iam_services/internal/platform/outbox"
	"github.com/cobo/cobo_iam_services/internal/platform/refreshtoken"
	"golang.org/x/crypto/bcrypt"
)

type service struct {
	cred        CredentialVerifier
	sessions    SessionRepository
	tokens      TokenIssuer
	memberships ca.MembershipQueryService
	idgen       idgen.Generator
	mfa         MFACheck
	sso         SSOLoginBridge
	attempts    LoginAttemptRecorder
	recovery    AuthRecoveryRepository
	outbox      outbox.Publisher
	webBaseURL  string
	passwordResetTTL time.Duration
	emailVerifyTTL time.Duration
}

func NewService(cred CredentialVerifier, sessions SessionRepository, tokens TokenIssuer, memberships ca.MembershipQueryService, idgen idgen.Generator, opts ...ServiceOption) Service {
	s := &service{
		cred: cred, sessions: sessions, tokens: tokens, memberships: memberships, idgen: idgen,
		webBaseURL: "http://localhost:5173", passwordResetTTL: 30 * time.Minute, emailVerifyTTL: 24 * time.Hour,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(s)
		}
	}
	return s
}

func (s *service) Login(ctx context.Context, req LoginRequest) (*LoginResponse, error) {
	loginID := strings.TrimSpace(req.LoginID)
	if loginID == "" {
		loginID = strings.TrimSpace(req.Email)
	}
	var user *AuthenticatedUser
	if s.sso != nil {
		u, handled, err := s.sso.TryExternalPrimaryAuth(ctx, req)
		if err != nil {
			s.recordLoginAttempt(ctx, req, nil, false, err)
			return nil, err
		}
		if handled {
			if u == nil {
				e := perr.NewHTTPError(http.StatusInternalServerError, perr.CodeInternal, "sso bridge returned no user", nil)
				s.recordLoginAttempt(ctx, req, nil, false, e)
				return nil, e
			}
			user = u
		}
	}
	if user == nil {
		if loginID == "" || strings.TrimSpace(req.Password) == "" {
			e := perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "login_id and password are required", nil)
			s.recordLoginAttempt(ctx, req, nil, false, e)
			return nil, e
		}
		var err error
		user, err = s.cred.Verify(ctx, loginID, req.Password)
		if err != nil {
			s.recordLoginAttempt(ctx, req, nil, false, err)
			return nil, err
		}
	}
	if strings.ToLower(user.Status) != "active" {
		e := perr.NewHTTPError(http.StatusForbidden, perr.CodeAccountLocked, "account is not active", nil)
		s.recordLoginAttempt(ctx, req, user, false, e)
		return nil, e
	}

	if s.mfa != nil {
		if err := s.mfa.VerifyAfterPrimaryAuth(ctx, user, req); err != nil {
			s.recordLoginAttempt(ctx, req, user, false, err)
			return nil, err
		}
	}

	memberships, err := s.memberships.GetMembershipsByUser(ctx, user.UserID)
	if err != nil {
		s.recordLoginAttempt(ctx, req, user, false, err)
		return nil, fmt.Errorf("list memberships: %w", err)
	}
	active := make([]ca.MembershipView, 0, len(memberships))
	for _, m := range memberships {
		if strings.EqualFold(m.Status, "active") {
			active = append(active, m)
		}
	}
	if len(active) == 0 {
		e := perr.NewHTTPError(http.StatusForbidden, perr.CodeNoActiveCompanyAccess, "User does not have any active company membership.", nil)
		s.recordLoginAttempt(ctx, req, user, false, e)
		return nil, e
	}

	sid := s.idgen.NewUUID()
	refresh, err := s.tokens.IssueRefreshToken(ctx, sid, user.UserID)
	if err != nil {
		s.recordLoginAttempt(ctx, req, user, false, err)
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
			s.recordLoginAttempt(ctx, req, user, false, err)
			return nil, fmt.Errorf("issue access token: %w", err)
		}
		resp.Session.AccessToken = access
		resp.Session.ExpiresIn = exp
		resp.CurrentContext = &LoginCurrentContext{CompanyID: m.CompanyID, MembershipID: m.MembershipID, AutoSelected: true}
		resp.NextAction = "load_effective_access"
		if err := s.sessions.Create(ctx, CreateSessionParams{SessionID: sid, UserID: user.UserID, MembershipID: m.MembershipID, CompanyID: m.CompanyID, RefreshToken: refresh, IP: req.IP, UserAgent: req.UserAgent}); err != nil {
			s.recordLoginAttempt(ctx, req, user, false, err)
			return nil, fmt.Errorf("create session: %w", err)
		}
		s.recordLoginAttempt(ctx, req, user, true, nil)
		return resp, nil
	}

	pre, exp, err := s.tokens.IssuePreCompanyToken(ctx, user.UserID, sid)
	if err != nil {
		s.recordLoginAttempt(ctx, req, user, false, err)
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
		s.recordLoginAttempt(ctx, req, user, false, err)
		return nil, fmt.Errorf("create session: %w", err)
	}
	s.recordLoginAttempt(ctx, req, user, true, nil)
	return resp, nil
}

func (s *service) recordLoginAttempt(ctx context.Context, req LoginRequest, user *AuthenticatedUser, success bool, errVal error) {
	if s.attempts == nil {
		return
	}
	loginID := strings.TrimSpace(strings.ToLower(req.LoginID))
	if user != nil && strings.TrimSpace(user.LoginID) != "" {
		loginID = strings.TrimSpace(strings.ToLower(user.LoginID))
	}
	if loginID == "" {
		return
	}
	var uid string
	if user != nil {
		uid = user.UserID
	}
	fc := ""
	if !success && errVal != nil {
		if he, ok := perr.AsHTTPError(errVal); ok && he != nil {
			fc = string(he.Code)
		} else {
			fc = "UNKNOWN"
		}
	}
	_ = s.attempts.Record(ctx, LoginAttemptRecord{
		LoginID: loginID, UserID: uid, Success: success, FailureCode: fc,
		IP: req.IP, UserAgent: req.UserAgent,
	})
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

func (s *service) ForgotPassword(ctx context.Context, req ForgotPasswordRequest) (*ForgotPasswordResponse, error) {
	email := strings.TrimSpace(strings.ToLower(req.Email))
	if email == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "email is required", nil)
	}
	// Fallback mode without persistent repo: keep generic success.
	if s.recovery == nil {
		return &ForgotPasswordResponse{Success: true}, nil
	}
	user, err := s.recovery.FindUserByEmail(ctx, email)
	if err != nil || user == nil {
		// Keep generic response to prevent account enumeration.
		return &ForgotPasswordResponse{Success: true}, nil
	}
	rawToken, tokenHash, err := s.generateRawTokenAndHash()
	if err != nil {
		return nil, fmt.Errorf("generate reset token: %w", err)
	}
	if err := s.recovery.StorePasswordResetToken(ctx, RecoveryTokenRecord{
		TokenID: s.idgen.NewUUID(), UserID: user.UserID, TokenHash: tokenHash, ExpiresAt: time.Now().UTC().Add(s.passwordResetTTL),
	}); err != nil {
		return nil, fmt.Errorf("store password reset token: %w", err)
	}
	s.publishEmail(ctx, "auth.password_reset_requested", user.UserID, map[string]any{
		"to": user.Email,
		"subject": "Reset your password",
		"body": fmt.Sprintf("Xin chao %s,\n\nVui long dat lai mat khau qua link sau:\n%s\n\nLink het han sau %d phut.",
			coalesce(user.FullName, user.LoginID), s.buildActionLink("/reset-password", rawToken), int(s.passwordResetTTL.Minutes())),
	})
	return &ForgotPasswordResponse{Success: true}, nil
}

func (s *service) ResendVerificationEmail(ctx context.Context, req ResendVerificationEmailRequest) (*ResendVerificationEmailResponse, error) {
	email := strings.TrimSpace(strings.ToLower(req.Email))
	if email == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "email is required", nil)
	}
	// Fallback mode without persistent repo: keep generic success.
	if s.recovery == nil {
		return &ResendVerificationEmailResponse{Success: true}, nil
	}
	user, err := s.recovery.FindUserByEmail(ctx, email)
	if err != nil || user == nil {
		// Keep generic response to prevent account enumeration.
		return &ResendVerificationEmailResponse{Success: true}, nil
	}
	rawToken, tokenHash, err := s.generateRawTokenAndHash()
	if err != nil {
		return nil, fmt.Errorf("generate verification token: %w", err)
	}
	if err := s.recovery.StoreEmailVerificationToken(ctx, RecoveryTokenRecord{
		TokenID: s.idgen.NewUUID(), UserID: user.UserID, TokenHash: tokenHash, ExpiresAt: time.Now().UTC().Add(s.emailVerifyTTL),
	}); err != nil {
		return nil, fmt.Errorf("store email verification token: %w", err)
	}
	s.publishEmail(ctx, "auth.email_verification_requested", user.UserID, map[string]any{
		"to": user.Email,
		"subject": "Verify your email",
		"body": fmt.Sprintf("Xin chao %s,\n\nVui long xac thuc email qua link sau:\n%s\n\nLink het han trong %d gio.",
			coalesce(user.FullName, user.LoginID), s.buildActionLink("/verify-email", rawToken), int(s.emailVerifyTTL.Hours())),
	})
	return &ResendVerificationEmailResponse{Success: true}, nil
}

func (s *service) ResetPassword(ctx context.Context, req ResetPasswordRequest) (*ResetPasswordResponse, error) {
	if strings.TrimSpace(req.Token) == "" || strings.TrimSpace(req.NewPassword) == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "token and new_password are required", nil)
	}
	if s.recovery == nil {
		// Non-DB fallback for local bootstrap mode.
		if !strings.HasPrefix(strings.TrimSpace(req.Token), "mock-reset-") {
			return nil, perr.NewHTTPError(http.StatusUnauthorized, perr.CodePasswordResetTokenInvalid, "reset token invalid or expired", nil)
		}
		return &ResetPasswordResponse{Success: true}, nil
	}
	tokenHash := refreshtoken.Hash(strings.TrimSpace(req.Token))
	userID, err := s.recovery.ConsumePasswordResetToken(ctx, tokenHash, time.Now().UTC())
	if err != nil || strings.TrimSpace(userID) == "" {
		return nil, perr.NewHTTPError(http.StatusUnauthorized, perr.CodePasswordResetTokenInvalid, "reset token invalid or expired", nil)
	}
	if len(strings.TrimSpace(req.NewPassword)) < 12 {
		return nil, perr.NewHTTPError(http.StatusUnprocessableEntity, perr.CodeInvalidRequest, "new_password must be at least 12 characters", nil)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}
	now := time.Now().UTC()
	if err := s.recovery.UpdatePasswordHash(ctx, userID, string(hash), now); err != nil {
		return nil, fmt.Errorf("update password hash: %w", err)
	}
	_ = s.sessions.RevokeAllByUser(ctx, userID, "password_reset")
	return &ResetPasswordResponse{Success: true}, nil
}

func (s *service) VerifyEmail(ctx context.Context, req VerifyEmailRequest) (*VerifyEmailResponse, error) {
	if strings.TrimSpace(req.Token) == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "token is required", nil)
	}
	if s.recovery == nil {
		// Non-DB fallback for local bootstrap mode.
		if !strings.HasPrefix(strings.TrimSpace(req.Token), "mock-verify-") {
			return nil, perr.NewHTTPError(http.StatusUnauthorized, perr.CodeEmailVerificationTokenInvalid, "verification token invalid or expired", nil)
		}
		return &VerifyEmailResponse{Success: true}, nil
	}
	tokenHash := refreshtoken.Hash(strings.TrimSpace(req.Token))
	userID, err := s.recovery.ConsumeEmailVerificationToken(ctx, tokenHash, time.Now().UTC())
	if err != nil || strings.TrimSpace(userID) == "" {
		return nil, perr.NewHTTPError(http.StatusUnauthorized, perr.CodeEmailVerificationTokenInvalid, "verification token invalid or expired", nil)
	}
	if err := s.recovery.MarkEmailVerified(ctx, userID, time.Now().UTC()); err != nil {
		return nil, fmt.Errorf("mark email verified: %w", err)
	}
	return &VerifyEmailResponse{Success: true}, nil
}

func (s *service) ListSessions(ctx context.Context, req ListSessionsRequest) (*ListSessionsResponse, error) {
	if strings.TrimSpace(req.UserID) == "" {
		return nil, perr.NewHTTPError(http.StatusUnauthorized, perr.CodeSessionExpired, "invalid session token", nil)
	}
	sessions, err := s.sessions.ListByUser(ctx, req.UserID)
	if err != nil {
		return nil, err
	}
	items := make([]SessionView, 0, len(sessions))
	for _, ss := range sessions {
		items = append(items, SessionView{
			SessionID:            ss.SessionID,
			CurrentCompanyID:     ss.CompanyID,
			CurrentMembershipID:  ss.MembershipID,
			IP:                   ss.IP,
			UserAgent:            ss.UserAgent,
			Current:              ss.SessionID == req.CurrentSessionID,
			Revoked:              ss.Revoked,
		})
	}
	return &ListSessionsResponse{Items: items}, nil
}

func (s *service) RevokeSession(ctx context.Context, req RevokeSessionRequest) (*RevokeSessionResponse, error) {
	if strings.TrimSpace(req.UserID) == "" || strings.TrimSpace(req.SessionID) == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "user_id and session_id are required", nil)
	}
	if err := s.sessions.RevokeBySessionID(ctx, req.UserID, req.SessionID); err != nil {
		return nil, err
	}
	return &RevokeSessionResponse{Success: true}, nil
}

func (s *service) generateRawTokenAndHash() (string, string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", err
	}
	raw := base64.RawURLEncoding.EncodeToString(buf)
	return raw, refreshtoken.Hash(raw), nil
}

func (s *service) buildActionLink(path, token string) string {
	base := strings.TrimRight(s.webBaseURL, "/")
	u := base + path
	v := url.Values{}
	v.Set("token", token)
	return u + "?" + v.Encode()
}

func (s *service) publishEmail(ctx context.Context, eventType, userID string, payload map[string]any) {
	if s.outbox == nil {
		return
	}
	_ = s.outbox.Publish(ctx, events.Event{
		EventID: s.idgen.NewUUID(), AggregateType: "user", AggregateID: userID,
		EventType: eventType, Payload: payload, OccurredAt: time.Now().UTC(),
	})
}

func coalesce(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
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
