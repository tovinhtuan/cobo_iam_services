package app_test

import (
	"context"
	"testing"
	"time"

	wfc "github.com/cobo/cobo_iam_services/internal/workflowconfig/app"
)

// fakeVersionRepo mirrors the real repo semantics in memory. Crucially it also holds an
// `overrideRows` counter representing tenant override tables — the VersionRepository interface has
// NO method that can ever touch it, so it must remain 0 after any publish/activate (tenant isolation
// by construction).
type fakeVersionRepo struct {
	manifest     wfc.Manifest
	versions     map[int]*wfc.VersionRecord
	pubPtr       *int
	actPtr       *int
	overrideRows int // tenant rows — never mutated (no interface method can)
}

func newFakeRepo(m wfc.Manifest) *fakeVersionRepo {
	return &fakeVersionRepo{manifest: m, versions: map[int]*wfc.VersionRecord{}}
}

func (f *fakeVersionRepo) BuildManifest(_ context.Context, typeID string) (wfc.Manifest, error) {
	m := f.manifest
	m.TypeID = typeID
	return wfc.NormalizeManifest(m), nil
}
func (f *fakeVersionRepo) ListVersions(_ context.Context, typeID string) ([]wfc.VersionInfo, error) {
	var out []wfc.VersionInfo
	for i := 1; i <= len(f.versions); i++ {
		if v, ok := f.versions[i]; ok {
			out = append(out, v.VersionInfo)
		}
	}
	return out, nil
}
func (f *fakeVersionRepo) GetVersion(_ context.Context, _ string, n int) (*wfc.VersionRecord, error) {
	return f.versions[n], nil
}
func (f *fakeVersionRepo) GetActiveVersion(_ context.Context, _ string) (*wfc.VersionRecord, error) {
	for _, v := range f.versions {
		if v.State == wfc.VersionStateActive {
			return v, nil
		}
	}
	return nil, nil
}
func (f *fakeVersionRepo) Publish(_ context.Context, m wfc.Manifest, note, actor string, at time.Time) (wfc.VersionInfo, error) {
	next := len(f.versions) + 1
	m.VersionNo = next
	pub := at
	info := wfc.VersionInfo{TypeID: m.TypeID, VersionNo: next, State: wfc.VersionStatePublished, ChangeNote: note, PublishedAt: &pub, PublishedBy: actor}
	f.versions[next] = &wfc.VersionRecord{VersionInfo: info, Manifest: wfc.NormalizeManifest(m)}
	f.pubPtr = &next
	return info, nil
}
func (f *fakeVersionRepo) Activate(_ context.Context, typeID string, n int, actor string, at time.Time) (wfc.VersionInfo, error) {
	for k, v := range f.versions {
		if v.State == wfc.VersionStateActive && k != n {
			v.State = wfc.VersionStatePublished // demote → single active
		}
	}
	v := f.versions[n]
	v.State = wfc.VersionStateActive
	act := at
	v.ActivatedAt = &act
	v.ActivatedBy = actor
	f.actPtr = &n
	return v.VersionInfo, nil
}

func sampleManifest() wfc.Manifest {
	return wfc.Manifest{
		TypeID: "t1", WorkflowID: "w1",
		Steps: []wfc.ManifestStep{
			{StepID: "s2", StepKey: "stp_bbbbbbbbbbbb", Stage: "Approve", Role: "approver", ProcessingDays: 2, DisplayOrder: 2},
			{StepID: "s1", StepKey: "stp_aaaaaaaaaaaa", Stage: "Review", Role: "reviewer", ProcessingDays: 3, DisplayOrder: 1},
		},
	}
}

func fixedClock() wfc.Clock { return func() time.Time { return time.Unix(1700000000, 0).UTC() } }

