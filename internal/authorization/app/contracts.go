package app

import (
	"context"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

type Decision string

const (
	DecisionAllow Decision = "allow"
	DecisionDeny  Decision = "deny"
)

// Service centralizes authorization checks for all business modules.
type Service interface {
	Authorize(ctx context.Context, req AuthorizeRequest) (*AuthorizeDecision, error)
	AuthorizeBatch(ctx context.Context, req AuthorizeBatchRequest) (*AuthorizeBatchResponse, error)
	GetEffectiveAccess(ctx context.Context, membershipID, companyID string) (*EffectiveAccessSummary, error)
}

// Resolver loads effective permission/scope/responsibility for membership context.
type Resolver interface {
	Resolve(ctx context.Context, membershipID, companyID string) (*EffectiveAccessSummary, error)
}

// Repository port for authorization read models and mappings.
type Repository interface {
	ListPermissionCodes(ctx context.Context, membershipID, companyID string) ([]string, error)
	ListDepartmentScopes(ctx context.Context, membershipID, companyID string) ([]DepartmentScope, error)
	ListAssignments(ctx context.Context, membershipID, companyID string) ([]ResourceAssignment, error)
	ListResponsibilities(ctx context.Context, membershipID, companyID string) ([]string, error)
	ListPositionCodes(ctx context.Context, membershipID, companyID string) ([]string, error)
	ListOrgUnitIDs(ctx context.Context, membershipID, companyID string) ([]string, error)
	ListOrgSubtreeUnitIDs(ctx context.Context, membershipID, companyID string) ([]string, error)
	GetActionPolicy(ctx context.Context, companyID, action string) (*ActionPolicy, error)
}

// Checker applies decision policy based on effective access and request.
type Checker interface {
	Check(ctx context.Context, req AuthorizeRequest, effective *EffectiveAccessSummary, policy *ActionPolicy) (*AuthorizeDecision, error)
}

type AuthorizeRequest struct {
	Subject  SubjectRef     `json:"subject"`
	Action   string         `json:"action"`
	Resource ResourceRef    `json:"resource"`
	Context  map[string]any `json:"context,omitempty"`
}

type SubjectRef struct {
	UserID       string `json:"user_id"`
	MembershipID string `json:"membership_id"`
	CompanyID    string `json:"company_id"`
}

type ResourceRef struct {
	Type       string         `json:"type"`
	ID         string         `json:"id"`
	Attributes map[string]any `json:"attributes,omitempty"`
}

type AuthorizeDecision struct {
	Decision              Decision   `json:"decision"`
	MatchedPermissions    []string   `json:"matched_permissions"`
	ScopeReasons          []string   `json:"scope_reasons"`
	ResponsibilityReasons []string   `json:"responsibility_reasons"`
	DenyReasonCode        *perr.Code `json:"deny_reason_code"`
}

type AuthorizeBatchRequest struct {
	Subject SubjectRef            `json:"subject"`
	Checks  []AuthorizeBatchCheck `json:"checks"`
}

type AuthorizeBatchCheck struct {
	Action   string      `json:"action"`
	Resource ResourceRef `json:"resource"`
}

type AuthorizeBatchResponse struct {
	Results []AuthorizeDecision `json:"results"`
}

type EffectiveAccessSummary struct {
	CompanyID        string             `json:"company_id"`
	MembershipID     string             `json:"membership_id"`
	Permissions      []string           `json:"permissions"`
	DataScope        EffectiveDataScope `json:"data_scope"`
	Responsibilities []string           `json:"responsibilities"`
}

type EffectiveDataScope struct {
	ScopeType            string               `json:"scope_type"`
	Departments          []DepartmentScope    `json:"departments,omitempty"`
	RecordAssignments    []ResourceAssignment `json:"record_assignments,omitempty"`
	HasCompanyWideAccess bool                 `json:"has_company_wide_access"`
	OrgUnitIDs           []string             `json:"org_unit_ids,omitempty"`
	OrgSubtreeUnitIDs    []string             `json:"org_subtree_unit_ids,omitempty"`
	PositionCodes        []string             `json:"position_codes,omitempty"`
}

type DepartmentScope struct {
	DepartmentID   string `json:"department_id"`
	DepartmentName string `json:"department_name"`
}

type ResourceAssignment struct {
	ResourceType string `json:"resource_type"`
	ResourceID   string `json:"resource_id"`
}

type ActionPolicy struct {
	ActionCode          string    `json:"action_code"`
	RequiredPermission  string    `json:"required_permission"`
	ScopeType           string    `json:"scope_type"`
	WorkflowState       string    `json:"workflow_state"`
	EligibleActor       string    `json:"eligible_actor"`
	EffectType          string    `json:"effect_type"`
	DenyReasonCode      perr.Code `json:"deny_reason_code"`
}
