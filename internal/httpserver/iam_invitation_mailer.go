package httpserver

import (
	"context"

	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
)

type iamInvitationMailer struct {
	iam iamapp.Service
}

func (m *iamInvitationMailer) SendInvitationEmail(ctx context.Context, p caapp.InvitationMailPayload) error {
	if m == nil || m.iam == nil {
		return nil
	}
	return m.iam.PublishUserInvitationEmail(ctx, p.UserID, p.ToEmail, p.FullName, p.LoginID, p.RawToken)
}
