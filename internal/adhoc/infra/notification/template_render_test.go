package notification_test

import (
	"context"
	"strings"
	"testing"

	notifapp "github.com/cobo/cobo_iam_services/internal/notification/app"
	notificationregistry "github.com/cobo/cobo_iam_services/internal/notification/infra/registry"
)

// adhocTemplateKeys lists the four user-facing adhoc email templates.
var adhocTemplateKeys = []string{
	"adhoc.controller_review_requested",
	"adhoc.focal_review_requested",
	"adhoc.proposal_approved",
	"adhoc.proposal_rejected",
}

// baseVars provides required fields present in all 4 adhoc templates.
func baseVars() map[string]any {
	return map[string]any{
		"proposal_id":    "prop-render-test",
		"proposal_title": "Báo cáo sự cố hệ thống test",
		"company_name":   "Công ty ABC",
		"portal_url":     "https://portal.cobo.vn",
	}
}

func renderAdhocTemplate(t *testing.T, key string, vars map[string]any) notifapp.RenderedEmail {
	t.Helper()
	registry := notificationregistry.NewEmbedRegistry()
	renderer := notifapp.NewEmailRenderer()
	resolved, err := registry.Resolve(context.Background(), key, "vi")
	if err != nil {
		t.Fatalf("resolve %s: %v", key, err)
	}
	rendered, err := renderer.Render(resolved, vars)
	if err != nil {
		t.Fatalf("render %s: %v", key, err)
	}
	return rendered
}

// TestAdhocTemplate_TitleLabelPresent asserts that all 4 templates render the
// label "Tiêu đề" in the email body.
func TestAdhocTemplate_TitleLabelPresent(t *testing.T) {
	for _, key := range adhocTemplateKeys {
		t.Run(key, func(t *testing.T) {
			vars := baseVars()
			vars["creator_name"] = "Nguyễn Văn A"
			vars["record_id"] = "rec-001"
			vars["reject_reason"] = "Không hợp lệ"
			rendered := renderAdhocTemplate(t, key, vars)
			if !strings.Contains(rendered.TextBody, "Tiêu đề") {
				t.Errorf("body does not contain label 'Tiêu đề':\n%s", rendered.TextBody)
			}
		})
	}
}

// TestAdhocTemplate_ContentLabelPresentWhenPopulated asserts "Nội dung" label
// appears when proposal_content is provided.
func TestAdhocTemplate_ContentLabelPresentWhenPopulated(t *testing.T) {
	for _, key := range adhocTemplateKeys {
		t.Run(key, func(t *testing.T) {
			vars := baseVars()
			vars["creator_name"] = "Nguyễn Văn A"
			vars["record_id"] = "rec-001"
			vars["reject_reason"] = "Không hợp lệ"
			vars["proposal_content"] = "Báo cáo sự cố hệ thống test content"
			rendered := renderAdhocTemplate(t, key, vars)
			if !strings.Contains(rendered.TextBody, "Nội dung") {
				t.Errorf("body does not contain label 'Nội dung' when proposal_content present:\n%s", rendered.TextBody)
			}
			if !strings.Contains(rendered.TextBody, "Báo cáo sự cố hệ thống test content") {
				t.Errorf("body does not contain proposal_content value:\n%s", rendered.TextBody)
			}
		})
	}
}

// TestAdhocTemplate_ContentLabelAbsentWhenEmpty asserts "Nội dung" is not
// rendered when proposal_content is absent (single-line change_note case).
func TestAdhocTemplate_ContentLabelAbsentWhenEmpty(t *testing.T) {
	for _, key := range adhocTemplateKeys {
		t.Run(key, func(t *testing.T) {
			vars := baseVars()
			vars["creator_name"] = "Nguyễn Văn A"
			vars["record_id"] = "rec-001"
			vars["reject_reason"] = "Không hợp lệ"
			// no proposal_content key
			rendered := renderAdhocTemplate(t, key, vars)
			if strings.Contains(rendered.TextBody, "Nội dung") {
				t.Errorf("body contains 'Nội dung' label when proposal_content is absent — should be hidden:\n%s", rendered.TextBody)
			}
		})
	}
}

