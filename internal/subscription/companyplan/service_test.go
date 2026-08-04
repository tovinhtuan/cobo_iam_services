package companyplan

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type errReader struct {
	err  error
	hits int
}

func (e *errReader) GetEffectivePlan(context.Context, string, time.Time) (*CompanyPlan, error) {
	e.hits++
	return nil, e.err
}

func (e *errReader) GetEffectivePlans(context.Context, []string, time.Time) (map[string]*CompanyPlan, error) {
	e.hits++
	return nil, e.err
}

type countingReader struct {
	inner Reader
	calls int
}

func (c *countingReader) GetEffectivePlan(ctx context.Context, companyID string, at time.Time) (*CompanyPlan, error) {
	c.calls++
	return c.inner.GetEffectivePlan(ctx, companyID, at)
}

func (c *countingReader) GetEffectivePlans(ctx context.Context, companyIDs []string, at time.Time) (map[string]*CompanyPlan, error) {
	c.calls++
	return c.inner.GetEffectivePlans(ctx, companyIDs, at)
}

func TestService_GetEffectivePlanSemantics(t *testing.T) {
	repo := NewMemoryRepository()
	svc := NewService(repo)
	ctx := context.Background()
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	at := time.Date(2026, 8, 4, 0, 0, 0, 0, time.UTC)

	if p, err := svc.GetEffectivePlan(ctx, "  ", at); err != nil || p != nil {
		t.Fatalf("empty company → nil, got %+v err=%v", p, err)
	}
	if p, err := svc.GetEffectivePlan(ctx, "c_missing", at); err != nil || p != nil {
		t.Fatalf("no-plan → nil, got %+v err=%v", p, err)
	}

	if err := repo.Create(ctx, CompanyPlan{
		ID: "p1", CompanyID: "c_001", Code: PlanCodePremium, Status: PlanStatusSuspended,
		EffectiveFrom: from, Origin: RecordOriginDevFixture,
	}); err != nil {
		t.Fatal(err)
	}
	got, err := svc.GetEffectivePlan(ctx, "c_001", at)
	if err != nil || got == nil || got.Status != PlanStatusSuspended {
		t.Fatalf("non-ACTIVE must keep real status, got %+v err=%v", got, err)
	}
	if got.WireSource() != PlanSourceCompanySubscription {
		t.Fatal("wire source")
	}
}

func TestService_BatchDedupEmptyNoQueryIsolation(t *testing.T) {
	repo := NewMemoryRepository()
	counter := &countingReader{inner: repo}
	svc := NewService(counter)
	ctx := context.Background()
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	at := time.Date(2026, 8, 4, 0, 0, 0, 0, time.UTC)

	out, err := svc.GetEffectivePlans(ctx, nil, at)
	if err != nil || len(out) != 0 {
		t.Fatalf("nil input: %+v err=%v", out, err)
	}
	out, err = svc.GetEffectivePlans(ctx, []string{"", "  "}, at)
	if err != nil || len(out) != 0 {
		t.Fatalf("blank ids: %+v err=%v", out, err)
	}
	if counter.calls != 0 {
		t.Fatalf("empty input must not call reader, calls=%d", counter.calls)
	}

	_ = repo.Create(ctx, CompanyPlan{
		ID: "p1", CompanyID: "c_001", Code: PlanCodePremium, Status: PlanStatusActive,
		EffectiveFrom: from, Origin: RecordOriginDevFixture,
	})
	_ = repo.Create(ctx, CompanyPlan{
		ID: "p2", CompanyID: "c_003", Code: PlanCodeEnterprise, Status: PlanStatusActive,
		EffectiveFrom: from, Origin: RecordOriginDevFixture,
	})

	m, err := svc.GetEffectivePlans(ctx, []string{"c_001", "c_001", "c_002", " c_001 "}, at)
	if err != nil {
		t.Fatal(err)
	}
	if counter.calls != 1 {
		t.Fatalf("want one batch call after dedupe, got %d", counter.calls)
	}
	if m["c_001"] == nil || m["c_002"] != nil || m["c_003"] != nil {
		t.Fatalf("isolation/no-fake: map keys=%v", keysOf(m))
	}
	if m["c_001"].CompanyID != "c_001" {
		t.Fatal("leak check failed")
	}
}

