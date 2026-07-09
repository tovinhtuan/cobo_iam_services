package app

import (
	"context"
)

// Readiness (Sprint 2 / Batch 1). Aggregates the EXISTING validation rules into granular checks for
// the FE readiness spine. It reuses ValidateManifestRoles / FindDuplicateStepKey / ValidateActiveIntegrity
// — no second validation implementation. Platform-scoped only (no tenant/company logic).

const (
	severityError   = "error"
	severityWarning = "warning"
)

// Readiness check keys.
const (
	CheckWorkflowExists  = "workflow_exists"
	CheckHasSteps        = "has_steps"
	CheckUniqueStepKey   = "unique_step_key"
	CheckValidRoles      = "valid_roles"
	CheckValidAssignees  = "valid_assignees"
	CheckSingleActiveVer = "single_active_version"
)

type ReadinessCheck struct {
	Key      string `json:"key"`
	Pass     bool   `json:"pass"`
	Severity string `json:"severity"` // error | warning
	Message  string `json:"message,omitempty"`
}

type ReadinessResult struct {
	Ready          bool             `json:"ready"`
	Score          int              `json:"score"`
	BlockingErrors []string         `json:"blocking_errors"`
	Warnings       []string         `json:"warnings"`
	Checks         []ReadinessCheck `json:"checks"`
}

type ReadinessService struct {
	versions *VersionService
	registry *RoleRegistry
	catalog  *AssigneeRoleCatalogService
}

func NewReadinessService(versions *VersionService, registry *RoleRegistry) *ReadinessService {
	if registry == nil {
		registry = DefaultRoleRegistry()
	}
	return &ReadinessService{versions: versions, registry: registry}
}

func (s *ReadinessService) WithCatalog(catalog *AssigneeRoleCatalogService) *ReadinessService {
	s.catalog = catalog
	return s
}

func (s *ReadinessService) registryFor(ctx context.Context) *RoleRegistry {
	if s.catalog == nil {
		return s.registry
	}
	reg, err := s.catalog.MergedRegistry(ctx)
	if err != nil || reg == nil {
		return s.registry
	}
	return reg
}

// Evaluate runs all readiness checks for a type and returns an aggregated result.
func (s *ReadinessService) Evaluate(ctx context.Context, typeID string) (ReadinessResult, error) {
	res := ReadinessResult{BlockingErrors: []string{}, Warnings: []string{}, Checks: []ReadinessCheck{}}
	add := func(key string, pass bool, severity, msg string) {
		res.Checks = append(res.Checks, ReadinessCheck{Key: key, Pass: pass, Severity: severity, Message: msg})
		if !pass {
			if severity == severityError {
				res.BlockingErrors = append(res.BlockingErrors, msgOr(msg, key))
			} else {
				res.Warnings = append(res.Warnings, msgOr(msg, key))
			}
		}
	}

	m, err := s.versions.BuildManifestFromCurrentWorkflow(ctx, typeID)
	if err != nil {
		add(CheckWorkflowExists, false, severityError, err.Error())
		return finalize(res), nil
	}
	add(CheckWorkflowExists, true, severityError, "")
	add(CheckHasSteps, len(m.Steps) > 0, severityError, "workflow has no steps")

	if dk := FindDuplicateStepKey(m); dk != "" {
		add(CheckUniqueStepKey, false, severityError, "duplicate step_key: "+dk)
	} else {
		add(CheckUniqueStepKey, true, severityError, "")
	}

	if roleErr := ValidateManifestRoles(m, s.registryFor(ctx)); roleErr != nil {
		add(CheckValidRoles, false, severityError, roleErr.Error())
	} else {
		add(CheckValidRoles, true, severityError, "")
	}

	// valid_assignees (platform-scoped): every step's role must be assignable (has an assignment
	// scope). Company-level membership resolution is tenant scope and out of this batch.
	assigneesOK, badStep := s.assigneesAssignable(ctx, m)
	add(CheckValidAssignees, assigneesOK, severityError, badStep)

	if intErr := s.versions.ValidateActiveIntegrity(ctx, typeID); intErr != nil {
		add(CheckSingleActiveVer, false, severityError, intErr.Error())
	} else {
		add(CheckSingleActiveVer, true, severityError, "")
	}

	return finalize(res), nil
}

func (s *ReadinessService) assigneesAssignable(ctx context.Context, m Manifest) (bool, string) {
	reg := s.registryFor(ctx)
	for _, st := range m.Steps {
		def, ok := reg.GetRole(st.Role)
		if !ok {
			return false, "step " + st.StepKey + " has unknown role"
		}
		if len(def.AllowedAssignmentScopes) == 0 {
			return false, "role " + def.Code + " is not assignable"
		}
	}
	return true, ""
}

func finalize(res ReadinessResult) ReadinessResult {
	total := len(res.Checks)
	pass := 0
	for _, c := range res.Checks {
		if c.Pass {
			pass++
		}
	}
	if total > 0 {
		res.Score = pass * 100 / total
	}
	res.Ready = len(res.BlockingErrors) == 0
	return res
}

func msgOr(msg, fallback string) string {
	if msg != "" {
		return msg
	}
	return fallback
}
