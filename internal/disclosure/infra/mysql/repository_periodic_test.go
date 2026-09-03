package mysql

import (
	"os"
	"strings"
	"testing"
)

func TestListActivePeriodicTypesSQLUsesActiveVersionNo(t *testing.T) {
	t.Helper()
	data, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatalf("read repository.go: %v", err)
	}
	src := string(data)
	if strings.Contains(src, "dtv.is_active") {
		t.Fatal("periodic type listing must not reference dtv.is_active (use dt.active_version_no)")
	}
	if !strings.Contains(src, "dtv.version_no = dt.active_version_no") {
		t.Fatal("expected join on disclosure_types.active_version_no")
	}
}

func TestListActivePeriodicTypesSQLIncludesDailyWeekly(t *testing.T) {
	data, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatalf("read repository.go: %v", err)
	}
	src := string(data)
	if !strings.Contains(src, "'daily'") || !strings.Contains(src, "'weekly'") {
		t.Fatal("periodic type listing must include daily and weekly frequency_unit values")
	}
}

func TestListActivePeriodicTypesSQLIncludesApplicableFrom(t *testing.T) {
	data, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatalf("read repository.go: %v", err)
	}
	src := string(data)
	if !strings.Contains(src, "applicable_from_mode") || !strings.Contains(src, "applicable_from_slot") {
		t.Fatal("periodic type listing must extract applicable_from_mode/slot from ACTIVE deadline_config_json")
	}
	if !strings.Contains(src, "dtv.version_no = dt.active_version_no") {
		t.Fatal("applicable_from must come from active version join")
	}
}

func TestListActivePeriodicTypesSQLIncludesApplicableTo(t *testing.T) {
	data, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatalf("read repository.go: %v", err)
	}
	src := string(data)
	if !strings.Contains(src, "applicable_to") {
		t.Fatal("periodic type listing must extract applicable_to from ACTIVE deadline_config_json")
	}
	if !strings.Contains(src, "dtv.version_no = dt.active_version_no") {
		t.Fatal("applicable_to must come from active version join")
	}
}
