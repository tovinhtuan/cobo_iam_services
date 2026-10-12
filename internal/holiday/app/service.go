package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	holidaymysql "github.com/cobo/cobo_iam_services/internal/holiday/infra/mysql"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

// Audit actions written in the same transaction as the calendar change.
const (
	AuditActionReplace = "cms.holiday_calendar.replace"
	AuditActionDelete  = "cms.holiday_calendar.delete"

	// MaxEditorRows caps a JSON calendar (a year has at most 366 days).
	MaxEditorRows       = 400
	maxReasonRunes      = 500
	maxDescriptionRunes = 2000
	maxSourceNameRunes  = 512
	exportSheetName     = "Lich"
)

// Service manages holiday calendar uploads and reads.
type Service interface {
	Preview(ctx context.Context, year int, sourceName string, raw []byte) (*PreviewResult, error)
	PreviewRows(ctx context.Context, year int, rows []HolidayInputRow) (*PreviewResult, error)
	Replace(ctx context.Context, req ReplaceRequest) (*ReplaceResult, error)
	Delete(ctx context.Context, req DeleteRequest) (*DeleteResult, error)
	Get(ctx context.Context, year int) (*CalendarView, error)
	Default(ctx context.Context, year int) (*DefaultCalendarView, error)
	ExportXLSX(ctx context.Context, year int, source string) ([]byte, error)
}

type service struct {
	repo       *holidaymysql.Repository
	dbCache    *holidaymysql.DBProvider
	idg        idgen.Generator
	location   *time.Location
	defaultDir string
	now        func() time.Time
}

// Option configures the service.
type Option func(*service)

// WithDefaultCalendarDir sets the directory of the built-in {year}.json calendars.
func WithDefaultCalendarDir(dir string) Option {
	return func(s *service) { s.defaultDir = dir }
}

// NewService wires holiday calendar admin operations.
func NewService(repo *holidaymysql.Repository, dbCache *holidaymysql.DBProvider, idg idgen.Generator, opts ...Option) Service {
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		loc = time.FixedZone("Asia/Ho_Chi_Minh", 7*60*60)
	}
	s := &service{
		repo:     repo,
		dbCache:  dbCache,
		idg:      idg,
		location: loc,
		now:      time.Now,
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

func checkYear(year int) error {
	if year < 2000 || year > 2100 {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "calendar_year out of range", nil)
	}
	return nil
}

func (s *service) Preview(ctx context.Context, year int, sourceName string, raw []byte) (*PreviewResult, error) {
	_ = ctx
	if err := checkYear(year); err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "empty file", nil)
	}
	sum := sha256.Sum256(raw)
	out := ParseHolidayXLSX(raw, year, s.location)
	out.ContentSHA256 = hex.EncodeToString(sum[:])
	out.SourceName = strings.TrimSpace(sourceName)
	return &out, nil
}

// datesFieldError reports a missing/oversized dates list (contract: details.field_errors.dates).
func datesFieldError(msg string) error {
	he := perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, msg, nil)
	he.Details = map[string]any{"field_errors": map[string]string{"dates": msg}}
	return he
}

func (s *service) PreviewRows(ctx context.Context, year int, rows []HolidayInputRow) (*PreviewResult, error) {
	_ = ctx
	if err := checkYear(year); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, datesFieldError("dates must contain at least one row")
	}
	if len(rows) > MaxEditorRows {
		return nil, datesFieldError(fmt.Sprintf("dates must contain at most %d rows", MaxEditorRows))
	}
	out := ValidateHolidayInput(rows, year, s.location)
	return &out, nil
}

