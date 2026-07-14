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

func TestEnrichDeadlineRuleDisplay_usesRawAdminText(t *testing.T) {
	item := &DisclosureTypeDTO{
		DeadlineRule:     "Trong vòng tối đa 20 ngày kể từ ngày kết thúc quý",
		TemplateCategory: TemplateCategoryPeriodic,
		DeadlineConfig: &TemplateDeadlineConfig{
			DeadlineMode: DeadlineModePeriodic,
			T0Policy:     "system_date",
			DeadlineDays: 20,
		},
	}
	enrichDeadlineRuleDisplay(item, defaultDeadlineRuleCatalog())
	if item.DeadlineRuleDisplay != item.DeadlineRule {
		t.Fatalf("display=%q want raw %q", item.DeadlineRuleDisplay, item.DeadlineRule)
	}
	if item.TimeCalculationBasis != "" {
		t.Fatalf("basis should stay empty for display-only SoT, got %q", item.TimeCalculationBasis)
	}
}

func TestEnrichDeadlineRuleDisplay_TPlusPassthroughNotCatalog(t *testing.T) {
	item := &DisclosureTypeDTO{DeadlineRule: "T+20"}
	enrichDeadlineRuleDisplay(item, []DeadlineRuleCatalogDTO{
		{Code: "T+N", LabelVI: "Trong vòng tối đa N ngày kể từ ngày sự kiện", Pattern: `^T\+\d+$`, InputType: "number"},
	})
	if item.DeadlineRuleDisplay != "T+20" {
		t.Fatalf("display=%q want T+20", item.DeadlineRuleDisplay)
	}
}

func TestT0PolicyBasisLabelVI(t *testing.T) {
	if T0PolicyBasisLabelVI("system_date") != "Ngày hệ thống" {
		t.Fatal(T0PolicyBasisLabelVI("system_date"))
	}
}
