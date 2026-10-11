package app

import (
	"context"
	"errors"
	"net/http"
	"testing"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

// preserveRepo extends upsertDeadlineRepo with a lifecycle + an open draft candidate.
type preserveRepo struct {
	upsertDeadlineRepo
	lifecycle    *TypeLifecycleDTO
	lifecycleErr error
	candidate    *DisclosureTypeDTO
	lastUpsert   UpsertTypeVersionRequest
}

func (r *preserveRepo) GetTypeLifecycle(_ context.Context, typeID string) (*TypeLifecycleDTO, error) {
	if r.lifecycleErr != nil {
		return nil, r.lifecycleErr
	}
	if r.lifecycle == nil {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "disclosure type not found", nil)
	}
	out := *r.lifecycle
	out.TypeID = typeID
	return &out, nil
}

func (r *preserveRepo) ListTypeVersions(context.Context, string, string) ([]DisclosureTypeVersionDTO, error) {
	if r.candidate == nil {
		return nil, nil
	}
	return []DisclosureTypeVersionDTO{{VersionNo: r.candidate.VersionNo}}, nil
}

func (r *preserveRepo) GetTypeVersionDetail(_ context.Context, _, _ string, versionNo int) (*DisclosureTypeDTO, error) {
	if r.candidate == nil || r.candidate.VersionNo != versionNo {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "not found", nil)
	}
	out := *r.candidate
	return &out, nil
}

func (r *preserveRepo) UpsertTypeVersion(ctx context.Context, req UpsertTypeVersionRequest) (*UpsertTypeVersionResponse, error) {
	r.lastUpsert = req
	return r.upsertDeadlineRepo.UpsertTypeVersion(ctx, req)
}

func TestUpsertTypeVersion_RejectsArchivedGlobalTemplate(t *testing.T) {
	repo := &preserveRepo{lifecycle: &TypeLifecycleDTO{Status: "archived"}}
	_, err := newCMSUpsertDeadlineService(repo).UpsertTypeVersion(context.Background(), baseUpsertRequest())
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusConflict || he.Code != "TEMPLATE_ARCHIVED" {
		t.Fatalf("want 409 TEMPLATE_ARCHIVED, got %v", err)
	}
	if repo.upsertCalled {
		t.Fatal("repository upsert must not run for archived template")
	}
}

func TestUpsertTypeVersion_AllowsActiveOrNewOrCompanyArchived(t *testing.T) {
	cases := map[string]*TypeLifecycleDTO{
		"new type (404)":          nil,
		"active global":           {Status: "active", ActiveVersionNo: 1},
		"archived company scoped": {Status: "archived", CompanyID: "c-001"},
	}
	for name, life := range cases {
		t.Run(name, func(t *testing.T) {
			repo := &preserveRepo{lifecycle: life}
			if _, err := newCMSUpsertDeadlineService(repo).UpsertTypeVersion(context.Background(), baseUpsertRequest()); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !repo.upsertCalled {
				t.Fatal("expected repository upsert")
			}
		})
	}
}

