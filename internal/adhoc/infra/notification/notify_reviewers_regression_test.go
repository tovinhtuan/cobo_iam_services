package notification

import (
	"os"
	"strings"
	"testing"
)

func TestNotifyReviewersForReview_SourceUnchanged(t *testing.T) {
	data, err := os.ReadFile("notifier.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	fnStart := strings.Index(src, "func (n *AdhocProposalNotifier) NotifyReviewersForReview")
	if fnStart < 0 {
		t.Fatal("NotifyReviewersForReview must remain")
	}
	fn := src[fnStart:]
	if end := strings.Index(fn[1:], "\nfunc "); end > 0 {
		fn = fn[:end+1]
	}
	for _, must := range []string{
		"KindAdhocReviewerReviewRequested",
		"adhoc.focal_review_requested",
		"reviewer_review_requested",
	} {
		if !strings.Contains(fn, must) {
			t.Fatalf("NotifyReviewersForReview must still contain %s", must)
		}
	}
	if strings.Contains(fn, "due_minus_") {
		t.Fatal("NotifyReviewersForReview must not be coupled to workflow step due-minus reminders")
	}
}
