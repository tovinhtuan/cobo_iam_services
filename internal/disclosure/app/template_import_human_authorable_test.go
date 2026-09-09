package app_test

import (
	"context"
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	"github.com/cobo/cobo_iam_services/internal/disclosure/infra/inmemory"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

var uuidLike = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)

func TestHumanAuthorableExampleHasNoOpaqueIDs(t *testing.T) {
	payload := disclosureapp.CanonicalTemplateImportExampleV1()
	raw := string(payload)
	if uuidLike.MatchString(raw) {
		t.Fatal("canonical example must not contain UUID-like opaque IDs")
	}
	forbidden := []string{
		`"step_id"`,
		`"department_id"`,
		`"assignee_role_ids"`,
		`"template_file_id"`,
		`"company_id"`,
		`"template_key"`,
		`"group_id"`,
		`"display_group_codes"`,
	}
	for _, f := range forbidden {
		if strings.Contains(raw, f) {
			t.Fatalf("canonical example must not contain opaque/required-internal field %s", f)
		}
	}

	var env disclosureapp.TemplateImportEnvelopeV1
	if err := json.Unmarshal(payload, &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(env.Template.LegalBases) == 0 || env.Template.LegalBases[0].ID != "" {
		t.Fatalf("legal_bases[0].id must be empty in example, got %#v", env.Template.LegalBases)
	}
	if len(env.Template.Checklist) == 0 || env.Template.Checklist[0].ID != "" {
		t.Fatalf("checklist[0].id must be empty in example")
	}
	step := env.Template.Workflow.Steps[0]
	if step.Department == nil || step.Department.Code == "" || step.Department.Name == "" {
		t.Fatalf("expected department{code,name}, got %#v", step.Department)
	}
	if len(step.AssigneeRoles) == 0 {
		t.Fatal("expected assignee_roles in example")
	}
}

func TestDownloadedExampleValidates(t *testing.T) {
	_ = os.Setenv("CMS_TEMPLATE_IMPORT_SIGNING_SECRET", "test-signing-secret-for-human-authorable-32b")
	repo := inmemory.NewRepository()
	auth := &confirmAuthMock{permissions: []string{"platform.cms.view", "cms.template.write"}}
	svc := disclosureapp.NewService(repo, auth, idgen.UUIDv7Generator{})
	sub := disclosureapp.Subject{UserID: "u-ha", MembershipID: "m1", CompanyID: "c1"}

	resp, err := svc.ValidateTemplateImport(context.Background(), disclosureapp.ValidateTemplateImportRequest{
		Subject:   sub,
		Filename:  disclosureapp.TemplateImportExampleFilename,
		FileBytes: disclosureapp.CanonicalTemplateImportExampleV1(),
	})
	if err != nil {
		t.Fatalf("ValidateTemplateImport: %v", err)
	}
	if !resp.ParseValid || !resp.DomainValid {
		t.Fatalf("expected parse/domain valid, parse=%v domain=%v errs=%#v", resp.ParseValid, resp.DomainValid, resp.Errors)
	}
	if !resp.MappingRequired {
		t.Fatal("expected mapping_required=true for portable illustrative departments")
	}
	for _, m := range resp.RequiredMappings {
		if strings.TrimSpace(m.SourceID) == "" {
			t.Fatalf("mapping source_id must be non-empty for FE keys: %#v", m)
		}
		if strings.Contains(m.SourceID, "-") && uuidLike.MatchString(m.SourceID) {
			t.Fatalf("mapping must not expose UUID source_id: %q", m.SourceID)
		}
	}
}

func TestServerOwnedIDsGeneratedAtConfirm(t *testing.T) {
	_ = os.Setenv("CMS_TEMPLATE_IMPORT_SIGNING_SECRET", "test-signing-secret-for-human-authorable-32b")
	repo := inmemory.NewRepository()
	auth := &confirmAuthMock{permissions: []string{"platform.cms.view", "cms.template.write"}}
	svc := disclosureapp.NewService(repo, auth, idgen.UUIDv7Generator{})
	sub := disclosureapp.Subject{UserID: "u-ha", MembershipID: "m1", CompanyID: "c1"}

	payload := disclosureapp.CanonicalTemplateImportExampleV1()
	vresp, err := svc.ValidateTemplateImport(context.Background(), disclosureapp.ValidateTemplateImportRequest{
		Subject: sub, Filename: "example.json", FileBytes: payload,
	})
	if err != nil || !vresp.DomainValid || vresp.Preview == nil || vresp.Preview.NormalizedTemplate == nil {
		t.Fatalf("validate failed: err=%v resp=%#v", err, vresp)
	}

	mappings := map[string]string{}
	for _, m := range vresp.RequiredMappings {
		// Map illustrative sources onto seeded catalog codes.
		switch {
		case strings.Contains(strings.ToLower(m.SourceName), "tài chính") || m.SourceID == "finance":
			mappings[m.SourceID] = "dept-003"
		case strings.Contains(strings.ToLower(m.SourceName), "pháp chế") || m.SourceID == "legal":
			mappings[m.SourceID] = "dept-001"
		default:
			mappings[m.SourceID] = "dept-002"
		}
	}

	cresp, err := svc.ConfirmTemplateImport(context.Background(), disclosureapp.ConfirmTemplateImportRequest{
		Subject:            sub,
		ValidationToken:    vresp.ValidationToken,
		TargetTypeID:       "qa-human-authorable-example",
		TargetName:         vresp.Preview.NormalizedTemplate.Name,
		DepartmentMappings: mappings,
		NormalizedTemplate: *vresp.Preview.NormalizedTemplate,
	})
	if err != nil {
		t.Fatalf("ConfirmTemplateImport: %v", err)
	}
	if cresp.VersionNo != 1 || cresp.IsActive {
		t.Fatalf("unexpected lifecycle: %#v", cresp)
	}

	got, err := repo.GetTypeVersionDetail(context.Background(), "", cresp.TypeID, 1)
	if err != nil || got == nil {
		t.Fatalf("GetTypeVersionDetail: %v", err)
	}
	steps := disclosureapp.ExtractTemplateWorkflow(got.Blocks)
	if len(steps) != 3 {
		t.Fatalf("expected 3 persisted steps, got %d", len(steps))
	}
	var foundStepUUID, foundDocUUID, foundChecklistUUID bool
	seenStep := map[string]struct{}{}
	for _, st := range steps {
		if uuidLike.MatchString(st.StepID) {
			foundStepUUID = true
			if _, dup := seenStep[st.StepID]; dup {
				t.Fatalf("duplicate step uuid %q", st.StepID)
			}
			seenStep[st.StepID] = struct{}{}
		}
		for _, d := range st.Documents {
			if uuidLike.MatchString(d.DocID) {
				foundDocUUID = true
			}
			if d.TemplateFileID != "" {
				t.Fatalf("template_file_id must stay empty, got %q", d.TemplateFileID)
			}
		}
	}
	for _, item := range got.Checklist {
		if uuidLike.MatchString(item.ID) {
			foundChecklistUUID = true
		}
	}
	for _, lb := range got.LegalBases {
		if !uuidLike.MatchString(lb.ID) {
			t.Fatalf("legal basis id must be server UUID, got %q", lb.ID)
		}
	}
	if !foundStepUUID || !foundDocUUID || !foundChecklistUUID {
		t.Fatalf("expected server-owned UUIDs step=%v doc=%v checklist=%v", foundStepUUID, foundDocUUID, foundChecklistUUID)
	}
}

func TestDepartmentNameCodeMapping(t *testing.T) {
	_ = os.Setenv("CMS_TEMPLATE_IMPORT_SIGNING_SECRET", "test-signing-secret-for-human-authorable-32b")
	repo := inmemory.NewRepository()
	auth := &confirmAuthMock{permissions: []string{"platform.cms.view", "cms.template.write"}}
	svc := disclosureapp.NewService(repo, auth, idgen.UUIDv7Generator{})
	sub := disclosureapp.Subject{UserID: "u-ha", MembershipID: "m1", CompanyID: "c1"}

	env := disclosureapp.TemplateImportEnvelopeV1{
		SchemaVersion: "1.0",
		Template: disclosureapp.TemplateImportDefinitionV1{
			Name:             "Dept mapping sample",
			TemplateCategory: "periodic",
			DeadlineRule:     "T+5",
			Periodicity:      "quarterly",
			Workflow: &disclosureapp.TemplateImportWorkflowV1{
				Steps: []disclosureapp.TemplateImportWorkflowStepV1{
					{
						Stage: "Soạn thảo",
						Department: &disclosureapp.TemplateImportDepartmentRefV1{
							Code: "finance",
							Name: "Phòng Tài chính - Kế toán",
						},
						AssigneeRoles:  []string{"REVIEWER"},
						ProcessingDays: 2,
					},
					{
						Stage:          "Phê duyệt",
						DepartmentName: "Phòng Không Có Trong Catalog",
						AssigneeRoles:  []string{"APPROVER"},
						ProcessingDays: 1,
					},
				},
			},
		},
	}
	raw, _ := json.Marshal(env)
	resp, err := svc.ValidateTemplateImport(context.Background(), disclosureapp.ValidateTemplateImportRequest{
		Subject: sub, Filename: "dept.json", FileBytes: raw,
	})
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if !resp.DomainValid {
		t.Fatalf("domain invalid: %#v", resp.Errors)
	}
	if len(resp.RequiredMappings) < 2 {
		t.Fatalf("expected mappings for code+name unresolved depts, got %#v", resp.RequiredMappings)
	}
	foundCodeKey, foundNameKey := false, false
	for _, m := range resp.RequiredMappings {
		if m.SourceID == "" {
			t.Fatalf("empty source_id: %#v", m)
		}
		if m.SourceID == "finance" {
			foundCodeKey = true
		}
		if strings.HasPrefix(m.SourceID, "dept-name:") {
			foundNameKey = true
		}
	}
	if !foundCodeKey || !foundNameKey {
		t.Fatalf("expected finance + dept-name keys, got %#v", resp.RequiredMappings)
	}
}

func TestStaticRoleCodeValidation(t *testing.T) {
	_ = os.Setenv("CMS_TEMPLATE_IMPORT_SIGNING_SECRET", "test-signing-secret-for-human-authorable-32b")
	repo := inmemory.NewRepository()
	auth := &confirmAuthMock{permissions: []string{"platform.cms.view", "cms.template.write"}}
	svc := disclosureapp.NewService(repo, auth, idgen.UUIDv7Generator{})
	sub := disclosureapp.Subject{UserID: "u-ha", MembershipID: "m1", CompanyID: "c1"}

	env := disclosureapp.TemplateImportEnvelopeV1{
		SchemaVersion: "1.0",
		Template: disclosureapp.TemplateImportDefinitionV1{
			Name:             "Role codes",
			TemplateCategory: "periodic",
			DeadlineRule:     "T+5",
			Periodicity:      "quarterly",
			Workflow: &disclosureapp.TemplateImportWorkflowV1{
				Steps: []disclosureapp.TemplateImportWorkflowStepV1{
					{
						Stage: "Draft",
						Department: &disclosureapp.TemplateImportDepartmentRefV1{
							Code: "dept-001",
							Name: "Phòng Pháp chế",
						},
						AssigneeRoles:  []string{"MAKER", "REVIEWER"},
						ProcessingDays: 2,
					},
					{
						Stage: "Approve",
						Department: &disclosureapp.TemplateImportDepartmentRefV1{
							Code: "dept-002",
						},
						AssigneeRoles:  []string{"APPROVER"},
						ProcessingDays: 1,
					},
				},
			},
		},
	}
	raw, _ := json.Marshal(env)
	resp, err := svc.ValidateTemplateImport(context.Background(), disclosureapp.ValidateTemplateImportRequest{
		Subject: sub, Filename: "roles.json", FileBytes: raw,
	})
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if !resp.DomainValid {
		t.Fatalf("expected static aliases to validate, errs=%#v", resp.Errors)
	}
	roles := resp.Preview.NormalizedTemplate.Workflow.Steps[0].AssigneeRoleIDs
	joined := strings.Join(roles, ",")
	if !strings.Contains(joined, "creator") || !strings.Contains(joined, "reviewer") {
		t.Fatalf("expected MAKER→creator and REVIEWER→reviewer, got %v", roles)
	}
}

func TestUnknownTenantReferenceProducesMapping(t *testing.T) {
	_ = os.Setenv("CMS_TEMPLATE_IMPORT_SIGNING_SECRET", "test-signing-secret-for-human-authorable-32b")
	repo := inmemory.NewRepository()
	auth := &confirmAuthMock{permissions: []string{"platform.cms.view", "cms.template.write"}}
	svc := disclosureapp.NewService(repo, auth, idgen.UUIDv7Generator{})
	sub := disclosureapp.Subject{UserID: "u-ha", MembershipID: "m1", CompanyID: "c1"}

	env := disclosureapp.TemplateImportEnvelopeV1{
		SchemaVersion: "1.0",
		Template: disclosureapp.TemplateImportDefinitionV1{
			Name:             "Unknown dept",
			TemplateCategory: "periodic",
			DeadlineRule:     "T+5",
			Periodicity:      "quarterly",
			Workflow: &disclosureapp.TemplateImportWorkflowV1{
				Steps: []disclosureapp.TemplateImportWorkflowStepV1{
					{
						Stage: "Step",
						Department: &disclosureapp.TemplateImportDepartmentRefV1{
							Code: "unknown-dept-xyz",
							Name: "Phòng Không Tồn Tại",
						},
						AssigneeRoles:  []string{"approver"},
						ProcessingDays: 1,
					},
				},
			},
		},
	}
	raw, _ := json.Marshal(env)
	resp, err := svc.ValidateTemplateImport(context.Background(), disclosureapp.ValidateTemplateImportRequest{
		Subject: sub, Filename: "unknown.json", FileBytes: raw,
	})
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if !resp.MappingRequired {
		t.Fatal("expected mapping_required")
	}
	if len(resp.RequiredMappings) != 1 || resp.RequiredMappings[0].SourceID != "unknown-dept-xyz" {
		t.Fatalf("unexpected mappings %#v", resp.RequiredMappings)
	}
}

func TestNoRawDatabaseIDRequired(t *testing.T) {
	key := disclosureapp.DepartmentMappingSourceKey("", "Phòng Tài chính")
	if key != "dept-name:phòng tài chính" {
		t.Fatalf("mapping key=%q", key)
	}
	if disclosureapp.DepartmentMappingSourceKey("finance", "X") != "finance" {
		t.Fatal("code should win over name for mapping key")
	}
}
