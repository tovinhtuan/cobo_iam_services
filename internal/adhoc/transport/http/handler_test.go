package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	adhocapp "github.com/cobo/cobo_iam_services/internal/adhoc/app"
	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
)

type fakeService struct {
	listReq adhocapp.ListProposalsRequest
}

func (f *fakeService) CreateProposal(context.Context, adhocapp.CreateProposalRequest) (*adhocapp.ProposalDTO, error) {
	return nil, nil
}

func (f *fakeService) SubmitProposal(context.Context, adhocapp.ProposalActionRequest) (*adhocapp.ProposalDTO, error) {
	return nil, nil
}

func (f *fakeService) FocalApprove(context.Context, adhocapp.ProposalActionRequest) (*adhocapp.ProposalDTO, error) {
	return nil, nil
}

func (f *fakeService) AdminApprove(context.Context, adhocapp.AdminApproveRequest) (*adhocapp.AdminApproveResponse, error) {
	return nil, nil
}

func (f *fakeService) Reject(context.Context, adhocapp.RejectRequest) (*adhocapp.ProposalDTO, error) {
	return nil, nil
}

func (f *fakeService) Cancel(context.Context, adhocapp.ProposalActionRequest) (*adhocapp.ProposalDTO, error) {
	return nil, nil
}

func (f *fakeService) GetProposal(context.Context, adhocapp.GetProposalRequest) (*adhocapp.ProposalDTO, error) {
	return nil, nil
}

func (f *fakeService) ListProposals(_ context.Context, req adhocapp.ListProposalsRequest) (*adhocapp.ListProposalsResponse, error) {
	f.listReq = req
	return &adhocapp.ListProposalsResponse{Items: []adhocapp.ProposalDTO{}, Page: req.Page, PageSize: req.PageSize, Total: 0}, nil
}

type fakeInspector struct{}

func (fakeInspector) InspectAccessToken(context.Context, string) (*iamapp.AccessTokenClaims, error) {
	return &iamapp.AccessTokenClaims{Sub: "user-001", MembershipID: "member-001", CompanyID: "company-001"}, nil
}

func (fakeInspector) InspectPreCompanyToken(context.Context, string) (*iamapp.PreCompanyTokenClaims, error) {
	return nil, nil
}

func TestListProposals_ParsesPagingQueryParams(t *testing.T) {
	svc := &fakeService{}
	handler := NewHandler(svc, fakeInspector{})
	mux := http.NewServeMux()
	handler.Register(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/company/ad-hoc-proposals?status=approved&page=2&page_size=10", nil)
	req.Header.Set("Authorization", "Bearer atk_test")
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	if got, want := svc.listReq.StatusFilter, []string{"approved"}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("expected status filter %v, got %v", want, got)
	}
	if svc.listReq.Page != 2 {
		t.Fatalf("expected page 2, got %d", svc.listReq.Page)
	}
	if svc.listReq.PageSize != 10 {
		t.Fatalf("expected page size 10, got %d", svc.listReq.PageSize)
	}
}
