package app

import (
	"context"
	"time"
)

type Service interface {
	CreateRecord(ctx context.Context, req CreateRecordRequest) (*RecordDTO, error)
	UpdateRecord(ctx context.Context, req UpdateRecordRequest) (*RecordDTO, error)
	SubmitRecord(ctx context.Context, req SubmitRecordRequest) (*RecordDTO, error)
	ConfirmRecord(ctx context.Context, req ConfirmRecordRequest) (*RecordDTO, error)
	ListRecords(ctx context.Context, req ListRecordsRequest) (*ListRecordsResponse, error)
	GetRecord(ctx context.Context, req GetRecordRequest) (*RecordDTO, error)
}

type Repository interface {
	Create(ctx context.Context, rec RecordDTO) (*RecordDTO, error)
	Update(ctx context.Context, rec RecordDTO) (*RecordDTO, error)
	FindByID(ctx context.Context, companyID, recordID string) (*RecordDTO, error)
	List(ctx context.Context, companyID string) ([]RecordDTO, error)
}

type CreateRecordRequest struct {
	Subject Subject
	Payload RecordPayload
}

type UpdateRecordRequest struct {
	Subject  Subject
	RecordID string
	Payload  RecordPayload
}

type SubmitRecordRequest struct {
	Subject  Subject
	RecordID string
}

type ConfirmRecordRequest struct {
	Subject  Subject
	RecordID string
}

type GetRecordRequest struct {
	Subject  Subject
	RecordID string
}

type ListRecordsRequest struct {
	Subject Subject
}

type ListRecordsResponse struct {
	Items []RecordDTO `json:"items"`
}

type Subject struct {
	UserID       string
	MembershipID string
	CompanyID    string
}

type RecordPayload struct {
	TypeID       string          `json:"type_id"`
	DepartmentID string          `json:"department_id"`
	Title        string          `json:"title"`
	Summary      string          `json:"summary"`
	Content      string          `json:"content"`
	PlannedDate  string          `json:"planned_date"`
	Attachments  []AttachmentDTO `json:"attachments"`
	EvidenceLink string          `json:"evidence_link"`
}

type AttachmentDTO struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	UploadedAt string `json:"uploaded_at"`
}

type RecordDTO struct {
	RecordID     string          `json:"record_id"`
	CompanyID    string          `json:"company_id"`
	TypeID       string          `json:"type_id"`
	DepartmentID string          `json:"department_id"`
	Title        string          `json:"title"`
	Summary      string          `json:"summary"`
	Content      string          `json:"content"`
	PlannedDate  string          `json:"planned_date,omitempty"`
	PublishedDate string         `json:"published_date,omitempty"`
	Status       string          `json:"status"`
	Attachments  []AttachmentDTO `json:"attachments"`
	EvidenceLink string          `json:"evidence_link,omitempty"`
	CreatedBy    string          `json:"created_by"`
	UpdatedBy    string          `json:"updated_by"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}
