package inmemory

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

type Repository struct {
	mu                    sync.RWMutex
	items                 map[string]disclosureapp.RecordDTO
	groups                []disclosureapp.DisclosureGroupDTO
	catalog               map[string]disclosureapp.DisclosureTypeDTO
	catalogByVer          map[string]map[int]disclosureapp.DisclosureTypeDTO
	versions              map[string][]disclosureapp.DisclosureTypeVersionDTO
	catalogScope          map[string]string
	overrideByCompanyType map[string]*overrideState
}

type overrideState struct {
	header   disclosureapp.CompanyWorkflowOverrideHeaderDTO
	versions map[int]disclosureapp.CompanyWorkflowOverrideVersionDTO
}

func NewRepository() *Repository {
	repo := &Repository{
		items:                 map[string]disclosureapp.RecordDTO{},
		groups:                disclosureapp.SeedDisclosureTypeGroups(),
		catalog:               map[string]disclosureapp.DisclosureTypeDTO{},
		catalogByVer:          map[string]map[int]disclosureapp.DisclosureTypeDTO{},
		versions:              map[string][]disclosureapp.DisclosureTypeVersionDTO{},
		catalogScope:          map[string]string{},
		overrideByCompanyType: map[string]*overrideState{},
	}
	for _, item := range disclosureapp.SeedDisclosureTypeCatalog() {
		item.VersionNo = 1
		repo.catalog[item.TypeID] = item
		repo.catalogByVer[item.TypeID] = map[int]disclosureapp.DisclosureTypeDTO{1: item}
		repo.catalogScope[item.TypeID] = "global"
		repo.versions[item.TypeID] = []disclosureapp.DisclosureTypeVersionDTO{
			{
				TypeID:      item.TypeID,
				VersionNo:   1,
				IsActive:    true,
				ChangeNote:  "seed",
				UpdatedBy:   "system",
				ActivatedAt: time.Now().UTC(),
			},
		}
	}
	return repo
}

func overrideKey(companyID, typeID string) string { return companyID + ":" + typeID }

func key(companyID, recordID string) string { return companyID + ":" + recordID }

func (r *Repository) Create(_ context.Context, rec disclosureapp.RecordDTO) (*disclosureapp.RecordDTO, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items[key(rec.CompanyID, rec.RecordID)] = rec
	cp := rec
	return &cp, nil
}

func (r *Repository) Update(_ context.Context, rec disclosureapp.RecordDTO) (*disclosureapp.RecordDTO, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := key(rec.CompanyID, rec.RecordID)
	if _, ok := r.items[k]; !ok {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "record not found", nil)
	}
	r.items[k] = rec
	cp := rec
	return &cp, nil
}

func (r *Repository) FindByID(_ context.Context, companyID, recordID string) (*disclosureapp.RecordDTO, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	it, ok := r.items[key(companyID, recordID)]
	if !ok {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "record not found", nil)
	}
	cp := it
	return &cp, nil
}

func (r *Repository) List(_ context.Context, companyID string) ([]disclosureapp.RecordDTO, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]disclosureapp.RecordDTO, 0)
	for _, it := range r.items {
		if it.CompanyID == companyID {
			out = append(out, it)
		}
	}
	return out, nil
}

func (r *Repository) ListTypeGroups(_ context.Context, _ string) ([]disclosureapp.DisclosureGroupDTO, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]disclosureapp.DisclosureGroupDTO, len(r.groups))
	copy(out, r.groups)
	return out, nil
}

func (r *Repository) ListTypes(_ context.Context, companyID, groupID, query string) ([]disclosureapp.DisclosureTypeSummaryDTO, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	groupID = strings.TrimSpace(groupID)
	query = strings.ToLower(strings.TrimSpace(query))
	out := make([]disclosureapp.DisclosureTypeSummaryDTO, 0)
	for _, item := range r.catalog {
		if scope := r.catalogScope[item.TypeID]; scope != "global" && scope != companyID {
			continue
		}
		if groupID != "" && item.GroupID != groupID {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(item.Name+" "+item.Description), query) {
			continue
		}
		out = append(out, disclosureapp.DisclosureTypeSummaryDTO{
			TypeID:           item.TypeID,
			GroupID:          item.GroupID,
			Scope:            func() string { if r.catalogScope[item.TypeID] == "global" { return "global" }; return "company" }(),
			OwnerCompanyID:   func() string { if r.catalogScope[item.TypeID] == "global" { return "" }; return r.catalogScope[item.TypeID] }(),
			Name:             item.Name,
			Category:         item.Category,
			TemplateCategory: item.TemplateCategory,
			Description:      item.Description,
			DeadlineRule:     item.DeadlineRule,
			Tags:             slices.Clone(item.Tags),
		})
	}
	return out, nil
}

