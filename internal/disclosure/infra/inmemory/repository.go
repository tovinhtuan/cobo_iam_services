package inmemory

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	"github.com/cobo/cobo_iam_services/internal/disclosure/app/applicability"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

type Repository struct {
	mu                        sync.RWMutex
	items                     map[string]disclosureapp.RecordDTO
	groups                    []disclosureapp.DisclosureGroupDTO
	displayGroups             []disclosureapp.DisplayGroupDTO
	templateDepartments       []disclosureapp.TemplateDepartmentDTO
	displayGroupCodes         map[string]string
	templateDisplayGroupCodes map[string][]string
	catalog                   map[string]disclosureapp.DisclosureTypeDTO
	catalogByVer              map[string]map[int]disclosureapp.DisclosureTypeDTO
	versions                  map[string][]disclosureapp.DisclosureTypeVersionDTO
	catalogScope              map[string]string
	overrideByCompanyType     map[string]*overrideState
	globalWorkflows           map[string]*disclosureapp.GlobalWorkflowDTO
	// globalWorkflowVersions backs Sprint 3 / Batch 3's GetGlobalWorkflowVersionManifest — keyed
	// by typeID then versionNo. Test-only seeding via SetGlobalWorkflowVersionManifestForTest;
	// no production code path writes this map (mirrors globalWorkflows' own test-fixture nature).
	globalWorkflowVersions map[string]map[int][]disclosureapp.GlobalWorkflowStepInput
	// workflowOverrideConflicts backs Sprint 3 / Batch 4 — keyed by conflict id (== conflict_key).
	workflowOverrideConflicts map[string]disclosureapp.PersistedConflictDTO
}

type overrideState struct {
	header   disclosureapp.CompanyWorkflowOverrideHeaderDTO
	versions map[int]disclosureapp.CompanyWorkflowOverrideVersionDTO
}

