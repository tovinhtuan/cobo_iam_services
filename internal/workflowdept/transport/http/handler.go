package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/cobo/cobo_iam_services/internal/platform/httpx"
	"github.com/cobo/cobo_iam_services/internal/workflowdept"
)

const roleAdmin = "admin_doanh_nghiep"

// Actor is the tenant identity taken from the session, never from the body.
type Actor struct {
	CompanyID    string
	MembershipID string
	Roles        []string
}

type Session func(r *http.Request) (Actor, error)

type DeptCheck func(ctx context.Context, companyID, departmentID string) error

type CodeCheck func(ctx context.Context, code string) (usable bool, err error)

// Store is the mapping persistence used by the admin API.
type Store interface {
	Commit(ctx context.Context, in Write) (version int64, err error)
	Close(ctx context.Context, companyID, templateCode, typeID, stepCode string, version int64, actor string) error
	List(ctx context.Context, companyID string) ([]Item, error)
	Suggest(ctx context.Context, companyID, templateCode string) ([]Suggestion, error)
	Preflight(ctx context.Context, companyID string) ([]PreflightRow, error)
}

type Suggestion struct {
	DepartmentID   string `json:"department_id"`
	DepartmentName string `json:"department_name"`
	Class          string `json:"suggestion_class"`
	Ambiguous      bool   `json:"ambiguity"`
}

type PreflightRow struct {
	CompanyID              string `json:"company_id"`
	TemplateDepartmentCode string `json:"template_department_code"`
	AffectedCount          int    `json:"affected_workflow_count"`
	CurrentResolution      string `json:"current_resolution"`
	FallbackImpact         string `json:"current_fallback_impact"`
	SuggestionClass        string `json:"suggestion_class"`
	Ambiguous              bool   `json:"ambiguity"`
}

type Write struct {
	CompanyID           string
	TemplateCode        string
	DisclosureTypeID    string
	StepCode            string
	CompanyDepartmentID string
	ExpectedVersion     int64
	Actor               string
}

type Item struct {
	TemplateDepartmentCode string `json:"template_department_code"`
	DisclosureTypeID       string `json:"disclosure_type_id,omitempty"`
	StepCode               string `json:"step_code,omitempty"`
	CompanyDepartmentID    string `json:"company_department_id"`
	CompanyDepartmentName  string `json:"company_department_name,omitempty"`
	Version                int64  `json:"version"`
	Status                 string `json:"status"`
}

type StepMatch func(ctx context.Context, companyID, typeID, stepCode, templateCode string) error

type Handler struct {
	Session   Session
	Dept      DeptCheck
	Code      CodeCheck
	StepMatch StepMatch
	Store     Store
}

func (h Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/admin/workflow-department-mappings", h.list)
	mux.HandleFunc("PUT /api/v1/admin/workflow-department-mappings/default/{template_department_code}", h.putDefault)
	mux.HandleFunc("DELETE /api/v1/admin/workflow-department-mappings/default/{template_department_code}", h.deleteDefault)
	mux.HandleFunc("PUT /api/v1/admin/disclosure-types/{type_id}/department-mapping/{template_department_code}", h.putType)
	mux.HandleFunc("DELETE /api/v1/admin/disclosure-types/{type_id}/department-mapping/{template_department_code}", h.deleteType)
	mux.HandleFunc("PUT /api/v1/admin/disclosure-types/{type_id}/workflow-steps/{step_code}/department-mapping/{template_department_code}", h.putStep)
	mux.HandleFunc("DELETE /api/v1/admin/disclosure-types/{type_id}/workflow-steps/{step_code}/department-mapping/{template_department_code}", h.deleteStep)
	mux.HandleFunc("GET /api/v1/admin/workflow-department-mapping-suggestions", h.suggestions)
	mux.HandleFunc("GET /api/v1/admin/workflow-department-mappings/preflight", h.preflight)
}

func (h Handler) list(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.allow(w, r)
	if !ok {
		return
	}
	items, err := h.Store.List(r.Context(), actor.CompanyID)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h Handler) putDefault(w http.ResponseWriter, r *http.Request) {
	h.put(w, r, "", "")
}

func (h Handler) deleteDefault(w http.ResponseWriter, r *http.Request) {
	h.del(w, r, "", "")
}

func (h Handler) putType(w http.ResponseWriter, r *http.Request) {
	h.put(w, r, r.PathValue("type_id"), "")
}

func (h Handler) deleteType(w http.ResponseWriter, r *http.Request) {
	h.del(w, r, r.PathValue("type_id"), "")
}

func (h Handler) putStep(w http.ResponseWriter, r *http.Request) {
	h.put(w, r, r.PathValue("type_id"), r.PathValue("step_code"))
}

