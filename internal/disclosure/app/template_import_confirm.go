package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/cobo/cobo_iam_services/internal/disclosure/app/applicability"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

const (
	creationModeTemplateImport = "TEMPLATE_IMPORT"
)

// ConfirmTemplateImport executes the transactional materialization of an imported template
// into a new root and Draft v1.
// It verifies the validation token, confirms payload hash binding, revalidates references,
// maps departments, and executes an atomic DB write via UpsertTypeVersion with CreateOnly=true.
func (s *service) ConfirmTemplateImport(ctx context.Context, req ConfirmTemplateImportRequest) (*ConfirmTemplateImportResponse, error) {
	// 1. CMS Auth Gate (requires platform admin write authority)
	if err := s.requireCMSTemplateWrite(ctx, req.Subject); err != nil {
		return nil, err
	}

	// 2. Structural input validation
	req.TargetTypeID = strings.TrimSpace(req.TargetTypeID)
	req.TargetName = strings.TrimSpace(req.TargetName)
	req.ValidationToken = strings.TrimSpace(req.ValidationToken)

	if req.TargetTypeID == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "target_type_id is required", nil)
	}
	if req.TargetName == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "target_name is required", nil)
	}
	if req.ValidationToken == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "validation_token is required", nil)
	}

	// TARGET_NAME_CONFIRM_AUTHORITY = MUST_EQUAL_NORMALIZED_NAME
	// Confirm cannot change target_name independently of the token-bound template name.
	normName := strings.TrimSpace(req.NormalizedTemplate.Name)
	if normName != "" && req.TargetName != normName {
		return nil, &perr.HTTPError{
			HTTPStatus: http.StatusBadRequest,
			Code:       perr.CodeInvalidRequest,
			Message:    "target_name must equal normalized_template name",
			Details: map[string]any{
				"target_name":              req.TargetName,
				"normalized_template_name": normName,
			},
		}
	}

	// 3. Validation Token Verification (Actor, Expiry, Purpose, SchemaVersion, Signature)
	signer := NewTemplateImportSigner("", 0)
	claims, err := signer.VerifyToken(req.ValidationToken, req.Subject.UserID, time.Now())
	if err != nil {
		return nil, perr.NewHTTPError(http.StatusUnprocessableEntity, perr.CodeInvalidImportToken, fmt.Sprintf("invalid validation token: %v", err), nil)
	}

	// 4. Canonical Payload Hash Verification
	// Confirm that the submitted normalized_template matches the hash embedded in the token.
	actualHash, err := ComputeCanonicalTemplatePayloadHash(&req.NormalizedTemplate)
	if err != nil {
		return nil, perr.NewHTTPError(http.StatusUnprocessableEntity, perr.CodeInvalidImportToken, fmt.Sprintf("failed to hash normalized payload: %v", err), nil)
	}
	if actualHash != claims.PayloadHash {
		return nil, &perr.HTTPError{
			HTTPStatus: http.StatusUnprocessableEntity,
			Code:       perr.CodeInvalidImportToken,
			Message:    "normalized_template payload hash does not match validation token",
			Details: map[string]any{
				"expected_payload_hash": claims.PayloadHash,
				"actual_payload_hash":   actualHash,
			},
		}
	}

	// 5. Mutable Target References Revalidation
	// Target catalogs may change during the token TTL, so we must re-query current state.
	// a) Department catalog
	depts, err := s.repo.ListTemplateDepartments(ctx)
	if err != nil {
		return nil, err
	}
	targetDeptByCode := make(map[string]string)
	targetDeptByName := make(map[string]string)
	for _, d := range depts {
		c := strings.ToLower(strings.TrimSpace(d.DepartmentCode))
		n := strings.ToLower(strings.TrimSpace(d.DepartmentName))
		if c != "" {
			targetDeptByCode[c] = d.DepartmentCode
		}
		if n != "" {
			targetDeptByName[n] = d.DepartmentCode
		}
	}

	// b) Identify source departments used in normalized workflow
	sourceDeptsUsed := make(map[string]string)
	if req.NormalizedTemplate.Workflow != nil {
		for _, st := range req.NormalizedTemplate.Workflow.Steps {
			sid := strings.TrimSpace(st.DepartmentID)
			sname := strings.TrimSpace(st.DepartmentName)
			if sid != "" || sname != "" {
				sourceDeptsUsed[sid] = sname
			}
		}
	}

	// c) Validate DepartmentMappings input (M5, M6, M7, M8, M10)
	for srcID, tgtCode := range req.DepartmentMappings {
		srcID = strings.TrimSpace(srcID)
		tgtCode = strings.TrimSpace(tgtCode)
		if _, exists := sourceDeptsUsed[srcID]; !exists {
			return nil, &perr.HTTPError{
				HTTPStatus: http.StatusBadRequest,
				Code:       perr.CodeInvalidRequest,
				Message:    fmt.Sprintf("unrecognized source department mapping key: %q", srcID),
				Details: map[string]any{
					"source_department_id": srcID,
				},
			}
		}
		if tgtCode == "" {
			return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, fmt.Sprintf("mapping target department code for %q cannot be blank", srcID), nil)
		}
		if _, ok := targetDeptByCode[strings.ToLower(tgtCode)]; !ok {
			return nil, &perr.HTTPError{
				HTTPStatus: http.StatusBadRequest,
				Code:       perr.CodeInvalidRequest,
				Message:    fmt.Sprintf("mapping target department %q does not exist in target catalog", tgtCode),
				Details: map[string]any{
					"target_department_code": tgtCode,
				},
			}
		}
	}

	// d) Resolve each workflow step department (M1, M2, M3, M4)
	resolvedSteps := make([]WorkflowStepDTO, 0)
	if req.NormalizedTemplate.Workflow != nil {
		for i, st := range req.NormalizedTemplate.Workflow.Steps {
			stepDeptID := strings.TrimSpace(st.DepartmentID)
			stepDeptName := strings.TrimSpace(st.DepartmentName)
			var finalDeptCode string

			// Precedence: 1. Explicit mapping -> 2. Code match -> 3. Name match
			if mappedTgt, ok := req.DepartmentMappings[stepDeptID]; ok && strings.TrimSpace(mappedTgt) != "" {
				finalDeptCode = targetDeptByCode[strings.ToLower(strings.TrimSpace(mappedTgt))]
			} else if canon, ok := targetDeptByCode[strings.ToLower(stepDeptID)]; ok {
				finalDeptCode = canon
			} else if canon, ok := targetDeptByName[strings.ToLower(stepDeptName)]; ok {
				finalDeptCode = canon
			}

			if finalDeptCode == "" {
				return nil, &perr.HTTPError{
					HTTPStatus: http.StatusBadRequest,
					Code:       perr.CodeInvalidRequest,
					Message:    fmt.Sprintf("required department mapping is missing for step %d (%q)", i+1, st.Stage),
					Details: map[string]any{
						"step_index":             i + 1,
						"source_department_id":   stepDeptID,
						"source_department_name": stepDeptName,
					},
				}
			}

			// Generate fresh server-owned UUID for workflow step (CONFIRM_MATERIALIZATION)
			stepUUID := ""
			if s.idg != nil {
				stepUUID = s.idg.NewUUID()
			}
			if stepUUID == "" {
				stepUUID = idgen.UUIDv7Generator{}.NewUUID()
			}

			// Map document requirements with fresh UUIDs and empty template_file_id
			docs := make([]WorkflowDocumentDTO, 0, len(st.Documents))
			for _, d := range st.Documents {
				docUUID := ""
				if s.idg != nil {
					docUUID = s.idg.NewUUID()
				}
				if docUUID == "" {
					docUUID = idgen.UUIDv7Generator{}.NewUUID()
				}
				docs = append(docs, WorkflowDocumentDTO{
					DocID:            docUUID,
					Name:             d.Name,
					Required:         d.Required,
					TemplateFileID:   "", // Strictly empty on import (DOCUMENT_BINARY_WRITE_COUNT=0)
					TemplateFileName: d.TemplateFileName,
				})
			}

			var reminderDTO *WorkflowStepReminderConfig
			if st.ReminderConfig != nil {
				reminderDTO = &WorkflowStepReminderConfig{
					Enabled:    st.ReminderConfig.Enabled,
					Mode:       "days_before",
					DaysBefore: append([]int(nil), st.ReminderConfig.OffsetsDays...),
				}
			}

			displayOrder := st.DisplayOrder
			if displayOrder <= 0 {
				displayOrder = i + 1
			}

			resolvedSteps = append(resolvedSteps, WorkflowStepDTO{
				StepID:                stepUUID,
				Stage:                 st.Stage,
				Description:           st.Description,
				Instructions:          st.Instructions,
				DepartmentID:          finalDeptCode,
				AssigneeRoleIds:       append([]string(nil), st.AssigneeRoleIDs...),
				AssigneeMembershipID:  "",
				AssigneeMembershipIDs: nil,
				DueRule:               st.DueRule,
				ProcessingDays:        st.ProcessingDays,
				DisplayOrder:          displayOrder,
				Documents:             docs,
				ReminderConfig:        reminderDTO,
			})
		}
	}

	// e) Display groups revalidation
	dgs, err := s.repo.ListDisplayGroups(ctx)
	if err != nil {
		return nil, err
	}
	validDGs := make(map[string]struct{})
	for _, dg := range dgs {
		validDGs[strings.TrimSpace(dg.DisplayGroupCode)] = struct{}{}
	}
	for _, code := range req.NormalizedTemplate.DisplayGroupCodes {
		if _, ok := validDGs[strings.TrimSpace(code)]; !ok {
			return nil, &perr.HTTPError{
				HTTPStatus: http.StatusBadRequest,
				Code:       perr.CodeInvalidRequest,
				Message:    fmt.Sprintf("display group %q does not exist in target catalog", code),
				Details: map[string]any{
					"display_group_code": code,
				},
			}
		}
	}

	// 6. Target Type ID Precheck
	exists, err := s.repo.TypeExists(ctx, req.TargetTypeID)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, &perr.HTTPError{
			Code:       perr.CodeStateConflict,
			Message:    "target_type_id already exists",
			HTTPStatus: http.StatusConflict,
			Details:    map[string]any{"target_type_id": req.TargetTypeID},
		}
	}

	// 7. Materialize UpsertTypeVersionRequest
	upsert, err := s.materializeImportUpsert(ctx, req, resolvedSteps)
	if err != nil {
		return nil, err
	}

	// 8. Execute Authoritative Persistence Call (CreateOnly=true)
	resp, err := s.UpsertTypeVersion(ctx, upsert)
	if err != nil {
		if isDuplicateKeyConflictError(err) {
			return nil, &perr.HTTPError{
				Code:       perr.CodeStateConflict,
				Message:    "target_type_id already exists",
				HTTPStatus: http.StatusConflict,
				Details:    map[string]any{"target_type_id": req.TargetTypeID},
			}
		}
		return nil, err
	}

	if resp.VersionNo != 1 || resp.IsActive {
		return nil, &perr.HTTPError{
			Code:       perr.CodeStateConflict,
			Message:    "imported template must be draft v1 and not active",
			HTTPStatus: http.StatusConflict,
			Details: map[string]any{
				"type_id":     resp.TypeID,
				"version_no":  resp.VersionNo,
				"is_active":   resp.IsActive,
				"expectation": "draft_v1",
			},
		}
	}

	return &ConfirmTemplateImportResponse{
		TypeID:      resp.TypeID,
		VersionNo:   resp.VersionNo,
		IsActive:    false,
		IsReleased:  false,
		PortalState: "not_active",
		RootStatus:  "active",
		Name:        req.TargetName,
		CreatedAt:   resp.ActivatedAt,
	}, nil
}