func (r *Repository) GetTypeDetail(_ context.Context, companyID, typeID string) (*disclosureapp.DisclosureTypeDTO, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	item, ok := r.catalog[typeID]
	if !ok {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "disclosure type not found", nil)
	}
	if scope := r.catalogScope[typeID]; scope != "global" && scope != companyID {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "disclosure type not found", nil)
	}
	cp := item
	if r.catalogScope[typeID] == "global" {
		cp.Scope = "global"
		cp.OwnerCompanyID = ""
	} else {
		cp.Scope = "company"
		cp.OwnerCompanyID = r.catalogScope[typeID]
	}
	cp.Tags = slices.Clone(item.Tags)
	cp.ReminderMilestones = slices.Clone(item.ReminderMilestones)
	cp.LegalBases = slices.Clone(item.LegalBases)
	cp.Checklist = slices.Clone(item.Checklist)
	cp.Blocks = cloneTemplateBlocks(item.Blocks)
	return &cp, nil
}

func (r *Repository) GetTypeVersionDetail(_ context.Context, companyID, typeID string, versionNo int) (*disclosureapp.DisclosureTypeDTO, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if scope := r.catalogScope[typeID]; scope != "global" && scope != companyID {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "disclosure type version not found", nil)
	}
	byVersion, ok := r.catalogByVer[typeID]
	if !ok {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "disclosure type version not found", nil)
	}
	item, ok := byVersion[versionNo]
	if !ok {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "disclosure type version not found", nil)
	}
	cp := item
	if r.catalogScope[typeID] == "global" {
		cp.Scope = "global"
		cp.OwnerCompanyID = ""
	} else {
		cp.Scope = "company"
		cp.OwnerCompanyID = r.catalogScope[typeID]
	}
	cp.Tags = slices.Clone(item.Tags)
	cp.ReminderMilestones = slices.Clone(item.ReminderMilestones)
	cp.LegalBases = slices.Clone(item.LegalBases)
	cp.Checklist = slices.Clone(item.Checklist)
	cp.Blocks = cloneTemplateBlocks(item.Blocks)
	return &cp, nil
}

func (r *Repository) UpsertTypeVersion(_ context.Context, req disclosureapp.UpsertTypeVersionRequest) (*disclosureapp.UpsertTypeVersionResponse, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	current, existed := r.catalog[req.TypeID]
	versionNo := 1
	if existed {
		versionNo = current.VersionNo + 1
	}
	next := disclosureapp.DisclosureTypeDTO{
		VersionNo:             versionNo,
		TypeID:                req.TypeID,
		GroupID:               req.GroupID,
		Name:                  req.Name,
		Category:              req.Category,
		TemplateCategory:      req.TemplateCategory,
		DeadlineStrategy:      req.DeadlineStrategy,
		Description:           req.Description,
		LegalBasis:            req.LegalBasis,
		Applicability:         req.Applicability,
		ImplementationContent: req.ImplementationContent,
		ImplementationNotes:   req.ImplementationNotes,
		SpecialCases:          req.SpecialCases,
		ReportContent:         req.ReportContent,
		RequiredDocs:          req.RequiredDocs,
		DeadlineRule:          req.DeadlineRule,
		Periodicity:           req.Periodicity,
		ChannelsText:          req.ChannelsText,
		Beneficiaries:         req.Beneficiaries,
		ReceivingAuthorities:  req.ReceivingAuthorities,
		Format:                req.Format,
		LegalRisksText:        req.LegalRisksText,
		GeneralInfo:           req.GeneralInfo,
		ReminderMilestones:    slices.Clone(req.ReminderMilestones),
		LegalBases:            slices.Clone(req.LegalBases),
		Checklist:             slices.Clone(req.Checklist),
		Tags:                  slices.Clone(req.Tags),
		Blocks:                cloneTemplateBlocks(req.Blocks),
	}
	r.catalog[req.TypeID] = next
	if _, ok := r.catalogByVer[req.TypeID]; !ok {
		r.catalogByVer[req.TypeID] = map[int]disclosureapp.DisclosureTypeDTO{}
	}
	r.catalogByVer[req.TypeID][versionNo] = next
	if strings.EqualFold(strings.TrimSpace(req.Scope), "company") {
		r.catalogScope[req.TypeID] = req.Subject.CompanyID
	} else {
		r.catalogScope[req.TypeID] = "global"
	}

	vs := r.versions[req.TypeID]
	for i := range vs {
		vs[i].IsActive = false
	}
	now := time.Now().UTC()
	vs = append(vs, disclosureapp.DisclosureTypeVersionDTO{
		TypeID:      req.TypeID,
		VersionNo:   versionNo,
		IsActive:    true,
		ChangeNote:  strings.TrimSpace(req.ChangeNote),
		UpdatedBy:   req.Subject.UserID,
		ActivatedAt: now,
	})
	r.versions[req.TypeID] = vs

	return &disclosureapp.UpsertTypeVersionResponse{
		TypeID:      req.TypeID,
		VersionNo:   versionNo,
		IsActive:    true,
		UpdatedBy:   req.Subject.UserID,
		ActivatedAt: now,
	}, nil
}

