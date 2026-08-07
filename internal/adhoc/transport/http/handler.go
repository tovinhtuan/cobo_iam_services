package http

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	adhocapp "github.com/cobo/cobo_iam_services/internal/adhoc/app"
	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	"github.com/cobo/cobo_iam_services/internal/platform/httpx"
	"github.com/cobo/cobo_iam_services/internal/platform/idempotency"
)

type Handler struct {
	log       *slog.Logger
	svc       adhocapp.Service
	inspector iamapp.TokenInspector
	idem      idempotency.Store
}

func NewHandler(log *slog.Logger, svc adhocapp.Service, inspector iamapp.TokenInspector, idem idempotency.Store) *Handler {
	return &Handler{log: log, svc: svc, inspector: inspector, idem: idem}
}

func (h *Handler) Register(mux *http.ServeMux) {
	// eligible-reviewers is the v3 primary route name; eligible-controllers is
	// kept as a deprecated alias pointing at the same renamed handler.
	mux.HandleFunc("GET /api/v1/company/ad-hoc-proposals/eligible-reviewers", h.listEligibleReviewers)
	mux.HandleFunc("GET /api/v1/company/ad-hoc-proposals/eligible-controllers", h.listEligibleReviewers)
	mux.HandleFunc("POST /api/v1/company/ad-hoc-proposals", h.createProposal)
	mux.HandleFunc("GET /api/v1/company/ad-hoc-proposals", h.listProposals)
	mux.HandleFunc("GET /api/v1/company/ad-hoc-proposals/{proposal_id}", h.getProposal)
	mux.HandleFunc("PATCH /api/v1/company/ad-hoc-proposals/{proposal_id}", h.patchDraftProposal)
	mux.HandleFunc("POST /api/v1/company/ad-hoc-proposals/{proposal_id}/submit", h.submitProposal)
	mux.HandleFunc("POST /api/v1/company/ad-hoc-proposals/{proposal_id}/approve", h.approve)
	// focal-approve is deprecated -> proxies straight to approve (v3 D1/D3).
	mux.HandleFunc("POST /api/v1/company/ad-hoc-proposals/{proposal_id}/focal-approve", h.focalApprove)
	mux.HandleFunc("POST /api/v1/company/ad-hoc-proposals/{proposal_id}/admin-approve", h.adminApprove)
	mux.HandleFunc("POST /api/v1/company/ad-hoc-proposals/{proposal_id}/reject", h.reject)
	mux.HandleFunc("POST /api/v1/company/ad-hoc-proposals/{proposal_id}/cancel", h.cancel)
	// One-time legacy migration endpoint (§6.7/A1) — platform admin only,
	// self-gated on rbac.manage inside the service layer.
	mux.HandleFunc("POST /api/v1/platform/cms/admin/ops/adhoc-migrate-legacy-approvals", h.migrateLegacyApprovals)
}

func (h *Handler) listEligibleReviewers(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, h.log, err)
		return
	}
	items, err := h.svc.ListEligibleReviewers(r.Context(), adhocapp.ListEligibleReviewersRequest{Subject: sub})
	if err != nil {
		httpx.WriteError(w, h.log, err)
		return
	}
	if items == nil {
		items = []adhocapp.EligibleController{}
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) createProposal(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, h.log, err)
		return
	}
	var req adhocapp.CreateProposalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, h.log, err)
		return
	}
	req.Subject = sub
	resp, err := h.svc.CreateProposal(r.Context(), req)
	if err != nil {
		httpx.WriteError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, resp)
}

