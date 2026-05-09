package app

import "time"

// ParsedHolidayRow is one non-empty row from the uploaded sheet.
type ParsedHolidayRow struct {
	RowIndex int       `json:"row_index"`
	Date     time.Time `json:"date"`
	DayType  string    `json:"day_type"`
	Name     string    `json:"name,omitempty"`
}

// ParseIssue is a row-level validation problem.
type ParseIssue struct {
	Row     int    `json:"row"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message"`
}

// PreviewResult is returned for dry-run parse (no DB write).
type PreviewResult struct {
	CalendarYear  int                `json:"calendar_year"`
	Rows          []ParsedHolidayRow `json:"rows"`
	Warnings      []ParseIssue       `json:"warnings,omitempty"`
	Errors        []ParseIssue       `json:"errors,omitempty"`
	TotalAccepted int                `json:"total_accepted"`
	ContentSHA256 string             `json:"content_sha256"`
	SourceName    string             `json:"source_file_name,omitempty"`
}

// CalendarView is persisted calendar metadata + dates for GET.
type CalendarView struct {
	CalendarID     string             `json:"calendar_id"`
	CalendarYear   int                `json:"calendar_year"`
	SourceFileName string             `json:"source_file_name,omitempty"`
	ContentSHA256  string             `json:"content_sha256,omitempty"`
	TotalDays      int                `json:"total_days"`
	Description    string             `json:"description,omitempty"`
	UploadedBy     string             `json:"uploaded_by"`
	CreatedAt      time.Time          `json:"created_at"`
	UpdatedAt      time.Time          `json:"updated_at"`
	Dates          []HolidayDateEntry `json:"dates"`
}

// HolidayDateEntry is one stored holiday date.
type HolidayDateEntry struct {
	Date    string `json:"date"`
	DayType string `json:"day_type"`
	Name    string `json:"name,omitempty"`
}
