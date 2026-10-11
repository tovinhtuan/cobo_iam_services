package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	platformcmsapp "github.com/cobo/cobo_iam_services/internal/platformcms/app"
)

type stubDashboardRepo struct{}

func (stubDashboardRepo) CompanyStats(context.Context, time.Time) (platformcmsapp.DashboardCompanies, error) {
	return platformcmsapp.DashboardCompanies{Total: 3, Active: 2, Unverified: 1}, nil
}

func (stubDashboardRepo) SubscriptionRows(context.Context, time.Time) ([]platformcmsapp.DashboardSubscriptionRow, error) {
	return nil, nil
}

func (stubDashboardRepo) UserStats(context.Context, time.Time) (platformcmsapp.DashboardUsers, error) {
	return platformcmsapp.DashboardUsers{ActiveUsers: 5}, nil
}

func (stubDashboardRepo) TemplateStats(context.Context, time.Time) (platformcmsapp.DashboardTemplates, error) {
	return platformcmsapp.DashboardTemplates{Active: 4}, nil
}

func (stubDashboardRepo) DisclosureStats(context.Context, string, time.Time) (platformcmsapp.DashboardDisclosures, error) {
	return platformcmsapp.DashboardDisclosures{Overdue: 2, OverdueCompanies: 1}, nil
}

func (stubDashboardRepo) SystemHealthStats(context.Context, time.Time, int) (platformcmsapp.DashboardSystemHealth, error) {
	return platformcmsapp.DashboardSystemHealth{HolidayCalendar: platformcmsapp.DashboardHolidayCalendar{CurrentYear: true}}, nil
}

func (stubDashboardRepo) GrowthCounts(context.Context, string, time.Time) (platformcmsapp.DashboardGrowthCounts, error) {
	return platformcmsapp.DashboardGrowthCounts{}, nil
}

func newDashboardTestHandler(perms []string, withSvc bool) *Handler {
	h := &Handler{
		inspector:  listedFakeInspector{},
		authorizer: listedFakeAuthorizer{perms: perms},
		metrics:    newCMSMetrics(),
	}
	if withSvc {
		h.WithDashboardOverviewService(platformcmsapp.NewDashboardOverviewService(stubDashboardRepo{}))
	}
	return h
}

func getDashboardOverview(h *Handler, query string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/platform/cms/dashboard/overview"+query, nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	mux := http.NewServeMux()
	h.Register(mux)
	mux.ServeHTTP(w, req)
	return w
}

func TestDashboardOverview_RequiresPlatformCMSView(t *testing.T) {
	// Enterprise admin permissions without platform.cms.view.
	h := newDashboardTestHandler([]string{"rbac.manage", "disclosure.view"}, true)
	w := getDashboardOverview(h, "")
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body %s", w.Code, w.Body.String())
	}
}

func TestDashboardOverview_NotConfiguredReturns503(t *testing.T) {
	h := newDashboardTestHandler([]string{"platform.cms.view"}, false)
	w := getDashboardOverview(h, "")
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", w.Code)
	}
}

func TestDashboardOverview_EnvelopeAndSectionGates(t *testing.T) {
	h := newDashboardTestHandler([]string{"platform.cms.view", "cms.template.read"}, true)
	w := getDashboardOverview(h, "?refresh=true")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", w.Code, w.Body.String())
	}
	var body struct {
		Data struct {
			GeneratedAt string                     `json:"generated_at"`
			Timezone    string                     `json:"timezone"`
			Sections    map[string]json.RawMessage `json:"sections"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Data.GeneratedAt == "" || body.Data.Timezone != "Asia/Ho_Chi_Minh" {
		t.Fatalf("meta fields missing: %+v", body.Data)
	}
	status := func(key string) (string, bool) {
		var s struct {
			Status string          `json:"status"`
			Data   json.RawMessage `json:"data"`
		}
		_ = json.Unmarshal(body.Data.Sections[key], &s)
		return s.Status, len(s.Data) > 0
	}
	for key, want := range map[string]string{
		"companies":     "forbidden",
		"subscriptions": "forbidden",
		"users":         "forbidden",
		"system_health": "forbidden",
		"growth":        "forbidden",
		"templates":     "ok",
		"disclosures":   "ok",
		"attention":     "ok",
	} {
		got, hasData := status(key)
		if got != want {
			t.Errorf("%s status = %q, want %q", key, got, want)
		}
		if want == "forbidden" && hasData {
			t.Errorf("%s must not include data when forbidden", key)
		}
	}
	var attention struct {
		Items []struct {
			Code  string `json:"code"`
			Count int    `json:"count"`
		} `json:"items"`
	}
	_ = json.Unmarshal(body.Data.Sections["attention"], &attention)
	if len(attention.Items) != 1 || attention.Items[0].Code != "disclosures_overdue" || attention.Items[0].Count != 2 {
		t.Fatalf("attention = %+v, want only disclosures_overdue=2 (companies forbidden)", attention.Items)
	}
}
