package workflowstepcomments

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"
)

// MemoryRepository is an in-memory store for unit tests.
type MemoryRepository struct {
	mu       sync.Mutex
	comments map[string]Comment
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{comments: map[string]Comment{}}
}

func (r *MemoryRepository) ListAliveByStep(_ context.Context, owner OwnerContext, page, pageSize int) ([]Comment, int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	alive := make([]Comment, 0)
	for _, c := range r.comments {
		if c.MatchesOwner(owner) && c.IsAlive() {
			alive = append(alive, c)
		}
	}
	sort.SliceStable(alive, func(i, j int) bool {
		if !alive[i].CreatedAt.Equal(alive[j].CreatedAt) {
			return alive[i].CreatedAt.Before(alive[j].CreatedAt)
		}
		return alive[i].ID < alive[j].ID
	})
	total := len(alive)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = DefaultPageSize
	}
	start := (page - 1) * pageSize
	if start >= total {
		return []Comment{}, total, nil
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	out := make([]Comment, end-start)
	copy(out, alive[start:end])
	return out, total, nil
}

func (r *MemoryRepository) GetByIDInContext(_ context.Context, owner OwnerContext, commentID string) (*Comment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.comments[commentID]
	if !ok || c.CompanyID != owner.CompanyID {
		return nil, nil
	}
	if !c.MatchesOwner(owner) {
		return nil, errPathMismatch{}
	}
	cp := c
	return &cp, nil
}

func (r *MemoryRepository) Insert(_ context.Context, c Comment) error {
	return r.InsertInTx(context.Background(), nil, c)
}

func (r *MemoryRepository) InsertInTx(_ context.Context, _ DBTX, c Comment) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.comments[c.ID] = c
	return nil
}

func (r *MemoryRepository) UpdateBody(_ context.Context, owner OwnerContext, commentID, body string, updatedAt time.Time) error {
	return r.UpdateBodyInTx(context.Background(), nil, owner, commentID, body, updatedAt)
}

func (r *MemoryRepository) UpdateBodyInTx(_ context.Context, _ DBTX, owner OwnerContext, commentID, body string, updatedAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.comments[commentID]
	if !ok || c.CompanyID != owner.CompanyID || !c.MatchesOwner(owner) || !c.IsAlive() {
		return errNotFound{}
	}
	c.Body = body
	c.UpdatedAt = &updatedAt
	r.comments[commentID] = c
	return nil
}

// SnapshotComments copies current comment map (for memory unit-of-work rollback).
func (r *MemoryRepository) SnapshotComments() map[string]Comment {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make(map[string]Comment, len(r.comments))
	for k, v := range r.comments {
		out[k] = v
	}
	return out
}

// RestoreComments replaces the comment map (memory rollback).
func (r *MemoryRepository) RestoreComments(snap map[string]Comment) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.comments = snap
}

func (r *MemoryRepository) SoftDelete(_ context.Context, owner OwnerContext, commentID, deletedByMembershipID string, deletedAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.comments[commentID]
	if !ok || c.CompanyID != owner.CompanyID || !c.MatchesOwner(owner) {
		return errNotFound{}
	}
	if !c.IsAlive() {
		return errAlreadyDeleted{}
	}
	c.DeletedAt = &deletedAt
	c.DeletedByMembershipID = strings.TrimSpace(deletedByMembershipID)
	r.comments[commentID] = c
	return nil
}

type errPathMismatch struct{}

func (errPathMismatch) Error() string { return "comment path mismatch" }

type errNotFound struct{}

func (errNotFound) Error() string { return "comment not found" }

type errAlreadyDeleted struct{}

func (errAlreadyDeleted) Error() string { return "comment already deleted" }

// Seed inserts a comment without validation (tests).
func (r *MemoryRepository) Seed(c Comment) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.comments[c.ID] = c
}
