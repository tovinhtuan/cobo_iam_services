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

func (r *AuthRecoveryRepository) FindUserByEmail(ctx context.Context, email string) (*iamapp.RecoveryUser, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	row := r.db.QueryRowContext(ctx, `
		SELECT user_id, IFNULL(email, ''), full_name, login_id
		FROM users
		WHERE LOWER(TRIM(email)) = ? OR LOWER(TRIM(login_id)) = ?
		LIMIT 1
	`, email, email)
	var u iamapp.RecoveryUser
	if err := row.Scan(&u.UserID, &u.Email, &u.FullName, &u.LoginID); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("find user by email: %w", err)
	}
	if strings.TrimSpace(u.Email) == "" {
		u.Email = u.LoginID
	}
	return &u, nil
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

func (r *AuthRecoveryRepository) InvalidatePendingEmailVerificationOTPs(ctx context.Context, userID string) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM email_verification_otps WHERE user_id = ? AND consumed_at IS NULL
	`, userID)
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