// TestAdhocTemplate_RegressionSample verifies the exact before/after scenario
// from the UX bug report: title and content must appear on separate labeled
// lines, never concatenated.
func TestAdhocTemplate_RegressionSample(t *testing.T) {
	title := "Báo cáo sự cố hệ thống test"
	content := "Báo cáo sự cố hệ thống test content"
	badConcat := title + " " + content

	for _, key := range adhocTemplateKeys {
		t.Run(key, func(t *testing.T) {
			vars := baseVars()
			vars["creator_name"] = "Nguyễn Văn A"
			vars["record_id"] = "rec-001"
			vars["reject_reason"] = "Không hợp lệ"
			vars["proposal_title"] = title
			vars["proposal_content"] = content
			rendered := renderAdhocTemplate(t, key, vars)
			if strings.Contains(rendered.TextBody, badConcat) {
				t.Errorf("body contains concatenated title+content %q — separation fix not applied:\n%s", badConcat, rendered.TextBody)
			}
			if !strings.Contains(rendered.TextBody, "Tiêu đề: "+title) {
				t.Errorf("body missing 'Tiêu đề: %s':\n%s", title, rendered.TextBody)
			}
			if !strings.Contains(rendered.TextBody, "Nội dung: "+content) {
				t.Errorf("body missing 'Nội dung: %s':\n%s", content, rendered.TextBody)
			}
		})
	}
}

// TestAdhocTemplate_CTAURLAbsolute verifies that the CTA URL in body text is
// absolute (starts with https:// or http://).
func TestAdhocTemplate_CTAURLAbsolute(t *testing.T) {
	for _, key := range adhocTemplateKeys {
		t.Run(key, func(t *testing.T) {
			vars := baseVars()
			vars["creator_name"] = "Nguyễn Văn A"
			vars["record_id"] = "rec-001"
			vars["reject_reason"] = "Không hợp lệ"
			rendered := renderAdhocTemplate(t, key, vars)
			if !strings.Contains(rendered.TextBody, "https://portal.cobo.vn") {
				t.Errorf("CTA URL is not absolute in body:\n%s", rendered.TextBody)
			}
		})
	}
}

// TestAdhocTemplate_SubjectUsesProposalTitle verifies that the email subject
// contains the proposal_title, not the raw change_note or a UUID.
func TestAdhocTemplate_SubjectUsesProposalTitle(t *testing.T) {
	title := "Báo cáo sự cố hệ thống test"
	for _, key := range adhocTemplateKeys {
		t.Run(key, func(t *testing.T) {
			vars := baseVars()
			vars["creator_name"] = "Nguyễn Văn A"
			vars["record_id"] = "rec-001"
			vars["reject_reason"] = "Không hợp lệ"
			vars["proposal_title"] = title
			rendered := renderAdhocTemplate(t, key, vars)
			if !strings.Contains(rendered.Subject, title) {
				t.Errorf("subject does not contain proposal_title %q:\n%s", title, rendered.Subject)
			}
		})
	}
}

// ---- Batch D: Email Regression Gates (E-CLONE, E-AUTOFILL, E-INLINE, E-REGRESSION) ----

var emailTechnicalLeakPatterns = []string{
	"membership_id",
	"workflow_instance_id",
	"type_id",
	"process_controller_id",
	"step_id",
	"localhost",
	"prop-test-001",
	"rec-test-001",
}

// emailVisibleText strips CTA URL lines — proposal_id/record_id may appear only in href per V2.1.
func emailVisibleText(body string) string {
	lines := strings.Split(body, "\n")
	var visible []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
			continue
		}
		if strings.Contains(trimmed, "/app/ad-hoc-proposals/") || strings.Contains(trimmed, "/app/disclosures/") {
			continue
		}
		visible = append(visible, line)
	}
	return strings.Join(visible, "\n")
}

func enrichAdhocTemplateVars(vars map[string]any) map[string]any {
	out := make(map[string]any, len(vars)+4)
	for k, v := range vars {
		out[k] = v
	}
	if _, ok := out["creator_name"]; !ok {
		out["creator_name"] = "Nguyễn Văn A"
	}
	if _, ok := out["record_id"]; !ok {
		out["record_id"] = "rec-email-gate"
	}
	if _, ok := out["reject_reason"]; !ok {
		out["reject_reason"] = "Không hợp lệ"
	}
	return out
}

func assertNoTechnicalLeakInEmail(t *testing.T, label, subject, textBody, htmlBody string) {
	t.Helper()
	visible := subject + "\n" + emailVisibleText(textBody) + "\n" + emailVisibleText(htmlBody)
	for _, pat := range emailTechnicalLeakPatterns {
		if strings.Contains(visible, pat) {
			t.Errorf("%s: visible email text contains forbidden pattern %q", label, pat)
		}
	}
	if strings.Contains(visible, "9f4b2c1a-") || strings.Contains(visible, "deadbeef-") {
		t.Errorf("%s: visible email text contains UUID-like technical value", label)
	}
	if strings.Contains(visible, "prop-render-test") {
		t.Errorf("%s: proposal_id leaked into visible email body (not CTA href)", label)
	}
}

