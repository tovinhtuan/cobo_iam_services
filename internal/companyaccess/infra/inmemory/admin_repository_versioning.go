package inmemory

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"time"

	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	"github.com/cobo/cobo_iam_services/internal/companyaccess/configversion"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

type notificationVersionRow struct {
	row  caapp.ConfigVersionRow
	raw  []byte
	rule string
}

type rbacVersionRow struct {
	row caapp.ConfigVersionRow
	raw []byte
}

func (r *AdminRepository) BuildNotificationRuleSnapshotJSON(ctx context.Context, companyID, ruleID string) ([]byte, error) {
	rules, err := r.ListNotificationRules(ctx, companyID)
	if err != nil {
		return nil, err
	}
	for _, rule := range rules {
		if rule.NotificationRuleID == ruleID {
			snap := configversion.NotificationRuleSnapshot{
				SchemaVersion:      configversion.NotificationSnapshotSchema,
				NotificationRuleID: rule.NotificationRuleID,
				RuleCode:           rule.RuleCode,
				Status:             rule.Status,
				Payload:            rule.Payload,
			}
			return json.Marshal(snap)
		}
	}
	return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "notification rule not found", nil)
}

func (r *AdminRepository) BuildRBACMatrixSnapshotJSON(ctx context.Context, companyID string) ([]byte, error) {
	roles, err := r.ListRoles(ctx, companyID)
	if err != nil {
		return nil, err
	}
	snap := configversion.RBACMatrixSnapshot{
		SchemaVersion:     configversion.RBACMatrixSnapshotSchema,
		RolePermissions:   []configversion.RolePermissionEntry{},
		DirectPermissions: []configversion.DirectPermissionEntry{},
	}
	for _, role := range roles {
		perms, err := r.ListRolePermissions(ctx, companyID, role.RoleID)
		if err != nil {
			return nil, err
		}
		for _, p := range perms.Permissions {
			snap.RolePermissions = append(snap.RolePermissions, configversion.RolePermissionEntry{
				RoleID: role.RoleID, PermissionID: p.PermissionID,
			})
		}
	}
	direct, err := r.ListActiveDirectPermissionsByCompany(ctx, companyID)
	if err != nil {
		return nil, err
	}
	for _, d := range direct {
		snap.DirectPermissions = append(snap.DirectPermissions, configversion.DirectPermissionEntry{
			MembershipID: d.MembershipID, PermissionCode: d.PermissionCode,
		})
	}
	return json.Marshal(snap)
}

func (r *AdminRepository) InsertNotificationRuleVersion(_ context.Context, in caapp.InsertNotificationRuleVersionInput) (*caapp.ConfigVersionRow, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.notificationVersions == nil {
		r.notificationVersions = []notificationVersionRow{}
	}
	next := 1
	for _, v := range r.notificationVersions {
		if v.row.CompanyID == in.CompanyID && v.rule == in.RuleID && v.row.VersionNo >= next {
			next = v.row.VersionNo + 1
		}
	}
	row := caapp.ConfigVersionRow{
		ID: in.ID, CompanyID: in.CompanyID, AggregateType: configversion.AggregateNotificationRule,
		AggregateID: in.RuleID, VersionNo: next, CreatedBy: in.CreatedBy, CreatedAt: time.Now().UTC(),
		Reason: in.Reason, Source: in.Source,
	}
	r.notificationVersions = append(r.notificationVersions, notificationVersionRow{row: row, raw: append([]byte(nil), in.SnapshotJSON...), rule: in.RuleID})
	return &row, nil
}

func (r *AdminRepository) InsertRBACMatrixSnapshot(_ context.Context, in caapp.InsertRBACMatrixSnapshotInput) (*caapp.ConfigVersionRow, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.rbacVersions == nil {
		r.rbacVersions = []rbacVersionRow{}
	}
	next := 1
	for _, v := range r.rbacVersions {
		if v.row.CompanyID == in.CompanyID && v.row.VersionNo >= next {
			next = v.row.VersionNo + 1
		}
	}
	row := caapp.ConfigVersionRow{
		ID: in.ID, CompanyID: in.CompanyID, AggregateType: configversion.AggregateRBACMatrix,
		VersionNo: next, CreatedBy: in.CreatedBy, CreatedAt: time.Now().UTC(),
		Reason: in.Reason, Source: in.Source,
	}
	r.rbacVersions = append(r.rbacVersions, rbacVersionRow{row: row, raw: append([]byte(nil), in.SnapshotJSON...)})
	return &row, nil
}

