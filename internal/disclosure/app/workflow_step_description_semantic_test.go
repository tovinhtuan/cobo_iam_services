package app

import (
	"strings"
	"testing"
)

func TestIsWorkflowStepDescriptionSemanticEmpty(t *testing.T) {
	emptyCases := []struct {
		name, desc, format string
	}{
		{"empty_string", "", ""},
		{"whitespace", "   \t  ", "plain_text"},
		{"newline", "\n\r\n", "plain_text"},
		{"p_empty", "<p></p>", "safe_html"},
		{"p_br", "<p><br></p>", "safe_html"},
		{"div_br", "<div><br></div>", "safe_html"},
		{"nbsp", "&nbsp;", "safe_html"},
		{"nbsp_numeric", "&#160;", "safe_html"},
		{"nested_empty", "<div><p><br></p></div>", "safe_html"},
		{"plain_looking_html_empty", "<p></p>", "plain_text"},
		{"whitespace_html", "<p>   </p>", "safe_html"},
	}
	for _, tc := range emptyCases {
		t.Run("empty_"+tc.name, func(t *testing.T) {
			if !IsWorkflowStepDescriptionSemanticEmpty(tc.desc, tc.format) {
				t.Fatalf("want empty for %q format=%q plain=%q", tc.desc, tc.format, WorkflowStepDescriptionMeaningfulPlain(tc.desc, tc.format))
			}
		})
	}

	validCases := []struct {
		name, desc, format string
	}{
		{"letter_a", "A", "plain_text"},
		{"vietnamese", "Xác nhận", "plain_text"},
		{"html_text", "<p>Mô tả bước</p>", "safe_html"},
		{"list", "<ul><li>Bước 1</li></ul>", "safe_html"},
		{"rich_vi", "<p><strong>Chuẩn bị</strong> hồ sơ</p>", "safe_html"},
		{"link_text", `<p><a href="https://example.com">Tải mẫu</a></p>`, "safe_html"},
	}
	for _, tc := range validCases {
		t.Run("valid_"+tc.name, func(t *testing.T) {
			if IsWorkflowStepDescriptionSemanticEmpty(tc.desc, tc.format) {
				t.Fatalf("want non-empty for %q format=%q", tc.desc, tc.format)
			}
		})
	}
}

func TestCollectWorkflowStepDescriptionActivationBlockers_All(t *testing.T) {
	steps := []WorkflowStepDTO{
		{StepID: "s1", Stage: "Thu thập", Description: ""},
		{StepID: "s2", Stage: "Rà soát", Description: "OK"},
		{StepID: "s3", Stage: "Phê duyệt", Description: "<p><br></p>", DescriptionFormat: "safe_html"},
	}
	blockers := CollectWorkflowStepDescriptionActivationBlockers(steps)
	if len(blockers) != 2 {
		t.Fatalf("want 2 blockers, got %v", blockers)
	}
	if blockers[0].Code != ActivationBlockerWorkflowStepDescriptionRequired || blockers[0].StepID != "s1" {
		t.Fatalf("blocker0=%+v", blockers[0])
	}
	if blockers[1].StepID != "s3" {
		t.Fatalf("blocker1=%+v", blockers[1])
	}
	if !strings.Contains(blockers[0].Message, "Thu thập") {
		t.Fatalf("message=%q", blockers[0].Message)
	}
}