// materializeImportUpsert maps a validated ConfirmTemplateImportRequest into
// an authoritative UpsertTypeVersionRequest with CreateOnly=true.
func (s *service) materializeImportUpsert(
	ctx context.Context,
	req ConfirmTemplateImportRequest,
	resolvedSteps []WorkflowStepDTO,
) (UpsertTypeVersionRequest, error) {
	norm := req.NormalizedTemplate
	isPeriodic := strings.EqualFold(norm.TemplateCategory, TemplateCategoryPeriodic)

	// Build workflow step projection for enterprise_workflow block
	rawSteps, err := json.Marshal(resolvedSteps)
	if err != nil {
		return UpsertTypeVersionRequest{}, fmt.Errorf("marshal workflow steps: %w", err)
	}
	var projectedSteps []any
	if err := json.Unmarshal(rawSteps, &projectedSteps); err != nil {
		return UpsertTypeVersionRequest{}, fmt.Errorf("unmarshal workflow steps: %w", err)
	}

	newUUID := func() string {
		if s.idg != nil {
			if id := s.idg.NewUUID(); id != "" {
				return id
			}
		}
		return idgen.UUIDv7Generator{}.NewUUID()
	}

	// Canonical mandatory blocks structure for global CMS template
	blocks := []TemplateBlockDTO{
		{
			BlockID:      newUUID(),
			BlockKey:     "legal_basis",
			BlockType:    "rich_text",
			Title:        "Cơ sở pháp lý",
			Description:  norm.LegalBasis,
			Config:       map[string]any{"max_length": 8000, "allow_html": false},
			Validation:   map[string]any{},
			DisplayOrder: 1,
			Enabled:      true,
		},
		{
			BlockID:      newUUID(),
			BlockKey:     "disclosure_content",
			BlockType:    "rich_text",
			Title:        "Nội dung công bố/báo cáo",
			Description:  norm.ReportContent,
			Config:       map[string]any{"max_length": 50000, "allow_html": true},
			Validation:   map[string]any{},
			DisplayOrder: 2,
			Enabled:      true,
		},
		{
			BlockID:      newUUID(),
			BlockKey:     "deadline",
			BlockType:    "text",
			Title:        "Kỳ hạn công bố/báo cáo",
			Description:  norm.DeadlineRule,
			Config:       map[string]any{"max_length": 4000},
			Validation:   map[string]any{},
			DisplayOrder: 3,
			Enabled:      true,
		},
		{
			BlockID:      newUUID(),
			BlockKey:     "channels_and_format",
			BlockType:    "rich_text",
			Title:        "Kênh và hình thức công bố/báo cáo",
			Description:  norm.ChannelsText,
			Config: map[string]any{
				"max_length": 12000,
				"allow_html": false,
				"channels": []any{
					map[string]any{
						"id": "ch-default",
						"name": func() string {
							if strings.TrimSpace(norm.ChannelsText) != "" {
								return norm.ChannelsText
							}
							return "Hệ thống CBTT UBCKNN / SGDCK"
						}(),
						"disclosure_method":      "ELECTRONIC",
						"file_types":             []any{"PDF"},
						"attachment_requirement": "REQUIRED",
					},
				},
				"file_types": func() []any {
					if norm.Format != "" {
						return []any{norm.Format}
					}
					return []any{"PDF"}
				}(),
			},
			Validation:   map[string]any{},
			DisplayOrder: 4,
			Enabled:      true,
		},
		{
			BlockID:      newUUID(),
			BlockKey:     "legal_risks",
			BlockType:    "rich_text",
			Title:        "Rủi ro pháp lý nếu không thực hiện đúng",
			Description:  norm.LegalRisksText,
			Config:       map[string]any{"max_length": 8000, "allow_html": false},
			Validation:   map[string]any{},
			DisplayOrder: 5,
			Enabled:      true,
		},
		{
			BlockID:      newUUID(),
			BlockKey:     "enterprise_workflow",
			BlockType:    "rich_text",
			Title:        "Workflow của doanh nghiệp",
			Description:  norm.ImplementationContent,
			Config: map[string]any{
				"max_length": 12000,
				"allow_html": true,
				"steps":      projectedSteps,
			},
			Validation:   map[string]any{},
			DisplayOrder: 6,
			Enabled:      true,
		},
	}

	// Legal bases
	legalBases, legalFlat, _ := PrepareLegalBasesForNewVersion(
		ctx, req.TargetTypeID, nil, "", norm.LegalBases, norm.LegalBasis, false, s.idg,
	)
	if len(norm.LegalBases) == 0 {
		legalFlat = norm.LegalBasis
	}
	blocks[0].Description = legalFlat

	// Checklist & Tags & Display Groups
	checklist := make([]ChecklistItemDTO, len(norm.Checklist))
	copy(checklist, norm.Checklist)
	tags := append([]string(nil), norm.Tags...)
	displayGroups := append([]string(nil), norm.DisplayGroupCodes...)

	// DeadlineConfig mapping
	var deadlineCfg *TemplateDeadlineConfig
	if norm.DeadlineConfig != nil {
		srcCfg := norm.DeadlineConfig
		deadlineCfg = &TemplateDeadlineConfig{
			TemplateCategory:     norm.TemplateCategory,
			FrequencyUnit:        srcCfg.FrequencyUnit,
			ApplicableFromMode:   srcCfg.ApplicableFromMode,
			ApplicableFromSlot:   srcCfg.ApplicableFromSlot,
			ApplicableTo:         srcCfg.ApplicableTo,
			ApplicableToProvided: srcCfg.ApplicableTo != "",
			DeadlineDurationType: srcCfg.DeadlineDurationType,
		}
		if srcCfg.CycleAnchorDay != nil {
			deadlineCfg.CycleAnchorDay = *srcCfg.CycleAnchorDay
		}
		if srcCfg.MonthInQuarter != nil {
			deadlineCfg.MonthInQuarter = srcCfg.MonthInQuarter
		}
		if srcCfg.CycleAnchorWeekday != "" {
			if wd, ok := parseWeekdayToInt(srcCfg.CycleAnchorWeekday); ok {
				deadlineCfg.CycleAnchorWeekday = &wd
			}
		}
		if srcCfg.DeadlineDays != nil {
			deadlineCfg.DeadlineDays = *srcCfg.DeadlineDays
		}
		if srcCfg.OpenDaysBefore != nil {
			deadlineCfg.OpenDaysBeforeT = *srcCfg.OpenDaysBefore
		}
	}

	// Applicability rules: preserve imported normalized rules if present;
	// default to canonical Draft/Create global rules only when omitted.
	rules := norm.ApplicabilityRules
	if rules == nil {
		rules = applicability.DefaultGlobalRules(isPeriodic)
	}

	upsert := UpsertTypeVersionRequest{
		Subject:               req.Subject,
		TypeID:                req.TargetTypeID,
		Scope:                 templateScopeGlobal,
		GroupID:               norm.GroupID,
		Name:                  req.TargetName,
		Category:              norm.Category,
		TemplateCategory:      norm.TemplateCategory,
		DeadlineStrategy:      norm.DeadlineStrategy,
		Description:           norm.Description,
		LegalBasis:            legalFlat,
		Applicability:         norm.Applicability,
		ImplementationContent: norm.ImplementationContent,
		ImplementationNotes:   norm.ImplementationNotes,
		SpecialCases:          norm.SpecialCases,
		ReportContent:         norm.ReportContent,
		RequiredDocs:          norm.RequiredDocs,
		DeadlineRule:          norm.DeadlineRule,
		Periodicity:           norm.Periodicity,
		ChannelsText:          norm.ChannelsText,
		Beneficiaries:         norm.Beneficiaries,
		ReceivingAuthorities:  norm.ReceivingAuthorities,
		Format:                norm.Format,
		LegalRisksText:        norm.LegalRisksText,
		GeneralInfo:           norm.GeneralInfo,
		LegalBases:            legalBases,
		LegalBasesProvided:    true,
		Checklist:             checklist,
		Tags:                  tags,
		DeadlineConfig:        deadlineCfg,
		Blocks:                blocks,
		DisplayGroupCodes:     displayGroups,
		ChangeNote:            "Imported template definition v1.0",
		ApplicabilityRules:    rules,
		CreateOnly:            true,
		ClearWorkflow:         len(resolvedSteps) == 0,
		PreserveLegalBases:    false,
		SkipPublicationMatrix: false,
	}

	return upsert, nil
}