func NewRepository() *Repository {
	repo := &Repository{
		items:                     map[string]disclosureapp.RecordDTO{},
		groups:                    disclosureapp.SeedDisclosureTypeGroups(),
		displayGroups:             disclosureapp.SeedDisplayGroups(),
		templateDepartments:       disclosureapp.SeedTemplateDepartments(),
		displayGroupCodes:         map[string]string{},
		templateDisplayGroupCodes: map[string][]string{},
		catalog:                   map[string]disclosureapp.DisclosureTypeDTO{},
		catalogByVer:              map[string]map[int]disclosureapp.DisclosureTypeDTO{},
		versions:                  map[string][]disclosureapp.DisclosureTypeVersionDTO{},
		catalogScope:              map[string]string{},
		overrideByCompanyType:     map[string]*overrideState{},
		globalWorkflowVersions:    map[string]map[int][]disclosureapp.GlobalWorkflowStepInput{},
		workflowOverrideConflicts: map[string]disclosureapp.PersistedConflictDTO{},
	}
	for _, item := range disclosureapp.SeedDisclosureTypeCatalog() {
		item.VersionNo = 1
		if (item.TypeID == "dt-periodic-financial" || item.TypeID == "dt-custom-obligation") && len(item.Blocks) == 0 {
			item.Blocks = []disclosureapp.TemplateBlockDTO{
				{
					BlockID:      "seed-workflow-block",
					BlockKey:     "enterprise_workflow",
					BlockType:    "rich_text",
					Title:        "Workflow",
					Description:  "Seed workflow for integration tests",
					DisplayOrder: 6,
					Enabled:      true,
					Config: map[string]any{
						"steps": []any{
							map[string]any{
								"step_id":           "seed-review",
								"stage":             "Review",
								"department_id":     "dept-finance",
								"assignee_role_ids": []any{"role-reviewer"},
								"processing_days":   2,
								"display_order":     1,
								"documents":         []any{},
							},
						},
					},
					Validation: map[string]any{},
				},
			}
		}
		repo.catalog[item.TypeID] = item
		repo.catalogByVer[item.TypeID] = map[int]disclosureapp.DisclosureTypeDTO{1: item}
		repo.catalogScope[item.TypeID] = "global"
		repo.displayGroupCodes[item.TypeID] = item.DisplayGroupCode
		if len(item.DisplayGroupCodes) > 0 {
			repo.templateDisplayGroupCodes[item.TypeID] = slices.Clone(item.DisplayGroupCodes)
		} else if code := strings.TrimSpace(item.DisplayGroupCode); code != "" {
			repo.templateDisplayGroupCodes[item.TypeID] = []string{code}
		}
		repo.versions[item.TypeID] = []disclosureapp.DisclosureTypeVersionDTO{
			{
				TypeID:      item.TypeID,
				VersionNo:   1,
				IsActive:    true,
				IsReleased:  true,
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

func (r *Repository) ListDisplayGroups(_ context.Context) ([]disclosureapp.DisplayGroupDTO, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]disclosureapp.DisplayGroupDTO, len(r.displayGroups))
	copy(out, r.displayGroups)
	return out, nil
}

func (r *Repository) ListTypes(_ context.Context, params disclosureapp.ListTypesParams) ([]disclosureapp.DisclosureTypeSummaryDTO, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	companyID := params.CompanyID
	groupID := strings.TrimSpace(params.GroupID)
	displayGroupCode := strings.TrimSpace(params.DisplayGroupCode)
	query := strings.ToLower(strings.TrimSpace(params.Query))
	page := params.Page
	pageSize := params.PageSize
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	typeIDFilter := map[string]struct{}{}
	for _, id := range params.TypeIDs {
		typeIDFilter[id] = struct{}{}
	}

	out := make([]disclosureapp.DisclosureTypeSummaryDTO, 0)
	for _, item := range r.catalog {
		if len(typeIDFilter) > 0 {
			if _, ok := typeIDFilter[item.TypeID]; !ok {
				continue
			}
		}
		itemScope := r.catalogScope[item.TypeID]
		scopeFilter := strings.ToLower(strings.TrimSpace(params.Scope))
		switch scopeFilter {
		case "global":
			if itemScope != "global" {
				continue
			}
		case "company":
			if itemScope != companyID {
				continue
			}
		default:
			if itemScope != "global" && itemScope != companyID {
				continue
			}
		}
		if groupID != "" && item.GroupID != groupID {
			continue
		}
		if displayGroupCode != "" && r.displayGroupCodes[item.TypeID] != displayGroupCode {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(item.Name+" "+item.Description), query) {
			continue
		}
		if len(params.Tags) > 0 {
			matched := false
			for _, want := range params.Tags {
				want = strings.TrimSpace(want)
				for _, have := range item.Tags {
					if strings.EqualFold(strings.TrimSpace(have), want) {
						matched = true
						break
					}
				}
				if matched {
					break
				}
			}
			if !matched {
				continue
			}
		}
		if freq := disclosureapp.NormalizePeriodicityFilter(params.Periodicity); freq != "" {
			p := strings.ToLower(strings.TrimSpace(item.Periodicity))
			tc := strings.ToLower(strings.TrimSpace(item.TemplateCategory))
			ok := false
			switch freq {
			case "ad_hoc":
				ok = p == "ad_hoc" || p == "event_based" || tc == "irregular" || tc == "custom"
			case "yearly":
				ok = p == "yearly" || p == "annual"
			default:
				ok = p == freq
			}
			if !ok {
				continue
			}
		}
		if dept := strings.TrimSpace(params.DepartmentID); dept != "" {
			hasDept := false
			if wf := r.globalWorkflows[item.TypeID]; wf != nil {
				for _, step := range wf.Steps {
					if strings.TrimSpace(step.DepartmentID) == dept {
						hasDept = true
						break
					}
				}
			}
			if !hasDept {
				continue
			}
		}
		scope := "company"
		ownerCompanyID := r.catalogScope[item.TypeID]
		if ownerCompanyID == "global" {
			scope = "global"
			ownerCompanyID = ""
		}
		if params.LightweightOnly {
			out = append(out, disclosureapp.DisclosureTypeSummaryDTO{
				TypeID:             item.TypeID,
				Scope:              scope,
				OwnerCompanyID:     ownerCompanyID,
				Name:               item.Name,
				ApplicabilityRules: item.ApplicabilityRules,
			})
			continue
		}
		out = append(out, disclosureapp.DisclosureTypeSummaryDTO{
			TypeID:           item.TypeID,
			GroupID:          item.GroupID,
			DisplayGroupCode: r.displayGroupCodes[item.TypeID],
			DisplayGroupCodes: func() []string {
				if codes := r.templateDisplayGroupCodes[item.TypeID]; len(codes) > 0 {
					return slices.Clone(codes)
				}
				if c := r.displayGroupCodes[item.TypeID]; c != "" {
					return []string{c}
				}
				return []string{}
			}(),
			Scope:              scope,
			OwnerCompanyID:     ownerCompanyID,
			Name:               item.Name,
			Category:           item.Category,
			TemplateCategory:   item.TemplateCategory,
			Description:        item.Description,
			DeadlineRule:       item.DeadlineRule,
			HasWorkflow:        disclosureapp.TemplateHasWorkflow(item.Blocks),
			Tags:               slices.Clone(item.Tags),
			ApplicabilityRules: item.ApplicabilityRules,
		})
	}
	total := len(out)
	if page > 0 {
		start := (page - 1) * pageSize
		if start >= total {
			return []disclosureapp.DisclosureTypeSummaryDTO{}, total, nil
		}
		end := start + pageSize
		if end > total {
			end = total
		}
		out = out[start:end]
	}
	return out, total, nil
}

func (r *Repository) ListTypeFilterOptions(_ context.Context, companyID string) (*disclosureapp.ListTypeFilterOptionsResponse, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_ = companyID
	out := &disclosureapp.ListTypeFilterOptionsResponse{
		Tags:        []disclosureapp.TypeFilterOptionDTO{},
		Departments: []disclosureapp.TypeFilterOptionDTO{},
		Frequencies: disclosureapp.DefaultFrequencyFilterOptions(),
	}
	tagSeen := map[string]struct{}{}
	for _, item := range r.catalog {
		for _, tag := range item.Tags {
			tag = strings.TrimSpace(tag)
			if tag == "" {
				continue
			}
			key := strings.ToLower(tag)
			if _, ok := tagSeen[key]; ok {
				continue
			}
			tagSeen[key] = struct{}{}
			out.Tags = append(out.Tags, disclosureapp.TypeFilterOptionDTO{ID: tag, Name: tag})
		}
	}
	deptSeen := map[string]struct{}{}
	for _, wf := range r.globalWorkflows {
		if wf == nil {
			continue
		}
		for _, step := range wf.Steps {
			id := strings.TrimSpace(step.DepartmentID)
			if id == "" {
				continue
			}
			if _, ok := deptSeen[id]; ok {
				continue
			}
			deptSeen[id] = struct{}{}
			out.Departments = append(out.Departments, disclosureapp.TypeFilterOptionDTO{ID: id, Name: id})
		}
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
	cp.LegalBases = slices.Clone(item.LegalBases)
	cp.Checklist = slices.Clone(item.Checklist)
	cp.Blocks = cloneTemplateBlocks(item.Blocks)
	cp.HasWorkflow = disclosureapp.TemplateHasWorkflow(cp.Blocks)
	if codes := r.templateDisplayGroupCodes[typeID]; len(codes) > 0 {
		cp.DisplayGroupCodes = slices.Clone(codes)
	} else if code := strings.TrimSpace(r.displayGroupCodes[typeID]); code != "" {
		cp.DisplayGroupCodes = []string{code}
	}
	disclosureapp.EnrichTemplateBlockDisplayNames(cp.Blocks)
	return &cp, nil
}

func (r *Repository) HasActiveEnterpriseWorkflow(_ context.Context, _ string, typeID string) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	item, ok := r.catalog[typeID]
	if !ok {
		return false, nil
	}
	return disclosureapp.TemplateHasWorkflow(item.Blocks), nil
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
	cp.LegalBases = slices.Clone(item.LegalBases)
	cp.Checklist = slices.Clone(item.Checklist)
	cp.Blocks = cloneTemplateBlocks(item.Blocks)
	cp.HasWorkflow = disclosureapp.TemplateHasWorkflow(cp.Blocks)
	if codes := r.templateDisplayGroupCodes[typeID]; len(codes) > 0 {
		cp.DisplayGroupCodes = slices.Clone(codes)
	} else if code := strings.TrimSpace(r.displayGroupCodes[typeID]); code != "" {
		cp.DisplayGroupCodes = []string{code}
	}
	disclosureapp.EnrichTemplateBlockDisplayNames(cp.Blocks)
	return &cp, nil
}

func (r *Repository) UpsertTypeVersion(ctx context.Context, req disclosureapp.UpsertTypeVersionRequest) (*disclosureapp.UpsertTypeVersionResponse, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	current, existed := r.catalog[req.TypeID]
	activeVersionNo := 0
	for _, ver := range r.versions[req.TypeID] {
		if ver.IsActive {
			activeVersionNo = ver.VersionNo
			break
		}
	}
	nextIsActive := !existed || activeVersionNo <= 0

	openDraftNo := 0
	if existed && activeVersionNo > 0 {
		for _, ver := range r.versions[req.TypeID] {
			if ver.VersionNo != activeVersionNo && !ver.IsReleased && ver.VersionNo > openDraftNo {
				openDraftNo = ver.VersionNo
			}
		}
	}

	overwriteDraft := openDraftNo > 0 && !nextIsActive
	versionNo := openDraftNo
	if !overwriteDraft {
		versionNo = 1
		for _, ver := range r.versions[req.TypeID] {
			if ver.VersionNo >= versionNo {
				versionNo = ver.VersionNo + 1
			}
		}
	}
	if req.PreserveLegalBases && overwriteDraft {
		if byVer, ok := r.catalogByVer[req.TypeID]; ok {
			if existing, ok := byVer[versionNo]; ok {
				req.LegalBases = slices.Clone(existing.LegalBases)
			}
		}
	}
	// Phase 12.5: independent version deep-copy + ID regeneration.
	if !overwriteDraft && versionNo > 1 {
		idg := idgen.UUIDv7Generator{}
		prevNo := versionNo - 1
		// Prefer exact prior max version from catalog map if version numbers are dense;
		// fall back to highest existing version number.
		var prev *disclosureapp.DisclosureTypeDTO
		if byVer, ok := r.catalogByVer[req.TypeID]; ok {
			if item, ok := byVer[prevNo]; ok {
				cp := item
				prev = &cp
			} else {
				best := 0
				for n, item := range byVer {
					if n > best {
						best = n
						cp := item
						prev = &cp
					}
				}
			}
		}
		if prev != nil {
			if req.PreserveLegalBases {
				bases, flat, _ := disclosureapp.PrepareLegalBasesForNewVersion(
					ctx, req.TypeID, prev.LegalBases, prev.LegalBasis, nil, req.LegalBasis, true, idg,
				)
				req.LegalBases = bases
				req.LegalBasis = flat
			} else if len(req.LegalBases) > 0 {
				bases, flat, _ := disclosureapp.PrepareLegalBasesForNewVersion(
					ctx, req.TypeID, nil, "", req.LegalBases, req.LegalBasis, false, idg,
				)
				req.LegalBases = bases
				req.LegalBasis = flat
			}
		}
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
		DeadlineConfig:        req.DeadlineConfig,
		LegalBases:            slices.Clone(req.LegalBases),
		Checklist:             slices.Clone(req.Checklist),
		Tags:                  slices.Clone(req.Tags),
		Blocks:                cloneTemplateBlocks(req.Blocks),
	}
	r.templateDisplayGroupCodes[req.TypeID] = slices.Clone(req.DisplayGroupCodes)
	if len(req.DisplayGroupCodes) > 0 {
		next.DisplayGroupCodes = slices.Clone(req.DisplayGroupCodes)
		next.DisplayGroupCode = req.DisplayGroupCodes[0]
		r.displayGroupCodes[req.TypeID] = req.DisplayGroupCodes[0]
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
	now := time.Now().UTC()
	if overwriteDraft {
		for i := range vs {
			if vs[i].VersionNo == versionNo {
				vs[i].ChangeNote = strings.TrimSpace(req.ChangeNote)
				vs[i].UpdatedBy = req.Subject.UserID
				vs[i].ActivatedAt = now
				vs[i].IsReleased = false
				vs[i].IsActive = false
			}
		}
		r.versions[req.TypeID] = vs
		if existed {
			r.catalog[req.TypeID] = current
		}
	} else {
		if nextIsActive {
			for i := range vs {
				vs[i].IsActive = false
			}
		}
		vs = append(vs, disclosureapp.DisclosureTypeVersionDTO{
			TypeID:      req.TypeID,
			VersionNo:   versionNo,
			IsActive:    nextIsActive,
			IsReleased:  nextIsActive,
			ChangeNote:  strings.TrimSpace(req.ChangeNote),
			UpdatedBy:   req.Subject.UserID,
			ActivatedAt: now,
		})
		r.versions[req.TypeID] = vs
		if !nextIsActive && existed {
			r.catalog[req.TypeID] = current
		}
	}

	return &disclosureapp.UpsertTypeVersionResponse{
		TypeID:      req.TypeID,
		VersionNo:   versionNo,
		IsActive:    nextIsActive,
		UpdatedBy:   req.Subject.UserID,
		ActivatedAt: now,
	}, nil
}

func (r *Repository) GetCompanyApplicabilityProfile(_ context.Context, _ string) (applicability.CompanyApplicabilityProfile, error) {
	return applicability.CompanyApplicabilityProfile{}, nil
}

func (r *Repository) GetCompanyDeadlineContext(_ context.Context, companyID string) (disclosureapp.CompanyDeadlineContext, error) {
	return disclosureapp.CompanyDeadlineContext{
		CompanyID:        companyID,
		CurrentYear:      time.Now().In(time.FixedZone("ICT", 7*60*60)).Year(),
		EstablishedDate:  nil,
		EstablishedMonth: 1,
		EstablishedDay:   1,
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
	activeNo := 0
	openDraftNo := 0
	for _, ver := range vs {
		if ver.IsActive {
			activeNo = ver.VersionNo
		}
	}
	for _, ver := range vs {
		if ver.VersionNo != activeNo && !ver.IsReleased && ver.VersionNo > openDraftNo {
			openDraftNo = ver.VersionNo
		}
	}
	filtered := make([]disclosureapp.DisclosureTypeVersionDTO, 0, len(vs))
	for _, ver := range vs {
		if ver.IsActive || ver.IsReleased || ver.VersionNo == openDraftNo {
			filtered = append(filtered, ver)
		}
	}
	slices.SortFunc(filtered, func(a, b disclosureapp.DisclosureTypeVersionDTO) int {
		return b.VersionNo - a.VersionNo
	})
	return filtered, nil
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
	vs[target].IsReleased = true
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

// ApplyWorkflowOverrideRebase mirrors the mysql repository's exact transactional shape and race
// guard (Sprint 3 / Batch 5) — under r.mu, so it is atomic relative to every other in-memory
// repository method by construction.
func (r *Repository) ApplyWorkflowOverrideRebase(_ context.Context, params disclosureapp.ApplyWorkflowOverrideRebaseParams) (*disclosureapp.ApplyWorkflowOverrideRebaseResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	st, ok := r.overrideByCompanyType[overrideKey(params.CompanyID, params.TypeID)]
	if !ok {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeOverrideNotFound, "workflow override not found", nil)
	}
	if st.header.ActiveVersionNo != params.ExpectedActiveVersionNo {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "override was modified concurrently; re-run rebase-preview and retry", nil)
	}

	nextVersion := 0
	for vn := range st.versions {
		if vn > nextVersion {
			nextVersion = vn
		}
	}
	nextVersion++

	approvedAt := params.Now
	st.versions[nextVersion] = disclosureapp.CompanyWorkflowOverrideVersionDTO{
		VersionNo:  nextVersion,
		State:      "approved",
		ChangeNote: "rebase-apply",
		Workflow:   params.NewSnapshot,
		CreatedBy:  params.UserID,
		ApprovedBy: params.UserID,
		ApprovedAt: &approvedAt,
		CreatedAt:  params.Now,
	}
	st.header.ActiveVersionNo = nextVersion
	st.header.Status = "approved"
	st.header.BaseSource = disclosureapp.BaseSourceGlobalWorkflow
	baseVersionNo := params.NewBaseVersionNo
	st.header.BaseVersionNo = &baseVersionNo
	st.header.BaseHash = ""
	st.header.StaleStatus = disclosureapp.StaleStatusCurrent
	lastCheck := params.Now
	st.header.LastRebaseCheckAt = &lastCheck
	st.header.UpdatedAt = params.Now

	return &disclosureapp.ApplyWorkflowOverrideRebaseResult{
		OverrideID:   st.header.OverrideID,
		NewVersionNo: nextVersion,
		AppliedAt:    params.Now,
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

func (r *Repository) GetActiveVersionDeadlineConfig(_ context.Context, _ string) (int, *disclosureapp.TemplateDeadlineConfig, error) {
	return 0, nil, nil
}

func (r *Repository) UpdateActiveVersionDeadlineConfig(_ context.Context, _ string, _ disclosureapp.TemplateDeadlineConfig, _ string) error {
	return perr.NewHTTPError(http.StatusNotImplemented, perr.CodeInternal, "not implemented in-memory", nil)
}

func (r *Repository) ListCompanyGroups(_ context.Context, _ string, _ string, _ *bool) ([]disclosureapp.CompanyGroupDTO, error) {
	return []disclosureapp.CompanyGroupDTO{}, nil
}

func (r *Repository) UpdateWorkflowOverrideStepGroups(_ context.Context, _ disclosureapp.UpdateWorkflowOverrideStepGroupsRequest) (*disclosureapp.UpdateWorkflowOverrideStepGroupsResponse, error) {
	return nil, perr.NewHTTPError(http.StatusNotImplemented, perr.CodeInternal, "not implemented in-memory", nil)
}

// Sprint 3 / Batch 2 — Workflow Override Staleness Detection (in-memory mirror).

// SeedCompanyWorkflowOverrideDraftForTest inserts a draft version with arbitrary workflow (including empty) for activation tests.
func (r *Repository) SeedCompanyWorkflowOverrideDraftForTest(companyID, typeID string, versionNo int, workflow []disclosureapp.WorkflowStepDTO) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := overrideKey(companyID, typeID)
	now := time.Now().UTC()
	st, ok := r.overrideByCompanyType[k]
	if !ok {
		st = &overrideState{
			header: disclosureapp.CompanyWorkflowOverrideHeaderDTO{
				OverrideID:      "ovr_" + companyID + "_" + typeID,
				TypeID:          typeID,
				CompanyID:       companyID,
				Status:          "draft",
				ActiveVersionNo: 0,
				UpdatedAt:       now,
			},
			versions: map[int]disclosureapp.CompanyWorkflowOverrideVersionDTO{},
		}
		r.overrideByCompanyType[k] = st
	}
	st.versions[versionNo] = disclosureapp.CompanyWorkflowOverrideVersionDTO{
		VersionNo: versionNo,
		State:     "draft",
		Workflow:  cloneWorkflowSteps(workflow),
		CreatedBy: "test",
		CreatedAt: now,
	}
	st.header.Status = "draft"
	st.header.UpdatedAt = now
	return nil
}

// SetOverrideBaseMetadataForTest is a white-box test helper (same convention as
// cms_repository_pointer_test.go's direct header field manipulation) letting tests set up a
// known base_source/base_version_no for an already-created override, since no production code
// path populates these for new overrides yet (Batch 1's known, disclosed gap — see
// docs/ai-cache/workflow-override-foundation-batch1/BACKFILL_REPORT.md "known gap" section).
func (r *Repository) SetOverrideBaseMetadataForTest(companyID, typeID, baseSource string, baseVersionNo *int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	st, ok := r.overrideByCompanyType[overrideKey(companyID, typeID)]
	if !ok {
		return perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "override not found for test setup", nil)
	}
	st.header.BaseSource = baseSource
	st.header.BaseVersionNo = baseVersionNo
	return nil
}

func (r *Repository) TypeExists(_ context.Context, typeID string) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.catalog[typeID]
	return ok, nil
}

func (r *Repository) GetOverrideStalenessMetadata(_ context.Context, companyID, typeID string) (*disclosureapp.OverrideStalenessRow, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	st, ok := r.overrideByCompanyType[overrideKey(companyID, typeID)]
	if !ok {
		return nil, false, nil
	}
	h := st.header
	return &disclosureapp.OverrideStalenessRow{
		HasOverride:       h.ActiveVersionNo > 0,
		BaseSource:        h.BaseSource,
		BaseWorkflowID:    nilIfEmptyPtr(h.BaseWorkflowID),
		BaseVersionNo:     h.BaseVersionNo,
		BaseHash:          nilIfEmptyPtr(h.BaseHash),
		StaleStatus:       h.StaleStatus,
		LastRebaseCheckAt: h.LastRebaseCheckAt,
	}, true, nil
}

func (r *Repository) GetCurrentGlobalActiveVersionNo(_ context.Context, typeID string) (*int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	wf, ok := r.globalWorkflows[typeID]
	if !ok || wf.Status != "active" {
		return nil, nil
	}
	return wf.ActiveVersionNo, nil
}

// SetGlobalActiveVersionForTest is a white-box test helper (mirrors SetOverrideBaseMetadataForTest's
// convention) for setting up GetCurrentGlobalActiveVersionNo's backing state directly, without
// requiring a full CMS publish/activate flow in every Batch 2/3 test.
func (r *Repository) SetGlobalActiveVersionForTest(typeID string, activeVersionNo int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.globalWorkflows == nil {
		r.globalWorkflows = map[string]*disclosureapp.GlobalWorkflowDTO{}
	}
	wf, ok := r.globalWorkflows[typeID]
	if !ok {
		v := activeVersionNo
		r.globalWorkflows[typeID] = &disclosureapp.GlobalWorkflowDTO{TypeID: typeID, Status: "active", ActiveVersionNo: &v}
		return nil
	}
	v := activeVersionNo
	wf.Status = "active"
	wf.ActiveVersionNo = &v
	return nil
}

func (r *Repository) UpdateOverrideStaleness(_ context.Context, companyID, typeID, staleStatus string, checkedAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	st, ok := r.overrideByCompanyType[overrideKey(companyID, typeID)]
	if !ok {
		return nil // Rule 1 — no override row, no-op, not an error.
	}
	st.header.StaleStatus = staleStatus
	at := checkedAt
	st.header.LastRebaseCheckAt = &at
	return nil
}

// GetGlobalWorkflowVersionManifest is Sprint 3 / Batch 3's read-only in-memory mirror. No
// production code path writes globalWorkflowVersions — see SetGlobalWorkflowVersionManifestForTest.
func (r *Repository) GetGlobalWorkflowVersionManifest(_ context.Context, typeID string, versionNo int) ([]disclosureapp.GlobalWorkflowStepInput, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	byVersion, ok := r.globalWorkflowVersions[typeID]
	if !ok {
		return nil, false, nil
	}
	steps, ok := byVersion[versionNo]
	if !ok {
		return nil, false, nil
	}
	return steps, true, nil
}

// SetGlobalWorkflowVersionManifestForTest is a white-box test helper (same convention as Batch
// 2's SetOverrideBaseMetadataForTest) — there is no production write path for this map, since the
// real MySQL repository reads global_workflow_versions directly and this in-memory repo has no
// equivalent versions table to derive it from.
func (r *Repository) SetGlobalWorkflowVersionManifestForTest(typeID string, versionNo int, steps []disclosureapp.GlobalWorkflowStepInput) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.globalWorkflowVersions[typeID] == nil {
		r.globalWorkflowVersions[typeID] = map[int][]disclosureapp.GlobalWorkflowStepInput{}
	}
	r.globalWorkflowVersions[typeID][versionNo] = steps
}

// UpsertWorkflowOverrideConflicts is the in-memory mirror of the mysql repository's upsert —
// same Option B idempotency (id == conflict_key), same "never reset resolution_status on
// re-detection" rule.
func (r *Repository) UpsertWorkflowOverrideConflicts(_ context.Context, inputs []disclosureapp.PersistedConflictInput) ([]disclosureapp.PersistedConflictDTO, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]disclosureapp.PersistedConflictDTO, 0, len(inputs))
	for _, in := range inputs {
		conflictKey := disclosureapp.BuildConflictKey(in.CompanyID, in.TypeID, in.BaseVersionNo, in.TargetVersionNo, in.StepKey, in.FieldPath, in.ConflictType)
		existing, exists := r.workflowOverrideConflicts[conflictKey]
		dto := disclosureapp.PersistedConflictDTO{
			ID:                conflictKey,
			CompanyID:         in.CompanyID,
			TypeID:            in.TypeID,
			OverrideID:        in.OverrideID,
			OverrideVersionNo: in.OverrideVersionNo,
			BaseVersionNo:     in.BaseVersionNo,
			TargetVersionNo:   in.TargetVersionNo,
			StepKey:           in.StepKey,
			FieldPath:         in.FieldPath,
			Severity:          in.Severity,
			ConflictType:      in.ConflictType,
			GlobalOld:         in.GlobalOld,
			GlobalNew:         in.GlobalNew,
			CompanyValue:      in.CompanyValue,
			ResolutionOptions: in.ResolutionOptions,
			ResolutionStatus:  disclosureapp.ResolutionStatusUnresolved,
			CreatedAt:         time.Now().UTC(),
		}
		if exists {
			// Preserve resolution state across re-detection; refresh only the content fields.
			dto.ResolutionStatus = existing.ResolutionStatus
			dto.Resolution = existing.Resolution
			dto.ResolvedBy = existing.ResolvedBy
			dto.ResolvedAt = existing.ResolvedAt
			dto.CreatedAt = existing.CreatedAt
		}
		r.workflowOverrideConflicts[conflictKey] = dto
		out = append(out, dto)
	}
	return out, nil
}

