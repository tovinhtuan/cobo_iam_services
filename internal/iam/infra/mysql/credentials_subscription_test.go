package mysql

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

func TestLoadUserSubscription_expiresAt(t *testing.T) {
	db, cleanup := openTestDB(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	exp := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)

	_, err := db.ExecContext(ctx, `
		INSERT INTO users (user_id, login_id, full_name, account_status)
		VALUES ('u_sub_exp', 'sub.exp@example.com', 'Sub Exp', 'active')
		ON DUPLICATE KEY UPDATE full_name = VALUES(full_name)
	`)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	_, err = db.ExecContext(ctx, `
		INSERT INTO user_subscription_tiers (user_id, subscription_tier, source, effective_from, effective_to)
		VALUES ('u_sub_exp', 'Premium', 'test', NULL, ?)
		ON DUPLICATE KEY UPDATE subscription_tier = VALUES(subscription_tier), effective_to = VALUES(effective_to)
	`, exp)
	if err != nil {
		t.Fatalf("insert tier: %v", err)
	}

	sub := loadUserSubscription(ctx, db, "u_sub_exp", now)
	if sub.Tier != "Premium" {
		t.Fatalf("tier=%q want Premium", sub.Tier)
	}
	if sub.ExpiresAt == nil || !sub.ExpiresAt.Equal(exp.UTC()) {
		t.Fatalf("expiresAt=%v want %v", sub.ExpiresAt, exp.UTC())
	}
}

func openTestDB(t *testing.T) (*sql.DB, func()) {
	t.Helper()
	dsn := "root:secret@tcp(127.0.0.1:3306)/cobo_iam?parseTime=true&loc=UTC"
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Skipf("mysql not available: %v", err)
	}
	if err := db.Ping(); err != nil {
		t.Skipf("mysql ping: %v", err)
	}
	return db, func() { _ = db.Close() }
}
