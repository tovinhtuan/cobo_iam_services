package http

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	marketapp "github.com/cobo/cobo_iam_services/internal/marketreference/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/httpx"
)

const (
	lookupDisclaimer = "Thông tin được lấy từ dữ liệu công ty niêm yết công khai, chỉ mang tính tham khảo. Nền tảng không chịu trách nhiệm pháp lý về tính chính xác của thông tin."
)

// validateBusinessCode trims whitespace, enforces length (1–50) and allowlist [0-9A-Za-z\-/].
// Returns the trimmed code or error. The allowlist rejects SQL special chars, control chars,
// Unicode outside ASCII — preventing log injection, SQL injection variants, and cache key confusion.
func validateBusinessCode(raw string) (string, error) {
	code := strings.TrimSpace(raw)
	if len(code) == 0 {
		return "", perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "business_code is required", nil)
	}
	if len(code) > 50 {
		return "", perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "business_code exceeds maximum length", nil)
	}
	for _, c := range code {
		if !isAllowedBusinessCodeChar(c) {
			return "", perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "business_code contains invalid characters", nil)
		}
	}
	return code, nil
}

// isAllowedBusinessCodeChar allows alphanumeric, hyphen, and forward slash.
// This covers Vietnamese ĐKKD formats while rejecting SQL special chars and control chars.
func isAllowedBusinessCodeChar(c rune) bool {
	return (c >= '0' && c <= '9') ||
		(c >= 'A' && c <= 'Z') ||
		(c >= 'a' && c <= 'z') ||
		c == '-' || c == '/'
}

// safePrefix returns at most n runes from s — safe for logging (4 chars, no PII exposure).
func safePrefix(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}

// lookupMapErr maps marketapp errors to HTTP errors.
// Defined locally — not imported from platformcms which has its own package-private mapping.
func lookupMapErr(err error) error {
	if errors.Is(err, marketapp.ErrUnavailable) {
		return perr.NewHTTPError(http.StatusServiceUnavailable, perr.CodeServiceUnavailable, "market reference data is unavailable", nil)
	}
	if errors.Is(err, marketapp.ErrInvalidRequest) {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid business_code", nil)
	}
	return perr.NewHTTPError(http.StatusServiceUnavailable, perr.CodeServiceUnavailable, "market reference data is unavailable", nil)
}

