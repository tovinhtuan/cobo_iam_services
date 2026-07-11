package app

import (
	"regexp"
	"strings"
)

var (
	reDeptUUID     = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	reDeptTechCode = regexp.MustCompile(`(?i)^(d\d+|d[_-][a-z0-9_-]+|dept-\d+|dep_[a-z0-9_-]+|ou_dept[_-][a-z0-9_-]+|tpl_dept[_-][a-z0-9_-]+)$`)
)

// DepartmentRef is a resolved department identity from company org and/or
// workflow template catalog sources.
type DepartmentRef struct {
	ID   string
	Code string
	Name string
}

// DepartmentDict resolves department ids/codes to display names and expands
// filter aliases so company UUID options match workflow template codes
// (e.g. dept-004) that share the same name.
type DepartmentDict struct {
	byID   map[string]DepartmentRef
	byCode map[string]DepartmentRef
	byName map[string][]DepartmentRef
}

func NewDepartmentDict(companyDepts, templateDepts []DeadlineAlertFilterOptionDTO) DepartmentDict {
	d := DepartmentDict{
		byID:   map[string]DepartmentRef{},
		byCode: map[string]DepartmentRef{},
		byName: map[string][]DepartmentRef{},
	}
	// Company org is added first and must win on id/code collisions. Template
	// catalog only fills missing codes (e.g. dept-004) so a corrupted template
	// name cannot overwrite a correct company department label.
	add := func(id, code, name string) {
		id = strings.TrimSpace(id)
		code = strings.TrimSpace(code)
		name = strings.TrimSpace(name)
		if id == "" && code == "" {
			return
		}
		if id == "" {
			id = code
		}
		if name == "" {
			return
		}
		ref := DepartmentRef{ID: id, Code: code, Name: name}
		idKey := strings.ToLower(id)
		if _, exists := d.byID[idKey]; !exists {
			d.byID[idKey] = ref
		}
		if code != "" {
			codeKey := strings.ToLower(code)
			if _, exists := d.byCode[codeKey]; !exists {
				d.byCode[codeKey] = ref
			}
		}
		nk := strings.ToLower(name)
		d.byName[nk] = append(d.byName[nk], ref)
	}
	for _, opt := range companyDepts {
		code := strings.TrimSpace(opt.Code)
		if code == "" {
			code = strings.TrimSpace(opt.ID)
		}
		add(opt.ID, code, opt.Name)
	}
	for _, opt := range templateDepts {
		code := strings.TrimSpace(opt.Code)
		if code == "" {
			code = strings.TrimSpace(opt.ID)
		}
		add(opt.ID, code, opt.Name)
	}
	return d
}

// LooksLikeTechnicalDepartmentRef detects raw id/code tokens that must not be shown to users.
func LooksLikeTechnicalDepartmentRef(raw string) bool {
	s := strings.TrimSpace(raw)
	if s == "" {
		return false
	}
	if reDeptUUID.MatchString(s) {
		return true
	}
	return reDeptTechCode.MatchString(s)
}

// ResolveLabel maps a raw workflow/record department token to a display name.
// Empty string means no reliable mapping for a technical token.
// Prefer byCode over byID so a template catalog row keyed as id=dept-00N cannot
// shadow a company department that already registered the same code with a
// correct UTF-8 name.
func (d DepartmentDict) ResolveLabel(raw string) string {
	key := strings.ToLower(strings.TrimSpace(raw))
	if key == "" {
		return ""
	}
	if ref, ok := d.byCode[key]; ok {
		return ref.Name
	}
	if ref, ok := d.byID[key]; ok {
		return ref.Name
	}
	if refs := d.byName[key]; len(refs) > 0 {
		return refs[0].Name
	}
	if LooksLikeTechnicalDepartmentRef(raw) {
		return ""
	}
	// Already a human-readable label stored in the snapshot.
	return strings.TrimSpace(raw)
}

// ResolveActiveDepartmentLabels converts raw active-department tokens to names.
// Unmapped technical ids/codes are dropped (FE shows "Chưa xác định").
func ResolveActiveDepartmentLabels(raw []string, dict DepartmentDict) []string {
	if len(raw) == 0 {
		return []string{}
	}
	out := make([]string, 0, len(raw))
	seen := map[string]struct{}{}
	for _, token := range raw {
		label := dict.ResolveLabel(token)
		if label == "" {
			continue
		}
		lk := strings.ToLower(label)
		if _, ok := seen[lk]; ok {
			continue
		}
		seen[lk] = struct{}{}
		out = append(out, label)
	}
	return out
}

// FilterAliases returns all tokens that should match a selected department option value
// (company department id/code/name + template codes sharing the same name).
func (d DepartmentDict) FilterAliases(selectedID string) []string {
	selectedID = strings.TrimSpace(selectedID)
	if selectedID == "" {
		return nil
	}
	key := strings.ToLower(selectedID)
	ref, ok := d.byID[key]
	if !ok {
		ref, ok = d.byCode[key]
	}
	if !ok {
		if refs := d.byName[key]; len(refs) > 0 {
			ref = refs[0]
			ok = true
		}
	}
	if !ok {
		return []string{selectedID}
	}

	aliases := map[string]struct{}{}
	add := func(v string) {
		v = strings.TrimSpace(v)
		if v == "" {
			return
		}
		aliases[strings.ToLower(v)] = struct{}{}
	}
	add(ref.ID)
	add(ref.Code)
	add(ref.Name)
	for _, sibling := range d.byName[strings.ToLower(ref.Name)] {
		add(sibling.ID)
		add(sibling.Code)
		add(sibling.Name)
	}
	out := make([]string, 0, len(aliases))
	for a := range aliases {
		out = append(out, a)
	}
	return out
}

// MergeDepartmentFilterOptions merges company departments with template catalog
// entries that are not already represented by the same id/code/name.
func MergeDepartmentFilterOptions(companyDepts, templateDepts []DeadlineAlertFilterOptionDTO) []DeadlineAlertFilterOptionDTO {
	out := make([]DeadlineAlertFilterOptionDTO, 0, len(companyDepts)+len(templateDepts))
	seen := map[string]struct{}{}
	seenName := map[string]struct{}{}
	add := func(opt DeadlineAlertFilterOptionDTO) {
		id := strings.TrimSpace(opt.ID)
		name := strings.TrimSpace(opt.Name)
		if id == "" || name == "" {
			return
		}
		ik := strings.ToLower(id)
		if _, ok := seen[ik]; ok {
			return
		}
		nk := strings.ToLower(name)
		if _, ok := seenName[nk]; ok {
			return
		}
		seen[ik] = struct{}{}
		seenName[nk] = struct{}{}
		code := strings.TrimSpace(opt.Code)
		if code == "" {
			code = id
		}
		out = append(out, DeadlineAlertFilterOptionDTO{ID: id, Code: code, Name: name})
	}
	for _, opt := range companyDepts {
		add(opt)
	}
	for _, opt := range templateDepts {
		id := strings.TrimSpace(opt.ID)
		if id == "" {
			id = strings.TrimSpace(opt.Code)
		}
		add(DeadlineAlertFilterOptionDTO{ID: id, Code: opt.Code, Name: opt.Name})
	}
	return out
}
