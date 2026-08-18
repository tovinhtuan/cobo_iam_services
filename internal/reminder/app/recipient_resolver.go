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
	Instructions    string
	AssigneeRoleIDs []string
	// AssigneeMembershipIDs are frozen direct assignees (company override / proposal snapshot).
	// When non-empty they outrank department-head resolution.
	AssigneeMembershipIDs []string
	DepartmentID          string
}

type recipientResolver struct {
	configReader       ConfigRepository
	stepReader         WorkflowStepReader
	instanceStepReader InstanceStepReader
	membershipQuerier  MembershipEmailQuerier
	taskAssigneeReader WorkflowTaskAssigneeReader
	log                *slog.Logger
}

// NewRecipientResolver constructs a RecipientResolver.
// All parameters may be nil — a nil component causes the corresponding path to return empty results.
func NewRecipientResolver(
	configReader ConfigRepository,
	stepReader WorkflowStepReader,
	membershipQuerier MembershipEmailQuerier,
	taskAssigneeReader WorkflowTaskAssigneeReader,
	log *slog.Logger,
) RecipientResolver {
	if log == nil {
		log = slog.Default()
	}
	return &recipientResolver{
		configReader:       configReader,
		stepReader:         stepReader,
		membershipQuerier:  membershipQuerier,
		taskAssigneeReader: taskAssigneeReader,
		log:                log,
	}
}

// SetInstanceStepReader attaches frozen-instance snapshot lookup. Optional.
func SetInstanceStepReader(r RecipientResolver, ir InstanceStepReader) {
	if impl, ok := r.(*recipientResolver); ok {
		impl.instanceStepReader = ir
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
// Frozen instance snapshot is preferred over live global_workflow_steps.
//
// Authority (COMPANY_OVERRIDE already frozen into the snapshot at materialize):
//  1. valid direct assignee memberships (exclusive)
//  2. matching company department + valid head → head only
//  3. matching company department + no valid head:
//     active department employees if any; otherwise Enterprise Admin
//  4. missing company department / no step → Enterprise Admin
//
// Assignee roles and pending task assignees are not recipient authorities.
func (r *recipientResolver) ResolveForWorkflowStep(ctx context.Context, companyID, workflowInstanceID, stepID string) ([]string, error) {
	if companyID == "" || stepID == "" {
		return nil, nil
	}
	stepCode := stepID
	if idx := strings.LastIndex(stepID, ":"); idx >= 0 {
		stepCode = stepID[idx+1:]
	}
	if r.membershipQuerier == nil {
		return nil, nil
	}
	step, err := r.lookupWorkflowStep(ctx, companyID, workflowInstanceID, stepCode)
	if err != nil {
		return nil, err
	}
	if step != nil && len(step.AssigneeMembershipIDs) > 0 {
		emails, err := r.membershipQuerier.EmailsByMemberships(ctx, companyID, step.AssigneeMembershipIDs)
		if err != nil {
			return nil, err
		}
		// Valid direct assignee is exclusive: do not evaluate department/CMS/admin.
		return deduplicateEmails(emails), nil
	}
	if step != nil && strings.TrimSpace(step.DepartmentID) != "" {
		matchedID, matched, err := r.membershipQuerier.ResolveCompanyDepartment(ctx, companyID, step.DepartmentID)
		if err != nil {
			return nil, err
		}
		if matched {
			emails, err := r.membershipQuerier.EmailsByDepartmentHead(ctx, companyID, matchedID)
			if err != nil {
				return nil, err
			}
			if len(emails) > 0 {
				return deduplicateEmails(emails), nil
			}
			employees, err := r.membershipQuerier.EmailsByDepartments(ctx, companyID, []string{matchedID})
			if err != nil {
				return nil, err
			}
			if len(employees) > 0 {
				return deduplicateEmails(employees), nil
			}
			return r.adminFallback(ctx, companyID)
		}
		return r.adminFallback(ctx, companyID)
	}
	return r.adminFallback(ctx, companyID)
}

func (r *recipientResolver) adminFallback(ctx context.Context, companyID string) ([]string, error) {
	if r.membershipQuerier == nil {
		return nil, nil
	}
	adminEmails, err := r.membershipQuerier.AdminEmailsByCompany(ctx, companyID)
	if err != nil {
		return nil, err
	}
	return deduplicateEmails(adminEmails), nil
}

func (r *recipientResolver) lookupWorkflowStep(ctx context.Context, companyID, workflowInstanceID, stepCode string) (*WorkflowStepConfig, error) {
	if r.instanceStepReader != nil && strings.TrimSpace(workflowInstanceID) != "" {
		step, err := r.instanceStepReader.GetStepByInstance(ctx, companyID, workflowInstanceID, stepCode)
		if err != nil {
			return nil, err
		}
		if step != nil {
			return step, nil
		}
	}
	if r.stepReader == nil {
		return nil, nil
	}
	return r.stepReader.GetStepByID(ctx, stepCode)
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