// 2. Publish creates version N+1.
func TestPublish_CreatesNextVersion(t *testing.T) {
	repo := newFakeRepo(sampleManifest())
	svc := wfc.NewVersionService(repo, fixedClock())
	v1, err := svc.PublishVersion(context.Background(), "t1", "v1", "admin")
	if err != nil || v1.VersionNo != 1 || v1.State != wfc.VersionStatePublished {
		t.Fatalf("publish v1 unexpected: %+v err=%v", v1, err)
	}
	v2, err := svc.PublishVersion(context.Background(), "t1", "v2", "admin")
	if err != nil || v2.VersionNo != 2 {
		t.Fatalf("publish v2 unexpected: %+v err=%v", v2, err)
	}
}

// 3. Published immutable: re-publishing does not mutate the earlier version row.
func TestPublish_Immutable(t *testing.T) {
	repo := newFakeRepo(sampleManifest())
	svc := wfc.NewVersionService(repo, fixedClock())
	_, _ = svc.PublishVersion(context.Background(), "t1", "v1", "admin")
	before := *repo.versions[1]
	_, _ = svc.PublishVersion(context.Background(), "t1", "v2", "admin")
	after := repo.versions[1]
	if before.State != after.State || before.ChangeNote != after.ChangeNote || len(before.Manifest.Steps) != len(after.Manifest.Steps) {
		t.Fatal("v1 mutated by subsequent publish")
	}
}

// 4 + 5 + 7. Activate changes only the active pointer; writes 0 tenant rows; single active.
func TestActivate_PointerOnly_NoTenantWrites_SingleActive(t *testing.T) {
	repo := newFakeRepo(sampleManifest())
	svc := wfc.NewVersionService(repo, fixedClock())
	_, _ = svc.PublishVersion(context.Background(), "t1", "v1", "admin")
	_, _ = svc.PublishVersion(context.Background(), "t1", "v2", "admin")

	if _, err := svc.ActivateVersion(context.Background(), "t1", 1, "admin"); err != nil {
		t.Fatalf("activate v1: %v", err)
	}
	if _, err := svc.ActivateVersion(context.Background(), "t1", 2, "admin"); err != nil {
		t.Fatalf("activate v2: %v", err)
	}
	// single active
	active := 0
	for _, v := range repo.versions {
		if v.State == wfc.VersionStateActive {
			active++
		}
	}
	if active != 1 || repo.actPtr == nil || *repo.actPtr != 2 {
		t.Fatalf("expected single active=v2, active=%d ptr=%v", active, repo.actPtr)
	}
	// tenant isolation: 0 tenant rows touched (structurally impossible).
	if repo.overrideRows != 0 {
		t.Fatalf("tenant override rows changed: %d (must be 0)", repo.overrideRows)
	}
}

// 8. Duplicate active is prevented (activating v2 demotes v1).
func TestActivate_DuplicateActivePrevented(t *testing.T) {
	repo := newFakeRepo(sampleManifest())
	svc := wfc.NewVersionService(repo, fixedClock())
	_, _ = svc.PublishVersion(context.Background(), "t1", "v1", "admin")
	_, _ = svc.PublishVersion(context.Background(), "t1", "v2", "admin")
	_, _ = svc.ActivateVersion(context.Background(), "t1", 1, "admin")
	_, _ = svc.ActivateVersion(context.Background(), "t1", 2, "admin")
	if repo.versions[1].State == wfc.VersionStateActive {
		t.Fatal("v1 should have been demoted; two active versions not allowed")
	}
}

// Activate missing / non-published fails.
func TestActivate_MissingOrUnpublishedFails(t *testing.T) {
	repo := newFakeRepo(sampleManifest())
	svc := wfc.NewVersionService(repo, fixedClock())
	if _, err := svc.ActivateVersion(context.Background(), "t1", 99, "admin"); err == nil {
		t.Fatal("expected error activating missing version")
	}
}

