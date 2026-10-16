package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	"golang.org/x/crypto/bcrypt"
)

// AuthRecoveryRepository persists password reset / email verify tokens and updates identity credentials.
type AuthRecoveryRepository struct {
	db *sql.DB
}

func NewAuthRecoveryRepository(db *sql.DB) *AuthRecoveryRepository {
	return &AuthRecoveryRepository{db: db}
}

func (r *AuthRecoveryRepository) FindUserByUserID(ctx context.Context, userID string) (*iamapp.RecoveryUser, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, nil
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT user_id, IFNULL(email, ''), full_name, login_id
		FROM users
		WHERE user_id = ?
		LIMIT 1
	`, userID)
	var u iamapp.RecoveryUser
	if err := row.Scan(&u.UserID, &u.Email, &u.FullName, &u.LoginID); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("find user by id: %w", err)
	}
	if strings.TrimSpace(u.Email) == "" {
		u.Email = u.LoginID
	}
	return &u, nil
}

func (r *AuthRecoveryRepository) FindRecoveryUsers(ctx context.Context, address string, limit int) ([]iamapp.RecoveryUser, error) {
	address = strings.TrimSpace(strings.ToLower(address))
	if address == "" || limit <= 0 {
		return nil, nil
	}
	// Mailbox first: the reset mail must go to the address that was typed. users.email is not unique
	// (login_id is), and `email = ? OR login_id = ? LIMIT 1` used to pick any one of the matches.
	users, err := r.queryRecoveryUsers(ctx, `
		SELECT user_id, IFNULL(email, ''), full_name, login_id
		FROM users
		WHERE LOWER(TRIM(COALESCE(NULLIF(TRIM(email), ''), login_id))) = ?
		ORDER BY created_at, user_id
		LIMIT ?
	`, address, limit)
	if err != nil || len(users) > 0 {
		return users, err
	}
	// Nobody receives mail there: the address may be a login_id whose mailbox is another email.
	return r.queryRecoveryUsers(ctx, `
		SELECT user_id, IFNULL(email, ''), full_name, login_id
		FROM users
		WHERE LOWER(TRIM(login_id)) = ?
		LIMIT 1
	`, address)
}

func (r *AuthRecoveryRepository) queryRecoveryUsers(ctx context.Context, query string, args ...any) ([]iamapp.RecoveryUser, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("find recovery users: %w", err)
	}
	defer rows.Close()
	var users []iamapp.RecoveryUser
	for rows.Next() {
		var u iamapp.RecoveryUser
		if err := rows.Scan(&u.UserID, &u.Email, &u.FullName, &u.LoginID); err != nil {
			return nil, fmt.Errorf("find recovery users: %w", err)
		}
		if strings.TrimSpace(u.Email) == "" {
			u.Email = u.LoginID
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("find recovery users: %w", err)
	}
	return users, nil
}

func (r *AuthRecoveryRepository) StorePasswordResetToken(ctx context.Context, token iamapp.RecoveryTokenRecord) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO password_reset_tokens (token_id, user_id, token_hash, expires_at)
		VALUES (?, ?, ?, ?)
	`, token.TokenID, token.UserID, token.TokenHash, token.ExpiresAt)
	if err != nil {
		return fmt.Errorf("store password reset token: %w", err)
	}
	return nil
}

func (r *AuthRecoveryRepository) ConsumePasswordResetToken(ctx context.Context, tokenHash string, now time.Time) (string, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("begin consume reset token tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	row := tx.QueryRowContext(ctx, `
		SELECT token_id, user_id
		FROM password_reset_tokens
		WHERE token_hash = ? AND used_at IS NULL AND expires_at > ?
		FOR UPDATE
	`, tokenHash, now)
	var tokenID, userID string
	if err := row.Scan(&tokenID, &userID); err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", fmt.Errorf("consume reset token scan: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE password_reset_tokens SET used_at = ? WHERE token_id = ? AND used_at IS NULL
	`, now, tokenID); err != nil {
		return "", fmt.Errorf("consume reset token update: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("consume reset token commit: %w", err)
	}
	return userID, nil
}

func (r *AuthRecoveryRepository) StoreEmailVerificationToken(ctx context.Context, token iamapp.RecoveryTokenRecord) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO email_verification_tokens (token_id, user_id, token_hash, expires_at)
		VALUES (?, ?, ?, ?)
	`, token.TokenID, token.UserID, token.TokenHash, token.ExpiresAt)
	if err != nil {
		return fmt.Errorf("store email verify token: %w", err)
	}
	return nil
}

func (r *AuthRecoveryRepository) ConsumeEmailVerificationToken(ctx context.Context, tokenHash string, now time.Time) (string, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("begin consume verify token tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	row := tx.QueryRowContext(ctx, `
		SELECT token_id, user_id
		FROM email_verification_tokens
		WHERE token_hash = ? AND used_at IS NULL AND expires_at > ?
		FOR UPDATE
	`, tokenHash, now)
	var tokenID, userID string
	if err := row.Scan(&tokenID, &userID); err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", fmt.Errorf("consume verify token scan: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE email_verification_tokens SET used_at = ? WHERE token_id = ? AND used_at IS NULL
	`, now, tokenID); err != nil {
		return "", fmt.Errorf("consume verify token update: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("consume verify token commit: %w", err)
	}
	return userID, nil
}

