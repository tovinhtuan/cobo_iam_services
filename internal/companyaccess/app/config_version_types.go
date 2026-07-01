package app

import "time"

// Config versioning types (Sprint 5 Batch 1B).

type ConfigVersionRow struct {
	ID            string    `json:"id"`
	CompanyID     string    `json:"company_id"`
	AggregateType string    `json:"aggregate_type"`
	AggregateID   string    `json:"aggregate_id,omitempty"`
	VersionNo     int       `json:"version_no"`
	CreatedBy     string    `json:"created_by"`
	CreatedAt     time.Time `json:"created_at"`
	Reason        string    `json:"reason,omitempty"`
	Source        string    `json:"source"`
}

type ConfigVersionDetail struct {
	ConfigVersionRow
	SnapshotJSON []byte `json:"-"`
}

type ConfigVersionListView struct {
	Items []ConfigVersionRow `json:"items"`
	Meta  map[string]any     `json:"meta"`
}

type CompareVersionsView struct {
	FromVersionNo int            `json:"from_version_no"`
	ToVersionNo   int            `json:"to_version_no"`
	Equal         bool           `json:"equal"`
	ChangedKeys   []string       `json:"changed_keys"`
	Summary       map[string]any `json:"summary"`
}

type InsertNotificationRuleVersionInput struct {
	ID           string
	CompanyID    string
	RuleID       string
	SnapshotJSON []byte
	CreatedBy    string
	Reason       string
	Source       string
}

type InsertRBACMatrixSnapshotInput struct {
	ID           string
	CompanyID    string
	SnapshotJSON []byte
	CreatedBy    string
	Reason       string
	Source       string
}

type ListNotificationRuleVersionsRequest struct {
	Subject AdminSubject
	RuleID  string
	Limit   int
}

type GetNotificationRuleVersionRequest struct {
	Subject   AdminSubject
	RuleID    string
	VersionNo int
}

type CompareNotificationRuleVersionsRequest struct {
	Subject       AdminSubject
	RuleID        string
	FromVersionNo int
	ToVersionNo   int
}

type RollbackNotificationRuleVersionRequest struct {
	Subject   AdminSubject
	RuleID    string
	VersionNo int
	Reason    string
}

type ListRBACMatrixVersionsRequest struct {
	Subject AdminSubject
	Limit   int
}

type GetRBACMatrixVersionRequest struct {
	Subject   AdminSubject
	VersionNo int
}

type CompareRBACMatrixVersionsRequest struct {
	Subject       AdminSubject
	FromVersionNo int
	ToVersionNo   int
}

type RollbackRBACMatrixVersionRequest struct {
	Subject   AdminSubject
	VersionNo int
	Reason    string
}
