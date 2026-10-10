package http

import (
	"encoding/json"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/cobo/cobo_iam_services/internal/companyaccess/companystatus"
	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/httpx"
	oauth "github.com/cobo/cobo_iam_services/internal/templatebuilderoauth"
)

type Handler struct {
	oauth        *oauth.Service
	disclosure   disclosureapp.Service
	inspector    iamapp.TokenInspector
	publicWebURL string
	companies    iamapp.CompanyStatusReader
}

// WithCompanyStatus rejects builder action tokens bound to a deactivated or missing company: those
// tokens are checked here, not by the session-bound token inspector.
func (h *Handler) WithCompanyStatus(r iamapp.CompanyStatusReader) *Handler {
	h.companies = r
	return h
}

func NewHandler(oauthService *oauth.Service, disclosure disclosureapp.Service, inspector iamapp.TokenInspector, publicWebURL string) *Handler {
	return &Handler{oauth: oauthService, disclosure: disclosure, inspector: inspector, publicWebURL: strings.TrimRight(strings.TrimSpace(publicWebURL), "/")}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/oauth/template-builder/authorize", h.authorize)
	mux.HandleFunc("POST /api/v1/oauth/template-builder/approve", h.approve)
	mux.HandleFunc("POST /api/v1/oauth/template-builder/token", h.token)
	mux.HandleFunc("POST /api/v1/template-builder/validate", h.validate)
}

func (h *Handler) authorize(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if err := h.oauth.ValidateAuthorizationRequest(q.Get("client_id"), q.Get("redirect_uri"), q.Get("scope"), q.Get("code_challenge"), q.Get("code_challenge_method")); err != nil {
		httpx.WriteError(w, nil, invalidOAuthRequest())
		return
	}
	if h.publicWebURL == "" {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusServiceUnavailable, perr.CodeServiceUnavailable, "oauth consent screen unavailable", nil))
		return
	}
	destination, _ := url.Parse(h.publicWebURL + "/oauth/template-builder/authorize")
	destination.RawQuery = q.Encode()
	http.Redirect(w, r, destination.String(), http.StatusFound)
}

func (h *Handler) approve(w http.ResponseWriter, r *http.Request) {
	claims, err := h.inspector.InspectAccessToken(r.Context(), bearerToken(r.Header.Get("Authorization")))
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	var body struct {
		ClientID            string `json:"client_id"`
		RedirectURI         string `json:"redirect_uri"`
		Scope               string `json:"scope"`
		State               string `json:"state,omitempty"`
		CodeChallenge       string `json:"code_challenge"`
		CodeChallengeMethod string `json:"code_challenge_method"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16*1024)).Decode(&body); err != nil {
		httpx.WriteError(w, nil, invalidOAuthRequest())
		return
	}
	redirect, err := h.oauth.Approve(r.Context(), oauth.ApproveRequest{
		ClientID: body.ClientID, RedirectURI: body.RedirectURI, Scope: body.Scope, State: body.State,
		CodeChallenge: body.CodeChallenge, CodeChallengeMethod: body.CodeChallengeMethod,
		Subject: oauth.Subject{UserID: claims.Sub, MembershipID: claims.MembershipID, CompanyID: claims.CompanyID},
	})
	if err != nil {
		httpx.WriteError(w, nil, invalidOAuthRequest())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"redirect_to": redirect})
}

func (h *Handler) token(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil || r.Form.Get("grant_type") != "authorization_code" {
		httpx.WriteError(w, nil, invalidOAuthRequest())
		return
	}
	response, err := h.oauth.Exchange(r.Context(), oauth.ExchangeRequest{
		ClientID: r.Form.Get("client_id"), Code: r.Form.Get("code"), RedirectURI: r.Form.Get("redirect_uri"), CodeVerifier: r.Form.Get("code_verifier"),
	})
	if err != nil {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusUnauthorized, perr.CodeSessionExpired, "invalid oauth grant", nil))
		return
	}
	httpx.WriteJSON(w, http.StatusOK, response)
}

func (h *Handler) validate(w http.ResponseWriter, r *http.Request) {
	sub, err := h.oauth.ValidateAccessToken(bearerToken(r.Header.Get("Authorization")))
	if err != nil {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusUnauthorized, perr.CodeSessionExpired, "invalid oauth access token", nil))
		return
	}
	if h.companies != nil {
		status, err := h.companies.CompanyStatus(r.Context(), sub.CompanyID)
		if err != nil {
			httpx.WriteError(w, nil, err)
			return
		}
		if strings.TrimSpace(status) == "" || companystatus.BlocksAccess(status) {
			httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusForbidden, perr.CodeCompanyInactive, "the company is no longer active", nil))
			return
		}
	}
	var body struct {
		Filename string `json:"filename"`
		JSON     string `json:"json"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, int64(disclosureapp.MaxTemplateImportFileSizeBytes)+1024)).Decode(&body); err != nil {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid JSON request", nil))
		return
	}
	name := strings.TrimSpace(body.Filename)
	if name == "" || filepath.Base(name) != name || strings.ToLower(filepath.Ext(name)) != ".json" || len([]byte(body.JSON)) == 0 || len([]byte(body.JSON)) > disclosureapp.MaxTemplateImportFileSizeBytes {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "filename and JSON payload are invalid", nil))
		return
	}
	response, err := h.disclosure.ValidateTemplateImportForBuilder(r.Context(), disclosureapp.ValidateTemplateImportForBuilderRequest{
		Subject: disclosureapp.Subject{UserID: sub.UserID, MembershipID: sub.MembershipID, CompanyID: sub.CompanyID}, Filename: name, FileBytes: []byte(body.JSON),
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, response)
}

func invalidOAuthRequest() error {
	return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid oauth request", nil)
}

func bearerToken(header string) string {
	parts := strings.SplitN(strings.TrimSpace(header), " ", 2)
	if len(parts) == 2 && strings.EqualFold(parts[0], "bearer") {
		return strings.TrimSpace(parts[1])
	}
	return ""
}
