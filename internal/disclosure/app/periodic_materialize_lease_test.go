package app

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// leaseFakeRepo adds the stale-claim reaper and records the ctx state at completion.
type leaseFakeRepo struct {
	*fakePeriodicRepo
	reaps          int
	reapOlderThan  int
	completeCtxErr error
	retryCtxErr    error
	failedCodes    map[string]string
}

func (r *leaseFakeRepo) RequeueStalePeriodicClaims(_ context.Context, olderThanSec int) (int64, error) {
	r.reaps++
	r.reapOlderThan = olderThanSec
	return 0, nil
}

func (r *leaseFakeRepo) UpdateCycleRecord(ctx context.Context, cycleID, recordID string) error {
	r.completeCtxErr = ctx.Err()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return r.fakePeriodicRepo.UpdateCycleRecord(ctx, cycleID, recordID)
}

func (r *leaseFakeRepo) MarkPeriodicCycleRetry(ctx context.Context, cycleID, attemptRecordID, code, msg string) error {
	r.retryCtxErr = ctx.Err()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return r.fakePeriodicRepo.MarkPeriodicCycleRetry(ctx, cycleID, attemptRecordID, code, msg)
}

func (r *leaseFakeRepo) MarkPeriodicCycleFailed(ctx context.Context, cycleID, attemptRecordID, code, msg string) error {
	if r.failedCodes == nil {
		r.failedCodes = map[string]string{}
	}
	r.failedCodes[cycleID] = code
	return r.fakePeriodicRepo.MarkPeriodicCycleFailed(ctx, cycleID, attemptRecordID, code, msg)
}

// cancelingCreator cancels the tick context (SIGTERM) while the record is being created.
type cancelingCreator struct {
	fakePeriodicCreator
	cancel context.CancelFunc
}

func (c *cancelingCreator) CreateAndSubmitPeriodicRecord(ctx context.Context, cycleID, companyID, typeID, by, title string, t0 *time.Time, planned string) (string, string, error) {
	c.cancel()
	return c.fakePeriodicCreator.CreateAndSubmitPeriodicRecord(ctx, cycleID, companyID, typeID, by, title, t0, planned)
}

func leaseCycle(id string) PeriodicCycleRow {
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	return PeriodicCycleRow{CycleID: id, TypeID: "t1", CompanyID: "c1", CycleLabel: "2026-09", CycleStart: start, DueDate: start.AddDate(0, 0, 10)}
}

// PER-04: a shutdown signal after the claim must not strand the cycle in CLAIMED; the claimed
// item finishes on a context detached from the tick (bounded by its own timeout).
func TestMaterializePeriodic_CancelAfterClaimStillFinalizes(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	repo := &leaseFakeRepo{fakePeriodicRepo: newFakePeriodicRepo([]PeriodicCycleRow{leaseCycle("cy-1")})}
	creator := &cancelingCreator{fakePeriodicCreator: fakePeriodicCreator{recordID: "rec-1", workflowInstanceID: "wf-1"}, cancel: cancel}
	if _, err := materializePeriodicDisclosures(ctx, time.Date(2026, 9, 10, 3, 0, 0, 0, time.UTC), repo, creator); err != nil {
		t.Fatalf("materialize: %v", err)
	}
	if repo.completeCtxErr != nil {
		t.Fatalf("completion ran on a cancelled context: %v", repo.completeCtxErr)
	}
	if repo.completed["cy-1"] != "rec-1" {
		t.Fatalf("cycle not completed, completed=%v claimed=%v", repo.completed, repo.claimed)
	}
}

// PER-04: every run first requeues claims older than the lease.
func TestMaterializePeriodic_ReapsStaleClaims(t *testing.T) {
	repo := &leaseFakeRepo{fakePeriodicRepo: newFakePeriodicRepo(nil)}
	creator := &fakePeriodicCreator{}
	if _, err := materializePeriodicDisclosures(context.Background(), time.Now(), repo, creator); err != nil {
		t.Fatal(err)
	}
	if repo.reaps != 1 || repo.reapOlderThan != int(PeriodicClaimLease/time.Second) {
		t.Fatalf("reaper calls=%d olderThan=%d", repo.reaps, repo.reapOlderThan)
	}
}

