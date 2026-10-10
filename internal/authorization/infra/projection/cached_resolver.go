package projection

import (
	"context"
	"fmt"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
)

type CachedResolver struct {
	base  authapp.Resolver
	store SnapshotStore
}

func NewCachedResolver(base authapp.Resolver, store SnapshotStore) *CachedResolver {
	return &CachedResolver{base: base, store: store}
}

func (r *CachedResolver) Resolve(ctx context.Context, membershipID, companyID string) (*authapp.EffectiveAccessSummary, error) {
	v, gen, ok := r.store.Get(ctx, membershipID, companyID)
	if ok {
		return v, nil
	}
	// gen was read before the database: if an invalidation lands while we resolve, this
	// entry is written at an old generation and is never served.
	resolved, err := r.base.Resolve(ctx, membershipID, companyID)
	if err != nil {
		return nil, fmt.Errorf("resolve base effective access: %w", err)
	}
	r.store.Put(ctx, resolved, gen)
	return resolved, nil
}
