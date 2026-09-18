package workflowstepevidence_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	wse "github.com/cobo/cobo_iam_services/internal/workflowstepevidence"
	"github.com/cobo/cobo_iam_services/internal/workflowstepevidence/memory"
)

func TestConcurrency_ActiveLimitAtNine(t *testing.T) {
	repo := memory.NewRepository()
	ctx := context.Background()
	o := owner()
	for i := 0; i < 9; i++ {
		f := baseFile(fmt.Sprintf("wse_pre_%d", i), o)
		if err := repo.CreateActiveInTx(ctx, wse.CreateTxInput{File: f, Owner: o, RequireNotCompleted: true}); err != nil {
			t.Fatal(err)
		}
	}
	var okCount, failCount atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			f := baseFile(fmt.Sprintf("wse_race_%d", i), o)
			err := repo.CreateActiveInTx(ctx, wse.CreateTxInput{File: f, Owner: o, RequireNotCompleted: true})
			if err == nil {
				okCount.Add(1)
			} else if errors.As(err, &wse.ErrActiveLimitReached{}) {
				failCount.Add(1)
			} else {
				t.Errorf("unexpected err: %v", err)
			}
		}(i)
	}
	wg.Wait()
	n, _ := repo.CountActiveByStep(ctx, o)
	if n != 10 {
		t.Fatalf("active=%d want 10 (ok=%d fail=%d)", n, okCount.Load(), failCount.Load())
	}
	if okCount.Load() != 1 {
		t.Fatalf("exactly one success expected, ok=%d", okCount.Load())
	}
}

func TestConcurrency_DoubleDelete(t *testing.T) {
	repo := memory.NewRepository()
	ctx := context.Background()
	o := owner()
	f := baseFile("wse_dd", o)
	repo.Seed(f)
	var okCount atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := repo.DeleteInTx(ctx, wse.DeleteTxInput{
				FileID: f.ID, Owner: o, DeletedBy: "u1", DeletedAt: time.Now().UTC(), RequireNotCompleted: true,
			})
			if err == nil {
				okCount.Add(1)
			}
		}()
	}
	wg.Wait()
	if okCount.Load() != 1 {
		t.Fatalf("ok=%d", okCount.Load())
	}
	got, _ := repo.GetByIDInContext(ctx, o, f.ID)
	if got.LifecycleStatus != wse.LifecycleDeleted {
		t.Fatalf("status=%s", got.LifecycleStatus)
	}
}

func TestConcurrency_DoubleReplace(t *testing.T) {
	repo := memory.NewRepository()
	ctx := context.Background()
	o := owner()
	old := baseFile("wse_dr_old", o)
	repo.Seed(old)
	var okCount atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			err := repo.ReplaceInTx(ctx, wse.ReplaceTxInput{
				OldFileID: old.ID,
				NewFile:   baseFile(fmt.Sprintf("wse_dr_new_%d", i), o),
				Owner:     o, RequireNotCompleted: true,
			})
			if err == nil {
				okCount.Add(1)
			}
		}(i)
	}
	wg.Wait()
	if okCount.Load() != 1 {
		t.Fatalf("ok=%d", okCount.Load())
	}
	got, _ := repo.GetByIDInContext(ctx, o, old.ID)
	if got.LifecycleStatus != wse.LifecycleSuperseded {
		t.Fatalf("status=%s", got.LifecycleStatus)
	}
	n, _ := repo.CountActiveByStep(ctx, o)
	if n != 1 {
		t.Fatalf("active=%d", n)
	}
}

