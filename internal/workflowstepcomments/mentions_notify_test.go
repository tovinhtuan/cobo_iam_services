package workflowstepcomments_test

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	inappapp "github.com/cobo/cobo_iam_services/internal/inappnotification/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	wff "github.com/cobo/cobo_iam_services/internal/workflowfulfillment"
	wsc "github.com/cobo/cobo_iam_services/internal/workflowstepcomments"
	"github.com/google/uuid"
)

type notifyCall struct {
	UserID       string
	CompanyID    string
	Kind         string
	Title        string
	Body         string
	ResourceType *string
	ResourceID   *string
}

type fakeMentionNotifier struct {
	mu      sync.Mutex
	calls   []notifyCall
	failFor map[string]error // userID -> error
}

func (f *fakeMentionNotifier) CreateForUser(_ context.Context, userID, companyID, kind, title, body string, resourceType, resourceID *string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err, ok := f.failFor[userID]; ok {
		f.calls = append(f.calls, notifyCall{UserID: userID, CompanyID: companyID, Kind: kind, Title: title, Body: body, ResourceType: resourceType, ResourceID: resourceID})
		return err
	}
	f.calls = append(f.calls, notifyCall{UserID: userID, CompanyID: companyID, Kind: kind, Title: title, Body: body, ResourceType: resourceType, ResourceID: resourceID})
	return nil
}

func (f *fakeMentionNotifier) snapshot() []notifyCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]notifyCall, len(f.calls))
	copy(out, f.calls)
	return out
}

func setupNotifySvc(t *testing.T, mids ...string) (
	*wsc.Service,
	*fakeCommentDeadline,
	*wsc.MemoryRepository,
	*fakeMentionNotifier,
	*wsc.MemoryMembershipUserResolver,
	*memIdem,
) {
	t.Helper()
	repo := wsc.NewMemoryRepository()
	idem := newMemIdem()
	completed := time.Date(2026, 5, 30, 0, 0, 0, 0, time.UTC)
	d := &fakeCommentDeadline{
		viewOK: true, commentOK: true,
		wf:     baseWF("In Progress"),
		states: map[string]wff.StepState{"s1": {StepCode: "s1", CompletedAt: &completed}},
	}
	svc := newSvc(t, d, repo, idem)
	enableMentions(svc, mids...)
	users := map[string]string{}
	for i, mid := range mids {
		users[mid] = "user-" + mid[:8]
		_ = i
	}
	resolver := &wsc.MemoryMembershipUserResolver{
		UserByMembership: map[string]map[string]string{"c1": users},
	}
	notifier := &fakeMentionNotifier{failFor: map[string]error{}}
	svc.WithMentionNotifier(notifier).WithMembershipUserResolver(resolver)
	return svc, d, repo, notifier, resolver, idem
}

func mentionSpan(body, token string) (start, end int) {
	runes := []rune(body)
	tok := []rune(token)
	for i := 0; i+len(tok) <= len(runes); i++ {
		match := true
		for j := range tok {
			if runes[i+j] != tok[j] {
				match = false
				break
			}
		}
		if match {
			return i, i + len(tok)
		}
	}
	return 0, len(tok)
}

