package dashboard

import (
	"time"

	"github.com/cobo/cobo_iam_services/internal/companyaccess/validation"
)

// HealthInput is configuration-health output for aggregation.
type HealthInput struct {
	View *HealthView
	Err  error
}

// ValidationInput is validation suite output for aggregation.
type ValidationInput struct {
	View *validation.Result
	Err  error
}

// NotificationInput is notification status output for aggregation.
type NotificationInput struct {
	View *NotificationView
	Err  error
}

// AuditTimelineInput is change-timeline summary for dashboard widget.
type AuditTimelineInput struct {
	LastSummary string
	EventCount  int
	Err         error
}

// AggregateInput bundles read-only source outputs (no detection logic).
type AggregateInput struct {
	CompanyID   string
	EvaluatedAt time.Time
	Health      HealthInput
	Validation  ValidationInput
	Notification NotificationInput
	AuditTimeline AuditTimelineInput
}

// Build composes widgets from existing read-only sources only.
func Build(in AggregateInput) Result {
	evalAt := in.EvaluatedAt
	if evalAt.IsZero() {
		evalAt = time.Now().UTC()
	}
	widgets := []Widget{
		buildHealthWidget(in.Health),
		buildValidationWidget(in.Validation),
		buildNotificationWidget(in.Notification),
		buildSubscriptionWidget(in.Notification),
		buildAuditWidget(in.AuditTimeline),
		buildDependencyWidget(),
	}
	return Result{
		OverallStatus: rollupOverall(widgets),
		CompanyID:     in.CompanyID,
		Widgets:       widgets,
		EvaluatedAt:   evalAt,
	}
}

func buildHealthWidget(in HealthInput) Widget {
	w := Widget{
		Key:          WidgetConfigurationHealth,
		Title:        "Sức khỏe cấu hình",
		ActionLink:   "/app/admin?tab=notifications",
		Availability: AvailabilityOK,
		Status:       StatusUnknown,
		Summary:      "Chưa tải được dữ liệu sức khỏe cấu hình.",
	}
	if in.Err != nil {
		w.Summary = "Không đọc được configuration-health."
		return w
	}
	if in.View == nil {
		return w
	}
	w.Status = mapHealthOverall(in.View.OverallStatus)
	blocking, warning, info := countHealthSeverities(in.View.Checks)
	w.Summary = healthSummary(blocking, warning, info)
	metrics := []Metric{
		{Key: "overall_status", Label: "Trạng thái", Value: in.View.OverallStatus},
		{Key: "check_count", Label: "Số kiểm tra", Value: len(in.View.Checks)},
		{Key: "blocking", Label: "Blocking", Value: blocking},
		{Key: "warning", Label: "Warning", Value: warning},
		{Key: "info", Label: "Info", Value: info},
	}
	if in.View.ScoreValue != nil {
		metrics = append(metrics, Metric{
			Key:   "health_score",
			Label: "Điểm sức khỏe",
			Value: *in.View.ScoreValue,
		})
		if in.View.ScoreStatus != "" {
			metrics = append(metrics, Metric{
				Key:   "health_score_status",
				Label: "Mức điểm",
				Value: in.View.ScoreStatus,
			})
		}
	}
	w.Metrics = metrics
	return w
}

func buildValidationWidget(in ValidationInput) Widget {
	w := Widget{
		Key:          WidgetValidationSummary,
		Title:        "Kiểm tra cấu hình",
		ActionLink:   "/app/admin?tab=notifications",
		Availability: AvailabilityOK,
		Status:       StatusUnknown,
		Summary:      "Chưa chạy kiểm tra validation.",
	}
	if in.Err != nil {
		w.Summary = "Không đọc được validation suite."
		return w
	}
	if in.View == nil {
		return w
	}
	w.Status = mapValidationPassed(in.View.Passed, in.View.Summary.Blocking, in.View.Summary.Warning)
	if in.View.Passed {
		w.Summary = "Validation passed — không có blocking."
	} else if in.View.Summary.Blocking > 0 {
		w.Summary = "Validation có blocking — cần xem chi tiết."
	} else {
		w.Summary = "Validation có cảnh báo — xem chi tiết."
	}
	w.Metrics = []Metric{
		{Key: "passed", Label: "Passed", Value: in.View.Passed},
		{Key: "blocking", Label: "Blocking", Value: in.View.Summary.Blocking},
		{Key: "warning", Label: "Warning", Value: in.View.Summary.Warning},
		{Key: "total_checks", Label: "Tổng checks", Value: in.View.Summary.Total},
	}
	return w
}

func buildNotificationWidget(in NotificationInput) Widget {
	w := Widget{
		Key:          WidgetNotificationRuntime,
		Title:        "Notification runtime",
		ActionLink:   "/app/admin?tab=notifications",
		Availability: AvailabilityOK,
		Status:       StatusUnknown,
		Summary:      "Chưa tải trạng thái notification.",
	}
	if in.Err != nil {
		w.Summary = "Không đọc được notification status."
		return w
	}
	if in.View == nil {
		return w
	}
	v := in.View
	w.Status = mapNotificationUIState(v.UIState, v.PayloadValid, len(v.Warnings))
	w.Summary = notificationSummary(v)
	w.Metrics = []Metric{
		{Key: "storage_configured", Label: "Đã lưu prefs", Value: v.StorageConfigured},
		{Key: "payload_valid", Label: "Payload hợp lệ", Value: v.PayloadValid},
		{Key: "runtime_consumer_enabled", Label: "Runtime consumer", Value: v.RuntimeConsumerEnabled},
		{Key: "simulation_available", Label: "Simulation", Value: v.SimulationAvailable},
		{Key: "ui_state", Label: "UI state", Value: v.UIState},
	}
	return w
}

