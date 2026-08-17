package app

import (
	"strings"
	"testing"
)

func TestResolveWorkflowStepReminderRule_AbsentDefaultsToThreeOne(t *testing.T) {
	res := ResolveWorkflowStepReminderRule(nil)
	if res.Kind != WorkflowStepReminderKindDefault || res.Invalid {
		t.Fatalf("kind=%s invalid=%v", res.Kind, res.Invalid)
	}
	assertDays(t, res.EffectiveDays, DefaultWorkflowStepReminderDays)
}

func TestResolveWorkflowStepReminderRule_EnabledFalseIsEmpty(t *testing.T) {
	res := ResolveWorkflowStepReminderRule(&WorkflowStepReminderConfig{
		Enabled: false,
		Mode:    WorkflowStepReminderModeDaysBefore,
	})
	if res.Kind != WorkflowStepReminderKindDisabled {
		t.Fatalf("kind=%s", res.Kind)
	}
	if res.EffectiveDays == nil || len(res.EffectiveDays) != 0 {
		t.Fatalf("effective=%v want empty slice", res.EffectiveDays)
	}
}

func TestResolveWorkflowStepReminderRule_CustomExactNoMerge(t *testing.T) {
	res := ResolveWorkflowStepReminderRule(&WorkflowStepReminderConfig{
		Enabled:    true,
		Mode:       WorkflowStepReminderModeDaysBefore,
		DaysBefore: []int{7, 2},
	})
	if res.Kind != WorkflowStepReminderKindCustom {
		t.Fatalf("kind=%s", res.Kind)
	}
	assertDays(t, res.EffectiveDays, []int{7, 2})
}

func TestNormalizeWorkflowStepReminderDays_UniqueDescending(t *testing.T) {
	days, fail, ok := NormalizeWorkflowStepReminderDays([]int{1, 3, 3, 7})
	if !ok || fail != "" {
		t.Fatalf("ok=%v fail=%s", ok, fail)
	}
	assertDays(t, days, []int{7, 3, 1})
}

func TestResolveWorkflowStepReminderRule_EmptyCustomRejected(t *testing.T) {
	res := ResolveWorkflowStepReminderRule(&WorkflowStepReminderConfig{
		Enabled:    true,
		Mode:       WorkflowStepReminderModeDaysBefore,
		DaysBefore: []int{},
	})
	if !res.Invalid || res.NormalizeFail != ReminderDaysNormalizeEmpty {
		t.Fatalf("res=%+v", res)
	}
	if res.EffectiveDays != nil {
		t.Fatalf("must not default invalid custom, got %v", res.EffectiveDays)
	}
}

func TestNormalizeWorkflowStepReminderDays_RejectsZeroNegativeTooLargeTooMany(t *testing.T) {
	cases := []struct {
		in   []int
		fail ReminderDaysNormalizeFailure
	}{
		{[]int{0}, ReminderDaysNormalizeNotPositive},
		{[]int{-1}, ReminderDaysNormalizeNotPositive},
		{[]int{91}, ReminderDaysNormalizeOffsetTooLarge},
		{[]int{1, 2, 3, 4, 5, 6, 7, 8, 9}, ReminderDaysNormalizeTooMany},
	}
	for _, tc := range cases {
		_, fail, ok := NormalizeWorkflowStepReminderDays(tc.in)
		if ok || fail != tc.fail {
			t.Fatalf("in=%v ok=%v fail=%s want %s", tc.in, ok, fail, tc.fail)
		}
	}
}

func TestResolveWorkflowStepReminderRule_SpecificDatePreserved(t *testing.T) {
	res := ResolveWorkflowStepReminderRule(&WorkflowStepReminderConfig{
		Enabled:    true,
		Mode:       WorkflowStepReminderModeSpecificDate,
		DaysBefore: []int{7, 2},
	})
	if res.Kind != WorkflowStepReminderKindSpecificDate || res.Invalid {
		t.Fatalf("res=%+v", res)
	}
	if len(res.EffectiveDays) != 0 {
		t.Fatalf("specific_date must not be reinterpreted as days_before, got %v", res.EffectiveDays)
	}
	if res.Configured == nil || res.Configured.Mode != WorkflowStepReminderModeSpecificDate {
		t.Fatalf("configured=%+v", res.Configured)
	}
}