func TestService_PropagatesReaderError(t *testing.T) {
	want := errors.New("db_down")
	svc := NewService(&errReader{err: want})
	ctx := context.Background()
	at := time.Now().UTC()
	if _, err := svc.GetEffectivePlan(ctx, "c_001", at); !errors.Is(err, want) {
		t.Fatalf("single: want %v got %v", want, err)
	}
	if _, err := svc.GetEffectivePlans(ctx, []string{"c_001"}, at); !errors.Is(err, want) {
		t.Fatalf("batch: want %v got %v", want, err)
	}
}

func TestSelectEffectivePlan_UnknownCodeStillReturned(t *testing.T) {
	at := time.Date(2026, 8, 4, 0, 0, 0, 0, time.UTC)
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	rows := []CompanyPlan{{
		ID: "u1", CompanyID: "c1", Code: PlanCode("GOLD"), Status: PlanStatusActive,
		EffectiveFrom: from,
	}}
	got := SelectEffectivePlan(rows, at)
	if got == nil || got.Code != "GOLD" || ValidPlanCode(got.Code) {
		t.Fatalf("unknown covering code must still be returned for FE fail-close, got %+v", got)
	}
	if !ValidPlanStatus(got.Status) {
		t.Fatal("status should remain known ACTIVE")
	}
}

func keysOf(m map[string]*CompanyPlan) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func openCompanyPlanMySQL(t *testing.T) *sql.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("MYSQL_TEST_DSN"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("MYSQL_DSN"))
	}
	if dsn == "" {
		dsn = "root:secret@tcp(127.0.0.1:3306)/cobo_iam?parseTime=true&loc=UTC"
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Log("MYSQL_CONCURRENCY_VALIDATION_PENDING_PHASE_5")
		t.Skipf("mysql open: %v", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		t.Log("MYSQL_CONCURRENCY_VALIDATION_PENDING_PHASE_5")
		t.Skipf("mysql ping: %v — overlap concurrency not proven in this environment", err)
	}
	return db
}

// TestMySQLCreate_ConcurrentOverlap_EmptyCompany proves TX + parent companies FOR UPDATE
// serializes two concurrent overlapping inserts when the company has zero subscription rows.
func TestMySQLCreate_ConcurrentOverlap_EmptyCompany(t *testing.T) {
	db := openCompanyPlanMySQL(t)
	defer db.Close()

	ctx := context.Background()
	companyID := fmt.Sprintf("c_cps_conc_%d", time.Now().UnixNano())
	_, err := db.ExecContext(ctx, `
		INSERT INTO companies (company_id, company_code, company_name, status)
		VALUES (?, ?, 'companyplan concurrency', 'active')
		ON DUPLICATE KEY UPDATE company_name = VALUES(company_name)`,
		companyID, companyID)
	if err != nil {
		// Table or schema may be missing if 0125 not applied.
		t.Log("MYSQL_CONCURRENCY_VALIDATION_PENDING_PHASE_5")
		t.Skipf("cannot seed company (schema?): %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM company_subscriptions WHERE company_id = ?`, companyID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM companies WHERE company_id = ?`, companyID)
	})

	// Ensure table exists.
	var n int
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(1) FROM information_schema.tables
		WHERE table_schema = DATABASE() AND table_name = 'company_subscriptions'`).Scan(&n); err != nil || n == 0 {
		t.Log("MYSQL_CONCURRENCY_VALIDATION_PENDING_PHASE_5")
		t.Skip("company_subscriptions missing — apply 0125 before concurrency proof")
	}

	repo := NewMySQLRepository(db)
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		id := fmt.Sprintf("cps_conc_%d_%d", time.Now().UnixNano(), i)
		go func(planID string) {
			defer wg.Done()
			errs <- repo.Create(ctx, CompanyPlan{
				ID: planID, CompanyID: companyID, Code: PlanCodePremium, Status: PlanStatusActive,
				EffectiveFrom: from, Origin: RecordOrigin("concurrency_test"),
			})
		}(id)
	}
	wg.Wait()
	close(errs)

	var ok, overlap, other int
	for err := range errs {
		switch {
		case err == nil:
			ok++
		case errors.Is(err, ErrOverlap):
			overlap++
		default:
			other++
			t.Logf("unexpected create err: %v", err)
		}
	}
	if ok != 1 || overlap != 1 || other != 0 {
		t.Fatalf("concurrent empty-company overlap: ok=%d overlap=%d other=%d (want 1/1/0)", ok, overlap, other)
	}

	var ver string
	_ = db.QueryRowContext(ctx, `SELECT VERSION()`).Scan(&ver)
	t.Logf("mysql_version=%s isolation=REPEATABLE_READ lock=companies.FOR_UPDATE+occupying.FOR_UPDATE", ver)
}
