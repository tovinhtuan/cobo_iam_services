package app

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"net/mail"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/cobo/cobo_iam_services/internal/companyaccess/companystatus"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// Platform company safety (contract docs/ai-cache/cms-companies-safety-2026-10-11/02-contract.md):
// status changes need a reason (and the company code to deactivate), every mutation is audited in
// the same transaction with personal data masked, and writes can carry an expected revision.

// Audit actions for platform company mutations. The "cms.company" prefix also matches the existing
// "cms.company_plan.activate", so the company history lists plan activations too.
const (
	// CompanyAuditActionPrefix selects the company history (includes cms.company_plan.activate).
	CompanyAuditActionPrefix = "cms.company"
	// CompanyAuditTenantExcludedPrefix is hidden from tenant audit views (D-02: cms.company.* only).
	CompanyAuditTenantExcludedPrefix = "cms.company."

	CompanyAuditCreate             = "cms.company.create"
	CompanyAuditUpdate             = "cms.company.update"
	CompanyAuditVerificationChange = "cms.company.verification_change"
	CompanyAuditStatusChange       = "cms.company.status_change"
	companyAuditResourceType       = "company"
)

// Status actions and reason codes (design §1).
const (
	CompanyActionActivate   = "activate"
	CompanyActionSuspend    = "suspend"
	CompanyActionDeactivate = "deactivate"

	CompanyReasonPaymentOverdue      = "PAYMENT_OVERDUE"
	CompanyReasonPendingVerification = "PENDING_VERIFICATION"
	CompanyReasonSecurity            = "SECURITY"
	CompanyReasonCustomerRequest     = "CUSTOMER_REQUEST"
	CompanyReasonTermsViolation      = "TERMS_VIOLATION"
	CompanyReasonContractEnded       = "CONTRACT_ENDED"
	CompanyReasonPaymentReceived     = "PAYMENT_RECEIVED"
	CompanyReasonIssueResolved       = "ISSUE_RESOLVED"
	CompanyReasonOther               = "OTHER"

	companyReasonNoteMaxRunes = 500
)

var companyStatusReasonCodes = map[string]map[string]bool{
	CompanyActionSuspend: {
		CompanyReasonPaymentOverdue: true, CompanyReasonPendingVerification: true, CompanyReasonSecurity: true,
		CompanyReasonCustomerRequest: true, CompanyReasonTermsViolation: true, CompanyReasonOther: true,
	},
	CompanyActionDeactivate: {
		CompanyReasonPaymentOverdue: true, CompanyReasonSecurity: true, CompanyReasonCustomerRequest: true,
		CompanyReasonTermsViolation: true, CompanyReasonContractEnded: true, CompanyReasonOther: true,
	},
	CompanyActionActivate: {
		CompanyReasonCustomerRequest: true, CompanyReasonPaymentReceived: true, CompanyReasonIssueResolved: true,
		CompanyReasonOther: true,
	},
}

// Vietnamese labels used in the notification email (the FE has its own i18n labels).
var companyReasonLabelsVI = map[string]string{
	CompanyReasonPaymentOverdue:      "Nợ phí",
	CompanyReasonPendingVerification: "Chờ xác minh",
	CompanyReasonSecurity:            "Bảo mật / điều tra",
	CompanyReasonCustomerRequest:     "Khách hàng yêu cầu",
	CompanyReasonTermsViolation:      "Vi phạm điều khoản",
	CompanyReasonContractEnded:       "Chấm dứt hợp đồng",
	CompanyReasonPaymentReceived:     "Đã thanh toán",
	CompanyReasonIssueResolved:       "Đã xử lý xong",
	CompanyReasonOther:               "Khác",
}

var companyStatusLabelsVI = map[string]string{
	companystatus.StatusActive:    "Hoạt động",
	companystatus.StatusSuspended: "Tạm ngưng",
	companystatus.StatusInactive:  "Ngừng hoạt động",
}

// CompanyActionForStatus maps a target status to its action name.
func CompanyActionForStatus(status string) string {
	switch status {
	case companystatus.StatusActive:
		return CompanyActionActivate
	case companystatus.StatusSuspended:
		return CompanyActionSuspend
	default:
		return CompanyActionDeactivate
	}
}

// CompanySnapshot is the locked company row a mutation is computed from.
type CompanySnapshot struct {
	CompanyID          string
	CompanyCode        string
	CompanyName        string
	Status             string
	VerificationStatus string
	TaxCode            string
	RegistrationNumber string
	Address            string
	Phone              string
	ContactEmail       string
	RepresentativeName string
	UpdatedAt          time.Time
}