// GetWorkflowOverrideConflict mirrors the mysql repository's tenant-scoped lookup — returns nil
// (no error) for both "doesn't exist" and "exists but isn't yours."
func (r *Repository) GetWorkflowOverrideConflict(_ context.Context, companyID, typeID, conflictID string) (*disclosureapp.PersistedConflictDTO, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	dto, ok := r.workflowOverrideConflicts[conflictID]
	if !ok || dto.CompanyID != companyID || dto.TypeID != typeID {
		return nil, nil
	}
	cp := dto
	return &cp, nil
}

// ResolveWorkflowOverrideConflict mirrors the mysql repository's scoped update — writes ONLY the
// 5 resolution fields (resolution_value included, Sprint 3 / Batch 5 — needed so an in-memory
// merge_manual resolution carries its value through to the apply engine exactly as the mysql
// repository's resolution_json round-trip now does).
func (r *Repository) ResolveWorkflowOverrideConflict(_ context.Context, companyID, typeID, conflictID, resolution string, resolutionValue any, resolvedBy string, resolvedAt time.Time) (*disclosureapp.PersistedConflictDTO, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	dto, ok := r.workflowOverrideConflicts[conflictID]
	if !ok || dto.CompanyID != companyID || dto.TypeID != typeID {
		return nil, nil
	}
	dto.ResolutionStatus = disclosureapp.ResolutionStatusResolved
	res := resolution
	dto.Resolution = &res
	dto.ResolutionValue = resolutionValue
	by := resolvedBy
	dto.ResolvedBy = &by
	at := resolvedAt
	dto.ResolvedAt = &at
	r.workflowOverrideConflicts[conflictID] = dto
	cp := dto
	return &cp, nil
}

