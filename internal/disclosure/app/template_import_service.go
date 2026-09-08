package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// ValidateTemplateImport executes the complete read-only parsing, normalization,
// catalog lookup, domain validation, preview construction, and stateless HMAC token issuance.
// It executes ZERO DB WRITES.
func (s *service) ValidateTemplateImport(ctx context.Context, req ValidateTemplateImportRequest) (*ValidateTemplateImportResponse, error) {
	// 1. Authorization: exact CMS template write permission gate
	if err := s.requireCMSTemplateWrite(ctx, req.Subject); err != nil {
		return nil, err
	}

	// 2. File size & empty guards
	if len(req.FileBytes) == 0 {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "tập tin không được để trống", nil)
	}
	if len(req.FileBytes) > MaxTemplateImportFileSizeBytes {
		return nil, perr.NewHTTPError(http.StatusRequestEntityTooLarge, perr.CodeInvalidRequest, fmt.Sprintf("kích thước tập tin vượt quá giới hạn %d bytes (2 MiB)", MaxTemplateImportFileSizeBytes), nil)
	}

	// 3. Strict JSON decoding with DisallowUnknownFields
	dec := json.NewDecoder(bytes.NewReader(req.FileBytes))
	dec.DisallowUnknownFields()

	var env TemplateImportEnvelopeV1
	if err := dec.Decode(&env); err != nil {
		return &ValidateTemplateImportResponse{
			ParseValid:  false,
			DomainValid: false,
			CanConfirm:  false,
			Errors: []TemplateImportValidationIssueDTO{{
				Code:            "INVALID_JSON_PAYLOAD",
				FieldPath:       "file",
				Severity:        ValidationSeverityBlocker,
				Message:         fmt.Sprintf("Không thể phân tích cú pháp JSON hoặc phát hiện trường không được hỗ trợ: %v", err),
				SuggestedAction: "Kiểm tra lại cú pháp JSON và loại bỏ các trường không thuộc đặc tả schema v1.0.",
			}},
		}, nil
	}

	// Reject concatenated multiple JSON documents
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return &ValidateTemplateImportResponse{
			ParseValid:  false,
			DomainValid: false,
			CanConfirm:  false,
			Errors: []TemplateImportValidationIssueDTO{{
				Code:            "MULTIPLE_JSON_VALUES_REJECTED",
				FieldPath:       "file",
				Severity:        ValidationSeverityBlocker,
				Message:         "Tập tin chứa nhiều khối JSON nối tiếp; chỉ chấp nhận đúng một đối tượng JSON duy nhất.",
				SuggestedAction: "Đảm bảo tập tin chỉ chứa duy nhất một đối tượng JSON gốc.",
			}},
		}, nil
	}

	// 4. Schema version validation
	if strings.TrimSpace(env.SchemaVersion) != TemplateImportSchemaVersion {
		return nil, perr.NewHTTPError(
			http.StatusUnprocessableEntity,
			perr.CodeInvalidRequest,
			fmt.Sprintf("unsupported schema_version %q (hệ thống chỉ hỗ trợ phiên bản %q)", env.SchemaVersion, TemplateImportSchemaVersion),
			nil,
		)
	}

	// 5. Normalization pipeline (pure in-memory)
	normalized := NormalizeTemplateImportV1(env.Template)

	// 6. Bulk read-only catalog preloading (O(1) queries, no N+1)
	validDisplayGroups := make(map[string]struct{})
	if dgs, err := s.repo.ListDisplayGroups(ctx); err == nil {
		for _, dg := range dgs {
			validDisplayGroups[strings.TrimSpace(dg.DisplayGroupCode)] = struct{}{}
		}
	}

	departmentsByCode := make(map[string]TemplateDepartmentDTO)
	departmentsByName := make(map[string]TemplateDepartmentDTO)
	if depts, err := s.repo.ListTemplateDepartments(ctx); err == nil {
		for _, d := range depts {
			code := strings.ToLower(strings.TrimSpace(d.DepartmentCode))
			name := strings.ToLower(strings.TrimSpace(d.DepartmentName))
			if code != "" {
				departmentsByCode[code] = d
			}
			if name != "" {
				departmentsByName[name] = d
			}
		}
	}

	// 7. Domain validation & Activation readiness preview
	errors, warnings, requiredMappings, activationBlockers := ValidateImportTemplate(
		normalized,
		time.Now(),
		validDisplayGroups,
		departmentsByCode,
		departmentsByName,
	)

	// 8. Suggested type_id (read-only existence check)
	suggestedTypeID := normalized.TypeID
	if suggestedTypeID == "" {
		suggestedTypeID = slugifyTypeID(normalized.Name)
	} else {
		suggestedTypeID = slugifyTypeID(suggestedTypeID)
	}
	if exists, _ := s.repo.TypeExists(ctx, suggestedTypeID); exists {
		suggestedTypeID = fmt.Sprintf("%s-%d", suggestedTypeID, time.Now().Year())
	}

	// 9. Document count
	docCount := 0
	stepCount := 0
	if normalized.Workflow != nil {
		stepCount = len(normalized.Workflow.Steps)
		for _, st := range normalized.Workflow.Steps {
			docCount += len(st.Documents)
		}
	}

	// 10. Preview DTO
	preview := &TemplateImportPreviewDTO{
		Name:                      normalized.Name,
		TemplateCategory:          normalized.TemplateCategory,
		Periodicity:               normalized.Periodicity,
		DeadlineRule:              normalized.DeadlineRule,
		ResolvedGroupID:           normalized.GroupID,
		ResolvedDisplayGroupCodes: normalized.DisplayGroupCodes,
		WorkflowStepCount:         stepCount,
		DocumentRequirementCount:  docCount,
		NormalizedTemplate:        normalized,
	}
	if normalized.DeadlineConfig != nil {
		preview.ApplicableFromMode = normalized.DeadlineConfig.ApplicableFromMode
		preview.ApplicableTo = normalized.DeadlineConfig.ApplicableTo
	}

	// 11. State flags
	parseValid := true
	domainValid := (len(errors) == 0)
	mappingRequired := (len(requiredMappings) > 0)
	canConfirm := (domainValid && !mappingRequired)
	activationReady := (len(activationBlockers) == 0)

	// 12. Token issuance (when domain is valid)
	var validationToken string
	var tokenExpiresAt string
	if domainValid {
		payloadHash, err := ComputeCanonicalTemplatePayloadHash(normalized)
		if err == nil {
			signer := NewTemplateImportSigner("", 0)
			now := time.Now()
			exp := now.Add(signer.ttl).Unix()
			claims := TemplateImportTokenClaims{
				SchemaVersion: TemplateImportSchemaVersion,
				PayloadHash:   payloadHash,
				ActorID:       req.Subject.UserID,
				IssuedAt:      now.Unix(),
				ExpiresAt:     exp,
			}
			if tok, err := signer.IssueToken(claims); err == nil {
				validationToken = tok
				tokenExpiresAt = time.Unix(exp, 0).UTC().Format(time.RFC3339)
			}
		}
	}

	return &ValidateTemplateImportResponse{
		ParseValid:         parseValid,
		DomainValid:        domainValid,
		MappingRequired:    mappingRequired,
		CanConfirm:         canConfirm,
		ActivationReady:    activationReady,
		SuggestedTypeID:    suggestedTypeID,
		ValidationToken:    validationToken,
		TokenExpiresAt:     tokenExpiresAt,
		Errors:             errors,
		Warnings:           warnings,
		RequiredMappings:   requiredMappings,
		ActivationBlockers: activationBlockers,
		Preview:            preview,
	}, nil
}

func slugifyTypeID(name string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	ascii, _, _ := transform.String(t, strings.TrimSpace(name))
	ascii = strings.ToLower(ascii)

	var b strings.Builder
	lastDash := false
	for _, r := range ascii {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteRune('-')
			lastDash = true
		}
	}
	s := strings.Trim(b.String(), "-")
	if len(s) > 60 {
		s = strings.TrimRight(s[:60], "-")
	}
	if s == "" {
		return "template"
	}
	return s
}
