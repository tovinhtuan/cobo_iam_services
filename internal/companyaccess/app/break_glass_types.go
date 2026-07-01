package app

import "time"

const (
	EmergencyScopeCompany = "company"

	EmergencyStatusPendingFirst  = "pending_first_approval"
	EmergencyStatusPendingSecond = "pending_second_approval"
	EmergencyStatusActive        = "active"
	EmergencyStatusExpired       = "expired"
	EmergencyStatusRevoked       = "revoked"
	EmergencyStatusDenied        = "denied"
	EmergencyStatusCancelled     = "cancelled"

	BGCapabilityApproveConfiguration  = "approve_configuration"
	BGCapabilityRollbackConfiguration = "rollback_configuration"
	BGCapabilityAdminSensitiveRead    = "admin_sensitive_read"

	defaultEmergencyDurationSeconds = 14400 // 4h per ADR-033
	maxEmergencyDurationSeconds     = 14400
	minEmergencyDurationSeconds     = 900 // 15m
)

// DefaultBreakGlassCapabilities is the fixed MVP capability bundle (not user-editable in UI).
var DefaultBreakGlassCapabilities = []string{
	BGCapabilityApproveConfiguration,
	BGCapabilityRollbackConfiguration,
	BGCapabilityAdminSensitiveRead,
}

// breakGlassCapabilityPermissions maps overlay capabilities to RBAC permission codes checked by hasPermission.
var breakGlassCapabilityPermissions = map[string][]string{
	BGCapabilityApproveConfiguration:  {"system.settings"},
	BGCapabilityRollbackConfiguration: {"system.settings"},
	BGCapabilityAdminSensitiveRead:    {"system.settings"},
}

// EmergencyAccessGrant is the break glass session record (M4).
type EmergencyAccessGrant struct {
	SessionID              string     `json:"session_id"`
	CompanyID              string     `json:"company_id"`
	TargetMembershipID     string     `json:"target_membership_id"`
	RequesterMembershipID  string     `json:"requester_membership_id"`
	ApproverMembershipID1  string     `json:"approver_membership_id_1,omitempty"`
	ApproverMembershipID2  string     `json:"approver_membership_id_2,omitempty"`
	Reason                 string     `json:"reason"`
	Scope                  string     `json:"scope"`
	CapabilitySet          []string   `json:"capability_set"`
	RequestedDurationSeconds int      `json:"requested_duration_seconds,omitempty"`
	Status                 string     `json:"status"`
	RequestedAt            time.Time  `json:"requested_at"`
	ActivatedAt            *time.Time `json:"activated_at,omitempty"`
	ExpiresAt              *time.Time `json:"expires_at,omitempty"`
	RevokedAt              *time.Time `json:"revoked_at,omitempty"`
}

type CreateEmergencyAccessRequest struct {
	Subject                 AdminSubject
	TargetMembershipID      string
	Reason                  string
	RequestedDurationSeconds int
}

type ListEmergencyAccessRequests struct {
	Subject            AdminSubject
	Status             string
	TargetMembershipID string
	Limit              int
}

type EmergencyAccessListView struct {
	Items []EmergencyAccessGrant `json:"items"`
	Total int                    `json:"total"`
}

type GetEmergencyAccessRequest struct {
	Subject   AdminSubject
	SessionID string
}

type ApproveEmergencyAccessRequest struct {
	Subject   AdminSubject
	SessionID string
}

type DenyEmergencyAccessRequest struct {
	Subject   AdminSubject
	SessionID string
}

type CancelEmergencyAccessRequest struct {
	Subject   AdminSubject
	SessionID string
}

type RevokeEmergencyAccessRequest struct {
	Subject   AdminSubject
	SessionID string
}

type GetEmergencyAccessTimelineRequest struct {
	Subject   AdminSubject
	SessionID string
	Limit     int
}

type InsertEmergencyAccessGrantInput struct {
	SessionID             string
	CompanyID             string
	TargetMembershipID    string
	RequesterMembershipID string
	Reason                string
	Scope                 string
	CapabilitySet         []string
	RequestedDurationSec  int
}
