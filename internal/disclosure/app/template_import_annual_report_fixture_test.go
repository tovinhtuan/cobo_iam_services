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
	if len(errs) != 0 {
		for _, e := range errs {
			t.Errorf("%s %s %s", e.Code, e.FieldPath, e.Message)
		}
	}
	t.Logf("mappings=%d blockers=%d", len(mappings), len(blockers))
}
