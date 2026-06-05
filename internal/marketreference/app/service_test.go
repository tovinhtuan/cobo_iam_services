package app

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

type fakeListedRepo struct {
	listFn              func(context.Context, ListParams) (ListResult, error)
	getFn               func(context.Context, string) (ListedCompanyDetail, error)
	getByBusinessCodeFn func(context.Context, string) (ListedCompanyDetail, error)
	callCount           int // counts GetByBusinessCode calls for cache verification
}

func (f *fakeListedRepo) List(ctx context.Context, p ListParams) (ListResult, error) {
	if f.listFn != nil {
		return f.listFn(ctx, p)
	}
	return ListResult{}, nil
}

func (f *fakeListedRepo) GetBySymbol(ctx context.Context, symbol string) (ListedCompanyDetail, error) {
	if f.getFn != nil {
		return f.getFn(ctx, symbol)
	}
	return ListedCompanyDetail{}, nil
}

func (f *fakeListedRepo) GetByBusinessCode(ctx context.Context, businessCode string) (ListedCompanyDetail, error) {
	f.callCount++
	if f.getByBusinessCodeFn != nil {
		return f.getByBusinessCodeFn(ctx, businessCode)
	}
	return ListedCompanyDetail{}, ErrNotFound
}

func TestService_DisabledUnavailable(t *testing.T) {
	svc := NewDisabledService()
	_, err := svc.List(context.Background(), ListParams{})
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("List err = %v, want ErrUnavailable", err)
	}
	_, err = svc.GetDetail(context.Background(), "VIC")
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("GetDetail err = %v, want ErrUnavailable", err)
	}
}

func TestService_PingFailureUnavailable(t *testing.T) {
	svc := NewService(&fakeListedRepo{}, func(context.Context) error {
		return errors.New("db down")
	})
	_, err := svc.List(context.Background(), ListParams{})
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("List err = %v", err)
	}
}

func TestService_GetDetailNotFound(t *testing.T) {
	svc := NewService(&fakeListedRepo{
		getFn: func(context.Context, string) (ListedCompanyDetail, error) {
			return ListedCompanyDetail{}, ErrNotFound
		},
	}, nil)
	_, err := svc.GetDetail(context.Background(), "ZZZ")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v", err)
	}
}

// ---- GetByBusinessCode tests ----

func TestService_GetByBusinessCode_Disabled(t *testing.T) {
	svc := NewDisabledService()
	_, _, err := svc.GetByBusinessCode(context.Background(), "0101234567")
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("disabled: err = %v, want ErrUnavailable", err)
	}
}

func TestService_GetByBusinessCode_EmptyCode(t *testing.T) {
	svc := NewService(&fakeListedRepo{}, nil)
	_, _, err := svc.GetByBusinessCode(context.Background(), "")
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("empty code: err = %v, want ErrInvalidRequest", err)
	}
}

func TestService_GetByBusinessCode_WhitespaceCode(t *testing.T) {
	svc := NewService(&fakeListedRepo{}, nil)
	_, _, err := svc.GetByBusinessCode(context.Background(), "   ")
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("whitespace code: err = %v, want ErrInvalidRequest", err)
	}
}

func TestService_GetByBusinessCode_Found(t *testing.T) {
	want := ListedCompanyDetail{Symbol: "VNM", CompanyName: "Vinamilk", HasProfile: true}
	repo := &fakeListedRepo{
		getByBusinessCodeFn: func(_ context.Context, code string) (ListedCompanyDetail, error) {
			if code != "0300588569" {
				return ListedCompanyDetail{}, ErrNotFound
			}
			return want, nil
		},
	}
	svc := NewService(repo, nil)
	got, hit, err := svc.GetByBusinessCode(context.Background(), "0300588569")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if hit {
		t.Fatal("first call must be cache miss")
	}
	if got.Symbol != want.Symbol {
		t.Fatalf("symbol = %q, want %q", got.Symbol, want.Symbol)
	}
}

