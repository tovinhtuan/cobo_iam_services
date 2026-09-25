package mysql

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/cobo/cobo_iam_services/internal/workflowdept"
	wdhttp "github.com/cobo/cobo_iam_services/internal/workflowdept/transport/http"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

func (s *Store) Commit(ctx context.Context, in wdhttp.Write) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	if err := assertDept(ctx, tx, in.CompanyID, in.CompanyDepartmentID); err != nil {
		return 0, err
	}
	var version int64
	var dept string
	err = tx.QueryRowContext(ctx, `
		SELECT version, company_department_id
		FROM company_workflow_department_mappings
		WHERE company_id = ? AND template_department_code = ? AND disclosure_type_id = ? AND step_code = ?
		  AND effective_to IS NULL
		FOR UPDATE
	`, in.CompanyID, in.TemplateCode, empty(in.DisclosureTypeID), empty(in.StepCode)).Scan(&version, &dept)
	has := err == nil
	if err != nil && err != sql.ErrNoRows {
		return 0, err
	}
	action, planErr := workflowdept.PlanWrite(version, has, dept, in.CompanyDepartmentID, in.ExpectedVersion)
	if planErr != nil {
		return 0, planErr
	}
	now := time.Now().UTC()
	switch action {
	case workflowdept.ActionIdempotent:
		if err := tx.Commit(); err != nil {
			return 0, err
		}
		return version, nil
	case workflowdept.ActionReplace:
		if _, err := tx.ExecContext(ctx, `
			UPDATE company_workflow_department_mappings
			SET effective_to = ?, status = 'closed', open_scope_hash = NULL, updated_by = ?
			WHERE company_id = ? AND template_department_code = ? AND disclosure_type_id = ? AND step_code = ?
			  AND version = ? AND effective_to IS NULL
		`, now, in.Actor, in.CompanyID, in.TemplateCode, empty(in.DisclosureTypeID), empty(in.StepCode), version); err != nil {
			return 0, err
		}
		version++
	default:
		var maxV int64
		if err := tx.QueryRowContext(ctx, `
			SELECT COALESCE(MAX(version), 0)
			FROM company_workflow_department_mappings
			WHERE company_id = ? AND template_department_code = ? AND disclosure_type_id = ? AND step_code = ?
		`, in.CompanyID, in.TemplateCode, empty(in.DisclosureTypeID), empty(in.StepCode)).Scan(&maxV); err != nil {
			return 0, err
		}
		version = maxV + 1
	}
	id, err := newID()
	if err != nil {
		return 0, err
	}
	hash := workflowdept.ScopeKeyHash(in.CompanyID, in.TemplateCode, empty(in.DisclosureTypeID), empty(in.StepCode))
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO company_workflow_department_mappings (
			mapping_id, company_id, template_department_code, disclosure_type_id, step_code,
			company_department_id, status, effective_from, version, source,
			scope_key_hash, open_scope_hash, created_by, updated_by
		) VALUES (?, ?, ?, ?, ?, ?, 'active', ?, ?, 'manual', ?, ?, ?, ?)
	`, id, in.CompanyID, in.TemplateCode, empty(in.DisclosureTypeID), empty(in.StepCode), in.CompanyDepartmentID, now, version, hash, hash, in.Actor, in.Actor); err != nil {
		if isDup(err) {
			return 0, workflowdept.ErrVersionConflict
		}
		return 0, err
	}
	if err := registerCode(ctx, tx, in.TemplateCode); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return version, nil
}

func (s *Store) Close(ctx context.Context, companyID, templateCode, typeID, stepCode string, version int64, actor string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var openVer int64
	err = tx.QueryRowContext(ctx, `
		SELECT version FROM company_workflow_department_mappings
		WHERE company_id = ? AND template_department_code = ? AND disclosure_type_id = ? AND step_code = ?
		  AND effective_to IS NULL
		FOR UPDATE
	`, companyID, templateCode, empty(typeID), empty(stepCode)).Scan(&openVer)
	if err == sql.ErrNoRows {
		return workflowdept.ErrNotFound
	}
	if err != nil {
		return err
	}
	if _, err := workflowdept.PlanClose(openVer, true, version); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE company_workflow_department_mappings
		SET effective_to = ?, status = 'closed', open_scope_hash = NULL, updated_by = ?
		WHERE company_id = ? AND template_department_code = ? AND disclosure_type_id = ? AND step_code = ?
		  AND version = ? AND effective_to IS NULL
	`, time.Now().UTC(), actor, companyID, templateCode, empty(typeID), empty(stepCode), version)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) List(ctx context.Context, companyID string) ([]wdhttp.Item, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT m.template_department_code, m.disclosure_type_id, m.step_code, m.company_department_id,
		       COALESCE(d.department_name, ''), m.version, m.status
		FROM company_workflow_department_mappings m
		LEFT JOIN departments d ON d.department_id = m.company_department_id AND d.company_id = m.company_id
		WHERE m.company_id = ? AND m.effective_to IS NULL
		ORDER BY m.template_department_code, m.disclosure_type_id, m.step_code
	`, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []wdhttp.Item
	for rows.Next() {
		var item wdhttp.Item
		if err := rows.Scan(&item.TemplateDepartmentCode, &item.DisclosureTypeID, &item.StepCode, &item.CompanyDepartmentID, &item.CompanyDepartmentName, &item.Version, &item.Status); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) Suggest(ctx context.Context, companyID, templateCode string) ([]wdhttp.Suggestion, error) {
	var catalogName string
	if err := s.db.QueryRowContext(ctx, `
		SELECT department_name FROM workflow_template_departments WHERE department_code = ?
	`, templateCode).Scan(&catalogName); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT department_id, department_name FROM departments
		WHERE company_id = ? AND status = 'active'
	`, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []wdhttp.Suggestion
	var matched int
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		class := workflowdept.ClassifySuggestion(catalogName, name)
		if class == workflowdept.SuggestNone {
			continue
		}
		matched++
		out = append(out, wdhttp.Suggestion{DepartmentID: id, DepartmentName: name, Class: string(class)})
	}
	ambiguous := matched > 1
	for i := range out {
		out[i].Ambiguous = ambiguous
	}
	return out, rows.Err()
}

