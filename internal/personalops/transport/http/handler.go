package http

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/httpx"
	personalopsapp "github.com/cobo/cobo_iam_services/internal/personalops/app"
	"github.com/cobo/cobo_iam_services/internal/personalops/domain"
)

type Handler struct {
	log       *slog.Logger
	svc       personalopsapp.Service
	inspector iamapp.TokenInspector
}

func NewHandler(log *slog.Logger, svc personalopsapp.Service, inspector iamapp.TokenInspector) *Handler {
	return &Handler{log: log, svc: svc, inspector: inspector}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/me/operational-overview", h.getOperationalOverview)
}

func (h *Handler) getOperationalOverview(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	sub, err := h.subjectFromToken(r)
	if err != nil {
		h.log.Warn("personalops: auth failed", slog.Duration("duration", time.Since(start)))
		httpx.WriteError(w, h.log, err)
		return
	}

	resp, err := h.svc.GetOperationalOverview(r.Context(), sub)
	if err != nil {
		h.log.Warn("personalops: overview failed",
			slog.String("user_id", sub.UserID),
			slog.Duration("duration", time.Since(start)),
			slog.String("error", err.Error()),
		)
		httpx.WriteError(w, h.log, err)
		return
	}
	if resp.CompanyOverviews == nil {
		resp.CompanyOverviews = []domain.CompanyOverview{}
	}
	if resp.MyTasks == nil {
		resp.MyTasks = []domain.MyTaskItem{}
	}
	if resp.RoleAssignments == nil {
		resp.RoleAssignments = []domain.RoleAssignment{}
	}
	if resp.AdminScopes == nil {
		resp.AdminScopes = []domain.AdminScope{}
	}
	if resp.Activities == nil {
		resp.Activities = []domain.ActivityItem{}
	}
	if resp.Meta.Warnings == nil {
		resp.Meta.Warnings = []domain.Warning{}
	}
	if resp.Meta.Sources == nil {
		resp.Meta.Sources = []string{}
	}

	h.log.Info("personalops: overview ok",
		slog.String("user_id", sub.UserID),
		slog.Bool("partial", resp.Meta.Partial),
		slog.Int("my_tasks", len(resp.MyTasks)),
		slog.Int("companies", len(resp.CompanyOverviews)),
		slog.Duration("duration", time.Since(start)),
	)
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) subjectFromToken(r *http.Request) (personalopsapp.Subject, error) {
	tok := bearerToken(r.Header.Get("Authorization"))
	if tok == "" {
		return personalopsapp.Subject{}, perr.NewHTTPError(http.StatusUnauthorized, perr.CodeSessionExpired, "authentication required", nil)
	}
	claims, err := h.inspector.InspectAccessToken(r.Context(), tok)
	if err != nil {
		return personalopsapp.Subject{}, err
	}
	return personalopsapp.Subject{
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
