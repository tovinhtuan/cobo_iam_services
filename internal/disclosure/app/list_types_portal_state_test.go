package app

import (
	"context"
	"net/http"
	"testing"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

func TestListTypes_InvalidPortalState_400(t *testing.T) {
	repo := &listTypesPaginationRepo{}
	svc := newListTypesPaginationService(repo)
	_, err := svc.ListTypes(context.Background(), ListTypesRequest{
		Subject:     Subject{CompanyID: "c1"},
		PortalState: "foo",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	httpErr, ok := err.(*perr.HTTPError)
	if !ok || httpErr.HTTPStatus != http.StatusBadRequest {
		t.Fatalf("want 400, got %#v", err)
	}
}

func TestListTypes_PassesPortalStateAndHasOpenDraft(t *testing.T) {
	repo := &listTypesPaginationRepo{
		light: []DisclosureTypeSummaryDTO{{TypeID: "t-1", Scope: "global", Name: "A"}},
		full:  map[string]DisclosureTypeSummaryDTO{"t-1": {TypeID: "t-1", Name: "A"}},
	}
	svc := newListTypesPaginationService(repo)
	yes := true
	_, err := svc.ListTypes(context.Background(), ListTypesRequest{
		Subject:          Subject{CompanyID: "c1"},
		PortalState:      PortalStateActive,
		HasOpenDraft:     &yes,
		Page:             1,
		PageSize:         20,
		PageProvided:     true,
		PageSizeProvided: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(repo.calls) == 0 {
		t.Fatal("expected repo call")
	}
	first := repo.calls[0]
	if first.PortalState != PortalStateActive {
		t.Fatalf("portal_state=%q", first.PortalState)
	}
	if first.HasOpenDraft == nil || !*first.HasOpenDraft {
		t.Fatalf("has_open_draft=%v", first.HasOpenDraft)
	}
}
