package app

import (
	"context"
	"database/sql"
	"time"

	notificationapp "github.com/cobo/cobo_iam_services/internal/notification/app"
	"github.com/cobo/cobo_iam_services/internal/platform/outbox"
)

// ServiceOption configures iam.service (P2.3 extension points). Nil hooks are ignored.
type ServiceOption func(*service)

// WithMFACheck registers a post–primary-auth MFA step. Runs after active user check and before listing memberships.
func WithMFACheck(m MFACheck) ServiceOption {
	return func(s *service) {
		s.mfa = m
	}
}

// WithSSOLoginBridge registers an optional SSO / external IdP primary auth path before password verification.
func WithSSOLoginBridge(b SSOLoginBridge) ServiceOption {
	return func(s *service) {
		s.sso = b
	}
}

// WithLoginAttemptRecorder records each login outcome (e.g. MySQL login_attempts). Nil ignored.
func WithLoginAttemptRecorder(r LoginAttemptRecorder) ServiceOption {
	return func(s *service) {
		s.attempts = r
	}
}

// WithAuthRecoveryRepository enables forgot/reset/verify flows with persisted tokens and password/email updates.
func WithAuthRecoveryRepository(r AuthRecoveryRepository) ServiceOption {
	return func(s *service) {
		s.recovery = r
	}
}

// WithOutboxPublisher enables asynchronous email dispatch via outbox worker.
func WithOutboxPublisher(p outbox.Publisher) ServiceOption {
	return func(s *service) {
		s.outbox = p
	}
}

type AuthFlowConfig struct {
	WebBaseURL                string
	PasswordResetTokenTTL     time.Duration
	EmailVerificationTokenTTL time.Duration
	// EmailVerificationOTPTTL expiry for numeric email OTP (register/resend).
	EmailVerificationOTPTTL time.Duration
	UserInvitationTokenTTL  time.Duration
	EmailTemplateSource     string
	EmailTemplateRegistry   notificationapp.TemplateRegistry
	EmailRenderer           notificationapp.EmailRenderer
}

// WithAuthFlowConfig overrides token TTLs and link base URL for email actions.
func WithAuthFlowConfig(cfg AuthFlowConfig) ServiceOption {
	return func(s *service) {
		if cfg.WebBaseURL != "" {
			s.webBaseURL = cfg.WebBaseURL
		}
		if cfg.PasswordResetTokenTTL > 0 {
			s.passwordResetTTL = cfg.PasswordResetTokenTTL
		}
		if cfg.EmailVerificationTokenTTL > 0 {
			s.emailVerifyTTL = cfg.EmailVerificationTokenTTL
		}
		if cfg.EmailVerificationOTPTTL > 0 {
			s.emailOTPTTL = cfg.EmailVerificationOTPTTL
		}
		if cfg.UserInvitationTokenTTL > 0 {
			s.invitationTTL = cfg.UserInvitationTokenTTL
		}
		if cfg.EmailTemplateSource != "" {
			s.emailTemplateSource = cfg.EmailTemplateSource
		}
		if cfg.EmailTemplateRegistry != nil {
			s.emailTemplateRegistry = cfg.EmailTemplateRegistry
		}
		if cfg.EmailRenderer != nil {
			s.emailRenderer = cfg.EmailRenderer
		}
	}
}

// WithUserInvitationExecutor enables invitation peek/accept against persistent storage.
func WithUserInvitationExecutor(e UserInvitationExecutor) ServiceOption {
	return func(s *service) {
		s.invite = e
	}
}

// WithPublicRegistration enables POST /api/v1/auth/register (self-service company bootstrap).
func WithPublicRegistration(db *sql.DB) ServiceOption {
	return func(s *service) {
		s.regDB = db
	}
}

// WithRegistrationDisabled disables public registration when true (e.g. REGISTRATION_DISABLED=true).
func WithRegistrationDisabled(disabled bool) ServiceOption {
	return func(s *service) {
		s.registrationDisabled = disabled
	}
}

// MFACheck verifies second factor (TOTP, WebAuthn callback, etc.) before membership enumeration and session issuance.
// Return nil to allow login to continue; return *perr.HTTPError (or wrapped) to block with a stable code.
type MFACheck interface {
	VerifyAfterPrimaryAuth(ctx context.Context, user *AuthenticatedUser, req LoginRequest) error
}

// SSOLoginBridge attempts external primary authentication for this login request.
// If handled is true, user must be non-nil and password verification is skipped.
// If handled is false and err is nil, normal CredentialVerifier runs.
// If err is non-nil, login fails (e.g. invalid IdP assertion).
type SSOLoginBridge interface {
	TryExternalPrimaryAuth(ctx context.Context, req LoginRequest) (user *AuthenticatedUser, handled bool, err error)
}
