package mysql

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/cobo/cobo_iam_services/internal/workflowdept"
)

// ErrLostLease means the row left SENDING or the lease changed. It is not a successful write.
var ErrLostLease = errors.New("delivery lease lost")

// DeliveryStore persists claim/reaper updates for reminder_dispatch_resolutions.
type DeliveryStore struct {
	db *sql.DB
}

func NewDeliveryStore(db *sql.DB) *DeliveryStore { return &DeliveryStore{db: db} }

// ClaimPredicateSQL is the atomic claim. next_retry_at is the due guard.
const ClaimPredicateSQL = `
UPDATE reminder_dispatch_resolutions
SET send_status = 'SENDING',
    lease_id = ?,
    lease_until = ?,
    updated_at = ?
WHERE resolution_id = ?
  AND send_status IN ('PENDING', 'RETRYABLE_FAILED')
  AND (lease_until IS NULL OR lease_until < ?)
  AND (next_retry_at IS NULL OR next_retry_at <= ?)`

const claimDueSQL = `
UPDATE reminder_dispatch_resolutions
SET send_status = 'SENDING',
    lease_id = ?,
    lease_until = ?,
    updated_at = ?
WHERE send_status IN ('PENDING', 'RETRYABLE_FAILED')
  AND (lease_until IS NULL OR lease_until < ?)
  AND (next_retry_at IS NULL OR next_retry_at <= ?)
ORDER BY resolved_at
LIMIT 1`

func (s *DeliveryStore) ReapStaleSending(ctx context.Context, now time.Time) (int64, error) {
	now = now.UTC()
	res, err := s.db.ExecContext(ctx, `
		UPDATE reminder_dispatch_resolutions
		SET send_status = 'SEND_UNKNOWN',
		    lease_id = NULL,
		    lease_until = NULL,
		    last_error_code = ?,
		    updated_at = ?
		WHERE send_status = 'SENDING'
		  AND lease_until IS NOT NULL
		  AND lease_until < ?
	`, workflowdept.ErrCodeLeaseExpired, now, now)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (s *DeliveryStore) ClaimDueResolution(ctx context.Context, leaseID string, now, leaseUntil time.Time) (workflowdept.ClaimResult, error) {
	now = now.UTC()
	leaseUntil = leaseUntil.UTC()
	res, err := s.db.ExecContext(ctx, claimDueSQL, leaseID, leaseUntil, now, now, now)
	if err != nil {
		return workflowdept.ClaimResult{}, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return workflowdept.ClaimResult{}, err
	}
	if n == 0 {
		return workflowdept.ClaimResult{Outcome: workflowdept.ClaimNotFound}, nil
	}
	row, err := s.getByLease(ctx, leaseID)
	if err != nil {
		return workflowdept.ClaimResult{}, err
	}
	return workflowdept.ClaimResult{Outcome: workflowdept.ClaimClaimed, Resolution: row}, nil
}

func (s *DeliveryStore) MarkSent(ctx context.Context, resolutionID, leaseID, providerID string, now time.Time) error {
	return s.mark(ctx, `
		UPDATE reminder_dispatch_resolutions
		SET send_status = 'SENT',
		    provider_message_id = ?,
		    lease_id = NULL,
		    lease_until = NULL,
		    updated_at = ?
		WHERE resolution_id = ?
		  AND send_status = 'SENDING'
		  AND lease_id = ?
	`, providerID, now.UTC(), resolutionID, leaseID)
}

func (s *DeliveryStore) MarkRetryableFailed(ctx context.Context, resolutionID, leaseID, errorCode string, nextRetry, now time.Time) error {
	return s.mark(ctx, `
		UPDATE reminder_dispatch_resolutions
		SET send_status = 'RETRYABLE_FAILED',
		    provider_message_id = NULL,
		    attempt_count = attempt_count + 1,
		    next_retry_at = ?,
		    last_error_code = ?,
		    lease_id = NULL,
		    lease_until = NULL,
		    updated_at = ?
		WHERE resolution_id = ?
		  AND send_status = 'SENDING'
		  AND lease_id = ?
	`, nextRetry.UTC(), errorCode, now.UTC(), resolutionID, leaseID)
}

func (s *DeliveryStore) MarkSendUnknown(ctx context.Context, resolutionID, leaseID, errorCode string, now time.Time) error {
	return s.mark(ctx, `
		UPDATE reminder_dispatch_resolutions
		SET send_status = 'SEND_UNKNOWN',
		    last_error_code = ?,
		    lease_id = NULL,
		    lease_until = NULL,
		    updated_at = ?
		WHERE resolution_id = ?
		  AND send_status = 'SENDING'
		  AND lease_id = ?
	`, errorCode, now.UTC(), resolutionID, leaseID)
}

func (s *DeliveryStore) MarkPermanentFailed(ctx context.Context, resolutionID, leaseID, errorCode string, now time.Time) error {
	return s.mark(ctx, `
		UPDATE reminder_dispatch_resolutions
		SET send_status = 'PERMANENT_FAILED',
		    provider_message_id = NULL,
		    attempt_count = attempt_count + 1,
		    next_retry_at = NULL,
		    last_error_code = ?,
		    lease_id = NULL,
		    lease_until = NULL,
		    updated_at = ?
		WHERE resolution_id = ?
		  AND send_status = 'SENDING'
		  AND lease_id = ?
	`, errorCode, now.UTC(), resolutionID, leaseID)
}

// BindingOwnsOccurrence reports whether the binding table already owns this occurrence.
// A missing table returns an error so the caller can keep the legacy sender.
func (s *DeliveryStore) BindingOwnsOccurrence(ctx context.Context, occurrenceID string) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `
		SELECT 1 FROM reminder_dispatch_resolutions WHERE occurrence_id = ? LIMIT 1
	`, occurrenceID).Scan(&n)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *DeliveryStore) GetResolutionForDelivery(ctx context.Context, resolutionID string) (workflowdept.DeliveryResolution, error) {
	row := s.db.QueryRowContext(ctx, resolutionSelect+` WHERE resolution_id = ?`, resolutionID)
	out, err := scanResolution(row)
	if errors.Is(err, sql.ErrNoRows) {
		return workflowdept.DeliveryResolution{}, workflowdept.ErrNotFound
	}
	return out, err
}

