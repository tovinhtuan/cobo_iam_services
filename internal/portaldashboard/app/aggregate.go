package app

import (
	"math"
	"sort"
	"strings"
	"time"

	deadlinealertsapp "github.com/cobo/cobo_iam_services/internal/deadlinealerts/app"
	adhocapp "github.com/cobo/cobo_iam_services/internal/adhoc/app"
	inappapp "github.com/cobo/cobo_iam_services/internal/inappnotification/app"
	"github.com/cobo/cobo_iam_services/internal/portaldashboard/domain"
)

type deadlineFetch struct {
	overdue        []deadlinealertsapp.DeadlineAlertDTO
	dueSoon        []deadlinealertsapp.DeadlineAlertDTO
	pendingConfirm []deadlinealertsapp.DeadlineAlertDTO
	upcoming       []deadlinealertsapp.DeadlineAlertDTO
	dueIn7Days     int
	overdueTotal   int
	err            error
}

type adHocFetch struct {
	pendingFocal []adhocapp.ProposalDTO
	pendingAdmin []adhocapp.ProposalDTO
	rejected     []adhocapp.ProposalDTO
	pendingTotal int
	rejectedTotal int
	skipped      bool
	warn         string
}

type inAppFetch struct {
	items []inappapp.InAppNotification
	err   error
	warn  string
}

func mapKpi(k domainKpi) domain.KpiMetric {
	return domain.KpiMetric{
		Value:    k.Value,
		Unit:     k.Unit,
		Severity: k.Severity,
		Source:   k.Source,
		Accuracy: k.Accuracy,
		Reason:   k.Reason,
	}
}

