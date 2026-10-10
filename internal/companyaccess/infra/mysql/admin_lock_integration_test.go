package mysql

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"
)

// ROLE-23: LockCompanyAdmins serializes per company, across connections. Skipped when
// MYSQL_TEST_DSN is not set (it needs no tables).
func TestIntegration_LockCompanyAdmins_SerializesPerCompany(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("MYSQL_TEST_DSN"))
	if dsn == "" {
		t.Skip("MYSQL_TEST_DSN not set")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open mysql: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Ping(); err != nil {
		t.Skipf("mysql not reachable: %v", err)
	}
	repo := NewAdminRepository(db)
	ctx := context.Background()
	company := "it-lock-" + time.Now().Format("150405.000000")

	release, err := repo.LockCompanyAdmins(ctx, company)
	if err != nil {
		t.Fatalf("first lock: %v", err)
	}
	other, err := repo.LockCompanyAdmins(ctx, company+"-other")
	if err != nil {
		t.Fatalf("another company must not wait: %v", err)
	}
	other()

	got := make(chan time.Time, 1)
	go func() {
		r2, err := repo.LockCompanyAdmins(ctx, company)
		if err != nil {
			t.Errorf("second lock: %v", err)
			got <- time.Time{}
			return
		}
		got <- time.Now()
		r2()
	}()
	time.Sleep(300 * time.Millisecond)
	released := time.Now()
	release()
	release() // idempotent
	select {
	case at := <-got:
		if at.Before(released) {
			t.Fatalf("second lock obtained before the first was released")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("second lock never obtained after release")
	}
}

func TestCompanyAdminLockName_FitsMySQLLimit(t *testing.T) {
	for _, id := range []string{"c_001", "41eab473-6cf4-4c53-aa2e-34f5d8e2495c", strings.Repeat("x", 100)} {
		if n := companyAdminLockName(id); len(n) > 64 {
			t.Errorf("lock name for %q is %d chars", id, len(n))
		}
	}
	if companyAdminLockName(strings.Repeat("x", 100)) == companyAdminLockName(strings.Repeat("y", 100)) {
		t.Error("long company ids must not share a lock name")
	}
}