// PER-03: permanent gate errors (400/403/404/422) fail the cycle instead of retrying forever;
// retryable ones (409/429, generic) keep RETRY.
func TestMaterializePeriodic_PermanentGateErrorMarksFailed(t *testing.T) {
	cases := []struct {
		status   int
		wantFail bool
	}{
		{http.StatusBadRequest, true}, {http.StatusForbidden, true}, {http.StatusNotFound, true},
		{http.StatusUnprocessableEntity, true}, {http.StatusConflict, false}, {http.StatusTooManyRequests, false},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprint(tc.status), func(t *testing.T) {
			repo := &leaseFakeRepo{fakePeriodicRepo: newFakePeriodicRepo([]PeriodicCycleRow{leaseCycle("cy-1")})}
			gateErr := fmt.Errorf("create: %w", perr.NewHTTPError(tc.status, perr.CodeInvalidRequest, "gate", nil))
			creator := &fakePeriodicCreator{err: gateErr}
			if _, err := materializePeriodicDisclosures(context.Background(), time.Date(2026, 9, 10, 3, 0, 0, 0, time.UTC), repo, creator); err != nil {
				t.Fatal(err)
			}
			_, failed := repo.failed["cy-1"]
			_, retried := repo.retries["cy-1"]
			if failed != tc.wantFail || retried == tc.wantFail {
				t.Fatalf("status %d: failed=%v retried=%v", tc.status, failed, retried)
			}
			if tc.wantFail && repo.failedCodes["cy-1"] != fmt.Sprintf("GATE_%d", tc.status) {
				t.Fatalf("failure code=%q", repo.failedCodes["cy-1"])
			}
		})
	}
}

// PER-03 / PER-04: SQL guards for the capped retry and the stale-claim reaper.
func TestPeriodicRetryAndReaperSQL(t *testing.T) {
	data, err := os.ReadFile("../infra/mysql/repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := strings.ReplaceAll(string(data), "\r\n", "\n")
	retry := src[strings.Index(src, "func (r *Repository) MarkPeriodicCycleRetry("):]
	retry = retry[:strings.Index(retry, "\n}\n")]
	for _, want := range []string{"IF(attempt_count >= ", "'RETRY_EXHAUSTED'", "LEAST(3600"} {
		if !strings.Contains(retry, want) {
			t.Fatalf("MarkPeriodicCycleRetry must contain %q", want)
		}
	}
	i := strings.Index(src, "func (r *Repository) RequeueStalePeriodicClaims(")
	if i < 0 {
		t.Fatal("RequeueStalePeriodicClaims missing")
	}
	reap := src[i:]
	reap = reap[:strings.Index(reap, "\n}\n")]
	for _, want := range []string{"materialization_state = 'CLAIMED'", "record_id IS NULL", "NOW(3) - INTERVAL ? SECOND", "'CLAIM_EXPIRED'", "IF(attempt_count >= ", "'RETRY_EXHAUSTED'"} {
		if !strings.Contains(reap, want) {
			t.Fatalf("RequeueStalePeriodicClaims must contain %q", want)
		}
	}
}

// slowFailingCreator outlives the item timeout and then fails.
type slowFailingCreator struct{ fakePeriodicCreator }

func (c *slowFailingCreator) CreateAndSubmitPeriodicRecord(ctx context.Context, _, _, _, _, _ string, _ *time.Time, _ string) (string, string, error) {
	<-ctx.Done()
	return "", "", ctx.Err()
}

// PERF-01: when the create call runs past the item timeout, the state transition must still be
// written (on its own short context), so the cycle reaches RETRY/FAILED instead of staying
// CLAIMED until the reaper.
func TestMaterializePeriodic_TimeoutStillRecordsRetry(t *testing.T) {
	prev := periodicItemTimeout
	periodicItemTimeout = 20 * time.Millisecond
	t.Cleanup(func() { periodicItemTimeout = prev })
	repo := &leaseFakeRepo{fakePeriodicRepo: newFakePeriodicRepo([]PeriodicCycleRow{leaseCycle("cy-1")})}
	if _, err := materializePeriodicDisclosures(context.Background(), time.Date(2026, 9, 10, 3, 0, 0, 0, time.UTC), repo, &slowFailingCreator{}); err != nil {
		t.Fatalf("materialize: %v", err)
	}
	if repo.retryCtxErr != nil {
		t.Fatalf("retry transition ran on the expired item context: %v", repo.retryCtxErr)
	}
	if _, ok := repo.retries["cy-1"]; !ok {
		t.Fatal("cycle must be marked RETRY after a timed-out create")
	}
}
