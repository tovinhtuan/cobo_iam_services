package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/refreshtoken"
)

// SessionRepository persists sessions using SHA256(refresh_token) for lookup.
type SessionRepository struct {
	db          *sql.DB
	refreshTTL  time.Duration
	now         func() time.Time
}

func NewSessionRepository(db *sql.DB, refreshTTL time.Duration) *SessionRepository {
	if refreshTTL <= 0 {
		refreshTTL = 720 * time.Hour // 30 days
	}
	return &SessionRepository{db: db, refreshTTL: refreshTTL, now: time.Now}
}

func (r *SessionRepository) Create(ctx context.Context, p iamapp.CreateSessionParams) error {
	h := refreshtoken.Hash(p.RefreshToken)
	exp := r.now().Add(r.refreshTTL)
	var cc, mc, ip, ua any
	if p.CompanyID != "" {
		cc = p.CompanyID
	}
	if p.MembershipID != "" {
		mc = p.MembershipID
	}
	if p.IP != "" {
		ip = p.IP
	}
	if p.UserAgent != "" {
		ua = p.UserAgent
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO sessions (
			session_id, user_id, current_company_id, current_membership_id,
			refresh_token_hash, refresh_expires_at, ip, user_agent
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, p.SessionID, p.UserID, cc, mc, h, exp, ip, ua)
	if err != nil {
		return fmt.Errorf("session insert: %w", err)
	}
	return nil
}

func (r *SessionRepository) FindByRefreshToken(ctx context.Context, refreshToken string) (*iamapp.SessionState, error) {
	h := refreshtoken.Hash(refreshToken)
	row := r.db.QueryRowContext(ctx, `
		SELECT session_id, user_id,
			IFNULL(current_membership_id, ''), IFNULL(current_company_id, ''),
			refresh_expires_at, revoked_at
		FROM sessions WHERE refresh_token_hash = ?
	`, h)
	var sid, uid, mid, cid string
	var exp sql.NullTime
	var revoked sql.NullTime
	if err := row.Scan(&sid, &uid, &mid, &cid, &exp, &revoked); err != nil {
		if err == sql.ErrNoRows {
			return nil, perr.NewHTTPError(401, perr.CodeSessionExpired, "session expired", nil)
		}
		return nil, err
	}
	if revoked.Valid {
		return nil, perr.NewHTTPError(401, perr.CodeSessionExpired, "session expired", nil)
	}
	if exp.Valid && r.now().After(exp.Time) {
		return nil, perr.NewHTTPError(401, perr.CodeSessionExpired, "session expired", nil)
	}
	return &iamapp.SessionState{
		SessionID:    sid,
		UserID:       uid,
		MembershipID: mid,
		CompanyID:    cid,
		RefreshToken: refreshToken,
		Revoked:      false,
	}, nil
}

func (r *SessionRepository) RevokeByRefreshToken(ctx context.Context, refreshToken string) error {
	h := refreshtoken.Hash(refreshToken)
	res, err := r.db.ExecContext(ctx, `
		UPDATE sessions SET revoked_at = ?, revoked_reason = 'logout'
		WHERE refresh_token_hash = ? AND revoked_at IS NULL
	`, r.now(), h)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return perr.NewHTTPError(401, perr.CodeSessionExpired, "session expired", nil)
	}
	return nil
}

func (r *SessionRepository) UpdateContext(ctx context.Context, sessionID, membershipID, companyID string) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE sessions SET current_membership_id = ?, current_company_id = ?
		WHERE session_id = ? AND revoked_at IS NULL
	`, membershipID, companyID, sessionID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return perr.NewHTTPError(401, perr.CodeSessionExpired, "session expired", nil)
	}
	return nil
}

func (r *SessionRepository) RotateRefreshToken(ctx context.Context, sessionID, newRefreshToken string) error {
	h := refreshtoken.Hash(newRefreshToken)
	exp := r.now().Add(r.refreshTTL)
	res, err := r.db.ExecContext(ctx, `
		UPDATE sessions SET refresh_token_hash = ?, refresh_expires_at = ?
		WHERE session_id = ? AND revoked_at IS NULL
	`, h, exp, sessionID)
	if err != nil {
		return fmt.Errorf("session rotate refresh: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return perr.NewHTTPError(401, perr.CodeSessionExpired, "session expired", nil)
	}
	return nil
}
