package app_test

import (
	"context"
	"encoding/json"
	"testing"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	"github.com/cobo/cobo_iam_services/internal/disclosure/infra/inmemory"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

func seedPeriodicWithApplicableTo(t *testing.T, typeID, applicableTo string) (disclosureapp.Service, *inmemory.Repository) {
	t.Helper()
	repo := inmemory.NewRepository()
	seedTemplateDraft(t, repo, typeID)
	detail, err := repo.GetTypeVersionDetail(context.Background(), testSubjectWF.CompanyID, typeID, 1)
	if err != nil {
		t.Fatalf("detail: %v", err)
	}
	cfg := &disclosureapp.TemplateDeadlineConfig{
		DeadlineMode:     "PERIODIC",
		T0Policy:         "system_date",
		DeadlineDays:     10,
		FrequencyUnit:    "monthly",
		CycleAnchorDay:   10,
		TemplateCategory: "periodic",
		ApplicableTo:     applicableTo,
	}
	if applicableTo != "" {
		cfg.ApplicableToProvided = true
	}
	upsert := disclosureapp.UpsertTypeVersionRequest{
		Subject: testSubjectWF, TypeID: typeID, Scope: "global", GroupID: detail.GroupID,
		Name: detail.Name, Category: detail.Category, TemplateCategory: detail.TemplateCategory,
		DeadlineStrategy: detail.DeadlineStrategy, DeadlineRule: detail.DeadlineRule,
		Periodicity: detail.Periodicity, DisplayGroupCodes: detail.DisplayGroupCodes,
		ApplicabilityRules: detail.ApplicabilityRules, Blocks: detail.Blocks,
		DeadlineConfig: cfg, Description: detail.Description,
	}
	cand, err := disclosureapp.BuildTemplatePublicationCandidate(upsert)
	if err != nil {
		t.Fatalf("candidate: %v", err)
	}
	upsert.PublicationCandidate = &cand
	if _, err := repo.UpsertTypeVersion(context.Background(), upsert); err != nil {
		t.Fatalf("seed deadline: %v", err)
	}
	svc := disclosureapp.NewService(repo, nil, idgen.UUIDv7Generator{})
	mustUpsertWF(t, svc, typeID, fourSteps(typeID))
	mustActivateSource(t, svc, typeID, 1)
	return svc, repo
}

func baseUpsertFromDetail(detail *disclosureapp.DisclosureTypeDTO, typeID string) disclosureapp.UpsertTypeVersionRequest {
	return disclosureapp.UpsertTypeVersionRequest{
		Subject: testSubjectWF, TypeID: typeID, Scope: "global", GroupID: detail.GroupID,
		Name: detail.Name, Category: detail.Category, TemplateCategory: detail.TemplateCategory,
		DeadlineStrategy: detail.DeadlineStrategy, DeadlineRule: detail.DeadlineRule,
		Periodicity: detail.Periodicity, DisplayGroupCodes: detail.DisplayGroupCodes,
		ApplicabilityRules: detail.ApplicabilityRules, Blocks: detail.Blocks,
		Description: detail.Description, SkipPublicationMatrix: true,
	}
}

func TestApplicableTo_JSONRoundTripAndOmitDetection(t *testing.T) {
	rawWith := []byte(`{"deadline_mode":"PERIODIC","frequency_unit":"monthly","applicable_to":"2026-12-31"}`)
	var cfg disclosureapp.TemplateDeadlineConfig
	if err := json.Unmarshal(rawWith, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.ApplicableTo != "2026-12-31" || !cfg.ApplicableToProvided {
		t.Fatalf("want provided date, got %+v", cfg)
	}
	out, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var again disclosureapp.TemplateDeadlineConfig
	if err := json.Unmarshal(out, &again); err != nil || again.ApplicableTo != "2026-12-31" {
		t.Fatalf("round-trip failed: %v %+v", err, again)
	}

	var omit disclosureapp.TemplateDeadlineConfig
	if err := json.Unmarshal([]byte(`{"deadline_mode":"PERIODIC","frequency_unit":"monthly"}`), &omit); err != nil {
		t.Fatal(err)
	}
	if omit.ApplicableToProvided || omit.ApplicableTo != "" {
		t.Fatalf("omit must leave Provided=false empty, got %+v", omit)
	}
	if !disclosureapp.ShouldPreserveApplicableTo(&omit) {
		t.Fatal("omit empty must preserve")
	}

	var clear disclosureapp.TemplateDeadlineConfig
	if err := json.Unmarshal([]byte(`{"deadline_mode":"PERIODIC","applicable_to":""}`), &clear); err != nil {
		t.Fatal(err)
	}
	if !clear.ApplicableToProvided || disclosureapp.ShouldPreserveApplicableTo(&clear) {
		t.Fatalf("explicit empty must clear, got %+v", clear)
	}
}

func TestApplicableTo_DetailReadRoundTrip(t *testing.T) {
	const typeID = "dt-at-detail"
	svc, _ := seedPeriodicWithApplicableTo(t, typeID, "2026-12-31")
	detail, err := svc.GetTypeVersionDetail(context.Background(), disclosureapp.GetTypeVersionDetailRequest{
		Subject: testSubjectWF, TypeID: typeID, VersionNo: 1,
	})
	if err != nil {
		t.Fatalf("GetTypeVersionDetail: %v", err)
	}
	if detail.DeadlineConfig == nil || detail.DeadlineConfig.ApplicableTo != "2026-12-31" {
		t.Fatalf("detail ApplicableTo want 2026-12-31 got %+v", detail.DeadlineConfig)
	}
}

func TestApplicableTo_CloneClearsAndSourceUnchanged(t *testing.T) {
	const sourceID = "dt-at-clone-src"
	const targetID = "dt-at-clone-tgt"
	svc, repo := seedPeriodicWithApplicableTo(t, sourceID, "2026-12-31")

	_, err := svc.CloneTypeFromActive(context.Background(), disclosureapp.CloneTypeFromActiveRequest{
		Subject: testSubjectWF, SourceTypeID: sourceID, TargetTypeID: targetID,
		TargetName: "Clone AT Target", ExpectedSourceVersionNo: 1,
	})
	if err != nil {
		t.Fatalf("clone: %v", err)
	}
	src, err := repo.GetTypeVersionDetail(context.Background(), testSubjectWF.CompanyID, sourceID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if src.DeadlineConfig == nil || src.DeadlineConfig.ApplicableTo != "2026-12-31" {
		t.Fatalf("source must stay 2026-12-31, got %+v", src.DeadlineConfig)
	}
	tgt, err := repo.GetTypeVersionDetail(context.Background(), testSubjectWF.CompanyID, targetID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if tgt.DeadlineConfig == nil {
		t.Fatal("clone must retain deadline_config shell")
	}
	if tgt.DeadlineConfig.ApplicableTo != "" {
		t.Fatalf("clone ApplicableTo must CLEAR, got %q", tgt.DeadlineConfig.ApplicableTo)
	}
	if tgt.DeadlineConfig.FrequencyUnit == "" && tgt.DeadlineConfig.CycleAnchorDay == 0 {
		// schedule may still be present from copy — only ApplicableTo clears
	}
	if tgt.DeadlineConfig.FrequencyUnit != "monthly" || tgt.DeadlineConfig.CycleAnchorDay != 10 {
		t.Fatalf("clone must keep schedule fields, got %+v", tgt.DeadlineConfig)
	}
}

func TestApplicableTo_SameRootNewVersionPreserves(t *testing.T) {
	const typeID = "dt-at-ver-preserve"
	svc, repo := seedPeriodicWithApplicableTo(t, typeID, "2026-12-31")
	detail, err := repo.GetTypeVersionDetail(context.Background(), testSubjectWF.CompanyID, typeID, 1)
	if err != nil {
		t.Fatal(err)
	}
	req := baseUpsertFromDetail(detail, typeID)
	// Simulate older FE: deadline_config without applicable_to key.
	var cfg disclosureapp.TemplateDeadlineConfig
	if err := json.Unmarshal([]byte(`{
		"deadline_mode":"PERIODIC","t0_policy":"system_date","deadline_days":10,
		"frequency_unit":"monthly","cycle_anchor_day":10,"template_category":"periodic"
	}`), &cfg); err != nil {
		t.Fatal(err)
	}
	req.DeadlineConfig = &cfg
	if _, err := svc.UpsertTypeVersion(context.Background(), req); err != nil {
		t.Fatalf("new version upsert: %v", err)
	}
	v2, err := repo.GetTypeVersionDetail(context.Background(), testSubjectWF.CompanyID, typeID, 2)
	if err != nil {
		t.Fatalf("v2: %v", err)
	}
	if v2.DeadlineConfig == nil || v2.DeadlineConfig.ApplicableTo != "2026-12-31" {
		t.Fatalf("same-root preserve want 2026-12-31 got %+v", v2.DeadlineConfig)
	}
	v1, err := repo.GetTypeVersionDetail(context.Background(), testSubjectWF.CompanyID, typeID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if v1.DeadlineConfig.ApplicableTo != "2026-12-31" {
		t.Fatal("v1 must remain unchanged")
	}
}

func TestApplicableTo_SameRootExplicitEditAndClear(t *testing.T) {
	const typeID = "dt-at-ver-edit"
	svc, repo := seedPeriodicWithApplicableTo(t, typeID, "2026-12-31")
	detail, err := repo.GetTypeVersionDetail(context.Background(), testSubjectWF.CompanyID, typeID, 1)
	if err != nil {
		t.Fatal(err)
	}
	req := baseUpsertFromDetail(detail, typeID)
	req.DeadlineConfig = &disclosureapp.TemplateDeadlineConfig{
		DeadlineMode: "PERIODIC", T0Policy: "system_date", DeadlineDays: 10,
		FrequencyUnit: "monthly", CycleAnchorDay: 10, TemplateCategory: "periodic",
		ApplicableTo: "2027-06-30", ApplicableToProvided: true,
	}
	if _, err := svc.UpsertTypeVersion(context.Background(), req); err != nil {
		t.Fatalf("edit upsert: %v", err)
	}
	v2, err := repo.GetTypeVersionDetail(context.Background(), testSubjectWF.CompanyID, typeID, 2)
	if err != nil {
		t.Fatal(err)
	}
	if v2.DeadlineConfig.ApplicableTo != "2027-06-30" {
		t.Fatalf("explicit edit want 2027-06-30 got %q", v2.DeadlineConfig.ApplicableTo)
	}
	v1, _ := repo.GetTypeVersionDetail(context.Background(), testSubjectWF.CompanyID, typeID, 1)
	if v1.DeadlineConfig.ApplicableTo != "2026-12-31" {
		t.Fatal("edit must not mutate previous version")
	}

	// Overwrite open draft v2 with explicit clear.
	detail2, _ := repo.GetTypeVersionDetail(context.Background(), testSubjectWF.CompanyID, typeID, 2)
	clearReq := baseUpsertFromDetail(detail2, typeID)
	var clearCfg disclosureapp.TemplateDeadlineConfig
	_ = json.Unmarshal([]byte(`{
		"deadline_mode":"PERIODIC","t0_policy":"system_date","deadline_days":10,
		"frequency_unit":"monthly","cycle_anchor_day":10,"template_category":"periodic",
		"applicable_to":""
	}`), &clearCfg)
	clearReq.DeadlineConfig = &clearCfg
	if _, err := svc.UpsertTypeVersion(context.Background(), clearReq); err != nil {
		t.Fatalf("clear upsert: %v", err)
	}
	v2b, _ := repo.GetTypeVersionDetail(context.Background(), testSubjectWF.CompanyID, typeID, 2)
	if v2b.DeadlineConfig.ApplicableTo != "" {
		t.Fatalf("explicit clear want OPEN_ENDED got %q", v2b.DeadlineConfig.ApplicableTo)
	}
	v1b, _ := repo.GetTypeVersionDetail(context.Background(), testSubjectWF.CompanyID, typeID, 1)
	if v1b.DeadlineConfig.ApplicableTo != "2026-12-31" {
		t.Fatal("clear must not mutate v1")
	}
}

func TestApplicableTo_UnrelatedEditPreservesViaOmit(t *testing.T) {
	const typeID = "dt-at-unrelated"
	svc, repo := seedPeriodicWithApplicableTo(t, typeID, "2026-12-31")
	detail, err := repo.GetTypeVersionDetail(context.Background(), testSubjectWF.CompanyID, typeID, 1)
	if err != nil {
		t.Fatal(err)
	}
	req := baseUpsertFromDetail(detail, typeID)
	req.Description = "unrelated description edit only"
	var cfg disclosureapp.TemplateDeadlineConfig
	_ = json.Unmarshal([]byte(`{
		"deadline_mode":"PERIODIC","t0_policy":"system_date","deadline_days":10,
		"frequency_unit":"monthly","cycle_anchor_day":10,"template_category":"periodic"
	}`), &cfg)
	req.DeadlineConfig = &cfg
	if _, err := svc.UpsertTypeVersion(context.Background(), req); err != nil {
		t.Fatalf("unrelated upsert: %v", err)
	}
	v2, err := repo.GetTypeVersionDetail(context.Background(), testSubjectWF.CompanyID, typeID, 2)
	if err != nil {
		t.Fatal(err)
	}
	if v2.Description != "unrelated description edit only" {
		t.Fatalf("description not updated: %q", v2.Description)
	}
	if v2.DeadlineConfig.ApplicableTo != "2026-12-31" {
		t.Fatalf("unrelated edit must preserve ApplicableTo, got %q", v2.DeadlineConfig.ApplicableTo)
	}
}

func TestApplicableTo_CloneVsVersionSemanticsPair(t *testing.T) {
	const sourceID = "dt-at-pair-src"
	const targetID = "dt-at-pair-clone"
	svc, repo := seedPeriodicWithApplicableTo(t, sourceID, "2026-12-31")

	detail, _ := repo.GetTypeVersionDetail(context.Background(), testSubjectWF.CompanyID, sourceID, 1)
	req := baseUpsertFromDetail(detail, sourceID)
	var cfg disclosureapp.TemplateDeadlineConfig
	_ = json.Unmarshal([]byte(`{
		"deadline_mode":"PERIODIC","t0_policy":"system_date","deadline_days":10,
		"frequency_unit":"monthly","cycle_anchor_day":10,"template_category":"periodic"
	}`), &cfg)
	req.DeadlineConfig = &cfg
	if _, err := svc.UpsertTypeVersion(context.Background(), req); err != nil {
		t.Fatalf("version: %v", err)
	}
	v2, _ := repo.GetTypeVersionDetail(context.Background(), testSubjectWF.CompanyID, sourceID, 2)
	if v2.DeadlineConfig.ApplicableTo != "2026-12-31" {
		t.Fatalf("version preserve got %q", v2.DeadlineConfig.ApplicableTo)
	}

	if _, err := svc.CloneTypeFromActive(context.Background(), disclosureapp.CloneTypeFromActiveRequest{
		Subject: testSubjectWF, SourceTypeID: sourceID, TargetTypeID: targetID,
		TargetName: "Pair Clone", ExpectedSourceVersionNo: 1,
	}); err != nil {
		t.Fatalf("clone: %v", err)
	}
	clone, _ := repo.GetTypeVersionDetail(context.Background(), testSubjectWF.CompanyID, targetID, 1)
	if clone.DeadlineConfig.ApplicableTo != "" {
		t.Fatalf("clone clear got %q", clone.DeadlineConfig.ApplicableTo)
	}
}

func TestApplicableTo_LegacyOpenEndedClone(t *testing.T) {
	const sourceID = "dt-at-open-src"
	const targetID = "dt-at-open-tgt"
	svc, repo := seedPeriodicWithApplicableTo(t, sourceID, "")
	if _, err := svc.CloneTypeFromActive(context.Background(), disclosureapp.CloneTypeFromActiveRequest{
		Subject: testSubjectWF, SourceTypeID: sourceID, TargetTypeID: targetID,
		TargetName: "Open Clone", ExpectedSourceVersionNo: 1,
	}); err != nil {
		t.Fatalf("clone: %v", err)
	}
	tgt, _ := repo.GetTypeVersionDetail(context.Background(), testSubjectWF.CompanyID, targetID, 1)
	if tgt.DeadlineConfig.ApplicableTo != "" {
		t.Fatalf("open-ended clone want empty got %q", tgt.DeadlineConfig.ApplicableTo)
	}
}

func TestApplicableTo_ApplyCloneDefaultsHelper(t *testing.T) {
	cfg := &disclosureapp.TemplateDeadlineConfig{ApplicableTo: "2026-12-31", FrequencyUnit: "monthly"}
	disclosureapp.ApplyCloneApplicableToDefaults(cfg)
	if cfg.ApplicableTo != "" || !cfg.ApplicableToProvided {
		t.Fatalf("clone defaults: %+v", cfg)
	}
}

func TestApplicableTo_CompanyPreferenceCannotSet(t *testing.T) {
	// Company preference DTO has no ApplicableTo field — structural guarantee.
	var pref disclosureapp.UpsertCompanyTypePreferenceRequest
	raw, _ := json.Marshal(pref)
	if string(raw) != "{}" && containsApplicableToKey(raw) {
		t.Fatalf("company preference must not carry applicable_to: %s", raw)
	}
}

func containsApplicableToKey(raw []byte) bool {
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	_, ok := m["applicable_to"]
	return ok
}