func nilIfEmptyPtr(s string) *string {
	if s == "" {
		return nil
	}
	v := s
	return &v
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
	legacyOrGlobalWorkflowFallback := func() {
		if wf, exists := r.globalWorkflows[typeID]; exists && wf.Status == "active" && wf.ActiveVersionNo != nil {
			dto.Source = "global_workflow"
			dto.VersionNo = *wf.ActiveVersionNo
			dto.Workflow = convertGlobalWorkflowSteps(wf.Steps)
			return
		}
		if current, exists := r.catalog[typeID]; exists {
			dto.VersionNo = current.VersionNo
			dto.Workflow = disclosureapp.ExtractTemplateWorkflow(current.Blocks)
		}
	}
	st, ok := r.overrideByCompanyType[overrideKey(companyID, typeID)]
	if !ok || st.header.ActiveVersionNo <= 0 {
		legacyOrGlobalWorkflowFallback()
		return dto, nil
	}
	v, ok := st.versions[st.header.ActiveVersionNo]
	if !ok || v.State != "approved" {
		legacyOrGlobalWorkflowFallback()
		return dto, nil
	}
	dto.Source = "company_override"
	dto.VersionNo = v.VersionNo
	dto.Workflow = cloneWorkflowSteps(v.Workflow)
	if len(dto.Workflow) == 0 {
		dto.OverrideInvalidEmpty = true
		if wf, exists := r.globalWorkflows[typeID]; exists && wf.Status == "active" && wf.ActiveVersionNo != nil && len(wf.Steps) > 0 {
			dto.GlobalWorkflowAvailable = true
		}
	}
	return dto, nil
}