func (r *Repository) ListTypeVersions(_ context.Context, companyID, typeID string) ([]disclosureapp.DisclosureTypeVersionDTO, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if _, ok := r.catalog[typeID]; !ok {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "disclosure type not found", nil)
	}
	if scope := r.catalogScope[typeID]; scope != "global" && scope != companyID {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "disclosure type not found", nil)
	}
	vs := r.versions[typeID]
	out := make([]disclosureapp.DisclosureTypeVersionDTO, len(vs))
	copy(out, vs)
	slices.SortFunc(out, func(a, b disclosureapp.DisclosureTypeVersionDTO) int {
		return b.VersionNo - a.VersionNo
	})
	return out, nil
}

func (r *Repository) ActivateTypeVersion(_ context.Context, req disclosureapp.ActivateTypeVersionRequest) (*disclosureapp.ActivateTypeVersionResponse, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	current, ok := r.catalog[req.TypeID]
	if !ok {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "disclosure type not found", nil)
	}
	if scope := r.catalogScope[req.TypeID]; scope != "global" && scope != req.Subject.CompanyID {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "disclosure type not found", nil)
	}
	vs := r.versions[req.TypeID]
	target := -1
	for i := range vs {
		if vs[i].VersionNo == req.VersionNo {
			target = i
		}
		vs[i].IsActive = false
	}
	if target < 0 {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "disclosure type version not found", nil)
	}
	now := time.Now().UTC()
	vs[target].IsActive = true
	vs[target].UpdatedBy = req.Subject.UserID
	vs[target].ActivatedAt = now
	r.versions[req.TypeID] = vs
	current.VersionNo = req.VersionNo
	if byVersion, ok := r.catalogByVer[req.TypeID]; ok {
		if snapshot, found := byVersion[req.VersionNo]; found {
			r.catalog[req.TypeID] = snapshot
		} else {
			r.catalog[req.TypeID] = current
		}
	} else {
		r.catalog[req.TypeID] = current
	}
	return &disclosureapp.ActivateTypeVersionResponse{
		TypeID:      req.TypeID,
		VersionNo:   req.VersionNo,
		IsActive:    true,
		UpdatedBy:   req.Subject.UserID,
		ActivatedAt: now,
	}, nil
}

func cloneTemplateBlocks(blocks []disclosureapp.TemplateBlockDTO) []disclosureapp.TemplateBlockDTO {
	if len(blocks) == 0 {
		return []disclosureapp.TemplateBlockDTO{}
	}
	out := make([]disclosureapp.TemplateBlockDTO, 0, len(blocks))
	for _, block := range blocks {
		next := block
		next.Config = cloneAnyMap(block.Config)
		next.Validation = cloneAnyMap(block.Validation)
		out = append(out, next)
	}
	return out
}

func cloneAnyMap(src map[string]any) map[string]any {
	if len(src) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(src))
	for key, value := range src {
		out[key] = value
	}
	return out
}

