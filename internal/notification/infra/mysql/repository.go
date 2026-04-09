package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	notificationapp "github.com/cobo/cobo_iam_services/internal/notification/app"
)

var _ notificationapp.TxJobRepository = (*Repository)(nil)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreateJob(ctx context.Context, job notificationapp.NotificationJobDTO) (*notificationapp.NotificationJobDTO, error) {
	return r.createJob(ctx, r.db, job)
}

// CreateJobTx persists a job inside an existing transaction (transactional enqueue + outbox).
func (r *Repository) CreateJobTx(ctx context.Context, tx *sql.Tx, job notificationapp.NotificationJobDTO) (*notificationapp.NotificationJobDTO, error) {
	return r.createJob(ctx, tx, job)
}

type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func (r *Repository) createJob(ctx context.Context, ex execer, job notificationapp.NotificationJobDTO) (*notificationapp.NotificationJobDTO, error) {
	payload, err := json.Marshal(job.Payload)
	if err != nil {
		return nil, fmt.Errorf("marshal notification payload: %w", err)
	}
	if len(payload) == 0 {
		payload = []byte("{}")
	}
	_, err = ex.ExecContext(ctx, `
		INSERT INTO notification_jobs (
			notification_job_id, company_id, event_type, resource_type, resource_id, payload_json, status
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`, job.NotificationJobID, job.CompanyID, job.EventType, job.ResourceType, job.ResourceID, payload, job.Status)
	if err != nil {
		return nil, fmt.Errorf("notification job insert: %w", err)
	}
	cp := job
	return &cp, nil
}

func (r *Repository) ListPendingJobs(ctx context.Context, companyID string, limit int) ([]notificationapp.NotificationJobDTO, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT notification_job_id, company_id, event_type, resource_type, resource_id, payload_json, status
		FROM notification_jobs WHERE company_id = ? AND status = 'pending' ORDER BY created_at ASC LIMIT ?
	`, companyID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanJobs(rows)
}

func scanJobs(rows *sql.Rows) ([]notificationapp.NotificationJobDTO, error) {
	var out []notificationapp.NotificationJobDTO
	for rows.Next() {
		var j notificationapp.NotificationJobDTO
		var payload []byte
		if err := rows.Scan(&j.NotificationJobID, &j.CompanyID, &j.EventType, &j.ResourceType, &j.ResourceID, &payload, &j.Status); err != nil {
			return nil, err
		}
		if len(payload) > 0 {
			_ = json.Unmarshal(payload, &j.Payload)
		}
		if j.Payload == nil {
			j.Payload = map[string]any{}
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

func (r *Repository) UpdateJobStatus(ctx context.Context, companyID, jobID, status string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE notification_jobs SET status = ? WHERE notification_job_id = ? AND company_id = ?
	`, status, jobID, companyID)
	return err
}

func (r *Repository) CreateDelivery(ctx context.Context, d notificationapp.NotificationDeliveryDTO) (*notificationapp.NotificationDeliveryDTO, error) {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO notification_deliveries (notification_delivery_id, notification_job_id, recipient, status)
		VALUES (?, ?, ?, ?)
	`, d.NotificationDeliveryID, d.NotificationJobID, d.Recipient, d.Status)
	if err != nil {
		return nil, err
	}
	cp := d
	return &cp, nil
}
