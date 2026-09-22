package inmemory

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// ─── Deadline Rule Catalog ─────────────────────────────────────────────────────

func (r *Repository) ListActiveDeadlineRuleCatalog(_ context.Context) ([]disclosureapp.DeadlineRuleCatalogDTO, error) {
	return disclosureapp.DefaultDeadlineRuleCatalog(), nil
}

func (r *Repository) ListCmsDeadlineRules(_ context.Context) ([]disclosureapp.CmsDeadlineRuleDTO, error) {
	catalog := disclosureapp.DefaultDeadlineRuleCatalog()
	out := make([]disclosureapp.CmsDeadlineRuleDTO, 0, len(catalog))
	for i, d := range catalog {
		out = append(out, disclosureapp.CmsDeadlineRuleDTO{
			RuleID:       fmt.Sprintf("seed-rule-%d", i+1),
			Code:         d.Code,
			LabelVI:      d.LabelVI,
			Pattern:      d.Pattern,
			InputType:    d.InputType,
			IsActive:     true,
			DisplayOrder: i + 1,
			CreatedAt:    time.Now().UTC(),
			UpdatedAt:    time.Now().UTC(),
		})
	}
	return out, nil
}

func (r *Repository) CreateDeadlineRule(_ context.Context, _ disclosureapp.CmsDeadlineRuleCreateRequest, _ string) (*disclosureapp.CmsDeadlineRuleDTO, error) {
	return nil, fmt.Errorf("not implemented in memory")
}

func (r *Repository) UpdateDeadlineRule(_ context.Context, _ disclosureapp.CmsDeadlineRuleUpdateRequest) (*disclosureapp.CmsDeadlineRuleDTO, error) {
	return nil, fmt.Errorf("not implemented in memory")
}

func (r *Repository) DeleteDeadlineRule(_ context.Context, _ string) error {
	return fmt.Errorf("not implemented in memory")
}

// ─── Global Workflows ─────────────────────────────────────────────────────────

func (r *Repository) CountGlobalWorkflowsByTypeId(_ context.Context, typeID string) (int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.globalWorkflows == nil {
		return 0, nil
	}
	if wf, ok := r.globalWorkflows[typeID]; ok && wf.Status == "active" {
		return 1, nil
	}
	return 0, nil
}

func (r *Repository) GetGlobalWorkflow(_ context.Context, typeID string) (*disclosureapp.GlobalWorkflowDTO, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.globalWorkflows == nil {
		return nil, nil
	}
	wf, ok := r.globalWorkflows[typeID]
	if !ok || wf.Status != "active" {
		return nil, nil
	}
	cp := *wf
	cp.Steps = append([]disclosureapp.GlobalWorkflowStepInput(nil), wf.Steps...)
	return &cp, nil
}

func (r *Repository) UpsertGlobalWorkflow(_ context.Context, req disclosureapp.CmsUpsertGlobalWorkflowRequest, workflowID string) (*disclosureapp.GlobalWorkflowDTO, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.globalWorkflows == nil {
		r.globalWorkflows = map[string]*disclosureapp.GlobalWorkflowDTO{}
	}
	now := time.Now().UTC()

	// mig-S1: build the existing step identity map BEFORE overwrite, to preserve step_key
	// across upsert (match by step_key, then step_id). Mirrors the MySQL repository.
	existingKeyByStepID := map[string]string{}
	existingKeys := map[string]bool{}
	// Batch 3: preserve version pointers across save-draft (mirrors MySQL repo).
	var prevPublishedNo, prevActiveNo *int
	if prev, ok := r.globalWorkflows[req.TypeID]; ok && prev != nil {
		for _, s := range prev.Steps {
			if s.StepKey != "" {
				existingKeyByStepID[s.StepID] = s.StepKey
				existingKeys[s.StepKey] = true
			}
		}
		prevPublishedNo = prev.PublishedVersionNo
		prevActiveNo = prev.ActiveVersionNo
	}

	steps := make([]disclosureapp.GlobalWorkflowStepInput, len(req.Steps))
	copy(steps, req.Steps)
	usedKeys := map[string]bool{}
	for i := range steps {
		if steps[i].StepID == "" {
			steps[i].StepID = fmt.Sprintf("%s-step-%d", workflowID, i+1)
		}
		if steps[i].DisplayOrder <= 0 {
			steps[i].DisplayOrder = i + 1
		}
		steps[i].StepKey = disclosureapp.ResolveStepKey(steps[i], existingKeys, existingKeyByStepID, usedKeys)
		usedKeys[steps[i].StepKey] = true
	}
	r.globalWorkflows[req.TypeID] = &disclosureapp.GlobalWorkflowDTO{
		WorkflowID:         workflowID,
		TypeID:             req.TypeID,
		Status:             "active",
		ChangeNote:         req.ChangeNote,
		PublishedVersionNo: prevPublishedNo,
		ActiveVersionNo:    prevActiveNo,
		Steps:              steps,
		CreatedBy:          req.Subject.UserID,
		UpdatedBy:          req.Subject.UserID,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	cp := *r.globalWorkflows[req.TypeID]
	cp.Steps = append([]disclosureapp.GlobalWorkflowStepInput(nil), steps...)
	return &cp, nil
}

func (r *Repository) DeleteGlobalWorkflow(_ context.Context, typeID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.globalWorkflows == nil {
		return perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "global workflow not found", nil)
	}
	if _, ok := r.globalWorkflows[typeID]; !ok {
		return perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "global workflow not found", nil)
	}
	delete(r.globalWorkflows, typeID)
	return nil
}

