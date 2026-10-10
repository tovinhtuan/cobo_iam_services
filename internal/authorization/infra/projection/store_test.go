package projection

import (
	"context"
	"encoding/json"
	"testing"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
)

type scriptedResolver struct {
	calls  int
	perms  []string
	during func() // runs inside Resolve, after the database read
}

func (r *scriptedResolver) Resolve(_ context.Context, membershipID, companyID string) (*authapp.EffectiveAccessSummary, error) {
	r.calls++
	out := &authapp.EffectiveAccessSummary{MembershipID: membershipID, CompanyID: companyID, Permissions: append([]string(nil), r.perms...)}
	if r.during != nil {
		r.during()
		r.during = nil
	}
	return out, nil
}

// CACHE-11: a request that read the database before a permission change committed must
// not put the old access back into the cache after the change invalidated it.
func TestCachedResolver_SnapshotResolvedBeforeInvalidationIsNotServed(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryStore(0)
	base := &scriptedResolver{perms: []string{"rbac.manage"}}
	base.during = func() {
		base.perms = nil // the revoke commits while the old read is in flight
		if err := store.InvalidateCompany(ctx, "c-1"); err != nil {
			t.Fatal(err)
		}
	}
	r := NewCachedResolver(base, store)
	if _, err := r.Resolve(ctx, "m-1", "c-1"); err != nil {
		t.Fatal(err)
	}
	got, err := r.Resolve(ctx, "m-1", "c-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Permissions) != 0 {
		t.Fatalf("revoked permission served from cache: %v", got.Permissions)
	}
	if base.calls != 2 {
		t.Fatalf("base resolver calls = %d, want 2 (stale entry must be a miss)", base.calls)
	}
}

func TestCachedResolver_HitWithoutInvalidation(t *testing.T) {
	ctx := context.Background()
	base := &scriptedResolver{perms: []string{"company.view"}}
	r := NewCachedResolver(base, NewInMemoryStore(0))
	for i := 0; i < 3; i++ {
		if _, err := r.Resolve(ctx, "m-1", "c-1"); err != nil {
			t.Fatal(err)
		}
	}
	if base.calls != 1 {
		t.Fatalf("base resolver calls = %d, want 1", base.calls)
	}
}

func TestInMemoryStore_InvalidateCompanyDropsOnlyThatCompany(t *testing.T) {
	ctx := context.Background()
	s := NewInMemoryStore(0)
	for _, k := range [][2]string{{"m-1", "c-1"}, {"m-2", "c-1"}, {"m-3", "c-2"}} {
		_, gen, _ := s.Get(ctx, k[0], k[1])
		s.Put(ctx, &authapp.EffectiveAccessSummary{MembershipID: k[0], CompanyID: k[1]}, gen)
	}
	if err := s.InvalidateCompany(ctx, "c-1"); err != nil {
		t.Fatal(err)
	}
	for _, k := range [][2]string{{"m-1", "c-1"}, {"m-2", "c-1"}} {
		if _, _, ok := s.Get(ctx, k[0], k[1]); ok {
			t.Errorf("%s@%s still cached after company invalidation", k[0], k[1])
		}
	}
	if _, _, ok := s.Get(ctx, "m-3", "c-2"); !ok {
		t.Error("other company's entry must stay cached")
	}
}

func TestDecodeRedisEntry_GenerationMustMatch(t *testing.T) {
	raw, err := json.Marshal(redisEntry{Gen: 3, Access: authapp.EffectiveAccessSummary{MembershipID: "m-1", CompanyID: "c-1", Permissions: []string{"x"}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := decodeRedisEntry(3, raw); !ok {
		t.Error("entry written at the current generation must be a hit")
	}
	if _, ok := decodeRedisEntry(4, raw); ok {
		t.Error("entry written before an invalidation must be a miss")
	}
	if _, ok := decodeRedisEntry(3, []byte(`{"company_id":"c-1","permissions":["x"]}`)); ok {
		t.Error("an entry without the generation envelope must be a miss")
	}
}

// PERF-31: generations are fresh random tokens, so a lost generation key cannot be re-created with
// a value that an old entry still carries.
func TestNewGenerationToken_IsNotReused(t *testing.T) {
	seen := map[int64]bool{}
	for i := 0; i < 1000; i++ {
		g := newGenerationToken()
		if g <= 0 {
			t.Fatalf("generation token must be positive, got %d", g)
		}
		seen[g] = true
	}
	if len(seen) != 1000 {
		t.Fatalf("generation tokens repeated: %d distinct of 1000", len(seen))
	}
}

func TestParseRedisGeneration(t *testing.T) {
	cases := map[string]struct {
		in   any
		want int64
		ok   bool
	}{
		"number":  {"1760000000000000000", 1760000000000000000, true},
		"garbage": {"x", 0, false},
		"type":    {int64(7), 0, false},
	}
	for name, c := range cases {
		got, ok := parseRedisGeneration(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("%s: got (%d,%v) want (%d,%v)", name, got, ok, c.want, c.ok)
		}
	}
}
