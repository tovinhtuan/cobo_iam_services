package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	iamregmysql "github.com/cobo/cobo_iam_services/internal/iam/registrationmysql"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// Same expressions as GetCompanyPlatform so CompanyRevision matches the detail read.
const companySnapshotColumns = `company_id, company_code, company_name, status, verification_status,
	COALESCE(tax_code, ''), COALESCE(registration_number, ''), COALESCE(address, ''), COALESCE(phone, ''),
	COALESCE(contact_email, ''), COALESCE(representative_name, ''), updated_at`

func duplicateTaxCodeError(err error) error {
	return &perr.HTTPError{
		Code: perr.CodeStateConflict, Message: "duplicate tax code", HTTPStatus: http.StatusConflict,
		Details: map[string]any{"field_errors": map[string]any{"tax_code": "Mã số thuế đã được dùng cho công ty khác."}},
		Cause:   err,
	}
}

// MutateCompanyPlatform locks the company row (FOR UPDATE), lets fn decide the mutation from the
// current snapshot, then writes the update, the status and the audit rows in one transaction.
func (r *AdminRepository) MutateCompanyPlatform(ctx context.Context, companyID string, actor caapp.CompanyAuditContext, fn func(caapp.CompanySnapshot) (*caapp.CompanyMutation, error)) error {
	companyID = strings.TrimSpace(companyID)
	if companyID == "" {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "company_id is required", nil)
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var snap caapp.CompanySnapshot
	err = tx.QueryRowContext(ctx, `SELECT `+companySnapshotColumns+` FROM companies WHERE company_id = ? FOR UPDATE`, companyID).Scan(
		&snap.CompanyID, &snap.CompanyCode, &snap.CompanyName, &snap.Status, &snap.VerificationStatus,
		&snap.TaxCode, &snap.RegistrationNumber, &snap.Address, &snap.Phone,
		&snap.ContactEmail, &snap.RepresentativeName, &snap.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "company not found", nil)
	}
	if err != nil {
		return mapMySQLSchemaErr(err)
	}
	snap.UpdatedAt = snap.UpdatedAt.UTC()

	mutation, err := fn(snap)
	if err != nil {
		return err
	}
	if mutation == nil {
		return tx.Commit()
	}
	if mutation.Update != nil {
		sets, args, err := companyUpdateSets(*mutation.Update)
		if err != nil {
			return err
		}
		if len(sets) > 0 {
			args = append(args, companyID)
			if _, err := tx.ExecContext(ctx, "UPDATE companies SET "+strings.Join(sets, ", ")+" WHERE company_id = ?", args...); err != nil {
				if isMySQLDuplicate(err) {
					return duplicateTaxCodeError(err)
				}
				return mapMySQLSchemaErr(err)
			}
		}
	}
	if mutation.Status != "" {
		if _, err := tx.ExecContext(ctx, `UPDATE companies SET status = ? WHERE company_id = ?`, mutation.Status, companyID); err != nil {
			return mapMySQLSchemaErr(err)
		}
	}
	for _, rec := range mutation.Audit {
		if err := insertCompanyAuditTx(ctx, tx, companyID, actor, rec); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// CreateStandaloneCompanyAudited creates the company (+ default tenant roles) and its audit row
// in one transaction.
func (r *AdminRepository) CreateStandaloneCompanyAudited(ctx context.Context, displayName string, boot caapp.CreateCompanyBootstrap, actor caapp.CompanyAuditContext, audit func(companyID, companyCode string) caapp.CompanyAuditRecord) (string, string, error) {
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		return "", "", perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "company_name is required", nil)
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return "", "", err
	}
	defer func() { _ = tx.Rollback() }()

	companyID := uuid.NewString()
	code, _, err := iamregmysql.InsertCompanyWithDefaultRolesTx(ctx, tx, companyID, displayName, iamregmysql.CompanyBootstrapProfile{
		VerificationStatus: boot.VerificationStatus,
		TaxCode:            boot.TaxCode,
		RegistrationNumber: boot.RegistrationNumber,
		Address:            boot.Address,
		Phone:              boot.Phone,
		ContactEmail:       boot.ContactEmail,
		RepresentativeName: boot.RepresentativeName,
	})
	if err != nil {
		if iamregmysql.IsDuplicateCompanyError(err) {
			return "", "", duplicateTaxCodeError(err)
		}
		return "", "", mapMySQLSchemaErr(err)
	}
	if audit != nil {
		if err := insertCompanyAuditTx(ctx, tx, companyID, actor, audit(companyID, code)); err != nil {
			return "", "", err
		}
	}
	if err := tx.Commit(); err != nil {
		return "", "", err
	}
	return companyID, code, nil
}

// ListCompanyPrimaryAdminEmails returns emails of the company's active primary admins.
func (r *AdminRepository) ListCompanyPrimaryAdminEmails(ctx context.Context, companyID string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT TRIM(COALESCE(NULLIF(TRIM(u.email), ''), u.login_id))
		FROM memberships m
		JOIN users u ON u.user_id = m.user_id
		WHERE m.company_id = ?
		  AND LOWER(TRIM(m.membership_status)) = 'active'
		  AND m.is_primary_admin = TRUE
		  AND u.account_status = 'active'`, strings.TrimSpace(companyID))
	if err != nil {
		return nil, mapMySQLSchemaErr(err)
	}
	defer rows.Close()
	out := make([]string, 0)
	for rows.Next() {
		var email string
		if err := rows.Scan(&email); err != nil {
			return nil, err
		}
		if strings.Contains(email, "@") {
			out = append(out, email)
		}
	}
	return out, rows.Err()
}

func insertCompanyAuditTx(ctx context.Context, tx *sql.Tx, companyID string, actor caapp.CompanyAuditContext, rec caapp.CompanyAuditRecord) error {
	meta, err := json.Marshal(rec.Metadata)
	if err != nil {
		return fmt.Errorf("company audit metadata: %w", err)
	}
	occurred := actor.OccurredAt
	if occurred.IsZero() {
		occurred = time.Now()
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO audit_logs (
			event_id, occurred_at, actor_user_id, actor_membership_id, company_id,
			action, resource_type, resource_id, decision, request_id, metadata_json
		) VALUES (?, ?, NULLIF(?, ''), NULLIF(?, ''), ?, ?, 'company', ?, 'allow', NULLIF(?, ''), ?)`,
		rec.EventID, occurred.UTC(), actor.ActorUserID, actor.ActorMembershipID, companyID,
		rec.Action, companyID, actor.RequestID, string(meta))
	if err != nil {
		return fmt.Errorf("company audit insert: %w", err)
	}
	return nil
}
