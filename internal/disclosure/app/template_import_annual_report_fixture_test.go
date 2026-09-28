package app

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

func TestAnnualReportFixedJSONDomain(t *testing.T) {
	raw, err := os.ReadFile("../../../docs/ai-cache/10_bao-cao-thuong-nien.fixed.json")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte("Đang hiển thị")) || bytes.Contains(raw, []byte("100%")) {
		t.Fatal("ui chrome leaked into json")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var env TemplateImportEnvelopeV1
	if err := dec.Decode(&env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		t.Fatalf("extra json: %v", err)
	}
	if env.SchemaVersion != TemplateImportSchemaVersion || strings.TrimSpace(env.Template.Name) == "" {
		t.Fatalf("schema/name %+v", env.SchemaVersion)
	}
	if env.Template.TemplateCategory != "periodic" || env.Template.Periodicity != "yearly" {
		t.Fatalf("category=%s periodicity=%s", env.Template.TemplateCategory, env.Template.Periodicity)
	}
	if env.Template.DeadlineConfig == nil || env.Template.DeadlineConfig.ApplicableFromSlot != "" {
		t.Fatal("NEXT_SLOT file must not carry applicable_from_slot")
	}
	normalized := NormalizeTemplateImportV1(env.Template)
	if normalized.DeadlineConfig.ApplicableFromMode != ApplicableFromModeNext || normalized.DeadlineConfig.ApplicableFromSlot != "" {
		t.Fatalf("normalized from mode=%s slot=%q", normalized.DeadlineConfig.ApplicableFromMode, normalized.DeadlineConfig.ApplicableFromSlot)
	}
	if normalized.DeadlineConfig.DeadlineDays == nil || *normalized.DeadlineConfig.DeadlineDays != 110 {
		t.Fatalf("deadline days %+v", normalized.DeadlineConfig.DeadlineDays)
	}
	errs, _, mappings, blockers := ValidateImportTemplate(normalized, time.Now(), nil, nil, nil)
	if len(errs) != 1 || errs[0].Code != "WORKFLOW_DOCUMENT_FILE_TYPES_TOO_LONG" {
		t.Fatalf("original format must fail file_types parity before confirm: %+v", errs)
	}
	if errs[0].FieldPath != "template.workflow.blocks[3].config.file_types" {
		t.Fatalf("field path %s", errs[0].FieldPath)
	}
	t.Logf("mappings=%d blockers=%d", len(mappings), len(blockers))
}

func TestAnnualReportFixedV2JSONDomain(t *testing.T) {
	raw, err := os.ReadFile("../../../docs/ai-cache/10_bao-cao-thuong-nien.fixed-v2.json")
	if err != nil {
		t.Fatal(err)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var env TemplateImportEnvelopeV1
	if err := dec.Decode(&env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	normalized := NormalizeTemplateImportV1(env.Template)
	if normalized.Periodicity != "yearly" || normalized.DeadlineRule != "T+110" {
		t.Fatalf("periodicity=%s rule=%s", normalized.Periodicity, normalized.DeadlineRule)
	}
	if normalized.DeadlineConfig == nil || normalized.DeadlineConfig.ApplicableFromMode != ApplicableFromModeNext {
		t.Fatal("expected NEXT_SLOT")
	}
	errs, _, mappings, _ := ValidateImportTemplate(normalized, time.Now(), nil, nil, nil)
	if len(errs) != 0 {
		for _, e := range errs {
			t.Errorf("%s %s %s", e.Code, e.FieldPath, e.Message)
		}
	}
	codes := map[string]int{}
	for _, step := range normalized.Workflow.Steps {
		if code := strings.TrimSpace(step.DepartmentID); code != "" {
			codes[code]++
		}
	}
	for _, code := range []string{"corporate_secretary", "legal", "bod"} {
		if codes[code] == 0 {
			t.Fatalf("missing mapping source %s", code)
		}
	}
	t.Logf("mappings=%d", len(mappings))
}

func TestImportFileTypesRuneLimitMatchesConfirm(t *testing.T) {
	base := TemplateImportDefinitionV1{Name: "N", TemplateCategory: TemplateCategoryPeriodic, Periodicity: "yearly"}
	pass := base
	pass.Format = strings.Repeat("á", 64)
	errs, _, _, _ := ValidateImportTemplate(&pass, time.Now(), nil, nil, nil)
	for _, e := range errs {
		if e.Code == "WORKFLOW_DOCUMENT_FILE_TYPES_TOO_LONG" {
			t.Fatalf("64 runes must pass: %+v", e)
		}
	}
	fail := base
	fail.Format = strings.Repeat("á", 65)
	errs, _, _, _ = ValidateImportTemplate(&fail, time.Now(), nil, nil, nil)
	found := false
	for _, e := range errs {
		if e.Code == "WORKFLOW_DOCUMENT_FILE_TYPES_TOO_LONG" && e.FieldPath == "template.workflow.blocks[3].config.file_types" {
			found = true
		}
	}
	if !found {
		t.Fatalf("65 runes must fail parity: %+v", errs)
	}
	blank := base
	blank.Format = "   "
	errs, _, _, _ = ValidateImportTemplate(&blank, time.Now(), nil, nil, nil)
	for _, e := range errs {
		if e.Code == "WORKFLOW_DOCUMENT_FILE_TYPES_TOO_LONG" {
			t.Fatalf("blank format falls back to PDF: %+v", e)
		}
	}
	if got := importMaterializedFileTypes("  PDF  "); len(got) != 1 || got[0] != "PDF" {
		t.Fatalf("trim mismatch: %#v", got)
	}
}
