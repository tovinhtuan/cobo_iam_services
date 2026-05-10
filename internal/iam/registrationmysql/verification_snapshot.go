package registrationmysql

import (
	"context"
	"database/sql"
)

// VerificationSnapshot returns email verified flag and company verification_status for the given user + company.
func VerificationSnapshot(ctx context.Context, db *sql.DB, userID, companyID string) (emailVerified bool, companyVerificationStatus string, err error) {
	row := db.QueryRowContext(ctx, `
		SELECT
			u.email_verified_at IS NOT NULL,
			IFNULL(c.verification_status, 'verified')
		FROM users u
		INNER JOIN companies c ON c.company_id = ?
		WHERE u.user_id = ?
		LIMIT 1
	`, companyID, userID)
	var ev int
	if err := row.Scan(&ev, &companyVerificationStatus); err != nil {
		if err == sql.ErrNoRows {
			return false, "", nil
		}
		return false, "", err
	}
	return ev != 0, companyVerificationStatus, nil
}
