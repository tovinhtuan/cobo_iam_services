package app

import "context"

// Service defines IAM use-cases used by transport layer.
type Service interface {
	Login(ctx context.Context, req LoginRequest) (*LoginResponse, error)
	Refresh(ctx context.Context, req RefreshRequest) (*RefreshResponse, error)
	Logout(ctx context.Context, req LogoutRequest) (*LogoutResponse, error)
	SelectCompany(ctx context.Context, req SelectCompanyRequest) (*SelectCompanyResponse, error)
	SwitchCompany(ctx context.Context, req SwitchCompanyRequest) (*SwitchCompanyResponse, error)
}

// CredentialVerifier verifies login credentials.
type CredentialVerifier interface {
	Verify(ctx context.Context, loginID, plainPassword string) (*AuthenticatedUser, error)
}

// IdentityQueryService returns authenticated identity profile data.
type IdentityQueryService interface {
	GetByUserID(ctx context.Context, userID string) (*AuthenticatedUser, error)
}

// LoginAttemptRecorder persists login success/failure for audit and rate-limit groundwork.
type LoginAttemptRecorder interface {
	Record(ctx context.Context, rec LoginAttemptRecord) error
}

// LoginAttemptRecord maps to table login_attempts (0001).
type LoginAttemptRecord struct {
	LoginID     string
	UserID      string
	Success     bool
	FailureCode string
	IP          string
	UserAgent   string
}

// SessionRepository persists and rotates sessions/refresh tokens.
type SessionRepository interface {
	Create(ctx context.Context, p CreateSessionParams) error
	FindByRefreshToken(ctx context.Context, refreshToken string) (*SessionState, error)
	RevokeByRefreshToken(ctx context.Context, refreshToken string) error
	UpdateContext(ctx context.Context, sessionID, membershipID, companyID string) error
	RotateRefreshToken(ctx context.Context, sessionID, newRefreshToken string) error
}

type TokenIssuer interface {
	IssueAccessToken(ctx context.Context, claims AccessTokenClaims) (token string, expiresInSec int64, err error)
	IssuePreCompanyToken(ctx context.Context, userID, sessionID string) (token string, expiresInSec int64, err error)
	IssueRefreshToken(ctx context.Context, sessionID, userID string) (token string, err error)
}

// TokenInspector validates opaque bearer tokens and extracts principal context.
type TokenInspector interface {
	InspectAccessToken(ctx context.Context, token string) (*AccessTokenClaims, error)
	InspectPreCompanyToken(ctx context.Context, token string) (*PreCompanyTokenClaims, error)
}

type AuthenticatedUser struct {
	UserID   string
	LoginID  string
	FullName string
	Status   string
}

type AccessTokenClaims struct {
	Sub          string
	SessionID    string
	MembershipID string
	CompanyID    string
}

type PreCompanyTokenClaims struct {
	Sub       string
	SessionID string
}

type CreateSessionParams struct {
	SessionID    string
	UserID       string
	MembershipID string
	CompanyID    string
	RefreshToken string
	IP           string
	UserAgent    string
}

type SessionState struct {
	SessionID    string
	UserID       string
	MembershipID string
	CompanyID    string
	RefreshToken string
	Revoked      bool
}

type LoginRequest struct {
	LoginID   string `json:"login_id"`
	Password  string `json:"password"`
	IP        string `json:"-"`
	UserAgent string `json:"-"`
	// MFAOTP optional second factor (TOTP etc.); consumed by MFACheck when wired.
	MFAOTP string `json:"mfa_otp,omitempty"`
	// Extensions carries forward-compatible fields (OIDC state, device id, etc.) for SSOLoginBridge / MFACheck.
	Extensions map[string]any `json:"extensions,omitempty"`
}

type LoginResponse struct {
	User           LoginUser                `json:"user"`
	Session        LoginSession             `json:"session"`
	CurrentContext *LoginCurrentContext     `json:"current_context,omitempty"`
	Memberships    []LoginMembershipSummary `json:"memberships,omitempty"`
	NextAction     string                   `json:"next_action"`
}

type LoginUser struct {
	UserID   string `json:"user_id"`
	FullName string `json:"full_name"`
}

type LoginSession struct {
	AccessToken     string `json:"access_token,omitempty"`
	PreCompanyToken string `json:"pre_company_token,omitempty"`
	RefreshToken    string `json:"refresh_token"`
	ExpiresIn       int64  `json:"expires_in"`
}

type LoginCurrentContext struct {
	CompanyID    string `json:"company_id"`
	MembershipID string `json:"membership_id"`
	AutoSelected bool   `json:"auto_selected"`
}

type LoginMembershipSummary struct {
	CompanyID    string `json:"company_id"`
	CompanyName  string `json:"company_name"`
	MembershipID string `json:"membership_id"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type RefreshResponse struct {
	AccessToken    string       `json:"access_token"`
	RefreshToken   string       `json:"refresh_token"`
	ExpiresIn      int64        `json:"expires_in"`
	CurrentContext TokenContext `json:"current_context"`
}

type LogoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type LogoutResponse struct {
	Success bool `json:"success"`
}

type SelectCompanyRequest struct {
	UserID    string `json:"-"`
	CompanyID string `json:"company_id"`
	SessionID string `json:"-"`
}

type SelectCompanyResponse struct {
	AccessToken    string       `json:"access_token"`
	ExpiresIn      int64        `json:"expires_in"`
	CurrentContext TokenContext `json:"current_context"`
}

type SwitchCompanyRequest struct {
	UserID    string `json:"-"`
	CompanyID string `json:"company_id"`
	SessionID string `json:"-"`
}

type SwitchCompanyResponse struct {
	AccessToken    string       `json:"access_token"`
	ExpiresIn      int64        `json:"expires_in"`
	CurrentContext TokenContext `json:"current_context"`
}

type TokenContext struct {
	CompanyID    string `json:"company_id"`
	MembershipID string `json:"membership_id"`
}
