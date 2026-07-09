package app

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Global Workflow Versioning (Batch 4 of Sprint 1 / mig-S2). publish ≠ activate.
//
// Anchor = type_id (stable). global_workflows.workflow_id is regenerated on every save, so it cannot
// anchor versions; type_id is the 1:1 stable key. Versioning is ADDITIVE: it never writes tenant
// override tables, never touches runtime instances, and the VersionRepository interface deliberately
// exposes NO tenant methods (tenant isolation by construction).

// Version states.
const (
	VersionStateDraft     = "draft"
	VersionStatePublished = "published"
	VersionStateActive    = "active"
	VersionStateArchived  = "archived"
)

// ManifestStep is one immutable step in a published version snapshot. Includes step_key.
type ManifestStep struct {
	StepID         string `json:"step_id"`
	StepKey        string `json:"step_key"`
	Stage          string `json:"stage"`
	Name           string `json:"name"`
	Instructions   string `json:"instructions,omitempty"`
	Role           string `json:"role"`
	DepartmentID   string `json:"department_id"`
	DueRule        string `json:"due_rule"`
	ProcessingDays int    `json:"processing_days"`
	DisplayOrder   int    `json:"display_order"`
}

// Manifest is the immutable, deterministic snapshot of a workflow version.
type Manifest struct {
	TypeID     string         `json:"type_id"`
	WorkflowID string         `json:"workflow_id"`
	VersionNo  int            `json:"version_no"`
	Steps      []ManifestStep `json:"steps"`
}

// VersionInfo is version metadata without the (large) manifest.
type VersionInfo struct {
	TypeID      string     `json:"type_id"`
	VersionNo   int        `json:"version_no"`
	State       string     `json:"state"`
	ChangeNote  string     `json:"change_note,omitempty"`
	PublishedAt *time.Time `json:"published_at,omitempty"`
	PublishedBy string     `json:"published_by,omitempty"`
	ActivatedAt *time.Time `json:"activated_at,omitempty"`
	ActivatedBy string     `json:"activated_by,omitempty"`
}

// VersionRecord is a full version (metadata + manifest).
type VersionRecord struct {
	VersionInfo
	Manifest Manifest `json:"manifest"`
}

// VersionRepository persists versions and pointers. ALL methods touch ONLY global tables
// (global_workflows, global_workflow_steps, global_workflow_versions). There is intentionally NO
// method that can write tenant override tables.
type VersionRepository interface {
	// BuildManifest reads the current editable workflow + steps and returns a deterministic manifest.
	BuildManifest(ctx context.Context, typeID string) (Manifest, error)
	ListVersions(ctx context.Context, typeID string) ([]VersionInfo, error)
	GetVersion(ctx context.Context, typeID string, versionNo int) (*VersionRecord, error)
	GetActiveVersion(ctx context.Context, typeID string) (*VersionRecord, error)
	// Publish inserts an immutable 'published' version (next version_no) and sets the published pointer.
	Publish(ctx context.Context, manifest Manifest, changeNote, actor string, at time.Time) (VersionInfo, error)
	// Activate flips the active pointer: demotes the current active to 'published', sets versionNo
	// 'active', updates global_workflows.active_version_no. Single transaction; global tables only.
	Activate(ctx context.Context, typeID string, versionNo int, actor string, at time.Time) (VersionInfo, error)
}

// Clock is injectable for deterministic tests.
type Clock func() time.Time

// VersionService orchestrates publish/activate with validation.
type VersionService struct {
	repo    VersionRepository
	now     Clock
	catalog *AssigneeRoleCatalogService
}

func NewVersionService(repo VersionRepository, clock Clock) *VersionService {
	if clock == nil {
		clock = time.Now
	}
	return &VersionService{repo: repo, now: clock}
}

func (s *VersionService) WithCatalog(catalog *AssigneeRoleCatalogService) *VersionService {
	s.catalog = catalog
	return s
}

func (s *VersionService) registryFor(ctx context.Context) *RoleRegistry {
	if s.catalog == nil {
		return DefaultRoleRegistry()
	}
	reg, err := s.catalog.MergedRegistry(ctx)
	if err != nil || reg == nil {
		return DefaultRoleRegistry()
	}
	return reg
}

// BuildManifestFromCurrentWorkflow returns the deterministic manifest of the current editable workflow.
func (s *VersionService) BuildManifestFromCurrentWorkflow(ctx context.Context, typeID string) (Manifest, error) {
	m, err := s.repo.BuildManifest(ctx, typeID)
	if err != nil {
		return Manifest{}, err
	}
	return NormalizeManifest(m), nil
}

func (s *VersionService) ListVersions(ctx context.Context, typeID string) ([]VersionInfo, error) {
	return s.repo.ListVersions(ctx, typeID)
}

func (s *VersionService) GetPublishedVersion(ctx context.Context, typeID string, versionNo int) (*VersionRecord, error) {
	return s.repo.GetVersion(ctx, typeID, versionNo)
}

func (s *VersionService) GetCurrentVersion(ctx context.Context, typeID string) (*VersionRecord, error) {
	return s.repo.GetActiveVersion(ctx, typeID)
}