func TestConcurrency_DeleteVsReplace(t *testing.T) {
	repo := memory.NewRepository()
	ctx := context.Background()
	o := owner()
	old := baseFile("wse_dvr", o)
	repo.Seed(old)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_ = repo.DeleteInTx(ctx, wse.DeleteTxInput{
			FileID: old.ID, Owner: o, DeletedBy: "u1", DeletedAt: time.Now().UTC(), RequireNotCompleted: true,
		})
	}()
	go func() {
		defer wg.Done()
		_ = repo.ReplaceInTx(ctx, wse.ReplaceTxInput{
			OldFileID: old.ID, NewFile: baseFile("wse_dvr_new", o), Owner: o, RequireNotCompleted: true,
		})
	}()
	wg.Wait()
	got, _ := repo.GetByIDInContext(ctx, o, old.ID)
	switch got.LifecycleStatus {
	case wse.LifecycleDeleted, wse.LifecycleSuperseded:
		// ok — exactly one winner
	default:
		t.Fatalf("invalid final state %s", got.LifecycleStatus)
	}
	n, _ := repo.CountActiveByStep(ctx, o)
	if got.LifecycleStatus == wse.LifecycleDeleted && n != 0 {
		t.Fatalf("deleted but active=%d", n)
	}
	if got.LifecycleStatus == wse.LifecycleSuperseded && n != 1 {
		t.Fatalf("superseded but active=%d", n)
	}
}

func TestConcurrency_CompleteVsCreate(t *testing.T) {
	repo := memory.NewRepository()
	ctx := context.Background()
	o := owner()
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		repo.CompleteStep(o.WorkflowInstanceID, o.StepCode)
	}()
	go func() {
		defer wg.Done()
		_ = repo.CreateActiveInTx(ctx, wse.CreateTxInput{
			File: baseFile("wse_cc", o), Owner: o, RequireNotCompleted: true,
		})
	}()
	wg.Wait()
	completed := repo.IsStepCompleted(o.WorkflowInstanceID, o.StepCode)
	n, _ := repo.CountActiveByStep(ctx, o)
	if !completed {
		t.Fatal("complete should win or both serialize; final must be completed for invariant check path")
	}
	// If create won first then complete, n may be 1 — that's OK (mutation before complete).
	// If complete won first, create fails and n=0. Never: completed=false with later issue.
	// Re-check: after completed, further create must fail.
	err := repo.CreateActiveInTx(ctx, wse.CreateTxInput{
		File: baseFile("wse_cc2", o), Owner: o, RequireNotCompleted: true,
	})
	if !errors.As(err, &wse.ErrStepCompleted{}) {
		t.Fatalf("post-complete create must fail, got %v (prior active=%d)", err, n)
	}
}

func TestConcurrency_CompleteVsDelete(t *testing.T) {
	repo := memory.NewRepository()
	ctx := context.Background()
	o := owner()
	f := baseFile("wse_cd", o)
	repo.Seed(f)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		repo.CompleteStep(o.WorkflowInstanceID, o.StepCode)
	}()
	go func() {
		defer wg.Done()
		_ = repo.DeleteInTx(ctx, wse.DeleteTxInput{
			FileID: f.ID, Owner: o, DeletedBy: "u1", DeletedAt: time.Now().UTC(), RequireNotCompleted: true,
		})
	}()
	wg.Wait()
	if !repo.IsStepCompleted(o.WorkflowInstanceID, o.StepCode) {
		t.Fatal("expected completed")
	}
	err := repo.DeleteInTx(ctx, wse.DeleteTxInput{
		FileID: f.ID, Owner: o, DeletedBy: "u1", DeletedAt: time.Now().UTC(), RequireNotCompleted: true,
	})
	// Either already deleted (not active) or step completed — both OK for immutability.
	if err == nil {
		t.Fatal("post-complete delete must not succeed")
	}
}

func TestConcurrency_CompleteVsReplace(t *testing.T) {
	repo := memory.NewRepository()
	ctx := context.Background()
	o := owner()
	f := baseFile("wse_cr", o)
	repo.Seed(f)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		repo.CompleteStep(o.WorkflowInstanceID, o.StepCode)
	}()
	go func() {
		defer wg.Done()
		_ = repo.ReplaceInTx(ctx, wse.ReplaceTxInput{
			OldFileID: f.ID, NewFile: baseFile("wse_cr_new", o), Owner: o, RequireNotCompleted: true,
		})
	}()
	wg.Wait()
	if !repo.IsStepCompleted(o.WorkflowInstanceID, o.StepCode) {
		t.Fatal("expected completed")
	}
	err := repo.ReplaceInTx(ctx, wse.ReplaceTxInput{
		OldFileID: f.ID, NewFile: baseFile("wse_cr_new2", o), Owner: o, RequireNotCompleted: true,
	})
	if err == nil {
		t.Fatal("post-complete replace must not succeed")
	}
}
