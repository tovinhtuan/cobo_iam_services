package app_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	cainmem "github.com/cobo/cobo_iam_services/internal/companyaccess/infra/inmemory"
	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	notificationapp "github.com/cobo/cobo_iam_services/internal/notification/app"
	notificationregistry "github.com/cobo/cobo_iam_services/internal/notification/infra/registry"
)

// Phase 0 baseline: every email entry point that uses the embed template path
// must (a) render without error, (b) publish to the outbox, (c) emit the
// canonical subject byte-for-byte, and (d) keep the template-specific
// boilerplate so the wiring (key + variable schema) cannot silently drift.
//
// Renderer-level byte-exact body matching is locked by
// internal/notification/app/email_renderer_test.go (TestEmailRenderer_GoldenOutputs).
// The OTP and reset flows include random values inside the body, so this test
// asserts subject + structural markers rather than full-body equality.
func TestEmailService_EmbedFlows_BaselineSubjectsAndStructure(t *testing.T) {
	ctx := context.Background()
	goldenDir := filepath.Join("..", "..", "notification", "app", "testdata", "email-golden")

	type flow struct {
		name              string
		fixture           string // matches files under testdata/email-golden/
		wantEventType     string
		wantBodyContains  []string
		wantBodyExcludes  []string
		trigger           func(t *testing.T, svc iamapp.Service, publisher *captureOutboxPublisher)
	}

	flows := []flow{
		{
			name:          "OTP email verification",
			fixture:       "auth.email_verification",
			wantEventType: "auth.email_verification_requested",
			wantBodyContains: []string{
				"Xin chào Quý khách,",
				"đăng ký tài khoản trên CoBo Portal",
				"Mã xác thực OTP của Quý khách là:",
				"hiệu lực trong",
				"phút.",
				"Đội ngũ CoBo Portal",
				"Email hỗ trợ:",
				"Website:",
			},
			trigger: func(t *testing.T, svc iamapp.Service, _ *captureOutboxPublisher) {
				t.Helper()
				if _, err := svc.ResendVerificationEmail(ctx, iamapp.ResendVerificationEmailRequest{UserID: "u_otp"}); err != nil {
					t.Fatalf("ResendVerificationEmail error = %v", err)
				}
			},
		},
		{
			name:          "user password reset",
			fixture:       "auth.password_reset.user",
			wantEventType: "auth.password_reset_requested",
			wantBodyContains: []string{
				"Xin chao Nguyen Van A,",
				"Vui long dat lai mat khau qua link sau:",
				"https://app.example.com/reset-password?token=",
				"Link het han sau",
				"phut.",
			},
			trigger: func(t *testing.T, svc iamapp.Service, _ *captureOutboxPublisher) {
				t.Helper()
				if _, err := svc.ForgotPassword(ctx, iamapp.ForgotPasswordRequest{Email: "nguyen@example.com"}); err != nil {
					t.Fatalf("ForgotPassword error = %v", err)
				}
			},
		},
		{
			name:          "admin password reset",
			fixture:       "auth.password_reset.admin",
			wantEventType: "auth.admin_password_reset_requested",
			wantBodyContains: []string{
				"Xin chao Nguyen Van A,",
				"Quan tri vien da yeu cau dat lai mat khau.",
				"https://app.example.com/reset-password?token=",
				"Link het han sau",
				"phut.",
			},
			// Admin reset body must NOT contain the user-self-service copy.
			wantBodyExcludes: []string{
				"Vui long dat lai mat khau qua link sau:",
			},
			trigger: func(t *testing.T, svc iamapp.Service, _ *captureOutboxPublisher) {
				t.Helper()
				if err := svc.AdminRequestPasswordReset(ctx, "u_admin_target"); err != nil {
					t.Fatalf("AdminRequestPasswordReset error = %v", err)
				}
			},
		},
	}

	for _, f := range flows {
		t.Run(f.name, func(t *testing.T) {
			publisher := &captureOutboxPublisher{}
			recovery := &stubRecoveryForBaseline{
				user: &iamapp.RecoveryUser{
					UserID:   "u_target",
					Email:    "nguyen@example.com",
					FullName: "Nguyen Van A",
					LoginID:  "nguyen@example.com",
				},
			}
			svc := newTestIAMService(t, testIAMDeps{
				cred:    testCred(),
				members: cainmem.NewMembershipQueryService(),
				opts: []iamapp.ServiceOption{
					iamapp.WithOutboxPublisher(publisher),
					iamapp.WithAuthRecoveryRepository(recovery),
					iamapp.WithAuthFlowConfig(iamapp.AuthFlowConfig{
						WebBaseURL:              "https://app.example.com",
						SupportEmail:            "support@cobo.vn",
						PasswordResetTokenTTL:   30 * time.Minute,
						EmailVerificationOTPTTL: 15 * time.Minute,
						EmailTemplateSource:     "embed",
						EmailTemplateRegistry:   notificationregistry.NewEmbedRegistry(),
						EmailRenderer:           notificationapp.NewEmailRenderer(),
					}),
				},
			})

			f.trigger(t, svc, publisher)

			if publisher.last.EventType != f.wantEventType {
				t.Fatalf("event_type = %q, want %q", publisher.last.EventType, f.wantEventType)
			}
			wantSubject, _ := loadGoldenForBaseline(t, goldenDir, f.fixture)
			gotSubject, _ := publisher.last.Payload["subject"].(string)
			if gotSubject != wantSubject {
				t.Fatalf("subject mismatch\nwant: %q\ngot:  %q", wantSubject, gotSubject)
			}
			gotBody, _ := publisher.last.Payload["body"].(string)
			for _, snippet := range f.wantBodyContains {
				if !strings.Contains(gotBody, snippet) {
					t.Fatalf("body missing %q\nfull body: %q", snippet, gotBody)
				}
			}
			for _, snippet := range f.wantBodyExcludes {
				if strings.Contains(gotBody, snippet) {
					t.Fatalf("body must not contain %q\nfull body: %q", snippet, gotBody)
				}
			}
			// Render must never leak Go template artefacts past the renderer.
			if strings.Contains(gotBody, "<no value>") || strings.Contains(gotBody, "{{") {
				t.Fatalf("body contains unrendered template artefact: %q", gotBody)
			}
		})
	}
}