// PublishVersion snapshots the current workflow into a new immutable 'published' version. It does NOT
// activate. Validation mirrors the activation-readiness checks (no reuse of the block-based validator
// because that operates on TemplateBlockDTO, not the manifest).
func (s *VersionService) PublishVersion(ctx context.Context, typeID, changeNote, actor string) (VersionInfo, error) {
	typeID = strings.TrimSpace(typeID)
	if typeID == "" {
		return VersionInfo{}, fmt.Errorf("type_id is required")
	}
	m, err := s.repo.BuildManifest(ctx, typeID)
	if err != nil {
		return VersionInfo{}, err
	}
	m = NormalizeManifest(m)
	if err := ValidateManifest(m); err != nil {
		return VersionInfo{}, err
	}
	// Reuse the Role Registry to reject invalid role / missing assignee at publish time.
	if err := ValidateManifestRoles(m, s.registryFor(ctx)); err != nil {
		return VersionInfo{}, err
	}
	return s.repo.Publish(ctx, m, changeNote, actor, s.now())
}

// ValidateActiveIntegrity fails if a type has more than one active version. No auto-fix.
func (s *VersionService) ValidateActiveIntegrity(ctx context.Context, typeID string) error {
	vs, err := s.repo.ListVersions(ctx, typeID)
	if err != nil {
		return err
	}
	active := 0
	for _, v := range vs {
		if v.State == VersionStateActive {
			active++
		}
	}
	if active > 1 {
		return fmt.Errorf("integrity violation: %d active versions for type %q (expected at most 1)", active, typeID)
	}
	return nil
}

// ActivateVersion sets a published version as active (pointer change only). Duplicate active is
// prevented by the repository's exclusive transition.
func (s *VersionService) ActivateVersion(ctx context.Context, typeID string, versionNo int, actor string) (VersionInfo, error) {
	typeID = strings.TrimSpace(typeID)
	if typeID == "" {
		return VersionInfo{}, fmt.Errorf("type_id is required")
	}
	// Defensive: refuse to activate if the type is already in a corrupt multi-active state.
	if err := s.ValidateActiveIntegrity(ctx, typeID); err != nil {
		return VersionInfo{}, err
	}
	v, err := s.repo.GetVersion(ctx, typeID, versionNo)
	if err != nil {
		return VersionInfo{}, err
	}
	if v == nil {
		return VersionInfo{}, fmt.Errorf("version %d not found for type %q", versionNo, typeID)
	}
	if v.State != VersionStatePublished && v.State != VersionStateActive {
		return VersionInfo{}, fmt.Errorf("version %d is not published (state=%s)", versionNo, v.State)
	}
	return s.repo.Activate(ctx, typeID, versionNo, actor, s.now())
}

// NormalizeManifest sorts steps deterministically (display_order, then step_key) for reproducible JSON.
func NormalizeManifest(m Manifest) Manifest {
	steps := append([]ManifestStep(nil), m.Steps...)
	sort.SliceStable(steps, func(i, j int) bool {
		if steps[i].DisplayOrder != steps[j].DisplayOrder {
			return steps[i].DisplayOrder < steps[j].DisplayOrder
		}
		return steps[i].StepKey < steps[j].StepKey
	})
	m.Steps = steps
	return m
}

// ManifestJSON returns the deterministic JSON encoding of a (normalized) manifest.
func ManifestJSON(m Manifest) ([]byte, error) {
	return json.Marshal(NormalizeManifest(m))
}

// ValidateManifestRoles rejects steps with a missing or unknown role (reuses the Role Registry).
// A published version must have a resolvable assignee role on every step.
func ValidateManifestRoles(m Manifest, reg *RoleRegistry) error {
	for i, st := range m.Steps {
		if strings.TrimSpace(st.Role) == "" {
			return fmt.Errorf("step %d (%s) has no assignee role", i+1, st.StepKey)
		}
		if _, ok := reg.GetRole(st.Role); !ok {
			return fmt.Errorf("step %d (%s) has unknown role %q", i+1, st.StepKey, st.Role)
		}
	}
	return nil
}

// ValidateManifest enforces publish readiness: non-empty steps, each with step_key + stage +
// processing_days > 0, and no duplicate step_key.
func ValidateManifest(m Manifest) error {
	if len(m.Steps) == 0 {
		return fmt.Errorf("workflow has no steps")
	}
	for i, st := range m.Steps {
		if strings.TrimSpace(st.StepKey) == "" {
			return fmt.Errorf("step %d missing step_key", i+1)
		}
		if strings.TrimSpace(st.Stage) == "" {
			return fmt.Errorf("step %d (%s) missing stage", i+1, st.StepKey)
		}
		if st.ProcessingDays <= 0 {
			return fmt.Errorf("step %d (%s) has invalid processing_days", i+1, st.StepKey)
		}
	}
	if dk := FindDuplicateStepKey(m); dk != "" {
		return fmt.Errorf("duplicate step_key %q", dk)
	}
	return nil
}

// FindDuplicateStepKey returns the first duplicated step_key, or "" if all are unique. Shared by
// ValidateManifest and the readiness service (single source of truth).
func FindDuplicateStepKey(m Manifest) string {
	seen := map[string]bool{}
	for _, st := range m.Steps {
		k := strings.TrimSpace(st.StepKey)
		if k == "" {
			continue
		}
		if seen[k] {
			return k
		}
		seen[k] = true
	}
	return ""
}