func (r *AuthRecoveryRepository) UpdatePasswordHash(ctx context.Context, userID string, passwordHash string, changedAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE credentials
		SET password_hash = ?, password_changed_at = ?, updated_at = CURRENT_TIMESTAMP
		WHERE user_id = ? AND credential_type = 'password'
	`, passwordHash, changedAt, userID)
	if err != nil {
		return fmt.Errorf("update password hash: %w", err)
	}
	return nil
}

func (r *AuthRecoveryRepository) MarkEmailVerified(ctx context.Context, userID string, verifiedAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE users
		SET email_verified_at = ?, updated_at = CURRENT_TIMESTAMP
		WHERE user_id = ?
	`, verifiedAt, userID)
	if err != nil {
		return fmt.Errorf("mark email verified: %w", err)
	}
	return nil
}

func (r *AuthRecoveryRepository) ActivateUserAfterEmailVerification(ctx context.Context, userID string) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin activate after verify tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `
		UPDATE users
		SET account_status = 'active', updated_at = CURRENT_TIMESTAMP
		WHERE user_id = ? AND account_status = 'pending_email_verification'
	`, userID); err != nil {
		return fmt.Errorf("activate user after verify: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE memberships
		SET membership_status = 'active', updated_at = CURRENT_TIMESTAMP
		WHERE user_id = ? AND membership_status = 'pending_verification'
	`, userID); err != nil {
		return fmt.Errorf("activate memberships after verify: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit activate after verify: %w", err)
	}
	return nil
}

func (r *AuthRecoveryRepository) IsEmailVerified(ctx context.Context, userID string) (bool, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return false, nil
	}
	var ok int
	err := r.db.QueryRowContext(ctx, `
		SELECT email_verified_at IS NOT NULL FROM users WHERE user_id = ? LIMIT 1
	`, userID).Scan(&ok)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("is email verified: %w", err)
	}
	return ok != 0, nil
}

// InvalidatePendingEmailVerificationOTPs expires the user's unconsumed OTPs. The rows are kept, not
// deleted, so CountEmailVerificationOTPsSince still counts them toward the send limit (AUTH-03).
func (r *AuthRecoveryRepository) InvalidatePendingEmailVerificationOTPs(ctx context.Context, userID string) error {
	expired := time.Now().UTC().Truncate(time.Second).Add(-time.Second)
	_, err := r.db.ExecContext(ctx, `
		UPDATE email_verification_otps SET expires_at = ?
		WHERE user_id = ? AND consumed_at IS NULL AND expires_at > ?
	`, expired, userID, expired)
	if err != nil {
		return fmt.Errorf("invalidate email otps: %w", err)
	}
	return nil
}

func (r *AuthRecoveryRepository) StoreEmailVerificationOTP(ctx context.Context, otp iamapp.EmailOTPRecord) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO email_verification_otps (otp_id, user_id, code_hash, expires_at)
		VALUES (?, ?, ?, ?)
	`, otp.OTPID, otp.UserID, otp.CodeHash, otp.ExpiresAt)
	if err != nil {
		return fmt.Errorf("store email verification otp: %w", err)
	}
	return nil
}

func (r *AuthRecoveryRepository) CountEmailVerificationOTPsSince(ctx context.Context, userID string, since time.Time) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM email_verification_otps WHERE user_id = ? AND created_at >= ?
	`, userID, since).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count email otps: %w", err)
	}
	return n, nil
}

// The service enforces the per-user send limits atomically only through this interface: keep the assertion.
var _ iamapp.LimitedRecoveryIssuer = (*AuthRecoveryRepository)(nil)

// IssueEmailVerificationOTPWithinLimits stores otp and expires the user's other pending OTPs, unless the
// user already reached a send limit (AUTH-03). Expired rows are kept so they stay counted.
func (r *AuthRecoveryRepository) IssueEmailVerificationOTPWithinLimits(ctx context.Context, otp iamapp.EmailOTPRecord, windows []iamapp.SendWindow) (bool, error) {
	return r.issueWithinLimits(ctx, otp.UserID, `
		SELECT COUNT(*) FROM email_verification_otps WHERE user_id = ? AND created_at >= ?
	`, windows, func(tx *sql.Tx) error {
		expired := time.Now().UTC().Truncate(time.Second).Add(-time.Second)
		if _, err := tx.ExecContext(ctx, `
			UPDATE email_verification_otps SET expires_at = ?
			WHERE user_id = ? AND consumed_at IS NULL AND expires_at > ?
		`, expired, otp.UserID, expired); err != nil {
			return fmt.Errorf("invalidate email otps: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO email_verification_otps (otp_id, user_id, code_hash, expires_at)
			VALUES (?, ?, ?, ?)
		`, otp.OTPID, otp.UserID, otp.CodeHash, otp.ExpiresAt); err != nil {
			return fmt.Errorf("store email verification otp: %w", err)
		}
		return nil
	})
}

