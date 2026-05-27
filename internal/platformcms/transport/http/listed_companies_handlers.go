package http

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	marketapp "github.com/cobo/cobo_iam_services/internal/marketreference/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/httpx"
)

var allowedListedExchanges = map[string]struct{}{
	"HOSE":  {},
	"HNX":   {},
	"UPCOM": {},
}

func (h *Handler) listListedCompanies(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.requireCMSAccess(r.Context(), sub.MembershipID, sub.CompanyID); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}

	params, err := parseListedCompaniesListQuery(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	out, err := h.marketListedCompanies().List(r.Context(), params)
	if err != nil {
		httpx.WriteError(w, nil, mapListedCompaniesError(err))
		return
	}

	items := make([]map[string]any, 0, len(out.Items))
	for _, item := range out.Items {
		items = append(items, listedCompanySummaryJSON(item))
	}
	writeEnvelope(w, http.StatusOK, map[string]any{"items": items}, map[string]any{
		"total": out.Total,
		"page":  params.Page,
		"limit": params.Limit,
	})
}

func (h *Handler) getListedCompany(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subject(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	if _, err := h.requireCMSAccess(r.Context(), sub.MembershipID, sub.CompanyID); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}

	symbol := marketapp.NormalizeSymbol(r.PathValue("symbol"))
	if symbol == "" {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "symbol is required", nil))
		return
	}
	detail, err := h.marketListedCompanies().GetDetail(r.Context(), symbol)
	if err != nil {
		httpx.WriteError(w, nil, mapListedCompaniesError(err))
		return
	}
	writeEnvelope(w, http.StatusOK, listedCompanyDetailJSON(detail), nil)
}

func parseListedCompaniesListQuery(r *http.Request) (marketapp.ListParams, error) {
	q := r.URL.Query()
	page, _ := strconv.Atoi(strings.TrimSpace(q.Get("page")))
	limit, _ := strconv.Atoi(strings.TrimSpace(q.Get("limit")))

	exchange := strings.ToUpper(strings.TrimSpace(q.Get("exchange")))
	if exchange != "" {
		if _, ok := allowedListedExchanges[exchange]; !ok {
			return marketapp.ListParams{}, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "exchange must be HOSE, HNX, or UPCOM", nil)
		}
	}

	hasProfile, err := parseOptionalBoolQuery(q.Get("has_profile"))
	if err != nil {
		return marketapp.ListParams{}, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "has_profile must be true or false", nil)
	}

	params := marketapp.ListParams{
		Q:          strings.TrimSpace(q.Get("q")),
		Exchange:   exchange,
		HasProfile: hasProfile,
		Page:       page,
		Limit:      limit,
		SortBy:     strings.TrimSpace(q.Get("sort_by")),
		SortOrder:  strings.TrimSpace(q.Get("sort_order")),
	}
	_, err = marketapp.NormalizeListParams(params)
	if err != nil {
		return marketapp.ListParams{}, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid list parameters", err)
	}
	return params, nil
}

func parseOptionalBoolQuery(raw string) (*bool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func mapListedCompaniesError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, marketapp.ErrUnavailable) {
		return marketUnavailableError()
	}
	if errors.Is(err, marketapp.ErrNotFound) {
		return perr.NewHTTPError(http.StatusNotFound, perr.CodeNotFound, "listed company not found", err)
	}
	if errors.Is(err, marketapp.ErrInvalidRequest) {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid request", err)
	}
	return marketUnavailableError()
}

func (h *Handler) marketListedCompanies() *marketapp.Service {
	return ensureListedCompaniesService(h.listedCompanies)
}

func marketUnavailableError() error {
	return perr.NewHTTPError(http.StatusServiceUnavailable, perr.CodeServiceUnavailable, "market reference data is unavailable", nil)
}

func listedCompanySummaryJSON(s marketapp.ListedCompanySummary) map[string]any {
	return map[string]any{
		"symbol":             s.Symbol,
		"company_name":       s.CompanyName,
		"exchange":           s.Exchange,
		"company_type":       s.CompanyType,
		"business_model":     s.BusinessModel,
		"listing_date":       formatTimePtr(s.ListingDate),
		"has_profile":        s.HasProfile,
		"profile_updated_at": formatTimePtr(s.ProfileUpdatedAt),
		"source":             s.Source,
	}
}

func listedCompanyDetailJSON(d marketapp.ListedCompanyDetail) map[string]any {
	out := map[string]any{
		"symbol":             d.Symbol,
		"company_name":       d.CompanyName,
		"source":             d.Source,
		"has_profile":        d.HasProfile,
		"profile_updated_at": formatTimePtr(d.ProfileUpdatedAt),
	}
	if d.Identity != nil {
		out["identity"] = map[string]any{
			"company_type":   d.Identity.CompanyType,
			"exchange":       d.Identity.Exchange,
			"business_model": d.Identity.BusinessModel,
			"business_code":  d.Identity.BusinessCode,
		}
	}
	if d.Timeline != nil {
		out["timeline"] = map[string]any{
			"founded_date":       formatTimePtr(d.Timeline.FoundedDate),
			"listing_date":       formatTimePtr(d.Timeline.ListingDate),
			"as_of_date":         formatTimePtr(d.Timeline.AsOfDate),
			"profile_updated_at": formatTimePtr(d.Timeline.ProfileUpdatedAt),
		}
	}
	if d.Capital != nil {
		out["capital"] = map[string]any{
			"charter_capital":        d.Capital.CharterCapital,
			"par_value":              d.Capital.ParValue,
			"listing_price":          d.Capital.ListingPrice,
			"listed_volume":          d.Capital.ListedVolume,
			"outstanding_shares":     d.Capital.OutstandingShares,
			"free_float":             d.Capital.FreeFloat,
			"free_float_percentage":  d.Capital.FreeFloatPercentage,
		}
	}
	if d.Leadership != nil {
		out["leadership"] = map[string]any{
			"ceo_name":              d.Leadership.CEOName,
			"ceo_position":          d.Leadership.CEOPosition,
			"inspector_name":        d.Leadership.InspectorName,
			"inspector_position":    d.Leadership.InspectorPosition,
			"number_of_employees":   d.Leadership.NumberOfEmployees,
		}
	}
	if d.LegalContact != nil {
		out["legal_contact"] = map[string]any{
			"establishment_license": d.LegalContact.EstablishmentLicense,
			"tax_id":                d.LegalContact.TaxID,
			"auditor":               d.LegalContact.Auditor,
			"address":               d.LegalContact.Address,
			"phone":                 d.LegalContact.Phone,
			"fax":                   d.LegalContact.Fax,
			"email":                 d.LegalContact.Email,
			"website":               d.LegalContact.Website,
		}
	}
	return out
}

func formatTimePtr(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.UTC().Format(timeLayout)
}
