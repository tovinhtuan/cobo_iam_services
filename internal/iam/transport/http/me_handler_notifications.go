package http

import (
	"net/http"
	"strings"
	"time"

	inappapp "github.com/cobo/cobo_iam_services/internal/inappnotification/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/httpx"
)

func (m *MeHandler) notificationList(w http.ResponseWriter, r *http.Request) {
	claims, err := m.h.inspector.InspectAccessToken(r.Context(), bearerToken(r.Header.Get("Authorization")))
	if err != nil {
		httpx.WriteError(w, m.h.log, err)
		return
	}
	if m.inAppNotifSvc == nil {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"notifications": []any{}, "total": 0})
		return
	}
	items, err := m.inAppNotifSvc.List(r.Context(), claims.Sub, claims.CompanyID)
	if err != nil {
		httpx.WriteError(w, m.h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"notifications": toNotifPayloads(items),
		"total":         len(items),
	})
}

func (m *MeHandler) notificationUnreadCount(w http.ResponseWriter, r *http.Request) {
	claims, err := m.h.inspector.InspectAccessToken(r.Context(), bearerToken(r.Header.Get("Authorization")))
	if err != nil {
		httpx.WriteError(w, m.h.log, err)
		return
	}
	if m.inAppNotifSvc == nil {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"count": 0})
		return
	}
	count, err := m.inAppNotifSvc.UnreadCount(r.Context(), claims.Sub, claims.CompanyID)
	if err != nil {
		httpx.WriteError(w, m.h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"count": count})
}

func (m *MeHandler) notificationMarkRead(w http.ResponseWriter, r *http.Request) {
	claims, err := m.h.inspector.InspectAccessToken(r.Context(), bearerToken(r.Header.Get("Authorization")))
	if err != nil {
		httpx.WriteError(w, m.h.log, err)
		return
	}
	notifID := strings.TrimSpace(r.PathValue("id"))
	if notifID == "" {
		httpx.WriteError(w, m.h.log, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "notification id required", nil))
		return
	}
	if m.inAppNotifSvc == nil {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
		return
	}
	if err := m.inAppNotifSvc.MarkRead(r.Context(), claims.Sub, notifID); err != nil {
		httpx.WriteError(w, m.h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (m *MeHandler) notificationMarkAllRead(w http.ResponseWriter, r *http.Request) {
	claims, err := m.h.inspector.InspectAccessToken(r.Context(), bearerToken(r.Header.Get("Authorization")))
	if err != nil {
		httpx.WriteError(w, m.h.log, err)
		return
	}
	if m.inAppNotifSvc == nil {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
		return
	}
	if err := m.inAppNotifSvc.MarkAllRead(r.Context(), claims.Sub, claims.CompanyID); err != nil {
		httpx.WriteError(w, m.h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func toNotifPayloads(items []inappapp.InAppNotification) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, n := range items {
		p := map[string]any{
			"id":         n.ID,
			"kind":       n.Kind,
			"title":      n.Title,
			"body":       n.Body,
			"is_read":    n.IsRead,
			"created_at": n.CreatedAt.UTC().Format(time.RFC3339),
		}
		if n.ResourceType != nil {
			p["resource_type"] = *n.ResourceType
		} else {
			p["resource_type"] = nil
		}
		if n.ResourceID != nil {
			p["resource_id"] = *n.ResourceID
		} else {
			p["resource_id"] = nil
		}
		out = append(out, p)
	}
	return out
}
