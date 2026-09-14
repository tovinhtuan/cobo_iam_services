package memory

import (
	"context"
	"sync"
	"time"

	workflowapp "github.com/cobo/cobo_iam_services/internal/workflow/app"
	wff "github.com/cobo/cobo_iam_services/internal/workflowfulfillment"
)

// Repository is an in-memory fulfillment store with mutex serialization for concurrency tests.
type Repository struct {
	mu              sync.Mutex
	files           map[string]wff.FulfillmentFile
	stepCompleted   map[string]bool // key: instanceID|stepCode
	snapshotsLocked map[string]struct{}
}

func NewRepository() *Repository {
	return &Repository{
		files:           map[string]wff.FulfillmentFile{},
		stepCompleted:   map[string]bool{},
		snapshotsLocked: map[string]struct{}{},
	}
}

func stepKey(instanceID, stepCode string) string {
	return instanceID + "|" + stepCode
}

func (r *Repository) SetStepCompleted(workflowInstanceID, stepCode string, completed bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stepCompleted[stepKey(workflowInstanceID, stepCode)] = completed
}

func (r *Repository) ListActiveByRequirement(_ context.Context, companyID, requirementSnapshotID string) ([]wff.FulfillmentFile, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]wff.FulfillmentFile, 0)
	for _, f := range r.files {
		if f.CompanyID == companyID && f.RequirementSnapshotID == requirementSnapshotID && f.LifecycleStatus == wff.LifecycleActive {
			out = append(out, f)
		}
	}
	return out, nil
}

func (r *Repository) ListActiveByInstanceStep(_ context.Context, companyID, workflowInstanceID, stepCode string) ([]wff.FulfillmentFile, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]wff.FulfillmentFile, 0)
	for _, f := range r.files {
		if f.CompanyID == companyID && f.WorkflowInstanceID == workflowInstanceID && f.StepCode == stepCode && f.LifecycleStatus == wff.LifecycleActive {
			out = append(out, f)
		}
	}
	return out, nil
}

func (r *Repository) GetByID(_ context.Context, companyID, fileID string) (*wff.FulfillmentFile, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	f, ok := r.files[fileID]
	if !ok || f.CompanyID != companyID {
		return nil, nil
	}
	cp := f
	return &cp, nil
}

func (r *Repository) GetActiveByID(_ context.Context, companyID, fileID string) (*wff.FulfillmentFile, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	f, ok := r.files[fileID]
	if !ok || f.CompanyID != companyID || f.LifecycleStatus != wff.LifecycleActive {
		return nil, nil
	}
	cp := f
	return &cp, nil
}

func (r *Repository) CountActiveByRequirement(_ context.Context, requirementSnapshotID string) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, f := range r.files {
		if f.RequirementSnapshotID == requirementSnapshotID && f.LifecycleStatus == wff.LifecycleActive {
			n++
		}
	}
	return n, nil
}

func (r *Repository) UploadInTx(_ context.Context, in wff.UploadTxInput) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if in.RequireNotCompleted && r.stepCompleted[stepKey(in.WorkflowInstanceID, in.StepCode)] {
		return wff.ErrStepCompleted{}
	}
	n := 0
	for _, f := range r.files {
		if f.RequirementSnapshotID == in.RequirementSnapshotID && f.LifecycleStatus == wff.LifecycleActive {
			n++
		}
	}
	maxActive := in.MaxActive
	if maxActive <= 0 {
		maxActive = wff.MaxActiveFilesPerRequirement
	}
	if n >= maxActive {
		return wff.ErrActiveLimitReached{}
	}
	if err := wff.ValidateInitialLifecycle(in.File.LifecycleStatus); err != nil {
		return err
	}
	r.files[in.File.ID] = in.File
	return nil
}

func (r *Repository) ReplaceInTx(_ context.Context, in wff.ReplaceTxInput) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if in.RequireNotCompleted && r.stepCompleted[stepKey(in.WorkflowInstanceID, in.StepCode)] {
		return wff.ErrStepCompleted{}
	}
	old, ok := r.files[in.OldFileID]
	if !ok || old.CompanyID != in.CompanyID || old.LifecycleStatus != wff.LifecycleActive {
		return wff.ErrNotActive{}
	}
	if old.RequirementSnapshotID != in.RequirementSnapshotID ||
		old.WorkflowInstanceID != in.WorkflowInstanceID ||
		old.StepCode != in.StepCode {
		return wff.ErrNotActive{Reason: "context mismatch"}
	}
	if err := wff.ValidateTransition(old.LifecycleStatus, wff.LifecycleSuperseded); err != nil {
		return err
	}
	if err := wff.ValidateInitialLifecycle(in.NewFile.LifecycleStatus); err != nil {
		return err
	}
	in.NewFile.SupersedesFileID = old.ID
	old.LifecycleStatus = wff.LifecycleSuperseded
	old.SupersededByFileID = in.NewFile.ID
	r.files[old.ID] = old
	r.files[in.NewFile.ID] = in.NewFile
	return nil
}

func (r *Repository) DeleteInTx(_ context.Context, in wff.DeleteTxInput) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if in.RequireNotCompleted && r.stepCompleted[stepKey(in.WorkflowInstanceID, in.StepCode)] {
		return wff.ErrStepCompleted{}
	}
	old, ok := r.files[in.FileID]
	if !ok || old.CompanyID != in.CompanyID || old.LifecycleStatus != wff.LifecycleActive {
		return wff.ErrNotActive{}
	}
	if old.WorkflowInstanceID != in.WorkflowInstanceID || old.StepCode != in.StepCode {
		return wff.ErrNotActive{Reason: "context mismatch"}
	}
	if err := wff.ValidateTransition(old.LifecycleStatus, wff.LifecycleDeleted); err != nil {
		return err
	}
	t := in.DeletedAt
	old.LifecycleStatus = wff.LifecycleDeleted
	old.DeletedAt = &t
	old.DeletedBy = in.DeletedBy
	r.files[old.ID] = old
	return nil
}

// Seed inserts a file without validation (tests).
func (r *Repository) Seed(f wff.FulfillmentFile) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if f.UploadedAt.IsZero() {
		f.UploadedAt = time.Now().UTC()
	}
	r.files[f.ID] = f
}

// CompleteStepWithRequiredDocuments mirrors B3 gate under the same mutex as B2 mutations
// (serialization equivalent to same-TX locks for in-memory race tests).
// Zero snapshots → complete allowed (legacy). Counts only ACTIVE files for company+instance+step.
func (r *Repository) CompleteStepWithRequiredDocuments(
	companyID, workflowInstanceID, stepCode string,
	snaps []workflowapp.DocumentRequirementSnapshot,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := stepKey(workflowInstanceID, stepCode)
	if r.stepCompleted[key] {
		return nil
	}
	counts := map[string]int{}
	for _, f := range r.files {
		if f.CompanyID != companyID ||
			f.WorkflowInstanceID != workflowInstanceID ||
			f.StepCode != stepCode ||
			f.LifecycleStatus != wff.LifecycleActive {
			continue
		}
		counts[f.RequirementSnapshotID]++
	}
	missing := wff.EvaluateMissingRequiredDocuments(snaps, counts)
	if len(missing) > 0 {
		return wff.NewRequiredDocumentMissingError(missing)
	}
	r.stepCompleted[key] = true
	return nil
}

// IsStepCompleted reports completion flag (tests).
func (r *Repository) IsStepCompleted(workflowInstanceID, stepCode string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.stepCompleted[stepKey(workflowInstanceID, stepCode)]
}
