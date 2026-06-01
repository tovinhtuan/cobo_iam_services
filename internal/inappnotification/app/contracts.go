package app

import (
	"context"
	"time"
)

// InAppNotification is an in-app notification record for a user.
type InAppNotification struct {
	ID           string
	UserID       string
	CompanyID    string
	Kind         string
	Title        string
	Body         string
	ResourceType *string
	ResourceID   *string
	IsRead       bool
	CreatedAt    time.Time
}

// Repository persists user_in_app_notifications rows.
type Repository interface {
	Create(ctx context.Context, n InAppNotification) error
	ListByUser(ctx context.Context, userID, companyID string, limit int) ([]InAppNotification, error)
	UnreadCount(ctx context.Context, userID, companyID string) (int, error)
	MarkRead(ctx context.Context, userID, notifID string) error
	MarkAllRead(ctx context.Context, userID, companyID string) error
}

// UserIDQuerier resolves user_ids from a company's active memberships by email.
// Used by InAppNotificationCreator to bridge reminder email recipients → user_ids.
type UserIDQuerier interface {
	UserIDsByEmails(ctx context.Context, companyID string, emails []string) ([]string, error)
}

// ReminderInAppRequest carries the context needed to create in-app notifications
// for a successfully dispatched reminder.
type ReminderInAppRequest struct {
	CompanyID       string
	Kind            string         // 'reminder.deadline_approaching' | 'reminder.workflow_step_due'
	Title           string
	Body            string
	ResourceType    string
	ResourceID      string
	RecipientEmails []string // already resolved by reminder service
}

// InAppNotificationCreator is implemented by Service and consumed by reminder service.
type InAppNotificationCreator interface {
	CreateForReminder(ctx context.Context, req ReminderInAppRequest) error
}

// InAppNotifier is implemented by Service and consumed by IAM service for auth events.
type InAppNotifier interface {
	CreateForUser(ctx context.Context, userID, companyID, kind, title, body string, resourceType, resourceID *string) error
}

// Service provides all in-app notification operations.
type Service interface {
	InAppNotificationCreator
	InAppNotifier
	List(ctx context.Context, userID, companyID string) ([]InAppNotification, error)
	UnreadCount(ctx context.Context, userID, companyID string) (int, error)
	MarkRead(ctx context.Context, userID, notifID string) error
	MarkAllRead(ctx context.Context, userID, companyID string) error
}

const (
	KindReminderDeadline    = "reminder.deadline_approaching"
	KindReminderWorkflow    = "reminder.workflow_step_due"
	KindAuthInvitation      = "auth.user_invitation_sent"
	KindAuthEmailVerif      = "auth.email_verification_requested"
	ResourceTypeDisclosure  = "disclosure"
)

const listLimit = 20
