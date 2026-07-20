package app

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	auditapp "github.com/cobo/cobo_iam_services/internal/audit/app"
	"github.com/cobo/cobo_iam_services/internal/audit/timeline"
	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	inappapp "github.com/cobo/cobo_iam_services/internal/inappnotification/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/personalops/domain"
)

const (
	myTasksLimit    = 20
	activitiesLimit = 12
)

type Service interface {
	GetOperationalOverview(ctx context.Context, sub Subject) (*domain.OverviewResponse, error)
}

type IdentityReader interface {
	GetByUserID(ctx context.Context, userID string) (*iamapp.AuthenticatedUser, error)
}

type ContactReader interface {
	GetContact(ctx context.Context, userID string) (email, phone string, err error)
}

type AvatarURLReader interface {
	AvatarURL(ctx context.Context, userID string) (*string, error)
}

type EmailVerifiedReader interface {
	IsEmailVerified(ctx context.Context, userID string) (bool, error)
}

type Authorizer interface {
	GetEffectiveAccess(ctx context.Context, membershipID, companyID string) (*authapp.EffectiveAccessSummary, error)
}

type InAppLister interface {
	List(ctx context.Context, userID, companyID string) ([]inappapp.InAppNotification, error)
}

type AuditLister interface {
	ListFiltered(ctx context.Context, filter auditapp.ListFilter) ([]auditapp.Entry, error)
}

type service struct {
	members   caapp.MembershipQueryService
	mine      MineRepository
	identity  IdentityReader
	contact   ContactReader // optional
	avatar    AvatarURLReader // optional
	emailVer  EmailVerifiedReader // optional
	auth      Authorizer // optional — admin scopes
	inApp     InAppLister // optional
	audit     AuditLister // optional — personal activity log
	clock     Clock
	loc       *time.Location
}

