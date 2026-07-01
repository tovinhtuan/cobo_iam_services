package app

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/cobo/cobo_iam_services/internal/companyaccess/configexport"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

func (s *adminService) CreateConfigExport(ctx context.Context, req CreateConfigExportRequest) (*ConfigExportJobView, error) {
	if err := s.authorizeConfigExport(ctx, req.Subject); err != nil {
		return nil, err
	}
	companyID := strings.TrimSpace(req.Subject.CompanyID)
	if companyID == "" {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeNotFound, "company context required", nil)
	}

	modules, err := resolveConfigExportModules(req.Modules)
	if err != nil {
		return nil, err
	}

	data := make(map[string]any)
	warnings := make([]string, 0)

	for _, mod := range modules {
		moduleData, moduleWarnings, moduleErr := s.buildConfigExportModule(ctx, companyID, mod)
		if moduleErr != nil {
			if isNotFound(moduleErr) && len(modules) > 1 {
				warnings = append(warnings, mod+": active configuration not found")
				continue
			}
			return nil, moduleErr
		}
		warnings = append(warnings, moduleWarnings...)
		canonical, err := canonicalizeModuleData(mod, moduleData)
		if err != nil {
			return nil, err
		}
		data[mod] = canonical
	}

	if len(data) == 0 {
		return nil, perr.NewHTTPError(http.StatusUnprocessableEntity, perr.CodeInvalidRequest, "no exportable active configuration", nil)
	}

	exportedModules := make([]string, 0, len(modules))
	for _, mod := range modules {
		if _, ok := data[mod]; ok {
			exportedModules = append(exportedModules, mod)
		}
	}

	checksum, err := computeConfigExportChecksum(exportedModules, data, warnings)
	if err != nil {
		return nil, err
	}

	exportedAt := time.Now().UTC()
	exportID := s.idg.NewUUID()
	artifactJSON, err := marshalExportArtifact(
		exportedModules,
		data,
		warnings,
		exportedAt.Format(time.RFC3339),
		req.Subject.MembershipID,
		checksum,
	)
	if err != nil {
		return nil, err
	}

	job := &configExportJob{
		ExportID:               exportID,
		CompanyID:              companyID,
		ExportedByMembershipID: req.Subject.MembershipID,
		SchemaVersion:          configexport.SchemaVersionEnterpriseExport,
		PackageType:            configexport.PackageTypeEnterpriseExport,
		Modules:                append([]string(nil), exportedModules...),
		Checksum:               checksum,
		Warnings:               append([]string(nil), warnings...),
		ArtifactJSON:           artifactJSON,
		CreatedAt:              exportedAt,
		Status:                 "completed",
	}
	if s.exportStore == nil {
		s.exportStore = newConfigExportStore()
	}
	s.exportStore.put(job)

	return configExportJobToView(job), nil
}

func (s *adminService) GetConfigExport(ctx context.Context, req GetConfigExportRequest) (*ConfigExportJobView, error) {
	if err := s.authorizeConfigExport(ctx, req.Subject); err != nil {
		return nil, err
	}
	job, err := s.loadConfigExportJob(req.Subject.CompanyID, req.ExportID)
	if err != nil {
		return nil, err
	}
	return configExportJobToView(job), nil
}

func (s *adminService) DownloadConfigExport(ctx context.Context, req DownloadConfigExportRequest) ([]byte, error) {
	if err := s.authorizeConfigExport(ctx, req.Subject); err != nil {
		return nil, err
	}
	job, err := s.loadConfigExportJob(req.Subject.CompanyID, req.ExportID)
	if err != nil {
		return nil, err
	}
	return append([]byte(nil), job.ArtifactJSON...), nil
}

func (s *adminService) authorizeConfigExport(ctx context.Context, sub AdminSubject) error {
	if err := s.requireActiveCompanyMember(ctx, sub); err != nil {
		return err
	}
	ok, err := s.hasPermissionRBACOnly(ctx, sub, "rbac.manage")
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	ok, err = s.hasPermissionRBACOnly(ctx, sub, "system.settings")
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	return perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "access denied", nil)
}

