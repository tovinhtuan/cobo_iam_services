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
	if v, ok := r.store.Get(ctx, membershipID, companyID); ok {
		return v, nil
	}
	resolved, err := r.base.Resolve(ctx, membershipID, companyID)
	if err != nil {
		return nil, fmt.Errorf("resolve base effective access: %w", err)
	}
	r.store.Put(ctx, resolved)
	return resolved, nil
}
