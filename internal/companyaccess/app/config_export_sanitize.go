package app

import (
	"fmt"
	"net/http"
	"strings"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

var configExportSecretKeyPatterns = []string{
	"password", "token", "secret", "smtp", "credential", "jwt", "private_key", "access_key", "refresh_token",
}

var configExportPIIKeys = map[string]struct{}{
	"email":     {},
	"full_name": {},
	"login_id":  {},
}

func isConfigExportSecretKey(key string) bool {
	k := strings.ToLower(strings.TrimSpace(key))
	for _, p := range configExportSecretKeyPatterns {
		if strings.Contains(k, p) {
			return true
		}
	}
	return false
}

func sanitizeConfigExportValue(v any, warnings *[]string, path string) (any, error) {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			fullPath := k
			if path != "" {
				fullPath = path + "." + k
			}
			if isConfigExportSecretKey(k) {
				*warnings = append(*warnings, fmt.Sprintf("omitted secret-shaped field %s", fullPath))
				continue
			}
			if _, isPII := configExportPIIKeys[strings.ToLower(k)]; isPII {
				*warnings = append(*warnings, fmt.Sprintf("omitted PII field %s", fullPath))
				continue
			}
			sanitized, err := sanitizeConfigExportValue(val, warnings, fullPath)
			if err != nil {
				return nil, err
			}
			out[k] = sanitized
		}
		return out, nil
	case []any:
		out := make([]any, 0, len(t))
		for i, item := range t {
			itemPath := fmt.Sprintf("%s[%d]", path, i)
			sanitized, err := sanitizeConfigExportValue(item, warnings, itemPath)
			if err != nil {
				return nil, err
			}
			out = append(out, sanitized)
		}
		return out, nil
	default:
		return v, nil
	}
}

func sanitizeConfigExportModuleData(raw []byte, warnings *[]string) (map[string]any, error) {
	var decoded any
	if err := jsonUnmarshal(raw, &decoded); err != nil {
		return nil, perr.NewHTTPError(http.StatusUnprocessableEntity, perr.CodeInvalidRequest, "invalid module snapshot", nil)
	}
	sanitized, err := sanitizeConfigExportValue(decoded, warnings, "")
	if err != nil {
		return nil, err
	}
	m, ok := sanitized.(map[string]any)
	if !ok {
		return nil, perr.NewHTTPError(http.StatusUnprocessableEntity, perr.CodeInvalidRequest, "module data must be object", nil)
	}
	if containsSecretKey(m) {
		return nil, perr.NewHTTPError(http.StatusUnprocessableEntity, perr.CodeInvalidRequest, "unable to sanitize secret-shaped fields", nil)
	}
	return m, nil
}

func containsSecretKey(v any) bool {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			if isConfigExportSecretKey(k) {
				return true
			}
			if containsSecretKey(val) {
				return true
			}
		}
	case []any:
		for _, item := range t {
			if containsSecretKey(item) {
				return true
			}
		}
	}
	return false
}

// jsonUnmarshal is a thin wrapper so tests can stub if needed; uses encoding/json.
func jsonUnmarshal(data []byte, v any) error {
	return jsonDecode(data, v)
}
