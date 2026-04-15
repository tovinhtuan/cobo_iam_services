package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
)

// AuthRecoveryRepository persists password reset / email verify tokens and updates identity credentials.
type AuthRecoveryRepository struct {
	db *sql.DB
}

func NewAuthRecoveryRepository(db *sql.DB) *AuthRecoveryRepository {
	return &AuthRecoveryRepository{db: db}
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
