package app

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
)

// WorkflowStepConfig is the minimal step data needed for recipient resolution and email templating.
type WorkflowStepConfig struct {
	StepID          string
	StageName       string
	AssigneeRoleIDs []string
	DepartmentID    string
}

type recipientResolver struct {
	configReader      ConfigRepository
	stepReader        WorkflowStepReader
	membershipQuerier MembershipEmailQuerier
	log               *slog.Logger
}

// NewRecipientResolver constructs a RecipientResolver.
// All parameters may be nil — a nil component causes the corresponding path to return empty results.
func NewRecipientResolver(
	configReader ConfigRepository,
	stepReader WorkflowStepReader,
	membershipQuerier MembershipEmailQuerier,
	log *slog.Logger,
) RecipientResolver {
	if log == nil {
		log = slog.Default()
	}
	return &recipientResolver{
		configReader:      configReader,
		stepReader:        stepReader,
		membershipQuerier: membershipQuerier,
		log:               log,
	}
}

// ResolveForDeadline expands recipients for a DISCLOSURE-scope occurrence.
// It reads reminder_config for the given scopeID and expands departments or uses direct emails.
// Always filters by companyID to enforce tenant isolation.
func (r *recipientResolver) ResolveForDeadline(ctx context.Context, companyID, scopeID string) ([]string, error) {
	if companyID == "" || scopeID == "" {
		return nil, nil
	}
	cfg, err := r.configReader.GetByScope(ctx, ScopeTypeDisclosure, scopeID)
	if err != nil || cfg == nil {
		return nil, nil // no config = no recipients
	}
	return r.expandConfig(ctx, companyID, cfg.Config)
}

// ResolveForWorkflowStep expands recipients for a WORKFLOW_STEP-scope occurrence.
// It reads the global_workflow_step by stepID and expands role-based membership.
// Always filters by companyID to enforce tenant isolation.
func (r *recipientResolver) ResolveForWorkflowStep(ctx context.Context, companyID, stepID string) ([]string, error) {
	if companyID == "" || stepID == "" {
		return nil, nil
	}
	if r.stepReader == nil || r.membershipQuerier == nil {
		return nil, nil
	}
	step, err := r.stepReader.GetStepByID(ctx, stepID)
	if err != nil || step == nil {
		return nil, nil
	}
	if len(step.AssigneeRoleIDs) == 0 {
		return nil, nil
	}
	emails, err := r.membershipQuerier.EmailsByRoles(ctx, companyID, step.AssigneeRoleIDs, step.DepartmentID)
	if err != nil {
		return nil, err
	}
	return deduplicateEmails(emails), nil
}

func (r *recipientResolver) expandConfig(ctx context.Context, companyID string, cfg ReminderConfigInput) ([]string, error) {
	var emails []string

	switch cfg.RecipientType {
	case ReminderRecipientTypeDepartments, ReminderRecipientTypeBoth:
		if r.membershipQuerier != nil && len(cfg.Departments) > 0 {
			deptEmails, err := r.membershipQuerier.EmailsByDepartments(ctx, companyID, cfg.Departments)
			if err != nil {
				return nil, err
			}
			emails = append(emails, deptEmails...)
		}
	}

	switch cfg.RecipientType {
	case ReminderRecipientTypeIndividuals, ReminderRecipientTypeBoth:
		for _, e := range cfg.Recipients {
			if e = strings.TrimSpace(e); e != "" {
				emails = append(emails, e)
			}
		}
	}

	return deduplicateEmails(emails), nil
}

func deduplicateEmails(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, e := range in {
		e = strings.TrimSpace(strings.ToLower(e))
		if e == "" {
			continue
		}
		if _, ok := seen[e]; ok {
			continue
		}
		seen[e] = struct{}{}
		out = append(out, e)
	}
	return out
}

// ParseAssigneeRoleIDs parses a JSON array of role ID strings.
// Used by globalWorkflowStepReader implementations.
func ParseAssigneeRoleIDs(rawJSON []byte) []string {
	var ids []string
	_ = json.Unmarshal(rawJSON, &ids)
	return ids
}