func TestEmailRegression_EClone(t *testing.T) {
	vars := enrichAdhocTemplateVars(baseVars())
	vars["proposal_title"] = "Báo cáo sự cố (bản sao)"
	vars["proposal_content"] = "Nội dung được sao chép từ đề xuất đã duyệt trước đó."
	vars["creator_name"] = "Trần Văn A"
	for _, key := range adhocTemplateKeys {
		t.Run(key, func(t *testing.T) {
			rendered := renderAdhocTemplate(t, key, vars)
			assertNoTechnicalLeakInEmail(t, key, rendered.Subject, rendered.TextBody, rendered.HTMLBody)
			if !strings.Contains(rendered.TextBody, "Tiêu đề") {
				t.Error("body missing label Tiêu đề")
			}
			if !strings.Contains(rendered.TextBody, vars["proposal_title"].(string)) {
				t.Error("body missing proposal_title")
			}
		})
	}
}

func TestEmailRegression_EAutofill(t *testing.T) {
	longContent := strings.Repeat("Mục báo cáo tự động điền. ", 20)
	vars := enrichAdhocTemplateVars(baseVars())
	vars["proposal_title"] = "Báo cáo tự động điền mô tả"
	vars["proposal_content"] = longContent
	vars["creator_name"] = "Lê Thị B"
	for _, key := range adhocTemplateKeys {
		t.Run(key, func(t *testing.T) {
			rendered := renderAdhocTemplate(t, key, vars)
			assertNoTechnicalLeakInEmail(t, key, rendered.Subject, rendered.TextBody, rendered.HTMLBody)
			if !strings.Contains(rendered.TextBody, "Báo cáo tự động điền mô tả") {
				t.Error("body missing auto-filled title")
			}
		})
	}
}

func TestEmailRegression_EInline(t *testing.T) {
	vars := enrichAdhocTemplateVars(baseVars())
	vars["proposal_title"] = "Đề xuất loại CBTT mới"
	vars["proposal_content"] = "Loại công bố mới được tạo trực tiếp khi lập đề xuất."
	vars["creator_name"] = "Phạm Văn C"
	for _, key := range adhocTemplateKeys {
		t.Run(key, func(t *testing.T) {
			rendered := renderAdhocTemplate(t, key, vars)
			assertNoTechnicalLeakInEmail(t, key, rendered.Subject, rendered.TextBody, rendered.HTMLBody)
		})
	}
}

func TestEmailRegression_ERegression(t *testing.T) {
	scenarios := []struct {
		name string
		vars map[string]any
	}{
		{
			name: "approved_with_record",
			vars: enrichAdhocTemplateVars(map[string]any{
				"proposal_id":       "prop-regression-approved",
				"proposal_title":    "Báo cáo quý I",
				"proposal_content":  "Nội dung chi tiết báo cáo.",
				"company_name":      "Công ty ABC",
				"portal_url":        "https://portal.cobo.vn",
				"creator_name":      "Nguyễn Văn D",
				"record_id":         "rec-internal-only",
			}),
		},
		{
			name: "rejected_with_reason",
			vars: enrichAdhocTemplateVars(map[string]any{
				"proposal_id":    "prop-regression-rejected",
				"proposal_title": "Đề xuất bị từ chối",
				"company_name":   "Công ty ABC",
				"portal_url":     "https://portal.cobo.vn",
				"creator_name":   "Hoàng Thị E",
				"reject_reason":  "Thiếu tài liệu đính kèm",
			}),
		},
	}
	for _, sc := range scenarios {
		for _, key := range adhocTemplateKeys {
			t.Run(sc.name+"/"+key, func(t *testing.T) {
				rendered := renderAdhocTemplate(t, key, enrichAdhocTemplateVars(sc.vars))
				assertNoTechnicalLeakInEmail(t, key, rendered.Subject, rendered.TextBody, rendered.HTMLBody)
				visible := emailVisibleText(rendered.TextBody)
				if strings.Contains(visible, "rec-internal-only") {
					t.Error("record_id leaked into visible email body (not CTA href)")
				}
			})
		}
	}
}
