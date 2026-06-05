package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/cobo/cobo_iam_services/internal/marketreference/app"
)

const (
	profileSourceKBS = app.ProfileSourceKBS

	// exchangeExpr resolves listing exchange from equity_list then profile JSON (dev equity_list.exchange is often NULL).
	exchangeExpr = `COALESCE(NULLIF(TRIM(e.exchange), ''), NULLIF(TRIM(JSON_UNQUOTE(JSON_EXTRACT(p.info, '$.exchange'))), ''))`
)

var symbolQueryPattern = regexp.MustCompile(`^[A-Za-z0-9]{1,10}$`)

// Repository reads listed company reference data from vnstock MySQL (read-only).
type Repository struct {
	db *sql.DB
}

// NewRepository wires a vnstock read-only pool.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// List returns a page of summaries; Total uses the same filters (call Count for only total).
func (r *Repository) List(ctx context.Context, p app.ListParams) (app.ListResult, error) {
	p, err := app.NormalizeListParams(p)
	if err != nil {
		return app.ListResult{}, err
	}
	total, err := r.Count(ctx, p)
	if err != nil {
		return app.ListResult{}, err
	}
	items, err := r.listPage(ctx, p)
	if err != nil {
		return app.ListResult{}, err
	}
	return app.ListResult{Items: items, Total: total}, nil
}

// Count returns the number of rows matching list filters (no LIMIT/OFFSET).
func (r *Repository) Count(ctx context.Context, p app.ListParams) (int, error) {
	p, err := app.NormalizeListParams(p)
	if err != nil {
		return 0, err
	}
	where, args := buildListWhere(p)
	query := `
		SELECT COUNT(1)
		FROM equity_list e
		LEFT JOIN company_profiles p ON p.symbol = e.symbol AND p.source = ?
	` + where
	args = append([]any{profileSourceKBS}, args...)
	var n int
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&n); err != nil {
		return 0, fmt.Errorf("marketreference count: %w", err)
	}
	return n, nil
}

