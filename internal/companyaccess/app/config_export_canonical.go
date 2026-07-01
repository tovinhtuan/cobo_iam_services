package app

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"

	"github.com/cobo/cobo_iam_services/internal/companyaccess/configexport"
	"github.com/cobo/cobo_iam_services/internal/companyaccess/configversion"
)

func canonicalizeModuleData(module string, data map[string]any) (map[string]any, error) {
	switch module {
	case configexport.ModuleNotificationAlertChannelPrefs:
		return canonicalizeNotificationSnapshot(data)
	case configexport.ModuleRBACMatrix:
		return canonicalizeRBACSnapshot(data)
	default:
		return data, nil
	}
}

func canonicalizeNotificationSnapshot(data map[string]any) (map[string]any, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	var snap configversion.NotificationRuleSnapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		return data, nil
	}
	if snap.Payload != nil {
		snap.Payload = sortMapKeysDeep(snap.Payload)
	}
	outRaw, err := json.Marshal(snap)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(outRaw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func canonicalizeRBACSnapshot(data map[string]any) (map[string]any, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	var snap configversion.RBACMatrixSnapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		return data, nil
	}
	sort.Slice(snap.RolePermissions, func(i, j int) bool {
		a, b := snap.RolePermissions[i], snap.RolePermissions[j]
		if a.RoleID != b.RoleID {
			return a.RoleID < b.RoleID
		}
		return a.PermissionID < b.PermissionID
	})
	sort.Slice(snap.DirectPermissions, func(i, j int) bool {
		a, b := snap.DirectPermissions[i], snap.DirectPermissions[j]
		if a.MembershipID != b.MembershipID {
			return a.MembershipID < b.MembershipID
		}
		return a.PermissionCode < b.PermissionCode
	})
	outRaw, err := json.Marshal(snap)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(outRaw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func sortMapKeysDeep(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := m[k]
		switch child := v.(type) {
		case map[string]any:
			out[k] = sortMapKeysDeep(child)
		case []any:
			out[k] = child
		default:
			out[k] = v
		}
	}
	return out
}

func buildCanonicalChecksumPayload(modules []string, data map[string]any, warnings []string) map[string]any {
	moduleCopy := append([]string(nil), modules...)
	sort.Strings(moduleCopy)
	warnCopy := append([]string(nil), warnings...)
	sort.Strings(warnCopy)

	dataCopy := make(map[string]any, len(data))
	moduleKeys := append([]string(nil), modules...)
	sort.Strings(moduleKeys)
	for _, mod := range moduleKeys {
		if v, ok := data[mod]; ok {
			dataCopy[mod] = v
		}
	}

	return map[string]any{
		"schema_version": configexport.SchemaVersionEnterpriseExport,
		"package_type":   configexport.PackageTypeEnterpriseExport,
		"modules":        moduleCopy,
		"data":           dataCopy,
		"warnings":       warnCopy,
	}
}

func computeConfigExportChecksum(modules []string, data map[string]any, warnings []string) (string, error) {
	payload := buildCanonicalChecksumPayload(modules, data, warnings)
	raw, err := marshalCanonicalJSON(payload)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func marshalCanonicalJSON(v any) ([]byte, error) {
	var buf bytes.Buffer
	if err := writeCanonicalJSON(&buf, v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeCanonicalJSON(buf *bytes.Buffer, v any) error {
	switch t := v.(type) {
	case nil:
		buf.WriteString("null")
	case bool:
		if t {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}
	case float64:
		b, err := json.Marshal(t)
		if err != nil {
			return err
		}
		buf.Write(b)
	case string:
		b, err := json.Marshal(t)
		if err != nil {
			return err
		}
		buf.Write(b)
	case []any:
		buf.WriteByte('[')
		for i, item := range t {
			if i > 0 {
				buf.WriteByte(',')
			}
			if err := writeCanonicalJSON(buf, item); err != nil {
				return err
			}
		}
		buf.WriteByte(']')
	case []string:
		items := make([]any, len(t))
		for i, s := range t {
			items[i] = s
		}
		return writeCanonicalJSON(buf, items)
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		buf.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				buf.WriteByte(',')
			}
			kb, err := json.Marshal(k)
			if err != nil {
				return err
			}
			buf.Write(kb)
			buf.WriteByte(':')
			if err := writeCanonicalJSON(buf, t[k]); err != nil {
				return err
			}
		}
		buf.WriteByte('}')
	default:
		b, err := json.Marshal(t)
		if err != nil {
			return err
		}
		buf.Write(b)
	}
	return nil
}

func marshalExportArtifact(modules []string, data map[string]any, warnings []string, exportedAt string, exportedBy string, checksum string) ([]byte, error) {
	artifact := map[string]any{
		"schema_version":            configexport.SchemaVersionEnterpriseExport,
		"exported_at":               exportedAt,
		"exported_by_membership_id": exportedBy,
		"package_type":              configexport.PackageTypeEnterpriseExport,
		"modules":                   modules,
		"data":                      data,
		"warnings":                  warnings,
		"checksum":                  checksum,
	}
	return marshalCanonicalJSON(artifact)
}
