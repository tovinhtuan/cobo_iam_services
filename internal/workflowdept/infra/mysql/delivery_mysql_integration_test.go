package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cobo/cobo_iam_services/internal/workflowdept"
	_ "github.com/go-sql-driver/mysql"
)

func TestMySQLConcurrentClaimAndTerminalGuards(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("MYSQL_TEST_DSN"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("MYSQL_DSN"))
	}
	if dsn == "" {
		t.Skip("LIVE_PREFLIGHT=NOT_RUN reason=MYSQL_DSN not configured; .env not loaded")
	}
	dbName := mysqlDatabaseName(dsn)
	if !strings.Contains(strings.ToLower(dbName), "test") {
		t.Skip("refusing DSN whose database name does not contain test")
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("mysql ping: %v", err)
	}

	var existed int
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM information_schema.tables
		WHERE table_schema = DATABASE() AND table_name = 'reminder_dispatch_resolutions'
	`).Scan(&existed); err != nil {
		t.Fatal(err)
	}
	if existed > 0 {
		var rows int
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM reminder_dispatch_resolutions`).Scan(&rows); err != nil {
			t.Fatal(err)
		}
		if rows > 0 {
			t.Skip("reminder_dispatch_resolutions is not empty; refusing to mutate it")
		}
	} else {
		if _, err := db.ExecContext(ctx, claimProbeDDL); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			_, _ = db.ExecContext(context.Background(), `DROP TABLE IF EXISTS reminder_dispatch_resolutions`)
		})
	}

	store := NewDeliveryStore(db)
	now := time.Now().UTC()
	insertProbe(t, ctx, db, "eds-pending", "occ-pending", workflowdept.SendPending, nil, nil)
	var claimed atomic.Int32
	for round := 0; round < 5; round++ {
		if _, err := db.ExecContext(ctx, `
			UPDATE reminder_dispatch_resolutions
			SET send_status = 'PENDING', lease_id = NULL, lease_until = NULL, next_retry_at = NULL
			WHERE resolution_id = 'eds-pending'
		`); err != nil {
			t.Fatal(err)
		}
		var wg sync.WaitGroup
		for i := 0; i < 2; i++ {
			wg.Add(1)
			lease := fmt.Sprintf("lease-%d-%d", round, i)
			go func() {
				defer wg.Done()
				got, err := store.ClaimDueResolution(context.Background(), lease, now, now.Add(workflowdept.BindingSMTPLease))
				if err != nil {
					t.Errorf("claim: %v", err)
					return
				}
				switch got.Outcome {
				case workflowdept.ClaimClaimed:
					claimed.Add(1)
				case workflowdept.ClaimNotFound, workflowdept.ClaimAlreadyOwned:
				default:
					t.Errorf("outcome %s", got.Outcome)
				}
			}()
		}
		wg.Wait()
		var sending int
		if err := db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM reminder_dispatch_resolutions
			WHERE resolution_id = 'eds-pending' AND send_status = 'SENDING'
		`).Scan(&sending); err != nil {
			t.Fatal(err)
		}
		if sending != 1 {
			t.Fatalf("round %d sending rows = %d", round, sending)
		}
	}
	if claimed.Load() != 5 {
		t.Fatalf("claimed rounds = %d", claimed.Load())
	}

	past := now.Add(-time.Minute)
	future := now.Add(time.Hour)
	insertProbe(t, ctx, db, "eds-retry", "occ-retry", workflowdept.SendRetryableFailed, &past, nil)
	first, err := store.ClaimDueResolution(ctx, "retry-owner", now, now.Add(workflowdept.BindingSMTPLease))
	if err != nil || first.Outcome != workflowdept.ClaimClaimed || first.Resolution.ResolutionID != "eds-retry" {
		t.Fatalf("due retry %+v err %v", first, err)
	}
	insertProbe(t, ctx, db, "eds-live", "occ-live", workflowdept.SendSending, nil, &future)
	if _, err := db.ExecContext(ctx, `UPDATE reminder_dispatch_resolutions SET lease_id = 'holder' WHERE resolution_id = 'eds-live'`); err != nil {
		t.Fatal(err)
	}
	live, err := store.ClaimDueResolution(ctx, "other", now, now.Add(workflowdept.BindingSMTPLease))
	if err != nil {
		t.Fatal(err)
	}
	if live.Resolution.ResolutionID == "eds-live" && live.Outcome == workflowdept.ClaimClaimed {
		t.Fatal("live lease was claimed")
	}
	for _, status := range []string{workflowdept.SendSent, workflowdept.SendUnknown, workflowdept.SendPermanentFailed} {
		id := "eds-" + strings.ToLower(status)
		insertProbe(t, ctx, db, id, "occ-"+strings.ToLower(status), status, nil, nil)
		got, err := store.ClaimDueResolution(ctx, "nope", now, now.Add(workflowdept.BindingSMTPLease))
		if err != nil {
			t.Fatal(err)
		}
		if got.Resolution.ResolutionID == id && got.Outcome == workflowdept.ClaimClaimed {
			t.Fatalf("%s was claimed", status)
		}
	}
}

func insertProbe(t *testing.T, ctx context.Context, db *sql.DB, id, occurrence, status string, nextRetry, leaseUntil *time.Time) {
	t.Helper()
	_, err := db.ExecContext(ctx, `
		INSERT INTO reminder_dispatch_resolutions (
			resolution_id, occurrence_id, company_id, resolved_at, send_status, next_retry_at, lease_until
		) VALUES (?, ?, 'co-test', UTC_TIMESTAMP(3), ?, ?, ?)
	`, id, occurrence, status, nextRetry, leaseUntil)
	if err != nil {
		t.Fatal(err)
	}
}

func mysqlDatabaseName(dsn string) string {
	slash := strings.LastIndex(dsn, "/")
	if slash < 0 || slash+1 >= len(dsn) {
		return ""
	}
	name := dsn[slash+1:]
	if cut := strings.IndexAny(name, "?"); cut >= 0 {
		name = name[:cut]
	}
	return name
}

const claimProbeDDL = `
CREATE TABLE reminder_dispatch_resolutions (
  resolution_id CHAR(36) NOT NULL,
  occurrence_id VARCHAR(80) NOT NULL,
  company_id VARCHAR(36) NOT NULL,
  resolved_at DATETIME(3) NOT NULL,
  send_status VARCHAR(32) NOT NULL,
  provider_message_id VARCHAR(255) NULL,
  recipient_email_ciphertext BLOB NULL,
  email_key_version INT UNSIGNED NULL,
  lease_id CHAR(36) NULL,
  lease_until DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  attempt_count INT NOT NULL DEFAULT 0,
  next_retry_at DATETIME(3) NULL,
  last_error_code VARCHAR(64) NULL,
  PRIMARY KEY (resolution_id),
  UNIQUE KEY uk_reminder_dispatch_occurrence (occurrence_id)
)`
