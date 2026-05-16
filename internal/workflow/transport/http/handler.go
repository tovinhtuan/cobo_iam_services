package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/httpx"
	workflowapp "github.com/cobo/cobo_iam_services/internal/workflow/app"
)

type Handler struct {
	svc       workflowapp.Service
	inspector iamapp.TokenInspector
}

func NewHandler(svc workflowapp.Service, inspector iamapp.TokenInspector) *Handler {
	return &Handler{svc: svc, inspector: inspector}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/workflows/instances", h.createInstance)
	mux.HandleFunc("GET /api/v1/workflows/instances/{instance_id}", h.getInstance)
	mux.HandleFunc("GET /api/v1/workflows/instances/{instance_id}/tasks", h.listInstanceTasks)
	mux.HandleFunc("GET /api/v1/workflows/instances/{instance_id}/reminders", h.listInstanceReminders)
	mux.HandleFunc("POST /api/v1/workflows/tasks/{task_id}/review", h.reviewTask)
	mux.HandleFunc("POST /api/v1/workflows/tasks/{task_id}/approve", h.approveTask)
	mux.HandleFunc("POST /api/v1/workflows/tasks/{task_id}/confirm", h.confirmTask)
	mux.HandleFunc("POST /api/v1/workflows/tasks/{task_id}/reject", h.rejectTask)
	mux.HandleFunc("POST /api/v1/workflows/tasks/{task_id}/actions/{action}", h.taskActionByName)
	mux.HandleFunc("POST /api/v1/workflows/resolve-assignees", h.resolveAssignees)
}

func (h *Handler) createInstance(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	var payload struct {
		RecordID string `json:"record_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	resp, err := h.svc.CreateWorkflowInstance(r.Context(), workflowapp.CreateWorkflowInstanceRequest{Subject: sub, RecordID: payload.RecordID})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"data": resp})
}

func (h *Handler) getInstance(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	instanceID := r.PathValue("instance_id")
	resp, err := h.svc.GetWorkflowInstance(r.Context(), workflowapp.GetWorkflowInstanceRequest{
		Subject: sub, WorkflowInstanceID: instanceID,
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"data": resp})
}

func (h *Handler) listInstanceTasks(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	instanceID := r.PathValue("instance_id")
	resp, err := h.svc.ListInstanceTasks(r.Context(), workflowapp.ListInstanceTasksRequest{
		Subject: sub, WorkflowInstanceID: instanceID,
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"data": resp})
}

func (h *Handler) listInstanceReminders(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	instanceID := r.PathValue("instance_id")
	resp, err := h.svc.ListInstanceReminders(r.Context(), workflowapp.ListInstanceRemindersRequest{
		Subject: sub, WorkflowInstanceID: instanceID,
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"data": resp})
}

func (h *Handler) reviewTask(w http.ResponseWriter, r *http.Request) {
	h.taskAction(w, r, h.svc.ReviewTask)
}

func (h *Handler) approveTask(w http.ResponseWriter, r *http.Request) {
	h.taskAction(w, r, h.svc.ApproveTask)
}

func (h *Handler) confirmTask(w http.ResponseWriter, r *http.Request) {
	h.taskAction(w, r, h.svc.ConfirmTask)
}

func (h *Handler) rejectTask(w http.ResponseWriter, r *http.Request) {
	h.taskAction(w, r, h.svc.RejectTask)
}

func (h *Handler) taskActionByName(w http.ResponseWriter, r *http.Request) {
	action := strings.TrimSpace(r.PathValue("action"))
	switch action {
	case "review":
		h.reviewTask(w, r)
	case "approve":
		h.approveTask(w, r)
	case "confirm":
		h.confirmTask(w, r)
	case "reject":
		h.rejectTask(w, r)
	default:
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "unsupported workflow task action: "+action, nil))
	}
}

func (h *Handler) taskAction(w http.ResponseWriter, r *http.Request, fn func(ctx context.Context, req workflowapp.TaskActionRequest) (*workflowapp.TaskDTO, error)) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	taskID := r.PathValue("task_id")
	var payload struct {
		Comment string `json:"comment"`
	}
	if r.Body != nil && r.ContentLength != 0 {
		_ = json.NewDecoder(r.Body).Decode(&payload)
	}
	resp, err := fn(r.Context(), workflowapp.TaskActionRequest{Subject: sub, TaskID: taskID, Comment: payload.Comment})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"data": resp})
}

func (h *Handler) resolveAssignees(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	var payload struct {
		WorkflowInstanceID string `json:"workflow_instance_id"`
		StepCode           string `json:"step_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	resp, err := h.svc.ResolveAssignees(r.Context(), workflowapp.ResolveAssigneesRequest{Subject: sub, WorkflowInstanceID: payload.WorkflowInstanceID, StepCode: payload.StepCode})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) subjectFromToken(r *http.Request) (workflowapp.Subject, error) {
	tok := bearerToken(r.Header.Get("Authorization"))
	claims, err := h.inspector.InspectAccessToken(r.Context(), tok)
	if err != nil {
		return workflowapp.Subject{}, err
	}
	return workflowapp.Subject{UserID: claims.Sub, MembershipID: claims.MembershipID, CompanyID: claims.CompanyID}, nil
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
