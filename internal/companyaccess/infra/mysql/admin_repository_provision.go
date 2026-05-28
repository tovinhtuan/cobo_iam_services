package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strings"

	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	iamregmysql "github.com/cobo/cobo_iam_services/internal/iam/registrationmysql"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/google/uuid"
)

func (r *AdminRepository) GetUserProvisioningGate(ctx context.Context, userID string) (accountStatus string, emailVerified bool, err error) {
	err = r.db.QueryRowContext(ctx, `
		SELECT account_status, email_verified_at IS NOT NULL
		FROM users WHERE user_id = ? LIMIT 1
	`, userID).Scan(&accountStatus, &emailVerified)
	if err == sql.ErrNoRows {
		return "", false, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "user not found", nil)
	}
	if err != nil {
		return "", false, err
	}
	return accountStatus, emailVerified, nil
}

func (r *AdminRepository) CountEligibleMembershipsForUser(ctx context.Context, userID string) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM memberships
		WHERE user_id = ? AND membership_status IN ('active', 'invited')
	`, userID).Scan(&n)
	return n, err
}

func countSelfProvisionedCompaniesForUserTx(ctx context.Context, tx *sql.Tx, userID string) (int, error) {
	var n int
	err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM companies
		WHERE founder_user_id = ?
		  AND provisioning_source IN (?, ?)
	`, userID, caapp.ProvisioningSourceSelfServiceInitialize, caapp.ProvisioningSourceSelfServiceCreate).Scan(&n)
	return n, err
}

