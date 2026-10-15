package app

import (
	"context"
	"errors"
	"testing"
)

type countedRepo struct {
	created []InAppNotification
	failFor map[string]bool
}

func (r *countedRepo) Create(_ context.Context, n InAppNotification) error {
	if r.failFor[n.UserID] {
		return errors.New("insert failed")
	}
	r.created = append(r.created, n)
	return nil
}
func (r *countedRepo) ListByUser(context.Context, string, string, int) ([]InAppNotification, error) {
	return nil, nil
}
func (r *countedRepo) UnreadCount(context.Context, string, string) (int, error) { return 0, nil }
func (r *countedRepo) MarkRead(context.Context, string, string) error           { return nil }
func (r *countedRepo) MarkAllRead(context.Context, string, string) error        { return nil }

type countedUsers struct {
	ids []string
	err error
}

func (u countedUsers) UserIDsByEmails(context.Context, string, []string) ([]string, error) {
	return u.ids, u.err
}

// H7 PERF-01: the reminder worker needs to know whether in-app delivery happened.
func TestCreateForReminderCounted(t *testing.T) {
	req := ReminderInAppRequest{CompanyID: "c1", Kind: KindReminderDeadline, Title: "t", RecipientEmails: []string{"a@x", "b@x"}}

	repo := &countedRepo{}
	n, err := NewService(repo, countedUsers{ids: []string{"u1", "u2"}}, nil).(*svc).CreateForReminderCounted(context.Background(), req)
	if err != nil || n != 2 || len(repo.created) != 2 {
		t.Fatalf("created=%d err=%v rows=%d, want 2/nil/2", n, err, len(repo.created))
	}

	n, err = NewService(&countedRepo{}, countedUsers{}, nil).(*svc).CreateForReminderCounted(context.Background(), req)
	if err != nil || n != 0 {
		t.Fatalf("no matching user: created=%d err=%v, want 0/nil", n, err)
	}

	n, err = NewService(&countedRepo{failFor: map[string]bool{"u2": true}}, countedUsers{ids: []string{"u1", "u2"}}, nil).(*svc).CreateForReminderCounted(context.Background(), req)
	if err == nil || n != 1 {
		t.Fatalf("partial failure: created=%d err=%v, want 1/error", n, err)
	}

	n, err = NewService(&countedRepo{}, countedUsers{err: errors.New("db down")}, nil).(*svc).CreateForReminderCounted(context.Background(), req)
	if err == nil || n != 0 {
		t.Fatalf("lookup failure: created=%d err=%v, want 0/error", n, err)
	}

	// The fire-and-forget API keeps returning nil.
	if err := NewService(&countedRepo{}, countedUsers{err: errors.New("db down")}, nil).CreateForReminder(context.Background(), req); err != nil {
		t.Fatalf("CreateForReminder must stay nil-returning, got %v", err)
	}
}