func TestValidateWorkflowStepReminderConfigForPersist(t *testing.T) {
	if err := ValidateWorkflowStepReminderConfigForPersist(nil); err != nil {
		t.Fatalf("nil: %v", err)
	}
	if err := ValidateWorkflowStepReminderConfigForPersist(&WorkflowStepReminderConfig{Enabled: false, Mode: WorkflowStepReminderModeDaysBefore}); err != nil {
		t.Fatalf("disabled: %v", err)
	}
	if err := ValidateWorkflowStepReminderConfigForPersist(&WorkflowStepReminderConfig{Enabled: true, Mode: WorkflowStepReminderModeDaysBefore, DaysBefore: []int{5}}); err != nil {
		t.Fatalf("custom: %v", err)
	}
	if err := ValidateWorkflowStepReminderConfigForPersist(&WorkflowStepReminderConfig{Enabled: true, Mode: WorkflowStepReminderModeDaysBefore, DaysBefore: []int{}}); err == nil {
		t.Fatal("empty custom must reject")
	}
}

func TestDocumentsJSONCompatibilityMatrix(t *testing.T) {
	custom := &WorkflowStepReminderConfig{Enabled: true, Mode: WorkflowStepReminderModeDaysBefore, DaysBefore: []int{7, 2}}
	legacyDocs := []byte(`[{"doc_id":"d1","name":"QĐ"}]`)
	docsOnlyObj := []byte(`{"documents":[{"doc_id":"d1"}]}`)

	// A. no documents + no reminder
	got, err := MergeGlobalWorkflowStepReminderDocumentsJSON(nil, nil)
	if err != nil || got != nil {
		t.Fatalf("A: %v %s", err, got)
	}

	// B. reminder only
	raw, err := MergeGlobalWorkflowStepReminderDocumentsJSON(nil, custom)
	if err != nil {
		t.Fatal(err)
	}
	decoded := DecodeGlobalWorkflowStepReminderDocumentsJSON(raw)
	if decoded == nil || len(decoded.DaysBefore) != 2 || decoded.DaysBefore[0] != 7 {
		t.Fatalf("B decode=%+v", decoded)
	}

	// C. documents only (legacy array)
	got, err = MergeGlobalWorkflowStepReminderDocumentsJSON(legacyDocs, nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(legacyDocs) {
		t.Fatalf("C must preserve array, got %s", got)
	}
	if DecodeGlobalWorkflowStepReminderDocumentsJSON(got) != nil {
		t.Fatal("C array must not decode as reminder")
	}

	// D. documents + reminder (wrap array)
	got, err = MergeGlobalWorkflowStepReminderDocumentsJSON(legacyDocs, custom)
	if err != nil {
		t.Fatal(err)
	}
	if DecodeGlobalWorkflowStepReminderDocumentsJSON(got) == nil {
		t.Fatalf("D reminder missing: %s", got)
	}
	if !bytesContains(got, []byte(`"documents"`)) || !bytesContains(got, []byte(`"d1"`)) {
		t.Fatalf("D documents erased: %s", got)
	}

	// E. legacy array read does not panic
	_ = DecodeGlobalWorkflowStepReminderDocumentsJSON(legacyDocs)
	_ = DecodeGlobalWorkflowStepReminderDocumentsJSON([]byte(`not-json`))

	// F. edit reminder while documents exist (object)
	got, err = MergeGlobalWorkflowStepReminderDocumentsJSON(docsOnlyObj, custom)
	if err != nil {
		t.Fatal(err)
	}
	if DecodeGlobalWorkflowStepReminderDocumentsJSON(got) == nil || !bytesContains(got, []byte(`"d1"`)) {
		t.Fatalf("F: %s", got)
	}

	// G. edit documents while reminder exists — reminder key kept when merging reminder-only write
	//    (product documents path is unused; object keys other than reminder_config stay)
	withBoth := got
	got, err = MergeGlobalWorkflowStepReminderDocumentsJSON(withBoth, custom)
	if err != nil {
		t.Fatal(err)
	}
	if DecodeGlobalWorkflowStepReminderDocumentsJSON(got) == nil || !bytesContains(got, []byte(`"d1"`)) {
		t.Fatalf("G: %s", got)
	}

	// H. switch custom → default while documents exist
	got, err = MergeGlobalWorkflowStepReminderDocumentsJSON(withBoth, nil)
	if err != nil {
		t.Fatal(err)
	}
	if DecodeGlobalWorkflowStepReminderDocumentsJSON(got) != nil {
		t.Fatalf("H reminder key must be removed: %s", got)
	}
	if !bytesContains(got, []byte(`"d1"`)) {
		t.Fatalf("H documents erased: %s", got)
	}
}

func bytesContains(haystack, needle []byte) bool {
	return strings.Contains(string(haystack), string(needle))
}

func TestDocumentsJSONRoundTripReminderConfig(t *testing.T) {
	raw, err := EncodeGlobalWorkflowStepReminderDocumentsJSON(&WorkflowStepReminderConfig{
		Enabled: true, Mode: WorkflowStepReminderModeDaysBefore, DaysBefore: []int{7, 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := DecodeGlobalWorkflowStepReminderDocumentsJSON(raw)
	if got == nil || !got.Enabled {
		t.Fatalf("got=%+v", got)
	}
	assertDays(t, got.DaysBefore, []int{7, 2})
	if DecodeGlobalWorkflowStepReminderDocumentsJSON(nil) != nil {
		t.Fatal("nil documents_json must decode as absent")
	}
	if DecodeGlobalWorkflowStepReminderDocumentsJSON([]byte(`[{"doc_id":"d1"}]`)) != nil {
		t.Fatal("legacy document array must not be treated as reminder_config")
	}
}

func TestParseDueMinusOffset(t *testing.T) {
	n, ok := ParseDueMinusOffset("due_minus_7d")
	if !ok || n != 7 {
		t.Fatalf("n=%d ok=%v", n, ok)
	}
	if _, ok := ParseDueMinusOffset("before_start_3d"); ok {
		t.Fatal("before_start must not parse as due-minus")
	}
	if DueMinusMilestoneType(2) != "due_minus_2d" {
		t.Fatalf("type=%s", DueMinusMilestoneType(2))
	}
}

func TestDueMinusMilestoneType_ExactPersistenceNames(t *testing.T) {
	want := map[int]string{
		1:  "due_minus_1d",
		2:  "due_minus_2d",
		3:  "due_minus_3d",
		5:  "due_minus_5d",
		7:  "due_minus_7d",
		90: "due_minus_90d",
	}
	for offset, name := range want {
		got := string(DueMinusMilestoneType(offset))
		if got != name {
			t.Fatalf("offset=%d type=%q want %q", offset, got, name)
		}
		parsed, ok := ParseDueMinusOffset(got)
		if !ok || parsed != offset {
			t.Fatalf("roundtrip offset=%d parsed=%d ok=%v", offset, parsed, ok)
		}
		if !IsDueMinusReminderMilestone(got) {
			t.Fatalf("IsDueMinusReminderMilestone(%q)=false", got)
		}
	}
	for _, invalid := range []string{"", "due_minus_d", "due_minus_0d", "due_minus_-1d", "due_minus_7", "before_start_5d", "step_start", "step_end"} {
		if IsDueMinusReminderMilestone(invalid) {
			t.Fatalf("resolver must not treat %q as due-minus", invalid)
		}
	}
}

func TestRecoverDueMinusMilestoneTypeFromID(t *testing.T) {
	id := buildMilestoneID("01a00f61-730f-71f0-bfe5-5d92421814ba", "01a00f5f-5a1a-7c0d-b0e7-a77f463f892f-step-1", "due_minus_7d", func() string { return "01a00f61xxxxxxxx" })
	got, ok := RecoverDueMinusMilestoneTypeFromID(id)
	if !ok || got != "due_minus_7d" {
		t.Fatalf("id=%q got=%q ok=%v", id, got, ok)
	}
	legacy := buildMilestoneID("instance1", "step-one", "before_start_3d", func() string { return "abcdefgh" })
	if recovered, ok := RecoverDueMinusMilestoneTypeFromID(legacy); ok {
		t.Fatalf("legacy before_start must not recover as due-minus: %q %q", legacy, recovered)
	}
	if _, ok := RecoverDueMinusMilestoneTypeFromID("blank-unknown"); ok {
		t.Fatal("unknown id must not recover")
	}
}

func TestDefaultWorkflowStepReminderDaysIsCanonical(t *testing.T) {
	assertDays(t, DefaultWorkflowStepReminderDays, []int{3, 1})
}

func assertDays(t *testing.T, got, want []int) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("days=%v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("days=%v want %v", got, want)
		}
	}
}
