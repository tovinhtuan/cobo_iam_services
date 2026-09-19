package http

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/httpx"
	wff "github.com/cobo/cobo_iam_services/internal/workflowfulfillment"
	wsc "github.com/cobo/cobo_iam_services/internal/workflowstepcomments"
)

// Handler serves step discussion comment routes.
type Handler struct {
	log       *slog.Logger
	svc       *wsc.Service
	inspector iamapp.TokenInspector
}

func NewHandler(log *slog.Logger, svc *wsc.Service, inspector iamapp.TokenInspector) *Handler {
	return &Handler{log: log, svc: svc, inspector: inspector}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/company/deadlines/{record_id}/steps/{step_code}/comments", h.listComments)
	mux.HandleFunc("GET /api/v1/company/deadlines/{record_id}/steps/{step_code}/comment-mention-candidates", h.listMentionCandidates)
	mux.HandleFunc("POST /api/v1/company/deadlines/{record_id}/steps/{step_code}/comments", h.createComment)
	mux.HandleFunc("PATCH /api/v1/company/deadlines/{record_id}/steps/{step_code}/comments/{comment_id}", h.updateComment)
	mux.HandleFunc("DELETE /api/v1/company/deadlines/{record_id}/steps/{step_code}/comments/{comment_id}", h.deleteComment)
}

func (h *Handler) subject(r *http.Request) (wff.Subject, error) {
	claims, err := h.inspector.InspectAccessToken(r.Context(), bearerToken(r.Header.Get("Authorization")))
	if err != nil {
		return wff.Subject{}, err
	}
	return wff.Subject{
		UserID:       claims.Sub,
		MembershipID: claims.MembershipID,
		CompanyID:    claims.CompanyID,
	}, nil
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
	return h
}

func (h *Handler) listComments(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, h.log, err)
		return
	}
	page, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("page")))
	pageSize, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("page_size")))
	resp, err := h.svc.ListComments(r.Context(), sub, r.PathValue("record_id"), r.PathValue("step_code"), page, pageSize)
	if err != nil {
		httpx.WriteError(w, h.log, err)
		return
	}
	if resp.Comments == nil {
		resp.Comments = []wsc.CommentDTO{}
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) listMentionCandidates(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, h.log, err)
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	page, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("page")))
	pageSize, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("page_size")))
	if page == 0 && strings.TrimSpace(r.URL.Query().Get("page")) == "" {
		page = 1
	}
	resp, err := h.svc.ListMentionCandidates(r.Context(), sub, r.PathValue("record_id"), r.PathValue("step_code"), q, page, pageSize)
	if err != nil {
		httpx.WriteError(w, h.log, err)
		return
	}
	if resp.Items == nil {
		resp.Items = []wsc.MentionCandidateDTO{}
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) createComment(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, h.log, err)
		return
	}
	var body wsc.CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteError(w, h.log, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid json body", err))
		return
	}
	idemKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idemKey == "" {
		idemKey = strings.TrimSpace(r.Header.Get("X-Idempotency-Key"))
	}
	resp, status, err := h.svc.CreateComment(r.Context(), sub, r.PathValue("record_id"), r.PathValue("step_code"), idemKey, body.Body, body.Mentions)
	if err != nil {
		httpx.WriteError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, status, resp)
}

func (h *Handler) updateComment(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, h.log, err)
		return
	}
	var body wsc.UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteError(w, h.log, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid json body", err))
		return
	}
	resp, err := h.svc.UpdateComment(r.Context(), sub, r.PathValue("record_id"), r.PathValue("step_code"), r.PathValue("comment_id"), body.Body, body.Mentions)
	if err != nil {
		httpx.WriteError(w, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) deleteComment(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, h.log, err)
		return
	}
	if err := h.svc.DeleteComment(r.Context(), sub, r.PathValue("record_id"), r.PathValue("step_code"), r.PathValue("comment_id")); err != nil {
		httpx.WriteError(w, h.log, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
