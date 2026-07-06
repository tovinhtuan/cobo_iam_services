package mysql

import (
	"context"
	"database/sql"
	"strings"
	"time"

	deadlinealertsapp "github.com/cobo/cobo_iam_services/internal/deadlinealerts/app"
	workflowapp "github.com/cobo/cobo_iam_services/internal/workflow/app"
)

func (r *Repository) GetWorkflowInstanceByRecord(ctx context.Context, companyID, recordID string) (*deadlinealertsapp.WorkflowInstanceRow, error) {
	var row deadlinealertsapp.WorkflowInstanceRow
	var t0 sql.NullTime
	var snapshot []byte
	err := r.db.QueryRowContext(ctx, `
		SELECT workflow_instance_id,
		       t0_date,
		       snapshot_json
		FROM workflow_instances
		WHERE company_id = ? AND record_id = ?
		ORDER BY workflow_instance_id ASC
		LIMIT 1
	`, companyID, recordID).Scan(&row.WorkflowInstanceID, &t0, &snapshot)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if t0.Valid {
		row.T0Date = t0.Time.UTC()
	}
	row.SnapshotJSON = snapshot
	return &row, nil
}

func (r *Repository) GetEffectiveWorkflowSnapshot(ctx context.Context, companyID, typeID string) ([]workflowapp.StepSnapshot, error) {
	typeID = strings.TrimSpace(typeID)
	if typeID == "" {
		return nil, nil
	}
	eff, err := r.disclosure.GetEffectiveWorkflow(ctx, companyID, typeID)
	if err != nil || eff == nil || len(eff.Workflow) == 0 {
		return nil, err
	}
	return workflowapp.MapEffectiveWorkflowToSnapshot(eff.Workflow, eff.Source), nil
}

func (r *Repository) ListStepStates(ctx context.Context, workflowInstanceID string) (map[string]deadlinealertsapp.StepRuntimeState, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT step_code, completed_at, completed_by_membership_id,
		       marked_incomplete_at, marked_incomplete_by_membership_id,
		       COALESCE(incomplete_reason, ''), delay_days_applied
		FROM workflow_instance_step_states
		WHERE workflow_instance_id = ?
	`, workflowInstanceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]deadlinealertsapp.StepRuntimeState{}
	for rows.Next() {
		var st deadlinealertsapp.StepRuntimeState
		var completedAt, markedIncompleteAt sql.NullTime
		var completedBy, markedBy sql.NullString
		if err := rows.Scan(
			&st.StepCode,
			&completedAt,
			&completedBy,
			&markedIncompleteAt,
			&markedBy,
			&st.IncompleteReason,
			&st.DelayDaysApplied,
		); err != nil {
			return nil, err
		}
		if completedAt.Valid {
			t := completedAt.Time.UTC()
			st.CompletedAt = &t
		}
		if completedBy.Valid {
			st.CompletedByMembershipID = strings.TrimSpace(completedBy.String)
		}
		if markedIncompleteAt.Valid {
			t := markedIncompleteAt.Time.UTC()
			st.MarkedIncompleteAt = &t
		}
		if markedBy.Valid {
			st.MarkedIncompleteByMembershipID = strings.TrimSpace(markedBy.String)
		}
		out[st.StepCode] = st
	}
	return out, rows.Err()
}

func (r *Repository) UpsertStepCompleted(
	ctx context.Context,
	companyID, workflowInstanceID, stepCode, membershipID string,
	at time.Time,
) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO workflow_instance_step_states (
			company_id, workflow_instance_id, step_code,
			completed_at, completed_by_membership_id
		) VALUES (?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			completed_at = VALUES(completed_at),
			completed_by_membership_id = VALUES(completed_by_membership_id),
			updated_at = CURRENT_TIMESTAMP
	`, companyID, workflowInstanceID, stepCode, at, membershipID)
	return err
}

func (r *Repository) UpsertStepIncomplete(
	ctx context.Context,
	companyID, workflowInstanceID, stepCode, membershipID, reason string,
	delayDays int,
	at time.Time,
) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO workflow_instance_step_states (
			company_id, workflow_instance_id, step_code,
			marked_incomplete_at, marked_incomplete_by_membership_id,
			incomplete_reason, delay_days_applied
		) VALUES (?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			marked_incomplete_at = COALESCE(marked_incomplete_at, VALUES(marked_incomplete_at)),
			marked_incomplete_by_membership_id = COALESCE(marked_incomplete_by_membership_id, VALUES(marked_incomplete_by_membership_id)),
			incomplete_reason = COALESCE(NULLIF(incomplete_reason, ''), VALUES(incomplete_reason)),
			delay_days_applied = CASE
				WHEN delay_days_applied > 0 THEN delay_days_applied
				ELSE VALUES(delay_days_applied)
			END,
			updated_at = CURRENT_TIMESTAMP
	`, companyID, workflowInstanceID, stepCode, at, membershipID, reason, delayDays)
	return err
}
