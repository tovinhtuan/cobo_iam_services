package http

import (
	"encoding/json"
	"net/http"
	"strings"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/httpx"
)

type Handler struct {
	svc       authapp.Service
	inspector iamapp.TokenInspector
}

func NewHandler(svc authapp.Service, inspector iamapp.TokenInspector) *Handler {
	return &Handler{svc: svc, inspector: inspector}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /internal/v1/authorize", h.authorize)
	mux.HandleFunc("POST /internal/v1/authorize/batch", h.authorizeBatch)
}

func (h *Handler) authorize(w http.ResponseWriter, r *http.Request) {
	claims, err := h.requireAccessToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	var req authapp.AuthorizeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if err := applyAccessTokenToSubject(claims, &req.Subject); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	decision, err := h.svc.Authorize(r.Context(), req)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, decision)
}

func (h *Handler) authorizeBatch(w http.ResponseWriter, r *http.Request) {
	claims, err := h.requireAccessToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	var req authapp.AuthorizeBatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if err := applyAccessTokenToSubject(claims, &req.Subject); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	resp, err := h.svc.AuthorizeBatch(r.Context(), req)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) requireAccessToken(r *http.Request) (*iamapp.AccessTokenClaims, error) {
	bearer := bearerToken(r.Header.Get("Authorization"))
	return h.inspector.InspectAccessToken(r.Context(), bearer)
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

// applyAccessTokenToSubject overwrites subject from the access token claims. Body
// user_id / membership_id / company_id are ignored to align decision with the caller
// context (see deploy gateway + security doc in `docs/`). RBAC is unchanged in the service layer.
func applyAccessTokenToSubject(claims *iamapp.AccessTokenClaims, sub *authapp.SubjectRef) error {
	if claims == nil {
		return perr.NewHTTPError(http.StatusUnauthorized, perr.CodeSessionExpired, "invalid access token", nil)
	}
	if strings.TrimSpace(claims.MembershipID) == "" || strings.TrimSpace(claims.CompanyID) == "" {
		return perr.NewHTTPError(http.StatusUnprocessableEntity, perr.CodeCompanyContextRequired, "access token must include company context for authorization", nil)
	}
	sub.UserID = claims.Sub
	sub.MembershipID = claims.MembershipID
	sub.CompanyID = claims.CompanyID
	return nil
}
