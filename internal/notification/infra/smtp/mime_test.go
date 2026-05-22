package smtp_test

import (
	"strings"
	"testing"

	"github.com/cobo/cobo_iam_services/internal/notification/infra/smtp"
)

func TestBuildMessage_TextOnly(t *testing.T) {
	raw, msgID := smtp.BuildMessage(
		"no-reply@cobo.local",
		"nguyen@example.com",
		"Verify your email",
		"Xin chao Nguyen Van A,\nMa xac thuc email: 123456",
		"",
	)
	out := string(raw)
	if !strings.HasPrefix(msgID, "<") || !strings.HasSuffix(msgID, "@cobo.local>") {
		t.Fatalf("unexpected Message-ID: %q", msgID)
	}
	for _, want := range []string{
		"From: no-reply@cobo.local\r\n",
		"To: nguyen@example.com\r\n",
		"Content-Type: text/plain; charset=UTF-8\r\n",
		"MIME-Version: 1.0\r\n",
		"Ma xac thuc email: 123456",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("payload missing %q\nfull:\n%s", want, out)
		}
	}
	if strings.Contains(out, "multipart/alternative") {
		t.Fatalf("text-only message must not wrap in multipart, got:\n%s", out)
	}
	// Body must be CRLF on the wire even though renderer emitted LF.
	if !strings.Contains(out, "Xin chao Nguyen Van A,\r\nMa xac thuc email: 123456") {
		t.Fatalf("LF was not normalised to CRLF:\n%s", out)
	}
}

func TestBuildMessage_Multipart(t *testing.T) {
	raw, _ := smtp.BuildMessage(
		"no-reply@cobo.local",
		"nguyen@example.com",
		"Test subject",
		"plain version",
		"<p>html version</p>",
	)
	out := string(raw)
	if !strings.Contains(out, "Content-Type: multipart/alternative; boundary=\"") {
		t.Fatalf("missing multipart header:\n%s", out)
	}
	if !strings.Contains(out, "Content-Type: text/plain; charset=UTF-8") {
		t.Fatalf("missing text part header:\n%s", out)
	}
	if !strings.Contains(out, "Content-Type: text/html; charset=UTF-8") {
		t.Fatalf("missing html part header:\n%s", out)
	}
	if !strings.Contains(out, "plain version") {
		t.Fatalf("missing plain body")
	}
	if !strings.Contains(out, "html version") {
		t.Fatalf("missing html body")
	}
	// Plain part must come BEFORE html so RFC 2046 clients pick html when
	// they can render it.
	plainIdx := strings.Index(out, "Content-Type: text/plain")
	htmlIdx := strings.Index(out, "Content-Type: text/html")
	if !(plainIdx > 0 && htmlIdx > plainIdx) {
		t.Fatalf("plain part must precede html part (plain=%d html=%d)", plainIdx, htmlIdx)
	}
	if !strings.HasSuffix(strings.TrimSpace(out), "--") {
		t.Fatalf("multipart must end with closing boundary, got tail:\n...%s", out[max(0, len(out)-80):])
	}
}

func TestBuildMessage_QEncodesUTF8Subject(t *testing.T) {
	raw, _ := smtp.BuildMessage(
		"no-reply@cobo.local",
		"a@example.com",
		"Đặt lại mật khẩu",
		"body",
		"",
	)
	out := string(raw)
	// Subject must NOT appear in raw UTF-8; it must be Q-encoded so legacy
	// gateways do not corrupt it.
	if strings.Contains(out, "Đặt lại mật khẩu") {
		t.Fatalf("UTF-8 subject must be Q-encoded:\n%s", out)
	}
	if !strings.Contains(out, "Subject: =?UTF-8?") {
		t.Fatalf("missing Q-encoded subject prefix:\n%s", out)
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
