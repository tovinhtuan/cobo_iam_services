package app

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

type mysqlSubscriptionUpgradePaymentRepo struct {
	db *sql.DB
}

func NewMySQLSubscriptionUpgradePaymentRepository(db *sql.DB) SubscriptionUpgradePaymentRepository {
	return &mysqlSubscriptionUpgradePaymentRepo{db: db}
}

func (r *mysqlSubscriptionUpgradePaymentRepo) Get(ctx context.Context) (*SubscriptionUpgradePaymentRecord, error) {
	if r == nil || r.db == nil {
		return EnsureEmptyRecord(), nil
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT description_vi, description_en, hotline, bank_name, account_name, account_number,
		       transfer_note_template, is_active, qr_object_key, qr_content_type, qr_file_name,
		       updated_by, updated_at
		FROM platform_subscription_upgrade_payment
		WHERE id = 1
	`)
	var rec SubscriptionUpgradePaymentRecord
	var descVI, descEN, hotline, bankName, accountName, accountNumber, tmpl sql.NullString
	var isActive int
	err := row.Scan(
		&descVI, &descEN, &hotline, &bankName, &accountName, &accountNumber,
		&tmpl, &isActive, &rec.QRObjectKey, &rec.QRContentType, &rec.QRFileName,
		&rec.UpdatedBy, &rec.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return EnsureEmptyRecord(), nil
	}
	if err != nil {
		return nil, err
	}
	rec.DescriptionVI = descVI.String
	rec.DescriptionEN = descEN.String
	rec.Hotline = hotline.String
	rec.BankName = bankName.String
	rec.AccountName = accountName.String
	rec.AccountNumber = accountNumber.String
	rec.TransferNoteTemplate = tmpl.String
	rec.IsActive = isActive == 1
	if rec.TransferNoteTemplate == "" {
		rec.TransferNoteTemplate = "COBO {{company_code}} NANGCAPGOI"
	}
	if rec.UpdatedAt.IsZero() {
		rec.UpdatedAt = time.Now().UTC()
	}
	return &rec, nil
}

func (r *mysqlSubscriptionUpgradePaymentRepo) UpsertFields(ctx context.Context, req UpdateSubscriptionUpgradePaymentRequest) error {
	active := 0
	if req.IsActive {
		active = 1
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO platform_subscription_upgrade_payment (
			id, description_vi, description_en, hotline, bank_name, account_name, account_number,
			transfer_note_template, is_active, updated_by
		) VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			description_vi = VALUES(description_vi),
			description_en = VALUES(description_en),
			hotline = VALUES(hotline),
			bank_name = VALUES(bank_name),
			account_name = VALUES(account_name),
			account_number = VALUES(account_number),
			transfer_note_template = VALUES(transfer_note_template),
			is_active = VALUES(is_active),
			updated_by = VALUES(updated_by)
	`, nullIfEmpty(req.DescriptionVI), nullIfEmpty(req.DescriptionEN), nullIfEmpty(req.Hotline),
		nullIfEmpty(req.BankName), nullIfEmpty(req.AccountName), nullIfEmpty(req.AccountNumber),
		nullIfEmpty(req.TransferNoteTemplate), active, nullIfEmpty(req.ActorID))
	return err
}

func (r *mysqlSubscriptionUpgradePaymentRepo) SetQR(ctx context.Context, objectKey, contentType, fileName, actorID string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO platform_subscription_upgrade_payment (
			id, qr_object_key, qr_content_type, qr_file_name, updated_by, transfer_note_template, is_active
		) VALUES (1, ?, ?, ?, ?, 'COBO {{company_code}} NANGCAPGOI', 0)
		ON DUPLICATE KEY UPDATE
			qr_object_key = VALUES(qr_object_key),
			qr_content_type = VALUES(qr_content_type),
			qr_file_name = VALUES(qr_file_name),
			updated_by = VALUES(updated_by)
	`, objectKey, contentType, fileName, nullIfEmpty(actorID))
	return err
}

func (r *mysqlSubscriptionUpgradePaymentRepo) ClearQR(ctx context.Context, actorID string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE platform_subscription_upgrade_payment
		SET qr_object_key = NULL, qr_content_type = NULL, qr_file_name = NULL, updated_by = ?
		WHERE id = 1
	`, nullIfEmpty(actorID))
	return err
}

func nullIfEmpty(v string) any {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return v
}