func (r *Repository) listPage(ctx context.Context, p app.ListParams) ([]app.ListedCompanySummary, error) {
	where, args := buildListWhere(p)
	order := buildListOrderBy(p)
	offset := (p.Page - 1) * p.Limit
	query := fmt.Sprintf(`
		SELECT
			e.symbol,
			e.company_name,
			%s AS exchange_resolved,
			NULLIF(TRIM(JSON_UNQUOTE(JSON_EXTRACT(p.info, '$.company_type'))), '') AS company_type,
			NULLIF(TRIM(JSON_UNQUOTE(JSON_EXTRACT(p.info, '$.business_model'))), '') AS business_model,
			NULLIF(TRIM(JSON_UNQUOTE(JSON_EXTRACT(p.info, '$.listing_date'))), '') AS listing_date_raw,
			(p.symbol IS NOT NULL) AS has_profile,
			p.updated_at AS profile_updated_at
		FROM equity_list e
		LEFT JOIN company_profiles p ON p.symbol = e.symbol AND p.source = ?
		%s
		ORDER BY %s
		LIMIT ? OFFSET ?
	`, exchangeExpr, where, order)
	args = append([]any{profileSourceKBS}, args...)
	args = append(args, p.Limit, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("marketreference list: %w", err)
	}
	defer rows.Close()

	var out []app.ListedCompanySummary
	for rows.Next() {
		var symbol, companyName string
		var exchange, companyType, businessModel, listingDateRaw sql.NullString
		var hasProfile bool
		var profileUpdatedAt sql.NullTime
		if err := rows.Scan(&symbol, &companyName, &exchange, &companyType, &businessModel, &listingDateRaw, &hasProfile, &profileUpdatedAt); err != nil {
			return nil, err
		}
		item := app.ListedCompanySummary{
			Symbol:        symbol,
			CompanyName:   companyName,
			Exchange:      nullString(exchange),
			CompanyType:   nullString(companyType),
			BusinessModel: nullString(businessModel),
			ListingDate:   parseOptionalTime(listingDateRaw),
			HasProfile:    hasProfile,
			Source:        profileSourceKBS,
		}
		if profileUpdatedAt.Valid {
			t := profileUpdatedAt.Time.UTC()
			item.ProfileUpdatedAt = &t
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if out == nil {
		out = []app.ListedCompanySummary{}
	}
	return out, nil
}

// GetBySymbol loads detail by uppercase symbol. Missing equity_list row returns app.ErrNotFound.
func (r *Repository) GetBySymbol(ctx context.Context, symbol string) (app.ListedCompanyDetail, error) {
	symbol = app.NormalizeSymbol(symbol)
	if symbol == "" {
		return app.ListedCompanyDetail{}, app.ErrInvalidRequest
	}

	const query = `
		SELECT
			e.symbol,
			e.company_name,
			e.exchange,
			p.info,
			p.updated_at,
			(p.symbol IS NOT NULL) AS has_profile
		FROM equity_list e
		LEFT JOIN company_profiles p ON p.symbol = e.symbol AND p.source = ?
		WHERE e.symbol = ?
	`
	var (
		rowSymbol, companyName string
		equityExchange         sql.NullString
		infoJSON               []byte
		profileUpdatedAt       sql.NullTime
		hasProfile             bool
	)
	err := r.db.QueryRowContext(ctx, query, profileSourceKBS, symbol).Scan(
		&rowSymbol, &companyName, &equityExchange, &infoJSON, &profileUpdatedAt, &hasProfile,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return app.ListedCompanyDetail{}, app.ErrNotFound
	}
	if err != nil {
		return app.ListedCompanyDetail{}, fmt.Errorf("marketreference detail: %w", err)
	}

	var profileAt *time.Time
	if profileUpdatedAt.Valid {
		t := profileUpdatedAt.Time.UTC()
		profileAt = &t
	}

	if !hasProfile {
		return app.ListedCompanyDetail{
			Symbol:           rowSymbol,
			CompanyName:      companyName,
			Source:           profileSourceKBS,
			HasProfile:       false,
			ProfileUpdatedAt: profileAt,
		}, nil
	}

	info, err := parseProfileInfo(infoJSON)
	if err != nil {
		return app.ListedCompanyDetail{}, fmt.Errorf("marketreference detail info json: %w", err)
	}
	return mapDetailFromProfile(rowSymbol, companyName, nullString(equityExchange), info, profileAt), nil
}

func buildListWhere(p app.ListParams) (string, []any) {
	var clauses []string
	var args []any

	if p.Exchange != "" {
		clauses = append(clauses, exchangeExpr+" = ?")
		args = append(args, p.Exchange)
	}
	if p.HasProfile != nil {
		if *p.HasProfile {
			clauses = append(clauses, "p.symbol IS NOT NULL")
		} else {
			clauses = append(clauses, "p.symbol IS NULL")
		}
	}
	if p.Q != "" {
		if symbolQueryPattern.MatchString(p.Q) {
			clauses = append(clauses, "UPPER(e.symbol) LIKE CONCAT(UPPER(?), '%')")
			args = append(args, p.Q)
		} else {
			clauses = append(clauses, "e.company_name LIKE CONCAT('%', ?, '%')")
			args = append(args, p.Q)
		}
	}
	if len(clauses) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

func buildListOrderBy(p app.ListParams) string {
	dir := strings.ToUpper(p.SortOrder)
	col := "e.symbol"
	switch p.SortBy {
	case "company_name":
		col = "e.company_name"
	case "profile_updated_at":
		col = "p.updated_at"
	}
	return col + " " + dir
}

// BuildListWhere exposes WHERE clause construction for unit tests.
func BuildListWhere(p app.ListParams) (string, []any) {
	return buildListWhere(p)
}

// NormalizeBusinessCode trims whitespace from a business registration code. Exported for tests.
func NormalizeBusinessCode(s string) string {
	return strings.TrimSpace(s)
}

// getByBusinessCodeQuery is the SQL for GetByBusinessCode. Exported as a constant for test verification.
//
// Uses JOIN (not LEFT JOIN): business_code lives in company_profiles.info, so a row without a profile
// cannot match. TRIM+JSON_UNQUOTE handles KBS data that may have surrounding whitespace.
const getByBusinessCodeQuery = `
	SELECT
		e.symbol,
		e.company_name,
		e.exchange,
		p.info,
		p.updated_at
	FROM equity_list e
	JOIN company_profiles p ON p.symbol = e.symbol AND p.source = ?
	WHERE TRIM(JSON_UNQUOTE(JSON_EXTRACT(p.info, '$.business_code'))) = ?
	LIMIT 1
`

// GetByBusinessCode looks up a listed company by its business registration code (mã ĐKKD / số ĐKKD).
// Returns app.ErrNotFound when no company_profiles row has a matching business_code.
// Returns app.ErrInvalidRequest when businessCode is empty after trimming.
func (r *Repository) GetByBusinessCode(ctx context.Context, businessCode string) (app.ListedCompanyDetail, error) {
	businessCode = NormalizeBusinessCode(businessCode)
	if businessCode == "" {
		return app.ListedCompanyDetail{}, app.ErrInvalidRequest
	}

	var (
		rowSymbol, companyName string
		equityExchange         sql.NullString
		infoJSON               []byte
		profileUpdatedAt       sql.NullTime
	)
	err := r.db.QueryRowContext(ctx, getByBusinessCodeQuery, profileSourceKBS, businessCode).Scan(
		&rowSymbol, &companyName, &equityExchange, &infoJSON, &profileUpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return app.ListedCompanyDetail{}, app.ErrNotFound
	}
	if err != nil {
		return app.ListedCompanyDetail{}, fmt.Errorf("marketreference business_code lookup: %w", err)
	}

	var profileAt *time.Time
	if profileUpdatedAt.Valid {
		t := profileUpdatedAt.Time.UTC()
		profileAt = &t
	}

	info, err := parseProfileInfo(infoJSON)
	if err != nil {
		return app.ListedCompanyDetail{}, fmt.Errorf("marketreference business_code info json: %w", err)
	}
	// JOIN guarantees has_profile=true — profile row exists by definition.
	return mapDetailFromProfile(rowSymbol, companyName, nullString(equityExchange), info, profileAt), nil
}

// SearchUsesSymbolPrefix reports whether q triggers symbol-prefix search.
func SearchUsesSymbolPrefix(q string) bool {
	q = strings.TrimSpace(q)
	return q != "" && symbolQueryPattern.MatchString(q)
}

func mapDetailFromProfile(symbol, companyName, equityExchange string, info map[string]any, profileAt *time.Time) app.ListedCompanyDetail {
	exchange := resolveExchange(equityExchange, info)
	detail := app.ListedCompanyDetail{
		Symbol:           symbol,
		CompanyName:      companyName,
		Source:           profileSourceKBS,
		HasProfile:       true,
		ProfileUpdatedAt: profileAt,
		Identity: &app.IdentityGroup{
			CompanyType:   stringPtr(info, "company_type"),
			Exchange:      stringPtrOr(exchange),
			BusinessModel: stringPtr(info, "business_model"),
			BusinessCode:  stringPtr(info, "business_code"),
		},
		Timeline: &app.TimelineGroup{
			FoundedDate:      timePtr(info, "founded_date"),
			ListingDate:      timePtr(info, "listing_date"),
			AsOfDate:         timePtr(info, "as_of_date"),
			ProfileUpdatedAt: profileAt,
		},
		Capital: &app.CapitalGroup{
			CharterCapital:      floatPtr(info, "charter_capital"),
			ParValue:            floatPtr(info, "par_value"),
			ListingPrice:        floatPtr(info, "listing_price"),
			ListedVolume:        floatPtr(info, "listed_volume"),
			OutstandingShares:   floatPtr(info, "outstanding_shares"),
			FreeFloat:           floatPtr(info, "free_float"),
			FreeFloatPercentage: floatPtr(info, "free_float_percentage"),
		},
		Leadership: &app.LeadershipGroup{
			CEOName:           stringPtr(info, "ceo_name"),
			CEOPosition:       stringPtr(info, "ceo_position"),
			InspectorName:     stringPtr(info, "inspector_name"),
			InspectorPosition: stringPtr(info, "inspector_position"),
			NumberOfEmployees: int64Ptr(info, "number_of_employees"),
		},
		LegalContact: &app.LegalContactGroup{
			EstablishmentLicense: stringPtr(info, "establishment_license"),
			TaxID:                stringPtr(info, "tax_id"),
			Auditor:              stringPtr(info, "auditor"),
			Address:              stringPtr(info, "address"),
			Phone:                stringPtr(info, "phone"),
			Fax:                  stringPtr(info, "fax"),
			Email:                stringPtr(info, "email"),
			Website:              stringPtr(info, "website"),
		},
	}
	return detail
}

// ResolveExchange applies COALESCE(equity_list.exchange, info.exchange) semantics in Go (for tests and detail mapping).
func ResolveExchange(equityExchange string, info map[string]any) string {
	return resolveExchange(equityExchange, info)
}

func resolveExchange(equityExchange string, info map[string]any) string {
	if s := strings.TrimSpace(equityExchange); s != "" {
		return s
	}
	if info == nil {
		return ""
	}
	if v, ok := info["exchange"]; ok && v != nil {
		if s := strings.TrimSpace(fmt.Sprint(v)); s != "" && s != "<nil>" {
			return s
		}
	}
	return ""
}

func parseProfileInfo(raw []byte) (map[string]any, error) {
	var info map[string]any
	if err := json.Unmarshal(raw, &info); err != nil {
		return nil, err
	}
	if info == nil {
		info = map[string]any{}
	}
	return info, nil
}

func nullString(ns sql.NullString) string {
	if ns.Valid {
		return strings.TrimSpace(ns.String)
	}
	return ""
}

func stringPtr(m map[string]any, key string) *string {
	if m == nil {
		return nil
	}
	v, ok := m[key]
	if !ok || v == nil {
		return nil
	}
	s := strings.TrimSpace(fmt.Sprint(v))
	if s == "" || s == "<nil>" {
		return nil
	}
	return &s
}

func stringPtrOr(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}

func floatPtr(m map[string]any, key string) *float64 {
	if m == nil {
		return nil
	}
	v, ok := m[key]
	if !ok || v == nil {
		return nil
	}
	switch n := v.(type) {
	case float64:
		return &n
	case float32:
		f := float64(n)
		return &f
	case int:
		f := float64(n)
		return &f
	case int64:
		f := float64(n)
		return &f
	case json.Number:
		f, err := n.Float64()
		if err != nil {
			return nil
		}
		return &f
	default:
		return nil
	}
}

func int64Ptr(m map[string]any, key string) *int64 {
	if m == nil {
		return nil
	}
	v, ok := m[key]
	if !ok || v == nil {
		return nil
	}
	switch n := v.(type) {
	case float64:
		i := int64(n)
		return &i
	case int:
		i := int64(n)
		return &i
	case int64:
		return &n
	case json.Number:
		i, err := n.Int64()
		if err != nil {
			return nil
		}
		return &i
	default:
		return nil
	}
}

func timePtr(m map[string]any, key string) *time.Time {
	s := stringPtr(m, key)
	if s == nil {
		return nil
	}
	return parseOptionalTime(sql.NullString{String: *s, Valid: true})
}

func parseOptionalTime(ns sql.NullString) *time.Time {
	if !ns.Valid {
		return nil
	}
	s := strings.TrimSpace(ns.String)
	if s == "" {
		return nil
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			u := t.UTC()
			return &u
		}
	}
	return nil
}
