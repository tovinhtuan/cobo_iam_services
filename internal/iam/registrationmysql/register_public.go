// Package registrationmysql holds IAM flows that must not live under iam/infra/mysql to avoid import cycles with iam/app.
package registrationmysql

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/google/uuid"
)

// SelfRegOwnerRoleID matches migrations/0026_self_registration_owner_role.up.sql (global role).
const SelfRegOwnerRoleID = "r0000000-0001-4000-8000-000099999001"

// Seed template role_permission rows copied from migrations/0009_seed_authz_test_accounts.up.sql (c_001).
// If these rows are absent (minimal DB), new company roles are created without permissions until templates exist.
const (
	seedTplAdminDoanhNghiepRoleID = "r0000001-0001-4000-8000-000000000012"
	seedTplUserThuongRoleID       = "r0000001-0001-4000-8000-000000000015"
)

var nonCompanyCodeChars = regexp.MustCompile(`[^a-z0-9]+`)

type rowQuerier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// CompanyBootstrapProfile configures optional legal/contact fields and verification for a new company row (migration 0029).
type CompanyBootstrapProfile struct {
	VerificationStatus string
	TaxCode            string
	RegistrationNumber string
	Address            string
	Phone              string
	ContactEmail       string
	RepresentativeName string
}

// InsertCompanyWithDefaultRolesTx inserts companies + tenant default roles (admin_doanh_nghiep, user_thuong) and copies role_permissions from seed templates. Runs inside an open transaction.
func InsertCompanyWithDefaultRolesTx(ctx context.Context, tx *sql.Tx, companyID, companyDisplayName string, profile CompanyBootstrapProfile) (companyCode string, err error) {
	companyDisplayName = strings.TrimSpace(companyDisplayName)
	if companyID == "" || companyDisplayName == "" {
		return "", perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "company_id and company display name are required", nil)
	}
	vStatus := strings.TrimSpace(profile.VerificationStatus)
	if vStatus == "" {
		vStatus = "unverified"
	}
	companyCode, err = pickUniqueCompanyCode(ctx, tx, companyDisplayName)
	if err != nil {
		return "", err
	}

	tax := strings.TrimSpace(profile.TaxCode)
	reg := strings.TrimSpace(profile.RegistrationNumber)
	addr := strings.TrimSpace(profile.Address)
	phone := strings.TrimSpace(profile.Phone)
	email := strings.TrimSpace(profile.ContactEmail)
	rep := strings.TrimSpace(profile.RepresentativeName)

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO companies (
			company_id, company_code, company_name,
			tax_code, registration_number, address, phone, contact_email, representative_name,
			status, verification_status
		)
		VALUES (
			?, ?, ?,
			NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''),
			'active', ?
		)
	`, companyID, companyCode, companyDisplayName,
		tax, reg, addr, phone, email, rep,
		vStatus,
	); err != nil {
		if isMySQLDuplicateCompany(err) {
			return "", perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "company tax code or unique field already exists", err)
		}
		return "", fmt.Errorf("insert company: %w", err)
	}

	roleAdminID := uuid.NewString()
	roleUserThuongID := uuid.NewString()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO roles (role_id, company_id, role_code, role_name, status) VALUES
		(?, ?, 'admin_doanh_nghiep', 'Admin doanh nghiệp', 'active'),
		(?, ?, 'user_thuong', 'User thường', 'active')
	`, roleAdminID, companyID, roleUserThuongID, companyID); err != nil {
		return "", fmt.Errorf("insert company default roles: %w", err)
	}
	for _, tpl := range []struct{ dst, src string }{
		{dst: roleAdminID, src: seedTplAdminDoanhNghiepRoleID},
		{dst: roleUserThuongID, src: seedTplUserThuongRoleID},
	} {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO role_permissions (role_id, permission_id, status)
			SELECT ?, permission_id, status
			FROM role_permissions
			WHERE role_id = ? AND status = 'active'
		`, tpl.dst, tpl.src); err != nil {
			return "", fmt.Errorf("copy seed role_permissions: %w", err)
		}
	}
	return companyCode, nil
}

// CreateStandaloneCompany creates only a company row and default tenant roles (no users / memberships).
// profile.VerificationStatus should be "verified" for CMS-provisioned tenants (see admin CreateCompany).
func CreateStandaloneCompany(ctx context.Context, db *sql.DB, companyDisplayName string, profile CompanyBootstrapProfile) (companyID, companyCode string, err error) {
	companyDisplayName = strings.TrimSpace(companyDisplayName)
	if companyDisplayName == "" {
		return "", "", perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "company_name is required", nil)
	}

	companyID = uuid.NewString()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return "", "", err
	}
	defer func() { _ = tx.Rollback() }()

	cc, err := InsertCompanyWithDefaultRolesTx(ctx, tx, companyID, companyDisplayName, profile)
	if err != nil {
		return "", "", err
	}
	if err := tx.Commit(); err != nil {
		return "", "", err
	}
	return companyID, cc, nil
}

func isMySQLDuplicateCompany(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate") && (strings.Contains(msg, "tax_code") || strings.Contains(msg, "uk_companies"))
}

// RegisterPublicAccount creates an active user, active company, membership, and assigns the global self-reg owner role.
func RegisterPublicAccount(ctx context.Context, db *sql.DB, email, fullName, companyName, passwordHash string) (userID, companyID, membershipID string, err error) {
	email = strings.TrimSpace(strings.ToLower(email))
	fullName = strings.TrimSpace(fullName)
	companyName = strings.TrimSpace(companyName)
	if email == "" || fullName == "" || companyName == "" || passwordHash == "" {
		return "", "", "", perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "email, full_name, company_name and password are required", nil)
	}

	var exists int
	err = db.QueryRowContext(ctx, `SELECT 1 FROM users WHERE LOWER(TRIM(login_id)) = ? LIMIT 1`, email).Scan(&exists)
	if err == nil {
		return "", "", "", perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "email already registered", nil)
	}
	if err != sql.ErrNoRows {
		return "", "", "", err
	}

	userID = uuid.NewString()
	companyID = uuid.NewString()
	membershipID = uuid.NewString()
	credID := uuid.NewString()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return "", "", "", err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := InsertCompanyWithDefaultRolesTx(ctx, tx, companyID, companyName, CompanyBootstrapProfile{VerificationStatus: "unverified"}); err != nil {
		return "", "", "", err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO users (user_id, login_id, full_name, email, phone, account_status)
		VALUES (?, ?, ?, ?, NULL, 'active')
	`, userID, email, fullName, email); err != nil {
		return "", "", "", fmt.Errorf("insert user: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO credentials (credential_id, user_id, credential_type, password_hash, password_algo, status, password_changed_at)
		VALUES (?, ?, 'password', ?, 'bcrypt', 'active', CURRENT_TIMESTAMP)
	`, credID, userID, passwordHash); err != nil {
		return "", "", "", fmt.Errorf("insert credential: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO user_subscription_tiers (user_id, subscription_tier, source, effective_from, effective_to)
		VALUES (?, 'Free', 'public_registration', NULL, NULL)
	`, userID); err != nil {
		return "", "", "", fmt.Errorf("insert subscription tier: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO memberships (membership_id, user_id, company_id, membership_status)
		VALUES (?, ?, ?, 'active')
	`, membershipID, userID, companyID); err != nil {
		return "", "", "", fmt.Errorf("insert membership: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO membership_roles (membership_id, role_id, status)
		VALUES (?, ?, 'active')
	`, membershipID, SelfRegOwnerRoleID); err != nil {
		return "", "", "", fmt.Errorf("insert membership role: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return "", "", "", err
	}
	return userID, companyID, membershipID, nil
}

func pickUniqueCompanyCode(ctx context.Context, q rowQuerier, companyName string) (string, error) {
	base := nonCompanyCodeChars.ReplaceAllString(strings.ToLower(strings.TrimSpace(companyName)), "_")
	base = strings.Trim(base, "_")
	if base == "" {
		base = "company"
	}
	if len(base) > 40 {
		base = base[:40]
	}
	for i := 0; i < 8; i++ {
		suffix := randomHex4()
		code := fmt.Sprintf("%s_%s", base, suffix)
		if len(code) > 64 {
			code = code[:64]
		}
		var x string
		err := q.QueryRowContext(ctx, `SELECT company_id FROM companies WHERE company_code = ? LIMIT 1`, code).Scan(&x)
		if err == sql.ErrNoRows {
			return code, nil
		}
		if err != nil {
			return "", err
		}
	}
	return "", perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "could not allocate unique company_code", nil)
}

func randomHex4() string {
	var b [2]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "0000"
	}
	return hex.EncodeToString(b[:])
}
