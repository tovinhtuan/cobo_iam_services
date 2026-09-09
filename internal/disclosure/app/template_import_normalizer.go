package app

import (
	"strings"

	"github.com/cobo/cobo_iam_services/internal/disclosure/app/applicability"
)

// NormalizeTemplateImportV1 converts a raw TemplateImportDefinitionV1 into
// its canonical normalized authoring representation.
// It is pure and has zero side-effects on storage or database.
func NormalizeTemplateImportV1(raw TemplateImportDefinitionV1) *TemplateImportDefinitionV1 {
	out := raw

	// 1. Strings trimming
	out.TypeID = strings.TrimSpace(out.TypeID)
	out.Name = strings.TrimSpace(out.Name)
	out.Description = strings.TrimSpace(out.Description)
	out.Category = strings.TrimSpace(out.Category)
	out.DeadlineRule = strings.TrimSpace(out.DeadlineRule)
	out.LegalBasis = strings.TrimSpace(out.LegalBasis)
	out.Applicability = strings.TrimSpace(out.Applicability)
	out.ImplementationContent = strings.TrimSpace(out.ImplementationContent)
	out.ImplementationNotes = strings.TrimSpace(out.ImplementationNotes)
	out.SpecialCases = strings.TrimSpace(out.SpecialCases)
	out.ReportContent = strings.TrimSpace(out.ReportContent)
	out.RequiredDocs = strings.TrimSpace(out.RequiredDocs)
	out.ChannelsText = strings.TrimSpace(out.ChannelsText)
	out.Beneficiaries = strings.TrimSpace(out.Beneficiaries)
	out.ReceivingAuthorities = strings.TrimSpace(out.ReceivingAuthorities)
	out.Format = strings.TrimSpace(out.Format)
	out.LegalRisksText = strings.TrimSpace(out.LegalRisksText)
	out.GeneralInfo = strings.TrimSpace(out.GeneralInfo)

	// 2. Canonical template_category
	catNorm := strings.ToLower(strings.TrimSpace(out.TemplateCategory))
	if catNorm == "periodic" || strings.Contains(catNorm, "định kỳ") || strings.Contains(catNorm, "dinh ky") {
		out.TemplateCategory = TemplateCategoryPeriodic
	} else if catNorm == "irregular" || strings.Contains(catNorm, "bất thường") || strings.Contains(catNorm, "bat thuong") {
		out.TemplateCategory = TemplateCategoryIrregular
	} else {
		out.TemplateCategory = catNorm
	}

	// 3. Category label
	if out.Category == "" {
		if out.TemplateCategory == TemplateCategoryPeriodic {
			out.Category = "Định kỳ"
		} else if out.TemplateCategory == TemplateCategoryIrregular {
			out.Category = "Bất thường"
		}
	}

	// 4. Target GroupID derivation
	out.GroupID = strings.TrimSpace(out.GroupID)
	if out.GroupID == "" {
		if out.TemplateCategory == TemplateCategoryPeriodic {
			out.GroupID = DefaultPeriodicGroupID
		} else if out.TemplateCategory == TemplateCategoryIrregular {
			out.GroupID = DefaultIrregularGroupID
		}
	}

	// 5. Target DisplayGroupCodes derivation (reusing existing normalizeDisplayGroupCodes)
	out.DisplayGroupCodes = normalizeDisplayGroupCodes(out.DisplayGroupCodes)
	if len(out.DisplayGroupCodes) == 0 {
		if out.TemplateCategory == TemplateCategoryPeriodic {
			out.DisplayGroupCodes = []string{DefaultPeriodicDisplayGroupCode}
		} else if out.TemplateCategory == TemplateCategoryIrregular {
			out.DisplayGroupCodes = []string{DefaultIrregularDisplayGroupCode}
		}
	}

	// 6. DeadlineStrategy
	out.DeadlineStrategy = strings.ToLower(strings.TrimSpace(out.DeadlineStrategy))
	if out.DeadlineStrategy == "" {
		if out.TemplateCategory == TemplateCategoryPeriodic {
			out.DeadlineStrategy = DeadlineStrategyFixedCycleDays
		} else if out.TemplateCategory == TemplateCategoryIrregular {
			out.DeadlineStrategy = DeadlineStrategyEventHours
		}
	}

	// 7. Periodicity
	out.Periodicity = strings.ToLower(strings.TrimSpace(out.Periodicity))

	// 8. DeadlineConfig normalization
	if out.TemplateCategory == TemplateCategoryPeriodic {
		if out.DeadlineConfig == nil {
			out.DeadlineConfig = &TemplateImportDeadlineConfigV1{}
		}
		normalizePeriodicDeadlineConfig(out.DeadlineConfig, out.Periodicity)
	} else if out.TemplateCategory == TemplateCategoryIrregular {
		if out.DeadlineConfig != nil {
			normalizeIrregularDeadlineConfig(out.DeadlineConfig)
		}
	}

	// 9. Workflow normalization
	if out.Workflow != nil {
		normalizeWorkflow(out.Workflow)
	}

	// 10. Tags deduplication & trimming
	out.Tags = cleanStringSlice(out.Tags)

	// 11. Legal bases trimming — strip optional file-local ids (Confirm regenerates UUIDs)
	if len(out.LegalBases) > 0 {
		lbs := make([]LegalBasisDTO, 0, len(out.LegalBases))
		for _, lb := range out.LegalBases {
			lb.ID = ""
			lb.Title = strings.TrimSpace(lb.Title)
			lb.Code = strings.TrimSpace(lb.Code)
			lb.Authority = strings.TrimSpace(lb.Authority)
			lb.IssueDate = strings.TrimSpace(lb.IssueDate)
			lb.Summary = strings.TrimSpace(lb.Summary)
			lb.Link = strings.TrimSpace(lb.Link)
			if lb.Title != "" {
				lbs = append(lbs, lb)
			}
		}
		out.LegalBases = lbs
	}

	// 12. Checklist trimming — strip optional file-local ids (Confirm regenerates UUIDs)
	if len(out.Checklist) > 0 {
		chks := make([]ChecklistItemDTO, 0, len(out.Checklist))
		for _, chk := range out.Checklist {
			chk.ID = ""
			chk.Title = strings.TrimSpace(chk.Title)
			chk.Owner = strings.TrimSpace(chk.Owner)
			chk.DueDate = strings.TrimSpace(chk.DueDate)
			chk.Status = strings.TrimSpace(chk.Status)
			if chk.Title != "" {
				chks = append(chks, chk)
			}
		}
		out.Checklist = chks
	}

	// 13. ApplicabilityRules normalization
	if out.ApplicabilityRules != nil {
		normRules := &applicability.TemplateApplicabilityRules{
			ApplicableCompanyClasses: append([]applicability.CompanyClass(nil), out.ApplicabilityRules.ApplicableCompanyClasses...),
			ApplicableSectors:        append([]applicability.BusinessSector(nil), out.ApplicabilityRules.ApplicableSectors...),
			DeadlineDays:            out.ApplicabilityRules.DeadlineDays,
			DeadlineDayType:         strings.ToLower(strings.TrimSpace(out.ApplicabilityRules.DeadlineDayType)),
			UseStructureDeadline:    out.ApplicabilityRules.UseStructureDeadline,
		}
		if normRules.DeadlineDayType == "" {
			normRules.DeadlineDayType = "calendar"
		}
		if out.ApplicabilityRules.DeadlineByStructure != nil {
			normRules.DeadlineByStructure = make(map[applicability.StructureCriterion]applicability.StructureDeadlineEntry, len(out.ApplicabilityRules.DeadlineByStructure))
			for k, v := range out.ApplicabilityRules.DeadlineByStructure {
				normRules.DeadlineByStructure[k] = v
			}
		}
		out.ApplicabilityRules = normRules
	}

	return &out
}

