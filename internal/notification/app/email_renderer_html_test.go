package app

import (
	"strings"
	"testing"
)

// Tenant-typed values (company name, disclosure title…) must not inject markup into HTML emails.
func TestEmailRenderer_HTMLBodyEscapesVariables(t *testing.T) {
	out, err := NewEmailRenderer().Render(ResolvedTemplate{
		Subject:  "[CoBo] {{.company_name}}",
		TextBody: "Công ty: {{.company_name}}",
		HTMLBody: `<td>{{.company_name}}</td><a href="{{.portal_url}}">Mở</a>`,
	}, map[string]any{
		"company_name": `X</td><a href="https://evil.example">Đăng nhập lại</a>`,
		"portal_url":   "https://portal.cobo.vn",
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.HTMLBody, `<a href="https://evil.example">`) || !strings.Contains(out.HTMLBody, "&lt;/td&gt;") {
		t.Fatalf("HTML body not escaped: %s", out.HTMLBody)
	}
	if !strings.Contains(out.HTMLBody, `href="https://portal.cobo.vn"`) {
		t.Fatalf("legit URL altered: %s", out.HTMLBody)
	}
	if !strings.Contains(out.TextBody, `X</td><a href="https://evil.example">`) {
		t.Fatalf("text body should keep the raw value (plain text): %s", out.TextBody)
	}
}