// isDuplicateKeyConflictError inspects an error to determine if it stems from
// a primary key or unique index conflict (e.g. MySQL Error 1062).
func isDuplicateKeyConflictError(err error) bool {
	if err == nil {
		return false
	}
	var he *perr.HTTPError
	if errors.As(err, &he) {
		if he.HTTPStatus == http.StatusConflict || he.Code == perr.CodeStateConflict {
			return true
		}
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate entry") ||
		strings.Contains(msg, "1062") ||
		strings.Contains(msg, "already exists") ||
		strings.Contains(msg, "unique constraint")
}

// parseWeekdayToInt parses string weekdays (including Vietnamese & English) into Go time.Weekday int (0=Sunday..6=Saturday).
func parseWeekdayToInt(raw string) (int, bool) {
	s := strings.ToLower(strings.TrimSpace(raw))
	switch s {
	case "sunday", "chủ nhật", "chu nhat", "sun", "0":
		return 0, true
	case "monday", "thứ hai", "thu hai", "mon", "1":
		return 1, true
	case "tuesday", "thứ ba", "thu ba", "tue", "2":
		return 2, true
	case "wednesday", "thứ tư", "thu tu", "wed", "3":
		return 3, true
	case "thursday", "thứ năm", "thu nam", "thu", "4":
		return 4, true
	case "friday", "thứ sáu", "thu sau", "fri", "5":
		return 5, true
	case "saturday", "thứ bảy", "thu bay", "sat", "6":
		return 6, true
	default:
		return 0, false
	}
}