// ─── System Template Archive / Restore ────────────────────────────────────────

func (r *Repository) ensureTypeRoot(typeID string) *typeRootState {
	if r.typeRoots == nil {
		r.typeRoots = map[string]*typeRootState{}
	}
	root, ok := r.typeRoots[typeID]
	if ok && root != nil {
		return root
	}
	activeNo := 0
	for _, ver := range r.versions[typeID] {
		if ver.IsActive {
			activeNo = ver.VersionNo
			break
		}
	}
	root = &typeRootState{Status: "active", ActiveVersionNo: activeNo}
	if scope := r.catalogScope[typeID]; scope != "" && scope != "global" {
		root.CompanyID = scope
	}
	r.typeRoots[typeID] = root
	return root
}

func (r *Repository) GetTypeLifecycle(_ context.Context, typeID string) (*disclosureapp.TypeLifecycleDTO, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.catalog[typeID]; !ok {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "disclosure type not found", nil)
	}
	root := r.ensureTypeRoot(typeID)
	out := &disclosureapp.TypeLifecycleDTO{
		TypeID:          typeID,
		CompanyID:       root.CompanyID,
		Status:          root.Status,
		ActiveVersionNo: root.ActiveVersionNo,
		ArchiveReason:   root.ArchiveReason,
	}
	if root.ArchivedFromVersionNo != nil {
		v := *root.ArchivedFromVersionNo
		out.ArchivedFromVersionNo = &v
	}
	return out, nil
}

func (r *Repository) ArchiveGlobalTemplate(_ context.Context, params disclosureapp.ArchiveGlobalTemplateParams) (*disclosureapp.ArchiveGlobalTemplateResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	typeID := strings.TrimSpace(params.TypeID)
	if _, ok := r.catalog[typeID]; !ok {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "global template not found", nil)
	}
	if scope := r.catalogScope[typeID]; scope != "" && scope != "global" {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "global template not found", nil)
	}
	root := r.ensureTypeRoot(typeID)
	result := &disclosureapp.ArchiveGlobalTemplateResult{
		TypeID:                typeID,
		BeforeStatus:          root.Status,
		BeforeActiveVersionNo: root.ActiveVersionNo,
		AfterStatus:           "archived",
		AfterActiveVersionNo:  0,
		ArchivedBy:            strings.TrimSpace(params.UpdatedBy),
		ArchiveReason:         strings.TrimSpace(params.Reason),
	}
	if strings.EqualFold(root.Status, "archived") {
		result.AlreadyArchived = true
		result.ArchivedFromVersionNo = cloneIntPtr(root.ArchivedFromVersionNo)
		result.ArchivedAt = root.ArchivedAt
		result.ArchivedBy = root.ArchivedBy
		result.ArchiveReason = root.ArchiveReason
		return result, nil
	}
	if root.ActiveVersionNo > 0 {
		v := root.ActiveVersionNo
		result.ArchivedFromVersionNo = &v
		root.ArchivedFromVersionNo = &v
	} else {
		root.ArchivedFromVersionNo = nil
		result.ArchivedFromVersionNo = nil
	}
	now := time.Now().UTC()
	root.Status = "archived"
	root.ActiveVersionNo = 0
	root.ArchivedAt = &now
	root.ArchivedBy = strings.TrimSpace(params.UpdatedBy)
	root.ArchiveReason = strings.TrimSpace(params.Reason)
	result.ArchivedAt = &now
	item := r.catalog[typeID]
	item.ReviewStatus = "archived"
	r.catalog[typeID] = item
	vs := r.versions[typeID]
	for i := range vs {
		vs[i].IsActive = false
	}
	r.versions[typeID] = vs
	return result, nil
}