func buildOverview(
	company domain.CompanyBrief,
	dr domain.DateRange,
	deadlines deadlineFetch,
	adHoc adHocFetch,
	inApp inAppFetch,
) domain.OverviewResponse {
	ref := dr.Now
	kpis := map[string]domain.KpiMetric{}

	// open_overdue — exact
	if deadlines.err != nil {
		kpis[KpiOpenOverdue] = mapKpi(unavailableKpi("count", "deadline_source_unavailable"))
	} else {
		kpis[KpiOpenOverdue] = mapKpi(countKpi(
			float64(deadlines.overdueTotal),
			"count",
			severityFromOverdueCount(deadlines.overdueTotal),
			SourceDeadlineAlerts,
			AccuracyExact,
		))
	}

	// needs_action_now — estimate (deadline exact + optional ad-hoc)
	needsIDs := map[string]struct{}{}
	for _, a := range deadlines.overdue {
		needsIDs[a.RecordID] = struct{}{}
	}
	for _, a := range deadlines.dueSoon {
		if daysUntilDue(a.DueDate, ref) <= 1 {
			needsIDs[a.RecordID] = struct{}{}
		}
	}
	for _, a := range deadlines.pendingConfirm {
		needsIDs[a.RecordID] = struct{}{}
	}
	adHocInNeeds := 0
	for _, p := range append(adHoc.pendingFocal, adHoc.pendingAdmin...) {
		key := "adhoc:" + p.ProposalID
		if _, ok := needsIDs[key]; !ok {
			needsIDs[key] = struct{}{}
			adHocInNeeds++
		}
	}
	needsAccuracy := AccuracyExact
	if adHocInNeeds > 0 {
		needsAccuracy = AccuracyEstimate
	}
	if deadlines.err != nil {
		kpis[KpiNeedsActionNow] = mapKpi(unavailableKpi("count", "deadline_source_unavailable"))
	} else {
		kpis[KpiNeedsActionNow] = mapKpi(countKpi(
			float64(len(needsIDs)),
			"count",
			severityFromNeedsAction(len(needsIDs)),
			SourceDeadlineAlerts,
			needsAccuracy,
		))
	}

	// due_next_7_days — exact from BE due date window
	if deadlines.err != nil {
		kpis[KpiDueNext7Days] = mapKpi(unavailableKpi("count", "deadline_source_unavailable"))
	} else {
		kpis[KpiDueNext7Days] = mapKpi(countKpi(
			float64(deadlines.dueIn7Days),
			"count",
			severityFromDueSoonCount(deadlines.dueIn7Days),
			SourceDeadlineAlerts,
			AccuracyExact,
		))
	}

	// blocked_or_exception — estimate from rejected ad-hoc + escalation notifications
	blockedCount := 0.0
	blockedAccuracy := AccuracyUnavailable
	if adHoc.skipped && inApp.err != nil {
		kpis[KpiBlockedOrException] = mapKpi(unavailableKpi("count", ReasonOfficialDefinitionRequired))
	} else {
		blockedCount = float64(adHoc.rejectedTotal) + float64(countEscalationNotifications(inApp.items))
		blockedAccuracy = AccuracyEstimate
		kpis[KpiBlockedOrException] = mapKpi(countKpi(
			blockedCount,
			"count",
			severityFromBlockedEstimate(int(blockedCount)),
			SourceDerived,
			blockedAccuracy,
		))
	}

	// pending_approval
	if adHoc.skipped && adHoc.pendingTotal == 0 {
		kpis[KpiPendingApproval] = mapKpi(unavailableKpi("count", ReasonOfficialDefinitionRequired))
	} else {
		kpis[KpiPendingApproval] = mapKpi(countKpi(
			float64(adHoc.pendingTotal),
			"count",
			severityFromPendingApproval(adHoc.pendingTotal),
			SourceAdHoc,
			AccuracyEstimate,
		))
	}

	// on_time_rate (percent of completed-on-time) remains unavailable in overview —
	// completion sampling is Personal Ops. The health block uses on_time_count instead.
	kpis[KpiOnTimeRate] = mapKpi(unavailableKpi("percent", ReasonCompletionTimestampOrDefinitionMissing))

	overdueItems := deadlines.overdue
	buckets := bucketOverdueAge(overdueItems, ref)
	totalOverdue := deadlines.overdueTotal
	if totalOverdue == 0 {
		totalOverdue = len(overdueItems)
	}
	onTimeCount := countOnTimeOpenAlerts(deadlines.dueSoon, deadlines.pendingConfirm, deadlines.upcoming)

	openAlerts := append(append(deadlines.dueSoon, deadlines.pendingConfirm...), deadlines.upcoming...)
	immediate := buildImmediateActions(deadlines.overdue, deadlines.dueSoon, deadlines.pendingConfirm, append(adHoc.pendingFocal, adHoc.pendingAdmin...), 10, ref)

	meta := domain.MetaBlock{
		Sources:  []string{SourceDeadlineAlerts},
		Partial:  false,
		Warnings: []string{},
	}
	if adHoc.warn != "" {
		meta.Warnings = append(meta.Warnings, adHoc.warn)
		meta.Partial = true
	}
	if inApp.warn != "" {
		meta.Warnings = append(meta.Warnings, inApp.warn)
		meta.Partial = true
	}
	if !adHoc.skipped {
		meta.Sources = append(meta.Sources, SourceAdHoc)
	}
	if len(inApp.items) > 0 || inApp.err == nil {
		meta.Sources = append(meta.Sources, SourceInApp)
	}

	return domain.OverviewResponse{
		Company:       company,
		Range:         domain.RangeInfo{From: dr.From, To: dr.To, Preset: dr.Preset, Timezone: dr.Timezone},
		LastUpdatedAt: time.Now().UTC().Format(time.RFC3339),
		Kpis:          kpis,
		DeadlineHealth: domain.DeadlineHealthBlock{
			OnTimeRate:        mapKpi(unavailableKpi("percent", ReasonCompletionTimestampOrDefinitionMissing)),
			OnTimeCount:       onTimeCount,
			OverdueAgeBuckets: buckets,
			TotalOverdue:      totalOverdue,
			Source:            SourceDeadlineAlerts,
			Accuracy:          AccuracyExact,
		},
		ImmediateActions:  immediate,
		FrequentLateFlows: buildWorkflowRiskRows(overdueItems, openAlerts, 10),
		DepartmentRisks:   buildDepartmentRiskRows(overdueItems, openAlerts, ref, 10),
		RecentActivities:  buildRecentActivities(inApp.items, 8),
		Exceptions:        buildExceptions(adHoc.rejected, inApp.items, 5),
		Meta:              meta,
	}
}

func parseYmd(dateStr string, loc *time.Location) time.Time {
	t, err := time.ParseInLocation("2006-01-02", dateStr, loc)
	if err != nil {
		return time.Time{}
	}
	return t
}

