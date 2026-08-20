package mysql

import (
	"strings"
	"testing"
)

// TestUpsertTypeVersion_SaveDraftNeverAutoActivates ensures Save Draft does not move
// active_version_no — only explicit ActivateTypeVersion may publish to Portal.
func TestUpsertTypeVersion_SaveDraftNeverAutoActivates(t *testing.T) {
	src := readRepositorySrc(t)
	fn := extractFunc(t, src, "func (r *Repository) UpsertTypeVersion")

	if strings.Contains(fn, "nextIsActive := !typeExists") || strings.Contains(fn, "activeVersionNo <= 0") {
		t.Fatal("UpsertTypeVersion must not auto-activate when active_version_no is 0")
	}
	if !strings.Contains(fn, "nextIsActive := false") {
		t.Fatal("UpsertTypeVersion must set nextIsActive := false (Save Draft never moves active pointer)")
	}
	if !strings.Contains(fn, "Save Draft never moves the portal active pointer") {
		t.Fatal("UpsertTypeVersion must document Save Draft vs Activate separation")
	}
}
