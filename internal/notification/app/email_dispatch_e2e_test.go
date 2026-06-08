package app_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"

	notificationapp "github.com/cobo/cobo_iam_services/internal/notification/app"
	notificationmysql "github.com/cobo/cobo_iam_services/internal/notification/infra/mysql"
	notificationregistry "github.com/cobo/cobo_iam_services/internal/notification/infra/registry"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
	platformoutbox "github.com/cobo/cobo_iam_services/internal/platform/outbox"
	outboxmysql "github.com/cobo/cobo_iam_services/internal/platform/outbox/mysql"
)

// This file proves AK.4's Batch 2A acceptance criterion end-to-end against a
// real MySQL-backed outbox: DispatchEmail persists the email_notifications row
// and publishes its email.dispatch outbox event atomically (Option A —
// Transactional Publish), the outbox processor dequeues it through the exact
// wrapper-closure registered in cmd/worker/main.go, and the synthetic
// EmailDispatchHandler drives the row from pending -> sending -> sent (or,
// after a transient SMTP error, pending -> retry -> sent on redelivery).
//
// Requires a real MySQL instance with migrations applied (mirrors
// iam/infra/mysql/credentials_subscription_test.go's openTestDB/t.Skipf
// convention) — skips automatically when unavailable.

// e2eIDGen issues IDs unique to this test run so reruns against a persistent
// test database never collide with rows left by a previous run.
type e2eIDGen struct {
	prefix string
	n      int
}

func (g *e2eIDGen) NewUUID() string {
	g.n++
	return fmt.Sprintf("%s-%03d", g.prefix, g.n)
}

