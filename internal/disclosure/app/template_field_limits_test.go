package app

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

type descriptionLimitRepo struct {
	Repository
	lastReq UpsertTypeVersionRequest
}

func (r *descriptionLimitRepo) ListActiveDeadlineRuleCatalog(_ context.Context) ([]DeadlineRuleCatalogDTO, error) {
	return defaultDeadlineRuleCatalog(), nil
}

func (r *descriptionLimitRepo) ListDisplayGroups(_ context.Context) ([]DisplayGroupDTO, error) {
	return []DisplayGroupDTO{{DisplayGroupCode: "display_groups_001", NameVI: "Test", IsActive: true}}, nil
}

func (r *descriptionLimitRepo) UpsertTypeVersion(_ context.Context, req UpsertTypeVersionRequest) (*UpsertTypeVersionResponse, error) {
	r.lastReq = req
	return &UpsertTypeVersionResponse{TypeID: req.TypeID, VersionNo: 1, IsActive: true, UpdatedBy: req.Subject.UserID}, nil
}

func TestUpsertTypeVersion_RejectsDescriptionOverMaxLength(t *testing.T) {
	repo := &descriptionLimitRepo{}
	svc := newCMSUpsertDeadlineService(repo)
	req := baseUpsertRequest()
	req.DisplayGroupCodes = []string{"display_groups_001"}
	req.Description = strings.Repeat("x", MaxTemplateDescriptionLength+1)

	_, err := svc.UpsertTypeVersion(context.Background(), req)
	if err == nil {
		t.Fatal("expected validation error")
	}
	var he *perr.HTTPError
	if !errors.As(err, &he) || he.HTTPStatus != http.StatusBadRequest {
		t.Fatalf("expected 400 validation, got %v", err)
	}
	if repo.lastReq.TypeID != "" {
		t.Fatal("repository should not be called when description exceeds max length")
	}
}

func TestUpsertTypeVersion_AcceptsDescriptionAtMaxLength(t *testing.T) {
	repo := &descriptionLimitRepo{}
	svc := newCMSUpsertDeadlineService(repo)
	req := baseUpsertRequest()
	req.DisplayGroupCodes = []string{"display_groups_001"}
	req.DeadlineRule = "T+20"
	req.Description = strings.Repeat("x", MaxTemplateDescriptionLength)

	resp, err := svc.UpsertTypeVersion(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil || resp.TypeID == "" {
		t.Fatal("expected successful upsert")
	}
}

func TestMapRepositoryUpsertError_DataTooLong(t *testing.T) {
	err := mapRepositoryUpsertError(errors.New("Error 1406: Data too long for column 'description'"))
	var he *perr.HTTPError
	if !errors.As(err, &he) || he.HTTPStatus != http.StatusBadRequest {
		t.Fatalf("expected 400 mapped error, got %v", err)
	}
}

func TestValidateTemplateDescription(t *testing.T) {
	if err := validateTemplateDescription(strings.Repeat("a", MaxTemplateDescriptionLength)); err != nil {
		t.Fatalf("at byte limit should pass: %v", err)
	}
	if err := validateTemplateDescription(strings.Repeat("a", MaxTemplateDescriptionLength+1)); err == nil {
		t.Fatal("over byte limit should fail")
	}
	// Multi-byte UTF-8 can exceed byte limit before rune count.
	multiByte := strings.Repeat("€", 1100)
	if err := validateTemplateDescription(multiByte); err == nil {
		t.Fatal("expected byte-limit failure for multi-byte description")
	}
}

func TestValidateTemplateBlockDescriptionLengths(t *testing.T) {
	fieldErrors := map[string]string{}
	req := &UpsertTypeVersionRequest{
		Blocks: []TemplateBlockDTO{{
			BlockKey: "deadline", BlockType: "text", Title: "DL", Description: strings.Repeat("x", 4001),
			Config: map[string]any{"max_length": 4000}, Validation: map[string]any{}, DisplayOrder: 1,
		}},
	}
	validateTemplateBlocks(req, fieldErrors)
	validateTemplateBlockDescriptionLengths(req.Blocks, fieldErrors)
	if len(fieldErrors) == 0 {
		t.Fatal("expected block description length error")
	}
}
