package inmemory

import (
	"context"
	"sort"
	"sync"

	notificationapp "github.com/cobo/cobo_iam_services/internal/notification/app"
)

type Repository struct {
	mu         sync.RWMutex
	jobs       map[string]notificationapp.NotificationJobDTO
	deliveries map[string]notificationapp.NotificationDeliveryDTO
}

func NewRepository() *Repository {
	return &Repository{jobs: map[string]notificationapp.NotificationJobDTO{}, deliveries: map[string]notificationapp.NotificationDeliveryDTO{}}
}

func key(companyID, id string) string { return companyID + ":" + id }

func (r *Repository) CreateJob(_ context.Context, job notificationapp.NotificationJobDTO) (*notificationapp.NotificationJobDTO, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.jobs[key(job.CompanyID, job.NotificationJobID)] = job
	cp := job
	return &cp, nil
}

func (r *Repository) ListPendingJobs(_ context.Context, companyID string, limit int) ([]notificationapp.NotificationJobDTO, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]notificationapp.NotificationJobDTO, 0)
	for _, j := range r.jobs {
		if j.CompanyID == companyID && j.Status == "pending" {
			out = append(out, j)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].NotificationJobID < out[j].NotificationJobID })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (r *Repository) UpdateJobStatus(_ context.Context, companyID, jobID, status string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := key(companyID, jobID)
	job, ok := r.jobs[k]
	if !ok {
		return nil
	}
	job.Status = status
	r.jobs[k] = job
	return nil
}

func (r *Repository) CreateDelivery(_ context.Context, d notificationapp.NotificationDeliveryDTO) (*notificationapp.NotificationDeliveryDTO, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.deliveries[d.NotificationDeliveryID] = d
	cp := d
	return &cp, nil
}
