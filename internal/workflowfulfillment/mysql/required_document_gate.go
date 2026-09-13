package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	workflowapp "github.com/cobo/cobo_iam_services/internal/workflow/app"
	wff "github.com/cobo/cobo_iam_services/internal/workflowfulfillment"
)

// ValidateRequiredDocumentFulfillmentLocked runs B3 after snapshots are already locked FOR UPDATE
// in this transaction. Caller must also hold the step-state row lock before calling.
// Zero snapshots → nil (legacy preserve). Does not read live documents[].
func ValidateRequiredDocumentFulfillmentLocked(
	ctx context.Context,
	tx *sql.Tx,
	companyID, workflowInstanceID, stepCode string,
	snaps []workflowapp.DocumentRequirementSnapshot,
) error {
	if len(snaps) == 0 {
		return nil
	}
	companyID = strings.TrimSpace(companyID)
	workflowInstanceID = strings.TrimSpace(workflowInstanceID)
	stepCode = strings.TrimSpace(stepCode)
	counts, err := activeFulfillmentCountsByRequirement(ctx, tx, companyID, workflowInstanceID, stepCode)
	if err != nil {
		return err
	}
	missing := wff.EvaluateMissingRequiredDocuments(snaps, counts)
	if len(missing) == 0 {
		return nil
	}
	return wff.NewRequiredDocumentMissingError(missing)
}

// LockDocumentRequirementSnapshots locks all B1 snapshot rows for the step (id ASC).
func LockDocumentRequirementSnapshots(
	ctx context.Context,
	tx *sql.Tx,
	companyID, workflowInstanceID, stepCode string,
) ([]workflowapp.DocumentRequirementSnapshot, error) {
	companyID = strings.TrimSpace(companyID)
	workflowInstanceID = strings.TrimSpace(workflowInstanceID)
	stepCode = strings.TrimSpace(stepCode)
	rows, err := tx.QueryContext(ctx, `
		SELECT id, company_id, disclosure_record_id, workflow_instance_id, step_code,
		       source_doc_id, requirement_key, name, required,
		       template_file_id, template_file_name, ordinal
		FROM workflow_step_document_requirement_snapshots
		WHERE company_id = ? AND workflow_instance_id = ? AND step_code = ?
		ORDER BY id ASC
		FOR UPDATE
	`, companyID, workflowInstanceID, stepCode)
	if err != nil {
		return nil, fmt.Errorf("lock document requirement snapshots: %w", err)
	}
	defer rows.Close()

	out := make([]workflowapp.DocumentRequirementSnapshot, 0)
	for rows.Next() {
		var row workflowapp.DocumentRequirementSnapshot
		var required int
		var tplID, tplName sql.NullString
		if err := rows.Scan(
			&row.ID,
			&row.CompanyID,
			&row.DisclosureRecordID,
			&row.WorkflowInstanceID,
			&row.StepCode,
			&row.SourceDocID,
			&row.RequirementKey,
			&row.Name,
			&required,
			&tplID,
			&tplName,
			&row.Ordinal,
		); err != nil {
			return nil, err
		}
		row.Required = required != 0
		if tplID.Valid {
			row.TemplateFileID = tplID.String
		}
		if tplName.Valid {
			row.TemplateFileName = tplName.String
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func activeFulfillmentCountsByRequirement(
	ctx context.Context,
	tx *sql.Tx,
	companyID, workflowInstanceID, stepCode string,
) (map[string]int, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT requirement_snapshot_id, COUNT(*)
		FROM workflow_step_document_fulfillment_files
		WHERE company_id = ?
		  AND workflow_instance_id = ?
		  AND step_code = ?
		  AND lifecycle_status = ?
		GROUP BY requirement_snapshot_id
	`, companyID, workflowInstanceID, stepCode, wff.LifecycleActive)
	if err != nil {
		return nil, fmt.Errorf("active fulfillment counts: %w", err)
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var reqID string
		var n int
		if err := rows.Scan(&reqID, &n); err != nil {
			return nil, err
		}
		out[reqID] = n
	}
	return out, rows.Err()
}