// convertGlobalWorkflowSteps mirrors the MySQL repository's manifest-step conversion (Batch R1),
// approximated for the in-memory test double using the current global_workflow_steps shape
// directly (no separate version-manifest snapshot exists in-memory).
func convertGlobalWorkflowSteps(steps []disclosureapp.GlobalWorkflowStepInput) []disclosureapp.WorkflowStepDTO {
	out := make([]disclosureapp.WorkflowStepDTO, 0, len(steps))
	for _, s := range steps {
		dueRule := s.DueRule
		if dueRule == "" && s.ProcessingDays > 0 {
			dueRule = fmt.Sprintf("T+%d", s.ProcessingDays)
		}
		out = append(out, disclosureapp.WorkflowStepDTO{
			StepID:          s.StepID,
			Stage:           s.Stage,
			Instructions:    s.Instructions,
			DepartmentID:    s.DepartmentID,
			AssigneeRoleIds: s.AssigneeRoleIds,
			DueRule:         dueRule,
			ProcessingDays:  s.ProcessingDays,
			DisplayOrder:    s.DisplayOrder,
		})
	}
	return out
}

// ── Periodic auto-creation stubs (not used by in-memory / dev server) ────────

func (r *Repository) ListActivePeriodicTypes(_ context.Context) ([]disclosureapp.PeriodicTypeRow, error) {
	return nil, nil
}

