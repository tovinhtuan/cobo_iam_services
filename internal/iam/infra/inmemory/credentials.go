package inmemory

import (
	"context"
	"net/http"
	"strings"
	"time"

	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// StaticCredentialVerifier is bootstrap-only verifier for P0.
type StaticCredentialVerifier struct {
	Users map[string]StaticUser // key: login_id
}

type StaticUser struct {
	UserID   string
	LoginID  string
	Email    string
	Password string
	FullName string
	Status   string
	EmailVerifiedAt *time.Time
}

func (v *StaticCredentialVerifier) Verify(_ context.Context, loginID, plainPassword string) (*iamapp.AuthenticatedUser, error) {
	u, ok := v.Users[strings.ToLower(strings.TrimSpace(loginID))]
	if !ok || u.Password != plainPassword {
		return nil, perr.NewHTTPError(http.StatusUnauthorized, perr.CodeInvalidCredentials, "invalid credentials", nil)
	}
	return &iamapp.AuthenticatedUser{UserID: u.UserID, LoginID: u.LoginID, FullName: u.FullName, Status: u.Status}, nil
}

func (v *StaticCredentialVerifier) GetByUserID(_ context.Context, userID string) (*iamapp.AuthenticatedUser, error) {
	for _, u := range v.Users {
		if u.UserID == userID {
			return &iamapp.AuthenticatedUser{
				UserID:   u.UserID,
				LoginID:  u.LoginID,
				FullName: u.FullName,
				Status:   u.Status,
			}, nil
		}
	}
	return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeMembershipNotFound, "user not found", nil)
}

func (v *StaticCredentialVerifier) FindUserByEmail(_ context.Context, email string) (*iamapp.RecoveryUser, error) {
	key := strings.TrimSpace(strings.ToLower(email))
	for _, u := range v.Users {
		if strings.TrimSpace(strings.ToLower(u.Email)) == key || strings.TrimSpace(strings.ToLower(u.LoginID)) == key {
			return &iamapp.RecoveryUser{
				UserID: u.UserID, Email: coalesce(u.Email, u.LoginID), FullName: u.FullName, LoginID: u.LoginID,
			}, nil
		}
	}
	return nil, nil
}

func (v *StaticCredentialVerifier) StorePasswordResetToken(_ context.Context, _ iamapp.RecoveryTokenRecord) error {
	return nil
}

func (v *StaticCredentialVerifier) ConsumePasswordResetToken(_ context.Context, tokenHash string, _ time.Time) (string, error) {
	if strings.TrimSpace(tokenHash) == "" {
		return "", nil
	}
	return "", nil
}

func (v *StaticCredentialVerifier) StoreEmailVerificationToken(_ context.Context, _ iamapp.RecoveryTokenRecord) error {
	return nil
}

func (v *StaticCredentialVerifier) ConsumeEmailVerificationToken(_ context.Context, tokenHash string, _ time.Time) (string, error) {
	if strings.TrimSpace(tokenHash) == "" {
		return "", nil
	}
	return "", nil
}

func (v *StaticCredentialVerifier) UpdatePasswordHash(_ context.Context, userID string, passwordHash string, _ time.Time) error {
	for k, u := range v.Users {
		if u.UserID == userID {
			u.Password = passwordHash
			v.Users[k] = u
			return nil
		}
	}
	return nil
}

func (v *StaticCredentialVerifier) MarkEmailVerified(_ context.Context, userID string, verifiedAt time.Time) error {
	for k, u := range v.Users {
		if u.UserID == userID {
			t := verifiedAt
			u.EmailVerifiedAt = &t
			v.Users[k] = u
			return nil
		}
	}
	return nil
}

func coalesce(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}
