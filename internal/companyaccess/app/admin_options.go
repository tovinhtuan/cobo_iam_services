package app

import (
	"context"
	"strings"
	"time"
)

// AdminOption configures AdminService construction.
type AdminOption func(*adminService)

// WithInvitationMailer wires invitation emails to IAM/outbox (required for InviteUser when email should be sent).
func WithInvitationMailer(m InvitationMailer) AdminOption {
	return func(s *adminService) {
		s.invMailer = m
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
