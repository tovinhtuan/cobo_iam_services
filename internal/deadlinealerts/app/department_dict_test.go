package app

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
)

func TestLooksLikeTechnicalDepartmentRef(t *testing.T) {
	cases := map[string]bool{
		"d1":                                true,
		"dept-004":                          true,
		"d_legal":                           true,
		"ou_dept_legal":                     true,
		"019f4e70-cbf9-7122-a150-c32718065b00": true,
		"Ban Tổng Giám đốc":                 false,
		"Phòng CBTT":                        false,
	}
	for in, want := range cases {
		if got := LooksLikeTechnicalDepartmentRef(in); got != want {
			t.Fatalf("%q: got %v want %v", in, got, want)
		}
	}
}

func TestResolveActiveDepartmentLabels_mapsTemplateCode(t *testing.T) {
	dict := NewDepartmentDict(
		[]DeadlineAlertFilterOptionDTO{{ID: "uuid-ceo", Code: "ceo", Name: "Ban Tổng Giám đốc"}},
		[]DeadlineAlertFilterOptionDTO{{ID: "dept-004", Code: "dept-004", Name: "Ban Tổng Giám đốc"}},
	)
	got := ResolveActiveDepartmentLabels([]string{"dept-004", "d1"}, dict)
	if len(got) != 1 || got[0] != "Ban Tổng Giám đốc" {
		t.Fatalf("got %+v", got)
	}
}

func TestFilterAliases_companyUUIDMatchesTemplateCode(t *testing.T) {
	dict := NewDepartmentDict(
		[]DeadlineAlertFilterOptionDTO{{ID: "uuid-ceo", Code: "ceo", Name: "Ban Tổng Giám đốc"}},
		[]DeadlineAlertFilterOptionDTO{{ID: "dept-004", Code: "dept-004", Name: "Ban Tổng Giám đốc"}},
	)
	aliases := dict.FilterAliases("uuid-ceo")
	found := false
	for _, a := range aliases {
		if a == "dept-004" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("aliases missing dept-004: %+v", aliases)
	}
}

func TestListDeadlineAlerts_resolvesDepartmentNameAndFiltersByAlias(t *testing.T) {
	repo := &stubRepo{
		rows: []AlertRow{
			{CompanyID: "c1", RecordID: "r-ceo", Title: "CEO alert", RecordStatus: "submitted", PlannedDate: "2026-06-10", CurrentStepDepartment: "dept-004"},
			{CompanyID: "c1", RecordID: "r-other", Title: "Other", RecordStatus: "submitted", PlannedDate: "2026-06-11", CurrentStepDepartment: "dept-001"},
		},
		departments: []DeadlineAlertFilterOptionDTO{
			{ID: "uuid-ceo", Code: "ceo", Name: "Ban Tổng Giám đốc"},
			{ID: "uuid-legal", Code: "legal", Name: "Phòng Pháp chế"},
		},
		templateDepts: []DeadlineAlertFilterOptionDTO{
			{ID: "dept-004", Code: "dept-004", Name: "Ban Tổng Giám đốc"},
			{ID: "dept-001", Code: "dept-001", Name: "Phòng Pháp chế"},
		},
	}
	svc := NewService(repo, allowAuthSvc(), disclosureapp.NewDeadlineCalculator(disclosureapp.NewHolidayCalendarFileProvider("configs/non_trading_days")))
	svc.(*service).now = func() time.Time { return time.Date(2026, 5, 25, 0, 0, 0, 0, time.UTC) }

	all, err := svc.ListDeadlineAlerts(context.Background(), ListDeadlineAlertsRequest{
		Subject: Subject{UserID: "u1", MembershipID: "m_admin_001", CompanyID: "c_001"},
		Page:    1, PageSize: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if all.Total != 2 {
		t.Fatalf("total=%d", all.Total)
	}
	if len(all.Items[0].ActiveDepartments) != 1 || all.Items[0].ActiveDepartments[0] != "Ban Tổng Giám đốc" {
		t.Fatalf("active departments %+v", all.Items[0].ActiveDepartments)
	}

	filtered, err := svc.ListDeadlineAlerts(context.Background(), ListDeadlineAlertsRequest{
		Subject:      Subject{UserID: "u1", MembershipID: "m_admin_001", CompanyID: "c_001"},
		DepartmentID: "uuid-ceo",
		Page:         1, PageSize: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if filtered.Total != 1 || filtered.Items[0].RecordID != "r-ceo" {
		t.Fatalf("filtered %+v", filtered)
	}
	if filtered.Items == nil {
		t.Fatal("items must not be nil")
	}

	empty, err := svc.ListDeadlineAlerts(context.Background(), ListDeadlineAlertsRequest{
		Subject:      Subject{UserID: "u1", MembershipID: "m_admin_001", CompanyID: "c_001"},
		DepartmentID: "uuid-missing",
		Page:         1, PageSize: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if empty.Total != 0 {
		t.Fatalf("expected 0 got %d", empty.Total)
	}
	if empty.Items == nil {
		t.Fatal("empty items must be non-nil slice")
	}
	if len(empty.Items) != 0 {
		t.Fatalf("expected empty slice got %+v", empty.Items)
	}
}

func TestMergeDepartmentFilterOptions_preservesVietnameseUTF8(t *testing.T) {
	names := []string{
		"Ban Tổng Giám đốc",
		"Phòng Pháp chế",
		"Tài chính",
		"Kiểm soát nội bộ",
		"Thư ký công ty",
	}
	company := make([]DeadlineAlertFilterOptionDTO, 0, len(names))
	for i, name := range names {
		company = append(company, DeadlineAlertFilterOptionDTO{
			ID:   "uuid-" + strconv.Itoa(i+1),
			Code: "c" + strconv.Itoa(i+1),
			Name: name,
		})
	}
	merged := MergeDepartmentFilterOptions(company, nil)
	raw, err := json.Marshal(merged)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, name := range names {
		if !strings.Contains(text, name) {
			t.Fatalf("JSON missing Vietnamese name %q in %s", name, text)
		}
	}
	for _, bad := range []string{"TÆ", "Ä‘", "Tá»", "GiÃ", "PhÃ"} {
		if strings.Contains(text, bad) {
			t.Fatalf("JSON contains mojibake marker %q in %s", bad, text)
		}
	}
}

func TestNewDepartmentDict_companyNameWinsOverTemplateMojibake(t *testing.T) {
	dict := NewDepartmentDict(
		[]DeadlineAlertFilterOptionDTO{{ID: "uuid-ceo", Code: "dept-004", Name: "Ban Tổng Giám đốc"}},
		[]DeadlineAlertFilterOptionDTO{{ID: "dept-004", Code: "dept-004", Name: "Ban Tá»•ng GiÃ¡m Ä‘á»‘c"}},
	)
	if got := dict.ResolveLabel("dept-004"); got != "Ban Tổng Giám đốc" {
		t.Fatalf("got %q, want company UTF-8 name", got)
	}
}
