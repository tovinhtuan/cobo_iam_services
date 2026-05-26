package mysql

import (
	"context"
	"database/sql"
	"net/http"
	"strings"
	"time"

	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"golang.org/x/crypto/bcrypt"
)

// CredentialVerifier loads password credentials from MySQL (users + credentials).
type CredentialVerifier struct {
	db *sql.DB
}

func NewCredentialVerifier(db *sql.DB) *CredentialVerifier {
	return &CredentialVerifier{db: db}
}

func (v *CredentialVerifier) Verify(ctx context.Context, loginID, plainPassword string) (*iamapp.AuthenticatedUser, error) {
	loginID = strings.TrimSpace(strings.ToLower(loginID))
	if loginID == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "login_id required", nil)
	}
	row := v.db.QueryRowContext(ctx, `
		SELECT u.user_id, u.login_id, u.full_name, u.account_status, c.password_hash
		FROM users u
		INNER JOIN credentials c ON c.user_id = u.user_id
			AND c.credential_type = 'password' AND c.status = 'active'
		WHERE LOWER(TRIM(u.login_id)) = ?
	`, loginID)
	var userID, lid, fullName, status string
	var hash []byte
	if err := row.Scan(&userID, &lid, &fullName, &status, &hash); err != nil {
		if err == sql.ErrNoRows {
			return nil, perr.NewHTTPError(http.StatusUnauthorized, perr.CodeInvalidCredentials, "invalid credentials", nil)
		}
		return nil, err
	}
	if err := bcrypt.CompareHashAndPassword(hash, []byte(plainPassword)); err != nil {
		return nil, perr.NewHTTPError(http.StatusUnauthorized, perr.CodeInvalidCredentials, "invalid credentials", nil)
	}
	sub := loadUserSubscription(ctx, v.db, userID, time.Now().UTC())
	return &iamapp.AuthenticatedUser{
		UserID:                userID,
		LoginID:               lid,
		FullName:              fullName,
		Status:                status,
		SubscriptionTier:      sub.Tier,
		SubscriptionExpiresAt: sub.ExpiresAt,
	}, nil
}

func (v *CredentialVerifier) VerifyPasswordForUser(ctx context.Context, userID, plainPassword string) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "user_id required", nil)
	}
	row := v.db.QueryRowContext(ctx, `
		SELECT c.password_hash
		FROM credentials c
		WHERE c.user_id = ? AND c.credential_type = 'password' AND c.status = 'active'
	`, userID)
	var hash []byte
	if err := row.Scan(&hash); err != nil {
		if err == sql.ErrNoRows {
			return perr.NewHTTPError(http.StatusUnauthorized, perr.CodeInvalidCredentials, "invalid credentials", nil)
		}
		return err
	}
	if err := bcrypt.CompareHashAndPassword(hash, []byte(plainPassword)); err != nil {
		return perr.NewHTTPError(http.StatusUnauthorized, perr.CodeInvalidCredentials, "invalid credentials", nil)
	}
	return nil
}

func (v *CredentialVerifier) GetByUserID(ctx context.Context, userID string) (*iamapp.AuthenticatedUser, error) {
	row := v.db.QueryRowContext(ctx, `
		SELECT user_id, login_id, full_name, account_status FROM users WHERE user_id = ?
	`, userID)
	var u iamapp.AuthenticatedUser
	if err := row.Scan(&u.UserID, &u.LoginID, &u.FullName, &u.Status); err != nil {
		if err == sql.ErrNoRows {
			return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeMembershipNotFound, "user not found", nil)
		}
		return nil, err
	}
	sub := loadUserSubscription(ctx, v.db, u.UserID, time.Now().UTC())
	u.SubscriptionTier = sub.Tier
	u.SubscriptionExpiresAt = sub.ExpiresAt
	return &u, nil
}

type userSubscription struct {
	Tier      string
	ExpiresAt *time.Time
}

func loadUserSubscription(ctx context.Context, db *sql.DB, userID string, now time.Time) userSubscription {
	row := db.QueryRowContext(ctx, `
		SELECT subscription_tier, effective_to
		FROM user_subscription_tiers
		WHERE user_id = ?
			AND (effective_from IS NULL OR effective_from <= ?)
			AND (effective_to IS NULL OR effective_to > ?)
		LIMIT 1
	`, userID, now, now)

	var tier string
	var effectiveTo sql.NullTime
	if err := row.Scan(&tier, &effectiveTo); err != nil {
		return userSubscription{Tier: "Free"}
	}
	tier = strings.TrimSpace(tier)
	if tier == "" {
		tier = "Free"
	}
	var expiresAt *time.Time
	if effectiveTo.Valid {
		t := effectiveTo.Time.UTC()
		expiresAt = &t
	}
	return userSubscription{Tier: tier, ExpiresAt: expiresAt}
}
