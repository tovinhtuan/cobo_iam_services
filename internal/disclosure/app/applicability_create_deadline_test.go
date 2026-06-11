package app_test

import (
	"context"
	"net/http"
	"testing"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	"github.com/cobo/cobo_iam_services/internal/disclosure/app/applicability"
	"github.com/cobo/cobo_iam_services/internal/disclosure/infra/inmemory"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

type e11TestRepo struct {
	*inmemory.Repository
	profile       applicability.CompanyApplicabilityProfile
	typeOverrides map[string]*disclosureapp.DisclosureTypeDTO
}

func (r *e11TestRepo) GetCompanyApplicabilityProfile(_ context.Context, _ string) (applicability.CompanyApplicabilityProfile, error) {
	return r.profile, nil
}

func (r *e11TestRepo) GetTypeDetail(ctx context.Context, companyID, typeID string) (*disclosureapp.DisclosureTypeDTO, error) {
	if override, ok := r.typeOverrides[typeID]; ok {
		cp := *override
		return &cp, nil
	}
	return r.Repository.GetTypeDetail(ctx, companyID, typeID)
}

func (r *e11TestRepo) HasActiveEnterpriseWorkflow(_ context.Context, _, typeID string) (bool, error) {
	if override, ok := r.typeOverrides[typeID]; ok {
		return disclosureapp.TemplateHasWorkflow(override.Blocks), nil
	}
	return true, nil
}

func fullStructureRules() *applicability.TemplateApplicabilityRules {
	return &applicability.TemplateApplicabilityRules{
		ApplicableCompanyClasses: []applicability.CompanyClass{applicability.CompanyClassListed},
		ApplicableSectors:        []applicability.BusinessSector{applicability.BusinessSectorCommercial},
		DeadlineByStructure: map[applicability.StructureCriterion]applicability.StructureDeadlineEntry{
			applicability.StructureHasSubsidiaries:     {Days: 30},
			applicability.StructureHasSubordinateUnits: {Days: 30},
			applicability.StructureSimpleStructure:     {Days: 20},
		},
	}
}

func applicableProfile() applicability.CompanyApplicabilityProfile {
	sector := applicability.BusinessSectorCommercial
	return applicability.CompanyApplicabilityProfile{
		IsListed:       true,
		BusinessSector: &sector,
	}
}

func newE11Service(repo *e11TestRepo) disclosureapp.Service {
	return disclosureapp.NewService(repo, nil, idgen.UUIDv7Generator{}, disclosureapp.WithTemplateApplicabilityStrictFilter(true))
}

func baseCreateRecordRequest(typeID string) disclosureapp.CreateRecordRequest {
	return disclosureapp.CreateRecordRequest{
		Subject: disclosureapp.Subject{UserID: "u1", MembershipID: "m1", CompanyID: "co-1"},
		Payload: disclosureapp.RecordPayload{
			TypeID:      typeID,
			Title:       "Báo cáo quý",
			Content:     "Nội dung",
			PlannedDate: "2026-06-30",
		},
	}
}

func workflowTypeDetail(typeID string, rules *applicability.TemplateApplicabilityRules) *disclosureapp.DisclosureTypeDTO {
	return &disclosureapp.DisclosureTypeDTO{
		TypeID:             typeID,
		Scope:              "global",
		ApplicabilityRules: rules,
		Blocks: []disclosureapp.TemplateBlockDTO{
			{
				BlockKey:  "enterprise_workflow",
				BlockType: "rich_text",
				Enabled:   true,
				Config: map[string]any{
					"steps": []any{
						map[string]any{
							"step_id":           "s1",
							"stage":             "Review",
							"department_id":     "dept-1",
							"assignee_role_ids": []any{"role-1"},
							"processing_days":   2,
							"display_order":     1,
						},
					},
				},
			},
		},
	}
}

func TestCreateRecord_E11_MissingHasSubsidiariesKey(t *testing.T) {
	sector := applicability.BusinessSectorCommercial
	repo := &e11TestRepo{
		Repository: inmemory.NewRepository(),
		profile: applicability.CompanyApplicabilityProfile{
			IsListed:        true,
			HasSubsidiaries: true,
			BusinessSector:  &sector,
		},
		typeOverrides: map[string]*disclosureapp.DisclosureTypeDTO{
			"dt-bctc": workflowTypeDetail("dt-bctc", &applicability.TemplateApplicabilityRules{
				ApplicableCompanyClasses: []applicability.CompanyClass{applicability.CompanyClassListed},
				ApplicableSectors:        []applicability.BusinessSector{applicability.BusinessSectorCommercial},
				DeadlineByStructure: map[applicability.StructureCriterion]applicability.StructureDeadlineEntry{
					applicability.StructureHasSubordinateUnits: {Days: 30},
					applicability.StructureSimpleStructure:     {Days: 20},
				},
			}),
		},
	}
	svc := newE11Service(repo)

	_, err := svc.CreateRecord(context.Background(), baseCreateRecordRequest("dt-bctc"))
	if err == nil {
		t.Fatal("expected E11 error")
	}
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusUnprocessableEntity || he.Code != "DEADLINE_RULE_UNRESOLVED" {
		t.Fatalf("got %#v want 422 DEADLINE_RULE_UNRESOLVED", err)
	}
}

func TestCreateRecord_E11_MissingSimpleStructureKey(t *testing.T) {
	repo := &e11TestRepo{
		Repository: inmemory.NewRepository(),
		profile:    applicableProfile(),
		typeOverrides: map[string]*disclosureapp.DisclosureTypeDTO{
			"dt-bctc": workflowTypeDetail("dt-bctc", &applicability.TemplateApplicabilityRules{
				ApplicableCompanyClasses: []applicability.CompanyClass{applicability.CompanyClassListed},
				ApplicableSectors:        []applicability.BusinessSector{applicability.BusinessSectorCommercial},
				DeadlineByStructure: map[applicability.StructureCriterion]applicability.StructureDeadlineEntry{
					applicability.StructureHasSubsidiaries:     {Days: 30},
					applicability.StructureHasSubordinateUnits: {Days: 30},
				},
			}),
		},
	}
	svc := newE11Service(repo)

	_, err := svc.CreateRecord(context.Background(), baseCreateRecordRequest("dt-bctc"))
	if err == nil {
		t.Fatal("expected E11 error")
	}
	he, ok := perr.AsHTTPError(err)
	if !ok || he.Code != "DEADLINE_RULE_UNRESOLVED" {
		t.Fatalf("got %#v want DEADLINE_RULE_UNRESOLVED", err)
	}
}

func TestCreateRecord_StructureDeadlineResolvable_Succeeds(t *testing.T) {
	sector := applicability.BusinessSectorCommercial
	profile := applicability.CompanyApplicabilityProfile{
		IsListed:        true,
		HasSubsidiaries: true,
		BusinessSector:  &sector,
	}
	repo := &e11TestRepo{
		Repository: inmemory.NewRepository(),
		profile:    profile,
		typeOverrides: map[string]*disclosureapp.DisclosureTypeDTO{
			"dt-bctc": workflowTypeDetail("dt-bctc", fullStructureRules()),
		},
	}
	svc := newE11Service(repo)

	rec, err := svc.CreateRecord(context.Background(), baseCreateRecordRequest("dt-bctc"))
	if err != nil {
		t.Fatalf("CreateRecord: %v", err)
	}
	if rec.PlannedDate != "2026-06-30" {
		t.Fatalf("planned_date=%q want client value preserved", rec.PlannedDate)
	}
	days, ok := applicability.ResolveDeadlineDays(fullStructureRules(), profile)
	if !ok || days != 30 {
		t.Fatalf("resolve days=%d ok=%v want 30 true", days, ok)
	}
}

func TestCreateRecord_NoDeadlineByStructure_LegacyUnchanged(t *testing.T) {
	repo := &e11TestRepo{
		Repository: inmemory.NewRepository(),
		profile:    applicableProfile(),
		typeOverrides: map[string]*disclosureapp.DisclosureTypeDTO{
			"dt-irregular": workflowTypeDetail("dt-irregular", &applicability.TemplateApplicabilityRules{
				ApplicableCompanyClasses: []applicability.CompanyClass{applicability.CompanyClassListed},
				ApplicableSectors:        []applicability.BusinessSector{applicability.BusinessSectorCommercial},
			}),
		},
	}
	svc := newE11Service(repo)

	rec, err := svc.CreateRecord(context.Background(), baseCreateRecordRequest("dt-irregular"))
	if err != nil {
		t.Fatalf("CreateRecord: %v", err)
	}
	if rec.RecordID == "" {
		t.Fatal("expected record created")
	}
}

func TestCreateRecord_E11_SkippedForCompanyScopedTemplate(t *testing.T) {
	repo := &e11TestRepo{
		Repository: inmemory.NewRepository(),
		profile:    applicableProfile(),
		typeOverrides: map[string]*disclosureapp.DisclosureTypeDTO{
			"dt-custom": {
				TypeID: "dt-custom",
				Scope:  "company",
				ApplicabilityRules: &applicability.TemplateApplicabilityRules{
					DeadlineByStructure: map[applicability.StructureCriterion]applicability.StructureDeadlineEntry{
						applicability.StructureSimpleStructure: {Days: 20},
					},
				},
				Blocks: []disclosureapp.TemplateBlockDTO{
					{
						BlockKey:  "enterprise_workflow",
						BlockType: "rich_text",
						Enabled:   true,
						Config: map[string]any{
							"steps": []any{
								map[string]any{
									"step_id":           "s1",
									"stage":             "Review",
									"department_id":     "dept-1",
									"assignee_role_ids": []any{"role-1"},
									"processing_days":   2,
									"display_order":     1,
								},
							},
						},
					},
				},
			},
		},
	}
	svc := newE11Service(repo)

	rec, err := svc.CreateRecord(context.Background(), baseCreateRecordRequest("dt-custom"))
	if err != nil {
		t.Fatalf("company-scoped create should skip E11: %v", err)
	}
	if rec.RecordID == "" {
		t.Fatal("expected record created")
	}
}
