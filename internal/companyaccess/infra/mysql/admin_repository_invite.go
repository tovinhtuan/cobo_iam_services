package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"time"

	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

func (r *AdminRepository) LookupUserByLoginID(ctx context.Context, loginID string) (string, string, bool, error) {
	loginID = strings.TrimSpace(strings.ToLower(loginID))
	row := r.db.QueryRowContext(ctx, `SELECT user_id, account_status FROM users WHERE LOWER(TRIM(login_id)) = ? LIMIT 1`, loginID)
	var userID, status string
	if err := row.Scan(&userID, &status); err != nil {
		if err == sql.ErrNoRows {
			return "", "", false, nil
		}
		return "", "", false, fmt.Errorf("lookup user: %w", err)
	}
	return userID, status, true, nil
}

func (r *AdminRepository) GetUserProfile(ctx context.Context, userID string) (loginID, email, fullName, accountStatus string, err error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT login_id, COALESCE(email, ''), full_name, account_status FROM users WHERE user_id = ?
	`, userID)
	if err := row.Scan(&loginID, &email, &fullName, &accountStatus); err != nil {
		if err == sql.ErrNoRows {
			return "", "", "", "", perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "user not found", nil)
		}
		return "", "", "", "", err
	}
	return loginID, email, fullName, accountStatus, nil
}

func (r *AdminRepository) MembershipExistsForUserCompany(ctx context.Context, userID, companyID string) (bool, error) {
	var x string
	err := r.db.QueryRowContext(ctx, `
		SELECT membership_id FROM memberships WHERE user_id = ? AND company_id = ? LIMIT 1
	`, userID, companyID).Scan(&x)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (r *AdminRepository) GetCompanyName(ctx context.Context, companyID string) (string, error) {
	companyID = strings.TrimSpace(companyID)
	if companyID == "" {
		return "", perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "company_id is required", nil)
	}
	var name string
	err := r.db.QueryRowContext(ctx, `SELECT company_name FROM companies WHERE company_id = ?`, companyID).Scan(&name)
	if err == sql.ErrNoRows {
		return "", perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "company not found", nil)
	}
	if err != nil {
		return "", err
	}
	return name, nil
}

func (r *AdminRepository) InviteUserWithMembership(ctx context.Context, u caapp.UserView, opts caapp.CreateUserOptions, invitationID, tokenHash, createdByUserID string, expiresAt time.Time) (*caapp.UserView, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO users (user_id, login_id, full_name, email, phone, account_status)
		VALUES (?, ?, ?, NULLIF(?, ''), NULL, ?)
	`, u.UserID, u.LoginID, u.FullName, u.Email, u.AccountStatus)
	if err != nil {
		if isMySQLDuplicate(err) {
			return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "login_id already exists", nil)
		}
		return nil, fmt.Errorf("create invited user: %w", err)
	}

	var companyName string
	if err := tx.QueryRowContext(ctx, `SELECT company_name FROM companies WHERE company_id = ?`, opts.CompanyID).Scan(&companyName); err != nil {
		if err == sql.ErrNoRows {
			return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "company not found", nil)
		}
		return nil, err
	}

	membershipStatus := strings.TrimSpace(opts.MembershipStatus)
	if membershipStatus == "" {
		membershipStatus = "active"
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO memberships (membership_id, user_id, company_id, membership_status)
		VALUES (?, ?, ?, ?)
	`, opts.MembershipID, u.UserID, opts.CompanyID, membershipStatus)
	if err != nil {
		if isMySQLDuplicate(err) {
			return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "membership already exists for user and company", nil)
		}
		return nil, fmt.Errorf("create membership: %w", err)
	}

	if strings.TrimSpace(opts.InitialRoleID) != "" {
		if err := inviteAssignMembershipRoleTx(ctx, tx, opts.MembershipID, opts.InitialRoleID); err != nil {
			return nil, err
		}
	}

	createdBy := sql.NullString{String: strings.TrimSpace(createdByUserID), Valid: strings.TrimSpace(createdByUserID) != ""}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO user_invitations (invitation_id, user_id, token_hash, expires_at, created_by_user_id, send_count, last_sent_at)
		VALUES (?, ?, ?, ?, ?, 1, CURRENT_TIMESTAMP)
	`, invitationID, u.UserID, tokenHash, expiresAt.UTC(), createdBy)
	if err != nil {
		return nil, fmt.Errorf("create invitation: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	u.MembershipID = opts.MembershipID
	u.MembershipStatus = membershipStatus
	u.CompanyID = opts.CompanyID
	u.CompanyName = companyName
	return &u, nil
}

func (r *AdminRepository) ReplaceUserInvitation(ctx context.Context, userID, invitationID, tokenHash, createdByUserID string, expiresAt time.Time) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var status string
	if err := tx.QueryRowContext(ctx, `SELECT account_status FROM users WHERE user_id = ? FOR UPDATE`, userID).Scan(&status); err != nil {
		if err == sql.ErrNoRows {
			return perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "user not found", nil)
		}
		return err
	}
	if !strings.EqualFold(strings.TrimSpace(status), "invited") {
		return perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "user is not invited", nil)
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE user_invitations SET revoked_at = CURRENT_TIMESTAMP
		WHERE user_id = ? AND used_at IS NULL AND revoked_at IS NULL
	`, userID); err != nil {
		return fmt.Errorf("revoke invitations: %w", err)
	}

	createdBy := sql.NullString{String: strings.TrimSpace(createdByUserID), Valid: strings.TrimSpace(createdByUserID) != ""}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO user_invitations (invitation_id, user_id, token_hash, expires_at, created_by_user_id, send_count, last_sent_at)
		VALUES (?, ?, ?, ?, ?, 1, CURRENT_TIMESTAMP)
	`, invitationID, userID, tokenHash, expiresAt.UTC(), createdBy)
	if err != nil {
		return fmt.Errorf("insert invitation: %w", err)
	}
	return tx.Commit()
}

func inviteAssignMembershipRoleTx(ctx context.Context, tx *sql.Tx, membershipID, roleID string) error {
	roleID = strings.TrimSpace(roleID)
	if roleID == "" {
		return nil
	}
	var mCompany string
	if err := tx.QueryRowContext(ctx, `SELECT company_id FROM memberships WHERE membership_id = ?`, membershipID).Scan(&mCompany); err != nil {
		return err
	}
	var rCompany sql.NullString
	if err := tx.QueryRowContext(ctx, `SELECT company_id FROM roles WHERE role_id = ? AND status = 'active'`, roleID).Scan(&rCompany); err != nil {
		if err == sql.ErrNoRows {
			return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "role not found", nil)
		}
		return err
	}
	if rCompany.Valid && rCompany.String != "" && rCompany.String != mCompany {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "role does not belong to membership company", nil)
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO membership_roles (membership_id, role_id, status)
		VALUES (?, ?, 'active')
	`, membershipID, roleID)
	return err
}
