package app_test

import (
	"context"
	"testing"

	wfc "github.com/cobo/cobo_iam_services/internal/workflowconfig/app"
)

func buildServices(m wfc.Manifest) (*wfc.VersionService, *wfc.ConfigService, *fakeVersionRepo) {
	repo := newFakeRepo(m)
	vs := wfc.NewVersionService(repo, fixedClock())
	rs := wfc.NewReadinessService(vs, wfc.DefaultRoleRegistry())
	cs := wfc.NewConfigService(vs, rs)
	return vs, cs, repo
}

// 2. readiness passes a valid workflow with all checks.
func TestReadiness_ValidWorkflowPasses(t *testing.T) {
	_, cs, _ := buildServices(sampleManifest())
	r, err := cs.Readiness(context.Background(), "t1")
	if err != nil {
		t.Fatal(err)
	}
	if !r.Ready || r.Score != 100 || len(r.BlockingErrors) != 0 {
		t.Fatalf("expected ready/100/no-blocking, got %+v", r)
	}
	keys := map[string]bool{}
	for _, c := range r.Checks {
		keys[c.Key] = true
	}
	for _, want := range []string{wfc.CheckWorkflowExists, wfc.CheckHasSteps, wfc.CheckUniqueStepKey, wfc.CheckValidRoles, wfc.CheckValidAssignees, wfc.CheckSingleActiveVer} {
		if !keys[want] {
			t.Fatalf("missing readiness check %q", want)
		}
	}
}

// 6 + 7. readiness blocks an invalid workflow (missing role).
func TestReadiness_InvalidWorkflowBlocks(t *testing.T) {
	m := sampleManifest()
	for i := range m.Steps {
		m.Steps[i].Role = ""
	}
	_, cs, _ := buildServices(m)
	r, _ := cs.Readiness(context.Background(), "t1")
	if r.Ready || len(r.BlockingErrors) == 0 || r.Score == 100 {
		t.Fatalf("expected not-ready with blocking errors, got %+v", r)
	}
}

// readiness reports workflow_exists=false when no workflow.
func TestReadiness_NoWorkflow(t *testing.T) {
	_, cs, _ := buildServices(wfc.Manifest{}) // empty manifest = no steps; still "exists" via fake
	r, _ := cs.Readiness(context.Background(), "t1")
	// empty manifest builds (fake) but has no steps → has_steps fails → not ready.
	if r.Ready {
		t.Fatalf("empty workflow should not be ready: %+v", r)
	}
}

// 3. validate reuses validation (valid for good workflow).
func TestValidate_ReusesValidation(t *testing.T) {
	_, cs, _ := buildServices(sampleManifest())
	v, err := cs.Validate(context.Background(), "t1")
	if err != nil || !v.Valid || len(v.Errors) != 0 {
		t.Fatalf("expected valid, got %+v err=%v", v, err)
	}
	// invalid
	m := sampleManifest()
	m.Steps[0].Role = "unknown_role_x"
	_, cs2, _ := buildServices(m)
	v2, _ := cs2.Validate(context.Background(), "t1")
	if v2.Valid || len(v2.Errors) == 0 {
		t.Fatalf("expected invalid, got %+v", v2)
	}
}

// 4. lifecycle returns active/published state + can_publish/can_activate.
func TestLifecycle_PublishedAndActiveState(t *testing.T) {
	vs, cs, _ := buildServices(sampleManifest())
	ctx := context.Background()

	// before any publish: no published, cannot activate, can publish (valid workflow).
	ls0, _ := cs.Lifecycle(ctx, "t1")
	if ls0.Published != nil || ls0.Active != nil || ls0.CanActivate {
		t.Fatalf("pre-publish lifecycle unexpected: %+v", ls0)
	}
	if !ls0.CanPublish || !ls0.Draft.Exists || ls0.Draft.StepCount != 2 {
		t.Fatalf("pre-publish draft/can_publish unexpected: %+v", ls0)
	}

	// publish + activate.
	if _, err := vs.PublishVersion(ctx, "t1", "v1", "admin"); err != nil {
		t.Fatal(err)
	}
	ls1, _ := cs.Lifecycle(ctx, "t1")
	if ls1.Published == nil || ls1.Published.VersionNo != 1 || !ls1.CanActivate || ls1.Active != nil {
		t.Fatalf("post-publish lifecycle unexpected: %+v", ls1)
	}
	if _, err := vs.ActivateVersion(ctx, "t1", 1, "admin"); err != nil {
		t.Fatal(err)
	}
	ls2, _ := cs.Lifecycle(ctx, "t1")
	if ls2.Active == nil || ls2.Active.VersionNo != 1 {
		t.Fatalf("post-activate lifecycle unexpected: %+v", ls2)
	}
}

// 1. aggregate API returns the expected shape (workflow/versions/readiness/lifecycle/governance).
func TestConfiguration_AggregateShape(t *testing.T) {
	vs, cs, _ := buildServices(sampleManifest())
	ctx := context.Background()
	_, _ = vs.PublishVersion(ctx, "t1", "v1", "admin")
	_, _ = vs.ActivateVersion(ctx, "t1", 1, "admin")

	cfgData, err := cs.Configuration(ctx, "t1")
	if err != nil {
		t.Fatal(err)
	}
	if len(cfgData.Workflow.Steps) != 2 {
		t.Fatalf("workflow steps = %d, want 2", len(cfgData.Workflow.Steps))
	}
	if len(cfgData.Versions) != 1 {
		t.Fatalf("versions = %d, want 1", len(cfgData.Versions))
	}
	if !cfgData.Readiness.Ready {
		t.Fatalf("readiness should be ready: %+v", cfgData.Readiness)
	}
	if cfgData.Governance.Status != "active" || cfgData.Governance.ActiveVersionNo == nil || *cfgData.Governance.ActiveVersionNo != 1 {
		t.Fatalf("governance unexpected: %+v", cfgData.Governance)
	}
	if cfgData.Lifecycle.Active == nil {
		t.Fatal("lifecycle.active should be set")
	}
}

// 5. version detail returns the manifest (via GetPublishedVersion).
func TestVersionDetail_ReturnsManifest(t *testing.T) {
	vs, _, _ := buildServices(sampleManifest())
	ctx := context.Background()
	_, _ = vs.PublishVersion(ctx, "t1", "v1", "admin")
	rec, err := vs.GetPublishedVersion(ctx, "t1", 1)
	if err != nil || rec == nil {
		t.Fatalf("get version: %v rec=%v", err, rec)
	}
	if len(rec.Manifest.Steps) != 2 || rec.Manifest.Steps[0].StepKey == "" {
		t.Fatalf("version manifest missing steps/step_key: %+v", rec.Manifest)
	}
}
