package app

import "time"

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