func countEligibleMembershipsForUserTx(ctx context.Context, tx *sql.Tx, userID string) (int, error) {
	var n int
	err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM memberships
		WHERE user_id = ? AND membership_status IN ('active', 'invited')
	`, userID).Scan(&n)
	return n, err
}

func (r *AdminRepository) BootstrapSelfServiceCompanyTx(ctx context.Context, in caapp.BootstrapSelfServiceInput) (*caapp.BootstrapSelfServiceResult, error) {
	name := strings.TrimSpace(in.CompanyName)
	if in.UserID == "" || in.MembershipID == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "user_id and membership_id required", nil)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var lockedUser string
	err = tx.QueryRowContext(ctx, `SELECT user_id FROM users WHERE user_id = ? FOR UPDATE`, in.UserID).Scan(&lockedUser)
	if err == sql.ErrNoRows {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "user not found", nil)
	}
	if err != nil {
		return nil, err
	}

	eligible, err := countEligibleMembershipsForUserTx(ctx, tx, in.UserID)
	if err != nil {
		return nil, err
	}

	switch in.Mode {
	case caapp.BootstrapModeInitialize:
		if eligible > 0 {
			return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "COMPANY_ALREADY_EXISTS", nil)
		}
	case caapp.BootstrapModeCreate:
		if eligible < 1 {
			return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "COMPANY_MEMBERSHIP_REQUIRED", nil)
		}
		if in.QuotaLimit > 0 {
			count, err := countSelfProvisionedCompaniesForUserTx(ctx, tx, in.UserID)
			if err != nil {
				return nil, err
			}
			if count >= in.QuotaLimit {
				he := perr.NewHTTPError(http.StatusPaymentRequired, perr.CodeQuotaExceeded, "subscription quota exceeded", nil)
				he.Details = map[string]any{
					"limit":   in.QuotaLimit,
					"current": count,
					"tier":    in.QuotaTier,
				}
				return nil, he
			}
		}
	default:
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid bootstrap mode", nil)
	}

	provSource := caapp.ProvisioningSourceSelfServiceInitialize
	if in.Mode == caapp.BootstrapModeCreate {
		provSource = caapp.ProvisioningSourceSelfServiceCreate
	}

	selfProvisionedAfter := 0
	if in.Mode == caapp.BootstrapModeInitialize || in.Mode == caapp.BootstrapModeCreate {
		n, err := countSelfProvisionedCompaniesForUserTx(ctx, tx, in.UserID)
		if err != nil {
			return nil, err
		}
		selfProvisionedAfter = n + 1
	}

	companyID := uuid.NewString()
	profile := iamregmysql.CompanyBootstrapProfile{
		VerificationStatus: "unverified",
		TaxCode:            strings.TrimSpace(in.TaxCode),
		RegistrationNumber: strings.TrimSpace(in.RegistrationNumber),
		Address:            strings.TrimSpace(in.Address),
		Phone:              strings.TrimSpace(in.Phone),
		ContactEmail:       strings.TrimSpace(in.ContactEmail),
		FounderUserID:      in.UserID,
		ProvisioningSource: provSource,
	}
	companyCode, tenantAdminRoleID, err := iamregmysql.InsertCompanyWithDefaultRolesTx(ctx, tx, companyID, name, profile)
	if err != nil {
		if iamregmysql.IsDuplicateCompanyError(err) {
			return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "COMPANY_ALREADY_EXISTS", nil)
		}
		return nil, mapMySQLSchemaErr(err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO memberships (membership_id, user_id, company_id, membership_status, is_primary_admin)
		VALUES (?, ?, ?, 'active', TRUE)
	`, in.MembershipID, in.UserID, companyID); err != nil {
		if isMySQLDuplicate(err) {
			return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "membership already exists for user and company", nil)
		}
		return nil, fmt.Errorf("insert membership: %w", err)
	}

	if tenantAdminRoleID != "" {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO membership_roles (membership_id, role_id, status)
			VALUES (?, ?, 'active')
			ON DUPLICATE KEY UPDATE status = 'active', updated_at = CURRENT_TIMESTAMP
		`, in.MembershipID, tenantAdminRoleID); err != nil {
			return nil, fmt.Errorf("assign tenant admin role: %w", err)
		}
	}

	for _, perm := range []string{"template.workflow.override.read"} {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO membership_direct_permissions (membership_id, company_id, permission_code, granted_by)
			VALUES (?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE revoked_at = NULL, revoked_by = NULL, granted_by = VALUES(granted_by), granted_at = CURRENT_TIMESTAMP
		`, in.MembershipID, companyID, perm, in.UserID); err != nil {
			return nil, fmt.Errorf("grant default permission %s: %w", perm, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &caapp.BootstrapSelfServiceResult{
		CompanyID:            companyID,
		CompanyCode:          companyCode,
		CompanyName:          name,
		MembershipID:         in.MembershipID,
		SelfProvisionedCount: selfProvisionedAfter,
	}, nil
}

func (r *AdminRepository) RollbackBootstrapSelfServiceCompany(ctx context.Context, companyID, userID string) error {
	companyID = strings.TrimSpace(companyID)
	userID = strings.TrimSpace(userID)
	if companyID == "" || userID == "" {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "company_id and user_id required", nil)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var founder sql.NullString
	var provSource sql.NullString
	err = tx.QueryRowContext(ctx, `
		SELECT founder_user_id, provisioning_source FROM companies WHERE company_id = ? FOR UPDATE
	`, companyID).Scan(&founder, &provSource)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	if !founder.Valid || founder.String != userID {
		return perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "rollback not allowed for company", nil)
	}
	src := strings.TrimSpace(provSource.String)
	if src != caapp.ProvisioningSourceSelfServiceInitialize && src != caapp.ProvisioningSourceSelfServiceCreate {
		return perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "rollback not allowed for provisioning source", nil)
	}

	var memCount int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM memberships WHERE company_id = ?`, companyID).Scan(&memCount); err != nil {
		return err
	}
	if memCount != 1 {
		return perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "rollback not allowed: unexpected membership count", nil)
	}

	var discCount int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM disclosure_records WHERE company_id = ?`, companyID).Scan(&discCount); err != nil {
		return perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "rollback not allowed: cannot verify disclosure_records", err)
	}
	if discCount > 0 {
		return perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "rollback not allowed: company has disclosure records", nil)
	}

	for _, q := range []string{
		`DELETE FROM membership_direct_permissions WHERE company_id = ?`,
		`DELETE mr FROM membership_roles mr INNER JOIN memberships m ON m.membership_id = mr.membership_id WHERE m.company_id = ?`,
		`DELETE dm FROM department_memberships dm INNER JOIN memberships m ON m.membership_id = dm.membership_id WHERE m.company_id = ?`,
		`DELETE mt FROM membership_titles mt INNER JOIN memberships m ON m.membership_id = mt.membership_id WHERE m.company_id = ?`,
		`DELETE FROM memberships WHERE company_id = ?`,
		`DELETE rp FROM role_permissions rp INNER JOIN roles r ON r.role_id = rp.role_id WHERE r.company_id = ?`,
		`DELETE FROM roles WHERE company_id = ?`,
		`DELETE FROM companies WHERE company_id = ?`,
	} {
		if _, err := tx.ExecContext(ctx, q, companyID); err != nil {
			return fmt.Errorf("rollback bootstrap: %w", err)
		}
	}

	return tx.Commit()
}
