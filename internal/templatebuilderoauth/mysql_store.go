package templatebuilderoauth

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
)

// MySQLStore persists only short-lived, hashed authorization codes. Access
// tokens remain signed, short-lived bearer credentials and are never stored.
type MySQLStore struct{ db *sql.DB }

func NewMySQLStore(db *sql.DB) *MySQLStore { return &MySQLStore{db: db} }

func (s *MySQLStore) CreateAuthorizationCode(ctx context.Context, code AuthorizationCode) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO template_builder_oauth_authorization_codes
		(code_id, code_hash, client_id, redirect_uri, scope, code_challenge, user_id, membership_id, company_id, expires_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		uuid.NewString(), code.CodeHash, code.ClientID, code.RedirectURI, code.Scope, code.CodeChallenge,
		code.Subject.UserID, code.Subject.MembershipID, code.Subject.CompanyID, code.ExpiresAt.UTC(), code.CreatedAt.UTC())
	return err
}

func (s *MySQLStore) ConsumeAuthorizationCode(ctx context.Context, codeHash string, now time.Time) (*AuthorizationCode, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var out AuthorizationCode
	var consumedAt sql.NullTime
	err = tx.QueryRowContext(ctx, `
		SELECT code_hash, client_id, redirect_uri, scope, code_challenge, user_id, membership_id, company_id, expires_at, created_at, consumed_at
		FROM template_builder_oauth_authorization_codes WHERE code_hash = ? FOR UPDATE`, codeHash).Scan(
		&out.CodeHash, &out.ClientID, &out.RedirectURI, &out.Scope, &out.CodeChallenge,
		&out.Subject.UserID, &out.Subject.MembershipID, &out.Subject.CompanyID, &out.ExpiresAt, &out.CreatedAt, &consumedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if consumedAt.Valid || !out.ExpiresAt.After(now) {
		return nil, nil
	}
	if _, err := tx.ExecContext(ctx, `UPDATE template_builder_oauth_authorization_codes SET consumed_at = ? WHERE code_hash = ? AND consumed_at IS NULL`, now.UTC(), codeHash); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &out, nil
}