func cloneWorkflowSteps(items []disclosureapp.WorkflowStepDTO) []disclosureapp.WorkflowStepDTO {
	if len(items) == 0 {
		return []disclosureapp.WorkflowStepDTO{}
	}
	out := make([]disclosureapp.WorkflowStepDTO, 0, len(items))
	for _, item := range items {
		next := item
		next.Documents = slices.Clone(item.Documents)
		out = append(out, next)
	}
	return out
}

func (r *Repository) GetCompanyWorkflowOverride(_ context.Context, companyID, typeID string) (*disclosureapp.CompanyWorkflowOverrideViewDTO, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	view := &disclosureapp.CompanyWorkflowOverrideViewDTO{
		TypeID:          typeID,
		CompanyID:       companyID,
		EffectiveSource: "global_template",
	}
	st, ok := r.overrideByCompanyType[overrideKey(companyID, typeID)]
	if !ok {
		return view, nil
	}
	h := st.header
	view.Override = &h
	if h.ActiveVersionNo > 0 {
		if v, exists := st.versions[h.ActiveVersionNo]; exists {
			cp := v
			cp.Workflow = cloneWorkflowSteps(v.Workflow)
			view.ActiveVersion = &cp
			view.EffectiveSource = "company_override"
		}
	}
	for _, v := range st.versions {
		if v.State == "draft" {
			cp := v
			cp.Workflow = cloneWorkflowSteps(v.Workflow)
			view.DraftVersion = &cp
			break
		}
	}
	return view, nil
}

func (r *Repository) UpsertCompanyWorkflowOverrideDraft(_ context.Context, req disclosureapp.UpsertCompanyWorkflowOverrideDraftRequest) (*disclosureapp.UpsertCompanyWorkflowOverrideDraftResponse, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := overrideKey(req.Subject.CompanyID, req.TypeID)
	st, ok := r.overrideByCompanyType[k]
	now := time.Now().UTC()
	if !ok {
		st = &overrideState{
			header: disclosureapp.CompanyWorkflowOverrideHeaderDTO{
				OverrideID:      "ovr_" + req.Subject.CompanyID + "_" + req.TypeID,
				TypeID:          req.TypeID,
				CompanyID:       req.Subject.CompanyID,
				Status:          "draft",
				ActiveVersionNo: 0,
				UpdatedAt:       now,
			},
			versions: map[int]disclosureapp.CompanyWorkflowOverrideVersionDTO{},
		}
		r.overrideByCompanyType[k] = st
	}
	nextVersion := 1
	for v := range st.versions {
		if v >= nextVersion {
			nextVersion = v + 1
		}
	}
	st.versions[nextVersion] = disclosureapp.CompanyWorkflowOverrideVersionDTO{
		VersionNo:  nextVersion,
		State:      "draft",
		ChangeNote: strings.TrimSpace(req.ChangeNote),
		Workflow:   cloneWorkflowSteps(req.Workflow),
		CreatedBy:  req.Subject.UserID,
		CreatedAt:  now,
	}
	st.header.Status = "draft"
	st.header.UpdatedAt = now
	return &disclosureapp.UpsertCompanyWorkflowOverrideDraftResponse{
		OverrideID:     st.header.OverrideID,
		TypeID:         req.TypeID,
		CompanyID:      req.Subject.CompanyID,
		DraftVersionNo: nextVersion,
		State:          "draft",
		UpdatedAt:      now,
	}, nil
}

func (r *Repository) ApproveCompanyWorkflowOverride(_ context.Context, req disclosureapp.ApproveCompanyWorkflowOverrideRequest) (*disclosureapp.ApproveCompanyWorkflowOverrideResponse, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	st, ok := r.overrideByCompanyType[overrideKey(req.Subject.CompanyID, req.TypeID)]
	if !ok {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "workflow override not found", nil)
	}
	v, ok := st.versions[req.VersionNo]
	if !ok {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "workflow override version not found", nil)
	}
	if v.State != "draft" {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "workflow override version is not draft", nil)
	}
	now := time.Now().UTC()
	v.State = "approved"
	v.ApprovedBy = req.Subject.UserID
	v.ApprovedAt = &now
	st.versions[req.VersionNo] = v
	st.header.ActiveVersionNo = req.VersionNo
	st.header.Status = "approved"
	st.header.UpdatedAt = now
	return &disclosureapp.ApproveCompanyWorkflowOverrideResponse{
		OverrideID:      st.header.OverrideID,
		TypeID:          req.TypeID,
		CompanyID:       req.Subject.CompanyID,
		ActiveVersionNo: req.VersionNo,
		State:           "approved",
		ApprovedBy:      req.Subject.UserID,
		ApprovedAt:      now,
		EffectiveSource: "company_override",
	}, nil
}