// StorePasswordResetTokenWithinLimits stores token unless the user already reached a send limit (AUTH-03).
func (r *AuthRecoveryRepository) StorePasswordResetTokenWithinLimits(ctx context.Context, token iamapp.RecoveryTokenRecord, windows []iamapp.SendWindow) (bool, error) {
	return r.issueWithinLimits(ctx, token.UserID, `
		SELECT COUNT(*) FROM password_reset_tokens WHERE user_id = ? AND created_at >= ?
	`, windows, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO password_reset_tokens (token_id, user_id, token_hash, expires_at)
			VALUES (?, ?, ?, ?)
		`, token.TokenID, token.UserID, token.TokenHash, token.ExpiresAt); err != nil {
			return fmt.Errorf("store password reset token: %w", err)
		}
		return nil
	})
}

// issueWithinLimits locks the users row, so concurrent issues for one user are counted one after
// another, then runs write only if countSQL (user_id, since) stays below Max for every window.
// The lock must stay the first statement: a plain SELECT before it would take the REPEATABLE READ
// snapshot too early and miss rows committed by a competing request.
// The counts rely on old rows: any retention job must delete by created_at older than the longest
// window (24h), never by expires_at, since replaced OTPs are expired immediately.
func (r *AuthRecoveryRepository) issueWithinLimits(ctx context.Context, userID, countSQL string, windows []iamapp.SendWindow, write func(*sql.Tx) error) (bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("begin limited issue: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var locked string
	if err := tx.QueryRowContext(ctx, `SELECT user_id FROM users WHERE user_id = ? FOR UPDATE`, userID).Scan(&locked); err != nil {
		if err == sql.ErrNoRows {
			return false, fmt.Errorf("limited issue: user %q not found", userID)
		}
		return false, fmt.Errorf("lock user for limited issue: %w", err)
	}
	for _, w := range windows {
		var n int
		if err := tx.QueryRowContext(ctx, countSQL, userID, w.Since).Scan(&n); err != nil {
			return false, fmt.Errorf("count sends for limit: %w", err)
		}
		if n >= w.Max {
			return false, nil
		}
	}
	if err := write(tx); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit limited issue: %w", err)
	}
	return true, nil
}

func (r *AuthRecoveryRepository) TryConsumeEmailVerificationOTP(ctx context.Context, userID, plainCode string, now time.Time) (iamapp.EmailOTPConsumeOutcome, error) {
	userID = strings.TrimSpace(userID)
	code := strings.TrimSpace(plainCode)
	if userID == "" || code == "" {
		return iamapp.EmailOTPNotFound, nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return iamapp.EmailOTPNotFound, fmt.Errorf("begin otp consume: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	row := tx.QueryRowContext(ctx, `
		SELECT otp_id, code_hash, attempt_count, max_attempts
		FROM email_verification_otps
		WHERE user_id = ? AND consumed_at IS NULL AND expires_at > ?
		ORDER BY created_at DESC
		LIMIT 1
		FOR UPDATE
	`, userID, now)
	var otpID, codeHash string
	var attemptCount, maxAttempts int
	if err := row.Scan(&otpID, &codeHash, &attemptCount, &maxAttempts); err != nil {
		if err == sql.ErrNoRows {
			return iamapp.EmailOTPNotFound, nil
		}
		return iamapp.EmailOTPNotFound, fmt.Errorf("scan otp row: %w", err)
	}

	if attemptCount >= maxAttempts {
		return iamapp.EmailOTPExhausted, nil
	}

	if err := bcrypt.CompareHashAndPassword([]byte(codeHash), []byte(code)); err != nil {
		if _, uerr := tx.ExecContext(ctx, `
			UPDATE email_verification_otps SET attempt_count = attempt_count + 1 WHERE otp_id = ?
		`, otpID); uerr != nil {
			return iamapp.EmailOTPWrongCode, fmt.Errorf("bump otp attempts: %w", uerr)
		}
		if err := tx.Commit(); err != nil {
			return iamapp.EmailOTPWrongCode, fmt.Errorf("commit otp wrong attempt: %w", err)
		}
		return iamapp.EmailOTPWrongCode, nil
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE email_verification_otps SET consumed_at = ? WHERE otp_id = ? AND consumed_at IS NULL
	`, now, otpID); err != nil {
		return iamapp.EmailOTPNotFound, fmt.Errorf("consume otp update: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return iamapp.EmailOTPNotFound, fmt.Errorf("commit otp consume: %w", err)
	}
	return iamapp.EmailOTPConsumed, nil
}