// listedLookupByBusinessCode handles GET /api/v1/company/listed-lookup?business_code=...
//
// Public endpoint — no auth required. Used during company onboarding to pre-fill form fields.
// Rate-limited at Nginx layer (see deploy/nginx/rate-limit-lookup.conf.example).
//
// Response headers (P1-1 fix): Cache-Control is set per response type:
//   - 200 found:    public, max-age=3600 (profile data changes ≤monthly)
//   - 200 not_found: public, max-age=600  (short TTL — may be indexed later)
//   - 400/503:       no-store             (errors must not be cached by CDN/browser)
func (h *AdminHandler) listedLookupByBusinessCode(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	// Always set nosniff — prevents MIME-type sniffing on any response.
	w.Header().Set("X-Content-Type-Options", "nosniff")

	code, err := validateBusinessCode(r.URL.Query().Get("business_code"))
	if err != nil {
		w.Header().Set("Cache-Control", "no-store")
		slog.InfoContext(r.Context(), "listed_lookup_requested",
			slog.String("event", "listed_lookup_requested"),
			slog.String("request_id", httpx.RequestIDFromContext(r.Context())),
			slog.String("ip", r.RemoteAddr),
			slog.String("result", "error_invalid"),
			slog.Bool("cache_hit", false),
			slog.Int64("duration_ms", time.Since(start).Milliseconds()),
		)
		httpx.WriteError(w, nil, err)
		return
	}

	if h.listedLookup == nil {
		w.Header().Set("Cache-Control", "no-store")
		slog.InfoContext(r.Context(), "listed_lookup_requested",
			slog.String("event", "listed_lookup_requested"),
			slog.String("request_id", httpx.RequestIDFromContext(r.Context())),
			slog.String("ip", r.RemoteAddr),
			slog.String("business_code_prefix", safePrefix(code, 4)),
			slog.String("result", "unavailable"),
			slog.Bool("cache_hit", false),
			slog.Int64("duration_ms", time.Since(start).Milliseconds()),
		)
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusServiceUnavailable, perr.CodeServiceUnavailable, "market reference data is unavailable", nil))
		return
	}

	detail, cacheHit, svcErr := h.listedLookup.GetByBusinessCode(r.Context(), code)
	durationMs := time.Since(start).Milliseconds()

	result := "found"
	if errors.Is(svcErr, marketapp.ErrNotFound) {
		result = "not_found"
	} else if svcErr != nil {
		result = "unavailable"
	}

	// Audit log — no PII: only code prefix (4 chars), no email/phone/name.
	slog.InfoContext(r.Context(), "listed_lookup_requested",
		slog.String("event", "listed_lookup_requested"),
		slog.String("request_id", httpx.RequestIDFromContext(r.Context())),
		slog.String("ip", r.RemoteAddr),
		slog.String("user_agent", r.UserAgent()),
		slog.String("business_code_prefix", safePrefix(code, 4)),
		slog.String("result", result),
		slog.Bool("cache_hit", cacheHit),
		slog.Int64("duration_ms", durationMs),
	)

	if errors.Is(svcErr, marketapp.ErrNotFound) {
		w.Header().Set("Cache-Control", "public, max-age=600, stale-while-revalidate=60")
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"found": false})
		return
	}
	if svcErr != nil {
		w.Header().Set("Cache-Control", "no-store")
		httpx.WriteError(w, nil, lookupMapErr(svcErr))
		return
	}

	// Build response — omit nil fields from sync and preview objects.
	w.Header().Set("Cache-Control", "public, max-age=3600, stale-while-revalidate=300")
	httpx.WriteJSON(w, http.StatusOK, buildLookupResponse(detail))
}

// buildLookupResponse maps ListedCompanyDetail to the flat lookup response contract.
// Only non-empty fields are included in sync and preview objects.
func buildLookupResponse(d marketapp.ListedCompanyDetail) map[string]any {
	sync := map[string]any{}
	if d.CompanyName != "" {
		sync["company_name"] = d.CompanyName
	}
	if d.LegalContact != nil {
		if d.LegalContact.TaxID != nil && *d.LegalContact.TaxID != "" {
			sync["tax_code"] = *d.LegalContact.TaxID
		}
		if d.LegalContact.Address != nil && *d.LegalContact.Address != "" {
			sync["address"] = *d.LegalContact.Address
		}
		if d.LegalContact.Phone != nil && *d.LegalContact.Phone != "" {
			sync["phone"] = *d.LegalContact.Phone
		}
		if d.LegalContact.Email != nil && *d.LegalContact.Email != "" {
			sync["contact_email"] = *d.LegalContact.Email
		}
	}
	if d.Identity != nil {
		if d.Identity.BusinessCode != nil && *d.Identity.BusinessCode != "" {
			sync["registration_number"] = *d.Identity.BusinessCode
		}
	}

	preview := map[string]any{
		"symbol": d.Symbol,
	}
	if d.Identity != nil {
		if d.Identity.Exchange != nil && *d.Identity.Exchange != "" {
			preview["exchange"] = *d.Identity.Exchange
		}
		if d.Identity.CompanyType != nil && *d.Identity.CompanyType != "" {
			preview["company_type"] = *d.Identity.CompanyType
		}
	}
	if d.Timeline != nil && d.Timeline.ListingDate != nil {
		preview["listing_date"] = d.Timeline.ListingDate.Format("2006-01-02")
	}

	return map[string]any{
		"found":      true,
		"disclaimer": lookupDisclaimer,
		"sync":       sync,
		"preview":    preview,
	}
}
