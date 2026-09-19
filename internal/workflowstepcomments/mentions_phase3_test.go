package workflowstepcomments_test

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	wff "github.com/cobo/cobo_iam_services/internal/workflowfulfillment"
	wsc "github.com/cobo/cobo_iam_services/internal/workflowstepcomments"
	"github.com/google/uuid"
)

func enableMentions(svc *wsc.Service, mid ...string) (*wsc.MemoryMentionsRepository, *wsc.MemoryMembershipActiveLookup, *wsc.MemoryMentionDisplayResolver) {
	active := map[string]bool{}
	displays := map[string]wsc.MentionDisplayInfo{}
	for _, id := range mid {
		active[id] = true
		displays[id] = wsc.MentionDisplayInfo{DisplayName: "User " + id[:8], Active: true}
	}
	lookup := &wsc.MemoryMembershipActiveLookup{Active: map[string]map[string]bool{"c1": active}}
	mentions := wsc.NewMemoryMentionsRepository()
	disp := &wsc.MemoryMentionDisplayResolver{ByMembership: displays}
	svc.WithMentionsEnabled(true).
		WithMentionsRepository(mentions).
		WithMembershipLookup(lookup).
		WithMentionDisplays(disp).
		WithMentionCandidates(&wsc.MemoryMentionCandidateStore{})
	return mentions, lookup, disp
}

