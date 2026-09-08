package app

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/cobo/cobo_iam_services/internal/disclosure/app/applicability"
	workflowconfigapp "github.com/cobo/cobo_iam_services/internal/workflowconfig/app"
)

// ValidateImportTemplate performs comprehensive Draft-level domain validation,
// target catalog reference verification, required mappings deduction,
// and activation-readiness evaluation.
func ValidateImportTemplate(
	template *TemplateImportDefinitionV1,
	evalAt time.Time,
	validDisplayGroups map[string]struct{},
	departmentsByCode map[string]TemplateDepartmentDTO,
	departmentsByName map[string]TemplateDepartmentDTO,
) (
	errors []TemplateImportValidationIssueDTO,
	warnings []TemplateImportValidationIssueDTO,
	requiredMappings []TemplateImportRequiredMappingDTO,
	activationBlockers []ActivationBlockerDTO,
) {
	if template == nil {
		errors = append(errors, TemplateImportValidationIssueDTO{
			Code:      "TEMPLATE_DEFINITION_REQUIRED",
			FieldPath: "template",
			Severity:  ValidationSeverityBlocker,
			Message:   "Thông tin mẫu template không được để trống.",
		})
		return
	}

	roleReg := workflowconfigapp.DefaultRoleRegistry()

	// 1. Mandatory base fields
	if template.Name == "" {
		errors = append(errors, TemplateImportValidationIssueDTO{
			Code:      "TEMPLATE_NAME_REQUIRED",
			FieldPath: "template.name",
			Severity:  ValidationSeverityBlocker,
			Message:   "Tên mẫu template là bắt buộc.",
			SuggestedAction: "Nhập tên mẫu template (1-255 ký tự).",
		})
	} else if len([]rune(template.Name)) > 255 {
		errors = append(errors, TemplateImportValidationIssueDTO{
			Code:      "TEMPLATE_NAME_TOO_LONG",
			FieldPath: "template.name",
			Severity:  ValidationSeverityBlocker,
			Message:   "Tên mẫu template vượt quá 255 ký tự.",
		})
	}

	if len(template.Description) > MaxTemplateDescriptionLength {
		errors = append(errors, TemplateImportValidationIssueDTO{
			Code:      "TEMPLATE_DESCRIPTION_TOO_LONG",
			FieldPath: "template.description",
			Severity:  ValidationSeverityBlocker,
			Message:   fmt.Sprintf("Mô tả mẫu template vượt quá giới hạn %d bytes.", MaxTemplateDescriptionLength),
		})
	}

	if template.TemplateCategory != TemplateCategoryPeriodic && template.TemplateCategory != TemplateCategoryIrregular {
		errors = append(errors, TemplateImportValidationIssueDTO{
			Code:      "INVALID_TEMPLATE_CATEGORY",
			FieldPath: "template.template_category",
			Severity:  ValidationSeverityBlocker,
			Message:   "Loại template không hợp lệ (chỉ chấp nhận 'periodic' hoặc 'irregular').",
			SuggestedAction: "Chọn 'periodic' cho định kỳ hoặc 'irregular' cho bất thường.",
		})
	}

	if template.DeadlineRule == "" {
		errors = append(errors, TemplateImportValidationIssueDTO{
			Code:      "DEADLINE_RULE_REQUIRED",
			FieldPath: "template.deadline_rule",
			Severity:  ValidationSeverityBlocker,
			Message:   "Quy tắc thời hạn (deadline_rule) là bắt buộc.",
			SuggestedAction: "Nhập quy tắc thời hạn (ví dụ: 'T+5' hoặc 'Trong vòng 20 ngày kể từ khi kết thúc quý.').",
		})
	}

	// 2. Periodicity & Deadline configuration
	if template.TemplateCategory == TemplateCategoryPeriodic {
		validatePeriodicConfiguration(template, &errors)
	} else if template.TemplateCategory == TemplateCategoryIrregular {
		validateIrregularConfiguration(template, &errors)
	}

	// 3. ApplicableTo evaluation
	if template.DeadlineConfig != nil && template.DeadlineConfig.ApplicableTo != "" {
		parsedTo, err := time.Parse("2006-01-02", template.DeadlineConfig.ApplicableTo)
		if err != nil {
			errors = append(errors, TemplateImportValidationIssueDTO{
				Code:      "INVALID_APPLICABLE_TO_FORMAT",
				FieldPath: "template.deadline_config.applicable_to",
				Severity:  ValidationSeverityBlocker,
				Message:   "Định dạng ngày kết thúc hiệu lực không hợp lệ (yêu cầu định dạng YYYY-MM-DD).",
				SuggestedAction: "Cung cấp ngày theo định dạng YYYY-MM-DD.",
			})
		} else {
			// Compare against today (truncated to date)
			today := evalAt.Truncate(24 * time.Hour)
			if parsedTo.Before(today) {
				// Draft warning, NOT import blocker!
				warnings = append(warnings, TemplateImportValidationIssueDTO{
					Code:      "APPLICABLE_TO_IN_PAST",
					FieldPath: "template.deadline_config.applicable_to",
					Severity:  ValidationSeverityWarning,
					Message:   fmt.Sprintf("Ngày kết thúc hiệu lực (%s) nằm trong quá khứ.", template.DeadlineConfig.ApplicableTo),
					SuggestedAction: "Cập nhật ngày kết thúc hiệu lực trong CMS Editor trước khi kích hoạt template.",
				})
				// Recorded as activation blocker
				activationBlockers = append(activationBlockers, ActivationBlockerDTO{
					Code:    "APPLICABLE_TO_EXPIRED",
					Message: fmt.Sprintf("Ngày kết thúc hiệu lực (%s) đã qua so với ngày hiện tại.", template.DeadlineConfig.ApplicableTo),
				})
			}
		}
	}

	// 3b. ApplicabilityRules validation if present
	if template.ApplicabilityRules != nil {
		isPeriodic := template.TemplateCategory == TemplateCategoryPeriodic
		if err := applicability.ValidateRules(template.ApplicabilityRules, isPeriodic); err != nil {
			errors = append(errors, TemplateImportValidationIssueDTO{
				Code:            "INVALID_APPLICABILITY_RULES",
				FieldPath:       "template.applicability_rules",
				Severity:        ValidationSeverityBlocker,
				Message:         fmt.Sprintf("Quy tắc áp dụng (applicability_rules) không hợp lệ: %v", err),
				SuggestedAction: "Cung cấp quy tắc áp dụng hợp lệ theo chuẩn CMS.",
			})
		}
	}

	// 4. Display group codes catalog validation
	if len(template.DisplayGroupCodes) > 0 && validDisplayGroups != nil {
		for _, code := range template.DisplayGroupCodes {
			if _, ok := validDisplayGroups[code]; !ok {
				errors = append(errors, TemplateImportValidationIssueDTO{
					Code:      "UNKNOWN_DISPLAY_GROUP_CODE",
					FieldPath: "template.display_group_codes",
					Severity:  ValidationSeverityBlocker,
					Message:   "Nhóm hiển thị Portal không tồn tại trong hệ thống: " + code,
					SuggestedAction: "Chọn nhóm hiển thị hợp lệ từ danh mục Portal.",
				})
			}
		}
	}

	// 5. Workflow steps validation
	hasApprover := false
	seenMappings := make(map[string]struct{})

	if template.Workflow != nil && len(template.Workflow.Steps) > 0 {
		for i, step := range template.Workflow.Steps {
			stepPath := fmt.Sprintf("template.workflow.steps[%d]", i)
			stepLabel := step.Stage
			if stepLabel == "" {
				stepLabel = fmt.Sprintf("Bước %d", i+1)
			}

			if step.Stage == "" {
				errors = append(errors, TemplateImportValidationIssueDTO{
					Code:      "WORKFLOW_STEP_STAGE_REQUIRED",
					FieldPath: stepPath + ".stage",
					Severity:  ValidationSeverityBlocker,
					Message:   fmt.Sprintf("Bước %d: Tên giai đoạn/bước là bắt buộc.", i+1),
				})
			}

			if step.ProcessingDays < 1 {
				errors = append(errors, TemplateImportValidationIssueDTO{
					Code:      "WORKFLOW_STEP_SLA_INVALID",
					FieldPath: stepPath + ".processing_days",
					Severity:  ValidationSeverityBlocker,
					Message:   fmt.Sprintf("%s: Số ngày xử lý (processing_days) phải lớn hơn hoặc bằng 1.", stepLabel),
					SuggestedAction: "Nhập số ngày xử lý SLA tối thiểu là 1.",
				})
			}

			// Role validation
			if len(step.AssigneeRoleIDs) == 0 {
				activationBlockers = append(activationBlockers, ActivationBlockerDTO{
					Code:    "WORKFLOW_STEP_ROLE_REQUIRED",
					Message: fmt.Sprintf("%s: chưa chọn vai trò người xử lý.", stepLabel),
				})
			} else {
				for _, roleID := range step.AssigneeRoleIDs {
					normRole := strings.ToLower(strings.TrimSpace(roleID))
					if _, ok := roleReg.GetRole(normRole); !ok && !isAllowedStandardRole(normRole) {
						errors = append(errors, TemplateImportValidationIssueDTO{
							Code:      "UNKNOWN_WORKFLOW_ROLE",
							FieldPath: stepPath + ".assignee_role_ids",
							Severity:  ValidationSeverityBlocker,
							Message:   fmt.Sprintf("%s: Vai trò người xử lý không tồn tại trong hệ thống: %q.", stepLabel, roleID),
							SuggestedAction: "Chọn vai trò hợp lệ từ danh mục vai trò.",
						})
					}
					if isApprovalRole(normRole, roleReg) {
						hasApprover = true
					}
				}
			}

			// Department resolution (O(1) in-memory)
			deptID := strings.TrimSpace(step.DepartmentID)
			deptName := strings.TrimSpace(step.DepartmentName)

			if deptID == "" && deptName == "" {
				activationBlockers = append(activationBlockers, ActivationBlockerDTO{
					Code:    "WORKFLOW_STEP_DEPARTMENT_REQUIRED",
					Message: fmt.Sprintf("%s: chưa chọn phòng/ban thực hiện.", stepLabel),
				})
			} else if deptID != "" {
				if _, ok := departmentsByCode[strings.ToLower(deptID)]; ok {
					// Exact code match!
				} else if dept, ok := departmentsByName[strings.ToLower(deptName)]; ok {
					// Exact name match! Auto-mapped.
					mapKey := deptID + ":" + dept.DepartmentCode
					if _, seen := seenMappings[mapKey]; !seen {
						seenMappings[mapKey] = struct{}{}
						requiredMappings = append(requiredMappings, TemplateImportRequiredMappingDTO{
							Type:          "department",
							SourceID:      deptID,
							SourceName:    deptName,
							TargetID:      dept.DepartmentCode,
							IsAutoMatched: true,
						})
					}
				} else {
					// Unmatched department requires explicit user mapping.
					mapKey := deptID
					if _, seen := seenMappings[mapKey]; !seen {
						seenMappings[mapKey] = struct{}{}
						requiredMappings = append(requiredMappings, TemplateImportRequiredMappingDTO{
							Type:          "department",
							SourceID:      deptID,
							SourceName:    deptName,
							TargetID:      "",
							IsAutoMatched: false,
						})
						warnings = append(warnings, TemplateImportValidationIssueDTO{
							Code:      "UNRESOLVED_DEPARTMENT_MAPPING",
							FieldPath: stepPath + ".department_id",
							Severity:  ValidationSeverityWarning,
							Message:   fmt.Sprintf("%s: Phòng/ban nguồn (%s - %s) chưa được gán vào phòng/ban nào của hệ thống.", stepLabel, deptID, deptName),
							SuggestedAction: "Chọn phòng/ban tương ứng từ danh mục hệ thống trước khi xác nhận.",
						})
					}
				}
			} else { // deptID is empty but deptName is provided
				if dept, ok := departmentsByName[strings.ToLower(deptName)]; ok {
					// Matched by name
					step.DepartmentID = dept.DepartmentCode
				} else {
					mapKey := deptName
					if _, seen := seenMappings[mapKey]; !seen {
						seenMappings[mapKey] = struct{}{}
						requiredMappings = append(requiredMappings, TemplateImportRequiredMappingDTO{
							Type:          "department",
							SourceID:      "",
							SourceName:    deptName,
							TargetID:      "",
							IsAutoMatched: false,
						})
						warnings = append(warnings, TemplateImportValidationIssueDTO{
							Code:      "UNRESOLVED_DEPARTMENT_MAPPING",
							FieldPath: stepPath + ".department_name",
							Severity:  ValidationSeverityWarning,
							Message:   fmt.Sprintf("%s: Phòng/ban %q chưa có trong hệ thống.", stepLabel, deptName),
							SuggestedAction: "Gán phòng/ban hệ thống tương ứng.",
						})
					}
				}
			}

			// Documents validation
			for j, doc := range step.Documents {
				if doc.Name == "" {
					errors = append(errors, TemplateImportValidationIssueDTO{
						Code:      "WORKFLOW_DOCUMENT_NAME_REQUIRED",
						FieldPath: fmt.Sprintf("%s.documents[%d].name", stepPath, j),
						Severity:  ValidationSeverityBlocker,
						Message:   fmt.Sprintf("%s: Tài liệu thứ %d thiếu tên tài liệu.", stepLabel, j+1),
					})
				}
			}
		}

		if !hasApprover {
			activationBlockers = append(activationBlockers, ActivationBlockerDTO{
				Code:    "WORKFLOW_APPROVER_ROLE_REQUIRED",
				Message: "Workflow chưa có bước nào được gán vai trò Người phê duyệt (approver).",
			})
		}
	} else {
		activationBlockers = append(activationBlockers, ActivationBlockerDTO{
			Code:    "TEMPLATE_NO_WORKFLOW",
			Message: "Template chưa có các bước quy trình thực hiện (workflow).",
		})
	}

	// Stable deterministic sorting
	sortValidationIssues(errors)
	sortValidationIssues(warnings)

	return
}

