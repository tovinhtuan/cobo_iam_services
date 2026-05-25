package app

import (
	"context"
	"time"
)

// Service defines IAM use-cases used by transport layer.
type Service interface {
	Login(ctx context.Context, req LoginRequest) (*LoginResponse, error)
	// RegisterPublic creates an active user then issues the same session as Login.
	// When company_name is set: also creates company + membership + self-reg owner role (tenant onboarding).
	// When company_name is empty: user only (no company); login follows the no-membership path.
	RegisterPublic(ctx context.Context, req RegisterPublicRequest) (*LoginResponse, error)
	Refresh(ctx context.Context, req RefreshRequest) (*RefreshResponse, error)
	Logout(ctx context.Context, req LogoutRequest) (*LogoutResponse, error)
	SelectCompany(ctx context.Context, req SelectCompanyRequest) (*SelectCompanyResponse, error)
	SwitchCompany(ctx context.Context, req SwitchCompanyRequest) (*SwitchCompanyResponse, error)
	ForgotPassword(ctx context.Context, req ForgotPasswordRequest) (*ForgotPasswordResponse, error)
	ResendVerificationEmail(ctx context.Context, req ResendVerificationEmailRequest) (*ResendVerificationEmailResponse, error)
	ResetPassword(ctx context.Context, req ResetPasswordRequest) (*ResetPasswordResponse, error)
	VerifyEmail(ctx context.Context, req VerifyEmailRequest) (*VerifyEmailResponse, error)
	ListSessions(ctx context.Context, req ListSessionsRequest) (*ListSessionsResponse, error)
	RevokeSession(ctx context.Context, req RevokeSessionRequest) (*RevokeSessionResponse, error)

	ValidateUserInvitation(ctx context.Context, token string) (*ValidateUserInvitationResult, error)
	AcceptUserInvitation(ctx context.Context, req AcceptUserInvitationRequest) (*AcceptUserInvitationResponse, error)
	PublishUserInvitationEmail(ctx context.Context, userID, toEmail, fullName, loginID, rawToken, companyName string) error
	AdminRequestPasswordReset(ctx context.Context, targetUserID string) error
}

// CredentialVerifier verifies login credentials.
type CredentialVerifier interface {
	Verify(ctx context.Context, loginID, plainPassword string) (*AuthenticatedUser, error)
}

// IdentityQueryService returns authenticated identity profile data.
type IdentityQueryService interface {
	GetByUserID(ctx context.Context, userID string) (*AuthenticatedUser, error)
}

// UserInvitationExecutor loads/consumes user_invitations rows (MySQL implementation in iam/infra/mysql).
type UserInvitationExecutor interface {
	PeekUserInvitation(ctx context.Context, rawToken string, now time.Time) (valid bool, reason string, emailHint string, expiresAtUTC time.Time, err error)
	// AcceptUserInvitation consumes the token and returns (userID, loginID) to allow the caller to issue a session.
	AcceptUserInvitation(ctx context.Context, rawToken string, bcryptPasswordHash string, now time.Time) (userID string, loginID string, err error)
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
	ListByUser(ctx context.Context, userID string) ([]SessionState, error)
	RevokeBySessionID(ctx context.Context, userID, sessionID string) error
	RevokeAllByUser(ctx context.Context, userID, reason string) error
}

type AuthRecoveryRepository interface {
	FindUserByEmail(ctx context.Context, email string) (*RecoveryUser, error)
	FindUserByUserID(ctx context.Context, userID string) (*RecoveryUser, error)
	StorePasswordResetToken(ctx context.Context, token RecoveryTokenRecord) error
	ConsumePasswordResetToken(ctx context.Context, tokenHash string, now time.Time) (string, error)
	StoreEmailVerificationToken(ctx context.Context, token RecoveryTokenRecord) error
	ConsumeEmailVerificationToken(ctx context.Context, tokenHash string, now time.Time) (string, error)
	UpdatePasswordHash(ctx context.Context, userID string, passwordHash string, changedAt time.Time) error
	MarkEmailVerified(ctx context.Context, userID string, verifiedAt time.Time) error

	IsEmailVerified(ctx context.Context, userID string) (bool, error)
	InvalidatePendingEmailVerificationOTPs(ctx context.Context, userID string) error
	StoreEmailVerificationOTP(ctx context.Context, otp EmailOTPRecord) error
	CountEmailVerificationOTPsSince(ctx context.Context, userID string, since time.Time) (int, error)
	// TryConsumeEmailVerificationOTP verifies bcrypt(code); increments attempts on mismatch; locks OTP row when exceeded.
	TryConsumeEmailVerificationOTP(ctx context.Context, userID, plainCode string, now time.Time) (EmailOTPConsumeOutcome, error)
}

// EmailOTPRecord stores a bcrypt hash of a numeric OTP for email verification.
type EmailOTPRecord struct {
	OTPID     string
	UserID    string
	CodeHash  string
	ExpiresAt time.Time
}

// EmailOTPConsumeOutcome is the result of submitting an OTP code.
type EmailOTPConsumeOutcome int

const (
	EmailOTPConsumed EmailOTPConsumeOutcome = iota
	EmailOTPWrongCode
	EmailOTPExhausted
	EmailOTPNotFound
)

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
	UserID           string
	LoginID          string
	FullName         string
	Status           string
	SubscriptionTier string
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
	IP           string
	UserAgent    string
}

