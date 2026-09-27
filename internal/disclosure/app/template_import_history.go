package app

import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// ListTemplateImportHistoryRequest is a company-scoped, retention-filtered page.
type ListTemplateImportHistoryRequest struct {
	Subject    Subject
	Status     string
	Filename   string
	FileSHA256 string
	From       time.Time
	To         time.Time
	Cursor     string
	Limit      int
}

// TemplateImportHistoryItem is the list allowlist. No token hash and no raw payload.
type TemplateImportHistoryItem struct {
	ID                     string `json:"id"`
	Filename               string `json:"filename"`
	FileSHA256Fingerprint  string `json:"file_sha256_fingerprint"`
	Status                 string `json:"status"`
	ParseValid             bool   `json:"parse_valid"`
	DomainValid            bool   `json:"domain_valid"`
	ActivationReady        bool   `json:"activation_ready"`
	CanConfirm             bool   `json:"can_confirm"`
	MappingRequired        bool   `json:"mapping_required"`
	RequiredMappingCount   int    `json:"required_mapping_count"`
	ResolvedMappingCount   int    `json:"resolved_mapping_count"`
	UnresolvedMappingCount int    `json:"unresolved_mapping_count"`
	TargetTypeID           string `json:"target_type_id,omitempty"`
	CreatedTypeID          string `json:"created_type_id,omitempty"`
	ConfirmErrorCode       string `json:"confirm_error_code,omitempty"`
	CreatedAt              string `json:"created_at"`
	UpdatedAt              string `json:"updated_at"`
}

// ListTemplateImportHistoryResponse is the history page.
type ListTemplateImportHistoryResponse struct {
	Items      []TemplateImportHistoryItem
	Limit      int
	NextCursor string
}

// TemplateImportHistoryDetail is the detail allowlist.
type TemplateImportHistoryDetail struct {
	TemplateImportHistoryItem
	FileSHA256             string                             `json:"file_sha256"`
	CanonicalPayloadSHA256 string                             `json:"canonical_payload_sha256,omitempty"`
	SchemaVersion          string                             `json:"schema_version,omitempty"`
	ErrorCodes             []TemplateImportValidationIssueDTO `json:"error_codes"`
	MappingSummary         []TemplateImportRequiredMappingDTO `json:"mapping_summary"`
	ActorUserID            string                             `json:"actor_user_id"`
	ActorMembershipID      string                             `json:"actor_membership_id,omitempty"`
	FileSizeBytes          int                                `json:"file_size_bytes"`
	ValidatedAt            string                             `json:"validated_at,omitempty"`
	ConfirmedAt            string                             `json:"confirmed_at,omitempty"`
}

// GetTemplateImportHistoryRequest loads one attempt inside the caller's company and retention window.
type GetTemplateImportHistoryRequest struct {
	Subject Subject
	ID      string
}

func (s *service) ListTemplateImportHistory(ctx context.Context, req ListTemplateImportHistoryRequest) (*ListTemplateImportHistoryResponse, error) {
	if err := s.requireHistoryRead(ctx, req.Subject); err != nil {
		return nil, err
	}
	if !ImportHistoryEnabled() {
		return nil, featureDisabled()
	}
	store, ok := importAttemptStore(s.repo)
	if !ok {
		return nil, historyUnavailable()
	}
	if req.Limit <= 0 {
		req.Limit = 20
	}
	if req.Limit > 100 {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "limit must be between 1 and 100", nil)
	}
	req.Status = strings.TrimSpace(req.Status)
	if req.Status != "" && !validImportAttemptStatus(req.Status) {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "unsupported status filter", nil)
	}
	req.FileSHA256 = strings.ToLower(strings.TrimSpace(req.FileSHA256))
	if req.FileSHA256 != "" && len(req.FileSHA256) != 64 {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "file_sha256 must be 64 hex characters", nil)
	}
	rows, err := store.ListImportAttempts(ctx, req)
	if err != nil {
		return nil, err
	}
	limit := req.Limit
	next := ""
	if len(rows) > limit {
		last := rows[limit-1]
		next = encodeImportHistoryCursor(last.CreatedAt, last.ID)
		rows = rows[:limit]
	}
	items := make([]TemplateImportHistoryItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, historyItem(row))
	}
	return &ListTemplateImportHistoryResponse{Items: items, Limit: limit, NextCursor: next}, nil
}