// loadGoldenForBaseline is a local helper (the renderer-package LoadGolden is in
// notification/app and not exported across modules) that mirrors its CRLF
// normalisation so Windows checkouts behave the same as Linux.
func loadGoldenForBaseline(t *testing.T, goldenDir, fixture string) (string, string) {
	t.Helper()
	subj, err := os.ReadFile(filepath.Join(goldenDir, fixture+".subject.txt"))
	if err != nil {
		t.Fatalf("read golden subject %s: %v", fixture, err)
	}
	body, err := os.ReadFile(filepath.Join(goldenDir, fixture+".body.txt"))
	if err != nil {
		t.Fatalf("read golden body %s: %v", fixture, err)
	}
	return strings.ReplaceAll(string(subj), "\r\n", "\n"), strings.ReplaceAll(string(body), "\r\n", "\n")
}

// stubRecoveryForBaseline lets the baseline test exercise OTP and password
// reset paths without a real MySQL backing store. Every method either returns
// the seeded user or accepts the write silently; no real persistence.
type stubRecoveryForBaseline struct {
	user *iamapp.RecoveryUser
}

func (s *stubRecoveryForBaseline) FindUserByEmail(_ context.Context, _ string) (*iamapp.RecoveryUser, error) {
	return s.user, nil
}

func (s *stubRecoveryForBaseline) FindUserByUserID(_ context.Context, _ string) (*iamapp.RecoveryUser, error) {
	return s.user, nil
}

func (s *stubRecoveryForBaseline) StorePasswordResetToken(_ context.Context, _ iamapp.RecoveryTokenRecord) error {
	return nil
}

func (s *stubRecoveryForBaseline) ConsumePasswordResetToken(_ context.Context, _ string, _ time.Time) (string, error) {
	return "", nil
}

func (s *stubRecoveryForBaseline) StoreEmailVerificationToken(_ context.Context, _ iamapp.RecoveryTokenRecord) error {
	return nil
}

func (s *stubRecoveryForBaseline) ConsumeEmailVerificationToken(_ context.Context, _ string, _ time.Time) (string, error) {
	return "", nil
}

func (s *stubRecoveryForBaseline) UpdatePasswordHash(_ context.Context, _ string, _ string, _ time.Time) error {
	return nil
}

func (s *stubRecoveryForBaseline) MarkEmailVerified(_ context.Context, _ string, _ time.Time) error {
	return nil
}

func (s *stubRecoveryForBaseline) IsEmailVerified(_ context.Context, _ string) (bool, error) {
	return false, nil
}

func (s *stubRecoveryForBaseline) InvalidatePendingEmailVerificationOTPs(_ context.Context, _ string) error {
	return nil
}

func (s *stubRecoveryForBaseline) StoreEmailVerificationOTP(_ context.Context, _ iamapp.EmailOTPRecord) error {
	return nil
}

func (s *stubRecoveryForBaseline) CountEmailVerificationOTPsSince(_ context.Context, _ string, _ time.Time) (int, error) {
	return 0, nil
}

func (s *stubRecoveryForBaseline) TryConsumeEmailVerificationOTP(_ context.Context, _ string, _ string, _ time.Time) (iamapp.EmailOTPConsumeOutcome, error) {
	return iamapp.EmailOTPConsumed, nil
}