func (h Handler) deleteStep(w http.ResponseWriter, r *http.Request) {
	h.del(w, r, r.PathValue("type_id"), r.PathValue("step_code"))
}

func (h Handler) put(w http.ResponseWriter, r *http.Request, typeID, stepCode string) {
	actor, ok := h.allow(w, r)
	if !ok {
		return
	}
	var body struct {
		CompanyDepartmentID string `json:"company_department_id"`
		ExpectedVersion     int64  `json:"expected_version"`
		CompanyID           string `json:"company_id"`
		DepartmentName      string `json:"department_name"`
		EffectiveFrom       string `json:"effective_from"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(body.CompanyDepartmentID) == "" {
		http.Error(w, "company_department_id is required", http.StatusBadRequest)
		return
	}
	code := r.PathValue("template_department_code")
	usable, err := h.Code(r.Context(), code)
	if err != nil || !usable {
		http.Error(w, "template department code is unknown or retired", http.StatusBadRequest)
		return
	}
	if err := h.Dept(r.Context(), actor.CompanyID, body.CompanyDepartmentID); err != nil {
		writeDeptErr(w, err)
		return
	}
	if h.StepMatch != nil && strings.TrimSpace(typeID) != "" {
		if err := h.StepMatch(r.Context(), actor.CompanyID, typeID, stepCode, code); err != nil {
			if errors.Is(err, workflowdept.ErrStepTokenMismatch) {
				http.Error(w, "STEP_TOKEN_MISMATCH", http.StatusBadRequest)
				return
			}
			httpx.WriteError(w, nil, err)
			return
		}
	}
	in := Write{
		CompanyID: actor.CompanyID, TemplateCode: code, DisclosureTypeID: typeID, StepCode: stepCode,
		CompanyDepartmentID: body.CompanyDepartmentID, ExpectedVersion: body.ExpectedVersion, Actor: actor.MembershipID,
	}
	version, err := h.Store.Commit(r.Context(), in)
	if errors.Is(err, workflowdept.ErrVersionConflict) {
		http.Error(w, "MAPPING_VERSION_CONFLICT", http.StatusConflict)
		return
	}
	if err != nil {
		writeDeptErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"template_department_code": code,
		"company_department_id":    body.CompanyDepartmentID,
		"version":                  version,
	})
}

func (h Handler) del(w http.ResponseWriter, r *http.Request, typeID, stepCode string) {
	actor, ok := h.allow(w, r)
	if !ok {
		return
	}
	var body struct {
		ExpectedVersion int64 `json:"expected_version"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	code := r.PathValue("template_department_code")
	if err := h.Store.Close(r.Context(), actor.CompanyID, code, typeID, stepCode, body.ExpectedVersion, actor.MembershipID); err != nil {
		if errors.Is(err, workflowdept.ErrVersionConflict) {
			http.Error(w, "MAPPING_VERSION_CONFLICT", http.StatusConflict)
			return
		}
		if errors.Is(err, workflowdept.ErrNotFound) {
			http.Error(w, "mapping not found", http.StatusNotFound)
			return
		}
		httpx.WriteError(w, nil, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h Handler) preflight(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.allow(w, r)
	if !ok {
		return
	}
	rows, err := h.Store.Preflight(r.Context(), actor.CompanyID)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": rows, "mode": "REPORT_ONLY"})
}

func (h Handler) suggestions(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.allow(w, r)
	if !ok {
		return
	}
	items, err := h.Store.Suggest(r.Context(), actor.CompanyID, r.URL.Query().Get("template_department_code"))
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func writeDeptErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, workflowdept.ErrDeptNotInCompany):
		http.Error(w, "DEPARTMENT_NOT_IN_COMPANY", http.StatusBadRequest)
	case errors.Is(err, workflowdept.ErrDeptInactive):
		http.Error(w, "DEPARTMENT_INACTIVE", http.StatusBadRequest)
	case errors.Is(err, workflowdept.ErrVersionConflict):
		http.Error(w, "MAPPING_VERSION_CONFLICT", http.StatusConflict)
	default:
		httpx.WriteError(w, nil, err)
	}
}

func (h Handler) allow(w http.ResponseWriter, r *http.Request) (Actor, bool) {
	if h.Session == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return Actor{}, false
	}
	actor, err := h.Session(r)
	if err != nil || actor.CompanyID == "" || actor.MembershipID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return Actor{}, false
	}
	if !hasRole(actor.Roles, roleAdmin) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return Actor{}, false
	}
	return actor, true
}

func hasRole(roles []string, want string) bool {
	for _, role := range roles {
		if role == want {
			return true
		}
	}
	return false
}
