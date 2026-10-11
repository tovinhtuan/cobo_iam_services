package app

import (
	"context"
	"errors"
	"net/http"
	"testing"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
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
