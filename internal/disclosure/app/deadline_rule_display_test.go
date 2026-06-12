package app

import "testing"

func TestFormatDeadlineRuleDisplay_TPlusN(t *testing.T) {
	catalog := []DeadlineRuleCatalogDTO{
		{
			Code:      "T+N",
			LabelVI:   "Trong vòng tối đa N ngày kể từ ngày sự kiện",
			Pattern:   `^T\+\d+$`,
			InputType: "number",
		},
	}
	got := FormatDeadlineRuleDisplay("T+3", catalog)
	want := "Trong vòng tối đa 3 ngày kể từ ngày sự kiện"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFormatDeadlineRuleDisplay_DateDM(t *testing.T) {
	catalog := []DeadlineRuleCatalogDTO{
		{
			Code:      "dd/mm",
			LabelVI:   "Ngày dd/mm hàng năm",
			Pattern:   `^\d{2}/\d{2}$`,
			InputType: "date_dm",
		},
	}
	got := FormatDeadlineRuleDisplay("31/03", catalog)
	want := "Ngày 31/03 hàng năm"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFormatDeadlineRuleDisplay_UnmatchedPassthrough(t *testing.T) {
	catalog := defaultDeadlineRuleCatalog()
	got := FormatDeadlineRuleDisplay("Theo cấu hình admin", catalog)
	if got != "Theo cấu hình admin" {
		t.Fatalf("got %q want passthrough", got)
	}
}