func TestUpsertTypeVersion_PreservesOmittedNarrativeFields(t *testing.T) {
	repo := &preserveRepo{
		lifecycle: &TypeLifecycleDTO{Status: "active", ActiveVersionNo: 2},
		candidate: &DisclosureTypeDTO{
			VersionNo: 2, Applicability: "old-app", ImplementationNotes: "old-notes", SpecialCases: "old-special",
			Beneficiaries: "old-ben", ReceivingAuthorities: "old-auth",
		},
	}
	req := baseUpsertRequest()
	req.SpecialCases = "" // explicitly cleared by client (key present)
	req.Beneficiaries = "new-ben"
	req.NarrativeFieldsOmitted = []string{"applicability", "implementation_notes", "receiving_authorities"}
	if _, err := newCMSUpsertDeadlineService(repo).UpsertTypeVersion(context.Background(), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := repo.lastUpsert
	if got.Applicability != "old-app" || got.ImplementationNotes != "old-notes" || got.ReceivingAuthorities != "old-auth" {
		t.Fatalf("omitted fields not preserved: %+v", got)
	}
	if got.SpecialCases != "" {
		t.Fatalf("explicit clear must win, got %q", got.SpecialCases)
	}
	if got.Beneficiaries != "new-ben" {
		t.Fatalf("explicit value must win, got %q", got.Beneficiaries)
	}
}

// listErrRepo fails the candidate lookup with an infrastructure error.
type listErrRepo struct {
	preserveRepo
}

func (r *listErrRepo) ListTypeVersions(context.Context, string, string) ([]DisclosureTypeVersionDTO, error) {
	return nil, errors.New("db timeout")
}

func TestUpsertTypeVersion_PreserveLookupFailureAbortsWrite(t *testing.T) {
	repo := &listErrRepo{preserveRepo{lifecycle: &TypeLifecycleDTO{Status: "active", ActiveVersionNo: 1}}}
	req := baseUpsertRequest()
	req.NarrativeFieldsOmitted = []string{"applicability"}
	if _, err := newCMSUpsertDeadlineService(repo).UpsertTypeVersion(context.Background(), req); err == nil {
		t.Fatal("expected error: writing would erase omitted narrative fields")
	}
	if repo.upsertCalled {
		t.Fatal("repository upsert must not run when preserve lookup fails")
	}
}

func TestUpsertTypeVersion_CreateOnlyDoesNotPreserve(t *testing.T) {
	repo := &preserveRepo{candidate: &DisclosureTypeDTO{VersionNo: 1, Applicability: "other-template"}}
	req := baseUpsertRequest()
	req.CreateOnly = true
	req.NarrativeFieldsOmitted = []string{"applicability"}
	if _, err := newCMSUpsertDeadlineService(repo).UpsertTypeVersion(context.Background(), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.lastUpsert.Applicability != "" {
		t.Fatalf("CreateOnly must not copy fields, got %q", repo.lastUpsert.Applicability)
	}
}

func TestUpsertTypeVersion_DerivesDeadlineDurationTypeFromApplicability(t *testing.T) {
	cases := []struct {
		dayType   string
		clientVal string
		want      string
	}{
		{dayType: "working", clientVal: "", want: "WORKING_DAYS"},
		{dayType: "calendar", clientVal: "WORKING_DAYS", want: "CALENDAR_DAYS"},
		{dayType: "", clientVal: "", want: "CALENDAR_DAYS"},
	}
	for _, tc := range cases {
		repo := &preserveRepo{lifecycle: &TypeLifecycleDTO{Status: "active", ActiveVersionNo: 1}}
		req := baseUpsertRequest()
		req.ApplicabilityRules.DeadlineDayType = tc.dayType
		req.DeadlineConfig = &TemplateDeadlineConfig{DeadlineDays: 10, DeadlineDurationType: tc.clientVal}
		if _, err := newCMSUpsertDeadlineService(repo).UpsertTypeVersion(context.Background(), req); err != nil {
			t.Fatalf("day_type=%q: unexpected error: %v", tc.dayType, err)
		}
		if got := repo.lastUpsert.DeadlineConfig.DeadlineDurationType; got != tc.want {
			t.Fatalf("day_type=%q client=%q: got %q want %q", tc.dayType, tc.clientVal, got, tc.want)
		}
	}
}

func TestUpsertTypeVersion_KeepsDurationTypeWithoutApplicabilityRules(t *testing.T) {
	repo := &preserveRepo{lifecycle: &TypeLifecycleDTO{Status: "active", ActiveVersionNo: 1}}
	req := baseUpsertRequest()
	req.Scope = "company"
	req.TemplateCategory = TemplateCategoryCustom
	req.ApplicabilityRules = nil
	req.SkipPublicationMatrix = true
	req.DeadlineConfig = &TemplateDeadlineConfig{DeadlineDays: 5, DeadlineDurationType: "WORKING_DAYS"}
	svc := NewService(repo, &fakeAuthService{permissions: []string{permissionPlatformCMSView, permissionCMSTemplateWrite}}, idgen.UUIDv7Generator{})
	if _, err := svc.UpsertTypeVersion(context.Background(), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := repo.lastUpsert.DeadlineConfig.DeadlineDurationType; got != "WORKING_DAYS" {
		t.Fatalf("without rules the client value must be kept, got %q", got)
	}
}

// configScopeRepo serves GET /config lookups.
type configScopeRepo struct {
	preserveRepo
}

func (r *configScopeRepo) GetActiveVersionDeadlineConfig(context.Context, string) (int, *TemplateDeadlineConfig, error) {
	return 2, &TemplateDeadlineConfig{DeadlineDays: 7}, nil
}

func TestGetTemplateDeadlineConfig_HidesOtherCompanyTemplate(t *testing.T) {
	svc := func(life *TypeLifecycleDTO) Service {
		return NewService(&configScopeRepo{preserveRepo{lifecycle: life}}, &fakeAuthService{
			permissions: []string{permissionPlatformCMSView, permissionCMSTemplateWrite},
		}, idgen.UUIDv7Generator{})
	}
	sub := Subject{UserID: "u", MembershipID: "m", CompanyID: "c-001"}
	_, err := svc(&TypeLifecycleDTO{Status: "active", CompanyID: "c-999"}).GetTemplateDeadlineConfig(context.Background(), GetTemplateDeadlineConfigRequest{Subject: sub, TypeID: "t"})
	if he, ok := perr.AsHTTPError(err); !ok || he.HTTPStatus != http.StatusNotFound {
		t.Fatalf("other company template must be 404, got %v", err)
	}
	for _, life := range []*TypeLifecycleDTO{{Status: "active"}, {Status: "active", CompanyID: "c-001"}} {
		out, err := svc(life).GetTemplateDeadlineConfig(context.Background(), GetTemplateDeadlineConfigRequest{Subject: sub, TypeID: "t"})
		if err != nil || out.DeadlineConfig.DeadlineDays != 7 {
			t.Fatalf("company=%q: want config, got %v %+v", life.CompanyID, err, out)
		}
	}
}

func TestCandidateHashFromRevision_RequiresPrefix(t *testing.T) {
	if got := CandidateHashFromRevision("pc1:abc"); got != "abc" {
		t.Fatalf("got %q", got)
	}
	if got := CandidateHashFromRevision("abc"); got == "abc" {
		t.Fatal("a bare hash must never match a stored hash")
	}
}