func daysBetween(from, to time.Time) int {
	a := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, from.Location())
	b := time.Date(to.Year(), to.Month(), to.Day(), 0, 0, 0, 0, to.Location())
	return int(b.Sub(a).Hours() / 24)
}

func daysLate(dueDate string, ref time.Time) int {
	if dueDate == "" {
		return 0
	}
	due := parseYmd(dueDate, ref.Location())
	if due.IsZero() {
		return 0
	}
	return int(math.Max(0, float64(daysBetween(due, ref))))
}

func daysUntilDue(dueDate string, ref time.Time) int {
	if dueDate == "" {
		return 999
	}
	due := parseYmd(dueDate, ref.Location())
	if due.IsZero() {
		return 999
	}
	return daysBetween(ref, due)
}

func severityFromOverdueCount(n int) string {
	if n >= 10 {
		return "critical"
	}
	if n >= 5 {
		return "high"
	}
	if n > 0 {
		return "warning"
	}
	return "neutral"
}

func severityFromNeedsAction(n int) string {
	if n >= 15 {
		return "critical"
	}
	if n >= 8 {
		return "high"
	}
	if n > 0 {
		return "warning"
	}
	return "neutral"
}

func severityFromDueSoonCount(n int) string {
	if n >= 20 {
		return "high"
	}
	if n > 0 {
		return "warning"
	}
	return "neutral"
}

func severityFromBlockedEstimate(n int) string {
	if n >= 5 {
		return "high"
	}
	if n > 0 {
		return "warning"
	}
	return "neutral"
}

func severityFromPendingApproval(n int) string {
	if n >= 5 {
		return "warning"
	}
	if n > 0 {
		return "neutral"
	}
	return "success"
}

func severityFromRate(rate float64) string {
	if rate >= 50 {
		return "critical"
	}
	if rate >= 30 {
		return "high"
	}
	if rate >= 15 {
		return "warning"
	}
	if rate > 0 {
		return "neutral"
	}
	return "success"
}

func displayStatus(status string) string {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case "OVERDUE":
		return "Overdue"
	case "DUE_SOON":
		return "Due Soon"
	case "PENDING_CONFIRM":
		return "Pending Confirm"
	case "UPCOMING":
		return "Upcoming"
	case "DONE":
		return "Done"
	default:
		return status
	}
}

func bucketOverdueAge(overdue []deadlinealertsapp.DeadlineAlertDTO, ref time.Time) []domain.OverdueBucket {
	counts := map[string]int{"1_3": 0, "4_7": 0, "8_14": 0, "gt_14": 0}
	for _, item := range overdue {
		late := daysLate(item.DueDate, ref)
		if late <= 0 {
			continue
		}
		switch {
		case late <= 3:
			counts["1_3"]++
		case late <= 7:
			counts["4_7"]++
		case late <= 14:
			counts["8_14"]++
		default:
			counts["gt_14"]++
		}
	}
	total := counts["1_3"] + counts["4_7"] + counts["8_14"] + counts["gt_14"]
	keys := []string{"1_3", "4_7", "8_14", "gt_14"}
	out := make([]domain.OverdueBucket, 0, 4)
	for _, key := range keys {
		pct := 0.0
		if total > 0 {
			pct = math.Round(float64(counts[key]) / float64(total) * 100)
		}
		out = append(out, domain.OverdueBucket{Key: key, Count: counts[key], Percent: pct})
	}
	return out
}