type RecoveryUser struct {
	UserID   string
	Email    string
	FullName string
	LoginID  string
}

type RecoveryTokenRecord struct {
	TokenID   string
	UserID    string
	TokenHash string
	ExpiresAt time.Time
}

// LoginPasswordCipher carries a password encrypted for transport (browser → API).
// When present, the HTTP layer decrypts into Password before credential verification.
type LoginPasswordCipher struct {
	Alg           string `json:"alg"`
	KID           string `json:"kid,omitempty"`
	CiphertextB64 string `json:"ciphertext_b64"`
}

type RegisterPublicRequest struct {
	Email           string `json:"email"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirm_password"`
	FullName    string `json:"full_name"`
	CompanyName string `json:"company_name,omitempty"` // optional: omit or empty for account without company
	IP              string `json:"-"`
	UserAgent       string `json:"-"`
}

type LoginRequest struct {
	LoginID string `json:"login_id"`
	// Email alias for frontend compatibility (cobo_web_design login form).
	Email    string `json:"email,omitempty"`
	Password string `json:"password,omitempty"`
	// PasswordCipher optional RSA-OAEP-256 ciphertext (base64) when LOGIN_PASSWORD_RSA_PRIVATE_KEY_PEM is configured.
	PasswordCipher *LoginPasswordCipher `json:"password_cipher,omitempty"`
	// RememberMe is optional and can be used by session policy (TTL) in later phases.
	RememberMe bool   `json:"remember_me,omitempty"`
	IP         string `json:"-"`
	UserAgent  string `json:"-"`
	// MFAOTP optional second factor (TOTP etc.); consumed by MFACheck when wired.
	MFAOTP string `json:"mfa_otp,omitempty"`
	// Extensions carries forward-compatible fields (OIDC state, device id, etc.) for SSOLoginBridge / MFACheck.
	Extensions map[string]any `json:"extensions,omitempty"`
}

type LoginResponse struct {
	User               LoginUser                `json:"user"`
	Session            LoginSession             `json:"session"`
	CurrentContext     *LoginCurrentContext     `json:"current_context,omitempty"`
	Memberships        []LoginMembershipSummary `json:"memberships,omitempty"`
	PlatformAccessHint bool                     `json:"platform_access_hint,omitempty"`
	NextAction         string                   `json:"next_action"`
	// EmailVerified false when users.email_verified_at is NULL (snapshot at login).
	// Must not use omitempty: false must serialize so clients can gate verify-email UX.
	EmailVerified bool `json:"email_verified"`
	// CompanyVerificationStatus from companies.verification_status for current context (e.g. unverified for self-reg).
	CompanyVerificationStatus string `json:"company_verification_status,omitempty"`
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

type ForgotPasswordRequest struct {
	Email string `json:"email"`
}

type ForgotPasswordResponse struct {
	Success bool `json:"success"`
}

type ResendVerificationEmailRequest struct {
	Email string `json:"email,omitempty"`
	// UserID set by HTTP handler when Authorization bearer identifies the caller (email optional).
	UserID string `json:"-"`
}

type ResendVerificationEmailResponse struct {
	Success bool `json:"success"`
}

type ResetPasswordRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}

type ResetPasswordResponse struct {
	Success bool `json:"success"`
}

type VerifyEmailRequest struct {
	Token string `json:"token,omitempty"`
	// Code is a numeric OTP (preferred for web); requires UserID from authenticated session.
	Code string `json:"code,omitempty"`
	// UserID set by HTTP handler when verifying via OTP (Bearer access token).
	UserID string `json:"-"`
}

type VerifyEmailResponse struct {
	Success       bool `json:"success"`
	EmailVerified bool `json:"email_verified,omitempty"`
}

type ListSessionsRequest struct {
	UserID           string
	CurrentSessionID string
}

type SessionView struct {
	SessionID           string `json:"session_id"`
	CurrentCompanyID    string `json:"current_company_id,omitempty"`
	CurrentMembershipID string `json:"current_membership_id,omitempty"`
	IP                  string `json:"ip,omitempty"`
	UserAgent           string `json:"user_agent,omitempty"`
	Current             bool   `json:"current"`
	Revoked             bool   `json:"revoked"`
}

type ListSessionsResponse struct {
	Items []SessionView `json:"items"`
}

type RevokeSessionRequest struct {
	UserID    string
	SessionID string
}

type RevokeSessionResponse struct {
	Success bool `json:"success"`
}

type ValidateUserInvitationResult struct {
	Valid     bool   `json:"valid"`
	Reason    string `json:"reason,omitempty"`
	EmailHint string `json:"email_hint,omitempty"`
	ExpiresAt string `json:"expires_at,omitempty"`
}

type AcceptUserInvitationRequest struct {
	Token           string `json:"token"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirm_password,omitempty"`
}

type AcceptUserInvitationResponse struct {
	Success bool `json:"success"`
	// Session is populated when auto-login succeeds after accepting the invitation.
	// FE should store tokens and bootstrap the session instead of redirecting to /login.
	Session    *LoginSession `json:"session,omitempty"`
	NextAction string        `json:"next_action,omitempty"`
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
