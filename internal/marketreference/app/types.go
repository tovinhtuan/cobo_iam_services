package app

import (
	"context"
	"errors"
	"strings"
	"time"
)

// Profile source for listed company reference data (Phase 1: KBS pipeline only).
const ProfileSourceKBS = "kbs"

var (
	// ErrNotFound is returned when the symbol is absent from equity_list.
	ErrNotFound = errors.New("marketreference: listed company not found")
	// ErrInvalidRequest is returned for invalid list parameters (pagination, sort, etc.).
	ErrInvalidRequest = errors.New("marketreference: invalid request")
	// ErrUnavailable is returned when market reference is disabled or vnstock DB is unreachable.
	ErrUnavailable = errors.New("marketreference: service unavailable")
)

// ListedCompanyReader loads listed company rows from vnstock (read-only).
type ListedCompanyReader interface {
	List(ctx context.Context, p ListParams) (ListResult, error)
	GetBySymbol(ctx context.Context, symbol string) (ListedCompanyDetail, error)
}

// ListParams filters and paginates listed companies from vnstock.
type ListParams struct {
	Q           string
	Exchange    string
	HasProfile  *bool
	Page        int
	Limit       int
	SortBy      string
	SortOrder   string
}

// ListResult is a page of summaries plus total count (same filters as Count).
type ListResult struct {
	Items []ListedCompanySummary
	Total int
}

// ListedCompanySummary is a list row (no tax_id).
type ListedCompanySummary struct {
	Symbol           string
	CompanyName      string
	Exchange         string
	CompanyType      string
	BusinessModel    string
	ListingDate      *time.Time
	HasProfile       bool
	ProfileUpdatedAt *time.Time
	Source           string
}

// ListedCompanyDetail is the detail DTO (tax_id only under LegalContact).
type ListedCompanyDetail struct {
	Symbol           string
	CompanyName      string
	Source           string
	HasProfile       bool
	ProfileUpdatedAt *time.Time
	Identity         *IdentityGroup
	Timeline         *TimelineGroup
	Capital          *CapitalGroup
	Leadership       *LeadershipGroup
	LegalContact     *LegalContactGroup
}

type IdentityGroup struct {
	CompanyType   *string
	Exchange      *string
	BusinessModel *string
	BusinessCode  *string
}

type TimelineGroup struct {
	FoundedDate      *time.Time
	ListingDate      *time.Time
	AsOfDate         *time.Time
	ProfileUpdatedAt *time.Time
}

type CapitalGroup struct {
	CharterCapital       *float64
	ParValue             *float64
	ListingPrice         *float64
	ListedVolume         *float64
	OutstandingShares    *float64
	FreeFloat            *float64
	FreeFloatPercentage  *float64
}

type LeadershipGroup struct {
	CEOName             *string
	CEOPosition         *string
	InspectorName       *string
	InspectorPosition   *string
	NumberOfEmployees   *int64
}

type LegalContactGroup struct {
	EstablishmentLicense *string
	TaxID                *string
	Auditor              *string
	Address              *string
	Phone                *string
	Fax                  *string
	Email                *string
	Website              *string
}

// NormalizeListParams applies defaults and validates list input.
func NormalizeListParams(p ListParams) (ListParams, error) {
	p.Q = trimSpace(p.Q)
	p.Exchange = trimSpace(p.Exchange)

	if p.Page <= 0 {
		p.Page = 1
	}
	if p.Limit <= 0 {
		p.Limit = 20
	}
	if p.Limit > 100 {
		return ListParams{}, ErrInvalidRequest
	}

	p.SortBy = trimSpace(p.SortBy)
	if p.SortBy == "" {
		p.SortBy = "symbol"
	}
	switch p.SortBy {
	case "symbol", "company_name", "profile_updated_at":
	default:
		return ListParams{}, ErrInvalidRequest
	}

	p.SortOrder = strings.ToLower(trimSpace(p.SortOrder))
	if p.SortOrder == "" {
		p.SortOrder = "asc"
	}
	switch p.SortOrder {
	case "asc", "desc":
	default:
		return ListParams{}, ErrInvalidRequest
	}

	return p, nil
}

// NormalizeSymbol uppercases and trims a ticker symbol for lookups.
func NormalizeSymbol(symbol string) string {
	return strings.ToUpper(strings.TrimSpace(symbol))
}

func trimSpace(s string) string {
	return strings.TrimSpace(s)
}
