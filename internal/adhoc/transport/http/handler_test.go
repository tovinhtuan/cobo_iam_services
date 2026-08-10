package http

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	adhocapp "github.com/cobo/cobo_iam_services/internal/adhoc/app"
	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	"github.com/cobo/cobo_iam_services/internal/platform/idempotency"
)

// discardLogger is a no-op *slog.Logger used by handler tests that exercise
// a code path with a log call but don't assert on log output.
func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type fakeService struct {
	listReq           adhocapp.ListProposalsRequest
	lastAdminApprove  adhocapp.AdminApproveRequest
	adminApproveCalls int
}

func (f *fakeService) CreateProposal(context.Context, adhocapp.CreateProposalRequest) (*adhocapp.ProposalDTO, error) {
	return nil, nil
}

func (f *fakeService) PatchDraftProposal(context.Context, adhocapp.PatchDraftProposalRequest) (*adhocapp.ProposalDTO, error) {
	return nil, nil
}

func (f *fakeService) SubmitProposal(context.Context, adhocapp.ProposalActionRequest) (*adhocapp.ProposalDTO, error) {
	return nil, nil
}

func (f *fakeService) Approve(context.Context, adhocapp.ApproveRequest) (*adhocapp.ApproveResponse, error) {
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

func (f *fakeService) ListEligibleReviewers(_ context.Context, _ adhocapp.ListEligibleReviewersRequest) ([]adhocapp.EligibleController, error) {
	return []adhocapp.EligibleController{}, nil
}

func (f *fakeService) FinalizeLegacyApproval(context.Context, adhocapp.Subject, string, string) error {
	return nil
}

func (f *fakeService) ListPendingLegacyApprovals(context.Context, adhocapp.Subject) ([]adhocapp.PendingApprovalRow, error) {
	return nil, nil
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
	handler := NewHandler(nil, svc, fakeInspector{}, nil)
	mux := http.NewServeMux()
	handler.Register(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/company/ad-hoc-proposals?status=approved&page=2&page_size=10", nil)
	req.Header.Set("Authorization", "Bearer atk_test")
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	if svc.listReq.Page != 2 || svc.listReq.PageSize != 10 {
		t.Fatalf("paging not parsed: %#v", svc.listReq)
	}
	if len(svc.listReq.StatusFilter) != 1 || svc.listReq.StatusFilter[0] != "approved" {
		t.Fatalf("status filter: %#v", svc.listReq.StatusFilter)
	}
	if svc.listReq.Scope != "" {
		t.Fatalf("scope should be empty, got %q", svc.listReq.Scope)
	}
}

func TestListProposals_ParsesScopeMy(t *testing.T) {
	svc := &fakeService{}
	handler := NewHandler(nil, svc, fakeInspector{}, nil)
	mux := http.NewServeMux()
	handler.Register(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/company/ad-hoc-proposals?scope=my&page=1&page_size=5", nil)
	req.Header.Set("Authorization", "Bearer atk_test")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}
	if svc.listReq.Scope != "my" {
		t.Fatalf("expected scope=my, got %#v", svc.listReq)
	}
	if svc.listReq.Subject.MembershipID != "member-001" || svc.listReq.Subject.CompanyID != "company-001" {
		t.Fatalf("subject not from token: %#v", svc.listReq.Subject)
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
	handler := NewHandler(discardLogger(), svc, fakeInspector{}, idem)
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
	handler := NewHandler(discardLogger(), svc, fakeInspector{}, idem)
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

// TestApproveResponse_GoldenJSONShape asserts that the wire shape of ApproveResponse
// and ProposalDTO (embedded) contains the exact JSON keys required by the FE contract
// (§6.8/B4, plan Phase 2 task 7). Any accidental json:"..." tag rename fails here.
func TestApproveResponse_GoldenJSONShape(t *testing.T) {
	t.Run("approve_response_not_finalized", func(t *testing.T) {
		resp := adhocapp.ApproveResponse{
			Proposal: adhocapp.ProposalDTO{
				ProposalID: "p-001",
				CompanyID:  "c-001",
				Status:     adhocapp.StatusPendingFocalApproval,
			},
			ApprovalProgress: adhocapp.ApprovalProgressDTO{Required: 3, Completed: 2},
			Finalized:        false,
		}
		data, err := json.Marshal(resp)
		if err != nil {
			t.Fatalf("json.Marshal failed: %v", err)
		}
		var m map[string]any
		if err := json.Unmarshal(data, &m); err != nil {
			t.Fatalf("json.Unmarshal failed: %v", err)
		}
		// Top-level keys required by §4.3 (not-finalized response).
		for _, key := range []string{"proposal", "approval_progress", "finalized"} {
			if _, ok := m[key]; !ok {
				t.Errorf("missing required top-level key %q in approve response", key)
			}
		}
		// approval_progress shape: required + completed.
		ap, _ := m["approval_progress"].(map[string]any)
		for _, key := range []string{"required", "completed"} {
			if _, ok := ap[key]; !ok {
				t.Errorf("missing key %q inside approval_progress", key)
			}
		}
		// finalized must be false.
		if fin, _ := m["finalized"].(bool); fin {
			t.Errorf("expected finalized=false, got true")
		}
	})

	t.Run("approve_response_finalized", func(t *testing.T) {
		resp := adhocapp.ApproveResponse{
			Proposal: adhocapp.ProposalDTO{
				ProposalID: "p-001",
				CompanyID:  "c-001",
				Status:     adhocapp.StatusApproved,
			},
			ApprovalProgress:   adhocapp.ApprovalProgressDTO{Required: 3, Completed: 3},
			Finalized:          true,
			RecordID:           "rec-abc",
			WorkflowInstanceID: "wf-xyz",
		}
		data, err := json.Marshal(resp)
		if err != nil {
			t.Fatalf("json.Marshal failed: %v", err)
		}
		var m map[string]any
		if err := json.Unmarshal(data, &m); err != nil {
			t.Fatalf("json.Unmarshal failed: %v", err)
		}
		// Top-level keys required by §4.3 (finalized response).
		for _, key := range []string{"proposal", "approval_progress", "finalized", "record_id", "workflow_instance_id"} {
			if _, ok := m[key]; !ok {
				t.Errorf("missing required top-level key %q in finalized approve response", key)
			}
		}
		if fin, _ := m["finalized"].(bool); !fin {
			t.Errorf("expected finalized=true, got false")
		}
	})

	t.Run("proposal_dto_review_state_keys", func(t *testing.T) {
		// Verify ProposalDTO embeds reviewers/approvals/approval_progress (GET {id} shape).
		req1 := adhocapp.ApprovalProgressDTO{Required: 2, Completed: 1}
		dto := adhocapp.ProposalDTO{
			ProposalID: "p-001",
			CompanyID:  "c-001",
			Status:     adhocapp.StatusPendingFocalApproval,
			Reviewers: []adhocapp.ReviewerDTO{
				{MembershipID: "m-001", FullName: "Alice"},
			},
			Approvals: []adhocapp.ApprovalDTO{
				{MembershipID: "m-002"},
			},
			ApprovalProgress: &req1,
		}
		data, err := json.Marshal(dto)
		if err != nil {
			t.Fatalf("json.Marshal ProposalDTO failed: %v", err)
		}
		var m map[string]any
		if err := json.Unmarshal(data, &m); err != nil {
			t.Fatalf("json.Unmarshal failed: %v", err)
		}
		for _, key := range []string{"reviewers", "approvals", "approval_progress"} {
			if _, ok := m[key]; !ok {
				t.Errorf("missing key %q in ProposalDTO JSON (GET {id} shape)", key)
			}
		}
		// reviewer shape: membership_id.
		reviewers, _ := m["reviewers"].([]any)
		if len(reviewers) != 1 {
			t.Fatalf("expected 1 reviewer, got %d", len(reviewers))
		}
		rev0, _ := reviewers[0].(map[string]any)
		if _, ok := rev0["membership_id"]; !ok {
			t.Errorf("missing key membership_id in ReviewerDTO")
		}
	})
}