func validatePeriodicConfiguration(template *TemplateImportDefinitionV1, errors *[]TemplateImportValidationIssueDTO) {
	validFrequencies := map[string]struct{}{
		"daily":     {},
		"weekly":    {},
		"monthly":   {},
		"quarterly": {},
		"yearly":    {},
	}

	if _, ok := validFrequencies[template.Periodicity]; !ok {
		*errors = append(*errors, TemplateImportValidationIssueDTO{
			Code:      "INVALID_PERIODICITY",
			FieldPath: "template.periodicity",
			Severity:  ValidationSeverityBlocker,
			Message:   fmt.Sprintf("Chu kỳ %q không hợp lệ cho template định kỳ (chỉ chấp nhận: daily, weekly, monthly, quarterly, yearly).", template.Periodicity),
		})
	}

	cfg := template.DeadlineConfig
	if cfg == nil {
		return
	}

	// Anchor validations
	if cfg.CycleAnchorDay != nil {
		if *cfg.CycleAnchorDay < 1 || *cfg.CycleAnchorDay > 31 {
			*errors = append(*errors, TemplateImportValidationIssueDTO{
				Code:      "INVALID_CYCLE_ANCHOR_DAY",
				FieldPath: "template.deadline_config.cycle_anchor_day",
				Severity:  ValidationSeverityBlocker,
				Message:   "Ngày neo chu kỳ (cycle_anchor_day) phải từ 1 đến 31.",
				SuggestedAction: "Nhập ngày neo chu kỳ trong khoảng 1-31.",
			})
		}
	}

	if cfg.MonthInQuarter != nil {
		if *cfg.MonthInQuarter < 1 || *cfg.MonthInQuarter > 3 {
			*errors = append(*errors, TemplateImportValidationIssueDTO{
				Code:      "INVALID_MONTH_IN_QUARTER",
				FieldPath: "template.deadline_config.month_in_quarter",
				Severity:  ValidationSeverityBlocker,
				Message:   "Tháng trong quý (month_in_quarter) phải là 1, 2 hoặc 3.",
				SuggestedAction: "Nhập tháng trong quý trong khoảng 1-3.",
			})
		}
	}

	if cfg.CycleAnchorWeekday != "" {
		validDays := map[string]struct{}{
			"monday": {}, "tuesday": {}, "wednesday": {}, "thursday": {}, "friday": {}, "saturday": {}, "sunday": {},
		}
		if _, ok := validDays[cfg.CycleAnchorWeekday]; !ok {
			*errors = append(*errors, TemplateImportValidationIssueDTO{
				Code:      "INVALID_CYCLE_ANCHOR_WEEKDAY",
				FieldPath: "template.deadline_config.cycle_anchor_weekday",
				Severity:  ValidationSeverityBlocker,
				Message:   fmt.Sprintf("Thứ neo chu kỳ %q không hợp lệ.", cfg.CycleAnchorWeekday),
				SuggestedAction: "Chọn một trong các thứ: monday, tuesday, wednesday, thursday, friday, saturday, sunday.",
			})
		}
	}

	if cfg.DeadlineDays != nil && *cfg.DeadlineDays < 0 {
		*errors = append(*errors, TemplateImportValidationIssueDTO{
			Code:      "INVALID_DEADLINE_DAYS",
			FieldPath: "template.deadline_config.deadline_days",
			Severity:  ValidationSeverityBlocker,
			Message:   "Số ngày thời hạn (deadline_days) không được là số âm.",
		})
	}

	if cfg.ApplicableFromMode != "" {
		validModes := map[string]struct{}{
			ApplicableFromModeNext:     {},
			ApplicableFromModeCurrent:  {},
			ApplicableFromModeSpecific: {},
		}
		if _, ok := validModes[cfg.ApplicableFromMode]; !ok {
			*errors = append(*errors, TemplateImportValidationIssueDTO{
				Code:      "INVALID_APPLICABLE_FROM_MODE",
				FieldPath: "template.deadline_config.applicable_from_mode",
				Severity:  ValidationSeverityBlocker,
				Message:   fmt.Sprintf("Chế độ áp dụng bắt đầu %q không hợp lệ.", cfg.ApplicableFromMode),
			})
		}
	}
}