func (h *Handler) listProposals(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, h.log, err)
		return
	}
	var statusFilter []string
	if s := strings.TrimSpace(r.URL.Query().Get("status")); s != "" {
		for _, v := range strings.Split(s, ",") {
			if v = strings.TrimSpace(v); v != "" {
				statusFilter = append(statusFilter, v)
			}
		}
	}
	page, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("page")))
	pageSize, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("page_size")))
	resp, err := h.svc.ListProposals(r.Context(), adhocapp.ListProposalsRequest{
		Subject:      sub,
		StatusFilter: statusFilter,
		Page:         page,
		PageSize:     pageSize,
	})
	if err != nil {
		httpx.WriteError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) getProposal(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, h.log, err)
		return
	}
	resp, err := h.svc.GetProposal(r.Context(), adhocapp.GetProposalRequest{
		Subject:    sub,
		ProposalID: strings.TrimSpace(r.PathValue("proposal_id")),
	})
	if err != nil {
		httpx.WriteError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) patchDraftProposal(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, h.log, err)
		return
	}
	var req adhocapp.PatchDraftProposalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, h.log, err)
		return
	}
	req.Subject = sub
	req.ProposalID = strings.TrimSpace(r.PathValue("proposal_id"))
	resp, err := h.svc.PatchDraftProposal(r.Context(), req)
	if err != nil {
		httpx.WriteError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) submitProposal(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, h.log, err)
		return
	}
	resp, err := h.svc.SubmitProposal(r.Context(), adhocapp.ProposalActionRequest{
		Subject:    sub,
		ProposalID: strings.TrimSpace(r.PathValue("proposal_id")),
	})
	if err != nil {
		httpx.WriteError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) approve(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, h.log, err)
		return
	}
	var body adhocapp.ApproveRequest
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			httpx.WriteError(w, h.log, err)
			return
		}
	}
	body.Subject = sub
	body.ProposalID = strings.TrimSpace(r.PathValue("proposal_id"))
	resp, err := h.svc.Approve(r.Context(), body)
	if err != nil {
		httpx.WriteError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

// focalApprove is deprecated — proxies to approve (v3 D1/D3: one round only).
func (h *Handler) focalApprove(w http.ResponseWriter, r *http.Request) {
	h.approve(w, r)
}

func (h *Handler) adminApprove(w http.ResponseWriter, r *http.Request) {
	h.log.Warn("deprecated_endpoint_called", slog.String("endpoint", "admin-approve"))
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, h.log, err)
		return
	}
	var body adhocapp.AdminApproveRequest
	if r.ContentLength > 0 {
		_ = json.NewDecoder(r.Body).Decode(&body)
	}
	body.Subject = sub
	body.ProposalID = strings.TrimSpace(r.PathValue("proposal_id"))
	body.IdempotencyKey = strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if body.IdempotencyKey == "" {
		body.IdempotencyKey = adhocAdminApproveFallbackKey(sub.CompanyID, body.ProposalID, sub.MembershipID, body.FinalT0Date, body.FinalDeadlineDate, body.AdjustmentNote)
	}
	var res idempotency.Result
	if h.idem != nil {
		hash := adhocAdminApproveRequestHash(sub.CompanyID, body.ProposalID, sub.MembershipID, body.FinalT0Date, body.FinalDeadlineDate, body.AdjustmentNote)
		res, err = h.idem.TryReserve(r.Context(), idempotency.Params{
			CompanyID:   sub.CompanyID,
			Scope:       "adhoc.admin_approve",
			Key:         body.IdempotencyKey,
			RequestHash: hash,
		})
		if err != nil {
			httpx.WriteError(w, h.log, err)
			return
		}
		if res.Replay {
			httpx.WriteJSONRaw(w, res.ReplayHTTPStatus, res.ReplayBody)
			return
		}
		if res.Conflict {
			httpx.WriteJSON(w, http.StatusConflict, idempotencyConflictBody("idempotency conflict or request in progress"))
			return
		}
	}
	resp, err := h.svc.AdminApprove(r.Context(), body)
	if res.ReservationID != "" && h.idem != nil {
		if err != nil {
			_ = h.idem.Abandon(r.Context(), res.ReservationID)
		} else {
			respBody, _ := json.Marshal(resp)
			env := idempotency.Envelope{HTTPStatus: http.StatusOK, Body: respBody}
			envBytes, _ := json.Marshal(&env)
			_ = h.idem.Complete(r.Context(), res.ReservationID, envBytes)
		}
	}
	if err != nil {
		httpx.WriteError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) reject(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, h.log, err)
		return
	}
	var body adhocapp.RejectRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteError(w, h.log, err)
		return
	}
	body.Subject = sub
	body.ProposalID = strings.TrimSpace(r.PathValue("proposal_id"))
	resp, err := h.svc.Reject(r.Context(), body)
	if err != nil {
		httpx.WriteError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) cancel(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, h.log, err)
		return
	}
	resp, err := h.svc.Cancel(r.Context(), adhocapp.ProposalActionRequest{
		Subject:    sub,
		ProposalID: strings.TrimSpace(r.PathValue("proposal_id")),
	})
	if err != nil {
		httpx.WriteError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

// migrateLegacyApprovals is the one-time migration endpoint (§6.7/A1, A2):
// finds every proposal stuck at pending_admin_approval (cross-company) and
// auto-finalizes each via the legacy AdminApprove path. Per plan §6.7/A1 and
// §12.3: stops immediately at the first error — does NOT continue remaining rows.
// The endpoint is idempotent (deterministic IdempotencyKey per §5.3), so it
// is safe to call again after fixing the failure.
func (h *Handler) migrateLegacyApprovals(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, h.log, err)
		return
	}
	rows, err := h.svc.ListPendingLegacyApprovals(r.Context(), sub)
	if err != nil {
		httpx.WriteError(w, h.log, err)
		return
	}
	processed := 0
	for _, row := range rows {
		if err := h.svc.FinalizeLegacyApproval(r.Context(), sub, row.CompanyID, row.ProposalID); err != nil {
			h.log.Error("adhoc_migrate_legacy_approval_failed",
				slog.String("proposal_id", row.ProposalID),
				slog.String("company_id", row.CompanyID),
				slog.Any("error", err))
			httpx.WriteJSON(w, http.StatusOK, map[string]any{
				"processed":              processed,
				"stopped_at_proposal_id": row.ProposalID,
				"error":                  err.Error(),
			})
			return
		}
		processed++
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"processed":              processed,
		"stopped_at_proposal_id": nil,
	})
}