func (s *Store) Preflight(ctx context.Context, companyID string) ([]wdhttp.PreflightRow, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT cat.department_code, cat.department_name
		FROM workflow_template_departments cat
		ORDER BY cat.display_order, cat.department_code
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []wdhttp.PreflightRow
	for rows.Next() {
		var code, name string
		if err := rows.Scan(&code, &name); err != nil {
			return nil, err
		}
		var n int
		if err := s.db.QueryRowContext(ctx, `
			SELECT COUNT(DISTINCT wi.workflow_instance_id)
			FROM workflow_instances wi
			JOIN JSON_TABLE(wi.snapshot_json, '$[*]' COLUMNS (dept VARCHAR(64) PATH '$.department')) jt
			  ON jt.dept = ?
			WHERE wi.company_id = ?
		`, code, companyID).Scan(&n); err != nil {
			return nil, err
		}
		suggestions, err := s.Suggest(ctx, companyID, code)
		if err != nil {
			return nil, err
		}
		var open int
		if err := s.db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM company_workflow_department_mappings
			WHERE company_id = ? AND template_department_code = ?
			  AND disclosure_type_id = '' AND step_code = '' AND effective_to IS NULL
		`, companyID, code).Scan(&open); err != nil {
			return nil, err
		}
		resolution, fallback := "MISSING", "COMPANY_ADMIN"
		if open > 0 {
			resolution, fallback = "MATCHED", "none"
		}
		row := wdhttp.PreflightRow{
			CompanyID: companyID, TemplateDepartmentCode: code, AffectedCount: n,
			CurrentResolution: resolution, FallbackImpact: fallback,
		}
		if len(suggestions) == 1 {
			row.SuggestionClass = suggestions[0].Class
		} else if len(suggestions) > 1 {
			row.SuggestionClass = "AMBIGUOUS"
			row.Ambiguous = true
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *Store) CloseBatch(ctx context.Context, companyID, batchID, actor string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE company_workflow_department_mappings
		SET effective_to = ?, status = 'closed', open_scope_hash = NULL, updated_by = ?
		WHERE company_id = ? AND backfill_batch_id = ? AND effective_to IS NULL
	`, time.Now().UTC(), actor, companyID, batchID)
	return err
}

func assertDept(ctx context.Context, tx *sql.Tx, companyID, departmentID string) error {
	var status, owner string
	err := tx.QueryRowContext(ctx, `
		SELECT status, company_id FROM departments WHERE department_id = ?
	`, departmentID).Scan(&status, &owner)
	if err == sql.ErrNoRows || owner != companyID {
		return workflowdept.ErrDeptNotInCompany
	}
	if err != nil {
		return err
	}
	if status != "active" {
		return workflowdept.ErrDeptInactive
	}
	return nil
}

func registerCode(ctx context.Context, tx *sql.Tx, code string) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO workflow_template_department_code_registry (department_code)
		SELECT department_code FROM workflow_template_departments WHERE department_code = ?
		ON DUPLICATE KEY UPDATE department_code = workflow_template_department_code_registry.department_code
	`, code)
	return err
}

func (s *Store) CheckDept(ctx context.Context, companyID, departmentID string) error {
	var status, owner string
	err := s.db.QueryRowContext(ctx, `SELECT status, company_id FROM departments WHERE department_id = ?`, departmentID).Scan(&status, &owner)
	if err == sql.ErrNoRows || (err == nil && owner != companyID) {
		return workflowdept.ErrDeptNotInCompany
	}
	if err != nil {
		return err
	}
	if status != "active" {
		return workflowdept.ErrDeptInactive
	}
	return nil
}

func (s *Store) CodeUsable(ctx context.Context, code string) (bool, error) {
	var retired sql.NullTime
	err := s.db.QueryRowContext(ctx, `
		SELECT r.retired_at
		FROM workflow_template_departments d
		LEFT JOIN workflow_template_department_code_registry r ON r.department_code = d.department_code
		WHERE d.department_code = ?
	`, code).Scan(&retired)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return !retired.Valid, nil
}

func (s *Store) AdminRoles(ctx context.Context, companyID, membershipID string) ([]string, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM memberships m
		JOIN membership_roles mr ON mr.membership_id = m.membership_id AND mr.status = 'active'
		JOIN roles r ON r.role_id = mr.role_id AND r.status = 'active' AND r.role_code = 'admin_doanh_nghiep'
		JOIN users u ON u.user_id = m.user_id AND u.account_status = 'active'
		WHERE m.membership_id = ? AND m.company_id = ? AND m.membership_status = 'active'
	`, membershipID, companyID).Scan(&n)
	if err != nil || n == 0 {
		return nil, err
	}
	return []string{"admin_doanh_nghiep"}, nil
}

func empty(s string) string { return strings.TrimSpace(s) }

func isDup(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "1062") || strings.Contains(msg, "duplicate")
}

func newID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}