func (s *service) Replace(ctx context.Context, req ReplaceRequest) (*ReplaceResult, error) {
	if strings.TrimSpace(req.Actor.UserID) == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "uploaded_by is required", nil)
	}
	if runeLen(strings.TrimSpace(req.Description)) > maxDescriptionRunes {
		he := perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, fmt.Sprintf("description must be at most %d characters", maxDescriptionRunes), nil)
		he.Details = map[string]any{"field_errors": map[string]string{"description": "too long"}}
		return nil, he
	}
	var (
		prev *PreviewResult
		err  error
	)
	source := "xlsx"
	if req.FromEditor {
		source = "editor"
		prev, err = s.PreviewRows(ctx, req.Year, req.Rows)
	} else {
		prev, err = s.Preview(ctx, req.Year, req.SourceName, req.Raw)
	}
	if err != nil {
		return nil, err
	}
	if len(prev.Errors) > 0 {
		he := perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "validation failed", nil)
		he.Details = map[string]any{"errors": prev.Errors}
		return nil, he
	}
	if prev.TotalAccepted == 0 {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "no holiday rows found", nil)
	}

	sourceName := truncateRunes(req.SourceName, maxSourceNameRunes)
	in := holidaymysql.ReplaceCalendarInput{
		CalendarID:       s.idg.NewUUID(),
		CalendarYear:     req.Year,
		SourceFileName:   sourceName,
		ContentSHA256:    prev.ContentSHA256,
		Description:      strings.TrimSpace(req.Description),
		UploadedBy:       strings.TrimSpace(req.Actor.UserID),
		ExpectedRevision: req.ExpectedRevision,
		Audit: s.auditEntry(req.Actor, AuditActionReplace, map[string]any{
			"year":             req.Year,
			"total_days":       prev.TotalAccepted,
			"source":           source,
			"source_file_name": sourceName,
		}),
	}
	for _, row := range prev.Rows {
		in.Dates = append(in.Dates, holidaymysql.HolidayDateRow{
			Date:    row.Date.In(s.location),
			DayType: row.DayType,
			Name:    row.Name,
		})
	}

	res, err := s.repo.ReplaceCalendar(ctx, in)
	if err != nil {
		return nil, fmt.Errorf("replace calendar: %w", err)
	}
	s.dbCache.InvalidateYear(req.Year)
	return &ReplaceResult{
		CalendarYear: req.Year,
		Status:       "replaced",
		TotalDays:    prev.TotalAccepted,
		Revision:     prev.ContentSHA256,
		Added:        res.Added,
		Removed:      res.Removed,
	}, nil
}

func (s *service) Delete(ctx context.Context, req DeleteRequest) (*DeleteResult, error) {
	if err := checkYear(req.Year); err != nil {
		return nil, err
	}
	reason := strings.TrimSpace(req.Reason)
	if runeLen(reason) > maxReasonRunes {
		he := perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, fmt.Sprintf("reason must be at most %d characters", maxReasonRunes), nil)
		he.Details = map[string]any{"field_errors": map[string]string{"reason": "too long"}}
		return nil, he
	}
	if _, err := s.repo.DeleteCalendar(ctx, holidaymysql.DeleteCalendarInput{
		CalendarYear:     req.Year,
		ExpectedRevision: req.ExpectedRevision,
		Audit:            s.auditEntry(req.Actor, AuditActionDelete, map[string]any{"year": req.Year, "reason": reason}),
	}); err != nil {
		return nil, err
	}
	s.dbCache.InvalidateYear(req.Year)
	fallback := "none"
	if _, err := s.loadDefault(req.Year); err == nil {
		fallback = "default"
	}
	return &DeleteResult{CalendarYear: req.Year, Deleted: true, Fallback: fallback}, nil
}

func (s *service) auditEntry(a Actor, action string, meta map[string]any) *holidaymysql.AuditEntry {
	return &holidaymysql.AuditEntry{
		EventID:           s.idg.NewUUID(),
		OccurredAt:        s.now(),
		ActorUserID:       a.UserID,
		ActorMembershipID: a.MembershipID,
		CompanyID:         a.CompanyID,
		Action:            action,
		RequestID:         a.RequestID,
		IP:                a.IP,
		UserAgent:         a.UserAgent,
		Metadata:          meta,
	}
}