func validateIrregularConfiguration(template *TemplateImportDefinitionV1, errors *[]TemplateImportValidationIssueDTO) {
	cfg := template.DeadlineConfig
	if cfg == nil {
		return
	}
	if cfg.FrequencyUnit != "" || cfg.CycleAnchorDay != nil || cfg.CycleAnchorWeekday != "" || cfg.MonthInQuarter != nil {
		*errors = append(*errors, TemplateImportValidationIssueDTO{
			Code:      "IRREGULAR_HAS_PERIODIC_ANCHORS",
			FieldPath: "template.deadline_config",
			Severity:  ValidationSeverityBlocker,
			Message:   "Template bất thường không được chứa cấu hình neo chu kỳ định kỳ.",
			SuggestedAction: "Xóa các trường frequency_unit, cycle_anchor_day, cycle_anchor_weekday đối với template bất thường.",
		})
	}
}

func isAllowedStandardRole(role string) bool {
	switch role {
	case "admin", "creator", "viewer", "publisher", "editor":
		return true
	default:
		return false
	}
}

func isApprovalRole(role string, reg *workflowconfigapp.RoleRegistry) bool {
	if role == "approver" || role == "role-approver" {
		return true
	}
	if d, ok := reg.GetRole(role); ok && d.IsApprovalRole {
		return true
	}
	return false
}

func sortValidationIssues(issues []TemplateImportValidationIssueDTO) {
	sort.Slice(issues, func(i, j int) bool {
		if issues[i].FieldPath != issues[j].FieldPath {
			return issues[i].FieldPath < issues[j].FieldPath
		}
		return issues[i].Code < issues[j].Code
	})
}
