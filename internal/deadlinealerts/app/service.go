package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

func (s *service) ListDeadlineAlerts(ctx context.Context, req ListDeadlineAlertsRequest) (*ListDeadlineAlertsResponse, error) {
	if strings.TrimSpace(req.Subject.CompanyID) == "" {
		return nil, perr.NewHTTPError(http.StatusUnprocessableEntity, perr.CodeCompanyContextRequired, "company_id is required", nil)
	}
	if err := s.authorizeView(ctx, req.Subject); err != nil {
		return nil, err
	}

	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	eff, err := s.auth.GetEffectiveAccess(ctx, req.Subject.MembershipID, req.Subject.CompanyID)
	if err != nil {
		return nil, fmt.Errorf("resolve effective access: %w", err)
	}
	accessScope := ResolveDeadlineAlertAccessScope(eff)

	rows, err := s.repo.ListRows(ctx, req.Subject.CompanyID, accessScope)
	if err != nil {
		return nil, err
	}

	companyCtx, err := s.repo.GetCompanyDeadlineContext(ctx, req.Subject.CompanyID)
	if err != nil {
		return nil, err
	}

	now := s.now().UTC()
	typeConfigCache := map[string]*disclosureapp.TemplateDeadlineConfig{}

	statusFilter := normalizeStatusFilter(req.Status)
	query := strings.ToLower(strings.TrimSpace(req.Query))
	startDate := strings.TrimSpace(req.StartDate)
	endDate := strings.TrimSpace(req.EndDate)
	departmentID := strings.TrimSpace(req.DepartmentID)
	displayGroupCode := strings.TrimSpace(req.DisplayGroupCode)

	companyDepts, deptErr := s.repo.ListCompanyDepartments(ctx, req.Subject.CompanyID)
	if deptErr != nil {
		return nil, deptErr
	}
	templateDepts, tplErr := s.repo.ListTemplateDepartments(ctx)
	if tplErr != nil {
		return nil, tplErr
	}
	deptDict := NewDepartmentDict(companyDepts, templateDepts)
	departmentAliases := deptDict.FilterAliases(departmentID)

	codesByType := map[string][]string{}
	if displayGroupCode != "" {
		typeIDs := make([]string, 0, len(rows))
		seenType := map[string]struct{}{}
		for _, row := range rows {
			tid := strings.TrimSpace(row.TypeID)
			if tid == "" {
				continue
			}
			if _, ok := seenType[tid]; ok {
				continue
			}
			seenType[tid] = struct{}{}
			typeIDs = append(typeIDs, tid)
		}
		var codesErr error
		codesByType, codesErr = s.repo.ListDisplayGroupCodesByTypeIDs(ctx, typeIDs)
		if codesErr != nil {
			return nil, codesErr
		}
	}

	enriched := make([]DeadlineAlertDTO, 0)
	for _, row := range rows {
		if isDraftRecordStatus(row.RecordStatus) {
			continue
		}
		if !accessScope.AllowsRow(row) {
			continue
		}
		dueDate, alertStatus, err := s.resolveDueDateAndStatus(ctx, row, companyCtx, now, typeConfigCache)
		if err != nil {
			return nil, err
		}
		if dueDate == "" {
			continue
		}
		if statusFilter != "" && alertStatus != statusFilter {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(DisplayAlertTitle(row)), query) && !strings.Contains(strings.ToLower(row.Title), query) {
			continue
		}
		if !matchesDateRange(dueDate, startDate, endDate) {
			continue
		}
		if departmentID != "" && !rowMatchesDepartmentAliases(row, departmentAliases) {
			continue
		}
		if displayGroupCode != "" && !rowMatchesDisplayGroup(row.TypeID, displayGroupCode, codesByType) {
			continue
		}

		rawDepts := ActiveDepartmentsFromRow(row.CurrentStepCode, row.CurrentStepDepartment, row.SnapshotJSON)
		enriched = append(enriched, DeadlineAlertDTO{
			AlertID:            row.RecordID,
			RecordID:           row.RecordID,
			WorkflowInstanceID: strings.TrimSpace(row.WorkflowInstanceID),
			TypeID:             row.TypeID,
			Title:              DisplayAlertTitle(row),
			DueDate:            dueDate,
			Status:             alertStatus,
			ActiveDepartments:  ResolveActiveDepartmentLabels(rawDepts, deptDict),
			CurrentStepName:    CurrentStepNameFromRow(row.CurrentStepCode, row.CurrentStepName, row.SnapshotJSON),
			Source:             "disclosure_record",
			TemplateCategory:   strings.TrimSpace(row.TemplateCategory),
		})
	}

	total := len(enriched)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	items := enriched[start:end]
	if items == nil {
		items = []DeadlineAlertDTO{}
	}

	return &ListDeadlineAlertsResponse{
		Items:    items,
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	}, nil
}

