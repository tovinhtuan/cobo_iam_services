package dashboard_test

import (
	"testing"
	"time"

	"github.com/cobo/cobo_iam_services/internal/companyaccess/dashboard"
	"github.com/cobo/cobo_iam_services/internal/companyaccess/validation"
)

func TestBuildMapsHealthWidget(t *testing.T) {
	result := dashboard.Build(dashboard.AggregateInput{
		CompanyID:   "co-1",
		EvaluatedAt: time.Unix(0, 0).UTC(),
		Health: dashboard.HealthInput{
			View: &dashboard.HealthView{
				OverallStatus: "warning",
				Checks: []dashboard.HealthCheck{
					{Severity: "warning"},
				},
			},
		},
		Validation: dashboard.ValidationInput{
			View: &validation.Result{Passed: true, Summary: validation.Summary{}},
		},
		Notification: dashboard.NotificationInput{
			View: &dashboard.NotificationView{UIState: "storage_configured", StorageConfigured: true, PayloadValid: true},
		},
	})
	if result.OverallStatus != dashboard.StatusWarning {
		t.Fatalf("overall: %q", result.OverallStatus)
	}
	var health *dashboard.Widget
	for i := range result.Widgets {
		if result.Widgets[i].Key == dashboard.WidgetConfigurationHealth {
			health = &result.Widgets[i]
			break
		}
	}
	if health == nil || health.Status != dashboard.StatusWarning {
		t.Fatalf("health widget: %+v", health)
	}
}

func TestBuildValidationBlockingAttention(t *testing.T) {
	result := dashboard.Build(dashboard.AggregateInput{
		CompanyID: "co-1",
		Health: dashboard.HealthInput{
			View: &dashboard.HealthView{OverallStatus: "ok"},
		},
		Validation: dashboard.ValidationInput{
			View: &validation.Result{
				Passed: false,
				Summary: validation.Summary{Blocking: 1, Warning: 2, Total: 3},
			},
		},
	})
	if result.OverallStatus != dashboard.StatusAttention {
		t.Fatalf("overall: %q", result.OverallStatus)
	}
}

func TestBuildAuditWidgetEmpty(t *testing.T) {
	result := dashboard.Build(dashboard.AggregateInput{CompanyID: "co-1"})
	var audit *dashboard.Widget
	for i := range result.Widgets {
		if result.Widgets[i].Key == dashboard.WidgetAuditTimeline {
			audit = &result.Widgets[i]
		}
	}
	if audit == nil || audit.Availability != dashboard.AvailabilityOK {
		t.Fatalf("audit widget: %+v", audit)
	}
}

func TestBuildDoesNotDuplicateDetection(t *testing.T) {
	// Aggregator only maps pre-built views; no engine invocation in package.
	result := dashboard.Build(dashboard.AggregateInput{})
	if len(result.Widgets) < 4 {
		t.Fatalf("expected widgets, got %d", len(result.Widgets))
	}
}