func openEmailE2EDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := "root:secret@tcp(127.0.0.1:3306)/cobo_iam?parseTime=true&loc=UTC"
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Skipf("mysql not available: %v", err)
	}
	if err := db.Ping(); err != nil {
		t.Skipf("mysql ping: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// cleanupEmailE2ERows deletes the synthetic notification (cascades to its
// delivery attempts via fk_email_delivery_attempts_notif ON DELETE CASCADE)
// and its outbox event so reruns start from a clean slate.
func cleanupEmailE2ERows(db *sql.DB, notificationID string) {
	ctx := context.Background()
	_, _ = db.ExecContext(ctx, `DELETE FROM outbox_events WHERE aggregate_id = ? AND event_type = ?`, notificationID, notificationapp.EmailDispatchOutboxEventType)
	_, _ = db.ExecContext(ctx, `DELETE FROM email_notifications WHERE email_notification_id = ?`, notificationID)
}

func assertOutboxEventStatus(t *testing.T, db *sql.DB, notificationID, want string) {
	t.Helper()
	var got string
	err := db.QueryRowContext(context.Background(),
		`SELECT status FROM outbox_events WHERE aggregate_id = ? AND event_type = ?`,
		notificationID, notificationapp.EmailDispatchOutboxEventType,
	).Scan(&got)
	if err != nil {
		t.Fatalf("query outbox event status: %v", err)
	}
	if got != want {
		t.Fatalf("outbox event status = %q, want %q", got, want)
	}
}

type emailE2EFixture struct {
	db        *sql.DB
	svc       *notificationapp.EmailNotificationService
	processor *platformoutbox.Processor
	notifRepo *notificationmysql.EmailNotificationRepository
	adapter   *fakeAdapter
	idg       *e2eIDGen
}

// newEmailDispatchE2EFixture wires the full durable pipeline with REAL
// MySQL-backed repositories end to end:
//
//	EmailNotificationService (WithTransactionalDispatch: BeginTx -> InsertNotificationTx -> PublishEventTx -> Commit)
//	  -> outbox_events (real outboxmysql.Repository)
//	    -> platformoutbox.Processor.Tick (LockPendingBatch / MarkProcessed / MarkRetry)
//	      -> wrapper closure: event.PayloadJSON -> EmailDispatchHandler.Handle  (mirrors cmd/worker/main.go's registration)
//	        -> fake DeliveryAdapter.Send
//
// Only the SMTP transport is faked; every persistence and routing layer is the
// genuine production implementation.
func newEmailDispatchE2EFixture(t *testing.T) emailE2EFixture {
	t.Helper()
	sqlDB := openEmailE2EDB(t)

	notifRepo := notificationmysql.NewEmailNotificationRepository(sqlDB)
	attemptRepo := notificationmysql.NewEmailDeliveryAttemptRepository(sqlDB)
	outboxRepo := outboxmysql.NewRepository(sqlDB)
	registry := notificationregistry.NewEmbedRegistry()
	renderer := notificationapp.NewEmailRenderer()
	adapter := &fakeAdapter{}
	idg := &e2eIDGen{prefix: fmt.Sprintf("e2e%d", time.Now().UnixNano())}
	clock := func() time.Time { return time.Now().UTC() }

	svc := notificationapp.NewEmailNotificationService(
		notifRepo,
		registry,
		renderer,
		idg,
		clock,
		notificationapp.WithTransactionalDispatch(sqlDB, outboxRepo),
	)

	handler := notificationapp.NewEmailDispatchHandler(
		notifRepo,
		attemptRepo,
		registry,
		renderer,
		adapter,
		idgen.UUIDv7Generator{},
		clock,
		0,
	)

	processor := platformoutbox.NewProcessor(outboxRepo, 50)
	// Mirrors cmd/worker/main.go's registration EXACTLY: a wrapper closure
	// that unwraps event.PayloadJSON and forwards it to the handler — never a
	// direct processor.Register(EmailDispatchOutboxEventType, handler.Handle).
	processor.Register(notificationapp.EmailDispatchOutboxEventType, platformoutbox.HandlerFunc(func(ctx context.Context, event platformoutbox.QueuedEvent) error {
		return handler.Handle(ctx, event.PayloadJSON)
	}))

	return emailE2EFixture{db: sqlDB, svc: svc, processor: processor, notifRepo: notifRepo, adapter: adapter, idg: idg}
}

func (f emailE2EFixture) dispatchRequest(label string) notificationapp.DispatchEmailRequest {
	return notificationapp.DispatchEmailRequest{
		To:          fmt.Sprintf("%s-%s@example.com", f.idg.prefix, label),
		TemplateKey: "auth.email_verification",
		Locale:      "vi",
		Variables: map[string]any{
			"otp_code": "777" + label, "expiry_minutes": 15,
			"support_email": "support@cobo.vn", "website_url": "https://app.example.com",
		},
		IdempotencyKey:      fmt.Sprintf("%s-idem-%s", f.idg.prefix, label),
		TriggeredByUserID:   "u_" + f.idg.prefix,
		SourceEventType:     "auth.email_verification_requested",
		SourceAggregateType: "user",
		SourceAggregateID:   "u_" + f.idg.prefix,
	}
}

// TestEmailDispatchE2E_TransactionalPublishWorkerDeliversHappyPath proves the
// full chain: DispatchEmail inserts + publishes atomically, the worker's
// registered handler consumes the event and the fake adapter "sends" it,
// landing the row on status=sent — and that replaying the same idempotency key
// is a safe no-op (no duplicate row, no duplicate outbox publish, no resend).
func TestEmailDispatchE2E_TransactionalPublishWorkerDeliversHappyPath(t *testing.T) {
	f := newEmailDispatchE2EFixture(t)
	ctx := context.Background()
	req := f.dispatchRequest("happy")

	notif, err := f.svc.DispatchEmail(ctx, req)
	if err != nil {
		t.Fatalf("DispatchEmail err = %v", err)
	}
	t.Cleanup(func() { cleanupEmailE2ERows(f.db, notif.EmailNotificationID) })

	if notif.Status != notificationapp.EmailStatusPending {
		t.Fatalf("status after transactional dispatch = %q, want pending", notif.Status)
	}
	assertOutboxEventStatus(t, f.db, notif.EmailNotificationID, "pending")

	if err := f.processor.Tick(ctx); err != nil {
		t.Fatalf("processor.Tick err = %v", err)
	}

	stored, err := f.notifRepo.GetByID(ctx, notif.EmailNotificationID)
	if err != nil || stored == nil {
		t.Fatalf("GetByID err=%v stored=%v", err, stored)
	}
	if stored.Status != notificationapp.EmailStatusSent {
		t.Fatalf("status after worker dispatch = %q, want sent", stored.Status)
	}
	if stored.SentAt == nil {
		t.Fatalf("sent_at must be populated once delivered")
	}
	if len(f.adapter.calls) != 1 {
		t.Fatalf("adapter calls = %d, want 1", len(f.adapter.calls))
	}
	if !strings.Contains(f.adapter.calls[0].TextBody, "777happy") {
		t.Fatalf("delivered body missing OTP: %q", f.adapter.calls[0].TextBody)
	}
	assertOutboxEventStatus(t, f.db, notif.EmailNotificationID, "processed")

	// Replay safety: same idempotency key must short-circuit to the existing
	// row — no second insert, no second outbox publish, no second send.
	replay, err := f.svc.DispatchEmail(ctx, req)
	if err != nil {
		t.Fatalf("replay DispatchEmail err = %v", err)
	}
	if replay.EmailNotificationID != notif.EmailNotificationID {
		t.Fatalf("replay produced a different notification: got %s want %s", replay.EmailNotificationID, notif.EmailNotificationID)
	}
	var outboxCount int
	if err := f.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM outbox_events WHERE aggregate_id = ? AND event_type = ?`,
		notif.EmailNotificationID, notificationapp.EmailDispatchOutboxEventType,
	).Scan(&outboxCount); err != nil {
		t.Fatalf("count outbox rows: %v", err)
	}
	if outboxCount != 1 {
		t.Fatalf("outbox event rows after replay = %d, want 1 (no duplicate publish)", outboxCount)
	}
	if err := f.processor.Tick(ctx); err != nil {
		t.Fatalf("processor.Tick after replay err = %v", err)
	}
	if len(f.adapter.calls) != 1 {
		t.Fatalf("adapter calls after replay tick = %d, want still 1 (no resend)", len(f.adapter.calls))
	}
}

// TestEmailDispatchE2E_TransientErrorRetriesThenSucceeds proves the retry
// path required by Batch 2A: a first transient SMTP error marks the
// notification "retry" and schedules a future redelivery (status stays
// pending with a future available_at — never lost), an early tick must NOT
// redeliver before that window opens, and once the window opens the processor
// redelivers and the second attempt lands the row on sent.
func TestEmailDispatchE2E_TransientErrorRetriesThenSucceeds(t *testing.T) {
	f := newEmailDispatchE2EFixture(t)
	ctx := context.Background()
	f.adapter.results = []fakeAdapterResult{
		{err: errors.New("421 service not available, try again later")},
	}
	req := f.dispatchRequest("retry")

	notif, err := f.svc.DispatchEmail(ctx, req)
	if err != nil {
		t.Fatalf("DispatchEmail err = %v", err)
	}
	t.Cleanup(func() { cleanupEmailE2ERows(f.db, notif.EmailNotificationID) })

	// First delivery attempt: transient SMTP error.
	if err := f.processor.Tick(ctx); err != nil {
		t.Fatalf("processor.Tick (1st attempt) err = %v", err)
	}
	if len(f.adapter.calls) != 1 {
		t.Fatalf("adapter calls after 1st tick = %d, want 1", len(f.adapter.calls))
	}
	stored, err := f.notifRepo.GetByID(ctx, notif.EmailNotificationID)
	if err != nil || stored == nil {
		t.Fatalf("GetByID err=%v stored=%v", err, stored)
	}
	if stored.Status != notificationapp.EmailStatusRetry {
		t.Fatalf("status after transient failure = %q, want retry", stored.Status)
	}
	if stored.LastErrorCode != "transient_smtp" {
		t.Fatalf("last_error_code = %q, want transient_smtp", stored.LastErrorCode)
	}

	var outboxStatus string
	var availableAt time.Time
	if err := f.db.QueryRowContext(ctx,
		`SELECT status, available_at FROM outbox_events WHERE aggregate_id = ? AND event_type = ?`,
		notif.EmailNotificationID, notificationapp.EmailDispatchOutboxEventType,
	).Scan(&outboxStatus, &availableAt); err != nil {
		t.Fatalf("query outbox row: %v", err)
	}
	if outboxStatus != "pending" {
		t.Fatalf("outbox status after transient failure = %q, want pending (redelivery scheduled, not dropped)", outboxStatus)
	}
	if !availableAt.After(time.Now().UTC()) {
		t.Fatalf("expected redelivery scheduled in the future, available_at = %v", availableAt)
	}

	// A tick before the scheduled window must not redeliver early.
	if err := f.processor.Tick(ctx); err != nil {
		t.Fatalf("processor.Tick (early, before window) err = %v", err)
	}
	if len(f.adapter.calls) != 1 {
		t.Fatalf("adapter calls before redelivery window = %d, want still 1 (no early redelivery)", len(f.adapter.calls))
	}

	// Open the redelivery window (simulates the backoff elapsing) — the next
	// attempt is programmed to succeed.
	f.adapter.results = []fakeAdapterResult{
		{res: notificationapp.DeliveryResult{ProviderMessageID: "redelivered-ok", Provider: "smtp"}},
	}
	if _, err := f.db.ExecContext(ctx,
		`UPDATE outbox_events SET available_at = ? WHERE aggregate_id = ? AND event_type = ?`,
		time.Now().UTC().Add(-time.Second), notif.EmailNotificationID, notificationapp.EmailDispatchOutboxEventType,
	); err != nil {
		t.Fatalf("force redelivery window open: %v", err)
	}

	if err := f.processor.Tick(ctx); err != nil {
		t.Fatalf("processor.Tick (redelivery) err = %v", err)
	}
	if len(f.adapter.calls) != 2 {
		t.Fatalf("adapter calls after redelivery = %d, want 2 (1 transient + 1 success)", len(f.adapter.calls))
	}

	stored, err = f.notifRepo.GetByID(ctx, notif.EmailNotificationID)
	if err != nil || stored == nil {
		t.Fatalf("GetByID err=%v stored=%v", err, stored)
	}
	if stored.Status != notificationapp.EmailStatusSent {
		t.Fatalf("final status = %q, want sent", stored.Status)
	}
	if stored.SentAt == nil {
		t.Fatalf("sent_at must be populated once redelivery succeeds")
	}
	assertOutboxEventStatus(t, f.db, notif.EmailNotificationID, "processed")
}