// 9. Manifest includes step_key.
func TestManifest_IncludesStepKey(t *testing.T) {
	repo := newFakeRepo(sampleManifest())
	svc := wfc.NewVersionService(repo, fixedClock())
	m, err := svc.BuildManifestFromCurrentWorkflow(context.Background(), "t1")
	if err != nil {
		t.Fatal(err)
	}
	for _, st := range m.Steps {
		if st.StepKey == "" {
			t.Fatalf("manifest step %q missing step_key", st.Stage)
		}
	}
}

// 10. Manifest deterministic: normalize sorts by display_order; JSON stable across runs.
func TestManifest_Deterministic(t *testing.T) {
	m := sampleManifest()
	n1 := wfc.NormalizeManifest(m)
	if n1.Steps[0].DisplayOrder != 1 || n1.Steps[0].StepKey != "stp_aaaaaaaaaaaa" {
		t.Fatalf("normalize did not sort by display_order: %+v", n1.Steps)
	}
	j1, _ := wfc.ManifestJSON(m)
	j2, _ := wfc.ManifestJSON(m)
	if string(j1) != string(j2) {
		t.Fatal("manifest JSON not deterministic")
	}
}

// Integrity: detect (and refuse to act on) a corrupt multi-active state. No auto-fix.
func TestActiveIntegrity_TwoActiveFails(t *testing.T) {
	repo := newFakeRepo(sampleManifest())
	svc := wfc.NewVersionService(repo, fixedClock())
	// seed two active versions directly (simulating corruption).
	repo.versions[1] = &wfc.VersionRecord{VersionInfo: wfc.VersionInfo{TypeID: "t1", VersionNo: 1, State: wfc.VersionStateActive}}
	repo.versions[2] = &wfc.VersionRecord{VersionInfo: wfc.VersionInfo{TypeID: "t1", VersionNo: 2, State: wfc.VersionStateActive}}
	if err := svc.ValidateActiveIntegrity(context.Background(), "t1"); err == nil {
		t.Fatal("expected integrity error for 2 active versions")
	}
	// activate must refuse to proceed on a corrupt state (no auto-fix).
	if _, err := svc.ActivateVersion(context.Background(), "t1", 1, "admin"); err == nil {
		t.Fatal("activate should fail when integrity is violated")
	}
}

// Publish rejects steps with missing or unknown role (reuses Role Registry).
func TestPublish_InvalidRoleFails(t *testing.T) {
	mNoRole := sampleManifest()
	for i := range mNoRole.Steps {
		mNoRole.Steps[i].Role = ""
	}
	repo := newFakeRepo(mNoRole)
	if _, err := wfc.NewVersionService(repo, fixedClock()).PublishVersion(context.Background(), "t1", "v1", "admin"); err == nil {
		t.Fatal("publish should fail when a step has no role")
	}

	mBadRole := sampleManifest()
	mBadRole.Steps[0].Role = "totally_unknown_role"
	repo2 := newFakeRepo(mBadRole)
	if _, err := wfc.NewVersionService(repo2, fixedClock()).PublishVersion(context.Background(), "t1", "v1", "admin"); err == nil {
		t.Fatal("publish should fail when a step has an unknown role")
	}
}

// ValidateManifest rejects empty / missing-key / duplicate-key.
func TestValidateManifest(t *testing.T) {
	if err := wfc.ValidateManifest(wfc.Manifest{}); err == nil {
		t.Fatal("empty manifest should fail")
	}
	dup := wfc.Manifest{Steps: []wfc.ManifestStep{
		{StepKey: "k", Stage: "A", ProcessingDays: 1},
		{StepKey: "k", Stage: "B", ProcessingDays: 1},
	}}
	if err := wfc.ValidateManifest(dup); err == nil {
		t.Fatal("duplicate step_key should fail")
	}
	noKey := wfc.Manifest{Steps: []wfc.ManifestStep{{Stage: "A", ProcessingDays: 1}}}
	if err := wfc.ValidateManifest(noKey); err == nil {
		t.Fatal("missing step_key should fail")
	}
}
