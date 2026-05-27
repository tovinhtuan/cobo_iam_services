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
