package httpserver

import (
	"context"

	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
)

// iamEmailVerificationIssuer adapts iam.Service to the companyaccess
// EmailVerificationIssuer port so admin staff-create can dispatch a verify-link
// email (pending user must verify before the account is activated).
type iamEmailVerificationIssuer struct {
	iam iamapp.Service
}

func (m *iamEmailVerificationIssuer) IssueEmailVerificationLink(ctx context.Context, userID string) error {
	if m == nil || m.iam == nil {
		return nil
	}
	return m.iam.IssueEmailVerificationLink(ctx, userID)
}