func (s *service) Get(ctx context.Context, year int) (*CalendarView, error) {
	st, err := s.repo.GetCalendar(ctx, year)
	if err != nil {
		return nil, err
	}
	out := &CalendarView{
		CalendarID:     st.CalendarID,
		CalendarYear:   st.CalendarYear,
		TotalDays:      st.TotalDays,
		UploadedBy:     st.UploadedBy,
		CreatedAt:      st.CreatedAt,
		UpdatedAt:      st.UpdatedAt,
		SourceFileName: st.SourceFileName,
		ContentSHA256:  st.ContentSHA256,
		Description:    st.Description,
		Revision:       st.ContentSHA256,
		Source:         "db",
	}
	// Same fallback as the repository revision lock (content_sha256, else calendar_id).
	if out.Revision == "" {
		out.Revision = st.CalendarID
	}
	for _, d := range st.Dates {
		out.Dates = append(out.Dates, HolidayDateEntry{
			Date:    d.Date,
			DayType: d.DayType,
			Name:    d.Name,
		})
	}
	return out, nil
}

var errNoDefault = errors.New("no default calendar")

// loadDefault reads configs/non_trading_days/{year}.json (the calendar used when no DB calendar exists).
func (s *service) loadDefault(year int) (*DefaultCalendarView, error) {
	if s.defaultDir == "" {
		return nil, errNoDefault
	}
	raw, err := os.ReadFile(filepath.Join(s.defaultDir, fmt.Sprintf("%d.json", year)))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, errNoDefault
		}
		return nil, fmt.Errorf("read default holiday calendar %d: %w", year, err)
	}
	var file disclosureapp.HolidayCalendarYearFile
	if err := json.Unmarshal(raw, &file); err != nil {
		return nil, fmt.Errorf("parse default holiday calendar %d: %w", year, err)
	}
	out := &DefaultCalendarView{CalendarYear: year, Source: "default", Dates: []HolidayDateEntry{}}
	for _, h := range file.Holidays {
		if strings.TrimSpace(h.Date) == "" {
			continue
		}
		dt := strings.TrimSpace(h.Type)
		if dt == "" {
			dt = defaultDayType
		}
		out.Dates = append(out.Dates, HolidayDateEntry{Date: h.Date, DayType: dt, Name: h.Name})
	}
	sort.Slice(out.Dates, func(i, j int) bool { return out.Dates[i].Date < out.Dates[j].Date })
	return out, nil
}

func (s *service) Default(ctx context.Context, year int) (*DefaultCalendarView, error) {
	_ = ctx
	if err := checkYear(year); err != nil {
		return nil, err
	}
	out, err := s.loadDefault(year)
	if errors.Is(err, errNoDefault) {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeNotFound, "no default holiday calendar for year", nil)
	}
	return out, err
}

// ExportXLSX writes the calendar as an .xlsx the upload parser reads back unchanged:
// header row "Ngày | Tên | Loại", dates as dd/mm/yyyy text.
func (s *service) ExportXLSX(ctx context.Context, year int, source string) ([]byte, error) {
	if err := checkYear(year); err != nil {
		return nil, err
	}
	var dates []HolidayDateEntry
	switch source {
	case "", "db":
		cv, err := s.Get(ctx, year)
		if err != nil {
			return nil, err
		}
		dates = cv.Dates
	case "default":
		dv, err := s.Default(ctx, year)
		if err != nil {
			return nil, err
		}
		dates = dv.Dates
	default:
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "source must be db or default", nil)
	}

	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	if err := f.SetSheetName(f.GetSheetName(0), exportSheetName); err != nil {
		return nil, err
	}
	rows := [][]string{{"Ngày", "Tên", "Loại"}}
	for _, d := range dates {
		day, err := time.Parse("2006-01-02", d.Date)
		if err != nil {
			return nil, fmt.Errorf("export holiday date %q: %w", d.Date, err)
		}
		rows = append(rows, []string{day.Format("02/01/2006"), d.Name, d.DayType})
	}
	for i, r := range rows {
		for j, v := range r {
			cell, _ := excelize.CoordinatesToCellName(j+1, i+1)
			if err := f.SetCellStr(exportSheetName, cell, v); err != nil {
				return nil, err
			}
		}
	}
	_ = f.SetColWidth(exportSheetName, "A", "A", 14)
	_ = f.SetColWidth(exportSheetName, "B", "B", 40)
	_ = f.SetColWidth(exportSheetName, "C", "C", 24)
	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
