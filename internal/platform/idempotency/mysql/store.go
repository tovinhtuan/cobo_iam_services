package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cobo/cobo_iam_services/internal/platform/idempotency"
	"github.com/google/uuid"
)

// Store implements idempotency.Store using idempotency_keys (0001).
type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) TryReserve(ctx context.Context, p idempotency.Params) (idempotency.Result, error) {
	var out idempotency.Result
	scope := strings.TrimSpace(p.Scope)
	key := strings.TrimSpace(p.Key)
	if scope == "" || key == "" {
		return out, fmt.Errorf("idempotency: scope and key required")
	}
	if len(key) > 191 {
		key = key[:191]
	}
	id := uuid.NewString()
	var companyID interface{}
	if strings.TrimSpace(p.CompanyID) != "" {
		companyID = strings.TrimSpace(p.CompanyID)
	}
	expires := time.Now().UTC().Add(24 * time.Hour)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO idempotency_keys (
			idempotency_key_id, company_id, scope, idempotency_key, request_hash, status, expires_at
		) VALUES (?, ?, ?, ?, ?, 'in_progress', ?)
	`, id, companyID, scope, key, nullOrString(p.RequestHash), expires)
	if err == nil {
		out.ReservationID = id
		return out, nil
	}
	if !isDuplicateKey(err) {
		return out, fmt.Errorf("idempotency insert: %w", err)
	}
	return s.loadAfterConflict(ctx, scope, key, p.RequestHash)
}

func (s *Store) loadAfterConflict(ctx context.Context, scope, key, requestHash string) (idempotency.Result, error) {
	var out idempotency.Result
	var rowID, status, storedHash sql.NullString
	var resp sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT idempotency_key_id, status, request_hash, response_json
		FROM idempotency_keys
		WHERE scope = ? AND idempotency_key = ?
	`, scope, key).Scan(&rowID, &status, &storedHash, &resp)
	if err != nil {
		if err == sql.ErrNoRows {
			return out, fmt.Errorf("idempotency: row missing after duplicate")
		}
		return out, err
	}
	switch status.String {
	case "completed":
		if !storedHash.Valid || storedHash.String != requestHash {
			out.Conflict = true
			return out, nil
		}
		if !resp.Valid {
			out.Conflict = true
			return out, nil
		}
		var env idempotency.Envelope
		if err := json.Unmarshal([]byte(resp.String), &env); err != nil {
			return out, fmt.Errorf("idempotency decode envelope: %w", err)
		}
		out.Replay = true
		out.ReplayHTTPStatus = env.HTTPStatus
		if env.HTTPStatus == 0 {
			out.ReplayHTTPStatus = 200
		}
		out.ReplayBody = env.Body
		return out, nil
	case "in_progress":
		out.Conflict = true
		return out, nil
	default:
		out.Conflict = true
		return out, nil
	}
}

func (s *Store) Complete(ctx context.Context, reservationID string, responseEnvelopeJSON []byte) error {
	if reservationID == "" {
		return nil
	}
	res, err := s.db.ExecContext(ctx, `
		UPDATE idempotency_keys
		SET status = 'completed', response_json = ?
		WHERE idempotency_key_id = ? AND status = 'in_progress'
	`, responseEnvelopeJSON, reservationID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("idempotency: complete affected 0 rows")
	}
	return nil
}

func (s *Store) Abandon(ctx context.Context, reservationID string) error {
	if reservationID == "" {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM idempotency_keys WHERE idempotency_key_id = ?`, reservationID)
	return err
}

func nullOrString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func isDuplicateKey(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate") || strings.Contains(msg, "1062")
}