// CompanyRevision is an opaque content hash of the editable company state ("co1:" + 32 hex).
// It changes whenever any of these fields changes, even within the same second.
func CompanyRevision(s CompanySnapshot) string {
	parts := []string{
		s.CompanyID, s.Status, s.VerificationStatus, s.CompanyName, s.TaxCode, s.RegistrationNumber,
		s.Address, s.Phone, s.ContactEmail, s.RepresentativeName, s.UpdatedAt.UTC().Format(time.RFC3339),
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x1f")))
	return "co1:" + hex.EncodeToString(sum[:])[:32]
}

// SnapshotFromDetail rebuilds the revision input from a detail read (same columns as the locked read).
func SnapshotFromDetail(d *PlatformCompanyDetail) CompanySnapshot {
	updated, _ := time.Parse(time.RFC3339, d.UpdatedAtRFC3339)
	return CompanySnapshot{
		CompanyID: d.CompanyID, CompanyCode: d.CompanyCode, CompanyName: d.CompanyName, Status: d.Status,
		VerificationStatus: d.VerificationStatus, TaxCode: d.TaxCode, RegistrationNumber: d.RegistrationNumber,
		Address: d.Address, Phone: d.Phone, ContactEmail: d.ContactEmail, RepresentativeName: d.RepresentativeName,
		UpdatedAt: updated,
	}
}

// CompanyAuditContext identifies who performed a mutation (written on every audit row).
type CompanyAuditContext struct {
	ActorUserID       string
	ActorMembershipID string
	RequestID         string
	OccurredAt        time.Time
}

// CompanyAuditRecord is one audit_logs row written inside the mutation transaction.
type CompanyAuditRecord struct {
	EventID  string
	Action   string
	Metadata map[string]any
}

// CompanyMutation is what a MutateCompanyPlatform callback asks the repository to apply.
type CompanyMutation struct {
	Update *UpdatePlatformCompanyRequest // nil: no profile change
	Status string                        // "": no status change
	Audit  []CompanyAuditRecord
}

// PlatformCompanyStatusChange is returned by SetPlatformCompanyStatus (meta.status_change).
type PlatformCompanyStatusChange struct {
	Changed       bool   `json:"changed"`
	From          string `json:"from"`
	To            string `json:"to"`
	NotifiedCount int    `json:"notified_count"`
	AuditEventID  string `json:"audit_event_id,omitempty"`
}

// CompanyStatusEmail is one notification for the status_changed template; the emailer adds
// support/portal variables and dispatches it through the durable email pipeline.
type CompanyStatusEmail struct {
	To             string
	CompanyID      string
	ActorUserID    string
	IdempotencyKey string
	Variables      map[string]any
}

// CompanyStatusEmailer sends one company.status_changed email.
type CompanyStatusEmailer func(ctx context.Context, email CompanyStatusEmail) error

// ---------------------------------------------------------------- validation (contract §5)

var (
	companyTaxCodePattern = regexp.MustCompile(`^\d{10}(-\d{3})?$`)
	companyPhonePattern   = regexp.MustCompile(`^[0-9+().\-\s]{8,20}$`)
)

type companyFieldRule struct {
	maxRunes int
	check    func(string) bool
	message  string
}

var companyFieldRules = map[string]companyFieldRule{
	"company_name":        {maxRunes: 255, message: "Tên công ty tối đa 255 ký tự."},
	"tax_code":            {check: companyTaxCodePattern.MatchString, message: "Mã số thuế gồm 10 chữ số, hoặc 10 chữ số kèm -3 chữ số (vd. 0101234567-001)."},
	"registration_number": {maxRunes: 64, message: "Số ĐKKD tối đa 64 ký tự."},
	"address":             {maxRunes: 500, message: "Địa chỉ tối đa 500 ký tự."},
	"phone":               {check: companyPhonePattern.MatchString, message: "Số điện thoại 8–20 ký tự, chỉ gồm số và + ( ) . -"},
	"contact_email":       {maxRunes: 255, check: isValidCompanyEmail, message: "Email liên hệ không hợp lệ."},
	"representative_name": {maxRunes: 255, message: "Tên người đại diện tối đa 255 ký tự."},
}

func isValidCompanyEmail(v string) bool {
	addr, err := mail.ParseAddress(v)
	return err == nil && addr.Address == v && strings.Contains(v, "@")
}

// validateCompanyFields checks only the fields that were sent; empty values clear a field and are
// valid except for company_name.
func validateCompanyFields(fields map[string]*string) map[string]string {
	errs := map[string]string{}
	for field, ptr := range fields {
		if ptr == nil {
			continue
		}
		v := strings.TrimSpace(*ptr)
		if field == "company_name" && v == "" {
			errs[field] = "Tên công ty là bắt buộc."
			continue
		}
		if v == "" {
			continue
		}
		rule := companyFieldRules[field]
		if rule.maxRunes > 0 && utf8.RuneCountInString(v) > rule.maxRunes {
			errs[field] = rule.message
			continue
		}
		if rule.check != nil && !rule.check(v) {
			errs[field] = rule.message
		}
	}
	return errs
}

func companyFieldErrors(errs map[string]string) error {
	fieldErrors := make(map[string]any, len(errs))
	for k, v := range errs {
		fieldErrors[k] = v
	}
	return &perr.HTTPError{
		Code: perr.CodeInvalidRequest, Message: "invalid company fields", HTTPStatus: http.StatusBadRequest,
		Details: map[string]any{"field_errors": fieldErrors},
	}
}

func staleCompanyRevision(current string) error {
	return &perr.HTTPError{
		Code: perr.CodeStaleEtag, Message: "company was updated by someone else", HTTPStatus: http.StatusConflict,
		Details: map[string]any{"current_revision": current},
	}
}

// ---------------------------------------------------------------- masking (design §4)

var companyPersonalFields = map[string]bool{"contact_email": true, "phone": true, "representative_name": true}

func maskCompanyField(field, value string) string {
	value = strings.TrimSpace(value)
	if value == "" || !companyPersonalFields[field] {
		return value
	}
	switch field {
	case "contact_email":
		return maskEmail(value)
	case "phone":
		return maskPhone(value)
	default:
		return maskPersonName(value)
	}
}

func maskEmail(v string) string {
	at := strings.LastIndex(v, "@")
	if at <= 0 {
		return "***"
	}
	local, domain := v[:at], v[at+1:]
	maskPart := func(s string) string {
		r := []rune(s)
		if len(r) == 0 {
			return "***"
		}
		return string(r[0]) + "***"
	}
	tld := ""
	if dot := strings.LastIndex(domain, "."); dot > 0 {
		tld = domain[dot:]
		domain = domain[:dot]
	}
	return maskPart(local) + "@" + maskPart(domain) + tld
}

func maskPhone(v string) string {
	digits := make([]rune, 0, len(v))
	for _, r := range v {
		if r >= '0' && r <= '9' {
			digits = append(digits, r)
		}
	}
	if len(digits) <= 5 {
		return strings.Repeat("*", len(digits))
	}
	return string(digits[:2]) + strings.Repeat("*", len(digits)-5) + string(digits[len(digits)-3:])
}

func maskPersonName(v string) string {
	words := strings.Fields(v)
	out := make([]string, 0, len(words))
	for _, w := range words {
		r := []rune(w)
		out = append(out, strings.ToUpper(string(r[0]))+".")
	}
	return strings.Join(out, " ")
}

// companyChanges diffs the requested profile fields against the snapshot (masked for personal data).
func companyChanges(snap CompanySnapshot, req UpdatePlatformCompanyRequest) []map[string]any {
	current := map[string]string{
		"company_name": snap.CompanyName, "tax_code": snap.TaxCode, "registration_number": snap.RegistrationNumber,
		"address": snap.Address, "phone": snap.Phone, "contact_email": snap.ContactEmail,
		"representative_name": snap.RepresentativeName, "verification_status": snap.VerificationStatus,
	}
	requested := []struct {
		field string
		value *string
	}{
		{"company_name", req.CompanyName}, {"tax_code", req.TaxCode}, {"registration_number", req.RegistrationNumber},
		{"address", req.Address}, {"phone", req.Phone}, {"contact_email", req.ContactEmail},
		{"representative_name", req.RepresentativeName}, {"verification_status", req.VerificationStatus},
	}
	changes := make([]map[string]any, 0)
	for _, f := range requested {
		if f.value == nil {
			continue
		}
		after := strings.TrimSpace(*f.value)
		before := strings.TrimSpace(current[f.field])
		if after == before {
			continue
		}
		change := map[string]any{
			"field":  f.field,
			"before": maskCompanyField(f.field, before),
			"after":  maskCompanyField(f.field, after),
		}
		if companyPersonalFields[f.field] {
			change["masked"] = true
		}
		changes = append(changes, change)
	}
	return changes
}

// changedOnly keeps the requested fields whose value differs from the snapshot.
func changedOnly(snap CompanySnapshot, req UpdatePlatformCompanyRequest) UpdatePlatformCompanyRequest {
	keep := func(ptr *string, current string) *string {
		if ptr == nil {
			return nil
		}
		v := strings.TrimSpace(*ptr)
		if v == strings.TrimSpace(current) {
			return nil
		}
		return &v
	}
	out := req
	out.CompanyName = keep(req.CompanyName, snap.CompanyName)
	out.TaxCode = keep(req.TaxCode, snap.TaxCode)
	out.RegistrationNumber = keep(req.RegistrationNumber, snap.RegistrationNumber)
	out.Address = keep(req.Address, snap.Address)
	out.Phone = keep(req.Phone, snap.Phone)
	out.ContactEmail = keep(req.ContactEmail, snap.ContactEmail)
	out.RepresentativeName = keep(req.RepresentativeName, snap.RepresentativeName)
	out.VerificationStatus = keep(req.VerificationStatus, snap.VerificationStatus)
	return out
}

func hasNonAuditedUpdate(req UpdatePlatformCompanyRequest) bool {
	return req.IsListed != nil || req.IsLargePublic != nil || req.IsNonLargePublic != nil ||
		req.HasSubsidiaries != nil || req.HasSubordinateAccountingUnits != nil ||
		req.BusinessSectors != nil || req.BusinessSector != nil
}

func truncateRunes(v string, max int) string {
	r := []rune(v)
	if len(r) <= max {
		return v
	}
	return string(r[:max])
}

// ---------------------------------------------------------------- history (contract §4)

type ListPlatformCompanyHistoryRequest struct {
	Subject   AdminSubject
	CompanyID string
	Cursor    string
	Limit     int
}

type PlatformCompanyHistoryActor struct {
	UserID      string `json:"user_id"`
	DisplayName string `json:"display_name"`
}

type PlatformCompanyHistoryItem struct {
	EventID    string                      `json:"event_id"`
	OccurredAt string                      `json:"occurred_at"`
	Action     string                      `json:"action"`
	Actor      PlatformCompanyHistoryActor `json:"actor"`
	Before     map[string]any              `json:"before,omitempty"`
	After      map[string]any              `json:"after,omitempty"`
	ReasonCode string                      `json:"reason_code,omitempty"`
	ReasonNote string                      `json:"reason_note,omitempty"`
	Changes    []any                       `json:"changes"`
	Metadata   map[string]any              `json:"metadata,omitempty"`
}

// History cursors are opaque to clients: "h1." + base64url(occurred_at "|" event_id). A bare RFC3339 timestamp
// (cursor format before 2026-10-11) is still accepted and pages by second only.
const companyHistoryCursorPrefix = "h1."

func encodeCompanyHistoryCursor(occurredAt, eventID string) string {
	return companyHistoryCursorPrefix + base64.RawURLEncoding.EncodeToString([]byte(occurredAt+"|"+eventID))
}

func decodeCompanyHistoryCursor(raw string) (occurredAt, eventID string, err error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", nil
	}
	invalid := &perr.HTTPError{
		Code: perr.CodeInvalidRequest, Message: "invalid history cursor", HTTPStatus: http.StatusBadRequest,
		Details: map[string]any{"field_errors": map[string]any{"cursor": "Con trỏ phân trang không hợp lệ."}},
	}
	// audit_logs.occurred_at has second precision: only whole-second instants can match a row, so reject fractions
	// and hand the repository the canonical UTC form (the same form it emits).
	canonical := func(v string) (string, bool) {
		t, parseErr := time.Parse(time.RFC3339, v)
		if parseErr != nil || t.Nanosecond() != 0 {
			return "", false
		}
		return t.UTC().Format(time.RFC3339), true
	}
	if !strings.HasPrefix(raw, companyHistoryCursorPrefix) {
		at, ok := canonical(raw)
		if !ok {
			return "", "", invalid
		}
		return at, "", nil
	}
	decoded, decErr := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(raw, companyHistoryCursorPrefix))
	if decErr != nil {
		return "", "", invalid
	}
	rawAt, id, cut := strings.Cut(string(decoded), "|")
	at, ok := canonical(rawAt)
	if !cut || !ok || id == "" || len(id) > 64 {
		return "", "", invalid
	}
	return at, id, nil
}

type PlatformCompanyHistory struct {
	Items      []PlatformCompanyHistoryItem `json:"items"`
	Limit      int                          `json:"-"`
	NextCursor string                       `json:"-"`
}