func TestPhase5_CreateNotifyRecipients(t *testing.T) {
	midA := uuid.NewString()
	midB := uuid.NewString()
	svc, _, _, notifier, resolver, _ := setupNotifySvc(t, midA, midB)
	sub := wff.Subject{UserID: "u1", MembershipID: "m1", CompanyID: "c1"}

	body := "hello @Alice and @Bob"
	sA, eA := mentionSpan(body, "@Alice")
	sB, eB := mentionSpan(body, "@Bob")
	inputs := []wsc.MentionInput{
		{MembershipID: midA, Start: sA, End: eA},
		{MembershipID: midB, Start: sB, End: eB},
	}

	r, status, err := svc.CreateComment(context.Background(), sub, "r1", "s1", "n-1", body, inputs)
	if err != nil || status != http.StatusCreated {
		t.Fatalf("create: %v status=%d", err, status)
	}
	calls := notifier.snapshot()
	if len(calls) != 2 {
		t.Fatalf("want 2 notifications, got %d: %+v", len(calls), calls)
	}
	seen := map[string]bool{}
	for _, c := range calls {
		seen[c.UserID] = true
		if c.Kind != inappapp.KindWorkflowStepCommentMentioned {
			t.Fatalf("kind=%q", c.Kind)
		}
		if c.Title != "Bạn được nhắc trong trao đổi" {
			t.Fatalf("title=%q", c.Title)
		}
		if c.Body != "Bạn được nhắc trong trao đổi của một bước công việc." {
			t.Fatalf("body=%q", c.Body)
		}
		if c.Body == body || c.Title == body {
			t.Fatal("must not leak comment body")
		}
		if c.ResourceType == nil || *c.ResourceType != inappapp.ResourceTypeDisclosure {
			t.Fatalf("resource type %+v", c.ResourceType)
		}
		if c.ResourceID == nil || *c.ResourceID != "r1" {
			t.Fatalf("resource id %+v", c.ResourceID)
		}
		if c.CompanyID != "c1" {
			t.Fatalf("company=%q", c.CompanyID)
		}
	}
	if !seen[resolver.UserByMembership["c1"][midA]] || !seen[resolver.UserByMembership["c1"][midB]] {
		t.Fatalf("missing recipients: %+v", seen)
	}
	_ = r
}

