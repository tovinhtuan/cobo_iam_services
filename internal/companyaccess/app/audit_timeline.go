package app

import (
	"context"
	"net/http"
	"time"

	auditapp "github.com/cobo/cobo_iam_services/internal/audit/app"
	"github.com/cobo/cobo_iam_services/internal/audit/timeline"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

const (
	auditListDefaultLimit = 50
	auditListMaxLimit     = 200
)

// AuditLogItem is one row in GET /api/v1/admin/audit-logs (CMS-compatible shape).
type AuditLogItem struct {
	EventID      string         `json:"event_id"`
	Action       string         `json:"action"`
	Actor        string         `json:"actor"`
	CompanyID    string         `json:"company_id"`
	ResourceType string         `json:"resource_type"`
	ResourceID   string         `json:"resource_id"`
	Decision     string         `json:"decision"`
	RequestID    string         `json:"request_id"`
	CreatedAt    string         `json:"created_at"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

// AuditLogsListView is returned by ListAuditLogs.
type AuditLogsListView struct {
	Items      []AuditLogItem `json:"items"`
	Total      int            `json:"total"`
	Limit      int            `json:"limit"`
	NextCursor string         `json:"next_cursor,omitempty"`
}

// ChangeTimelineView is returned by ListChangeTimeline.
type ChangeTimelineView struct {
	Items       []timeline.TimelineEvent `json:"items"`
	NextCursor  string                   `json:"next_cursor,omitempty"`
	EvaluatedAt time.Time                `json:"evaluated_at"`
}

type ListAuditLogsRequest struct {
	Subject        AdminSubject
	Limit          int
	Cursor         string
	Action         string
	ActionPrefix   bool
	ResourceType   string
	ResourceID     string
	FromOccurredAt string
	ToOccurredAt   string
}

type ListChangeTimelineRequest struct {
	Subject        AdminSubject
	Limit          int
	Cursor         string
	Action         string
	ActionPrefix   bool
	Domain         string
	ResourceType   string
	ResourceID     string
	FromOccurredAt string
	ToOccurredAt   string
}

func (s *adminService) ListAuditLogs(ctx context.Context, req ListAuditLogsRequest) (*AuditLogsListView, error) {
	if err := s.authorizeConfigurationHealth(ctx, req.Subject); err != nil {
		return nil, err
	}
	if s.auditRepo == nil {
		return nil, perr.NewHTTPError(http.StatusInternalServerError, perr.CodeInternal, "audit repository not configured", nil)
	}
	companyID := req.Subject.CompanyID
	if companyID == "" {
		return nil, perrNewBadRequest("company context required")
	}
	if err := validateOccurredAtQuery(req.FromOccurredAt, req.ToOccurredAt); err != nil {
		return nil, err
	}
	limit := normalizeAuditLimit(req.Limit)
	entries, err := s.auditRepo.ListFiltered(ctx, auditapp.ListFilter{
		CompanyID:      companyID,
		Action:         req.Action,
		ActionPrefix:   req.ActionPrefix,
		ResourceType:   req.ResourceType,
		ResourceID:     req.ResourceID,
		FromOccurredAt: req.FromOccurredAt,
		ToOccurredAt:   req.ToOccurredAt,
		Cursor:         req.Cursor,
		Limit:          limit,
	})
	if err != nil {
		return nil, err
	}
	items := make([]AuditLogItem, 0, len(entries))
	for _, e := range entries {
		if e.CompanyID != "" && e.CompanyID != companyID {
			continue
		}
		items = append(items, auditEntryToLogItem(e))
	}
	view := &AuditLogsListView{Items: items, Total: len(items), Limit: limit}
	if len(items) == limit && len(items) > 0 {
		view.NextCursor = items[len(items)-1].CreatedAt
	}
	return view, nil
}

func (s *adminService) ListChangeTimeline(ctx context.Context, req ListChangeTimelineRequest) (*ChangeTimelineView, error) {
	if err := s.authorizeConfigurationHealth(ctx, req.Subject); err != nil {
		return nil, err
	}
	if s.auditRepo == nil {
		return nil, perr.NewHTTPError(http.StatusInternalServerError, perr.CodeInternal, "audit repository not configured", nil)
	}
	companyID := req.Subject.CompanyID
	if companyID == "" {
		return nil, perrNewBadRequest("company context required")
	}
	if err := validateOccurredAtQuery(req.FromOccurredAt, req.ToOccurredAt); err != nil {
		return nil, err
	}
	limit := normalizeAuditLimit(req.Limit)
	filter := auditapp.ListFilter{
		CompanyID:      companyID,
		Action:         req.Action,
		ActionPrefix:   req.ActionPrefix,
		ResourceType:   req.ResourceType,
		ResourceID:     req.ResourceID,
		FromOccurredAt: req.FromOccurredAt,
		ToOccurredAt:   req.ToOccurredAt,
		Cursor:         req.Cursor,
		Limit:          limit,
	}
	if filter.Action == "" && !filter.ActionPrefix {
		filter.RequireAdminPrefix = true
	}
	entries, err := s.auditRepo.ListFiltered(ctx, filter)
	if err != nil {
		return nil, err
	}
	events := timeline.NormalizeBatch(entries)
	events = timeline.FilterByDomain(events, req.Domain)
	// Company safety: drop mismatched rows
	safe := make([]timeline.TimelineEvent, 0, len(events))
	for _, ev := range events {
		safe = append(safe, ev)
	}
	view := &ChangeTimelineView{
		Items:       safe,
		EvaluatedAt: time.Now().UTC(),
	}
	if len(safe) == limit && len(safe) > 0 {
		view.NextCursor = safe[len(safe)-1].OccurredAt
	}
	return view, nil
}

func auditEntryToLogItem(e auditapp.Entry) AuditLogItem {
	meta := timeline.SanitizeMetadata(e.Metadata)
	return AuditLogItem{
		EventID:      e.EventID,
		Action:       e.Action,
		Actor:        e.ActorUserID,
		CompanyID:    e.CompanyID,
		ResourceType: e.ResourceType,
		ResourceID:   e.ResourceID,
		Decision:     e.Decision,
		RequestID:    e.RequestID,
		CreatedAt:    e.OccurredAt,
		Metadata:     meta,
	}
}

func normalizeAuditLimit(limit int) int {
	if limit <= 0 {
		return auditListDefaultLimit
	}
	if limit > auditListMaxLimit {
		return auditListMaxLimit
	}
	return limit
}

func validateOccurredAtQuery(from, to string) error {
	if from != "" {
		if _, err := time.Parse(time.RFC3339, from); err != nil {
			return perrNewBadRequest("invalid from timestamp")
		}
	}
	if to != "" {
		if _, err := time.Parse(time.RFC3339, to); err != nil {
			return perrNewBadRequest("invalid to timestamp")
		}
	}
	return nil
}