func (h *Handler) subjectFromToken(r *http.Request) (adhocapp.Subject, error) {
	tok := bearerToken(r.Header.Get("Authorization"))
	claims, err := h.inspector.InspectAccessToken(r.Context(), tok)
	if err != nil {
		return adhocapp.Subject{}, err
	}
	return adhocapp.Subject{UserID: claims.Sub, MembershipID: claims.MembershipID, CompanyID: claims.CompanyID}, nil
}

func bearerToken(h string) string {
	h = strings.TrimSpace(h)
	if h == "" {
		return ""
	}
	parts := strings.SplitN(h, " ", 2)
	if len(parts) == 2 && strings.EqualFold(parts[0], "bearer") {
		return strings.TrimSpace(parts[1])
	}
	return ""
}

func adhocAdminApproveFallbackKey(companyID, proposalID, membershipID, finalT0Date, finalDeadlineDate, adjustmentNote string) string {
	return adhocAdminApproveRequestHash(companyID, proposalID, membershipID, finalT0Date, finalDeadlineDate, adjustmentNote)
}

func adhocAdminApproveRequestHash(companyID, proposalID, membershipID, finalT0Date, finalDeadlineDate, adjustmentNote string) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%s|%s|%s|%s",
		strings.TrimSpace(companyID),
		strings.TrimSpace(proposalID),
		strings.TrimSpace(membershipID),
		strings.TrimSpace(finalT0Date),
		strings.TrimSpace(finalDeadlineDate),
		strings.TrimSpace(adjustmentNote),
	)))
	return hex.EncodeToString(h[:])
}

func idempotencyConflictBody(msg string) map[string]any {
	return map[string]any{
		"error": map[string]any{
			"code":    "IDEMPOTENCY_CONFLICT",
			"message": msg,
		},
	}
}
