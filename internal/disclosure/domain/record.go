package domain

import "time"

// Record is the main disclosure business object in tenant scope.
type Record struct {
	RecordID     string
	CompanyID    string
	DepartmentID string
	Title        string
	Content      string
	Status       string
	CreatedBy    string
	UpdatedBy    string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
