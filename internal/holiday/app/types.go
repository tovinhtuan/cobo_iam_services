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
	// Revision is the value to send back as expected_revision when saving (optimistic lock).
	Revision string `json:"revision"`
	Source   string `json:"source"`
}

// DefaultCalendarView is the built-in calendar (configs/non_trading_days/{year}.json) that
// applies when the year has no published calendar.
type DefaultCalendarView struct {
	CalendarYear int                `json:"calendar_year"`
	Source       string             `json:"source"`
	Dates        []HolidayDateEntry `json:"dates"`
}

// HolidayInputRow is one row sent by the CMS editor.
type HolidayInputRow struct {
	Date    string `json:"date"`
	Name    string `json:"name"`
	DayType string `json:"day_type"`
}

// ReplaceRequest publishes a calendar from an .xlsx file (Raw) or editor rows (Rows).
type ReplaceRequest struct {
	Year        int
	SourceName  string
	Description string
	Raw         []byte
	Rows        []HolidayInputRow
	FromEditor  bool
	// ExpectedRevision: nil = no check; "" = the year must have no calendar.
	ExpectedRevision *string
	Actor            Actor
}

// DeleteRequest removes a year's published calendar.
type DeleteRequest struct {
	Year             int
	Reason           string
	ExpectedRevision *string
	Actor            Actor
}

// Actor identifies who made a change, for the audit event.
type Actor struct {
	UserID       string
	MembershipID string
	CompanyID    string
	RequestID    string
	IP           string
	UserAgent    string
}

// ReplaceResult is returned after a publish.
type ReplaceResult struct {
	CalendarYear int    `json:"calendar_year"`
	Status       string `json:"status"`
	TotalDays    int    `json:"total_days"`
	Revision     string `json:"revision"`
	Added        int    `json:"added"`
	Removed      int    `json:"removed"`
}

// DeleteResult is returned after a calendar is removed.
type DeleteResult struct {
	CalendarYear int    `json:"calendar_year"`
	Deleted      bool   `json:"deleted"`
	Fallback     string `json:"fallback"`
}

// HolidayDateEntry is one stored holiday date.
type HolidayDateEntry struct {
	Date    string `json:"date"`
	DayType string `json:"day_type"`
	Name    string `json:"name,omitempty"`
}
