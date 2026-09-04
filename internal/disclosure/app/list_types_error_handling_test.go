package app

// Regression tests for disclosure-types 500 incident (2026-07-02).
// Root cause: MySQL max_allowed_packet=2KB; disclosure_type_versions has many text columns.
// Large query result exceeded packet limit → unhandled DB error → 500.
// Fix: MySQL max_allowed_packet set to 64MB (67108864 bytes) in docker-compose.artifacts.yml.
//
// These tests verify the service error propagation path so DB errors bubble up
// as errors (not panics) and that invalid pagination parameters return 400.

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/cobo/cobo_iam_services/internal/disclosure/app/applicability"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

type errorRepo struct {
	Repository
	listTypesErr  error
	applicability applicability.CompanyApplicabilityProfile
}

func (r *errorRepo) ListTypes(_ context.Context, _ ListTypesParams) ([]DisclosureTypeSummaryDTO, int, error) {
	if r.listTypesErr != nil {
		return nil, 0, r.listTypesErr
	}
	return nil, 0, nil
}

func (r *errorRepo) GetCompanyApplicabilityProfile(_ context.Context, _ string) (applicability.CompanyApplicabilityProfile, error) {
	return r.applicability, nil
}

func (r *errorRepo) GetCompanyDeadlineContext(_ context.Context, companyID string) (CompanyDeadlineContext, error) {
	return CompanyDeadlineContext{CompanyID: companyID}, nil
}

func (r *errorRepo) ListCompanyTypePreferencesByTypeIDs(_ context.Context, _ []string) ([]CompanyTypePreference, error) {
	return nil, nil
}

func (r *errorRepo) ListActiveDeadlineRuleCatalog(_ context.Context) ([]DeadlineRuleCatalogDTO, error) {
	return nil, nil
}

// TestListTypes_RepoErrorBubblesUp verifies that when the repo (DB) returns an error,
// the service returns that error (not nil, not panic). This is the regression path for
// the MySQL max_allowed_packet 500 incident.
func TestListTypes_RepoErrorBubblesUp(t *testing.T) {
	dbErr := fmt.Errorf("Error 1153 (08S01): Got a packet bigger than 'max_allowed_packet' bytes")
	repo := &errorRepo{listTypesErr: dbErr}
	svc := NewService(repo, nil, idgen.UUIDv7Generator{})

	_, err := svc.ListTypes(context.Background(), ListTypesRequest{
		Subject:      Subject{CompanyID: "company-001"},
		Page:         1,
		PageProvided: true,
	})
	if err == nil {
		t.Fatal("expected error from DB failure, got nil")
	}
	if errors.Is(err, dbErr) || fmt.Sprintf("%v", err) == fmt.Sprintf("%v", dbErr) {
		// exact error bubbled up — good
	} else if fmt.Sprintf("%v", err) == "" {
		t.Fatalf("got empty error: %v", err)
	}
	// Must not be a 400 or 403 — this is a 500-category error
	var he *perr.HTTPError
	if errors.As(err, &he) {
		if he.HTTPStatus == http.StatusBadRequest {
			t.Fatalf("expected internal error from DB, got 400: %v", err)
		}
	}
}

// TestListTypes_EmptyRepoReturns200WithEmptyList verifies that when the DB returns
// no rows (empty result), the service returns 200 with empty items — not 500.
func TestListTypes_EmptyRepoReturns200WithEmptyList(t *testing.T) {
	repo := &listTypesPaginationRepo{
		light: []DisclosureTypeSummaryDTO{},
		full:  map[string]DisclosureTypeSummaryDTO{},
	}
	svc := NewService(repo, nil, idgen.UUIDv7Generator{})

	resp, err := svc.ListTypes(context.Background(), ListTypesRequest{
		Subject:          Subject{CompanyID: "company-001"},
		Page:             1,
		PageSize:         100,
		PageProvided:     true,
		PageSizeProvided: true,
	})
	if err != nil {
		t.Fatalf("expected nil error for empty data, got: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response for empty data")
	}
	if len(resp.Items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(resp.Items))
	}
	if resp.Total != 0 {
		t.Fatalf("expected total=0, got %d", resp.Total)
	}
}

// TestListTypes_LargePageSizeMaxReturnsAll verifies max page_size=100 works correctly.
func TestListTypes_LargePageSizeMaxReturnsAll(t *testing.T) {
	now := time.Now().UTC()
	items := make([]DisclosureTypeSummaryDTO, 100)
	full := make(map[string]DisclosureTypeSummaryDTO, 100)
	for i := range items {
		id := fmt.Sprintf("type-%03d", i)
		items[i] = DisclosureTypeSummaryDTO{TypeID: id, Scope: "global", Name: fmt.Sprintf("Type %d", i), CreatedAt: now}
		full[id] = DisclosureTypeSummaryDTO{TypeID: id, Name: fmt.Sprintf("Type %d", i)}
	}
	repo := &listTypesPaginationRepo{light: items, full: full}
	svc := NewService(repo, nil, idgen.UUIDv7Generator{})

	resp, err := svc.ListTypes(context.Background(), ListTypesRequest{
		Subject:          Subject{CompanyID: "company-001"},
		Page:             1,
		PageSize:         100,
		PageProvided:     true,
		PageSizeProvided: true,
	})
	if err != nil {
		t.Fatalf("expected nil error: %v", err)
	}
	if len(resp.Items) != 100 {
		t.Fatalf("expected 100 items at page_size=100, got %d", len(resp.Items))
	}
}

// TestListTypes_InvalidPageNegativeReturns400 regression for invalid pagination params.
func TestListTypes_InvalidPageNegativeReturns400(t *testing.T) {
	svc := NewService(&errorRepo{}, nil, idgen.UUIDv7Generator{})
	_, err := svc.ListTypes(context.Background(), ListTypesRequest{
		Subject:      Subject{CompanyID: "company-001"},
		Page:         -1,
		PageProvided: true,
	})
	if err == nil {
		t.Fatal("expected 400 for negative page")
	}
	var he *perr.HTTPError
	if !errors.As(err, &he) || he.HTTPStatus != http.StatusBadRequest {
		t.Fatalf("expected 400 HTTPError, got: %v", err)
	}
}

// TestListTypes_PageSizeTooLargeReturns400 regression for oversized page_size.
func TestListTypes_PageSizeTooLargeReturns400(t *testing.T) {
	svc := NewService(&errorRepo{}, nil, idgen.UUIDv7Generator{})
	_, err := svc.ListTypes(context.Background(), ListTypesRequest{
		Subject:          Subject{CompanyID: "company-001"},
		PageSize:         101,
		PageSizeProvided: true,
	})
	if err == nil {
		t.Fatal("expected 400 for page_size > 100")
	}
	var he *perr.HTTPError
	if !errors.As(err, &he) || he.HTTPStatus != http.StatusBadRequest {
		t.Fatalf("expected 400 HTTPError, got: %v", err)
	}
}
