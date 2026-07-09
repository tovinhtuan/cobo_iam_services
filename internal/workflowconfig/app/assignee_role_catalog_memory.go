package app

import (
	"context"
	"sync"
)

type inMemoryAssigneeRoleCatalog struct {
	mu    sync.Mutex
	items []AssigneeRoleCatalogItem
}

func NewInMemoryAssigneeRoleCatalog() *inMemoryAssigneeRoleCatalog {
	return &inMemoryAssigneeRoleCatalog{items: []AssigneeRoleCatalogItem{}}
}

func (r *inMemoryAssigneeRoleCatalog) List(_ context.Context) ([]AssigneeRoleCatalogItem, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]AssigneeRoleCatalogItem, len(r.items))
	copy(out, r.items)
	return out, nil
}

func (r *inMemoryAssigneeRoleCatalog) Create(_ context.Context, item AssigneeRoleCatalogItem) (*AssigneeRoleCatalogItem, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := item
	r.items = append(r.items, out)
	return &out, nil
}