func (r *Repository) RestoreGlobalTemplate(_ context.Context, params disclosureapp.RestoreGlobalTemplateParams) (*disclosureapp.RestoreGlobalTemplateResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	typeID := strings.TrimSpace(params.TypeID)
	if _, ok := r.catalog[typeID]; !ok {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "global template not found", nil)
	}
	if scope := r.catalogScope[typeID]; scope != "" && scope != "global" {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "global template not found", nil)
	}
	root := r.ensureTypeRoot(typeID)
	if !strings.EqualFold(root.Status, "archived") {
		return nil, &perr.HTTPError{
			HTTPStatus: http.StatusConflict,
			Code:       perr.CodeStateConflict,
			Message:    "template is not archived; restore is not applicable",
			Details:    map[string]any{"type_id": typeID, "status": root.Status},
		}
	}
	beforeAV := root.ActiveVersionNo
	metaFrom := cloneIntPtr(root.ArchivedFromVersionNo)

	if params.ExpectedFromVersionNo == nil {
		if metaFrom != nil {
			return nil, &perr.HTTPError{
				HTTPStatus: http.StatusConflict,
				Code:       "TEMPLATE_RESTORE_METADATA_MISMATCH",
				Message:    "archived template has prior active version metadata; use published restore path",
				Details:    map[string]any{"type_id": typeID, "archived_from_version_no": *metaFrom},
			}
		}
		root.Status = "active"
		root.ActiveVersionNo = 0
		root.ArchivedFromVersionNo = nil
		root.ArchivedAt = nil
		root.ArchivedBy = ""
		root.ArchiveReason = ""
		item := r.catalog[typeID]
		item.ReviewStatus = ""
		r.catalog[typeID] = item
		return &disclosureapp.RestoreGlobalTemplateResult{
			TypeID: typeID, BeforeStatus: "archived", AfterStatus: "active",
			BeforeActiveVersionNo: beforeAV, AfterActiveVersionNo: 0, RestoredMode: "draft",
		}, nil
	}

	if metaFrom == nil {
		return nil, &perr.HTTPError{
			HTTPStatus: http.StatusConflict,
			Code:       "TEMPLATE_RESTORE_METADATA_MISSING",
			Message:    "cannot restore published template: archived_from_version_no is missing; activate a version manually after review",
			Details:    map[string]any{"type_id": typeID},
		}
	}
	if *metaFrom != *params.ExpectedFromVersionNo || params.RestoreActiveVersionNo != *metaFrom {
		return nil, &perr.HTTPError{
			HTTPStatus: http.StatusConflict,
			Code:       "TEMPLATE_RESTORE_METADATA_MISMATCH",
			Message:    "archived_from_version_no changed or restore version mismatch",
			Details: map[string]any{
				"type_id": typeID, "archived_from_version_no": *metaFrom,
				"expected": *params.ExpectedFromVersionNo, "restore_version_no": params.RestoreActiveVersionNo,
			},
		}
	}
	restoreVer := params.RestoreActiveVersionNo
	found := false
	vs := r.versions[typeID]
	for i := range vs {
		vs[i].IsActive = vs[i].VersionNo == restoreVer
		if vs[i].VersionNo == restoreVer {
			found = true
			vs[i].IsReleased = true
			vs[i].ActivatedAt = time.Now().UTC()
			if strings.TrimSpace(params.UpdatedBy) != "" {
				vs[i].UpdatedBy = params.UpdatedBy
			}
		}
	}
	if !found {
		if _, ok := r.catalogByVer[typeID][restoreVer]; !ok {
			return nil, &perr.HTTPError{
				HTTPStatus: http.StatusConflict,
				Code:       "TEMPLATE_RESTORE_VERSION_MISSING",
				Message:    "archived version no longer exists; cannot restore automatically",
				Details:    map[string]any{"type_id": typeID, "version_no": restoreVer},
			}
		}
	}
	r.versions[typeID] = vs
	if snap, ok := r.catalogByVer[typeID][restoreVer]; ok {
		r.catalog[typeID] = snap
	}
	root.Status = "active"
	root.ActiveVersionNo = restoreVer
	root.ArchivedFromVersionNo = nil
	root.ArchivedAt = nil
	root.ArchivedBy = ""
	root.ArchiveReason = ""
	item := r.catalog[typeID]
	item.ReviewStatus = ""
	r.catalog[typeID] = item
	return &disclosureapp.RestoreGlobalTemplateResult{
		TypeID: typeID, BeforeStatus: "archived", AfterStatus: "active",
		BeforeActiveVersionNo: beforeAV, AfterActiveVersionNo: restoreVer,
		ArchivedFromVersionNo: metaFrom, RestoredMode: "active",
	}, nil
}

