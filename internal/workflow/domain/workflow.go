package domain

import "time"

// Instance represents one workflow execution inside a company.
type Instance struct {
	WorkflowInstanceID string
	CompanyID          string
	RecordID           string
	Status             string
	CurrentStepCode    string
	CreatedBy          string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// Task is actionable work item for a membership.
type Task struct {
	TaskID               string
	CompanyID            string
	WorkflowInstanceID   string
	StepCode             string
	AssigneeMembershipID string
	Status               string
	CreatedAt            time.Time
	UpdatedAt            time.Time
}