func normalizePeriodicDeadlineConfig(cfg *TemplateImportDeadlineConfigV1, periodicity string) {
	if cfg == nil {
		return
	}

	// FrequencyUnit
	cfg.FrequencyUnit = strings.ToUpper(strings.TrimSpace(cfg.FrequencyUnit))
	if cfg.FrequencyUnit == "" && periodicity != "" {
		cfg.FrequencyUnit = strings.ToUpper(periodicity)
	}

	// ApplicableFrom authoring intent
	mode := strings.ToUpper(strings.TrimSpace(cfg.ApplicableFromMode))
	switch mode {
	case "CURRENT", "CURRENT_SLOT":
		cfg.ApplicableFromMode = ApplicableFromModeCurrent
		// Critical: clear frozen stale historical slot to prevent freezing
		cfg.ApplicableFromSlot = ""
	case "NEXT", "NEXT_SLOT":
		cfg.ApplicableFromMode = ApplicableFromModeNext
		// Critical: clear frozen stale historical slot to prevent freezing
		cfg.ApplicableFromSlot = ""
	case "SPECIFIC", "SPECIFIC_SLOT":
		cfg.ApplicableFromMode = ApplicableFromModeSpecific
		cfg.ApplicableFromSlot = strings.TrimSpace(cfg.ApplicableFromSlot)
	case "":
		cfg.ApplicableFromMode = ApplicableFromModeNext
		cfg.ApplicableFromSlot = ""
	default:
		cfg.ApplicableFromMode = mode
		cfg.ApplicableFromSlot = strings.TrimSpace(cfg.ApplicableFromSlot)
	}

	// ApplicableTo
	cfg.ApplicableTo = strings.TrimSpace(cfg.ApplicableTo)

	// Duration types
	dur := strings.ToUpper(strings.TrimSpace(cfg.DurationType))
	if dur == "WORKING_DAYS" {
		cfg.DurationType = DurationTypeWorkingDays
	} else {
		cfg.DurationType = DurationTypeCalendarDays
	}

	deadDur := strings.ToUpper(strings.TrimSpace(cfg.DeadlineDurationType))
	if deadDur == "WORKING_DAYS" {
		cfg.DeadlineDurationType = DurationTypeWorkingDays
	} else {
		cfg.DeadlineDurationType = DurationTypeCalendarDays
	}

	// Weekday
	cfg.CycleAnchorWeekday = strings.ToLower(strings.TrimSpace(cfg.CycleAnchorWeekday))
}

