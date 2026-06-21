package inmemory

import (
	"context"
	"fmt"
	"net/http"
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

// ─── System Template Archive ──────────────────────────────────────────────────

func (r *Repository) ArchiveGlobalTemplate(_ context.Context, typeID, _ string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.catalog[typeID]; !ok {
		return perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "global template not found", nil)
	}
	item := r.catalog[typeID]
	// Mark as archived by setting a status field via ReviewStatus reuse.
	item.ReviewStatus = "archived"
	r.catalog[typeID] = item
	return nil
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