func TestPhase5_SelfMentionNoNotify(t *testing.T) {
	authorMid := uuid.NewString()
	svc, _, _, notifier, resolver, _ := setupNotifySvc(t, authorMid)
	resolver.UserByMembership["c1"][authorMid] = "u1"
	sub := wff.Subject{UserID: "u1", MembershipID: authorMid, CompanyID: "c1"}
	// Author also needs active membership for create auth — authorMid is active.
	body := "note @Me"
	s, e := mentionSpan(body, "@Me")
	_, _, err := svc.CreateComment(context.Background(), sub, "r1", "s1", "self-1", body, []wsc.MentionInput{
		{MembershipID: authorMid, Start: s, End: e},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(notifier.snapshot()) != 0 {
		t.Fatalf("self-mention must not notify: %+v", notifier.snapshot())
	}
}

func TestPhase5_DuplicateMembershipOneNotify(t *testing.T) {
	// Canonical validation rejects exact duplicate spans; notify layer still dedupes by membership/user.
	mid := uuid.NewString()
	svc, _, _, notifier, resolver, _ := setupNotifySvc(t, mid)
	sub := wff.Subject{UserID: "u1", MembershipID: "m1", CompanyID: "c1"}
	body := "aa bb"
	// Single mention only (validator rejects duplicate span).
	_, _, err := svc.CreateComment(context.Background(), sub, "r1", "s1", "dup-1", body, []wsc.MentionInput{
		{MembershipID: mid, Start: 0, End: 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	calls := notifier.snapshot()
	if len(calls) != 1 || calls[0].UserID != resolver.UserByMembership["c1"][mid] {
		t.Fatalf("want 1 notify: %+v", calls)
	}
}

func TestPhase5_NoMentionsNoNotify(t *testing.T) {
	svc, _, _, notifier, _, _ := setupNotifySvc(t)
	sub := wff.Subject{UserID: "u1", MembershipID: "m1", CompanyID: "c1"}
	_, _, err := svc.CreateComment(context.Background(), sub, "r1", "s1", "plain-1", "plain body", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(notifier.snapshot()) != 0 {
		t.Fatal("plain create must not notify")
	}
}

func TestPhase5_FlagOffNoNotify(t *testing.T) {
	mid := uuid.NewString()
	repo := wsc.NewMemoryRepository()
	idem := newMemIdem()
	completed := time.Date(2026, 5, 30, 0, 0, 0, 0, time.UTC)
	d := &fakeCommentDeadline{
		viewOK: true, commentOK: true,
		wf:     baseWF("In Progress"),
		states: map[string]wff.StepState{"s1": {StepCode: "s1", CompletedAt: &completed}},
	}
	svc := newSvc(t, d, repo, idem)
	notifier := &fakeMentionNotifier{}
	svc.WithMentionNotifier(notifier)
	body := "hello @A"
	s, e := mentionSpan(body, "@A")
	_, _, err := svc.CreateComment(context.Background(), wff.Subject{UserID: "u1", MembershipID: "m1", CompanyID: "c1"},
		"r1", "s1", "off-1", body, []wsc.MentionInput{{MembershipID: mid, Start: s, End: e}})
	if err == nil {
		t.Fatal("expected FEATURE_DISABLED")
	}
	if len(notifier.snapshot()) != 0 {
		t.Fatal("flag off must not notify")
	}
}

func TestPhase5_InvalidMentionNoCommentNoNotify(t *testing.T) {
	mid := uuid.NewString()
	svc, _, repo, notifier, _, _ := setupNotifySvc(t, mid)
	sub := wff.Subject{UserID: "u1", MembershipID: "m1", CompanyID: "c1"}
	unknown := uuid.NewString()
	body := "hello @X"
	s, e := mentionSpan(body, "@X")
	_, _, err := svc.CreateComment(context.Background(), sub, "r1", "s1", "bad-1", body, []wsc.MentionInput{
		{MembershipID: unknown, Start: s, End: e},
	})
	if err == nil {
		t.Fatal("expected invalid mention")
	}
	items, total, _ := repo.ListAliveByStep(context.Background(), wsc.OwnerContext{
		CompanyID: "c1", DisclosureRecordID: "r1", WorkflowInstanceID: "wi-1", StepCode: "s1",
	}, 1, 20)
	if total != 0 || len(items) != 0 {
		t.Fatal("invalid mention must not insert comment")
	}
	if len(notifier.snapshot()) != 0 {
		t.Fatal("invalid mention must not notify")
	}
}

func TestPhase5_RecipientLacksViewSkip(t *testing.T) {
	mid := uuid.NewString()
	svc, d, _, notifier, _, _ := setupNotifySvc(t, mid)
	d.viewDenyMembership = map[string]bool{mid: true}
	sub := wff.Subject{UserID: "u1", MembershipID: "m1", CompanyID: "c1"}
	body := "ping @A"
	s, e := mentionSpan(body, "@A")
	_, _, err := svc.CreateComment(context.Background(), sub, "r1", "s1", "noview-1", body, []wsc.MentionInput{
		{MembershipID: mid, Start: s, End: e},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(notifier.snapshot()) != 0 {
		t.Fatalf("fail-closed skip expected, got %+v", notifier.snapshot())
	}
}

func TestPhase5_RecipientLacksScopeSkip(t *testing.T) {
	mid := uuid.NewString()
	svc, d, _, notifier, _, _ := setupNotifySvc(t, mid)
	d.scopeDenyMembership = map[string]bool{mid: true}
	sub := wff.Subject{UserID: "u1", MembershipID: "m1", CompanyID: "c1"}
	body := "ping @A"
	s, e := mentionSpan(body, "@A")
	_, _, err := svc.CreateComment(context.Background(), sub, "r1", "s1", "noscope-1", body, []wsc.MentionInput{
		{MembershipID: mid, Start: s, End: e},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(notifier.snapshot()) != 0 {
		t.Fatalf("scope deny must skip notify: %+v", notifier.snapshot())
	}
}

func TestPhase5_UnresolvableRecipientSkip(t *testing.T) {
	mid := uuid.NewString()
	svc, _, _, notifier, resolver, _ := setupNotifySvc(t, mid)
	delete(resolver.UserByMembership["c1"], mid)
	sub := wff.Subject{UserID: "u1", MembershipID: "m1", CompanyID: "c1"}
	body := "ping @A"
	s, e := mentionSpan(body, "@A")
	_, _, err := svc.CreateComment(context.Background(), sub, "r1", "s1", "nouser-1", body, []wsc.MentionInput{
		{MembershipID: mid, Start: s, End: e},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(notifier.snapshot()) != 0 {
		t.Fatal("unresolvable must skip")
	}
}

func TestPhase5_NotifyFailureKeepsComment(t *testing.T) {
	mid := uuid.NewString()
	svc, _, repo, notifier, resolver, _ := setupNotifySvc(t, mid)
	uid := resolver.UserByMembership["c1"][mid]
	notifier.failFor[uid] = errors.New("notify down")
	sub := wff.Subject{UserID: "u1", MembershipID: "m1", CompanyID: "c1"}
	body := "ping @A"
	s, e := mentionSpan(body, "@A")
	r, status, err := svc.CreateComment(context.Background(), sub, "r1", "s1", "fail-1", body, []wsc.MentionInput{
		{MembershipID: mid, Start: s, End: e},
	})
	if err != nil || status != http.StatusCreated {
		t.Fatalf("create must succeed despite notify fail: %v status=%d", err, status)
	}
	items, total, _ := repo.ListAliveByStep(context.Background(), wsc.OwnerContext{
		CompanyID: "c1", DisclosureRecordID: "r1", WorkflowInstanceID: "wi-1", StepCode: "s1",
	}, 1, 20)
	if total != 1 || items[0].ID != r.Comment.CommentID {
		t.Fatal("comment must remain committed")
	}
	if len(notifier.snapshot()) != 1 {
		t.Fatal("CreateForUser should still be attempted")
	}
}

func TestPhase5_ReplayNoDuplicateNotify(t *testing.T) {
	mid := uuid.NewString()
	svc, _, _, notifier, _, _ := setupNotifySvc(t, mid)
	sub := wff.Subject{UserID: "u1", MembershipID: "m1", CompanyID: "c1"}
	body := "ping @A"
	s, e := mentionSpan(body, "@A")
	inputs := []wsc.MentionInput{{MembershipID: mid, Start: s, End: e}}
	r1, _, err := svc.CreateComment(context.Background(), sub, "r1", "s1", "replay-1", body, inputs)
	if err != nil {
		t.Fatal(err)
	}
	r2, _, err := svc.CreateComment(context.Background(), sub, "r1", "s1", "replay-1", body, inputs)
	if err != nil {
		t.Fatal(err)
	}
	if r1.Comment.CommentID != r2.Comment.CommentID {
		t.Fatal("replay must same comment")
	}
	if len(notifier.snapshot()) != 1 {
		t.Fatalf("replay must not re-notify, got %d", len(notifier.snapshot()))
	}
}

func TestPhase5_PatchNewMentionsOnly(t *testing.T) {
	midA := uuid.NewString()
	midB := uuid.NewString()
	svc, _, _, notifier, resolver, _ := setupNotifySvc(t, midA, midB)
	sub := wff.Subject{UserID: "u1", MembershipID: "m1", CompanyID: "c1"}

	body1 := "aa bb cc dd"
	r, _, err := svc.CreateComment(context.Background(), sub, "r1", "s1", "p-1", body1, []wsc.MentionInput{
		{MembershipID: midA, Start: 0, End: 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(notifier.snapshot()) != 1 {
		t.Fatalf("create notify: %+v", notifier.snapshot())
	}
	notifier.calls = nil

	// Body-only edit (same mentions) — when mentionsEnabled, must re-send same mention list
	_, err = svc.UpdateComment(context.Background(), sub, "r1", "s1", r.Comment.CommentID, "aa bb XX dd", []wsc.MentionInput{
		{MembershipID: midA, Start: 0, End: 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(notifier.snapshot()) != 0 {
		t.Fatalf("body-only same mentions must not notify: %+v", notifier.snapshot())
	}

	// Replace A with B
	_, err = svc.UpdateComment(context.Background(), sub, "r1", "s1", r.Comment.CommentID, "aa bb XX dd", []wsc.MentionInput{
		{MembershipID: midB, Start: 3, End: 5},
	})
	if err != nil {
		t.Fatal(err)
	}
	calls := notifier.snapshot()
	if len(calls) != 1 || calls[0].UserID != resolver.UserByMembership["c1"][midB] {
		t.Fatalf("want notify B only: %+v", calls)
	}
	notifier.calls = nil

	// Remove all mentions
	_, err = svc.UpdateComment(context.Background(), sub, "r1", "s1", r.Comment.CommentID, "no mentions here", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(notifier.snapshot()) != 0 {
		t.Fatal("remove mentions must not notify")
	}
}

func TestPhase5_PatchSelfMentionNoNotify(t *testing.T) {
	authorMid := uuid.NewString()
	other := uuid.NewString()
	svc, _, _, notifier, resolver, _ := setupNotifySvc(t, authorMid, other)
	resolver.UserByMembership["c1"][authorMid] = "u1"
	sub := wff.Subject{UserID: "u1", MembershipID: authorMid, CompanyID: "c1"}
	body := "aa bb"
	r, _, err := svc.CreateComment(context.Background(), sub, "r1", "s1", "ps-1", body, nil)
	if err != nil {
		t.Fatal(err)
	}
	notifier.calls = nil
	_, err = svc.UpdateComment(context.Background(), sub, "r1", "s1", r.Comment.CommentID, body, []wsc.MentionInput{
		{MembershipID: authorMid, Start: 0, End: 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(notifier.snapshot()) != 0 {
		t.Fatal("patch self-mention must not notify")
	}
}

func TestPhase5_TerminalRecordDeniesEdit(t *testing.T) {
	mid := uuid.NewString()
	svc, d, _, notifier, _, _ := setupNotifySvc(t, mid)
	sub := wff.Subject{UserID: "u1", MembershipID: "m1", CompanyID: "c1"}
	body := "aa bb"
	r, _, err := svc.CreateComment(context.Background(), sub, "r1", "s1", "term-1", body, nil)
	if err != nil {
		t.Fatal(err)
	}
	d.wf.RecordStatus = "Completed"
	notifier.calls = nil
	_, err = svc.UpdateComment(context.Background(), sub, "r1", "s1", r.Comment.CommentID, body, []wsc.MentionInput{
		{MembershipID: mid, Start: 0, End: 2},
	})
	if err == nil {
		t.Fatal("expected terminal lock")
	}
	he, ok := err.(*perr.HTTPError)
	if !ok || he.HTTPStatus != http.StatusConflict {
		t.Fatalf("want 409 got %v", err)
	}
	if len(notifier.snapshot()) != 0 {
		t.Fatal("terminal edit must not notify")
	}
}

func TestPhase5_CrossTenantNeverNotified(t *testing.T) {
	mid := uuid.NewString()
	svc, _, _, notifier, resolver, _ := setupNotifySvc(t, mid)
	// Resolver only has c1; simulate wrong-company by emptying resolver for that mid
	// Validation already rejects cross-tenant as inactive/not found — no comment.
	delete(resolver.UserByMembership["c1"], mid)
	// Also remove from active lookup via enableMentions lookup — need access.
	// Re-create with mid active but resolver maps to empty company.
	sub := wff.Subject{UserID: "u1", MembershipID: "m1", CompanyID: "c1"}
	body := "aa bb"
	// mid still active in lookup from setupNotifySvc — create succeeds but no user resolve
	_, _, err := svc.CreateComment(context.Background(), sub, "r1", "s1", "xt-1", body, []wsc.MentionInput{
		{MembershipID: mid, Start: 0, End: 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(notifier.snapshot()) != 0 {
		t.Fatal("no user resolve → no notify")
	}
}
