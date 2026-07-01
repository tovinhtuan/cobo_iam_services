package app

import "time"

const (
	DelegationScopeTypeDepartment = "department"
	DelegationStatusActive          = "active"
	DelegationStatusRevoked         = "revoked"
)

// DelegatableMembershipPermissions is the Phase 1 permission_set allowlist.
var DelegatableMembershipPermissions = []string{
	"admin.membership.invite",
	"admin.membership.create",
	"admin.membership.update",
	"admin.membership.delete",
}

// DelegatedAdminGrant is the persisted delegation grant record.
type DelegatedAdminGrant struct {
	DelegationID            string    `json:"delegation_id"`
	CompanyID               string    `json:"company_id"`
	DelegateeMembershipID   string    `json:"delegatee_membership_id"`
	DelegatorMembershipID   string    `json:"delegator_membership_id"`
	ScopeType               string    `json:"scope_type"`
	ScopeID                 string    `json:"scope_id"`
	PermissionSet           []string  `json:"permission_set"`
	Status                  string    `json:"status"`
	CreatedAt               time.Time `json:"created_at"`
	CreatedBy               string    `json:"created_by"`
	UpdatedAt               time.Time `json:"updated_at"`
	UpdatedBy               string    `json:"updated_by"`
}

type CreateDelegationRequest struct {
	Subject               AdminSubject
	DelegateeMembershipID string
	ScopeType             string
	ScopeID               string
	PermissionSet         []string
}

type ListDelegationsRequest struct {
	Subject               AdminSubject
	Status                string
	DelegateeMembershipID string
	ScopeID               string
	Limit                 int
}

type DelegationListView struct {
	Items []DelegatedAdminGrant `json:"items"`
	Total int                   `json:"total"`
}

type GetDelegationRequest struct {
	Subject      AdminSubject
	DelegationID string
}

type PatchDelegationRequest struct {
	Subject       AdminSubject
	DelegationID  string
	PermissionSet []string
}

type RevokeDelegationRequest struct {
	Subject      AdminSubject
	DelegationID string
}

type InsertDelegationGrantInput struct {
	ID                      string
	CompanyID               string
	DelegateeMembershipID   string
	DelegatorMembershipID   string
	ScopeType               string
	ScopeID                 string
	PermissionSet           []string
	CreatedBy               string
}