func NewService(
	members caapp.MembershipQueryService,
	mine MineRepository,
	identity IdentityReader,
	auth Authorizer,
	inApp InAppLister,
	opts ...Option,
) Service {
	s := &service{
		members:  members,
		mine:     mine,
		identity: identity,
		auth:     auth,
		inApp:    inApp,
		clock:    systemClock{},
		loc:      time.Local,
	}
	if s.mine == nil {
		s.mine = EmptyMineRepository{}
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

type Option func(*service)

func WithContactReader(c ContactReader) Option {
	return func(s *service) { s.contact = c }
}

func WithAvatarURLReader(a AvatarURLReader) Option {
	return func(s *service) { s.avatar = a }
}

func WithInAppLister(l InAppLister) Option {
	return func(s *service) { s.inApp = l }
}

func WithAuditLister(a AuditLister) Option {
	return func(s *service) { s.audit = a }
}

func WithClock(c Clock) Option {
	return func(s *service) { s.clock = c }
}

func WithLocation(loc *time.Location) Option {
	return func(s *service) {
		if loc != nil {
			s.loc = loc
		}
	}
}

func (s *service) GetOperationalOverview(ctx context.Context, sub Subject) (*domain.OverviewResponse, error) {
	if strings.TrimSpace(sub.UserID) == "" {
		return nil, perr.NewHTTPError(http.StatusUnauthorized, perr.CodeSessionExpired, "authentication required", nil)
	}
	now := s.clock.Now()
	warnings := make([]domain.Warning, 0)
	sources := make([]string, 0, 8)
	partial := false

	user, err := s.identity.GetByUserID(ctx, sub.UserID)
	if err != nil {
		return nil, err
	}
	sources = append(sources, "identity")

	memberships, err := s.members.GetMembershipsByUser(ctx, sub.UserID)
	if err != nil {
		return nil, fmt.Errorf("list memberships: %w", err)
	}
	sources = append(sources, "memberships")

	active := make([]caapp.MembershipView, 0, len(memberships))
	membershipIDs := make([]string, 0, len(memberships))
	for _, m := range memberships {
		if !strings.EqualFold(strings.TrimSpace(m.Status), "active") {
			continue
		}
		active = append(active, m)
		membershipIDs = append(membershipIDs, m.MembershipID)
	}

	profile := domain.ProfileBrief{
		UserID:           user.UserID,
		DisplayName:      user.FullName,
		SubscriptionTier: user.SubscriptionTier,
		Email:            user.LoginID,
	}
	if s.contact != nil {
		email, phone, cerr := s.contact.GetContact(ctx, sub.UserID)
		if cerr != nil {
			partial = true
			warnings = append(warnings, warn("contact_unavailable", "Không tải được email/phone hồ sơ."))
		} else {
			if strings.TrimSpace(email) != "" {
				profile.Email = email
			}
			profile.Phone = phone
			sources = append(sources, "contact")
		}
	}
	if s.avatar != nil {
		if url, aerr := s.avatar.AvatarURL(ctx, sub.UserID); aerr == nil {
			profile.AvatarURL = url
			sources = append(sources, "avatar")
		}
	}
	if s.emailVer != nil {
		if ok, verr := s.emailVer.IsEmailVerified(ctx, sub.UserID); verr == nil {
			profile.EmailVerified = ok
			sources = append(sources, "email_verified")
		} else {
			partial = true
			warnings = append(warnings, warn("email_verified_unavailable", "Không xác định được trạng thái xác thực email."))
		}
	}

	roleAssignments, activeRoleCount, roleWarn := s.buildRoleAssignments(ctx, active)
	if roleWarn != nil {
		partial = true
		warnings = append(warnings, *roleWarn)
	} else {
		sources = append(sources, "roles_titles_departments")
	}

	adminScopes, adminPartial, adminWarns := s.buildAdminScopes(ctx, active, sub.CompanyID)
	if adminPartial {
		partial = true
	}
	warnings = append(warnings, adminWarns...)
	if len(adminWarns) == 0 {
		sources = append(sources, "effective_access")
	}

	var mineRecords []MineRecord
	var mineTasks []MineTask
	adhocDues := map[string]string{}
	if len(membershipIDs) > 0 {
		var merr error
		mineRecords, merr = s.mine.ListMineRecords(ctx, membershipIDs)
		if merr != nil {
			partial = true
			warnings = append(warnings, warn("mine_records_unavailable", "Không tải được cảnh báo mine; KPI assigned/overdue tạm unavailable."))
			mineRecords = nil
		} else {
			sources = append(sources, "mine_workflow_tasks", "mine_assignments")
		}
		mineTasks, merr = s.mine.ListMineOpenTasks(ctx, membershipIDs, myTasksLimit)
		if merr != nil {
			partial = true
			warnings = append(warnings, warn("mine_tasks_unavailable", "Không tải được my_tasks."))
			mineTasks = nil
		} else {
			sources = append(sources, "mine_open_tasks")
		}
		companyIDs := make([]string, 0, len(active))
		for _, m := range active {
			companyIDs = append(companyIDs, m.CompanyID)
		}
		adhocDues, merr = s.mine.ListApprovedAdHocDues(ctx, companyIDs)
		if merr != nil {
			partial = true
			warnings = append(warnings, warn("ADHOC_DUE_SOURCE_UNAVAILABLE", "Không tải được ad-hoc approved deadline; due dùng planned_date fallback khi có."))
			adhocDues = map[string]string{}
		} else {
			sources = append(sources, "ad_hoc_approved_deadline")
		}
	}

	applyAdHoc := func(companyID, recordID, planned string) (plannedOut, adhocOut string) {
		plannedOut = planned
		if adhocDues != nil {
			adhocOut = adhocDues[companyID+"|"+recordID]
		}
		return plannedOut, adhocOut
	}
	for i := range mineRecords {
		_, ah := applyAdHoc(mineRecords[i].CompanyID, mineRecords[i].RecordID, mineRecords[i].PlannedDate)
		mineRecords[i].AdHocDueDate = ah
	}
	for i := range mineTasks {
		_, ah := applyAdHoc(mineTasks[i].CompanyID, mineTasks[i].RecordID, mineTasks[i].PlannedDate)
		mineTasks[i].AdHocDueDate = ah
	}

	companyOverviews, assignedTotal, overdueTotal, dueSourcesUsed, onTimeSampleTotal := s.buildCompanyOverviews(active, mineRecords, now)
	warnings = append(warnings, warn(
		"OUTCOME_TIMESTAMP_FORWARD_ONLY",
		"Outcome timestamp chỉ được ghi nhận cho dữ liệu mới từ thời điểm triển khai, dữ liệu cũ không được backfill nếu thiếu source tin cậy.",
	))
	if onTimeSampleTotal == 0 {
		warnings = append(warnings, warn(
			"ON_TIME_RATE_SAMPLE_EMPTY",
			"Chưa có hồ sơ hoàn thành có đủ due date và outcome timestamp để tính tỷ lệ đúng hạn.",
		))
	}
	warnings = append(warnings, warn(
		"WORKFLOW_TASK_DUE_ABSENT",
		"workflow_tasks không có cột due_date; due ưu tiên ad-hoc approved rồi planned_date_fallback.",
	))
	partial = true // forward-only outcome + workflow due absent remain non-blocking gaps

	myTasks := s.buildMyTasks(mineTasks, now)
	activities, actPartial, actWarn := s.buildReportActivities(ctx, sub.UserID, active, sub.CompanyID)
	if actPartial {
		partial = true
	}
	if actWarn != nil {
		warnings = append(warnings, *actWarn)
	} else if s.inApp != nil {
		sources = append(sources, "in_app_notifications")
	}
	activityLog, logPartial, logWarn := s.buildActivityLog(ctx, sub.UserID)
	if logPartial {
		partial = true
	}
	if logWarn != nil {
		warnings = append(warnings, *logWarn)
	} else if s.audit != nil {
		sources = append(sources, "audit_logs")
	}

	var assignedMetric, overdueMetric domain.Metric
	if containsWarningCode(warnings, "mine_records_unavailable") {
		assignedMetric = toMetric(UnavailableMetric("mine_records_unavailable"))
		overdueMetric = toMetric(UnavailableMetric("mine_records_unavailable"))
	} else {
		assignedMetric = toMetric(ExactMetric(assignedTotal))
		overdueMetric = toMetric(ExactMetric(overdueTotal))
		overdueMetric.DeadlineSources = dueSourcesUsed
	}

	resp := &domain.OverviewResponse{
		Profile: profile,
		Kpis: domain.KpiBlock{
			LinkedCompanies: toMetric(ExactMetric(len(active))),
			ActiveRoles:     toMetric(ExactMetric(activeRoleCount)),
			AssignedAlerts:  assignedMetric,
			OverdueAlerts:   overdueMetric,
		},
		CompanyOverviews: companyOverviews,
		MyTasks:          myTasks,
		RoleAssignments:  roleAssignments,
		AdminScopes:      adminScopes,
		Activities:       activities,
		ActivityLog:      activityLog,
		Meta: domain.MetaBlock{
			GeneratedAt: now.UTC().Format(time.RFC3339),
			Partial:     partial,
			Warnings:    warnings,
			Sources:     uniqueStrings(sources),
		},
	}
	ensureEmptySlices(resp)
	return resp, nil
}

func (s *service) buildRoleAssignments(ctx context.Context, active []caapp.MembershipView) ([]domain.RoleAssignment, int, *domain.Warning) {
	out := make([]domain.RoleAssignment, 0, len(active))
	roleKeys := map[string]struct{}{}
	var firstErr error
	for _, m := range active {
		roles, err := s.members.GetMembershipRoles(ctx, m.MembershipID)
		if err != nil {
			firstErr = err
			roles = nil
		}
		titles, err := s.members.GetMembershipTitles(ctx, m.MembershipID)
		if err != nil {
			firstErr = err
			titles = nil
		}
		depts, err := s.members.GetMembershipDepartments(ctx, m.MembershipID)
		if err != nil {
			firstErr = err
			depts = nil
		}
		var deptName *string
		if len(depts) > 0 {
			name := depts[0].DepartmentName
			if name == "" {
				name = depts[0].Name
			}
			if name != "" {
				deptName = &name
			}
		}
		if roles == nil {
			roles = []string{}
		}
		if titles == nil {
			titles = []string{}
		}
		for _, r := range roles {
			r = strings.TrimSpace(r)
			if r == "" {
				continue
			}
			roleKeys[m.CompanyID+"|"+r] = struct{}{}
		}
		out = append(out, domain.RoleAssignment{
			CompanyID:      m.CompanyID,
			CompanyName:    m.CompanyName,
			MembershipID:   m.MembershipID,
			DepartmentName: deptName,
			TitleNames:     titles,
			RoleNames:      roles,
			Status:         "active",
		})
	}
	if firstErr != nil {
		w := warn("roles_partial", "Một số vai trò/phòng ban/chức danh không tải được đầy đủ.")
		return out, len(roleKeys), &w
	}
	return out, len(roleKeys), nil
}

func (s *service) buildAdminScopes(ctx context.Context, active []caapp.MembershipView, activeCompanyID string) ([]domain.AdminScope, bool, []domain.Warning) {
	out := make([]domain.AdminScope, 0, len(active))
	warnings := make([]domain.Warning, 0)
	partial := false
	if s.auth == nil {
		for _, m := range active {
			out = append(out, domain.AdminScope{
				CompanyID:   m.CompanyID,
				CompanyName: m.CompanyName,
				Note:        ptrString("effective_access_unavailable"),
			})
		}
		return out, true, []domain.Warning{warn("admin_scopes_unavailable", "Không có authorizer để suy ra phạm vi quản trị.")}
	}
	for _, m := range active {
		eff, err := s.auth.GetEffectiveAccess(ctx, m.MembershipID, m.CompanyID)
		if err != nil {
			partial = true
			warnings = append(warnings, warn("admin_scope_company_failed", fmt.Sprintf("Không tải effective-access cho %s.", m.CompanyName)))
			out = append(out, domain.AdminScope{
				CompanyID:   m.CompanyID,
				CompanyName: m.CompanyName,
				Note:        ptrString("effective_access_failed"),
			})
			continue
		}
		perms := eff.Permissions
		canManage := hasPerm(perms, "rbac.manage") || eff.DataScope.HasCompanyWideAccess
		canEdit := canManage || hasEditish(perms)
		canView := true
		href := "/app/dashboard"
		if canManage {
			href = "/app/admin"
		}
		scope := domain.AdminScope{
			CompanyID:   m.CompanyID,
			CompanyName: m.CompanyName,
			CanView:     &canView,
			CanEdit:     &canEdit,
			CanManage:   &canManage,
			Href:        &href,
		}
		if activeCompanyID != "" && m.CompanyID == activeCompanyID {
			// no-op; already exact
		}
		out = append(out, scope)
	}
	return out, partial, warnings
}

func (s *service) buildCompanyOverviews(active []caapp.MembershipView, records []MineRecord, now time.Time) ([]domain.CompanyOverview, int, int, []string, int) {
	type agg struct {
		name           string
		assigned       int
		overdue        int
		dueSoon        int
		completed      int
		onTimeOn       int
		onTimeTot      int
		seen           map[string]struct{}
	}
	byCompany := map[string]*agg{}
	for _, m := range active {
		byCompany[m.CompanyID] = &agg{name: m.CompanyName, seen: map[string]struct{}{}}
	}
	assignedTotal := 0
	overdueTotal := 0
	onTimeSampleTotal := 0
	dueSources := map[string]struct{}{}
	for _, rec := range records {
		a := byCompany[rec.CompanyID]
		if a == nil {
			continue
		}
		if _, ok := a.seen[rec.RecordID]; ok {
			continue
		}
		a.seen[rec.RecordID] = struct{}{}
		resolved := ResolveDeadline(rec.PlannedDate, rec.AdHocDueDate)
		if resolved.Source != DeadlineSourceUnavailable {
			dueSources[resolved.Source] = struct{}{}
		}
		status := ClassifyMineAlert(rec.RecordStatus, resolved.Date, rec.Confirmed, now, s.loc)
		switch status {
		case AlertDONE:
			a.completed++
		case AlertOVERDUE:
			a.assigned++
			a.overdue++
			assignedTotal++
			overdueTotal++
		case AlertDUE_SOON:
			a.assigned++
			a.dueSoon++
			assignedTotal++
		case AlertPENDINGConfirm, AlertUPCOMING:
			a.assigned++
			assignedTotal++
		default:
			a.assigned++
			assignedTotal++
		}
		// on_time_rate: terminal + reliable due + reliable completed_at only (no status-only, no updated_at).
		if IsTerminalRecordStatus(rec.RecordStatus) &&
			resolved.Source != DeadlineSourceUnavailable &&
			rec.CompletedAt != nil {
			a.onTimeTot++
			onTimeSampleTotal++
			if IsOutcomeOnTime(*rec.CompletedAt, resolved.Date, s.loc) {
				a.onTimeOn++
			}
		}
	}
	out := make([]domain.CompanyOverview, 0, len(active))
	for _, m := range active {
		a := byCompany[m.CompanyID]
		out = append(out, domain.CompanyOverview{
			CompanyID:       m.CompanyID,
			CompanyName:     m.CompanyName,
			AssignedAlerts:  toMetric(ExactMetric(a.assigned)),
			OverdueAlerts:   toMetric(ExactMetric(a.overdue)),
			DueSoonAlerts:   toMetric(ExactMetric(a.dueSoon)),
			CompletedAlerts: toMetric(ExactMetric(a.completed)),
			OnTimeRate:      ComputeOnTimeRate(a.onTimeOn, a.onTimeTot),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CompanyName < out[j].CompanyName
	})
	sourcesUsed := make([]string, 0, len(dueSources))
	for src := range dueSources {
		sourcesUsed = append(sourcesUsed, src)
	}
	sort.Strings(sourcesUsed)
	return out, assignedTotal, overdueTotal, sourcesUsed, onTimeSampleTotal
}

func (s *service) buildMyTasks(tasks []MineTask, now time.Time) []domain.MyTaskItem {
	out := make([]domain.MyTaskItem, 0, len(tasks))
	for _, t := range tasks {
		resolved := ResolveDeadline(t.PlannedDate, t.AdHocDueDate)
		status, label := TaskStatusLabel(resolved.Date, now, s.loc)
		title := strings.TrimSpace(t.Title)
		if title == "" {
			title = t.RecordID
		}
		step := strings.TrimSpace(t.StepCode)
		if step == "" {
			step = "—"
		}
		out = append(out, domain.MyTaskItem{
			ID:               t.TaskID,
			CompanyID:        t.CompanyID,
			CompanyName:      t.CompanyName,
			AlertID:          t.RecordID,
			AlertTitle:       title,
			MyRoleLabel:      "Người xử lý",
			CurrentStepLabel: step,
			Deadline:         resolved.Date,
			DeadlineSource:   resolved.Source,
			DeadlineAccuracy: resolved.Accuracy,
			Status:           status,
			StatusLabel:      label,
			Action: domain.MyTaskAction{
				Label: "Xử lý",
				Href:  "/app/deadlines/" + t.RecordID,
			},
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		rank := func(st string) int {
			switch st {
			case "overdue":
				return 0
			case "due_soon":
				return 1
			default:
				return 2
			}
		}
		ri, rj := rank(out[i].Status), rank(out[j].Status)
		if ri != rj {
			return ri < rj
		}
		return out[i].Deadline < out[j].Deadline
	})
	return out
}

func (s *service) buildReportActivities(ctx context.Context, userID string, active []caapp.MembershipView, preferredCompanyID string) ([]domain.ActivityItem, bool, *domain.Warning) {
	if s.inApp == nil {
		w := warn("activities_unavailable", "Nguồn in-app notifications không sẵn sàng.")
		return []domain.ActivityItem{}, true, &w
	}
	companyIDs := make([]string, 0, len(active))
	if preferredCompanyID != "" {
		companyIDs = append(companyIDs, preferredCompanyID)
	}
	for _, m := range active {
		if m.CompanyID == preferredCompanyID {
			continue
		}
		companyIDs = append(companyIDs, m.CompanyID)
	}
	collected := make([]domain.ActivityItem, 0, activitiesLimit)
	var firstErr error
	for _, cid := range companyIDs {
		if len(collected) >= activitiesLimit {
			break
		}
		list, err := s.inApp.List(ctx, userID, cid)
		if err != nil {
			firstErr = err
			continue
		}
		for _, n := range list {
			if len(collected) >= activitiesLimit {
				break
			}
			if !isReportRelatedNotification(n) {
				continue
			}
			href := "/app/profile"
			if n.ResourceType != nil && n.ResourceID != nil &&
				strings.EqualFold(strings.TrimSpace(*n.ResourceType), inappapp.ResourceTypeDisclosure) {
				href = "/app/deadlines/" + strings.TrimSpace(*n.ResourceID)
			}
			collected = append(collected, domain.ActivityItem{
				ID:          n.ID,
				Title:       n.Title,
				Description: n.Body,
				CreatedAt:   n.CreatedAt.UTC().Format(time.RFC3339),
				Href:        href,
				Source:      "in_app_notifications",
			})
		}
	}
	if firstErr != nil && len(collected) == 0 {
		w := warn("activities_unavailable", "Không tải được hoạt động báo cáo gần đây.")
		return []domain.ActivityItem{}, true, &w
	}
	if firstErr != nil {
		w := warn("activities_partial", "Một phần hoạt động báo cáo theo công ty không tải được.")
		return collected, true, &w
	}
	return collected, false, nil
}

func isReportRelatedNotification(n inappapp.InAppNotification) bool {
	kind := strings.ToLower(strings.TrimSpace(n.Kind))
	switch {
	case strings.HasPrefix(kind, "reminder."):
		return true
	case strings.HasPrefix(kind, "adhoc."):
		return true
	case strings.HasPrefix(kind, "disclosure."):
		return true
	case strings.HasPrefix(kind, "workflow."):
		return true
	case strings.HasPrefix(kind, "deadline."):
		return true
	}
	if n.ResourceType != nil {
		rt := strings.ToLower(strings.TrimSpace(*n.ResourceType))
		if rt == inappapp.ResourceTypeDisclosure || rt == inappapp.ResourceTypeAdHocProposal {
			return true
		}
	}
	return false
}

func (s *service) buildActivityLog(ctx context.Context, userID string) ([]domain.ActivityItem, bool, *domain.Warning) {
	if s.audit == nil {
		w := warn("activity_log_unavailable", "Nguồn audit log không sẵn sàng.")
		return []domain.ActivityItem{}, true, &w
	}
	entries, err := s.audit.ListFiltered(ctx, auditapp.ListFilter{
		ActorUserID: strings.TrimSpace(userID),
		Limit:       activitiesLimit,
	})
	if err != nil {
		w := warn("activity_log_unavailable", "Không tải được lịch sử thao tác của bạn.")
		return []domain.ActivityItem{}, true, &w
	}
	out := make([]domain.ActivityItem, 0, len(entries))
	for _, e := range entries {
		title := timeline.SummaryForAction(e.Action)
		desc := timeline.FriendlyDescription(e.Action, e.ResourceType)
		href := timeline.ActionLinkFor(e.Action)
		// Profile history deep-links into admin audit with a back marker when a link exists.
		if href == "/app/admin/audit" {
			href = "/app/admin/audit?from=profile-history"
		}
		out = append(out, domain.ActivityItem{
			ID:           e.EventID,
			Title:        title,
			Description:  desc,
			CreatedAt:    e.OccurredAt,
			Href:         href,
			Source:       "audit_logs",
			Action:       e.Action,
			ResourceType: e.ResourceType,
			ResourceID:   e.ResourceID,
			Domain:       timeline.DomainForAction(e.Action),
		})
	}
	return out, false, nil
}

func ensureEmptySlices(resp *domain.OverviewResponse) {
	if resp.CompanyOverviews == nil {
		resp.CompanyOverviews = []domain.CompanyOverview{}
	}
	if resp.MyTasks == nil {
		resp.MyTasks = []domain.MyTaskItem{}
	}
	if resp.RoleAssignments == nil {
		resp.RoleAssignments = []domain.RoleAssignment{}
	}
	if resp.AdminScopes == nil {
		resp.AdminScopes = []domain.AdminScope{}
	}
	if resp.Activities == nil {
		resp.Activities = []domain.ActivityItem{}
	}
	if resp.ActivityLog == nil {
		resp.ActivityLog = []domain.ActivityItem{}
	}
	if resp.Meta.Warnings == nil {
		resp.Meta.Warnings = []domain.Warning{}
	}
	if resp.Meta.Sources == nil {
		resp.Meta.Sources = []string{}
	}
}

func toMetric(m domainMetric) domain.Metric {
	return domain.Metric{Value: m.Value, Accuracy: m.Accuracy, Reason: m.Reason}
}

func warn(code, message string) domain.Warning {
	return domain.Warning{Code: code, Message: message}
}

func containsWarningCode(ws []domain.Warning, code string) bool {
	for _, w := range ws {
		if w.Code == code {
			return true
		}
	}
	return false
}

func hasPerm(perms []string, want string) bool {
	for _, p := range perms {
		if p == want {
			return true
		}
	}
	return false
}

func hasEditish(perms []string) bool {
	for _, p := range perms {
		if strings.Contains(p, ".update") || strings.Contains(p, ".edit") || strings.Contains(p, ".manage") {
			return true
		}
	}
	return false
}

func uniqueStrings(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
