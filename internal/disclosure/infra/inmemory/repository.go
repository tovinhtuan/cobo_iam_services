package inmemory

import (
	"context"
	"net/http"
	"sync"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

type Repository struct {
	mu    sync.RWMutex
	items map[string]disclosureapp.RecordDTO
}

func NewRepository() *Repository {
	return &Repository{items: map[string]disclosureapp.RecordDTO{}}
}

func key(companyID, recordID string) string { return companyID + ":" + recordID }

func (r *Repository) Create(_ context.Context, rec disclosureapp.RecordDTO) (*disclosureapp.RecordDTO, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items[key(rec.CompanyID, rec.RecordID)] = rec
	cp := rec
	return &cp, nil
}

func (r *Repository) Update(_ context.Context, rec disclosureapp.RecordDTO) (*disclosureapp.RecordDTO, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := key(rec.CompanyID, rec.RecordID)
	if _, ok := r.items[k]; !ok {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "record not found", nil)
	}
	r.items[k] = rec
	cp := rec
	return &cp, nil
}

func (r *Repository) FindByID(_ context.Context, companyID, recordID string) (*disclosureapp.RecordDTO, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	it, ok := r.items[key(companyID, recordID)]
	if !ok {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "record not found", nil)
	}
	cp := it
	return &cp, nil
}

func (r *Repository) List(_ context.Context, companyID string) ([]disclosureapp.RecordDTO, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]disclosureapp.RecordDTO, 0)
	for _, it := range r.items {
		if it.CompanyID == companyID {
			out = append(out, it)
		}
	}
	return out, nil
}
