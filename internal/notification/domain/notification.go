package domain

import "time"

type Job struct {
	NotificationJobID string
	CompanyID         string
	EventType         string
	ResourceType      string
	ResourceID        string
	Payload           map[string]any
	Status            string
	CreatedAt         time.Time
}

type Delivery struct {
	DeliveryID string
	JobID      string
	Recipient  string
	Status     string
	SentAt     *time.Time
}