func (s *DeliveryStore) getByLease(ctx context.Context, leaseID string) (workflowdept.DeliveryResolution, error) {
	row := s.db.QueryRowContext(ctx, resolutionSelect+` WHERE lease_id = ? AND send_status = 'SENDING'`, leaseID)
	return scanResolution(row)
}

func (s *DeliveryStore) mark(ctx context.Context, query string, args ...any) error {
	res, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrLostLease
	}
	return nil
}

const resolutionSelect = `
SELECT resolution_id, occurrence_id, company_id,
       recipient_email_ciphertext, email_key_version, send_status,
       provider_message_id, attempt_count, next_retry_at, lease_id, lease_until,
       last_error_code, updated_at
FROM reminder_dispatch_resolutions`

func scanResolution(row *sql.Row) (workflowdept.DeliveryResolution, error) {
	var out workflowdept.DeliveryResolution
	var version sql.NullInt64
	var provider, lease, errCode sql.NullString
	var nextRetry, leaseUntil, updated sql.NullTime
	var ciphertext []byte
	err := row.Scan(
		&out.ResolutionID, &out.OccurrenceID, &out.CompanyID,
		&ciphertext, &version, &out.SendStatus,
		&provider, &out.AttemptCount, &nextRetry, &lease, &leaseUntil,
		&errCode, &updated,
	)
	if err != nil {
		return workflowdept.DeliveryResolution{}, err
	}
	out.Ciphertext = ciphertext
	if version.Valid && version.Int64 > 0 {
		out.KeyVersion = uint32(version.Int64)
	}
	if provider.Valid {
		out.ProviderID = provider.String
	}
	if lease.Valid {
		out.LeaseID = lease.String
	}
	if errCode.Valid {
		out.LastErrorCode = errCode.String
	}
	if nextRetry.Valid {
		t := nextRetry.Time.UTC()
		out.NextRetryAt = &t
	}
	if leaseUntil.Valid {
		t := leaseUntil.Time.UTC()
		out.LeaseUntil = &t
	}
	if updated.Valid {
		out.UpdatedAt = updated.Time.UTC()
	}
	return out, nil
}