func TestPhase3_CreatePatchMentionsAndIdempotency(t *testing.T) {
	repo := wsc.NewMemoryRepository()
	idem := newMemIdem()
	completed := time.Date(2026, 5, 30, 0, 0, 0, 0, time.UTC)
	d := &fakeCommentDeadline{
		viewOK: true, commentOK: true,
		wf:     baseWF("In Progress"),
		states: map[string]wff.StepState{"s1": {StepCode: "s1", CompletedAt: &completed}},
	}
	svc := newSvc(t, d, repo, idem)
	mid := uuid.NewString()
	mentionsRepo, lookup, _ := enableMentions(svc, mid)
	sub := wff.Subject{UserID: "u1", MembershipID: "m1", CompanyID: "c1"}

	body := "please review @Alice now"
	start := len([]rune("please review "))
	end := len([]rune("please review @Alice"))
	inputs := []wsc.MentionInput{{MembershipID: mid, Start: start, End: end}}

	// Flag OFF + mentions → 400, no insert, no idem success
	svcOff := newSvc(t, d, wsc.NewMemoryRepository(), newMemIdem())
	_, _, err := svcOff.CreateComment(context.Background(), sub, "r1", "s1", "flag-off", body, inputs)
	if err == nil {
		t.Fatal("expected FEATURE_DISABLED")
	}
	he, ok := err.(*perr.HTTPError)
	if !ok || he.Code != perr.CodeFeatureDisabled || he.HTTPStatus != http.StatusBadRequest {
		t.Fatalf("want 400 FEATURE_DISABLED got %v", err)
	}

	// Create with mentions
	r1, status, err := svc.CreateComment(context.Background(), sub, "r1", "s1", "mk-1", body, inputs)
	if err != nil || status != http.StatusCreated {
		t.Fatalf("create: %v status=%d", err, status)
	}
	if len(r1.Comment.Mentions) != 1 || r1.Comment.Mentions[0].MembershipID != mid {
		t.Fatalf("dto mentions: %+v", r1.Comment.Mentions)
	}
	stored, _ := mentionsRepo.ListByComment(context.Background(), "c1", r1.Comment.CommentID)
	if len(stored) != 1 {
		t.Fatalf("persisted mentions=%d", len(stored))
	}

	// V1 body-only create still works when flag ON with nil mentions
	rV1, _, err := svc.CreateComment(context.Background(), sub, "r1", "s1", "v1-only", "plain body", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(rV1.Comment.Mentions) != 0 {
		// empty slice ok; nil also ok with omitempty
	}

	// Replay same body+mentions
	r2, _, err := svc.CreateComment(context.Background(), sub, "r1", "s1", "mk-1", body, inputs)
	if err != nil {
		t.Fatal(err)
	}
	if r2.Comment.CommentID != r1.Comment.CommentID {
		t.Fatal("replay must same comment")
	}
	stored2, _ := mentionsRepo.ListByComment(context.Background(), "c1", r1.Comment.CommentID)
	if len(stored2) != 1 {
		t.Fatal("replay must not duplicate mention rows")
	}

	// Same key + different mentions → 409
	otherMid := uuid.NewString()
	lookup.Active["c1"][otherMid] = true
	_, _, err = svc.CreateComment(context.Background(), sub, "r1", "s1", "mk-1", body, []wsc.MentionInput{
		{MembershipID: otherMid, Start: start, End: end},
	})
	if err == nil {
		t.Fatal("expected hash conflict")
	}

	// Reordered equivalent mentions → same hash → replay
	mid2 := uuid.NewString()
	lookup.Active["c1"][mid2] = true
	body2 := "aa bb cc dd"
	inA := []wsc.MentionInput{
		{MembershipID: mid2, Start: 3, End: 5},
		{MembershipID: mid, Start: 0, End: 2},
	}
	inB := []wsc.MentionInput{
		{MembershipID: mid, Start: 0, End: 2},
		{MembershipID: mid2, Start: 3, End: 5},
	}
	ra, _, err := svc.CreateComment(context.Background(), sub, "r1", "s1", "reorder", body2, inA)
	if err != nil {
		t.Fatal(err)
	}
	rb, _, err := svc.CreateComment(context.Background(), sub, "r1", "s1", "reorder", body2, inB)
	if err != nil {
		t.Fatal(err)
	}
	if ra.Comment.CommentID != rb.Comment.CommentID {
		t.Fatal("reordered mentions must replay")
	}

	// omit/[] equivalent hash for V1 path
	idem2 := newMemIdem()
	svc2 := newSvc(t, d, wsc.NewMemoryRepository(), idem2)
	enableMentions(svc2, mid)
	rOmit, _, err := svc2.CreateComment(context.Background(), sub, "r1", "s1", "empty-eq", "solo", nil)
	if err != nil {
		t.Fatal(err)
	}
	rEmpty, _, err := svc2.CreateComment(context.Background(), sub, "r1", "s1", "empty-eq", "solo", []wsc.MentionInput{})
	if err != nil {
		t.Fatal(err)
	}
	if rOmit.Comment.CommentID != rEmpty.Comment.CommentID {
		t.Fatal("nil and [] must share V1 hash")
	}

	// PATCH replace mentions — same mentionsRepo/lookup
	newBody := "updated @Bob xx"
	ns := len([]rune("updated "))
	ne := len([]rune("updated @Bob"))
	midB := uuid.NewString()
	lookup.Active["c1"][midB] = true
	upd, err := svc.UpdateComment(context.Background(), sub, "r1", "s1", r1.Comment.CommentID, newBody, []wsc.MentionInput{
		{MembershipID: midB, Start: ns, End: ne},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(upd.Comment.Mentions) != 1 || upd.Comment.Mentions[0].MembershipID != midB {
		t.Fatalf("patch mentions: %+v", upd.Comment.Mentions)
	}
	after, _ := mentionsRepo.ListByComment(context.Background(), "c1", r1.Comment.CommentID)
	if len(after) != 1 || after[0].MentionedMembershipID != midB {
		t.Fatalf("replace failed: %+v", after)
	}

	// PATCH clear mentions
	cleared, err := svc.UpdateComment(context.Background(), sub, "r1", "s1", r1.Comment.CommentID, "no mentions", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(cleared.Comment.Mentions) != 0 {
		t.Fatalf("clear failed: %+v", cleared.Comment.Mentions)
	}
	emptyRows, _ := mentionsRepo.ListByComment(context.Background(), "c1", r1.Comment.CommentID)
	if len(emptyRows) != 0 {
		t.Fatal("clear must delete mention rows")
	}

	// Validation fail must not reserve
	idem3 := newMemIdem()
	svc3 := newSvc(t, d, wsc.NewMemoryRepository(), idem3)
	enableMentions(svc3, mid)
	_, _, err = svc3.CreateComment(context.Background(), sub, "r1", "s1", "bad-val", "ab", []wsc.MentionInput{
		{MembershipID: mid, Start: 0, End: 99},
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	if len(idem3.rows) != 0 {
		t.Fatal("validation fail must not create idempotency row")
	}

	// Mention insert failure rolls back comment
	repoFail := wsc.NewMemoryRepository()
	idemFail := newMemIdem()
	svcFail := newSvc(t, d, repoFail, idemFail)
	mFail, _, _ := enableMentions(svcFail, mid)
	mFail.FailAfterDelete = true
	// Force fail on replace after empty delete by setting FailOnInsertID after we know ID — use FailAfterDelete on empty which restores and errors
	// For create, Replace deletes then inserts; FailAfterDelete triggers after delete of empty set.
	_, _, err = svcFail.CreateComment(context.Background(), sub, "r1", "s1", "tx-fail", body, inputs)
	if err == nil {
		t.Fatal("expected mention persist failure")
	}
	items, total, _ := repoFail.ListAliveByStep(context.Background(), wsc.OwnerContext{
		CompanyID: "c1", DisclosureRecordID: "r1", WorkflowInstanceID: "wi-1", StepCode: "s1",
	}, 1, 20)
	if total != 0 || len(items) != 0 {
		t.Fatalf("comment must rollback on mention failure: total=%d", total)
	}
	if len(idemFail.rows) != 0 {
		t.Fatal("failed create must abandon reservation")
	}

	// Complete failure leaves comment but logs in_progress limitation
	repoC := wsc.NewMemoryRepository()
	idemC := newMemIdem()
	idemC.failC = true
	svcC := newSvc(t, d, repoC, idemC)
	enableMentions(svcC, mid)
	respC, _, err := svcC.CreateComment(context.Background(), sub, "r1", "s1", "complete-fail", body, inputs)
	if err != nil {
		t.Fatal(err)
	}
	alive, tot, _ := repoC.ListAliveByStep(context.Background(), wsc.OwnerContext{
		CompanyID: "c1", DisclosureRecordID: "r1", WorkflowInstanceID: "wi-1", StepCode: "s1",
	}, 1, 20)
	if tot != 1 || alive[0].ID != respC.Comment.CommentID {
		t.Fatal("comment must remain when Complete fails")
	}
	row := idemC.rows["workflow_step_comment.create.v1|"+ /* can't know key */ ""]
	_ = row
	// Reservation left in_progress: verify via TryReserve conflict on same key
	_, _, err = svcC.CreateComment(context.Background(), sub, "r1", "s1", "complete-fail", body, inputs)
	if err == nil {
		t.Fatal("in_progress reservation should conflict on retry")
	}

	// Inactive display enrichment
	inactive := uuid.NewString()
	svcDisp := newSvc(t, d, wsc.NewMemoryRepository(), nil)
	mentionsD, lookupD, disp := enableMentions(svcDisp, mid)
	lookupD.Active["c1"][inactive] = false
	disp.ByMembership[inactive] = wsc.MentionDisplayInfo{DisplayName: "gone", Active: false}
	// seed mention row manually for LIST
	cmtID := uuid.NewString()
	svcDispRepo := wsc.NewMemoryRepository()
	svcList := newSvc(t, d, svcDispRepo, nil)
	enableMentions(svcList, mid)
	svcList.WithMentionsRepository(mentionsD).WithMentionDisplays(disp)
	svcDispRepo.Seed(wsc.Comment{
		ID: cmtID, CompanyID: "c1", DisclosureRecordID: "r1", WorkflowInstanceID: "wi-1", StepCode: "s1",
		AuthorUserID: "u1", AuthorMembershipID: "m1", Body: "x @y", CreatedAt: time.Now().UTC(),
	})
	_ = mentionsD.ReplaceForComment(context.Background(), nil, "c1", cmtID, []wsc.Mention{{
		ID: uuid.NewString(), CompanyID: "c1", CommentID: cmtID, MentionedMembershipID: inactive,
		StartOffset: 2, EndOffset: 4, CreatedAt: time.Now().UTC(),
	}})
	list, err := svcList.ListComments(context.Background(), sub, "r1", "s1", 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range list.Comments {
		if c.CommentID == cmtID && len(c.Mentions) == 1 {
			found = true
			if c.Mentions[0].DisplayName != wsc.InactiveMentionDisplayName {
				t.Fatalf("inactive display=%q", c.Mentions[0].DisplayName)
			}
		}
	}
	if !found {
		t.Fatal("expected inactive mention enrichment on LIST")
	}

	_ = errors.New("keep import")
}

func TestPhase3_AuthzBeforeReserveWithMentions(t *testing.T) {
	repo := wsc.NewMemoryRepository()
	idem := newMemIdem()
	d := &fakeCommentDeadline{
		viewOK: true, commentOK: false,
		wf: baseWF("Draft"),
		states: map[string]wff.StepState{
			"s1": {StepCode: "s1", CompletedAt: ptrTime(time.Date(2026, 5, 30, 0, 0, 0, 0, time.UTC))},
		},
	}
	svc := newSvc(t, d, repo, idem)
	mid := uuid.NewString()
	enableMentions(svc, mid)
	sub := wff.Subject{UserID: "u1", MembershipID: "m1", CompanyID: "c1"}
	_, _, err := svc.CreateComment(context.Background(), sub, "r1", "s1", "authz", "hi @x", []wsc.MentionInput{
		{MembershipID: mid, Start: 3, End: 5},
	})
	if err == nil {
		t.Fatal("expected authz fail")
	}
	if len(idem.rows) != 0 {
		t.Fatal("authz fail must not reserve")
	}
}
