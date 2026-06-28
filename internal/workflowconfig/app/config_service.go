package app

import "context"

// ConfigService is the read-side facade for the Workflow Configuration tab. It composes the existing
// VersionService + ReadinessService so the FE can load everything with one aggregate call. It performs
// NO new validation and NO tenant logic.

type DraftInfo struct {
	Exists    bool `json:"exists"`
	StepCount int  `json:"step_count"`
}

type GovernanceInfo struct {
	Status             string `json:"status"`
	PublishedVersionNo *int   `json:"published_version_no"`
	ActiveVersionNo    *int   `json:"active_version_no"`
	TotalVersions      int    `json:"total_versions"`
}

type LifecycleStatus struct {
	Draft       DraftInfo    `json:"draft"`
	Published   *VersionInfo `json:"published"`
	Active      *VersionInfo `json:"active"`
	CanPublish  bool         `json:"can_publish"`
	CanActivate bool         `json:"can_activate"`
}

type ValidationResult struct {
	Valid    bool     `json:"valid"`
	Errors   []string `json:"errors"`
	Warnings []string `json:"warnings"`
}

type WorkflowConfiguration struct {
	Workflow   Manifest        `json:"workflow"`
	Versions   []VersionInfo   `json:"versions"`
	Readiness  ReadinessResult `json:"readiness"`
	Lifecycle  LifecycleStatus `json:"lifecycle"`
	Governance GovernanceInfo  `json:"governance"`
}

type ConfigService struct {
	versions  *VersionService
	readiness *ReadinessService
}

func NewConfigService(versions *VersionService, readiness *ReadinessService) *ConfigService {
	return &ConfigService{versions: versions, readiness: readiness}
}

func (c *ConfigService) Readiness(ctx context.Context, typeID string) (ReadinessResult, error) {
	return c.readiness.Evaluate(ctx, typeID)
}

// Validate projects the readiness result into a simple valid/errors/warnings shape (reuses readiness,
// which reuses the existing validation rules — no second validator).
func (c *ConfigService) Validate(ctx context.Context, typeID string) (ValidationResult, error) {
	r, err := c.readiness.Evaluate(ctx, typeID)
	if err != nil {
		return ValidationResult{}, err
	}
	return ValidationResult{Valid: r.Ready, Errors: r.BlockingErrors, Warnings: r.Warnings}, nil
}

func (c *ConfigService) Lifecycle(ctx context.Context, typeID string) (LifecycleStatus, error) {
	versions, err := c.versions.ListVersions(ctx, typeID)
	if err != nil {
		return LifecycleStatus{}, err
	}
	active, err := c.versions.GetCurrentVersion(ctx, typeID)
	if err != nil {
		return LifecycleStatus{}, err
	}
	published := latestPublished(versions)

	draft := DraftInfo{}
	if m, err := c.versions.BuildManifestFromCurrentWorkflow(ctx, typeID); err == nil {
		draft.Exists = true
		draft.StepCount = len(m.Steps)
	}

	readiness, err := c.readiness.Evaluate(ctx, typeID)
	if err != nil {
		return LifecycleStatus{}, err
	}

	ls := LifecycleStatus{
		Draft:       draft,
		Published:   published,
		CanPublish:  readiness.Ready,
	}
	if active != nil {
		ls.Active = &active.VersionInfo
	}
	// Allow activate when a published version exists and is not yet the active runtime version.
	if published != nil {
		if active == nil || active.VersionNo != published.VersionNo {
			ls.CanActivate = true
		}
	}
	return ls, nil
}

func (c *ConfigService) Configuration(ctx context.Context, typeID string) (WorkflowConfiguration, error) {
	cfg := WorkflowConfiguration{Versions: []VersionInfo{}}

	if m, err := c.versions.BuildManifestFromCurrentWorkflow(ctx, typeID); err == nil {
		cfg.Workflow = m
	}
	versions, err := c.versions.ListVersions(ctx, typeID)
	if err != nil {
		return WorkflowConfiguration{}, err
	}
	if versions != nil {
		cfg.Versions = versions
	}
	cfg.Readiness, err = c.readiness.Evaluate(ctx, typeID)
	if err != nil {
		return WorkflowConfiguration{}, err
	}
	cfg.Lifecycle, err = c.Lifecycle(ctx, typeID)
	if err != nil {
		return WorkflowConfiguration{}, err
	}

	gov := GovernanceInfo{Status: "draft", TotalVersions: len(versions)}
	if active := cfg.Lifecycle.Active; active != nil {
		gov.Status = "active"
		gov.ActiveVersionNo = &active.VersionNo
	}
	if pub := cfg.Lifecycle.Published; pub != nil {
		gov.PublishedVersionNo = &pub.VersionNo
	}
	cfg.Governance = gov
	return cfg, nil
}

// latestPublished returns the highest version that is published or active (the newest releasable one).
func latestPublished(versions []VersionInfo) *VersionInfo {
	var out *VersionInfo
	for i := range versions {
		v := versions[i]
		if v.State == VersionStatePublished || v.State == VersionStateActive {
			if out == nil || v.VersionNo > out.VersionNo {
				vv := v
				out = &vv
			}
		}
	}
	return out
}
