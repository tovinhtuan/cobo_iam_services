package inmemory

import (
	"context"
	"net/http"
	"sync"
	"time"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	wdt "github.com/cobo/cobo_iam_services/internal/workflowdoctemplate"
)

// Repository is an in-memory Asset store for tests.
type Repository struct {
	mu     sync.RWMutex
	byID   map[string]wdt.Asset
}

// NewRepository creates an empty in-memory repository.
func NewRepository() *Repository {
	return &Repository{byID: make(map[string]wdt.Asset)}
}

func (r *Repository) Create(_ context.Context, asset wdt.Asset) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.byID[asset.FileID]; ok {
		return perr.NewHTTPError(http.StatusConflict, perr.CodeInvalidRequest, "file already exists", nil)
	}
	r.byID[asset.FileID] = asset
	return nil
}

func (r *Repository) GetByFileID(_ context.Context, fileID string) (*wdt.Asset, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	asset, ok := r.byID[fileID]
	if !ok {
		return nil, nil
	}
	cp := asset
	return &cp, nil
}

func (r *Repository) MarkDeleted(_ context.Context, fileID string, deletedAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	asset, ok := r.byID[fileID]
	if !ok {
		return perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "file not found", nil)
	}
	asset.State = wdt.StateDeleted
	asset.DeletedAt = &deletedAt
	r.byID[fileID] = asset
	return nil
}
