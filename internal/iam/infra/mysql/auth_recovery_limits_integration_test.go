package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	_ "github.com/go-sql-driver/mysql"
)

// Run against a migrated schema (0001, 0007, 0027 at least):
//
//	MYSQL_TEST_DSN='<user>:<pass>@tcp(127.0.0.1:3306)/cobo_iam?parseTime=true&loc=UTC' go test ./internal/iam/infra/mysql/ -run AuthRecoveryLimits
func openRecoveryLimitsDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("MYSQL_TEST_DSN"))
	if dsn == "" {
		t.Skip("MYSQL_TEST_DSN not set")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Ping(); err != nil {
		t.Fatal(err)
	}
	return db
}

func seedRecoveryLimitsUser(t *testing.T, db *sql.DB, userID string) {
	t.Helper()
	ctx := context.Background()
	for _, q := range []string{
		`DELETE FROM email_verification_otps WHERE user_id = ?`,
		`DELETE FROM password_reset_tokens WHERE user_id = ?`,
		`DELETE FROM users WHERE user_id = ?`,
	} {
		if _, err := db.ExecContext(ctx, q, userID); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO users (user_id, login_id, full_name, email, account_status)
		VALUES (?, ?, 'Limits Test', ?, 'active')
	`, userID, userID+"@example.test", userID+"@example.test"); err != nil {
		t.Fatal(err)
	}
}

// AUTH-03 (risk-review-auth-flows-2026-10-15): invalidating the pending OTP before a resend
// used to DELETE the row, so the hourly send counter never exceeded 1.
func TestAuthRecoveryLimits_OTPSendsStayCountedAfterInvalidation(t *testing.T) {
	db := openRecoveryLimitsDB(t)
	ctx := context.Background()
	const userID = "u_authrl_otp"
	seedRecoveryLimitsUser(t, db, userID)
	repo := NewAuthRecoveryRepository(db)

	for i := 0; i < 5; i++ {
		if err := repo.InvalidatePendingEmailVerificationOTPs(ctx, userID); err != nil {
			t.Fatal(err)
		}
		if err := repo.StoreEmailVerificationOTP(ctx, iamapp.EmailOTPRecord{
			OTPID: userID + "-otp-" + string(rune('a'+i)), UserID: userID, CodeHash: "x", ExpiresAt: time.Now().UTC().Add(15 * time.Minute),
		}); err != nil {
			t.Fatal(err)
		}
	}

	n, err := repo.CountEmailVerificationOTPsSince(ctx, userID, time.Now().UTC().Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if n != 5 {
		t.Fatalf("OTP sends in the last hour = %d, want 5", n)
	}
	var usable int
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM email_verification_otps WHERE user_id = ? AND consumed_at IS NULL AND expires_at > ?
	`, userID, time.Now().UTC()).Scan(&usable); err != nil {
		t.Fatal(err)
	}
	if usable != 1 {
		t.Fatalf("usable OTPs after resends = %d, want only the latest", usable)
	}
}

func TestAuthRecoveryLimits_IssueWithinLimits(t *testing.T) {
	db := openRecoveryLimitsDB(t)
	ctx := context.Background()
	const userID = "u_authrl_within"
	seedRecoveryLimitsUser(t, db, userID)
	seedRecoveryLimitsUser(t, db, "u_authrl_other")
	repo := NewAuthRecoveryRepository(db)
	now := time.Now().UTC()
	windows := []iamapp.SendWindow{{Since: now.Add(-time.Hour), Max: 2}, {Since: now.Add(-24 * time.Hour), Max: 3}}

	token := func(id, user string) iamapp.RecoveryTokenRecord {
		return iamapp.RecoveryTokenRecord{TokenID: id, UserID: user, TokenHash: id + "-hash", ExpiresAt: now.Add(30 * time.Minute)}
	}
	// One token from 3 hours ago counts for the day window only.
	if err := repo.StorePasswordResetToken(ctx, token(userID+"-old", userID)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE password_reset_tokens SET created_at = ? WHERE token_id = ?`, now.Add(-3*time.Hour), userID+"-old"); err != nil {
		t.Fatal(err)
	}
	// Another user's tokens never count.
	for i := 0; i < 3; i++ {
		if err := repo.StorePasswordResetToken(ctx, token(fmt.Sprintf("u_authrl_other-%d", i), "u_authrl_other")); err != nil {
			t.Fatal(err)
		}
	}
	var got []bool
	for i := 0; i < 3; i++ {
		ok, err := repo.StorePasswordResetTokenWithinLimits(ctx, token(fmt.Sprintf("%s-%d", userID, i), userID), windows)
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, ok)
	}
	if fmt.Sprint(got) != "[true true false]" {
		t.Fatalf("reset tokens issued = %v, want [true true false] (day window 3 incl. the old one)", got)
	}

	var otpIssued []bool
	for i := 0; i < 3; i++ {
		ok, err := repo.IssueEmailVerificationOTPWithinLimits(ctx, iamapp.EmailOTPRecord{
			OTPID: fmt.Sprintf("%s-otp-%d", userID, i), UserID: userID, CodeHash: "x", ExpiresAt: now.Add(15 * time.Minute),
		}, windows)
		if err != nil {
			t.Fatal(err)
		}
		otpIssued = append(otpIssued, ok)
	}
	if fmt.Sprint(otpIssued) != "[true true false]" {
		t.Fatalf("OTPs issued = %v, want [true true false]", otpIssued)
	}
	var usable int
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM email_verification_otps WHERE user_id = ? AND consumed_at IS NULL AND expires_at > ?
	`, userID, time.Now().UTC()).Scan(&usable); err != nil {
		t.Fatal(err)
	}
	if usable != 1 {
		t.Fatalf("usable OTPs = %d, want only the latest", usable)
	}
}
