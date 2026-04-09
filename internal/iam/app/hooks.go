package app

import "context"

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
