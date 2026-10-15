package app_test

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	cainmem "github.com/cobo/cobo_iam_services/internal/companyaccess/infra/inmemory"
	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	iaminmem "github.com/cobo/cobo_iam_services/internal/iam/infra/inmemory"
	iammysql "github.com/cobo/cobo_iam_services/internal/iam/infra/mysql"
	"github.com/cobo/cobo_iam_services/internal/platform/events"
	_ "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
)

// AUTH-03 against MySQL: parallel requests for one user must not pass the per-user limits.
// Run with a migrated schema (0001, 0007, 0027 at least):
//
//	MYSQL_TEST_DSN='<user>:<pass>@tcp(127.0.0.1:3306)/cobo_iam?parseTime=true&loc=UTC' go test ./internal/iam/app/ -run 'Concurrent.*MySQL'

type uuidGen struct{}

func (uuidGen) NewUUID() string { return uuid.NewString() }

type atomicOutboxPublisher struct{ n atomic.Int64 }

func (p *atomicOutboxPublisher) Publish(context.Context, events.Event) error {
	p.n.Add(1)
	return nil
}

func openLimitsMySQL(t *testing.T) *sql.DB {
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

func seedLimitsMySQLUser(t *testing.T, db *sql.DB, userID, email string) {
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
		INSERT INTO users (user_id, login_id, full_name, email, account_status) VALUES (?, ?, 'Limits', ?, 'active')
	`, userID, email, email); err != nil {
		t.Fatal(err)
	}
}

func newLimitsMySQLService(db *sql.DB, pub *atomicOutboxPublisher) iamapp.Service {
	id := uuidGen{}
	return iamapp.NewService(testCred(), iaminmem.NewSessionRepository(), iaminmem.NewTokenManager(id), cainmem.NewMembershipQueryService(), id,
		iamapp.WithOutboxPublisher(pub),
		iamapp.WithAuthRecoveryRepository(iammysql.NewAuthRecoveryRepository(db)),
		iamapp.WithAuthFlowConfig(iamapp.AuthFlowConfig{
			WebBaseURL:              "https://app.example.com",
			PasswordResetTokenTTL:   30 * time.Minute,
			EmailVerificationOTPTTL: 15 * time.Minute,
		}),
	)
}

func runParallel(n int, call func()) {
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			call()
		}()
	}
	close(start)
	wg.Wait()
}

func countRows(t *testing.T, db *sql.DB, query, userID string) int {
	t.Helper()
	var n int
	if err := db.QueryRowContext(context.Background(), query, userID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestAuthEmailLimits_ConcurrentForgotPasswordMySQL(t *testing.T) {
	db := openLimitsMySQL(t)
	const userID, email = "u_ael_conc_reset", "ael.conc.reset@example.test"
	seedLimitsMySQLUser(t, db, userID, email)
	pub := &atomicOutboxPublisher{}
	svc := newLimitsMySQLService(db, pub)

	runParallel(20, func() {
		_, _ = svc.ForgotPassword(context.Background(), iamapp.ForgotPasswordRequest{Email: email})
	})

	tokens := countRows(t, db, `SELECT COUNT(*) FROM password_reset_tokens WHERE user_id = ?`, userID)
	if sent := pub.n.Load(); sent < 1 || sent > 3 || int(sent) != tokens {
		t.Fatalf("20 parallel forgot-password requests: %d emails, %d tokens; want 1..3 and equal", sent, tokens)
	}
}

func TestAuthEmailLimits_ConcurrentResendOTPMySQL(t *testing.T) {
	db := openLimitsMySQL(t)
	const userID, email = "u_ael_conc_otp", "ael.conc.otp@example.test"
	seedLimitsMySQLUser(t, db, userID, email)
	pub := &atomicOutboxPublisher{}
	svc := newLimitsMySQLService(db, pub)

	runParallel(20, func() {
		_, _ = svc.ResendVerificationEmail(context.Background(), iamapp.ResendVerificationEmailRequest{UserID: userID})
	})

	otps := countRows(t, db, `SELECT COUNT(*) FROM email_verification_otps WHERE user_id = ?`, userID)
	usable := countRows(t, db, `SELECT COUNT(*) FROM email_verification_otps WHERE user_id = ? AND consumed_at IS NULL AND expires_at > UTC_TIMESTAMP()`, userID)
	if sent := pub.n.Load(); sent < 1 || sent > 5 || int(sent) != otps {
		t.Fatalf("20 parallel resends: %d emails, %d OTP rows; want 1..5 and equal", sent, otps)
	}
	if usable != 1 {
		t.Fatalf("usable OTPs after parallel resends = %d, want 1", usable)
	}
}
