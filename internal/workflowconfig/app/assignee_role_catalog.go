package app

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"unicode"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

const assigneeRoleCodePrefix = "wf_role"
const assigneeRoleCodeMaxLen = 64
const assigneeRoleNameMaxLen = 255

// AssigneeRoleCatalogItem is a platform workflow assignee role catalog entry (not IAM RBAC).
type AssigneeRoleCatalogItem struct {
	RoleCode    string `json:"role_code"`
	RoleName    string `json:"role_name"`
	Description string `json:"description,omitempty"`
	IsSystem    bool   `json:"is_system"`
}

type AssigneeRoleCatalogRepository interface {
	List(ctx context.Context) ([]AssigneeRoleCatalogItem, error)
	Create(ctx context.Context, item AssigneeRoleCatalogItem) (*AssigneeRoleCatalogItem, error)
}

type AssigneeRoleCatalogService struct {
	repo AssigneeRoleCatalogRepository
}

func NewAssigneeRoleCatalogService(repo AssigneeRoleCatalogRepository) *AssigneeRoleCatalogService {
	return &AssigneeRoleCatalogService{repo: repo}
}

// SlugifyAssigneeRoleCode builds an ASCII internal code from a Unicode display name.
func SlugifyAssigneeRoleCode(name string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	ascii, _, _ := transform.String(t, strings.TrimSpace(name))
	ascii = strings.ToLower(ascii)
	var b strings.Builder
	lastUnderscore := false
	for _, r := range ascii {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			b.WriteRune('_')
			lastUnderscore = true
		}
	}
	base := strings.Trim(b.String(), "_")
	maxBase := assigneeRoleCodeMaxLen - len(assigneeRoleCodePrefix) - 1
	if maxBase < 8 {
		maxBase = 8
	}
	if len(base) > maxBase {
		base = base[:maxBase]
		base = strings.TrimRight(base, "_")
	}
	if base == "" {
		return assigneeRoleCodePrefix + "_item"
	}
	return assigneeRoleCodePrefix + "_" + base
}

func normalizeAssigneeRoleNameKey(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func catalogItemToRoleDefinition(item AssigneeRoleCatalogItem) RoleDefinition {
	code := strings.TrimSpace(item.RoleCode)
	return RoleDefinition{
		RoleID:                  code,
		Code:                    code,
		Name:                    item.RoleName,
		Class:                   RoleClassStandard,
		Locked:                  false,
		AllowedOverride:         true,
		IsApprovalRole:          false,
		Description:             strings.TrimSpace(item.Description),
		AllowedAssignmentScopes: []string{ScopeUser, ScopeDepartment, ScopeGroup, ScopeCompanyDefault},
		Aliases:                 []string{code},
	}
}

// RegistryWithCatalog merges the static seed registry with custom catalog entries.
func RegistryWithCatalog(catalog []AssigneeRoleCatalogItem) *RoleRegistry {
	defs := CanonicalRoleSeed()
	for _, item := range catalog {
		defs = append(defs, catalogItemToRoleDefinition(item))
	}
	return NewRoleRegistry(defs)
}

func (s *AssigneeRoleCatalogService) List(ctx context.Context) ([]AssigneeRoleCatalogItem, error) {
	if s == nil || s.repo == nil {
		return nil, nil
	}
	return s.repo.List(ctx)
}

func (s *AssigneeRoleCatalogService) MergedRegistry(ctx context.Context) (*RoleRegistry, error) {
	items, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	return RegistryWithCatalog(items), nil
}

type CreateAssigneeRoleRequest struct {
	RoleName string `json:"role_name"`
	Name     string `json:"name"` // alias for role_name
}

func (s *AssigneeRoleCatalogService) Create(ctx context.Context, req CreateAssigneeRoleRequest) (*AssigneeRoleCatalogItem, error) {
	if s == nil || s.repo == nil {
		return nil, perr.NewHTTPError(http.StatusServiceUnavailable, perr.CodeInvalidRequest, "assignee role catalog is not available", nil)
	}
	name := strings.TrimSpace(req.RoleName)
	if name == "" {
		name = strings.TrimSpace(req.Name)
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "role_name is required", nil)
	}
	if len(name) > assigneeRoleNameMaxLen {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "role_name is too long", nil)
	}

	baseReg := DefaultRoleRegistry()
	nameKey := normalizeAssigneeRoleNameKey(name)
	for _, d := range baseReg.ListRoles() {
		if normalizeAssigneeRoleNameKey(d.Name) == nameKey {
			return nil, assigneeRoleDuplicateNameError()
		}
	}

	existing, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, d := range existing {
		if normalizeAssigneeRoleNameKey(d.RoleName) == nameKey {
			return nil, assigneeRoleDuplicateNameError()
		}
	}

	code := uniqueAssigneeRoleCode(baseReg, existing, SlugifyAssigneeRoleCode(name))
	item := AssigneeRoleCatalogItem{
		RoleCode: code,
		RoleName: name,
	}
	return s.repo.Create(ctx, item)
}

func assigneeRoleDuplicateNameError() error {
	return &perr.HTTPError{
		HTTPStatus: http.StatusConflict,
		Code:       perr.CodeStateConflict,
		Message:    "workflow assignee role name already exists",
		Details:    map[string]any{"field": "role_name"},
	}
}

func uniqueAssigneeRoleCode(base *RoleRegistry, existing []AssigneeRoleCatalogItem, baseCode string) string {
	used := make(map[string]struct{})
	for _, d := range base.ListRoles() {
		used[normKey(d.Code)] = struct{}{}
		used[normKey(d.RoleID)] = struct{}{}
		for _, a := range d.Aliases {
			used[normKey(a)] = struct{}{}
		}
	}
	for _, d := range existing {
		used[normKey(d.RoleCode)] = struct{}{}
	}
	code := baseCode
	for i := 2; ; i++ {
		if _, ok := used[normKey(code)]; !ok {
			return code
		}
		suffix := fmt.Sprintf("_%d", i)
		trimLen := assigneeRoleCodeMaxLen - len(suffix)
		if trimLen < 1 {
			trimLen = 1
		}
		prefix := baseCode
		if len(prefix) > trimLen {
			prefix = strings.TrimRight(prefix[:trimLen], "_")
		}
		code = prefix + suffix
	}
}
