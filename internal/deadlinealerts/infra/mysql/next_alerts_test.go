package mysql

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	deadlinealertsapp "github.com/cobo/cobo_iam_services/internal/deadlinealerts/app"
)

func TestListNextAlertCycles_sqlContract(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	repoPath := filepath.Join(filepath.Dir(thisFile), "repository.go")
	src, err := os.ReadFile(repoPath)
	if err != nil {
		t.Fatal(err)
	}
	body := string(src)
	idx := strings.Index(body, "func (r *Repository) ListNextAlertCycles")
	if idx < 0 {
		t.Fatal("ListNextAlertCycles missing")
	}
	chunk := body[idx:]
	if end := strings.Index(chunk, "\nfunc ("); end > 0 {
		chunk = chunk[:end]
	}
	want := deadlinealertsapp.MaterializerEffectiveOpenAtSQL
	if !strings.Contains(chunk, want+" > ?") {
		t.Fatalf("must filter TodayHCM < %s", want)
	}
	if !strings.Contains(chunk, "record_id IS NULL") {
		t.Fatal("must exclude materialized cycles via record_id")
	}
	if strings.Contains(chunk, "COALESCE(pc.open_at, pc.cycle_start, pc.due_date)") {
		t.Fatal("due_date must not participate in Next Alert OpenAt boundary")
	}
	lower := strings.ToLower(chunk)
	if strings.Contains(lower, "insert ") || strings.Contains(lower, "update ") || strings.Contains(lower, "delete ") {
		t.Fatal("projection must be read-only SQL")
	}
}

func TestListNextAlertCycles_openAtAuthorityMatchesMaterializer(t *testing.T) {
	next := deadlinealertsapp.MaterializerEffectiveOpenAtSQL
	if next != "COALESCE(pc.open_at, pc.cycle_start)" {
		t.Fatalf("got %q", next)
	}
}
