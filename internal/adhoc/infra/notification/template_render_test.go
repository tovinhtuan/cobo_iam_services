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