func normalizeIrregularDeadlineConfig(cfg *TemplateImportDeadlineConfigV1) {
	if cfg == nil {
		return
	}
	cfg.FrequencyUnit = ""
	cfg.CycleAnchorDay = nil
	cfg.CycleAnchorWeekday = ""
	cfg.MonthInQuarter = nil
	cfg.ApplicableFromMode = ""
	cfg.ApplicableFromSlot = ""
	cfg.ApplicableTo = strings.TrimSpace(cfg.ApplicableTo)
}

func normalizeWorkflow(wf *TemplateImportWorkflowV1) {
	if wf == nil {
		return
	}

	steps := make([]TemplateImportWorkflowStepV1, 0, len(wf.Steps))
	for i, step := range wf.Steps {
		s := step
		// Non-portable source step_id stripped. Defer final UUID to Confirm materialization.
		s.StepID = ""
		s.Stage = strings.TrimSpace(s.Stage)
		s.Description = strings.TrimSpace(s.Description)
		s.Instructions = strings.TrimSpace(s.Instructions)
		s.DepartmentID = strings.TrimSpace(s.DepartmentID)
		s.DepartmentName = strings.TrimSpace(s.DepartmentName)
		s.DueRule = strings.TrimSpace(s.DueRule)
		if s.DisplayOrder <= 0 {
			s.DisplayOrder = i + 1
		}

		// Preferred department {code,name} flattens into legacy fields for Validate/Confirm.
		if s.Department != nil {
			code := strings.TrimSpace(s.Department.Code)
			name := strings.TrimSpace(s.Department.Name)
			if s.DepartmentID == "" {
				s.DepartmentID = code
			}
			if s.DepartmentName == "" {
				s.DepartmentName = name
			}
			// Keep flattened portable refs; clear nested object to avoid dual authority.
			s.Department = nil
		}

		// Preferred assignee_roles merges into assignee_role_ids (static codes, not DB UUIDs).
		roles := append([]string(nil), s.AssigneeRoles...)
		roles = append(roles, s.AssigneeRoleIDs...)
		s.AssigneeRoleIDs = normalizePortableRoleCodes(cleanStringSlice(roles))
		s.AssigneeRoles = nil

		// Reminder template_key is non-portable / not persisted on Confirm — clear.
		if s.ReminderConfig != nil {
			s.ReminderConfig.TemplateKey = ""
		}

		// Documents
		if len(s.Documents) > 0 {
			docs := make([]TemplateImportWorkflowDocumentV1, 0, len(s.Documents))
			for _, d := range s.Documents {
				doc := d
				doc.Name = strings.TrimSpace(doc.Name)
				doc.TemplateFileName = strings.TrimSpace(doc.TemplateFileName)
				// ACCEPT_AND_CLEAR: template_file_id is cleared to empty string
				doc.TemplateFileID = ""
				if doc.Name != "" {
					docs = append(docs, doc)
				}
			}
			s.Documents = docs
		}

		steps = append(steps, s)
	}
	wf.Steps = steps
}

// normalizePortableRoleCodes maps human-facing aliases onto canonical static role codes.
func normalizePortableRoleCodes(roles []string) []string {
	if len(roles) == 0 {
		return roles
	}
	out := make([]string, 0, len(roles))
	seen := make(map[string]struct{}, len(roles))
	for _, raw := range roles {
		role := strings.ToLower(strings.TrimSpace(raw))
		switch role {
		case "maker", "author", "drafter":
			role = "creator"
		case "reviewer", "role-reviewer", "disclosure_reviewer":
			role = "reviewer"
		case "approver", "role-approver":
			role = "approver"
		case "legal_reviewer", "role-legal-reviewer":
			role = "legal_reviewer"
		case "finance_reviewer", "role-finance-reviewer":
			role = "finance_reviewer"
		}
		if role == "" {
			continue
		}
		if _, ok := seen[role]; ok {
			continue
		}
		seen[role] = struct{}{}
		out = append(out, role)
	}
	return out
}

func cleanStringSlice(slice []string) []string {
	if len(slice) == 0 {
		return nil
	}
	out := make([]string, 0, len(slice))
	seen := make(map[string]struct{}, len(slice))
	for _, item := range slice {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; !exists {
			seen[trimmed] = struct{}{}
			out = append(out, trimmed)
		}
	}
	return out
}
