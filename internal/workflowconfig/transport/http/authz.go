package http

import (
	"context"
	"net/http"
	"strings"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// Permission ladder mirrors disclosure/app/cms_template_permissions.go (workflowconfig must not import
// disclosure). Every gated route requires platform.cms.view first, then the template capability.
const (
	permPlatformCMSView     = "platform.cms.view"
	permCMSTemplateRead     = "cms.template.read"
	permCMSTemplateWrite    = "cms.template.write"
	permCMSTemplateActivate = "cms.template.activate"
	permCMSTemplateArchive  = "cms.template.archive"
	permCMSTemplateConfig   = "cms.template.config.write"
	permLegacyPublish       = "disclosure_type.publish"
	permLegacyConfigManage  = "rbac.manage"
)

// AccessResolver is the subset of authapp.Service needed to gate platform CMS routes.
type AccessResolver interface {
	GetEffectiveAccess(ctx context.Context, membershipID, companyID string) (*authapp.EffectiveAccessSummary, error)
}

type subject struct {
	UserID       string
	MembershipID string
	CompanyID    string
}

func (h *Handler) subject(r *http.Request) (subject, error) {
	claims, err := h.inspector.InspectAccessToken(r.Context(), bearer(r.Header.Get("Authorization")))
	if err != nil {
		return subject{}, err
	}
	return subject{UserID: claims.Sub, MembershipID: claims.MembershipID, CompanyID: claims.CompanyID}, nil
}

// authorize authenticates the caller and requires platform.cms.view plus one of capabilities.
// A nil authorizer fails closed (503).
func (h *Handler) authorize(r *http.Request, message string, required, legacy []string) (subject, error) {
	sub, err := h.subject(r)
	if err != nil {
		return subject{}, err
	}
	if h.authorizer == nil {
		return subject{}, perr.NewHTTPError(http.StatusServiceUnavailable, perr.CodeInternal, "authorizer unavailable", nil)
	}
	eff, err := h.authorizer.GetEffectiveAccess(r.Context(), sub.MembershipID, sub.CompanyID)
	if err != nil {
		return subject{}, err
	}
	if !hasAnyPermission(eff.Permissions, permPlatformCMSView) {
		return subject{}, permissionDenied("platform CMS access is required", []string{permPlatformCMSView}, nil)
	}
	if !hasAnyPermission(eff.Permissions, append(append([]string{}, required...), legacy...)...) {
		return subject{}, permissionDenied(message, required[:1], legacy)
	}
	return sub, nil
}

func (h *Handler) requireTemplateRead(r *http.Request) (subject, error) {
	return h.authorize(r, "CMS template read permission is required",
		[]string{permCMSTemplateRead, permCMSTemplateWrite, permCMSTemplateActivate, permCMSTemplateArchive, permCMSTemplateConfig},
		[]string{permLegacyConfigManage})
}

func (h *Handler) requireTemplateWrite(r *http.Request) (subject, error) {
	// ROLE-08: disclosure_type.manage (tenant-grantable) no longer opens the global CMS templates.
	return h.authorize(r, "CMS template write permission is required",
		[]string{permCMSTemplateWrite},
		nil)
}

// requireTemplateActivate is the checker capability; the maker permission (write/manage) is not enough.
func (h *Handler) requireTemplateActivate(r *http.Request) (subject, error) {
	return h.authorize(r, "CMS template activate permission is required",
		[]string{permCMSTemplateActivate},
		[]string{permLegacyPublish})
}

func permissionDenied(message string, required, legacy []string) error {
	details := map[string]any{"required_permissions": required}
	if len(legacy) > 0 {
		details["accepted_legacy_permissions"] = legacy
	}
	return &perr.HTTPError{
		Code:       perr.CodePermissionDenied,
		Message:    message,
		HTTPStatus: http.StatusForbidden,
		Details:    details,
	}
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
