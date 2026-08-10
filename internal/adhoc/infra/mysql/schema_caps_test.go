package mysql

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"
)

// TestHasProposedDeadlineDaysColumn_CachedFastPath_ConcurrentReadsAreRaceFree
// drives hasProposedDeadlineDaysColumn from many goroutines once the result is
// already cached. The cached branch (RLock → read → RUnlock) must be safe to
// call concurrently with no data race on deadlineDaysColCached/deadlineDaysColOK
// — exactly the "thread-safe" property CF-16 requires of the replacement for
// the old sync.Once. r.db is intentionally left nil: if any goroutine fell
// through to the query path the test would panic on a nil *sql.DB, so a clean
// pass also proves every call took the cached fast path (no redundant queries).
func TestHasProposedDeadlineDaysColumn_CachedFastPath_ConcurrentReadsAreRaceFree(t *testing.T) {
	repo := &Repository{}
	repo.deadlineDaysColCached = true
	repo.deadlineDaysColOK = true

	const goroutines = 32
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			ok, err := repo.hasProposedDeadlineDaysColumn(context.Background())
			if err != nil {
				t.Errorf("cached lookup must not return an error, got %v", err)
				return
			}
			if !ok {
				t.Errorf("cached lookup must return the cached value (true), got false")
			}
		}()
	}
	wg.Wait()
}

// TestHasProposedDeadlineDaysColumn_TransientErrorPath_DoesNotCache is a
// source-level regression guard for CF-16 (mirrors the readRepositorySrc
// convention used by repository_has_workflow_test.go in the disclosure
// package, since this codebase has no SQL-mocking library to drive a real
// transient-then-success sequence through *sql.DB).
//
// It asserts the fix's core invariant directly from the source: the lookup
// error is checked and returned (false, err) *before* any write to
// deadlineDaysColCached, so a transient information_schema failure can never
// poison the cache — only a successful lookup commits deadlineDaysColCached=true.
// It also asserts the old permanent-poisoning sync.Once pattern is gone.
func TestHasProposedDeadlineDaysColumn_TransientErrorPath_DoesNotCache(t *testing.T) {
	src := readSchemaCapsSrc(t)

	fnIdx := strings.Index(src, "func (r *Repository) hasProposedDeadlineDaysColumn(")
	if fnIdx < 0 {
		t.Fatal("expected to find func (r *Repository) hasProposedDeadlineDaysColumn in schema_caps.go")
	}
	body := src[fnIdx:]

	if strings.Contains(body, "sync.Once") {
		t.Fatal("hasProposedDeadlineDaysColumn must not use sync.Once — CF-16: a sync.Once-based cache permanently poisons every future caller on the first transient lookup error")
	}

	scanIdx := strings.Index(body, "Scan(&count)")
	cacheWriteIdx := strings.Index(body, "deadlineDaysColCached = true")
	if scanIdx < 0 {
		t.Fatal("expected hasProposedDeadlineDaysColumn to perform the information_schema lookup via Scan(&count)")
	}
	if cacheWriteIdx < 0 {
		t.Fatal("expected hasProposedDeadlineDaysColumn to cache a successful lookup via deadlineDaysColCached = true")
	}
	if cacheWriteIdx < scanIdx {
		t.Fatal("deadlineDaysColCached must be set after the lookup completes, not before — caching before the result is known would cache an unknown/failed state")
	}

	between := body[scanIdx:cacheWriteIdx]
	if !strings.Contains(between, "if err != nil") || !strings.Contains(between, "return false, err") {
		t.Fatal("CF-16: a lookup error must return early as (false, err) WITHOUT writing deadlineDaysColCached — only a *successful* lookup may be cached, so a transient failure (context cancellation, connection blip) is retried by the next caller instead of permanently poisoning every future caller")
	}
}

func TestHasProposedDeadlineDayTypeColumn_CachedFastPath_ConcurrentReadsAreRaceFree(t *testing.T) {
	repo := &Repository{}
	repo.deadlineDayTypeColCached = true
	repo.deadlineDayTypeColOK = true

	const goroutines = 32
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			ok, err := repo.hasProposedDeadlineDayTypeColumn(context.Background())
			if err != nil {
				t.Errorf("cached lookup must not return an error, got %v", err)
				return
			}
			if !ok {
				t.Errorf("cached lookup must return the cached value (true), got false")
			}
		}()
	}
	wg.Wait()
}

func TestHasProposedDeadlineDayTypeColumn_TransientErrorPath_DoesNotCache(t *testing.T) {
	src := readSchemaCapsSrc(t)
	fnIdx := strings.Index(src, "func (r *Repository) hasProposedDeadlineDayTypeColumn(")
	if fnIdx < 0 {
		t.Fatal("expected hasProposedDeadlineDayTypeColumn in schema_caps.go")
	}
	body := src[fnIdx:]
	if strings.Contains(body[:strings.Index(body, "func (r *Repository) loadProposalSchemaCaps")], "sync.Once") {
		t.Fatal("hasProposedDeadlineDayTypeColumn must not use sync.Once")
	}
	scanIdx := strings.Index(body, "Scan(&count)")
	cacheWriteIdx := strings.Index(body, "deadlineDayTypeColCached = true")
	if scanIdx < 0 || cacheWriteIdx < 0 || cacheWriteIdx < scanIdx {
		t.Fatal("day type column cache must write only after successful Scan")
	}
	between := body[scanIdx:cacheWriteIdx]
	if !strings.Contains(between, "if err != nil") || !strings.Contains(between, "return false, err") {
		t.Fatal("day type column: transient lookup error must not cache")
	}
}

func readSchemaCapsSrc(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("schema_caps.go")
	if err != nil {
		t.Fatalf("read schema_caps.go: %v", err)
	}
	return string(data)
}
