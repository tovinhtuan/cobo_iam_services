package app

import (
	"context"
	"net/http"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

const (
	permissionPlatformCMSView      = "platform.cms.view"
	permissionLegacyTemplateManage = "disclosure_type.manage"
	permissionLegacyConfigManage   = "rbac.manage"

	permissionCMSTemplateRead        = "cms.template.read"
	permissionCMSTemplateWrite       = "cms.template.write"
	permissionCMSTemplateActivate    = "cms.template.activate"
	permissionCMSTemplateArchive     = "cms.template.archive"
	permissionCMSTemplateConfigWrite = "cms.template.config.write"
)

func (s *service) hasAnyPermission(ctx context.Context, sub Subject, permissions ...string) bool {
	for _, permission := range permissions {
		if s.hasPermission(ctx, sub, permission) {
			return true
		}
	}
	return false
}

func newCMSPermissionDenied(message string, required []string, legacy []string) error {
	details := map[string]any{
		"required_permissions": required,
	}
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

func (s *service) requireCMSRouteAccess(ctx context.Context, sub Subject) error {
	if s.hasPermission(ctx, sub, permissionPlatformCMSView) {
		return nil
	}
	return newCMSPermissionDenied(
		"platform CMS access is required",
		[]string{permissionPlatformCMSView},
		nil,
	)
}

func (s *service) requireCMSTemplateRead(ctx context.Context, sub Subject) error {
	if err := s.requireCMSRouteAccess(ctx, sub); err != nil {
		return err
	}
	if s.hasAnyPermission(ctx, sub,
		permissionCMSTemplateRead,
		permissionCMSTemplateWrite,
		permissionCMSTemplateActivate,
		permissionCMSTemplateArchive,
		permissionCMSTemplateConfigWrite,
		permissionLegacyTemplateManage,
		permissionLegacyConfigManage,
	) {
		return nil
	}
	return newCMSPermissionDenied(
		"CMS template read permission is required",
		[]string{permissionCMSTemplateRead},
		[]string{permissionLegacyTemplateManage, permissionLegacyConfigManage},
	)
}

func (s *service) requireCMSTemplateWrite(ctx context.Context, sub Subject) error {
	if err := s.requireCMSRouteAccess(ctx, sub); err != nil {
		return err
	}
	if s.hasAnyPermission(ctx, sub, permissionCMSTemplateWrite, permissionLegacyTemplateManage) {
		return nil
	}
	return newCMSPermissionDenied(
		"CMS template write permission is required",
		[]string{permissionCMSTemplateWrite},
		[]string{permissionLegacyTemplateManage},
	)
}

func (s *service) requireCMSTemplateActivate(ctx context.Context, sub Subject) error {
	if err := s.requireCMSRouteAccess(ctx, sub); err != nil {
		return err
	}
	if s.hasAnyPermission(ctx, sub, permissionCMSTemplateActivate, permissionLegacyTemplateManage) {
		return nil
	}
	return newCMSPermissionDenied(
		"CMS template activate permission is required",
		[]string{permissionCMSTemplateActivate},
		[]string{permissionLegacyTemplateManage},
	)
}

func (s *service) requireCMSTemplateConfigWrite(ctx context.Context, sub Subject) error {
	if err := s.requireCMSRouteAccess(ctx, sub); err != nil {
		return err
	}
	if s.hasAnyPermission(ctx, sub,
		permissionCMSTemplateConfigWrite,
		permissionCMSTemplateWrite,
		permissionLegacyTemplateManage,
		permissionLegacyConfigManage,
	) {
		return nil
	}
	return newCMSPermissionDenied(
		"CMS template config write permission is required",
		[]string{permissionCMSTemplateConfigWrite},
		[]string{permissionCMSTemplateWrite, permissionLegacyTemplateManage, permissionLegacyConfigManage},
	)
}