func (r *Repository) DeleteCompanyWorkflowOverrideDraft(_ context.Context, req disclosureapp.DeleteCompanyWorkflowOverrideDraftRequest) (*disclosureapp.DeleteCompanyWorkflowOverrideDraftResponse, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	st, ok := r.overrideByCompanyType[overrideKey(req.Subject.CompanyID, req.TypeID)]
	if !ok {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "workflow override not found", nil)
	}
	v, ok := st.versions[req.VersionNo]
	if !ok {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "workflow override version not found", nil)
	}
	if v.State != "draft" {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "only draft version can be deleted", nil)
	}
	delete(st.versions, req.VersionNo)
	return &disclosureapp.DeleteCompanyWorkflowOverrideDraftResponse{Deleted: true, VersionNo: req.VersionNo}, nil
}

func (r *Repository) ResetCompanyWorkflowOverrideActive(_ context.Context, req disclosureapp.ResetCompanyWorkflowOverrideActiveRequest) (*disclosureapp.ResetCompanyWorkflowOverrideActiveResponse, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	st, ok := r.overrideByCompanyType[overrideKey(req.Subject.CompanyID, req.TypeID)]
	if !ok {
		return &disclosureapp.ResetCompanyWorkflowOverrideActiveResponse{
			TypeID:          req.TypeID,
			CompanyID:       req.Subject.CompanyID,
			ActiveVersionNo: 0,
			State:           "archived",
			EffectiveSource: "global_template",
		}, nil
	}
	st.header.ActiveVersionNo = 0
	st.header.Status = "archived"
	st.header.UpdatedAt = time.Now().UTC()
	return &disclosureapp.ResetCompanyWorkflowOverrideActiveResponse{
		OverrideID:      st.header.OverrideID,
		TypeID:          req.TypeID,
		CompanyID:       req.Subject.CompanyID,
		ActiveVersionNo: 0,
		State:           "archived",
		EffectiveSource: "global_template",
	}, nil
}

func (r *Repository) ListCompanyWorkflowOverrideVersions(_ context.Context, companyID, typeID string, page, pageSize int) ([]disclosureapp.CompanyWorkflowOverrideVersionDTO, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	st, ok := r.overrideByCompanyType[overrideKey(companyID, typeID)]
	if !ok {
		return []disclosureapp.CompanyWorkflowOverrideVersionDTO{}, 0, nil
	}
	out := make([]disclosureapp.CompanyWorkflowOverrideVersionDTO, 0, len(st.versions))
	for _, item := range st.versions {
		cp := item
		cp.Workflow = cloneWorkflowSteps(item.Workflow)
		out = append(out, cp)
	}
	slices.SortFunc(out, func(a, b disclosureapp.CompanyWorkflowOverrideVersionDTO) int { return b.VersionNo - a.VersionNo })
	total := len(out)
	start := (page - 1) * pageSize
	if start >= total {
		return []disclosureapp.CompanyWorkflowOverrideVersionDTO{}, total, nil
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return out[start:end], total, nil
}

func (r *Repository) GetEffectiveWorkflow(_ context.Context, companyID, typeID string) (*disclosureapp.EffectiveWorkflowDTO, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	dto := &disclosureapp.EffectiveWorkflowDTO{
		TypeID:    typeID,
		CompanyID: companyID,
		Source:    "global_template",
		VersionNo: 0,
		Workflow:  []disclosureapp.WorkflowStepDTO{},
	}
	st, ok := r.overrideByCompanyType[overrideKey(companyID, typeID)]
	if !ok || st.header.ActiveVersionNo <= 0 {
		return dto, nil
	}
	v, ok := st.versions[st.header.ActiveVersionNo]
	if !ok || v.State != "approved" {
		return dto, nil
	}
	dto.Source = "company_override"
	dto.VersionNo = v.VersionNo
	dto.Workflow = cloneWorkflowSteps(v.Workflow)
	return dto, nil
}
