package http

import (
	"encoding/json"
	"net/http"
	"strings"

	adhocapp "github.com/cobo/cobo_iam_services/internal/adhoc/app"
	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	"github.com/cobo/cobo_iam_services/internal/platform/httpx"
)

type Handler struct {
	svc       adhocapp.Service
	inspector iamapp.TokenInspector
}

func NewHandler(svc adhocapp.Service, inspector iamapp.TokenInspector) *Handler {
	return &Handler{svc: svc, inspector: inspector}
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
	resp, err := h.svc.ListProposals(r.Context(), adhocapp.ListProposalsRequest{
		Subject:      sub,
		StatusFilter: statusFilter,
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
	resp, err := h.svc.AdminApprove(r.Context(), body)
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
