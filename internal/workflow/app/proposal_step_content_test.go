package app

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	adhocapp "github.com/cobo/cobo_iam_services/internal/adhoc/app"
)

func proposalWithInheritedContent() *adhocapp.ProposalWorkflowSnapshot {
	return &adhocapp.ProposalWorkflowSnapshot{
		SchemaVersion: adhocapp.ProposalWorkflowSchemaV3,
		Frozen:        true,
		Steps: []adhocapp.ProposalWorkflowStep{
			{
				ID: "ps-1", SourceStepID: "tpl-1", Order: 1, Name: "A", ProcessingDays: 2, DepartmentID: "d1", AssigneeMembershipIDs: []string{"m1"},
				Inherited: &adhocapp.ProposalStepInheritedContent{
					Description: "<p>Đối chiếu</p>", DescriptionFormat: "safe_html", Instructions: "legacy",
					Documents: []adhocapp.ProposalStepDocument{
						{DocID: "doc-1", Name: "Nghị quyết", Required: true, TemplateFileID: "wdt-1", TemplateFileName: "mau.docx"},
						{DocID: "", Name: "Không mã", TemplateFileName: "orphan-name.docx"},
						{DocID: "doc-1", Name: "Trùng mã"},
					},
					ReminderConfig: &adhocapp.ProposalStepReminderConfig{Enabled: true, Mode: "days_before", DaysBefore: []int{2, 5}},
				},
			},
			{ID: "ps-2", SourceStepID: "tpl-2", Order: 2, Name: "B", ProcessingDays: 1, DepartmentID: "d1", AssigneeMembershipIDs: []string{"m1"},
				Inherited: &adhocapp.ProposalStepInheritedContent{ReminderConfig: &adhocapp.ProposalStepReminderConfig{Enabled: false}}},
			{ID: "ps-3", SourceStepID: "tpl-3", Order: 3, Name: "C", ProcessingDays: 1, DepartmentID: "d1", AssigneeMembershipIDs: []string{"m1"},
				Inherited: &adhocapp.ProposalStepInheritedContent{}},
			{ID: "ps-4", Order: 4, Name: "Tự thêm", ProcessingDays: 1, DepartmentID: "d1", AssigneeMembershipIDs: []string{"m1"}},
		},
	}
}

func TestMapProposalWorkflowToSnapshot_CopiesCapturedStepContent(t *testing.T) {
	snap := MapProposalWorkflowToSnapshot(proposalWithInheritedContent())
	if len(snap) != 4 {
		t.Fatalf("steps=%d", len(snap))
	}
	s1 := snap[0]
	if s1.SourceStepID != "tpl-1" || s1.Description != "<p>Đối chiếu</p>" || s1.DescriptionFormat != "safe_html" {
		t.Fatalf("content not copied: %#v", s1)
	}
	if raw, _ := json.Marshal(s1); strings.Contains(string(raw), `"instructions"`) {
		t.Fatalf("legacy instructions must not reach the runtime snapshot (reminder emails): %s", raw)
	}
	if len(s1.Documents) != 3 || !s1.Documents[0].Required || s1.Documents[0].TemplateFileName != "mau.docx" || s1.Documents[1].TemplateFileName != "" {
		t.Fatalf("documents=%#v", s1.Documents)
	}
	if s1.ReminderConfig == nil || !reflect.DeepEqual(s1.EffectiveReminderDays, []int{5, 2}) {
		t.Fatalf("custom reminder: cfg=%#v days=%v", s1.ReminderConfig, s1.EffectiveReminderDays)
	}
	if snap[1].EffectiveReminderDays == nil || len(snap[1].EffectiveReminderDays) != 0 {
		t.Fatalf("disabled reminder must freeze []: %v", snap[1].EffectiveReminderDays)
	}
	for _, i := range []int{2, 3} {
		if !reflect.DeepEqual(snap[i].EffectiveReminderDays, []int{3, 1}) || snap[i].ReminderConfig != nil || snap[i].Description != "" || snap[i].Documents != nil {
			t.Fatalf("step %d must keep default reminders and no content: %#v", i, snap[i])
		}
	}
	if snap[3].SourceStepID != "" {
		t.Fatalf("self-added step has no provenance: %q", snap[3].SourceStepID)
	}
	raw, err := json.Marshal(snap[2])
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{`"description"`, `"documents"`, `"reminder_config"`} {
		if strings.Contains(string(raw), field) {
			t.Fatalf("empty content must be omitted (%s): %s", field, raw)
		}
	}
}

func TestProjectProposalDocumentRequirementSnapshots(t *testing.T) {
	rows := ProjectProposalDocumentRequirementSnapshots(proposalWithInheritedContent())
	if len(rows) != 2 {
		t.Fatalf("want 2 rows (duplicate doc id dropped), got %#v", rows)
	}
	if rows[0].StepCode != "ps-1" || rows[0].SourceDocID != "doc-1" || rows[0].RequirementKey != "doc-1" || !rows[0].Required ||
		rows[0].TemplateFileID != "wdt-1" || rows[0].TemplateFileName != "mau.docx" || rows[0].Ordinal != 0 {
		t.Fatalf("row0=%#v", rows[0])
	}
	if rows[1].SourceDocID != "auto:ps-1:1" || rows[1].Name != "Không mã" || rows[1].TemplateFileName != "" || rows[1].Ordinal != 1 {
		t.Fatalf("row1=%#v", rows[1])
	}
	if got := ProjectProposalDocumentRequirementSnapshots(nil); got != nil {
		t.Fatalf("nil snapshot: %#v", got)
	}
	noContent := proposalWithInheritedContent()
	for i := range noContent.Steps {
		noContent.Steps[i].Inherited = nil
	}
	if got := ProjectProposalDocumentRequirementSnapshots(noContent); len(got) != 0 {
		t.Fatalf("proposals without captured content keep no requirements: %#v", got)
	}
}

func TestProjectProposalDocumentRequirementSnapshots_FitsTableLimits(t *testing.T) {
	longID := strings.Repeat("d", 129)
	snap := &adhocapp.ProposalWorkflowSnapshot{SchemaVersion: adhocapp.ProposalWorkflowSchemaV3, Steps: []adhocapp.ProposalWorkflowStep{{
		ID: "ps-1",
		Inherited: &adhocapp.ProposalStepInheritedContent{Documents: []adhocapp.ProposalStepDocument{
			{DocID: "Doc-1", Name: strings.Repeat("ê", 600), TemplateFileID: strings.Repeat("f", 65), TemplateFileName: "x.docx"},
			{DocID: "doc-1", Name: "case duplicate"},
			{DocID: longID, Name: "long id", TemplateFileID: "wdt-1", TemplateFileName: strings.Repeat("n", 300)},
		}},
	}}}
	rows := ProjectProposalDocumentRequirementSnapshots(snap)
	if len(rows) != 2 {
		t.Fatalf("case-insensitive duplicate must be dropped: %#v", rows)
	}
	if got := len([]rune(rows[0].Name)); got != 512 {
		t.Fatalf("name runes=%d want 512", got)
	}
	if rows[0].TemplateFileID != "" || rows[0].TemplateFileName != "" {
		t.Fatalf("oversized template file id must be dropped: %#v", rows[0])
	}
	if rows[1].SourceDocID != "auto:ps-1:2" || len([]rune(rows[1].TemplateFileName)) != 255 {
		t.Fatalf("long doc id → auto key, template name capped: %#v", rows[1])
	}
}