func resolveConfigExportModules(requested []string) ([]string, error) {
	if len(requested) == 0 {
		return append([]string(nil), configexport.DefaultModules...), nil
	}
	allowed := map[string]struct{}{
		configexport.ModuleNotificationAlertChannelPrefs: {},
		configexport.ModuleRBACMatrix:                    {},
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(requested))
	for _, raw := range requested {
		mod := strings.TrimSpace(raw)
		if mod == "" {
			continue
		}
		if _, ok := allowed[mod]; !ok {
			return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid module selection", nil)
		}
		if _, dup := seen[mod]; dup {
			continue
		}
		seen[mod] = struct{}{}
		out = append(out, mod)
	}
	if len(out) == 0 {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid module selection", nil)
	}
	// Preserve canonical module order for determinism.
	ordered := make([]string, 0, len(out))
	for _, mod := range configexport.DefaultModules {
		if _, ok := seen[mod]; ok {
			ordered = append(ordered, mod)
		}
	}
	return ordered, nil
}

func (s *adminService) buildConfigExportModule(ctx context.Context, companyID, module string) (map[string]any, []string, error) {
	warnings := make([]string, 0)
	switch module {
	case configexport.ModuleNotificationAlertChannelPrefs:
		rule, err := s.repo.GetNotificationRuleByCode(ctx, companyID, AlertChannelPrefsRuleCode)
		if err != nil {
			return nil, nil, err
		}
		if rule == nil {
			return nil, nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeNotFound, "notification rule not found", nil)
		}
		raw, err := s.repo.BuildNotificationRuleSnapshotJSON(ctx, companyID, rule.NotificationRuleID)
		if err != nil {
			return nil, nil, err
		}
		data, err := sanitizeConfigExportModuleData(raw, &warnings)
		return data, warnings, err
	case configexport.ModuleRBACMatrix:
		raw, err := s.repo.BuildRBACMatrixSnapshotJSON(ctx, companyID)
		if err != nil {
			return nil, nil, err
		}
		data, err := sanitizeConfigExportModuleData(raw, &warnings)
		return data, warnings, err
	default:
		return nil, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid module selection", nil)
	}
}

func (s *adminService) loadConfigExportJob(companyID, exportID string) (*configExportJob, error) {
	if s.exportStore == nil {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeNotFound, "export not found", nil)
	}
	job, ok := s.exportStore.get(companyID, strings.TrimSpace(exportID))
	if !ok {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeNotFound, "export not found", nil)
	}
	return job, nil
}

func configExportJobToView(job *configExportJob) *ConfigExportJobView {
	return &ConfigExportJobView{
		ExportID:               job.ExportID,
		SchemaVersion:          job.SchemaVersion,
		PackageType:            job.PackageType,
		Modules:                append([]string(nil), job.Modules...),
		ExportedAt:             job.CreatedAt,
		ExportedByMembershipID: job.ExportedByMembershipID,
		Checksum:               job.Checksum,
		Warnings:               append([]string(nil), job.Warnings...),
		Status:                 job.Status,
	}
}

func isNotFound(err error) bool {
	if he, ok := err.(*perr.HTTPError); ok {
		return he.HTTPStatus == http.StatusNotFound
	}
	return false
}

// ConfigExportAuditMetadata returns safe audit metadata (no artifact JSON).
func ConfigExportAuditMetadata(view *ConfigExportJobView) map[string]any {
	if view == nil {
		return map[string]any{}
	}
	return map[string]any{
		"export_id":      view.ExportID,
		"schema_version": view.SchemaVersion,
		"modules":        view.Modules,
		"checksum":       view.Checksum,
		"warning_count":  len(view.Warnings),
		"status":         view.Status,
	}
}

// ParseConfigExportArtifact validates downloadable JSON shape (tests).
func ParseConfigExportArtifact(raw []byte) (map[string]any, error) {
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}
