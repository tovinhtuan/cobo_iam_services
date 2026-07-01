package app

import (
	"encoding/json"
	"time"
)

// PendingAdminChange is a row in pending_admin_changes (M3).
type PendingAdminChange struct {
	ID                    string          `json:"approval_id"`
	CompanyID             string          `json:"company_id"`
	ApprovalSubjectType   string          `json:"approval_subject_type"`
	AggregateType         string          `json:"aggregate_type"`
	AggregateID           string          `json:"aggregate_id,omitempty"`
	ChangeType            string          `json:"change_type"`
	ProposedSnapshotJSON  json.RawMessage `json:"-"`
	BaseLiveVersionNo     *int            `json:"base_live_version_no,omitempty"`
	Status                string          `json:"status"`
	RequestedBy           string          `json:"requested_by"`
	RequestedAt           time.Time       `json:"requested_at"`
	ReviewedBy            string          `json:"reviewed_by,omitempty"`
	ReviewedAt            *time.Time      `json:"reviewed_at,omitempty"`
	Reason                string          `json:"reason,omitempty"`
	RejectReason          string          `json:"reject_reason,omitempty"`
}

// PendingAdminChangeSummary is the safe API view (no raw snapshot JSON).
type PendingAdminChangeSummary struct {
	ApprovalID        string     `json:"approval_id"`
	CompanyID         string     `json:"company_id"`
	AggregateType     string     `json:"aggregate_type"`
	AggregateID       string     `json:"aggregate_id,omitempty"`
	ChangeType        string     `json:"change_type"`
	BaseLiveVersionNo *int       `json:"base_live_version_no,omitempty"`
	Status            string     `json:"status"`
	RequestedBy       string     `json:"requested_by"`
	RequestedAt       time.Time  `json:"requested_at"`
	ReviewedBy        string     `json:"reviewed_by,omitempty"`
	ReviewedAt        *time.Time `json:"reviewed_at,omitempty"`
	Reason            string     `json:"reason,omitempty"`
	RejectReason      string     `json:"reject_reason,omitempty"`
	Summary           map[string]any `json:"summary,omitempty"`
}

type InsertPendingAdminChangeInput struct {
	ID                   string
	CompanyID            string
	ApprovalSubjectType  string
	AggregateType        string
	AggregateID          string
	ChangeType           string
	ProposedSnapshotJSON []byte
	BaseLiveVersionNo    *int
	RequestedBy          string
	Reason               string
}

type ListConfigApprovalsRequest struct {
	Subject       AdminSubject
	Status        string
	AggregateType string
	Limit         int
}

type ConfigApprovalListView struct {
	Items []PendingAdminChangeSummary `json:"items"`
}

type GetConfigApprovalRequest struct {
	Subject    AdminSubject
	ApprovalID string
}

type SubmitConfigApprovalRequest struct {
	Subject       AdminSubject
	AggregateType string
	AggregateID   string
	ChangeType    string
	Reason        string
	Proposed      map[string]any
}

type ApproveConfigApprovalRequest struct {
	Subject    AdminSubject
	ApprovalID string
}

type RejectConfigApprovalRequest struct {
	Subject      AdminSubject
	ApprovalID   string
	RejectReason string
}

type CancelConfigApprovalRequest struct {
	Subject    AdminSubject
	ApprovalID string
}

type CompareConfigApprovalRequest struct {
	Subject    AdminSubject
	ApprovalID string
}

type CompareConfigApprovalView struct {
	ApprovalID        string                `json:"approval_id"`
	AggregateType     string                `json:"aggregate_type"`
	AggregateID       string                `json:"aggregate_id,omitempty"`
	BaseLiveVersionNo *int                  `json:"base_live_version_no,omitempty"`
	CurrentVersionNo  int                   `json:"current_version_no"`
	Compare           *CompareVersionsView  `json:"compare"`
	Summary           map[string]any        `json:"summary,omitempty"`
}

type ApplyPendingApprovalInput struct {
	ApprovalID   string
	CompanyID    string
	ReviewedBy   string
	ActorUserID  string
	VersionRowID string
	CreatedBy    string
}

type ApplyPendingApprovalResult struct {
	PostApplyVersionNo int
}

func ToPendingSummary(row PendingAdminChange, summary map[string]any) PendingAdminChangeSummary {
	return PendingAdminChangeSummary{
		ApprovalID:        row.ID,
		CompanyID:         row.CompanyID,
		AggregateType:     row.AggregateType,
		AggregateID:       row.AggregateID,
		ChangeType:        row.ChangeType,
		BaseLiveVersionNo: row.BaseLiveVersionNo,
		Status:            row.Status,
		RequestedBy:       row.RequestedBy,
		RequestedAt:       row.RequestedAt,
		ReviewedBy:        row.ReviewedBy,
		ReviewedAt:        row.ReviewedAt,
		Reason:            row.Reason,
		RejectReason:      row.RejectReason,
		Summary:           summary,
	}
}
