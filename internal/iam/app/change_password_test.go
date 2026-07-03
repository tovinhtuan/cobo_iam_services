package app_test

import (
	"context"
	"testing"
	"time"

	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	iaminmem "github.com/cobo/cobo_iam_services/internal/iam/infra/inmemory"
)

type credRecoveryBridge struct {
	cred *iaminmem.StaticCredentialVerifier
}

func (b credRecoveryBridge) FindUserByEmail(context.Context, string) (*iamapp.RecoveryUser, error) {
	return nil, nil
}
func (b credRecoveryBridge) FindUserByUserID(context.Context, string) (*iamapp.RecoveryUser, error) {
	return nil, nil
}
func (b credRecoveryBridge) StorePasswordResetToken(context.Context, iamapp.RecoveryTokenRecord) error {
	return nil
}
func (b credRecoveryBridge) ConsumePasswordResetToken(context.Context, string, time.Time) (string, error) {
	return "", nil
}
func (b credRecoveryBridge) StoreEmailVerificationToken(context.Context, iamapp.RecoveryTokenRecord) error {
	return nil
}
func (b credRecoveryBridge) ConsumeEmailVerificationToken(context.Context, string, time.Time) (string, error) {
	return "", nil
}
func (b credRecoveryBridge) UpdatePasswordHash(ctx context.Context, userID, hash string, at time.Time) error {
	return b.cred.UpdatePasswordHash(ctx, userID, hash, at)
}
func (b credRecoveryBridge) MarkEmailVerified(context.Context, string, time.Time) error { return nil }
func (b credRecoveryBridge) ActivateUserAfterEmailVerification(context.Context, string) error {
	return nil
}
func (b credRecoveryBridge) IsEmailVerified(context.Context, string) (bool, error) { return true, nil }
func (b credRecoveryBridge) InvalidatePendingEmailVerificationOTPs(context.Context, string) error {
	return nil
}
func (b credRecoveryBridge) StoreEmailVerificationOTP(context.Context, iamapp.EmailOTPRecord) error {
	return nil
}
func (b credRecoveryBridge) CountEmailVerificationOTPsSince(context.Context, string, time.Time) (int, error) {
	return 0, nil
}
func (b credRecoveryBridge) TryConsumeEmailVerificationOTP(context.Context, string, string, time.Time) (iamapp.EmailOTPConsumeOutcome, error) {
	return iamapp.EmailOTPNotFound, nil
}

func TestChangeAccountPassword_success(t *testing.T) {
	cred := testCred()
	svc := newTestIAMService(t, testIAMDeps{
		cred: cred,
		opts: []iamapp.ServiceOption{iamapp.WithAuthRecoveryRepository(credRecoveryBridge{cred: cred})},
	})

	_, err := svc.ChangeAccountPassword(context.Background(), iamapp.ChangeAccountPasswordRequest{
		UserID:          "u_123",
		CurrentPassword: "secret",
		NewPassword:     "newsecret1234",
	})
	if err != nil {
		t.Fatalf("ChangeAccountPassword: %v", err)
	}
	if err := cred.VerifyPasswordForUser(context.Background(), "u_123", "newsecret1234"); err != nil {
		t.Fatalf("new password not applied: %v", err)
	}
}

func TestChangeAccountPassword_wrongCurrent(t *testing.T) {
	cred := testCred()
	svc := newTestIAMService(t, testIAMDeps{
		cred: cred,
		opts: []iamapp.ServiceOption{iamapp.WithAuthRecoveryRepository(credRecoveryBridge{cred: cred})},
	})

	_, err := svc.ChangeAccountPassword(context.Background(), iamapp.ChangeAccountPasswordRequest{
		UserID:          "u_123",
		CurrentPassword: "wrong",
		NewPassword:     "newsecret1234",
	})
	if err == nil {
		t.Fatal("expected error for wrong current password")
	}
}
