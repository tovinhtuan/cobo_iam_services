package app_test

import (
	"context"
	"strings"
	"testing"

	notificationapp "github.com/cobo/cobo_iam_services/internal/notification/app"
	notificationregistry "github.com/cobo/cobo_iam_services/internal/notification/infra/registry"
)

func TestEmailRenderer_GoldenOutputs(t *testing.T) {
	registry := notificationregistry.NewEmbedRegistry()
	renderer := notificationapp.NewEmailRenderer()

	tests := []struct {
		name     string
		key      string
		vars     map[string]any
		wantSubj string
		wantBody string
	}{
		{
			name:     "email verification",
			key:      "auth.email_verification",
			vars:     map[string]any{"full_name": "Nguyen Van A", "otp_code": "123456", "expiry_minutes": 15},
			wantSubj: "Verify your email",
			wantBody: "Xin chao Nguyen Van A,\n\nMa xac thuc email cua ban la: 123456\nMa het han sau 15 phut.\n\nNeu ban khong yeu cau ma nay, hay bo qua email nay.",
		},
		{
			name:     "user password reset",
			key:      "auth.password_reset.user",
			vars:     map[string]any{"full_name": "Nguyen Van A", "reset_link": "https://app/reset", "expiry_minutes": 30},
			wantSubj: "Reset your password",
			wantBody: "Xin chao Nguyen Van A,\n\nVui long dat lai mat khau qua link sau:\nhttps://app/reset\n\nLink het han sau 30 phut.",
		},
		{
			name:     "admin password reset",
			key:      "auth.password_reset.admin",
			vars:     map[string]any{"full_name": "Nguyen Van A", "reset_link": "https://app/reset", "expiry_minutes": 30},
			wantSubj: "Dat lai mat khau (yeu cau tu quan tri)",
			wantBody: "Xin chao Nguyen Van A,\n\nQuan tri vien da yeu cau dat lai mat khau.\nhttps://app/reset\n\nLink het han sau 30 phut.\n",
		},
		{
			name:     "new user invitation",
			key:      "auth.user_invitation.new_user",
			vars:     map[string]any{"display_name": "Nguyen Van A", "company_name": "COBO", "setup_link": "https://app/invite", "expiry_hours": 72},
			wantSubj: "Thiet lap mat khau tai khoan",
			wantBody: "Xin chao Nguyen Van A,\n\nCong ty: COBO\n\nTai khoan da duoc tao. Thiet lap mat khau qua link sau:\nhttps://app/invite\n\nLink het han sau khoang 72 gio. Neu ban khong yeu cau, bo qua email nay.\n",
		},
		{
			name:     "existing user invitation",
			key:      "auth.user_invitation.existing_user",
			vars:     map[string]any{"display_name": "Nguyen Van A", "company_name": "COBO"},
			wantSubj: "Tham gia cong ty",
			wantBody: "Xin chao Nguyen Van A,\n\nCong ty: COBO\n\nBan da duoc them vao tai khoan cong ty tren he thong. Vui long dang nhap bang email va mat khau hien tai cua ban.\n\nNeu ban khong cho doi thao tac nay, hay lien he quan tri vien.\n",
		},
		{
			name: "reminder disclosure deadline",
			key:  "reminder.disclosure_deadline",
			vars: map[string]any{
				"title":         "Annual disclosure report",
				"deadline_date": "2026-05-10",
				"disclosure_id": "disc-001",
				"status":        "draft",
				"action_url":    "/app/disclosures/disc-001",
			},
			wantSubj: "[COBO] Reminder: Annual disclosure report is due on 2026-05-10",
			wantBody: "Hello,\n\nThis is an automated reminder for a disclosure task.\n\nDisclosure: Annual disclosure report\nDisclosure ID: disc-001\nDeadline: 2026-05-10\nCurrent status: draft\nAction link: /app/disclosures/disc-001\n\nPlease review and complete the required action before the deadline.\n\nCOBO Notification System",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolved, err := registry.Resolve(context.Background(), tt.key, "vi")
			if err != nil {
				t.Fatalf("Resolve() error = %v", err)
			}
			rendered, err := renderer.Render(resolved, tt.vars)
			if err != nil {
				t.Fatalf("Render() error = %v", err)
			}
			if rendered.Subject != tt.wantSubj {
				t.Fatalf("subject mismatch\nwant: %q\ngot:  %q", tt.wantSubj, rendered.Subject)
			}
			if rendered.TextBody != tt.wantBody {
				t.Fatalf("body mismatch\nwant: %q\ngot:  %q", tt.wantBody, rendered.TextBody)
			}
		})
	}
}

func TestEmailRenderer_MissingRequiredVar(t *testing.T) {
	registry := notificationregistry.NewEmbedRegistry()
	renderer := notificationapp.NewEmailRenderer()

	resolved, err := registry.Resolve(context.Background(), "auth.email_verification", "vi")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	_, err = renderer.Render(resolved, map[string]any{"full_name": "Nguyen Van A", "expiry_minutes": 15})
	if err == nil || !strings.Contains(err.Error(), "otp_code") {
		t.Fatalf("expected missing otp_code error, got %v", err)
	}
}