func buildSubscriptionWidget(in NotificationInput) Widget {
	w := Widget{
		Key:          WidgetSubscriptionTier,
		Title:        "Subscription tier enforcement",
		ActionLink:   "/app/admin?tab=notifications",
		Availability: AvailabilityOK,
		Status:       StatusUnknown,
		Summary:      "Chưa tải trạng thái tier.",
	}
	if in.Err != nil || in.View == nil {
		if in.Err != nil {
			w.Summary = "Không đọc được tier status."
		}
		return w
	}
	enforced := in.View.SubscriptionTierEnforced
	w.Metrics = []Metric{
		{Key: "subscription_tier_enforced", Label: "Enforcement bật", Value: enforced},
		{Key: "dispatch_enforcement_enabled", Label: "Dispatch enforcement", Value: in.View.DispatchEnforcementEnabled},
	}
	if enforced {
		w.Status = StatusOK
		w.Summary = "Subscription tier enforcement đang bật trên server."
	} else {
		w.Status = StatusWarning
		w.Summary = "Subscription tier enforcement chưa bật — chỉ guard phía client."
	}
	return w
}

func buildAuditWidget(in AuditTimelineInput) Widget {
	w := Widget{
		Key:          WidgetAuditTimeline,
		Title:        "Nhật ký thay đổi",
		Status:       StatusUnknown,
		Summary:      "Chưa có sự kiện thay đổi gần đây.",
		Availability: AvailabilityOK,
		ActionLink:   "/app/admin/audit",
	}
	if in.Err != nil {
		w.Summary = "Không tải được timeline thay đổi."
		return w
	}
	if in.EventCount == 0 && in.LastSummary == "" {
		w.Status = StatusOK
		return w
	}
	w.Status = StatusOK
	if in.LastSummary != "" {
		w.Summary = in.LastSummary
	} else if in.EventCount > 0 {
		w.Summary = "Có sự kiện thay đổi cấu hình gần đây."
	}
	if in.EventCount > 0 {
		w.Metrics = []Metric{
			{Key: "recent_events", Label: "Sự kiện (trang đầu)", Value: in.EventCount},
		}
	}
	return w
}

func buildDependencyWidget() Widget {
	return Widget{
		Key:          WidgetDependencySummary,
		Title:        "Phụ thuộc cấu hình",
		Status:       StatusUnknown,
		Summary:      "Xem phụ thuộc theo từng department/role khi xóa.",
		Availability: AvailabilityNA,
		ActionLink:   "/app/admin?tab=org",
	}
}

func mapHealthOverall(overall string) string {
	switch overall {
	case "critical":
		return StatusAttention
	case "warning":
		return StatusWarning
	case "ok":
		return StatusOK
	default:
		return StatusUnknown
	}
}

func mapValidationPassed(passed bool, blocking, warning int) string {
	if blocking > 0 {
		return StatusAttention
	}
	if !passed || warning > 0 {
		return StatusWarning
	}
	return StatusOK
}

func mapNotificationUIState(uiState string, payloadValid bool, warningCount int) string {
	switch uiState {
	case "misconfigured":
		return StatusAttention
	case "not_configured":
		return StatusWarning
	}
	if !payloadValid {
		return StatusAttention
	}
	if warningCount > 0 {
		return StatusWarning
	}
	return StatusOK
}

func countHealthSeverities(checks []HealthCheck) (blocking, warning, info int) {
	for _, c := range checks {
		switch c.Severity {
		case "blocking":
			blocking++
		case "warning":
			warning++
		case "info":
			info++
		}
	}
	return blocking, warning, info
}

func healthSummary(blocking, warning, info int) string {
	if blocking > 0 {
		return "Có vấn đề blocking trong configuration-health."
	}
	if warning > 0 {
		return "Có cảnh báo configuration-health."
	}
	if info > 0 {
		return "Configuration-health ổn — có thông tin bổ sung."
	}
	return "Configuration-health ổn."
}

func notificationSummary(v *NotificationView) string {
	if !v.StorageConfigured {
		return "Chưa lưu cấu hình kênh cảnh báo."
	}
	if !v.PayloadValid {
		return "Prefs đã lưu nhưng payload chưa hợp lệ."
	}
	if !v.RuntimeConsumerEnabled {
		return "Prefs hợp lệ — runtime consumer chưa bật."
	}
	return "Notification storage đã cấu hình."
}

func rollupOverall(widgets []Widget) string {
	worst := StatusOK
	for _, w := range widgets {
		if w.Availability == AvailabilityNA {
			continue
		}
		worst = maxStatus(worst, w.Status)
	}
	return worst
}

func maxStatus(a, b string) string {
	rank := map[string]int{
		StatusOK: 0, StatusWarning: 1, StatusAttention: 2, StatusUnknown: 0,
	}
	if rank[b] > rank[a] {
		return b
	}
	return a
}
