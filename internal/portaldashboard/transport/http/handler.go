package http

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	portaldashboardapp "github.com/cobo/cobo_iam_services/internal/portaldashboard/app"
	"github.com/cobo/cobo_iam_services/internal/portaldashboard/domain"
	"github.com/cobo/cobo_iam_services/internal/portaldashboard/observability"
	"github.com/cobo/cobo_iam_services/internal/platform/httpx"
)

type Handler struct {
	log       *slog.Logger
	svc       portaldashboardapp.Service
	inspector iamapp.TokenInspector
}

func NewHandler(log *slog.Logger, svc portaldashboardapp.Service, inspector iamapp.TokenInspector) *Handler {
	return &Handler{log: log, svc: svc, inspector: inspector}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/company/dashboard/overview", h.getOverview)
}

func (h *Handler) getOverview(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	q := r.URL.Query()
	rangePreset := strings.TrimSpace(q.Get("range"))
	if rangePreset == "" {
		rangePreset = "30d"
	}

	sub, err := h.subjectFromToken(r)
	if err != nil {
		observability.RecordRequest("401", rangePreset, time.Since(start), false)
		h.log.Warn("portaldashboard: overview auth failed",
			slog.String("range", rangePreset),
			slog.Duration("duration", time.Since(start)),
		)
		httpx.WriteError(w, h.log, err)
		return
	}

	resp, err := h.svc.GetOverview(r.Context(), sub, domain.ParseRangeInput{
		Range:    rangePreset,
		From:     strings.TrimSpace(q.Get("from")),
		To:       strings.TrimSpace(q.Get("to")),
		Timezone: strings.TrimSpace(q.Get("timezone")),
		Now:      time.Now().UTC(),
	})
	if err != nil {
		status := "500"
		if he, ok := perr.AsHTTPError(err); ok {
			status = statusCodeString(he.HTTPStatus)
		}
		observability.RecordRequest(status, rangePreset, time.Since(start), false)
		h.log.Warn("portaldashboard: overview failed",
			slog.String("company_id", sub.CompanyID),
			slog.String("membership_id", sub.MembershipID),
			slog.String("range", rangePreset),
			slog.Duration("duration", time.Since(start)),
			slog.String("error", err.Error()),
		)
		httpx.WriteError(w, h.log, err)
		return
	}
	if resp.ImmediateActions == nil {
		resp.ImmediateActions = []domain.ImmediateActionItem{}
	}
	if resp.FrequentLateFlows == nil {
		resp.FrequentLateFlows = []domain.WorkflowRiskRow{}
	}
	if resp.DepartmentRisks == nil {
		resp.DepartmentRisks = []domain.DepartmentRiskRow{}
	}
	if resp.RecentActivities == nil {
		resp.RecentActivities = []domain.RecentActivityItem{}
	}
	if resp.Exceptions == nil {
		resp.Exceptions = []domain.ExceptionItem{}
	}
	if resp.Meta.Sources == nil {
		resp.Meta.Sources = []string{}
	}
	if resp.Meta.Warnings == nil {
		resp.Meta.Warnings = []string{}
	}
	observability.RecordRequest("200", rangePreset, time.Since(start), resp.Meta.Partial)
	h.log.Info("portaldashboard: overview ok",
		slog.String("company_id", sub.CompanyID),
		slog.String("membership_id", sub.MembershipID),
		slog.String("range", rangePreset),
		slog.Bool("partial", resp.Meta.Partial),
		slog.Int("warnings", len(resp.Meta.Warnings)),
		slog.Duration("duration", time.Since(start)),
	)
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func statusCodeString(code int) string {
	switch code {
	case http.StatusUnauthorized:
		return "401"
	case http.StatusForbidden:
		return "403"
	case http.StatusUnprocessableEntity:
		return "422"
	case http.StatusNotFound:
		return "404"
	default:
		return http.StatusText(code)
	}
}

func (h *Handler) subjectFromToken(r *http.Request) (portaldashboardapp.Subject, error) {
	tok := bearerToken(r.Header.Get("Authorization"))
	claims, err := h.inspector.InspectAccessToken(r.Context(), tok)
	if err != nil {
		return portaldashboardapp.Subject{}, err
	}
	return portaldashboardapp.Subject{
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
	return ""
}
