package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/refreshtoken"
)

// PeekUserInvitation evaluates the raw invitation token against user_invitations (no side effects).
func PeekUserInvitation(ctx context.Context, db *sql.DB, rawToken string, now time.Time) (valid bool, reason string, emailHint string, expiresAtUTC time.Time, err error) {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return false, "INVALID", "", time.Time{}, nil
	}
	tokenHash := refreshtoken.Hash(rawToken)
	row := db.QueryRowContext(ctx, `
		SELECT i.expires_at, i.used_at, i.revoked_at, COALESCE(NULLIF(TRIM(u.email), ''), u.login_id) AS mailbox
		FROM user_invitations i
		INNER JOIN users u ON u.user_id = i.user_id
		WHERE i.token_hash = ?
	`, tokenHash)
	var exp time.Time
	var usedAt, revokedAt sql.NullTime
	var mailbox string
	if err := row.Scan(&exp, &usedAt, &revokedAt, &mailbox); err != nil {
		if err == sql.ErrNoRows {
			return false, "INVALID", "", time.Time{}, nil
		}
		return false, "INVALID", "", time.Time{}, err
	}
	if revokedAt.Valid {
		return false, "REVOKED", "", exp.UTC(), nil
	}
	if usedAt.Valid {
		return false, "USED", "", exp.UTC(), nil
	}
	if !now.Before(exp.UTC()) {
		return false, "EXPIRED", "", exp.UTC(), nil
	}
	masked := maskEmail(mailbox)
	return true, "", masked, exp.UTC(), nil
}

func maskEmail(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return ""
	}
	parts := strings.SplitN(s, "@", 2)
	if len(parts) != 2 {
		return "***"
	}
	local := parts[0]
	domain := parts[1]
	show := 2
	if len(local) <= show {
		show = len(local)
	}
	prefix := local[:show]
	return prefix + "***@" + domain
}

// AcceptUserInvitation consumes invitation token (one-time), creates password credential, activates user.
// Returns (userID, loginID, companyID, membershipID) so the caller can issue a session for the inviting company.
func AcceptUserInvitation(ctx context.Context, db *sql.DB, rawToken string, bcryptPasswordHash string, now time.Time) (userID string, loginID string, companyID string, membershipID string, err error) {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return "", "", "", "", perr.NewHTTPError(http.StatusUnauthorized, perr.CodeUserInvitationTokenInvalid, "invitation token invalid or expired", nil)
	}
	tokenHash := refreshtoken.Hash(rawToken)
	tx, txErr := db.BeginTx(ctx, nil)
	if txErr != nil {
		return "", "", "", "", fmt.Errorf("begin invitation accept tx: %w", txErr)
	}
	defer func() { _ = tx.Rollback() }()

	row := tx.QueryRowContext(ctx, `
		SELECT i.invitation_id, i.user_id, u.login_id
		FROM user_invitations i
		INNER JOIN users u ON u.user_id = i.user_id
		WHERE i.token_hash = ? AND i.used_at IS NULL AND i.revoked_at IS NULL AND i.expires_at > ? AND LOWER(u.account_status) = 'invited'
		FOR UPDATE
	`, tokenHash, now)
	var invID string
	if scanErr := row.Scan(&invID, &userID, &loginID); scanErr != nil {
		if scanErr == sql.ErrNoRows {
			return "", "", "", "", perr.NewHTTPError(http.StatusUnauthorized, perr.CodeUserInvitationTokenInvalid, "invitation token invalid or expired", nil)
		}
		return "", "", "", "", fmt.Errorf("scan invitation: %w", scanErr)
	}

	if _, execErr := tx.ExecContext(ctx, `
		UPDATE user_invitations SET used_at = ? WHERE invitation_id = ? AND used_at IS NULL
	`, now, invID); execErr != nil {
		return "", "", "", "", fmt.Errorf("mark invitation used: %w", execErr)
	}

	credID := uuid.NewString()
	if _, execErr := tx.ExecContext(ctx, `
		INSERT INTO credentials (credential_id, user_id, credential_type, password_hash, password_algo, status, password_changed_at)
		VALUES (?, ?, 'password', ?, 'bcrypt', 'active', ?)
		ON DUPLICATE KEY UPDATE password_hash = VALUES(password_hash), status = 'active', password_changed_at = VALUES(password_changed_at), updated_at = CURRENT_TIMESTAMP
	`, credID, userID, bcryptPasswordHash, now); execErr != nil {
		return "", "", "", "", fmt.Errorf("upsert password credential: %w", execErr)
	}

	if _, execErr := tx.ExecContext(ctx, `
		UPDATE users SET account_status = 'active', updated_at = CURRENT_TIMESTAMP WHERE user_id = ?
	`, userID); execErr != nil {
		return "", "", "", "", fmt.Errorf("activate user: %w", execErr)
	}

	memRow := tx.QueryRowContext(ctx, `
		SELECT membership_id, company_id
		FROM memberships
		WHERE user_id = ?
		ORDER BY created_at DESC, membership_id DESC
		LIMIT 1
	`, userID)
	if scanErr := memRow.Scan(&membershipID, &companyID); scanErr != nil && scanErr != sql.ErrNoRows {
		return "", "", "", "", fmt.Errorf("resolve invited membership: %w", scanErr)
	}

	if commitErr := tx.Commit(); commitErr != nil {
		return "", "", "", "", fmt.Errorf("commit invitation accept: %w", commitErr)
	}
	return userID, loginID, companyID, membershipID, nil
}
