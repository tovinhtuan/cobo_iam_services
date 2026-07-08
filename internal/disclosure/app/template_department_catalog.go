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

const templateDepartmentCodePrefix = "tpl_dept"
const templateDepartmentCodeMaxLen = 64

// SlugifyTemplateDepartmentCode builds an ASCII internal code from a Unicode display name.
func SlugifyTemplateDepartmentCode(name string) string {
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
	maxBase := templateDepartmentCodeMaxLen - len(templateDepartmentCodePrefix) - 1
	if maxBase < 8 {
		maxBase = 8
	}
	if len(base) > maxBase {
		base = base[:maxBase]
		base = strings.TrimRight(base, "_")
	}
	if base == "" {
		return templateDepartmentCodePrefix + "_item"
	}
	return templateDepartmentCodePrefix + "_" + base
}

func normalizeTemplateDepartmentNameKey(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func (s *service) resolveTemplateDepartmentCreate(ctx context.Context, req *CmsTemplateDepartmentCreateRequest) error {
	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)
	if req.Name == "" {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "name is required", nil)
	}
	items, err := s.repo.ListTemplateDepartments(ctx)
	if err != nil {
		return err
	}
	nameKey := normalizeTemplateDepartmentNameKey(req.Name)
	for _, d := range items {
		if normalizeTemplateDepartmentNameKey(d.DepartmentName) == nameKey {
			return &perr.HTTPError{
				HTTPStatus: http.StatusConflict,
				Code:       perr.CodeStateConflict,
				Message:    "template department name already exists",
				Details:    map[string]any{"field": "name"},
			}
		}
	}
	if strings.TrimSpace(req.Code) == "" {
		req.Code = uniqueTemplateDepartmentCode(items, SlugifyTemplateDepartmentCode(req.Name))
	}
	return nil
}

func uniqueTemplateDepartmentCode(existing []TemplateDepartmentDTO, base string) string {
	used := make(map[string]struct{}, len(existing))
	for _, d := range existing {
		used[d.DepartmentCode] = struct{}{}
	}
	code := base
	for i := 2; ; i++ {
		if _, ok := used[code]; !ok {
			return code
		}
		suffix := fmt.Sprintf("_%d", i)
		trimLen := templateDepartmentCodeMaxLen - len(suffix)
		if trimLen < 1 {
			trimLen = 1
		}
		prefix := base
		if len(prefix) > trimLen {
			prefix = strings.TrimRight(prefix[:trimLen], "_")
		}
		code = prefix + suffix
	}
}
