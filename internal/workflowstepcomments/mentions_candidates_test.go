package workflowstepcomments_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	wff "github.com/cobo/cobo_iam_services/internal/workflowfulfillment"
	wsc "github.com/cobo/cobo_iam_services/internal/workflowstepcomments"
)

func TestEscapeLikePattern(t *testing.T) {
	got := wsc.EscapeLikePattern(`a%b_c\d`)
	want := `a\%b\_c\\d`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestListMentionCandidates_FlagOff(t *testing.T) {
	repo := wsc.NewMemoryRepository()
	d := &fakeCommentDeadline{viewOK: true, commentOK: true, wf: baseWF("Draft"), states: map[string]wff.StepState{}}
	svc := wsc.NewService(d, repo, nil) // mentions disabled
	_, err := svc.ListMentionCandidates(context.Background(), wff.Subject{CompanyID: "c1"}, "r1", "s1", "ab", 1, 20)
	if err == nil {
		t.Fatal("expected FEATURE_DISABLED")
	}
	he, ok := err.(*perr.HTTPError)
	if !ok || he.HTTPStatus != http.StatusNotFound || he.Code != perr.CodeFeatureDisabled {
		t.Fatalf("got %#v", err)
	}
}

func TestListMentionCandidates_AuthzAndQuery(t *testing.T) {
	repo := wsc.NewMemoryRepository()
	store := &wsc.MemoryMentionCandidateStore{
		ByCompany: map[string][]wsc.MentionCandidate{
			"c1": {
				{MembershipID: "m1", DisplayName: "Nguyen Van A"},
				{MembershipID: "m2", DisplayName: "Tran Van B"},
			},
			"c2": {
				{MembershipID: "m9", DisplayName: "Other Tenant"},
			},
		},
	}
	completed := time.Date(2026, 5, 30, 0, 0, 0, 0, time.UTC)
	d := &fakeCommentDeadline{
		viewOK: true, commentOK: true,
		wf:     baseWF("In Progress"),
		states: map[string]wff.StepState{"s1": {StepCode: "s1", CompletedAt: &completed}},
	}
	svc := wsc.NewService(d, repo, nil).WithMentionsEnabled(true).WithMentionCandidates(store)
	sub := wff.Subject{UserID: "u1", MembershipID: "m1", CompanyID: "c1"}

	_, err := svc.ListMentionCandidates(context.Background(), sub, "r1", "s1", "a", 1, 20)
	if err == nil {
		t.Fatal("expected q too short")
	}

	_, err = svc.ListMentionCandidates(context.Background(), sub, "r1", "s1", "", 1, 20)
	if err == nil {
		t.Fatal("expected empty q reject")
	}

	long := make([]rune, 101)
	for i := range long {
		long[i] = 'a'
	}
	_, err = svc.ListMentionCandidates(context.Background(), sub, "r1", "s1", string(long), 1, 20)
	if err == nil {
		t.Fatal("expected q too long")
	}

	_, err = svc.ListMentionCandidates(context.Background(), sub, "r1", "s1", "ng", 0, 20)
	if err == nil {
		t.Fatal("expected invalid page")
	}

	_, err = svc.ListMentionCandidates(context.Background(), sub, "r1", "s1", "ng", 1, 51)
	if err == nil {
		t.Fatal("expected page_size cap reject")
	}

	d.viewOK = false
	_, err = svc.ListMentionCandidates(context.Background(), sub, "r1", "s1", "ng", 1, 20)
	if err == nil {
		t.Fatal("expected forbidden")
	}
	d.viewOK = true

	resp, err := svc.ListMentionCandidates(context.Background(), sub, "r1", "s1", "ng", 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Total != 1 || len(resp.Items) != 1 || resp.Items[0].MembershipID != "m1" {
		t.Fatalf("got %+v", resp)
	}
	if resp.Items[0].DisplayName != "Nguyen Van A" {
		t.Fatalf("display_name=%q", resp.Items[0].DisplayName)
	}

	// cross-company store isolation
	sub2 := wff.Subject{UserID: "u2", MembershipID: "m9", CompanyID: "c2"}
	d.company = "c2"
	d.wf.CompanyID = "c2"
	resp2, err := svc.ListMentionCandidates(context.Background(), sub2, "r1", "s1", "ot", 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if resp2.Total != 1 || resp2.Items[0].MembershipID != "m9" {
		t.Fatalf("tenant2 got %+v", resp2)
	}
}

func TestListComments_CanMentionCapability(t *testing.T) {
	repo := wsc.NewMemoryRepository()
	completed := time.Date(2026, 5, 30, 0, 0, 0, 0, time.UTC)
	d := &fakeCommentDeadline{
		viewOK: true, commentOK: true,
		wf:     baseWF("Draft"),
		states: map[string]wff.StepState{"s1": {StepCode: "s1", CompletedAt: &completed}},
	}
	sub := wff.Subject{UserID: "u1", MembershipID: "m1", CompanyID: "c1"}

	svcOff := wsc.NewService(d, repo, nil)
	resp, err := svcOff.ListComments(context.Background(), sub, "r1", "s1", 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Capabilities.CanMention {
		t.Fatal("flag off must can_mention=false")
	}
	if !resp.Capabilities.CanCreate {
		t.Fatal("can_create should remain true")
	}

	store := &wsc.MemoryMentionCandidateStore{}
	svcOn := wsc.NewService(d, repo, nil).WithMentionsEnabled(true).WithMentionCandidates(store)
	resp2, err := svcOn.ListComments(context.Background(), sub, "r1", "s1", 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if !resp2.Capabilities.CanMention {
		t.Fatal("flag on + store must can_mention=true")
	}
}
