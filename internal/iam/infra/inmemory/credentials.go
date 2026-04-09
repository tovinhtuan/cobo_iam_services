package inmemory

import (
	"context"
	"net/http"
	"strings"

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
	Password string
	FullName string
	Status   string
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