// countOnTimeOpenAlerts counts open, non-overdue deadline alerts in the selected range.
// Dedupes by RecordID across DUE_SOON / PENDING_CONFIRM / UPCOMING.
func countOnTimeOpenAlerts(groups ...[]deadlinealertsapp.DeadlineAlertDTO) int {
	seen := map[string]struct{}{}
	for _, group := range groups {
		for _, a := range group {
			id := strings.TrimSpace(a.RecordID)
			if id == "" {
				continue
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
		}
	}
	return len(seen)
}

func buildImmediateActions(
	overdue, dueSoon, pendingConfirm []deadlinealertsapp.DeadlineAlertDTO,
	adHocPending []adhocapp.ProposalDTO,
	maxItems int,
	ref time.Time,
) []domain.ImmediateActionItem {
	type sortable struct {
		item     domain.ImmediateActionItem
		priority int
	}
	var items []sortable

	addAlert := func(a deadlinealertsapp.DeadlineAlertDTO) {
		dept := ""
		if len(a.ActiveDepartments) > 0 {
			dept = a.ActiveDepartments[0]
		}
		status := displayStatus(a.Status)
		items = append(items, sortable{
			priority: immediatePriority(status, a.DueDate, false, ref),
			item: domain.ImmediateActionItem{
				ID:              a.RecordID,
				Title:           a.Title,
				Status:          status,
				Severity:        severityFromItemStatus(status, a.DueDate, ref),
				DueDate:         a.DueDate,
				Reason:          reasonForStatus(status),
				CurrentStepName: a.CurrentStepName,
				Department:      dept,
				ActionLabelKey:  actionLabelForStatus(status),
				TargetURL:       "/app/deadlines/" + a.RecordID,
				Source:          SourceDeadlineAlerts,
				Accuracy:        AccuracyExact,
			},
		})
	}

	seen := map[string]struct{}{}
	for _, a := range overdue {
		if _, ok := seen[a.RecordID]; ok {
			continue
		}
		seen[a.RecordID] = struct{}{}
		addAlert(a)
	}
	for _, a := range pendingConfirm {
		if _, ok := seen[a.RecordID]; ok {
			continue
		}
		seen[a.RecordID] = struct{}{}
		addAlert(a)
	}
	for _, a := range dueSoon {
		if _, ok := seen[a.RecordID]; ok {
			continue
		}
		seen[a.RecordID] = struct{}{}
		addAlert(a)
	}
	for _, p := range adHocPending {
		key := "adhoc:" + p.ProposalID
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		due := ""
		if p.ProposedT0Date != nil {
			due = *p.ProposedT0Date
		}
		items = append(items, sortable{
			priority: immediatePriority("pending_approval", due, true, ref),
			item: domain.ImmediateActionItem{
				ID:             p.ProposalID,
				Title:          firstLine(p.ChangeNote, p.ProposalID),
				Status:         "pending_approval",
				Severity:       "warning",
				DueDate:        due,
				Reason:         "action.reason.pendingApproval",
				ActionLabelKey: "action.viewDetails",
				TargetURL:      "/app/ad-hoc-proposals/" + p.ProposalID,
				Source:         SourceDerived,
				Accuracy:       AccuracyEstimate,
			},
		})
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].priority != items[j].priority {
			return items[i].priority < items[j].priority
		}
		return items[i].item.DueDate < items[j].item.DueDate
	})

	limit := maxItems
	if limit > len(items) {
		limit = len(items)
	}
	out := make([]domain.ImmediateActionItem, 0, limit)
	for i := 0; i < limit; i++ {
		out = append(out, items[i].item)
	}
	return out
}

func firstLine(note, fallback string) string {
	note = strings.TrimSpace(note)
	if note == "" {
		return fallback
	}
	if idx := strings.IndexByte(note, '\n'); idx >= 0 {
		return strings.TrimSpace(note[:idx])
	}
	return note
}

func immediatePriority(status, dueDate string, adHoc bool, ref time.Time) int {
	if adHoc {
		return 40
	}
	switch status {
	case "Overdue":
		if daysLate(dueDate, ref) > 7 {
			return 0
		}
		return 10
	case "Pending Confirm":
		return 20
	case "Due Soon":
		until := daysUntilDue(dueDate, ref)
		if until <= 1 {
			return 25
		}
		if until <= 7 {
			return 35
		}
		return 50
	}
	return 60
}

func severityFromItemStatus(status, dueDate string, ref time.Time) string {
	if status == "Overdue" {
		if daysLate(dueDate, ref) > 7 {
			return "critical"
		}
		return "high"
	}
	if status == "Pending Confirm" {
		return "warning"
	}
	if status == "Due Soon" {
		if daysUntilDue(dueDate, ref) <= 1 {
			return "warning"
		}
		return "neutral"
	}
	return "neutral"
}

func reasonForStatus(status string) string {
	switch status {
	case "Overdue":
		return "action.reason.overdue"
	case "Due Soon":
		return "action.reason.dueSoon"
	case "Pending Confirm":
		return "action.reason.pendingConfirm"
	default:
		return "action.reason.default"
	}
}

