package app

import "context"

// Service appends immutable audit records.
type Service interface {
	AppendAuditLog(ctx context.Context, req AppendAuditLogRequest) error
}

// Repository persists audit events.
type Repository interface {
	Append(ctx context.Context, entry Entry) error
}

type AppendAuditLogRequest struct {
	EventID                      string
	OccurredAt                   string
	ActorUserID                  string
	ActorMembershipID            string
	CompanyID                    string
	Action                       string
	ResourceType                 string
	ResourceID                   string
	Decision                     string
	RequestID                    string
	IP                           string
	UserAgent                    string
	EffectivePermissionsSnapshot map[string]any
	EffectiveScopeSnapshot       map[string]any
	Metadata                     map[string]any
}

type Entry = AppendAuditLogRequest