func (r *AdminRepository) ListNotificationRuleVersions(_ context.Context, companyID, ruleID string, limit int) ([]caapp.ConfigVersionRow, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]caapp.ConfigVersionRow, 0)
	for _, v := range r.notificationVersions {
		if v.row.CompanyID != companyID {
			continue
		}
		if ruleID != "" && v.rule != ruleID {
			continue
		}
		out = append(out, v.row)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].VersionNo > out[j].VersionNo })
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (r *AdminRepository) GetNotificationRuleVersion(_ context.Context, companyID, ruleID string, versionNo int) (*caapp.ConfigVersionDetail, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, v := range r.notificationVersions {
		if v.row.CompanyID == companyID && v.rule == ruleID && v.row.VersionNo == versionNo {
			d := v.row
			return &caapp.ConfigVersionDetail{ConfigVersionRow: d, SnapshotJSON: append([]byte(nil), v.raw...)}, nil
		}
	}
	return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "version not found", nil)
}

func (r *AdminRepository) ListRBACMatrixVersions(_ context.Context, companyID string, limit int) ([]caapp.ConfigVersionRow, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]caapp.ConfigVersionRow, 0)
	for _, v := range r.rbacVersions {
		if v.row.CompanyID == companyID {
			out = append(out, v.row)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].VersionNo > out[j].VersionNo })
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (r *AdminRepository) GetRBACMatrixVersion(_ context.Context, companyID string, versionNo int) (*caapp.ConfigVersionDetail, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, v := range r.rbacVersions {
		if v.row.CompanyID == companyID && v.row.VersionNo == versionNo {
			d := v.row
			return &caapp.ConfigVersionDetail{ConfigVersionRow: d, SnapshotJSON: append([]byte(nil), v.raw...)}, nil
		}
	}
	return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "version not found", nil)
}

func (r *AdminRepository) RestoreNotificationRuleFromSnapshot(ctx context.Context, companyID string, raw []byte) error {
	var snap configversion.NotificationRuleSnapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid snapshot_json", nil)
	}
	status := strings.TrimSpace(snap.Status)
	if status == "" {
		status = "active"
	}
	payload := snap.Payload
	if payload == nil {
		payload = map[string]any{}
	}
	return r.UpdateNotificationRuleMerged(ctx, companyID, snap.NotificationRuleID, payload, &status)
}

func (r *AdminRepository) RestoreRBACMatrixFromSnapshot(ctx context.Context, companyID, actorUserID string, raw []byte) error {
	var snap configversion.RBACMatrixSnapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid snapshot_json", nil)
	}
	roles, err := r.ListRoles(ctx, companyID)
	if err != nil {
		return err
	}
	roleSet := map[string]struct{}{}
	for _, role := range roles {
		roleSet[role.RoleID] = struct{}{}
	}
	target := map[string]map[string]struct{}{}
	for _, e := range snap.RolePermissions {
		if _, ok := roleSet[e.RoleID]; !ok {
			continue
		}
		if target[e.RoleID] == nil {
			target[e.RoleID] = map[string]struct{}{}
		}
		target[e.RoleID][e.PermissionID] = struct{}{}
	}
	for roleID := range roleSet {
		current, err := r.ListRolePermissions(ctx, companyID, roleID)
		if err != nil {
			return err
		}
		cur := map[string]struct{}{}
		for _, p := range current.Permissions {
			cur[p.PermissionID] = struct{}{}
		}
		want := target[roleID]
		for pid := range cur {
			if _, ok := want[pid]; !ok {
				_ = r.RemoveRolePermission(ctx, roleID, pid)
			}
		}
		for pid := range want {
			if _, ok := cur[pid]; !ok {
				_ = r.AddRolePermission(ctx, roleID, pid)
			}
		}
	}
	for key := range r.directPermissions {
		parts := strings.SplitN(key, ":", 2)
		if len(parts) != 2 {
			continue
		}
		mid, code := parts[0], parts[1]
		_ = r.RevokeDirectPermission(ctx, mid, code, actorUserID)
	}
	for _, d := range snap.DirectPermissions {
		_ = r.InsertDirectPermission(ctx, d.MembershipID, companyID, d.PermissionCode, actorUserID)
	}
	return nil
}