func (s *service) GetTemplateImportHistory(ctx context.Context, req GetTemplateImportHistoryRequest) (*TemplateImportHistoryDetail, error) {
	if err := s.requireHistoryRead(ctx, req.Subject); err != nil {
		return nil, err
	}
	if !ImportHistoryEnabled() {
		return nil, featureDisabled()
	}
	store, ok := importAttemptStore(s.repo)
	if !ok {
		return nil, historyUnavailable()
	}
	row, err := store.GetImportAttemptByID(ctx, strings.TrimSpace(req.ID))
	if err != nil {
		return nil, err
	}
	if row == nil || row.CompanyID != req.Subject.CompanyID || row.CreatedAt.Before(templateImportRetentionCutoff(time.Now())) {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeImportAttemptNotFound, "import attempt not found", nil)
	}
	item := historyItem(*row)
	detail := TemplateImportHistoryDetail{
		TemplateImportHistoryItem: item,
		FileSHA256:                row.FileSHA256,
		CanonicalPayloadSHA256:    row.CanonicalPayloadSHA256,
		SchemaVersion:             row.SchemaVersion,
		ErrorCodes:                redactIssueCodes(row.ErrorCodes),
		MappingSummary:            row.MappingSummary,
		ActorUserID:               row.ActorUserID,
		ActorMembershipID:         row.ActorMembershipID,
		FileSizeBytes:             row.FileSizeBytes,
	}
	if !row.ValidatedAt.IsZero() {
		detail.ValidatedAt = row.ValidatedAt.UTC().Format(time.RFC3339)
	}
	if !row.ConfirmedAt.IsZero() {
		detail.ConfirmedAt = row.ConfirmedAt.UTC().Format(time.RFC3339)
	}
	if detail.ErrorCodes == nil {
		detail.ErrorCodes = []TemplateImportValidationIssueDTO{}
	}
	if detail.MappingSummary == nil {
		detail.MappingSummary = []TemplateImportRequiredMappingDTO{}
	}
	return &detail, nil
}

func (s *service) requireHistoryRead(ctx context.Context, sub Subject) error {
	if err := s.requireCMSRouteAccess(ctx, sub); err != nil {
		return err
	}
	if strings.TrimSpace(sub.CompanyID) == "" {
		return perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "company scope is required", nil)
	}
	return nil
}

func featureDisabled() error {
	return perr.NewHTTPError(http.StatusNotFound, perr.CodeFeatureDisabled, "FEATURE_DISABLED", nil)
}

func historyUnavailable() error {
	return perr.NewHTTPError(http.StatusServiceUnavailable, perr.CodeServiceUnavailable, "import history store is unavailable", nil)
}

func (s *service) attachImportAttempt(ctx context.Context, req ValidateTemplateImportRequest, resp *ValidateTemplateImportResponse, schemaVersion, canonicalHash, token string) (*ValidateTemplateImportResponse, error) {
	if !ImportHistoryEnabled() {
		return resp, nil
	}
	id, err := s.persistValidateAttempt(ctx, req, resp, schemaVersion, canonicalHash, token, nil)
	if err != nil {
		return nil, err
	}
	if resp != nil {
		resp.ImportAttemptID = id
	}
	return resp, nil
}

func (s *service) attachImportAttemptError(ctx context.Context, req ValidateTemplateImportRequest, schemaVersion string, herr error) error {
	if !ImportHistoryEnabled() || herr == nil {
		return herr
	}
	resp := &ValidateTemplateImportResponse{ParseValid: true, DomainValid: false, CanConfirm: false}
	id, err := s.persistValidateAttempt(ctx, req, resp, schemaVersion, "", "", herr)
	if err != nil {
		return err
	}
	if he, ok := perr.AsHTTPError(herr); ok && he != nil {
		if he.Details == nil {
			he.Details = map[string]any{}
		}
		he.Details["import_attempt_id"] = id
	}
	return herr
}

func (s *service) persistValidateAttempt(ctx context.Context, req ValidateTemplateImportRequest, resp *ValidateTemplateImportResponse, schemaVersion, canonicalHash, token string, _ error) (string, error) {
	store, ok := importAttemptStore(s.repo)
	if !ok {
		return "", historyUnavailable()
	}
	name, err := safeImportBasename(req.Filename)
	if err != nil {
		return "", err
	}
	now := time.Now().UTC()
	status := ImportAttemptStatusValidationFailed
	unresolved := 0
	if resp != nil && resp.ParseValid && resp.DomainValid {
		for _, m := range resp.RequiredMappings {
			if !m.IsAutoMatched && strings.TrimSpace(m.TargetID) == "" {
				unresolved++
			}
		}
		if unresolved > 0 {
			status = ImportAttemptStatusMappingRequired
		} else {
			status = ImportAttemptStatusValidated
		}
	}
	required := 0
	if resp != nil {
		required = len(resp.RequiredMappings)
	}
	resolved := required - unresolved
	if resolved < 0 {
		resolved = 0
	}
	row := &TemplateImportAttempt{
		ID:                     s.newImportID(),
		CompanyID:              req.Subject.CompanyID,
		ActorUserID:            req.Subject.UserID,
		ActorMembershipID:      req.Subject.MembershipID,
		Filename:               name,
		FileSizeBytes:          len(req.FileBytes),
		FileSHA256:             sha256BytesHex(req.FileBytes),
		CanonicalPayloadSHA256: canonicalHash,
		SchemaVersion:          strings.TrimSpace(schemaVersion),
		Status:                 status,
		RequiredMappingCount:   required,
		ResolvedMappingCount:   resolved,
		UnresolvedMappingCount: unresolved,
		RowVersion:             1,
		CreatedAt:              now,
		ValidatedAt:            now,
		UpdatedAt:              now,
	}
	if token != "" {
		row.ValidationTokenSHA256 = importSHA256Hex(token)
	}
	if resp != nil {
		row.ParseValid = resp.ParseValid
		row.DomainValid = resp.DomainValid
		row.ActivationReady = resp.ActivationReady
		row.CanConfirm = resp.CanConfirm
		row.MappingRequired = resp.MappingRequired
		row.ErrorCodes = redactIssueCodes(resp.Errors)
		row.MappingSummary = copyMappings(resp.RequiredMappings)
	}
	if err := store.CreateImportAttempt(ctx, row); err != nil {
		return "", err
	}
	return row.ID, nil
}

