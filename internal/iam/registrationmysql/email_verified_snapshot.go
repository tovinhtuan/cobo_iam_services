package registrationmysql

import (
	"context"
	"database/sql"
)

// EmailVerifiedSnapshot reports whether users.email_verified_at is set for the user.
func EmailVerifiedSnapshot(ctx context.Context, db *sql.DB, userID string) (bool, error) {
	var ev int
	err := db.QueryRowContext(ctx, `
		SELECT email_verified_at IS NOT NULL FROM users WHERE user_id = ? LIMIT 1
	`, userID).Scan(&ev)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return ev != 0, nil
}