func (r *Repository) GetPeriodicCycle(_ context.Context, _, _, _ string) (*disclosureapp.PeriodicCycleRow, error) {
	return nil, nil
}

func (r *Repository) InsertPeriodicCycle(_ context.Context, _ disclosureapp.PeriodicCycleRow) error {
	return nil
}

func (r *Repository) DeleteUnmaterializedPeriodicCycle(_ context.Context, _ string) error {
	return nil
}

func (r *Repository) UpsertPeriodicCycle(_ context.Context, _ disclosureapp.PeriodicCycleRow) error {
	return nil
}

func (r *Repository) ListPendingCycles(_ context.Context, _ time.Time, _ int) ([]disclosureapp.PeriodicCycleRow, error) {
	return nil, nil
}

func (r *Repository) TryClaimPeriodicCycle(_ context.Context, _ string) (bool, error) {
	return true, nil
}

func (r *Repository) ReleasePeriodicCycleClaim(_ context.Context, _ string) error {
	return nil
}

func (r *Repository) UpdateCycleRecord(_ context.Context, _, _ string) error {
	return nil
}

func (r *Repository) ListAllActiveCompanyIDs(_ context.Context) ([]string, error) {
	return nil, nil
}

func (r *Repository) GetCompanyTypePreference(_ context.Context, _, _ string) (*disclosureapp.CompanyTypePreference, error) {
	return nil, nil
}

