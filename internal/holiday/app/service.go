package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	holidaymysql "github.com/cobo/cobo_iam_services/internal/holiday/infra/mysql"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

// Service manages holiday calendar uploads and reads.
type Service interface {
	Preview(ctx context.Context, year int, sourceName string, raw []byte) (*PreviewResult, error)
	Replace(ctx context.Context, year int, sourceName, uploadedBy, description string, raw []byte) error
	Get(ctx context.Context, year int) (*CalendarView, error)
}

type service struct {
	repo     *holidaymysql.Repository
	dbCache  *holidaymysql.DBProvider
	idg      idgen.Generator
	location *time.Location
}

// NewService wires holiday calendar admin operations.
func NewService(repo *holidaymysql.Repository, dbCache *holidaymysql.DBProvider, idg idgen.Generator) Service {
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		loc = time.FixedZone("Asia/Ho_Chi_Minh", 7*60*60)
	}
	return &service{
		repo:     repo,
		dbCache:  dbCache,
		idg:      idg,
		location: loc,
	}
}

func (s *service) Preview(ctx context.Context, year int, sourceName string, raw []byte) (*PreviewResult, error) {
	_ = ctx
	if year < 2000 || year > 2100 {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "calendar_year out of range", nil)
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

func (s *service) Replace(ctx context.Context, year int, sourceName, uploadedBy, description string, raw []byte) error {
	if strings.TrimSpace(uploadedBy) == "" {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "uploaded_by is required", nil)
	}
	prev, err := s.Preview(ctx, year, sourceName, raw)
	if err != nil {
		return err
	}
	if len(prev.Errors) > 0 {
		he := perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "validation failed", nil)
		he.Details = map[string]any{"errors": prev.Errors}
		return he
	}
	if prev.TotalAccepted == 0 {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "no holiday rows found", nil)
	}

	in := holidaymysql.ReplaceCalendarInput{
		CalendarID:     s.idg.NewUUID(),
		CalendarYear:   year,
		SourceFileName: strings.TrimSpace(sourceName),
		ContentSHA256:  prev.ContentSHA256,
		Description:    strings.TrimSpace(description),
		UploadedBy:     strings.TrimSpace(uploadedBy),
	}
	for _, row := range prev.Rows {
		dt := row.DayType
		if dt == "" {
			dt = "PUBLIC_HOLIDAY"
		}
		in.Dates = append(in.Dates, holidaymysql.HolidayDateRow{
			Date:    row.Date.In(s.location),
			DayType: dt,
			Name:    row.Name,
		})
	}

	if err := s.repo.ReplaceCalendar(ctx, in); err != nil {
		return fmt.Errorf("replace calendar: %w", err)
	}
	s.dbCache.InvalidateYear(year)
	return nil
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
