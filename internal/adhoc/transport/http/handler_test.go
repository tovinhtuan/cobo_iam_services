package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	adhocapp "github.com/cobo/cobo_iam_services/internal/adhoc/app"
	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	"github.com/cobo/cobo_iam_services/internal/platform/idempotency"
)

type fakeService struct {
	listReq           adhocapp.ListProposalsRequest
	lastAdminApprove  adhocapp.AdminApproveRequest
	adminApproveCalls int
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

type fakeIdemStore struct {
	tryResult      idempotency.Result
	tryParams      idempotency.Params
	completeCalled bool
	abandonCalled  bool
}

func (f *fakeIdemStore) TryReserve(_ context.Context, p idempotency.Params) (idempotency.Result, error) {
	f.tryParams = p
	return f.tryResult, nil
}

func (f *fakeIdemStore) Complete(_ context.Context, _ string, _ []byte) error {
	f.completeCalled = true
	return nil
}

func (f *fakeIdemStore) Abandon(_ context.Context, _ string) error {
	f.abandonCalled = true
	return nil
}

func (fakeInspector) InspectAccessToken(context.Context, string) (*iamapp.AccessTokenClaims, error) {
	return &iamapp.AccessTokenClaims{Sub: "user-001", MembershipID: "member-001", CompanyID: "company-001"}, nil
}

func (fakeInspector) InspectPreCompanyToken(context.Context, string) (*iamapp.PreCompanyTokenClaims, error) {
	return nil, nil
}

func TestListProposals_ParsesPagingQueryParams(t *testing.T) {
	svc := &fakeService{}
	handler := NewHandler(svc, fakeInspector{}, nil)
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

func (f *fakeService) AdminApprove(ctx context.Context, req adhocapp.AdminApproveRequest) (*adhocapp.AdminApproveResponse, error) {
	f.lastAdminApprove = req
	f.adminApproveCalls++
	return &adhocapp.AdminApproveResponse{
		Proposal:           adhocapp.ProposalDTO{ProposalID: req.ProposalID, CompanyID: req.Subject.CompanyID, Status: adhocapp.StatusApproved, RecordID: "record-001", WorkflowInstanceID: "wf-001"},
		RecordID:           "record-001",
		WorkflowInstanceID: "wf-001",
	}, nil
}

func TestAdminApprove_UsesFallbackIdempotencyKeyAndCompletesReservation(t *testing.T) {
	svc := &fakeService{}
	idem := &fakeIdemStore{tryResult: idempotency.Result{ReservationID: "idem-001"}}
	handler := NewHandler(svc, fakeInspector{}, idem)
	mux := http.NewServeMux()
	handler.Register(mux)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/company/ad-hoc-proposals/proposal-001/admin-approve", strings.NewReader(`{"final_t0_date":"2026-06-01","adjustment_note":"ok"}`))
	req.Header.Set("Authorization", "Bearer atk_test")
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	if svc.adminApproveCalls != 1 {
		t.Fatalf("expected AdminApprove to be called once, got %d", svc.adminApproveCalls)
	}
	if strings.TrimSpace(svc.lastAdminApprove.IdempotencyKey) == "" {
		t.Fatal("expected fallback idempotency key to be set")
	}
	if idem.tryParams.Scope != "adhoc.admin_approve" {
		t.Fatalf("expected scope adhoc.admin_approve, got %q", idem.tryParams.Scope)
	}
	if idem.tryParams.Key != svc.lastAdminApprove.IdempotencyKey {
		t.Fatalf("expected reserved key %q, got %q", svc.lastAdminApprove.IdempotencyKey, idem.tryParams.Key)
	}
	if !idem.completeCalled {
		t.Fatal("expected idempotency reservation to be completed")
	}
}

func TestAdminApprove_ReplaysCompletedIdempotentResponse(t *testing.T) {
	svc := &fakeService{}
	body, _ := json.Marshal(&adhocapp.AdminApproveResponse{
		Proposal:           adhocapp.ProposalDTO{ProposalID: "proposal-001", CompanyID: "company-001", Status: adhocapp.StatusApproved, RecordID: "record-001", WorkflowInstanceID: "wf-001"},
		RecordID:           "record-001",
		WorkflowInstanceID: "wf-001",
	})
	envBytes, _ := json.Marshal(&idempotency.Envelope{HTTPStatus: http.StatusOK, Body: body})
	idem := &fakeIdemStore{tryResult: idempotency.Result{Replay: true, ReplayHTTPStatus: http.StatusOK, ReplayBody: body}}
	_ = envBytes
	handler := NewHandler(svc, fakeInspector{}, idem)
	mux := http.NewServeMux()
	handler.Register(mux)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/company/ad-hoc-proposals/proposal-001/admin-approve", strings.NewReader(`{"adjustment_note":"ok"}`))
	req.Header.Set("Authorization", "Bearer atk_test")
	req.Header.Set("Idempotency-Key", "idem-001")
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	if svc.adminApproveCalls != 0 {
		t.Fatalf("expected AdminApprove not to be called on replay, got %d", svc.adminApproveCalls)
	}
}