func cloneIntPtr(v *int) *int {
	if v == nil {
		return nil
	}
	x := *v
	return &x
}

// ─── Display Group CRUD ────────────────────────────────────────────────────────

func (r *Repository) CreateDisplayGroup(_ context.Context, req disclosureapp.CmsDisplayGroupCreateRequest) (*disclosureapp.DisplayGroupDTO, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, g := range r.displayGroups {
		if g.DisplayGroupCode == req.Code {
			return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "display group code already exists", nil)
		}
	}
	d := disclosureapp.DisplayGroupDTO{
		DisplayGroupCode: req.Code,
		NameVI:           req.NameVI,
		NameEN:           req.NameEN,
		Description:      req.Description,
		Icon:             req.Icon,
		DisplayOrder:     req.DisplayOrder,
		IsActive:         true,
		IsSystem:         false,
	}
	r.displayGroups = append(r.displayGroups, d)
	cp := d
	return &cp, nil
}

func (r *Repository) UpdateDisplayGroup(_ context.Context, req disclosureapp.CmsDisplayGroupUpdateRequest) (*disclosureapp.DisplayGroupDTO, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, g := range r.displayGroups {
		if g.DisplayGroupCode == req.Code {
			g.NameVI = req.NameVI
			g.NameEN = req.NameEN
			g.Description = req.Description
			g.Icon = req.Icon
			g.DisplayOrder = req.DisplayOrder
			if req.IsActive != nil {
				g.IsActive = *req.IsActive
			}
			r.displayGroups[i] = g
			cp := g
			return &cp, nil
		}
	}
	return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "display group not found", nil)
}

func (r *Repository) DeleteDisplayGroup(_ context.Context, code string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, g := range r.displayGroups {
		if g.DisplayGroupCode == code && !g.IsSystem {
			r.displayGroups = append(r.displayGroups[:i], r.displayGroups[i+1:]...)
			return nil
		}
	}
	return perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "display group not found or is system-protected", nil)
}

// ─── Template default department catalog ───────────────────────────────────────

func (r *Repository) ListTemplateDepartments(_ context.Context) ([]disclosureapp.TemplateDepartmentDTO, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]disclosureapp.TemplateDepartmentDTO, len(r.templateDepartments))
	copy(out, r.templateDepartments)
	return out, nil
}

func (r *Repository) CreateTemplateDepartment(_ context.Context, req disclosureapp.CmsTemplateDepartmentCreateRequest) (*disclosureapp.TemplateDepartmentDTO, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, d := range r.templateDepartments {
		if d.DepartmentCode == req.Code {
			return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "department code already exists", nil)
		}
		if strings.EqualFold(strings.TrimSpace(d.DepartmentName), strings.TrimSpace(req.Name)) {
			return nil, &perr.HTTPError{
				HTTPStatus: http.StatusConflict,
				Code:       perr.CodeStateConflict,
				Message:    "template department name already exists",
				Details:    map[string]any{"field": "name"},
			}
		}
	}
	d := disclosureapp.TemplateDepartmentDTO{
		DepartmentCode: req.Code,
		DepartmentName: req.Name,
		Description:    req.Description,
		DisplayOrder:   req.DisplayOrder,
		IsSystem:       false,
	}
	r.templateDepartments = append(r.templateDepartments, d)
	cp := d
	return &cp, nil
}
