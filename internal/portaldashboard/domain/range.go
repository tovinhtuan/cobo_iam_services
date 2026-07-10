package domain

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

const DefaultTimezone = "Asia/Ho_Chi_Minh"

// DateRange is the resolved dashboard query window.
type DateRange struct {
	From     string // YYYY-MM-DD
	To       string // YYYY-MM-DD
	Preset   string // 7d|30d|90d|quarter|custom
	Timezone string
	Loc      *time.Location
	Now      time.Time
}

type ParseRangeInput struct {
	Range    string
	From     string
	To       string
	Timezone string
	Now      time.Time
}

func ParseRange(in ParseRangeInput) (DateRange, error) {
	preset := strings.TrimSpace(strings.ToLower(in.Range))
	if preset == "" {
		preset = "30d"
	}
	tz := strings.TrimSpace(in.Timezone)
	if tz == "" {
		tz = DefaultTimezone
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return DateRange{}, perr.NewHTTPError(http.StatusUnprocessableEntity, perr.Code("INVALID_RANGE"), "invalid timezone", nil)
	}
	now := in.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	nowLocal := now.In(loc)
	today := time.Date(nowLocal.Year(), nowLocal.Month(), nowLocal.Day(), 0, 0, 0, 0, loc)

	var from, to time.Time
	switch preset {
	case "7d":
		from = today.AddDate(0, 0, -6)
		to = today
	case "30d":
		from = today.AddDate(0, 0, -29)
		to = today
	case "90d":
		from = today.AddDate(0, 0, -89)
		to = today
	case "quarter":
		month := int(today.Month())
		qStartMonth := month - ((month - 1) % 3)
		from = time.Date(today.Year(), time.Month(qStartMonth), 1, 0, 0, 0, 0, loc)
		to = today
	case "custom":
		fromStr := strings.TrimSpace(in.From)
		toStr := strings.TrimSpace(in.To)
		if fromStr == "" || toStr == "" {
			return DateRange{}, perr.NewHTTPError(http.StatusUnprocessableEntity, perr.Code("INVALID_RANGE"), "from and to are required for custom range", nil)
		}
		from, err = time.ParseInLocation("2006-01-02", fromStr, loc)
		if err != nil {
			return DateRange{}, perr.NewHTTPError(http.StatusUnprocessableEntity, perr.Code("INVALID_RANGE"), "invalid from date", nil)
		}
		to, err = time.ParseInLocation("2006-01-02", toStr, loc)
		if err != nil {
			return DateRange{}, perr.NewHTTPError(http.StatusUnprocessableEntity, perr.Code("INVALID_RANGE"), "invalid to date", nil)
		}
		if from.After(to) {
			return DateRange{}, perr.NewHTTPError(http.StatusUnprocessableEntity, perr.Code("INVALID_RANGE"), "from must be on or before to", nil)
		}
	default:
		return DateRange{}, perr.NewHTTPError(http.StatusUnprocessableEntity, perr.Code("INVALID_RANGE"), fmt.Sprintf("unsupported range preset: %s", preset), nil)
	}

	return DateRange{
		From:     from.Format("2006-01-02"),
		To:       to.Format("2006-01-02"),
		Preset:   preset,
		Timezone: tz,
		Loc:      loc,
		Now:      nowLocal,
	}, nil
}

// Next7DaysWindow returns today..today+7 inclusive in business timezone.
func Next7DaysWindow(dr DateRange) (start, end string) {
	today := time.Date(dr.Now.Year(), dr.Now.Month(), dr.Now.Day(), 0, 0, 0, 0, dr.Loc)
	endDay := today.AddDate(0, 0, 7)
	return today.Format("2006-01-02"), endDay.Format("2006-01-02")
}
