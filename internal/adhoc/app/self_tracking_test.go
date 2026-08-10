package app

import (
	"context"
	"net/http"
	"testing"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// trackingListRepo is a minimal list/get fake for creator self-tracking tests.
type trackingListRepo struct {
	fakeRepository
	items               []ProposalDTO
	lastCreatedByFilter string
	lastStatusFilter    []string
	lastPage            int
	lastPageSize        int
}

func (r *trackingListRepo) FindByID(_ context.Context, companyID, proposalID string) (*ProposalDTO, error) {
	for i := range r.items {
		p := r.items[i]
		if p.CompanyID == companyID && p.ProposalID == proposalID {
			cp := p
			return &cp, nil
		}
	}
	if r.proposal != nil && r.proposal.CompanyID == companyID && r.proposal.ProposalID == proposalID {
		cp := *r.proposal
		return &cp, nil
	}
	return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "proposal not found", nil)
}

func (r *trackingListRepo) List(_ context.Context, companyID string, statusFilter []string, createdByMembershipID string, page, pageSize int) ([]ProposalDTO, int, error) {
	r.lastCreatedByFilter = createdByMembershipID
	r.lastStatusFilter = append([]string(nil), statusFilter...)
	r.lastPage = page
	r.lastPageSize = pageSize
	var matched []ProposalDTO
	for _, p := range r.items {
		if p.CompanyID != companyID {
			continue
		}
		if createdByMembershipID != "" && p.CreatedBy != createdByMembershipID {
			continue
		}
		if len(statusFilter) > 0 {
			ok := false
			for _, st := range statusFilter {
				if p.Status == st {
					ok = true
					break
				}
			}
			if !ok {
				continue
			}
		}
		matched = append(matched, p)
	}
	total := len(matched)
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	start := (page - 1) * pageSize
	if start >= total {
		return []ProposalDTO{}, total, nil
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return matched[start:end], total, nil
}

func proposeOnlyAuth() *fakeAuthService {
	return &fakeAuthService{allowByAction: map[string]authapp.Decision{
		"ad_hoc_alert.propose": authapp.DecisionAllow,
	}}
}

func readOnlyAuth() *fakeAuthService {
	return &fakeAuthService{allowByAction: map[string]authapp.Decision{
		"ad_hoc_alert.read": authapp.DecisionAllow,
	}}
}

func readAndProposeAuth() *fakeAuthService {
	return &fakeAuthService{allowByAction: map[string]authapp.Decision{
		"ad_hoc_alert.read":    authapp.DecisionAllow,
		"ad_hoc_alert.propose": authapp.DecisionAllow,
	}}
}

func TestGetProposal_ReadPermission_AllowsAnyInCompany(t *testing.T) {
	repo := &trackingListRepo{items: []ProposalDTO{{
		ProposalID: "p-other", CompanyID: "c1", CreatedBy: "m-other", Status: StatusPendingFocalApproval,
	}}}
	svc := NewService(repo, &fakeRecordCreator{}, &fakeTypeCatalog{category: "irregular"}, fakeIDGen{}, false, readOnlyAuth(), newAllowValidator(), nil, noopMetrics{})
	got, err := svc.GetProposal(context.Background(), GetProposalRequest{
		Subject: Subject{UserID: "u1", MembershipID: "m-reader", CompanyID: "c1"}, ProposalID: "p-other",
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got.ProposalID != "p-other" {
		t.Fatalf("got %#v", got)
	}
}

func TestGetProposal_ProposeOnly_OwnAllowed(t *testing.T) {
	repo := &trackingListRepo{items: []ProposalDTO{{
		ProposalID: "p-own", CompanyID: "c1", CreatedBy: "m-creator", Status: StatusPendingFocalApproval,
	}}}
	svc := NewService(repo, &fakeRecordCreator{}, &fakeTypeCatalog{category: "irregular"}, fakeIDGen{}, false, proposeOnlyAuth(), newAllowValidator(), nil, noopMetrics{})
	got, err := svc.GetProposal(context.Background(), GetProposalRequest{
		Subject: Subject{UserID: "u1", MembershipID: "m-creator", CompanyID: "c1"}, ProposalID: "p-own",
	})
	if err != nil {
		t.Fatalf("own detail should succeed: %v", err)
	}
	if got.ProposalID != "p-own" {
		t.Fatalf("got %#v", got)
	}
}

func TestGetProposal_ProposeOnly_OtherCreatorDenied(t *testing.T) {
	repo := &trackingListRepo{items: []ProposalDTO{{
		ProposalID: "p-other", CompanyID: "c1", CreatedBy: "m-other", Status: StatusPendingFocalApproval,
	}}}
	svc := NewService(repo, &fakeRecordCreator{}, &fakeTypeCatalog{category: "irregular"}, fakeIDGen{}, false, proposeOnlyAuth(), newAllowValidator(), nil, noopMetrics{})
	_, err := svc.GetProposal(context.Background(), GetProposalRequest{
		Subject: Subject{UserID: "u1", MembershipID: "m-creator", CompanyID: "c1"}, ProposalID: "p-other",
	})
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusForbidden {
		t.Fatalf("expected 403, got %v", err)
	}
}

func TestGetProposal_ProposeOnly_CrossCompanyNotFound(t *testing.T) {
	repo := &trackingListRepo{items: []ProposalDTO{{
		ProposalID: "p-own", CompanyID: "c1", CreatedBy: "m-creator", Status: StatusPendingFocalApproval,
	}}}
	svc := NewService(repo, &fakeRecordCreator{}, &fakeTypeCatalog{category: "irregular"}, fakeIDGen{}, false, proposeOnlyAuth(), newAllowValidator(), nil, noopMetrics{})
	_, err := svc.GetProposal(context.Background(), GetProposalRequest{
		Subject: Subject{UserID: "u1", MembershipID: "m-creator", CompanyID: "c2"}, ProposalID: "p-own",
	})
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusNotFound {
		t.Fatalf("expected 404 cross-tenant, got %v", err)
	}
}

func TestGetProposal_ProposeIsNotGlobalRead(t *testing.T) {
	// Same as other-creator denial — documents NO_PROPOSE_PERMISSION_AS_GLOBAL_READ.
	TestGetProposal_ProposeOnly_OtherCreatorDenied(t)
}

func TestListProposals_CompanyWide_RequiresRead(t *testing.T) {
	repo := &trackingListRepo{items: []ProposalDTO{{
		ProposalID: "p1", CompanyID: "c1", CreatedBy: "m-creator", Status: StatusDraft,
	}}}
	svc := NewService(repo, &fakeRecordCreator{}, &fakeTypeCatalog{category: "irregular"}, fakeIDGen{}, false, proposeOnlyAuth(), newAllowValidator(), nil, noopMetrics{})
	_, err := svc.ListProposals(context.Background(), ListProposalsRequest{
		Subject: Subject{UserID: "u1", MembershipID: "m-creator", CompanyID: "c1"},
	})
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusForbidden {
		t.Fatalf("propose-only without scope must be denied: %v", err)
	}
}

func TestListProposals_CompanyWide_ReadAllowed(t *testing.T) {
	repo := &trackingListRepo{items: []ProposalDTO{
		{ProposalID: "p1", CompanyID: "c1", CreatedBy: "m-a", Status: StatusDraft},
		{ProposalID: "p2", CompanyID: "c1", CreatedBy: "m-b", Status: StatusDraft},
	}}
	svc := NewService(repo, &fakeRecordCreator{}, &fakeTypeCatalog{category: "irregular"}, fakeIDGen{}, false, readOnlyAuth(), newAllowValidator(), nil, noopMetrics{})
	resp, err := svc.ListProposals(context.Background(), ListProposalsRequest{
		Subject: Subject{UserID: "u1", MembershipID: "m-reader", CompanyID: "c1"}, Page: 1, PageSize: 20,
	})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if resp.Total != 2 || len(resp.Items) != 2 {
		t.Fatalf("expected company-wide 2, got total=%d items=%d", resp.Total, len(resp.Items))
	}
	if repo.lastCreatedByFilter != "" {
		t.Fatalf("company-wide must not set created_by filter, got %q", repo.lastCreatedByFilter)
	}
}

func TestListProposals_ScopeMy_ProposeOnly(t *testing.T) {
	repo := &trackingListRepo{items: []ProposalDTO{
		{ProposalID: "p1", CompanyID: "c1", CreatedBy: "m-creator", Status: StatusDraft},
		{ProposalID: "p2", CompanyID: "c1", CreatedBy: "m-other", Status: StatusDraft},
		{ProposalID: "p3", CompanyID: "c1", CreatedBy: "m-creator", Status: StatusPendingFocalApproval},
	}}
	svc := NewService(repo, &fakeRecordCreator{}, &fakeTypeCatalog{category: "irregular"}, fakeIDGen{}, false, proposeOnlyAuth(), newAllowValidator(), nil, noopMetrics{})
	resp, err := svc.ListProposals(context.Background(), ListProposalsRequest{
		Subject: Subject{UserID: "u1", MembershipID: "m-creator", CompanyID: "c1"},
		Scope:   ListScopeMy, Page: 1, PageSize: 20,
	})
	if err != nil {
		t.Fatalf("scope=my should succeed: %v", err)
	}
	if repo.lastCreatedByFilter != "m-creator" {
		t.Fatalf("filter must come from auth membership, got %q", repo.lastCreatedByFilter)
	}
	if resp.Total != 2 || len(resp.Items) != 2 {
		t.Fatalf("expected own 2, got total=%d items=%d", resp.Total, len(resp.Items))
	}
	for _, it := range resp.Items {
		if it.CreatedBy != "m-creator" {
			t.Fatalf("leaked other creator: %#v", it)
		}
	}
}

func TestListProposals_ScopeMy_ReadUserOwnOnly(t *testing.T) {
	repo := &trackingListRepo{items: []ProposalDTO{
		{ProposalID: "p1", CompanyID: "c1", CreatedBy: "m-reader", Status: StatusDraft},
		{ProposalID: "p2", CompanyID: "c1", CreatedBy: "m-other", Status: StatusDraft},
	}}
	svc := NewService(repo, &fakeRecordCreator{}, &fakeTypeCatalog{category: "irregular"}, fakeIDGen{}, false, readOnlyAuth(), newAllowValidator(), nil, noopMetrics{})
	resp, err := svc.ListProposals(context.Background(), ListProposalsRequest{
		Subject: Subject{UserID: "u1", MembershipID: "m-reader", CompanyID: "c1"},
		Scope:   ListScopeMy, Page: 1, PageSize: 20,
	})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if resp.Total != 1 || len(resp.Items) != 1 || resp.Items[0].ProposalID != "p1" {
		t.Fatalf("scope=my must honor own-only for read user: %#v", resp)
	}
}

func TestListProposals_ScopeMy_StatusAndPaginationCount(t *testing.T) {
	repo := &trackingListRepo{items: []ProposalDTO{
		{ProposalID: "p1", CompanyID: "c1", CreatedBy: "m-creator", Status: StatusDraft},
		{ProposalID: "p2", CompanyID: "c1", CreatedBy: "m-creator", Status: StatusPendingFocalApproval},
		{ProposalID: "p3", CompanyID: "c1", CreatedBy: "m-creator", Status: StatusDraft},
		{ProposalID: "p4", CompanyID: "c1", CreatedBy: "m-other", Status: StatusDraft},
	}}
	svc := NewService(repo, &fakeRecordCreator{}, &fakeTypeCatalog{category: "irregular"}, fakeIDGen{}, false, proposeOnlyAuth(), newAllowValidator(), nil, noopMetrics{})
	resp, err := svc.ListProposals(context.Background(), ListProposalsRequest{
		Subject:      Subject{UserID: "u1", MembershipID: "m-creator", CompanyID: "c1"},
		Scope:        ListScopeMy,
		StatusFilter: []string{StatusDraft},
		Page:         1,
		PageSize:     1,
	})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if resp.Total != 2 {
		t.Fatalf("total must be self-scoped draft count=2, got %d", resp.Total)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("page_size=1 expected 1 item, got %d", len(resp.Items))
	}
	if resp.Items[0].Status != StatusDraft || resp.Items[0].CreatedBy != "m-creator" {
		t.Fatalf("bad item %#v", resp.Items[0])
	}
}

func TestListProposals_InvalidScopeRejected(t *testing.T) {
	svc := NewService(&trackingListRepo{}, &fakeRecordCreator{}, &fakeTypeCatalog{category: "irregular"}, fakeIDGen{}, false, proposeOnlyAuth(), newAllowValidator(), nil, noopMetrics{})
	for _, scope := range []string{"assigned", "foo", "ALL"} {
		_, err := svc.ListProposals(context.Background(), ListProposalsRequest{
			Subject: Subject{UserID: "u1", MembershipID: "m-creator", CompanyID: "c1"},
			Scope:   scope,
		})
		he, ok := perr.AsHTTPError(err)
		if !ok || he.HTTPStatus != http.StatusBadRequest {
			t.Fatalf("scope=%q expected 400, got %v", scope, err)
		}
	}
}

func TestListProposals_ScopeMy_NoPermissionDenied(t *testing.T) {
	auth := &fakeAuthService{allowByAction: map[string]authapp.Decision{}}
	svc := NewService(&trackingListRepo{}, &fakeRecordCreator{}, &fakeTypeCatalog{category: "irregular"}, fakeIDGen{}, false, auth, newAllowValidator(), nil, noopMetrics{})
	_, err := svc.ListProposals(context.Background(), ListProposalsRequest{
		Subject: Subject{UserID: "u1", MembershipID: "m1", CompanyID: "c1"},
		Scope:   ListScopeMy,
	})
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusForbidden {
		t.Fatalf("expected 403, got %v", err)
	}
}

func TestListProposals_ReadAndPropose_CompanyWidePreserved(t *testing.T) {
	repo := &trackingListRepo{items: []ProposalDTO{
		{ProposalID: "p1", CompanyID: "c1", CreatedBy: "m-a", Status: StatusDraft},
		{ProposalID: "p2", CompanyID: "c1", CreatedBy: "m-b", Status: StatusDraft},
	}}
	svc := NewService(repo, &fakeRecordCreator{}, &fakeTypeCatalog{category: "irregular"}, fakeIDGen{}, false, readAndProposeAuth(), newAllowValidator(), nil, noopMetrics{})
	resp, err := svc.ListProposals(context.Background(), ListProposalsRequest{
		Subject: Subject{UserID: "u1", MembershipID: "m-a", CompanyID: "c1"},
	})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if resp.Total != 2 {
		t.Fatalf("existing read capability must remain company-wide, got %d", resp.Total)
	}
}