func (s *service) newImportID() string {
	if s.idg != nil {
		if id := s.idg.NewUUID(); id != "" {
			return id
		}
	}
	return fmt.Sprintf("imp-%d", time.Now().UnixNano())
}

func safeImportBasename(name string) (string, error) {
	cleaned := strings.ReplaceAll(strings.TrimSpace(name), "\\", "/")
	base := filepath.Base(cleaned)
	if base == "." || base == "" || base == "/" || strings.Contains(base, "..") {
		return "", perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid filename", nil)
	}
	runes := []rune(base)
	if len(runes) > 255 {
		base = string(runes[:255])
	}
	return base, nil
}

func redactIssueCodes(in []TemplateImportValidationIssueDTO) []TemplateImportValidationIssueDTO {
	if len(in) == 0 {
		return nil
	}
	if len(in) > 30 {
		in = in[:30]
	}
	out := make([]TemplateImportValidationIssueDTO, 0, len(in))
	for _, issue := range in {
		out = append(out, TemplateImportValidationIssueDTO{
			Code:      issue.Code,
			FieldPath: issue.FieldPath,
			Severity:  issue.Severity,
		})
	}
	return out
}

func copyMappings(in []TemplateImportRequiredMappingDTO) []TemplateImportRequiredMappingDTO {
	if len(in) == 0 {
		return nil
	}
	if len(in) > 50 {
		in = in[:50]
	}
	out := make([]TemplateImportRequiredMappingDTO, len(in))
	copy(out, in)
	return out
}

func historyItem(row TemplateImportAttempt) TemplateImportHistoryItem {
	fp := row.FileSHA256
	if len(fp) > 12 {
		fp = fp[:12]
	}
	return TemplateImportHistoryItem{
		ID:                     row.ID,
		Filename:               row.Filename,
		FileSHA256Fingerprint:  fp,
		Status:                 row.Status,
		ParseValid:             row.ParseValid,
		DomainValid:            row.DomainValid,
		ActivationReady:        row.ActivationReady,
		CanConfirm:             row.CanConfirm,
		MappingRequired:        row.MappingRequired,
		RequiredMappingCount:   row.RequiredMappingCount,
		ResolvedMappingCount:   row.ResolvedMappingCount,
		UnresolvedMappingCount: row.UnresolvedMappingCount,
		TargetTypeID:           row.TargetTypeID,
		CreatedTypeID:          row.CreatedTypeID,
		ConfirmErrorCode:       row.ConfirmErrorCode,
		CreatedAt:              row.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:              row.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func validImportAttemptStatus(status string) bool {
	switch status {
	case ImportAttemptStatusValidationFailed, ImportAttemptStatusMappingRequired, ImportAttemptStatusValidated, ImportAttemptStatusConfirmed, ImportAttemptStatusConfirmFailed, ImportAttemptStatusConfirming:
		return true
	default:
		return false
	}
}

type importHistoryCursor struct {
	CreatedAt string `json:"created_at"`
	ID        string `json:"id"`
}

func encodeImportHistoryCursor(created time.Time, id string) string {
	raw, _ := json.Marshal(importHistoryCursor{CreatedAt: created.UTC().Format(time.RFC3339Nano), ID: id})
	return base64.RawURLEncoding.EncodeToString(raw)
}

// DecodeImportHistoryCursor parses the opaque history cursor.
func DecodeImportHistoryCursor(cursor string) (time.Time, string, error) {
	if strings.TrimSpace(cursor) == "" {
		return time.Time{}, "", nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, "", perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid cursor", nil)
	}
	var parsed importHistoryCursor
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return time.Time{}, "", perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid cursor", nil)
	}
	ts, err := time.Parse(time.RFC3339Nano, parsed.CreatedAt)
	if err != nil {
		return time.Time{}, "", perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid cursor", nil)
	}
	return ts, parsed.ID, nil
}

func hashesEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
