package http

import (
	"context"
	"net/http"
	"sort"
	"strings"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/httpx"
)

type Handler struct {
	inspector iamapp.TokenInspector
	authorizer authapp.Service
	disclosures disclosureapp.Repository
}

func NewHandler(inspector iamapp.TokenInspector, authorizer authapp.Service, disclosures disclosureapp.Repository) *Handler {
	return &Handler{
		inspector:   inspector,
		authorizer:  authorizer,
		disclosures: disclosures,
	}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/platform/cms/dashboard/summary", h.dashboardSummary)
	mux.HandleFunc("GET /api/v1/platform/cms/collections", h.collections)
	mux.HandleFunc("GET /api/v1/platform/cms/entries", h.entries)
}

func (h *Handler) dashboardSummary(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	perms, err := h.requireAnyPermission(r.Context(), sub.MembershipID, sub.CompanyID, "platform.cms.view", "rbac.manage", "system.settings")
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	items, err := h.disclosures.List(r.Context(), sub.CompanyID)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	var draft, published, completed int
	for _, item := range items {
		switch strings.ToLower(strings.TrimSpace(item.Status)) {
		case "draft":
			draft++
		case "published", "submitted":
			published++
		case "completed", "confirmed":
			completed++
		}
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"total":        len(items),
		"draft":        draft,
		"published":    published,
		"completed":    completed,
		"platform_cms": hasAnyPermission(perms, "platform.cms.view", "rbac.manage", "system.settings"),
	})
}

func (h *Handler) collections(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.requireAnyPermission(r.Context(), sub.MembershipID, sub.CompanyID, "disclosure.view", "disclosure.create", "disclosure.edit", "rbac.manage", "system.settings"); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	items, err := h.disclosures.List(r.Context(), sub.CompanyID)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	type collection struct {
		CollectionID   string `json:"collection_id"`
		Name           string `json:"name"`
		EntryCount     int    `json:"entry_count"`
		LatestUpdatedAt string `json:"latest_updated_at,omitempty"`
	}
	agg := map[string]*collection{}
	for _, item := range items {
		key := strings.TrimSpace(item.TypeID)
		if key == "" {
			key = "general"
		}
		cur := agg[key]
		if cur == nil {
			cur = &collection{CollectionID: key, Name: key}
			agg[key] = cur
		}
		cur.EntryCount++
		updated := item.UpdatedAt.UTC().Format(timeLayout)
		if cur.LatestUpdatedAt == "" || updated > cur.LatestUpdatedAt {
			cur.LatestUpdatedAt = updated
		}
	}
	out := make([]collection, 0, len(agg))
	for _, item := range agg {
		out = append(out, *item)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].EntryCount == out[j].EntryCount {
			return out[i].CollectionID < out[j].CollectionID
		}
		return out[i].EntryCount > out[j].EntryCount
	})
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": out})
}

func (h *Handler) entries(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.requireAnyPermission(r.Context(), sub.MembershipID, sub.CompanyID, "disclosure.view", "disclosure.create", "disclosure.edit", "rbac.manage", "system.settings"); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	items, err := h.disclosures.List(r.Context(), sub.CompanyID)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		typeID := strings.TrimSpace(item.TypeID)
		if typeID == "" {
			typeID = "general"
		}
		out = append(out, map[string]any{
			"entry_id":    item.RecordID,
			"title":       item.Title,
			"status":      item.Status,
			"type_id":     typeID,
			"updated_at":  item.UpdatedAt.UTC().Format(timeLayout),
			"company_id":  item.CompanyID,
			"record_id":   item.RecordID,
		})
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": out})
}

func (h *Handler) subject(r *http.Request) (iamapp.AccessTokenClaims, error) {
	claims, err := h.inspector.InspectAccessToken(r.Context(), bearerToken(r.Header.Get("Authorization")))
	if err != nil {
		return iamapp.AccessTokenClaims{}, err
	}
	if claims == nil {
		return iamapp.AccessTokenClaims{}, perr.NewHTTPError(http.StatusUnauthorized, perr.CodeSessionExpired, "invalid token", nil)
	}
	return *claims, nil
}

func (h *Handler) requireAnyPermission(ctx context.Context, membershipID, companyID string, permissions ...string) ([]string, error) {
	eff, err := h.authorizer.GetEffectiveAccess(ctx, membershipID, companyID)
	if err != nil {
		return nil, err
	}
	if !hasAnyPermission(eff.Permissions, permissions...) {
		return nil, perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "access denied", nil)
	}
	return eff.Permissions, nil
}

func hasAnyPermission(items []string, expected ...string) bool {
	for _, candidate := range expected {
		for _, item := range items {
			if strings.EqualFold(strings.TrimSpace(item), strings.TrimSpace(candidate)) {
				return true
			}
		}
	}
	return false
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

const timeLayout = "2006-01-02T15:04:05Z"
