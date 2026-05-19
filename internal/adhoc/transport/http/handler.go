package http

import (
	"crypto/sha256"
	"encoding/json"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	adhocapp "github.com/cobo/cobo_iam_services/internal/adhoc/app"
	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	"github.com/cobo/cobo_iam_services/internal/platform/httpx"
	"github.com/cobo/cobo_iam_services/internal/platform/idempotency"
)

type Handler struct {
	svc       adhocapp.Service
	inspector iamapp.TokenInspector
	idem      idempotency.Store
}

func NewHandler(svc adhocapp.Service, inspector iamapp.TokenInspector, idem idempotency.Store) *Handler {
	return &Handler{svc: svc, inspector: inspector, idem: idem}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/company/ad-hoc-proposals", h.createProposal)
	mux.HandleFunc("GET /api/v1/company/ad-hoc-proposals", h.listProposals)
	mux.HandleFunc("GET /api/v1/company/ad-hoc-proposals/{proposal_id}", h.getProposal)
	mux.HandleFunc("POST /api/v1/company/ad-hoc-proposals/{proposal_id}/submit", h.submitProposal)
	mux.HandleFunc("POST /api/v1/company/ad-hoc-proposals/{proposal_id}/focal-approve", h.focalApprove)
	mux.HandleFunc("POST /api/v1/company/ad-hoc-proposals/{proposal_id}/admin-approve", h.adminApprove)
	mux.HandleFunc("POST /api/v1/company/ad-hoc-proposals/{proposal_id}/reject", h.reject)
	mux.HandleFunc("POST /api/v1/company/ad-hoc-proposals/{proposal_id}/cancel", h.cancel)
}

func (h *Handler) createProposal(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	var req adhocapp.CreateProposalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	req.Subject = sub
	resp, err := h.svc.CreateProposal(r.Context(), req)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, resp)
}

func (h *Handler) listProposals(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
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
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) getProposal(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	resp, err := h.svc.GetProposal(r.Context(), adhocapp.GetProposalRequest{
		Subject:    sub,
		ProposalID: strings.TrimSpace(r.PathValue("proposal_id")),
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) submitProposal(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	resp, err := h.svc.SubmitProposal(r.Context(), adhocapp.ProposalActionRequest{
		Subject:    sub,
		ProposalID: strings.TrimSpace(r.PathValue("proposal_id")),
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) focalApprove(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	resp, err := h.svc.FocalApprove(r.Context(), adhocapp.ProposalActionRequest{
		Subject:    sub,
		ProposalID: strings.TrimSpace(r.PathValue("proposal_id")),
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) adminApprove(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
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
			CompanyID: sub.CompanyID,
			Scope:     "adhoc.admin_approve",
			Key:       body.IdempotencyKey,
			RequestHash: hash,
		})
		if err != nil {
			httpx.WriteError(w, nil, err)
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
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) reject(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	var body adhocapp.RejectRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	body.Subject = sub
	body.ProposalID = strings.TrimSpace(r.PathValue("proposal_id"))
	resp, err := h.svc.Reject(r.Context(), body)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) cancel(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	resp, err := h.svc.Cancel(r.Context(), adhocapp.ProposalActionRequest{
		Subject:    sub,
		ProposalID: strings.TrimSpace(r.PathValue("proposal_id")),
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
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
