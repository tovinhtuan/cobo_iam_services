package app

import (
	"strings"
	"time"
)

// Reason codes for materialization eligibility (stable API contract).
const (
	ReasonCompanyInactive         = "COMPANY_INACTIVE"
	ReasonEntitlementMissing      = "ENTITLEMENT_MISSING"
	ReasonApplicabilityNotMatched = "APPLICABILITY_NOT_MATCHED"
	ReasonAutoCreateDisabled      = "AUTO_CREATE_DISABLED"
	ReasonNotYetApplicable        = "NOT_YET_APPLICABLE"
	ReasonNoLongerApplicable      = "NO_LONGER_APPLICABLE"
	ReasonAlreadyMaterialized     = "ALREADY_MATERIALIZED"
	ReasonEligible                = "ELIGIBLE"
)

// EligibilityDecision is one company evaluated for Global CMS Record materialization.
type EligibilityDecision struct {
	CompanyID            string `json:"company_id"`
	CompanyName          string `json:"company_name,omitempty"`
	Eligible             bool   `json:"eligible"`
	ReasonCode           string `json:"reason_code"`
	ReasonMessage        string `json:"reason_message,omitempty"`
	Active               bool   `json:"active"`
	EntitlementStatus    string `json:"entitlement_status,omitempty"`
	ApplicabilityMatch   bool   `json:"applicability_match"`
	AutoCreateEnabled    bool   `json:"auto_create_enabled"`
	ApplicableFromOK     bool   `json:"applicable_from_ok"`
	ApplicableToOK       bool   `json:"applicable_to_ok"`
	AlreadyMaterialized  bool   `json:"already_materialized"`
	ExistingCompanyRecID string `json:"existing_company_record_id,omitempty"`
}

// ReasonMessageVI returns a short Vietnamese label for UI preview.
func ReasonMessageVI(code string) string {
	switch code {
	case ReasonCompanyInactive:
		return "Công ty không hoạt động"
	case ReasonEntitlementMissing:
		return "Thiếu gói đăng ký / entitlement hợp lệ"
	case ReasonApplicabilityNotMatched:
		return "Không áp dụng — loại hình doanh nghiệp không phù hợp"
	case ReasonAutoCreateDisabled:
		return "Tạm bỏ qua — công ty chưa bật tự động tạo hồ sơ"
	case ReasonNotYetApplicable:
		return "Chưa đến kỳ áp dụng (applicable_from)"
	case ReasonNoLongerApplicable:
		return "Đã hết kỳ áp dụng (applicable_to)"
	case ReasonAlreadyMaterialized:
		return "Đã tồn tại — không tạo trùng"
	case ReasonEligible:
		return "Đủ điều kiện"
	default:
		return code
	}
}

// IsPreSubmitCompanyStatus reports statuses that may be submitted into company review.
func IsPreSubmitCompanyStatus(status string) bool {
	s := strings.TrimSpace(status)
	return strings.EqualFold(s, "Draft") || strings.EqualFold(s, CompanyStatusNotStarted)
}

// MapCompanyStatusForCounts maps storage status into Global CMS company_counts buckets.
func MapCompanyStatusForCounts(status string) string {
	switch {
	case strings.EqualFold(status, CompanyStatusNotStarted), strings.EqualFold(status, "Draft"):
		return "not_started"
	case strings.EqualFold(status, "PendingReview"), strings.EqualFold(status, "Submitted"):
		return "pending_review"
	case strings.EqualFold(status, "Approved"):
		return "approved"
	case strings.EqualFold(status, "Published"):
		return "published"
	case strings.EqualFold(status, "Completed"):
		return "completed"
	case strings.EqualFold(status, "Failed"):
		return "failed"
	default:
		return "in_progress"
	}
}

// ParseCycleKeySlot extracts frequency unit and logical slot from cycle_key (e.g. quarterly:2026-Q3).
func ParseCycleKeySlot(cycleKey string) (freq, slot string, ok bool) {
	key := strings.TrimSpace(cycleKey)
	if i := strings.Index(key, "#archived:"); i >= 0 {
		key = key[:i]
	}
	parts := strings.SplitN(key, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return strings.ToLower(parts[0]), parts[1], true
}

// AsiaHCM returns Asia/Ho_Chi_Minh or UTC fallback.
func AsiaHCM() *time.Location {
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		return time.UTC
	}
	return loc
}