func actionLabelForStatus(status string) string {
	if status == "Pending Confirm" {
		return "action.viewDetails"
	}
	return "action.takeAction"
}

type wfAcc struct {
	key         string
	name        string
	openCount   int
	overdueCount int
	stepCounts  map[string]int
	deptCounts  map[string]int
}

func buildWorkflowRiskRows(overdue, open []deadlinealertsapp.DeadlineAlertDTO, maxRows int) []domain.WorkflowRiskRow {
	acc := map[string]*wfAcc{}
	touch := func(a deadlinealertsapp.DeadlineAlertDTO, isOverdue bool) {
		key := a.TypeID
		if key == "" {
			key = a.TemplateCategory
		}
		if key == "" {
			key = a.Title
		}
		row, ok := acc[key]
		if !ok {
			name := a.TemplateCategory
			if name == "" {
				name = a.Title
			}
			row = &wfAcc{key: key, name: name, stepCounts: map[string]int{}, deptCounts: map[string]int{}}
			acc[key] = row
		}
		if isOverdue {
			row.overdueCount++
		} else {
			row.openCount++
		}
		if a.CurrentStepName != "" {
			row.stepCounts[a.CurrentStepName]++
		}
		for _, d := range a.ActiveDepartments {
			if d != "" {
				row.deptCounts[d]++
			}
		}
	}
	for _, a := range overdue {
		touch(a, true)
	}
	for _, a := range open {
		touch(a, false)
	}

	rows := make([]domain.WorkflowRiskRow, 0, len(acc))
	for _, r := range acc {
		total := r.openCount + r.overdueCount
		var rate *float64
		if total > 0 {
			v := math.Round(float64(r.overdueCount)/float64(total)*1000) / 10
			rate = &v
		}
		var bottleneck, ownerDept *string
		if s := modeKey(r.stepCounts); s != "" {
			bottleneck = &s
		}
		if d := modeKey(r.deptCounts); d != "" {
			ownerDept = &d
		}
		sev := "success"
		if rate != nil {
			sev = severityFromRate(*rate)
		}
		rows = append(rows, domain.WorkflowRiskRow{
			Key:             r.key,
			WorkflowName:    r.name,
			OpenCount:       r.openCount,
			OverdueCount:    r.overdueCount,
			OverdueRate:     rate,
			AvgDelayDays:    nil,
			BottleneckStep:  bottleneck,
			OwnerDepartment: ownerDept,
			Severity:        sev,
			Source:          SourceDerived,
			Accuracy:        AccuracyEstimate,
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		ri, rj := rows[i].OverdueRate, rows[j].OverdueRate
		if ri == nil && rj == nil {
			return rows[i].OverdueCount > rows[j].OverdueCount
		}
		if ri == nil {
			return false
		}
		if rj == nil {
			return true
		}
		if *ri != *rj {
			return *ri > *rj
		}
		return rows[i].OverdueCount > rows[j].OverdueCount
	})
	if maxRows > 0 && len(rows) > maxRows {
		rows = rows[:maxRows]
	}
	return rows
}

func modeKey(m map[string]int) string {
	best, bestN := "", 0
	for k, n := range m {
		if n > bestN {
			best, bestN = k, n
		}
	}
	return best
}

type deptAcc struct {
	name          string
	totalDue      int
	overdueCount  int
	upcoming3Days int
}

func buildDepartmentRiskRows(overdue, open []deadlinealertsapp.DeadlineAlertDTO, ref time.Time, maxRows int) []domain.DepartmentRiskRow {
	acc := map[string]*deptAcc{}
	add := func(depts []string, isOverdue bool, dueDate string) {
		for _, d := range depts {
			d = strings.TrimSpace(d)
			if d == "" {
				continue
			}
			row, ok := acc[d]
			if !ok {
				row = &deptAcc{name: d}
				acc[d] = row
			}
			row.totalDue++
			if isOverdue {
				row.overdueCount++
			} else if daysUntilDue(dueDate, ref) <= 3 {
				row.upcoming3Days++
			}
		}
	}
	for _, a := range overdue {
		add(a.ActiveDepartments, true, a.DueDate)
	}
	for _, a := range open {
		add(a.ActiveDepartments, false, a.DueDate)
	}

	rows := make([]domain.DepartmentRiskRow, 0, len(acc))
	for key, r := range acc {
		var rate *float64
		if r.totalDue > 0 {
			v := math.Round(float64(r.overdueCount)/float64(r.totalDue)*1000) / 10
			rate = &v
		}
		sev := "success"
		if rate != nil {
			sev = severityFromRate(*rate)
		}
		rows = append(rows, domain.DepartmentRiskRow{
			Key:            key,
			DepartmentName: r.name,
			TotalDue:       r.totalDue,
			OverdueCount:   r.overdueCount,
			OverdueRate:    rate,
			Upcoming3Days:  r.upcoming3Days,
			OwnerName:      nil,
			Severity:       sev,
			Source:         SourceDerived,
			Accuracy:       AccuracyEstimate,
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].OverdueCount != rows[j].OverdueCount {
			return rows[i].OverdueCount > rows[j].OverdueCount
		}
		return rows[i].TotalDue > rows[j].TotalDue
	})
	if maxRows > 0 && len(rows) > maxRows {
		rows = rows[:maxRows]
	}
	return rows
}

func countEscalationNotifications(items []inappapp.InAppNotification) int {
	n := 0
	for _, it := range items {
		k := strings.ToLower(it.Kind)
		if strings.Contains(k, "escalat") || strings.Contains(k, "exception") {
			n++
		}
	}
	return n
}

func buildRecentActivities(items []inappapp.InAppNotification, limit int) []domain.RecentActivityItem {
	out := make([]domain.RecentActivityItem, 0, limit)
	for _, n := range items {
		if len(out) >= limit {
			break
		}
		url := ""
		if n.ResourceType != nil && n.ResourceID != nil {
			switch *n.ResourceType {
			case inappapp.ResourceTypeDisclosure:
				url = "/app/deadlines/" + *n.ResourceID
			case inappapp.ResourceTypeAdHocProposal:
				url = "/app/ad-hoc-proposals/" + *n.ResourceID
			}
		}
		out = append(out, domain.RecentActivityItem{
			ID:         n.ID,
			Kind:       n.Kind,
			Title:      n.Title,
			Summary:    n.Body,
			OccurredAt: n.CreatedAt.UTC().Format(time.RFC3339),
			TargetURL:  url,
			Source:     SourceInApp,
			Accuracy:   AccuracyEstimate,
		})
	}
	return out
}

func buildExceptions(rejected []adhocapp.ProposalDTO, notifications []inappapp.InAppNotification, limit int) []domain.ExceptionItem {
	out := make([]domain.ExceptionItem, 0, limit)
	for _, p := range rejected {
		if len(out) >= limit {
			break
		}
		at := p.UpdatedAt.UTC().Format(time.RFC3339)
		if p.RejectedAt != nil {
			at = p.RejectedAt.UTC().Format(time.RFC3339)
		}
		out = append(out, domain.ExceptionItem{
			ID:         p.ProposalID,
			Kind:       "adhoc.rejected",
			Title:      firstLine(p.ChangeNote, p.ProposalID),
			Summary:    p.RejectReason,
			Severity:   "warning",
			OccurredAt: at,
			TargetURL:  "/app/ad-hoc-proposals/" + p.ProposalID,
			Source:     SourceAdHoc,
			Accuracy:   AccuracyEstimate,
		})
	}
	for _, n := range notifications {
		if len(out) >= limit {
			break
		}
		k := strings.ToLower(n.Kind)
		if !strings.Contains(k, "escalat") && !strings.Contains(k, "reject") && !strings.Contains(k, "exception") {
			continue
		}
		url := ""
		if n.ResourceType != nil && n.ResourceID != nil && *n.ResourceType == inappapp.ResourceTypeAdHocProposal {
			url = "/app/ad-hoc-proposals/" + *n.ResourceID
		}
		out = append(out, domain.ExceptionItem{
			ID:         n.ID,
			Kind:       n.Kind,
			Title:      n.Title,
			Summary:    n.Body,
			Severity:   "high",
			OccurredAt: n.CreatedAt.UTC().Format(time.RFC3339),
			TargetURL:  url,
			Source:     SourceInApp,
			Accuracy:   AccuracyEstimate,
		})
	}
	return out
}
