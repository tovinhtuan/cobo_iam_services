package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// UserInvitationStore implements iamapp.UserInvitationExecutor against MySQL.
type UserInvitationStore struct {
	DB *sql.DB
}

func (s *UserInvitationStore) PeekUserInvitation(ctx context.Context, rawToken string, now time.Time) (valid bool, reason string, emailHint string, expiresAtUTC time.Time, err error) {
	if s == nil || s.DB == nil {
		return false, "UNAVAILABLE", "", time.Time{}, nil
	}
	return PeekUserInvitation(ctx, s.DB, rawToken, now)
}

func (s *UserInvitationStore) AcceptUserInvitation(ctx context.Context, rawToken string, bcryptPasswordHash string, now time.Time) (string, string, error) {
	if s == nil || s.DB == nil {
		return "", "", fmt.Errorf("invitation store not configured")
	}
	return AcceptUserInvitation(ctx, s.DB, rawToken, bcryptPasswordHash, now)
}
