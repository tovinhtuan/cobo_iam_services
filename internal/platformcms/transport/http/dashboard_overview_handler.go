package http

import (
	"net/http"
	"strings"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/httpx"
	platformcmsapp "github.com/cobo/cobo_iam_services/internal/platformcms/app"
)

// WithDashboardOverviewService wires GET /dashboard/overview (tests or custom repositories).
func (h *Handler) WithDashboardOverviewService(svc *platformcmsapp.DashboardOverviewService) *Handler {
	if h != nil {
		h.dashboardOverviewSvc = svc
	}
	return h
}

// dashboardOverview serves platform-wide aggregates for the /cms home screen.
// Route gate: platform.cms.view; each section is gated again inside the service.
func (h *Handler) dashboardOverview(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	perms, err := h.requireCMSAccess(r.Context(), sub.MembershipID, sub.CompanyID)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if h.dashboardOverviewSvc == nil {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusServiceUnavailable, perr.CodeServiceUnavailable, "dashboard overview is not configured", nil))
		return
	}
	refresh := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("refresh")), "true")
	overview := h.dashboardOverviewSvc.Overview(r.Context(), platformcmsapp.DashboardOverviewRequest{
		UserID:      sub.Sub,
		Permissions: perms,
		Refresh:     refresh,
	})
	writeEnvelope(w, http.StatusOK, map[string]any{
		"generated_at": overview.GeneratedAt,
		"cached":       overview.Cached,
		"timezone":     overview.Timezone,
		"sections":     overview.SectionsPayload(),
	}, nil)
}