func (s *service) ListDeadlineAlertFilterOptions(ctx context.Context, sub Subject) (*DeadlineAlertFilterOptionsResponse, error) {
	if strings.TrimSpace(sub.CompanyID) == "" {
		return nil, perr.NewHTTPError(http.StatusUnprocessableEntity, perr.CodeCompanyContextRequired, "company_id is required", nil)
	}
	if err := s.authorizeView(ctx, sub); err != nil {
		return nil, err
	}
	departments, err := s.repo.ListCompanyDepartments(ctx, sub.CompanyID)
	if err != nil {
		return nil, err
	}
	templateDepts, err := s.repo.ListTemplateDepartments(ctx)
	if err != nil {
		return nil, err
	}
	reportGroups, err := s.repo.ListReportGroupOptions(ctx)
	if err != nil {
		return nil, err
	}
	departments = MergeDepartmentFilterOptions(departments, templateDepts)
	if departments == nil {
		departments = []DeadlineAlertFilterOptionDTO{}
	}
	if reportGroups == nil {
		reportGroups = []DeadlineAlertFilterOptionDTO{}
	}
	return &DeadlineAlertFilterOptionsResponse{
		Departments:  departments,
		ReportGroups: reportGroups,
	}, nil
}

func rowMatchesDepartmentAliases(row AlertRow, aliases []string) bool {
	if len(aliases) == 0 {
		return true
	}
	targets := map[string]struct{}{}
	for _, a := range aliases {
		key := strings.ToLower(strings.TrimSpace(a))
		if key != "" {
			targets[key] = struct{}{}
		}
	}
	if len(targets) == 0 {
		return true
	}
	match := func(v string) bool {
		key := strings.ToLower(strings.TrimSpace(v))
		if key == "" {
			return false
		}
		_, ok := targets[key]
		return ok
	}
	if match(row.RecordDepartmentID) || match(row.CurrentStepDepartment) {
		return true
	}
	for _, d := range ActiveDepartmentsFromRow(row.CurrentStepCode, row.CurrentStepDepartment, row.SnapshotJSON) {
		if match(d) {
			return true
		}
	}
	return false
}

func rowMatchesDepartment(row AlertRow, departmentID, departmentName string) bool {
	aliases := make([]string, 0, 2)
	if id := strings.TrimSpace(departmentID); id != "" {
		aliases = append(aliases, id)
	}
	if name := strings.TrimSpace(departmentName); name != "" {
		aliases = append(aliases, name)
	}
	return rowMatchesDepartmentAliases(row, aliases)
}

func rowMatchesDisplayGroup(typeID, displayGroupCode string, codesByType map[string][]string) bool {
	want := strings.ToLower(strings.TrimSpace(displayGroupCode))
	if want == "" {
		return true
	}
	codes := codesByType[strings.TrimSpace(typeID)]
	for _, code := range codes {
		if strings.ToLower(strings.TrimSpace(code)) == want {
			return true
		}
	}
	return false
}

func (s *service) authorize(ctx context.Context, sub Subject) error {
	return s.authorizeView(ctx, sub)
}

func (s *service) authorizeView(ctx context.Context, sub Subject) error {
	decision, err := s.auth.Authorize(ctx, authapp.AuthorizeRequest{
		Subject: authapp.SubjectRef{
			UserID:       sub.UserID,
			MembershipID: sub.MembershipID,
			CompanyID:    sub.CompanyID,
		},
		Action: "deadline.view",
		Resource: authapp.ResourceRef{
			Type: "disclosure_record",
			ID:   "",
			Attributes: map[string]any{
				"workflow_state": "*",
			},
		},
	})
	if err != nil {
		return fmt.Errorf("authorize deadline.view: %w", err)
	}
	if decision.Decision != authapp.DecisionAllow {
		return perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "deadline.view permission required", nil)
	}
	return nil
}

func (s *service) authorizeConfirm(ctx context.Context, sub Subject, recordID string) error {
	decision, err := s.auth.Authorize(ctx, authapp.AuthorizeRequest{
		Subject: authapp.SubjectRef{
			UserID:       sub.UserID,
			MembershipID: sub.MembershipID,
			CompanyID:    sub.CompanyID,
		},
		Action: "deadline.confirm",
		Resource: authapp.ResourceRef{
			Type: "disclosure_record",
			ID:   strings.TrimSpace(recordID),
			Attributes: map[string]any{
				"workflow_state": "*",
			},
		},
	})
	if err != nil {
		return fmt.Errorf("authorize deadline.confirm: %w", err)
	}
	if decision.Decision != authapp.DecisionAllow {
		return perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "deadline.manage permission required", nil)
	}
	return nil
}

