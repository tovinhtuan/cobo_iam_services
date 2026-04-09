package app

import "context"

type Service interface {
	ResolveRecipients(ctx context.Context, req ResolveRecipientsRequest) (*ResolveRecipientsResponse, error)
	EnqueueNotification(ctx context.Context, req EnqueueNotificationRequest) (*NotificationJobDTO, error)
	DispatchPending(ctx context.Context, req DispatchPendingRequest) (*DispatchPendingResponse, error)
}

type Repository interface {
	CreateJob(ctx context.Context, job NotificationJobDTO) (*NotificationJobDTO, error)
	ListPendingJobs(ctx context.Context, companyID string, limit int) ([]NotificationJobDTO, error)
	UpdateJobStatus(ctx context.Context, companyID, jobID, status string) error
	CreateDelivery(ctx context.Context, d NotificationDeliveryDTO) (*NotificationDeliveryDTO, error)
}

type Subject struct {
	UserID       string
	MembershipID string
	CompanyID    string
}

type ResolveRecipientsRequest struct {
	Subject      Subject
	EventType    string `json:"event_type"`
	ResourceType string `json:"resource_type"`
	ResourceID   string `json:"resource_id"`
}

type ResolveRecipientsResponse struct {
	Recipients []string `json:"recipients"`
}

type EnqueueNotificationRequest struct {
	Subject      Subject
	EventType    string         `json:"event_type"`
	ResourceType string         `json:"resource_type"`
	ResourceID   string         `json:"resource_id"`
	Payload      map[string]any `json:"payload"`
}

type DispatchPendingRequest struct {
	Subject Subject
	Limit   int `json:"limit,omitempty"`
}

type DispatchPendingResponse struct {
	Dispatched int `json:"dispatched"`
}

type NotificationJobDTO struct {
	NotificationJobID string         `json:"notification_job_id"`
	CompanyID         string         `json:"company_id"`
	EventType         string         `json:"event_type"`
	ResourceType      string         `json:"resource_type"`
	ResourceID        string         `json:"resource_id"`
	Payload           map[string]any `json:"payload"`
	Status            string         `json:"status"`
}

type NotificationDeliveryDTO struct {
	NotificationDeliveryID string `json:"notification_delivery_id"`
	NotificationJobID      string `json:"notification_job_id"`
	Recipient              string `json:"recipient"`
	Status                 string `json:"status"`
}
