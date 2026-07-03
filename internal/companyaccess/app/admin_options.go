package app

import (
	"context"
	"strings"
	"time"

	auditapp "github.com/cobo/cobo_iam_services/internal/audit/app"
	"github.com/cobo/cobo_iam_services/internal/companyaccess/conflict"
	"github.com/cobo/cobo_iam_services/internal/companyaccess/dependency"
)

// AdminOption configures AdminService construction.
type AdminOption func(*adminService)

// WithInvitationMailer wires invitation emails to IAM/outbox (required for InviteUser when email should be sent).
func WithInvitationMailer(m InvitationMailer) AdminOption {
	return func(s *adminService) {
		s.invMailer = m
	}
}

// WithEmailVerificationIssuer wires the email-verification link issuer (IAM/outbox)
// so admin staff-create can create a pending user and dispatch a verify email.
func WithEmailVerificationIssuer(m EmailVerificationIssuer) AdminOption {
	return func(s *adminService) {
		s.emailVerifIssuer = m
	}
}

// WithInvitationTTL sets invitation token lifetime (must match IAM AuthFlowConfig for email copy accuracy).
func WithInvitationTTL(d time.Duration) AdminOption {
	return func(s *adminService) {
		if d > 0 {
			s.inviteTTL = d
		}
	}
}

// WithInviteDefaultRoleCode sets the roles.role_code used when CMS invite omits role_id and role_code (e.g. user_thuong).
func WithInviteDefaultRoleCode(roleCode string) AdminOption {
	return func(s *adminService) {
		s.inviteDefaultRoleCode = strings.TrimSpace(roleCode)
	}
}

// WithSubscriptionTierLookup returns subscription tier for self-service company quota (Free default when nil or empty).
func WithSubscriptionTierLookup(fn func(ctx context.Context, userID string) string) AdminOption {
	return func(s *adminService) {
		s.tierLookup = fn
	}
}

// WithNotificationRulesConsumerEnabled sets runtime_consumer_enabled in notification status API.
func WithNotificationRulesConsumerEnabled(enabled bool) AdminOption {
	return func(s *adminService) {
		s.notificationRulesConsumerEnabled = enabled
	}
}

// WithSubscriptionTierEnforcementEnabled gates server-side tier checks (Batch 5).
func WithSubscriptionTierEnforcementEnabled(enabled bool) AdminOption {
	return func(s *adminService) {
		s.subscriptionTierEnforcementEnabled = enabled
	}
}

// WithConflictSnapshotReader wires read-only conflict snapshot loading (Sprint 4 Batch 1B).
func WithConflictSnapshotReader(r conflict.SnapshotReader) AdminOption {
	return func(s *adminService) {
		s.conflictReader = r
	}
}

// WithCompanyTierLookup resolves company subscription tier for conflict rules (read-only).
func WithCompanyTierLookup(fn func(ctx context.Context, companyID string) string) AdminOption {
	return func(s *adminService) {
		s.companyTierLookup = fn
	}
}

// WithDependencyReader wires read-only dependency viewer queries (Sprint 4 Batch 3).
func WithDependencyReader(r dependency.Reader) AdminOption {
	return func(s *adminService) {
		s.dependencyReader = r
	}
}

// WithDispatchSimulator wires read-only notification dispatch simulation (Batch 3B).
func WithDispatchSimulator(sim NotificationDispatchSimulator) AdminOption {
	return func(s *adminService) {
		s.dispatchSimulator = sim
	}
}

// WithAuditRepository wires read-only audit list for change timeline (Batch 5B).
func WithAuditRepository(repo auditapp.Repository) AdminOption {
	return func(s *adminService) {
		s.auditRepo = repo
	}
}

// WithEffectiveAccessCache wires ADR-025 targeted invalidation on RBAC rollback.
func WithEffectiveAccessCache(cache EffectiveAccessCache) AdminOption {
	return func(s *adminService) {
		s.effectiveAccessCache = cache
	}
}
