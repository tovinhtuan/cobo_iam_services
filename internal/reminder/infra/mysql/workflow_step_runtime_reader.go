package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	deadlinealertsapp "github.com/cobo/cobo_iam_services/internal/deadlinealerts/app"
	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	reminderapp "github.com/cobo/cobo_iam_services/internal/reminder/app"
	workflowapp "github.com/cobo/cobo_iam_services/internal/workflow/app"
)

const reminderRuntimeTimezone = "Asia/Ho_Chi_Minh"

// WorkflowStepRuntimeReader implements reminderapp.WorkflowStepRuntimeReader. It derives the
// current step exactly like the deadline steps screen (ComputeDeadlineSteps over
// workflow_instance_step_states), so reminders follow the same progress users see (RMC-01).
type WorkflowStepRuntimeReader struct {
	db *sql.DB
}

func NewWorkflowStepRuntimeReader(db *sql.DB) *WorkflowStepRuntimeReader {
	return &WorkflowStepRuntimeReader{db: db}
}

func (r *WorkflowStepRuntimeReader) WorkflowStepRuntime(ctx context.Context, companyID, workflowInstanceID string, asOf time.Time) (*reminderapp.WorkflowStepRuntime, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("workflow step runtime reader: nil db")
	}
	companyID = strings.TrimSpace(companyID)
	workflowInstanceID = strings.TrimSpace(workflowInstanceID)
	var (
		status, source sql.NullString
		t0             sql.NullTime
		snapshot       []byte
	)
	err := r.db.QueryRowContext(ctx, `
		SELECT status, workflow_source, t0_date, snapshot_json
		FROM workflow_instances
		WHERE company_id = ? AND workflow_instance_id = ?
	`, companyID, workflowInstanceID).Scan(&status, &source, &t0, &snapshot)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("workflow step runtime instance: %w", err)
	}
	states, err := r.listStepStates(ctx, companyID, workflowInstanceID)
	if err != nil {
		return nil, err
	}
	var t0Date *time.Time
	if t0.Valid {
		v := t0.Time.UTC()
		t0Date = &v
	}
	return buildWorkflowStepRuntime(status.String, source.String, t0Date, snapshot, states, asOf)
}

func (r *WorkflowStepRuntimeReader) listStepStates(ctx context.Context, companyID, workflowInstanceID string) (map[string]deadlinealertsapp.StepRuntimeState, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT step_code, completed_at, marked_incomplete_at, delay_days_applied
		FROM workflow_instance_step_states
		WHERE company_id = ? AND workflow_instance_id = ?
	`, companyID, workflowInstanceID)
	if err != nil {
		return nil, fmt.Errorf("workflow step runtime states: %w", err)
	}
	defer rows.Close()
	out := map[string]deadlinealertsapp.StepRuntimeState{}
	for rows.Next() {
		var (
			code              string
			completed, marked sql.NullTime
			delay             int
		)
		if err := rows.Scan(&code, &completed, &marked, &delay); err != nil {
			return nil, fmt.Errorf("workflow step runtime state scan: %w", err)
		}
		st := deadlinealertsapp.StepRuntimeState{StepCode: code, DelayDaysApplied: delay}
		if completed.Valid {
			v := completed.Time.UTC()
			st.CompletedAt = &v
		}
		if marked.Valid {
			v := marked.Time.UTC()
			st.MarkedIncompleteAt = &v
		}
		out[code] = st
	}
	return out, rows.Err()
}

// buildWorkflowStepRuntime maps an instance row plus step states to the reminder runtime view.
// Without T0 or snapshot, steps carry identity only (no current / end), so non-proposal
// reminders wait instead of being sent on guesses.
func buildWorkflowStepRuntime(status, source string, t0 *time.Time, snapshotJSON []byte, states map[string]deadlinealertsapp.StepRuntimeState, asOf time.Time) (*reminderapp.WorkflowStepRuntime, error) {
	out := &reminderapp.WorkflowStepRuntime{InstanceStatus: strings.TrimSpace(status), WorkflowSource: strings.TrimSpace(source)}
	var snapshot []workflowapp.StepSnapshot
	if len(snapshotJSON) > 0 && string(snapshotJSON) != "null" {
		if err := json.Unmarshal(snapshotJSON, &snapshot); err != nil {
			return nil, fmt.Errorf("workflow step runtime snapshot: %w", err)
		}
	}
	codeOf := func(s workflowapp.StepSnapshot) string {
		if c := strings.TrimSpace(s.StepCode); c != "" {
			return c
		}
		return strings.TrimSpace(s.StepID)
	}
	if t0 == nil || len(snapshot) == 0 {
		for _, s := range snapshot {
			code := codeOf(s)
			_, done := states[code]
			out.Steps = append(out.Steps, reminderapp.WorkflowStepRuntimeStep{
				StepID: strings.TrimSpace(s.StepID), StepCode: code,
				Completed: done && states[code].CompletedAt != nil,
			})
		}
		return out, nil
	}
	loc := disclosureapp.CompanyLocation(reminderRuntimeTimezone)
	resp, err := deadlinealertsapp.ComputeDeadlineSteps(deadlinealertsapp.WorkflowInstanceContext{
		T0Date:   *t0,
		Snapshot: snapshot,
		Timezone: reminderRuntimeTimezone,
	}, states, asOf.In(loc), reminderRuntimeTimezone, false)
	if err != nil {
		return nil, fmt.Errorf("workflow step runtime compute: %w", err)
	}
	stepIDByCode := map[string]string{}
	for _, s := range snapshot {
		stepIDByCode[codeOf(s)] = strings.TrimSpace(s.StepID)
	}
	for _, st := range resp.Steps {
		rs := reminderapp.WorkflowStepRuntimeStep{
			StepID:    stepIDByCode[st.StepCode],
			StepCode:  st.StepCode,
			Completed: st.IsCompleted,
			Current:   resp.CurrentStepCode != "" && st.StepCode == resp.CurrentStepCode && !st.IsCompleted,
		}
		if end, err := time.Parse("2006-01-02", st.PlannedEndDate); err == nil {
			rs.EndDate = end
			rs.HasEnd = true
		}
		out.Steps = append(out.Steps, rs)
	}
	return out, nil
}