func (r *Repository) UpsertCompanyTypePreference(_ context.Context, _ disclosureapp.CompanyTypePreference) error {
	return nil
}

func (r *Repository) GetCompanyTypeDeadlineContext(ctx context.Context, companyID, _ string) (disclosureapp.CompanyDeadlineContext, error) {
	return r.GetCompanyDeadlineContext(ctx, companyID)
}

func (r *Repository) CountCompanyTemplatesByCompanyID(_ context.Context, companyID string) (int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	count := 0
	for _, dt := range r.catalog {
		if dt.OwnerCompanyID == companyID {
			count++
		}
	}
	return count, nil
}

// BE-004A stubs — in-memory path not used in production for these write APIs.

func (r *Repository) CreateCompanyTemplate(_ context.Context, _ disclosureapp.CreateCompanyTemplateRequest) (*disclosureapp.CompanyTemplateWriteResponse, error) {
	return nil, fmt.Errorf("not implemented in memory")
}

func (r *Repository) UpdateCompanyTemplate(_ context.Context, _ disclosureapp.UpdateCompanyTemplateRequest) (*disclosureapp.CompanyTemplateWriteResponse, error) {
	return nil, fmt.Errorf("not implemented in memory")
}

func (r *Repository) GetCompanyTemplateForLifecycle(_ context.Context, _, _ string) (*disclosureapp.CompanyTemplateWriteResponse, error) {
	return nil, fmt.Errorf("not implemented in memory")
}

func (r *Repository) TransitionCompanyTemplateReviewStatus(_ context.Context, _, _, _, _ string) error {
	return fmt.Errorf("not implemented in memory")
}
