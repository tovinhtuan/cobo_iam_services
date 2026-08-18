package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	reminderapp "github.com/cobo/cobo_iam_services/internal/reminder/app"
)

// InstanceSnapshotStepReader reads frozen workflow_instances.snapshot_json.
type InstanceSnapshotStepReader struct {
	db *sql.DB
}

func NewInstanceSnapshotStepReader(db *sql.DB) *InstanceSnapshotStepReader {
	return &InstanceSnapshotStepReader{db: db}
}

type snapshotStepRow struct {
	StepID                string   `json:"step_id"`
	StepCode              string   `json:"step_code"`
	Stage                 string   `json:"stage"`
	Department            string   `json:"department"`
	AssigneeRole          string   `json:"assignee_role"`
	AssigneeMembershipID  string   `json:"assignee_membership_id"`
	AssigneeMembershipIDs []string `json:"assignee_membership_ids"`
	Instructions          string   `json:"instructions"`
}

// GetStepByInstance returns the frozen step matching stepID or step_code.
func (r *InstanceSnapshotStepReader) GetStepByInstance(ctx context.Context, companyID, workflowInstanceID, stepID string) (*reminderapp.WorkflowStepConfig, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	companyID = strings.TrimSpace(companyID)
	workflowInstanceID = strings.TrimSpace(workflowInstanceID)
	stepID = strings.TrimSpace(stepID)
	if companyID == "" || workflowInstanceID == "" || stepID == "" {
		return nil, nil
	}
	var raw sql.NullString
	err := r.db.QueryRowContext(ctx, `
		SELECT CAST(snapshot_json AS CHAR)
		FROM workflow_instances
		WHERE company_id = ? AND workflow_instance_id = ?
	`, companyID, workflowInstanceID).Scan(&raw)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read workflow instance snapshot: %w", err)
	}
	if !raw.Valid || strings.TrimSpace(raw.String) == "" {
		return nil, nil
	}
	var steps []snapshotStepRow
	if err := json.Unmarshal([]byte(raw.String), &steps); err != nil {
		return nil, fmt.Errorf("unmarshal workflow instance snapshot: %w", err)
	}
	want := strings.ToLower(stepID)
	for _, st := range steps {
		id := strings.TrimSpace(st.StepID)
		code := strings.TrimSpace(st.StepCode)
		if strings.ToLower(id) != want && strings.ToLower(code) != want {
			continue
		}
		roles := splitAssigneeRoles(st.AssigneeRole)
		return &reminderapp.WorkflowStepConfig{
			StepID:                firstNonEmpty(id, code, stepID),
			StageName:             strings.TrimSpace(st.Stage),
			Instructions:          strings.TrimSpace(st.Instructions),
			AssigneeRoleIDs:       roles,
			AssigneeMembershipIDs: mergeSnapshotMembershipIDs(st.AssigneeMembershipID, st.AssigneeMembershipIDs),
			DepartmentID:          strings.TrimSpace(st.Department),
		}, nil
	}
	return nil, nil
}

func splitAssigneeRoles(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	return out
}

func mergeSnapshotMembershipIDs(singular string, ids []string) []string {
	out := make([]string, 0, len(ids)+1)
	seen := map[string]struct{}{}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	if len(out) > 0 {
		return out
	}
	if s := strings.TrimSpace(singular); s != "" {
		return []string{s}
	}
	return nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