func (s *service) ConfirmDeadlineAlert(ctx context.Context, req ConfirmDeadlineAlertRequest) (*ConfirmDeadlineAlertResponse, error) {
	if strings.TrimSpace(req.Subject.CompanyID) == "" {
		return nil, perr.NewHTTPError(http.StatusUnprocessableEntity, perr.CodeCompanyContextRequired, "company_id is required", nil)
	}
	recordID := strings.TrimSpace(req.RecordID)
	if recordID == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "record_id is required", nil)
	}
	if err := s.authorizeConfirm(ctx, req.Subject, recordID); err != nil {
		return nil, err
	}
	exists, err := s.repo.HasDisclosureRecord(ctx, req.Subject.CompanyID, recordID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "record not found", nil)
	}
	confirmedAt := s.now().UTC()
	if err := s.repo.ConfirmDeadlineAlert(
		ctx,
		req.Subject.CompanyID,
		recordID,
		req.Subject.MembershipID,
		strings.TrimSpace(req.Note),
		strings.TrimSpace(req.IdempotencyKey),
		confirmedAt,
	); err != nil {
		return nil, err
	}
	return &ConfirmDeadlineAlertResponse{
		RecordID:    recordID,
		CompanyID:   req.Subject.CompanyID,
		ConfirmedBy: req.Subject.MembershipID,
		ConfirmedAt: confirmedAt.Format(time.RFC3339),
	}, nil
}

func (s *service) resolveDueDateAndStatus(
	ctx context.Context,
	row AlertRow,
	companyCtx disclosureapp.CompanyDeadlineContext,
	now time.Time,
	typeConfigCache map[string]*disclosureapp.TemplateDeadlineConfig,
) (dueDate string, alertStatus string, err error) {
	if row.ConfirmedAt != nil {
		due := firstNonEmpty(row.PlannedDate, row.AdHocDeadlineDate)
		if due == "" {
			due = now.In(s.calculatorLocation()).Format("2006-01-02")
		}
		return due, "DONE", nil
	}
	terminal := isTerminalRecordStatus(row.RecordStatus)
	if terminal {
		due := firstNonEmpty(row.PlannedDate, row.AdHocDeadlineDate)
		if due == "" {
			due = now.In(s.calculatorLocation()).Format("2006-01-02")
		}
		return due, "PENDING_CONFIRM", nil
	}

	due := strings.TrimSpace(row.PlannedDate)
	if due == "" {
		due = strings.TrimSpace(row.AdHocDeadlineDate)
	}
	if due == "" {
		due, err = s.dueDateFromTypeConfig(ctx, row, companyCtx, now, typeConfigCache)
		if err != nil {
			return "", "", err
		}
	}
	if due == "" {
		return "", "", nil
	}

	remaining := remainingDaysFromDue(due, now, s.calculatorLocation())
	return due, alertStatusFromRemainingDays(remaining, false), nil
}

func (s *service) dueDateFromTypeConfig(
	ctx context.Context,
	row AlertRow,
	companyCtx disclosureapp.CompanyDeadlineContext,
	now time.Time,
	cache map[string]*disclosureapp.TemplateDeadlineConfig,
) (string, error) {
	cfg, err := s.typeDeadlineConfig(ctx, row.TypeID, row.CompanyID, row.DeadlineConfigJSON, cache)
	if err != nil || cfg == nil {
		return "", err
	}
	summary, err := s.calculator.CalculateDeadlineSummary(ctx, cfg, companyCtx, now)
	if err != nil || summary == nil {
		return "", err
	}
	if summary.ActualDeadline != nil && strings.TrimSpace(*summary.ActualDeadline) != "" {
		return strings.TrimSpace(*summary.ActualDeadline), nil
	}
	if summary.DeadlineDate != nil && strings.TrimSpace(*summary.DeadlineDate) != "" {
		return strings.TrimSpace(*summary.DeadlineDate), nil
	}
	return "", nil
}

func (s *service) typeDeadlineConfig(
	ctx context.Context,
	typeID, companyID string,
	inlineJSON []byte,
	cache map[string]*disclosureapp.TemplateDeadlineConfig,
) (*disclosureapp.TemplateDeadlineConfig, error) {
	if len(inlineJSON) > 0 {
		var cfg disclosureapp.TemplateDeadlineConfig
		if err := json.Unmarshal(inlineJSON, &cfg); err == nil && strings.TrimSpace(cfg.DeadlineMode) != "" {
			return &cfg, nil
		}
	}
	key := typeID + "|" + companyID
	if cached, ok := cache[key]; ok {
		return cached, nil
	}
	cfg, err := s.repo.GetTypeDeadlineConfig(ctx, companyID, typeID)
	if err != nil {
		return nil, err
	}
	cache[key] = cfg
	return cfg, nil
}

func (s *service) calculatorLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		return time.FixedZone("Asia/Ho_Chi_Minh", 7*60*60)
	}
	return loc
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