func TestService_GetByBusinessCode_NotFound(t *testing.T) {
	svc := NewService(&fakeListedRepo{}, nil)
	_, _, err := svc.GetByBusinessCode(context.Background(), "9999999999")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestService_GetByBusinessCode_CacheHit(t *testing.T) {
	want := ListedCompanyDetail{Symbol: "VNM", CompanyName: "Vinamilk", HasProfile: true}
	repo := &fakeListedRepo{
		getByBusinessCodeFn: func(_ context.Context, _ string) (ListedCompanyDetail, error) {
			return want, nil
		},
	}
	svc := NewService(repo, nil)
	ctx := context.Background()

	_, hit1, _ := svc.GetByBusinessCode(ctx, "0300588569")
	if hit1 {
		t.Fatal("first call: expected cache miss")
	}
	_, hit2, err := svc.GetByBusinessCode(ctx, "0300588569")
	if err != nil {
		t.Fatalf("second call err = %v", err)
	}
	if !hit2 {
		t.Fatal("second call: expected cache hit")
	}
	if repo.callCount != 1 {
		t.Fatalf("repo called %d times, want 1", repo.callCount)
	}
}

func TestService_GetByBusinessCode_NegativeCacheHit(t *testing.T) {
	repo := &fakeListedRepo{} // always returns ErrNotFound
	svc := NewService(repo, nil)
	ctx := context.Background()

	_, _, err1 := svc.GetByBusinessCode(ctx, "0000000000")
	if !errors.Is(err1, ErrNotFound) {
		t.Fatalf("first call: err = %v", err1)
	}
	_, _, err2 := svc.GetByBusinessCode(ctx, "0000000000")
	if !errors.Is(err2, ErrNotFound) {
		t.Fatalf("second call: err = %v", err2)
	}
	if repo.callCount != 1 {
		t.Fatalf("repo called %d times, want 1 (negative cache)", repo.callCount)
	}
}

func TestService_GetByBusinessCode_TTLExpiry(t *testing.T) {
	want := ListedCompanyDetail{Symbol: "VNM", HasProfile: true}
	repo := &fakeListedRepo{
		getByBusinessCodeFn: func(_ context.Context, _ string) (ListedCompanyDetail, error) {
			return want, nil
		},
	}
	svc := NewService(repo, nil)
	ctx := context.Background()

	// Manually set a very short TTL by inserting an already-expired entry.
	svc.cache.mu.Lock()
	svc.cache.entries["0300588569"] = cacheEntry{
		result: &want,
		expiry: time.Now().Add(-1 * time.Second), // already expired
	}
	svc.cache.mu.Unlock()

	_, hit, err := svc.GetByBusinessCode(ctx, "0300588569")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if hit {
		t.Fatal("expired entry must not be a cache hit")
	}
	if repo.callCount != 1 {
		t.Fatalf("repo called %d times after expiry, want 1", repo.callCount)
	}
}

func TestService_GetByBusinessCode_ConcurrentMissSameKey(t *testing.T) {
	// Multiple goroutines all miss cache for the same key.
	// Repo should be called at most a small number of times (not once per goroutine).
	want := ListedCompanyDetail{Symbol: "VNM", HasProfile: true}
	repo := &fakeListedRepo{
		getByBusinessCodeFn: func(_ context.Context, _ string) (ListedCompanyDetail, error) {
			return want, nil
		},
	}
	svc := NewService(repo, nil)
	ctx := context.Background()

	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			_, _, _ = svc.GetByBusinessCode(ctx, "0300588569")
		}()
	}
	wg.Wait()

	// With double-checked locking, repo is called at most a few times (not 20).
	// In the worst case (all goroutines pass RLock before any write), all could call repo.
	// But after the first write, subsequent goroutines should hit double-check.
	// We assert the cache is now populated and the value is correct.
	got, hit, err := svc.GetByBusinessCode(ctx, "0300588569")
	if err != nil {
		t.Fatalf("after concurrent: err = %v", err)
	}
	if !hit {
		t.Fatal("after concurrent writes, cache must be warm")
	}
	if got.Symbol != want.Symbol {
		t.Fatalf("symbol = %q, want %q", got.Symbol, want.Symbol)
	}
}

func TestService_GetByBusinessCode_CacheEvictsOldest(t *testing.T) {
	svc := NewService(&fakeListedRepo{
		getByBusinessCodeFn: func(_ context.Context, code string) (ListedCompanyDetail, error) {
			return ListedCompanyDetail{Symbol: "X", HasProfile: true}, nil
		},
	}, nil)
	// Fill cache to maxSize with entries expiring far in the future.
	svc.cache.mu.Lock()
	for i := 0; i < cacheMaxSize; i++ {
		key := fmt.Sprintf("CODE%04d", i)
		d := ListedCompanyDetail{Symbol: key}
		svc.cache.entries[key] = cacheEntry{result: &d, expiry: time.Now().Add(cachePositiveTTL)}
	}
	svc.cache.mu.Unlock()

	// Adding one more should evict oldest without panic.
	_, _, err := svc.GetByBusinessCode(context.Background(), "NEWCODE")
	if err != nil {
		t.Fatalf("eviction test: err = %v", err)
	}
	svc.cache.mu.RLock()
	size := len(svc.cache.entries)
	svc.cache.mu.RUnlock()
	if size > cacheMaxSize {
		t.Fatalf("cache size %d exceeds maxSize %d", size, cacheMaxSize)
	}
}

func TestService_ListSuccess(t *testing.T) {
	now := time.Now().UTC()
	svc := NewService(&fakeListedRepo{
		listFn: func(_ context.Context, p ListParams) (ListResult, error) {
			if p.Limit != 20 {
				t.Fatalf("limit = %d", p.Limit)
			}
			return ListResult{
				Items: []ListedCompanySummary{{Symbol: "VIC", CompanyName: "Vingroup", Source: ProfileSourceKBS, HasProfile: true, ProfileUpdatedAt: &now}},
				Total: 1,
			}, nil
		},
	}, nil)
	out, err := svc.List(context.Background(), ListParams{})
	if err != nil {
		t.Fatal(err)
	}
	if out.Total != 1 || out.Items[0].Symbol != "VIC" {
		t.Fatalf("got %+v", out)
	}
}
