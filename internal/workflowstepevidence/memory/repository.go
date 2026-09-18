package memory

import (
	"context"
	"sort"
	"sync"
	"time"

	wse "github.com/cobo/cobo_iam_services/internal/workflowstepevidence"
)

// Repository is an in-memory generic evidence store with mutex serialization for concurrency tests.
type Repository struct {
	mu            sync.Mutex
	files         map[string]wse.EvidenceFile
	stepCompleted map[string]bool // key: instanceID|stepCode
}

func NewRepository() *Repository {
	return &Repository{
		files:         map[string]wse.EvidenceFile{},
		stepCompleted: map[string]bool{},
	}
}

func stepKey(instanceID, stepCode string) string {
	return instanceID + "|" + stepCode
}

// SetStepCompleted sets the canonical completed flag (tests / Complete race).
func (r *Repository) SetStepCompleted(workflowInstanceID, stepCode string, completed bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stepCompleted[stepKey(workflowInstanceID, stepCode)] = completed
}

// IsStepCompleted reports completion flag (tests).
func (r *Repository) IsStepCompleted(workflowInstanceID, stepCode string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.stepCompleted[stepKey(workflowInstanceID, stepCode)]
}

// CompleteStep marks the step completed under the same mutex as mutations
// (serialization equivalent to FOR UPDATE on workflow_instance_step_states).
func (r *Repository) CompleteStep(workflowInstanceID, stepCode string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stepCompleted[stepKey(workflowInstanceID, stepCode)] = true
}

func (r *Repository) ListActiveByStep(_ context.Context, owner wse.OwnerContext) ([]wse.EvidenceFile, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]wse.EvidenceFile, 0)
	for _, f := range r.files {
		if f.MatchesOwner(owner) && f.LifecycleStatus == wse.LifecycleActive {
			out = append(out, f)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if !out[i].UploadedAt.Equal(out[j].UploadedAt) {
			return out[i].UploadedAt.Before(out[j].UploadedAt)
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func (r *Repository) CountActiveByStep(_ context.Context, owner wse.OwnerContext) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.countActiveLocked(owner), nil
}

func (r *Repository) countActiveLocked(owner wse.OwnerContext) int {
	n := 0
	for _, f := range r.files {
		if f.MatchesOwner(owner) && f.LifecycleStatus == wse.LifecycleActive {
			n++
		}
	}
	return n
}

func (r *Repository) GetByIDInContext(_ context.Context, owner wse.OwnerContext, fileID string) (*wse.EvidenceFile, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	f, ok := r.files[fileID]
	if !ok || !f.MatchesOwner(owner) {
		return nil, nil
	}
	cp := f
	return &cp, nil
}

func (r *Repository) GetActiveByIDInContext(_ context.Context, owner wse.OwnerContext, fileID string) (*wse.EvidenceFile, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	f, ok := r.files[fileID]
	if !ok || !f.MatchesOwner(owner) || f.LifecycleStatus != wse.LifecycleActive {
		return nil, nil
	}
	cp := f
	return &cp, nil
}

func (r *Repository) CreateActiveInTx(_ context.Context, in wse.CreateTxInput) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if in.RequireNotCompleted && r.stepCompleted[stepKey(in.Owner.WorkflowInstanceID, in.Owner.StepCode)] {
		return wse.ErrStepCompleted{}
	}
	if !in.File.MatchesOwner(in.Owner) {
		return wse.ErrNotActive{Reason: "file owner mismatch"}
	}
	maxActive := in.MaxActive
	if maxActive <= 0 {
		maxActive = wse.MaxActiveFilesPerStep
	}
	if r.countActiveLocked(in.Owner) >= maxActive {
		return wse.ErrActiveLimitReached{}
	}
	if err := wse.ValidateInitialLifecycle(in.File.LifecycleStatus); err != nil {
		return err
	}
	r.files[in.File.ID] = in.File
	return nil
}

func (r *Repository) ReplaceInTx(_ context.Context, in wse.ReplaceTxInput) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if in.RequireNotCompleted && r.stepCompleted[stepKey(in.Owner.WorkflowInstanceID, in.Owner.StepCode)] {
		return wse.ErrStepCompleted{}
	}
	old, ok := r.files[in.OldFileID]
	if !ok || old.LifecycleStatus != wse.LifecycleActive || !old.MatchesOwner(in.Owner) {
		return wse.ErrNotActive{Reason: "file not active"}
	}
	if err := wse.ValidateTransition(old.LifecycleStatus, wse.LifecycleSuperseded); err != nil {
		return err
	}
	if err := wse.ValidateInitialLifecycle(in.NewFile.LifecycleStatus); err != nil {
		return err
	}
	if !in.NewFile.MatchesOwner(in.Owner) {
		return wse.ErrNotActive{Reason: "new file owner mismatch"}
	}
	// REPLACE_AT_ACTIVE_LIMIT_ALLOWED: no MaxActive check (net ACTIVE unchanged).
	in.NewFile.SupersedesFileID = old.ID
	old.LifecycleStatus = wse.LifecycleSuperseded
	old.SupersededByFileID = in.NewFile.ID
	r.files[old.ID] = old
	r.files[in.NewFile.ID] = in.NewFile
	return nil
}

func (r *Repository) DeleteInTx(_ context.Context, in wse.DeleteTxInput) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if in.RequireNotCompleted && r.stepCompleted[stepKey(in.Owner.WorkflowInstanceID, in.Owner.StepCode)] {
		return wse.ErrStepCompleted{}
	}
	old, ok := r.files[in.FileID]
	if !ok || old.LifecycleStatus != wse.LifecycleActive || !old.MatchesOwner(in.Owner) {
		return wse.ErrNotActive{Reason: "file not active"}
	}
	if err := wse.ValidateTransition(old.LifecycleStatus, wse.LifecycleDeleted); err != nil {
		return err
	}
	t := in.DeletedAt
	old.LifecycleStatus = wse.LifecycleDeleted
	old.DeletedAt = &t
	old.DeletedBy = in.DeletedBy
	r.files[old.ID] = old
	return nil
}

// Seed inserts a file without validation (tests).
func (r *Repository) Seed(f wse.EvidenceFile) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if f.UploadedAt.IsZero() {
		f.UploadedAt = time.Now().UTC()
	}
	r.files[f.ID] = f
}

// PhysicalKeys returns storage keys still present in memory map (tests; no unlink on delete).
func (r *Repository) PhysicalKeys() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, 0, len(r.files))
	for _, f := range r.files {
		out = append(out, f.StorageKey)
	}
	return out
}
