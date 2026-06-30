package app

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestPrepareDispatch_FlagOn_RuleMissing_Skip(t *testing.T) {
	eval := NewNotificationRulesEvaluator(&fakeNotificationRulesReader{}, true)
	svc := NewService(nil, nil, nil, WithNotificationRulesFoundation(&fakeNotificationRulesReader{}, eval)).(*service)
	c := DispatchCandidate{
		CompanyID:       "c1",
		OccurrenceID:    "occ-missing",
		IdempotencyKey:  "occ-missing",
		RecipientEmails: []string{"a@example.com"},
		ScheduledAt:     time.Now().UTC(),
	}
	out := svc.prepareDispatch(context.Background(), c, time.Now().UTC())
	if !out.skip || out.skipReason != SkipReasonRuleMissing {
		t.Fatalf("skip=%v reason=%q want %q", out.skip, out.skipReason, SkipReasonRuleMissing)
	}
}

func TestPrepareDispatch_FlagOn_ChannelDisabled_Skip(t *testing.T) {
	reader := &fakeNotificationRulesReader{byCompany: map[string]*AlertChannelPrefsDocument{
		"c1": validPrefsDoc("c1", false),
	}}
	eval := NewNotificationRulesEvaluator(reader, true)
	svc := NewService(nil, nil, nil, WithNotificationRulesFoundation(reader, eval)).(*service)
	now := time.Now().UTC()
	c := DispatchCandidate{
		CompanyID:       "c1",
		RecipientEmails: []string{"a@example.com"},
		ScheduledAt:     now,
		DeadlineAt:      now.Add(7 * 24 * time.Hour),
	}
	out := svc.prepareDispatch(context.Background(), c, now)
	if !out.skip || out.skipReason != SkipReasonChannelDisabled {
		t.Fatalf("skip=%v reason=%q", out.skip, out.skipReason)
	}
}

func TestPrepareDispatch_FlagOn_ScheduleNotMatched_Skip(t *testing.T) {
	reader := &fakeNotificationRulesReader{byCompany: map[string]*AlertChannelPrefsDocument{
		"c1": validPrefsDoc("c1", true),
	}}
	eval := NewNotificationRulesEvaluator(reader, true)
	svc := NewService(nil, nil, nil, WithNotificationRulesFoundation(reader, eval)).(*service)
	now := time.Now().UTC()
	c := DispatchCandidate{
		CompanyID:       "c1",
		RecipientEmails: []string{"a@example.com"},
		ScheduledAt:     now,
		DeadlineAt:      now.Add(3 * 24 * time.Hour), // offset 3, prefs only has 7
	}
	out := svc.prepareDispatch(context.Background(), c, now)
	if !out.skip || out.skipReason != SkipReasonRuleScheduleNotMatched {
		t.Fatalf("skip=%v reason=%q", out.skip, out.skipReason)
	}
}

func TestPrepareDispatch_FlagOn_Eligible_Continues(t *testing.T) {
	alertRepo := &fakeAlertConfigRepoForDispatch{rows: []AlertTemplateConfig{
		{TypeID: "dt1", AlertKind: string(AlertKindDeadline), Enabled: true, TemplateKey: "tpl"},
	}}
	reader := &fakeNotificationRulesReader{byCompany: map[string]*AlertChannelPrefsDocument{
		"c1": validPrefsDocNoPolicies("c1", true),
	}}
	eval := NewNotificationRulesEvaluator(reader, true)
	svc := NewService(nil, nil, nil,
		WithAlertConfigRepo(alertRepo),
		WithNotificationRulesFoundation(reader, eval),
	).(*service)
	now := time.Now().UTC()
	c := DispatchCandidate{
		CompanyID:        "c1",
		DisclosureTypeID: "dt1",
		RecipientEmails:  []string{"a@example.com"},
		ScheduledAt:      now,
		DeadlineAt:       now.Add(7 * 24 * time.Hour),
	}
	out := svc.prepareDispatch(context.Background(), c, now)
	if out.skip {
		t.Fatalf("unexpected skip reason=%q", out.skipReason)
	}
	if out.templateCode != "tpl" {
		t.Fatalf("template=%q", out.templateCode)
	}
}

func TestDispatchDue_FlagOn_RuleMissing_NoSend(t *testing.T) {
	reader := &fakeNotificationRulesReader{}
	eval := NewNotificationRulesEvaluator(reader, true)
	candidates := []DispatchCandidate{{
		OccurrenceID:    "occ1",
		IdempotencyKey:  "occ1",
		RecipientEmails: []string{"user@example.com"},
		CompanyID:       "c1",
		ScheduledAt:     time.Now().UTC(),
		DeadlineAt:      time.Now().UTC().Add(7 * 24 * time.Hour),
	}}
	svc := NewService(fakeConfigRepo{}, &fakeOccurrenceRepo{
		candidates: candidates,
		occ: ReminderOccurrenceDTO{
			OccurrenceID:   "occ1",
			IdempotencyKey: "occ1",
			Status:         ReminderStatusPending,
		},
	}, &fakeAttemptRepo{},
		WithEmailSender(stubEmailSender{}),
		WithNotificationRulesFoundation(reader, eval),
	)
	res, err := svc.DispatchDueOccurrences(context.Background(), time.Now().UTC(), 10)
	if err != nil {
		t.Fatalf("DispatchDueOccurrences: %v", err)
	}
	if res.Skipped != 1 || res.Sent != 0 {
		t.Fatalf("expected skip=1 sent=0 got %+v", res)
	}
}

func TestDispatchDue_FlagOn_Eligible_Sends(t *testing.T) {
	reader := &fakeNotificationRulesReader{byCompany: map[string]*AlertChannelPrefsDocument{
		"c1": validPrefsDocNoPolicies("c1", true),
	}}
	eval := NewNotificationRulesEvaluator(reader, true)
	now := time.Now().UTC()
	candidates := []DispatchCandidate{{
		OccurrenceID:    "occ2",
		IdempotencyKey:  "occ2",
		TemplateCode:    "reminder.deadline",
		RecipientEmails: []string{"user@example.com"},
		CompanyID:       "c1",
		ScheduledAt:     now,
		DeadlineAt:      now.Add(7 * 24 * time.Hour),
	}}
	svc := NewService(fakeConfigRepo{}, &fakeOccurrenceRepo{
		candidates: candidates,
		occ: ReminderOccurrenceDTO{
			OccurrenceID:   "occ2",
			IdempotencyKey: "occ2",
			Status:         ReminderStatusPending,
		},
	}, &fakeAttemptRepo{},
		WithEmailSender(stubEmailSender{}),
		WithNotificationRulesFoundation(reader, eval),
	)
	res, err := svc.DispatchDueOccurrences(context.Background(), now, 10)
	if err != nil {
		t.Fatalf("DispatchDueOccurrences: %v", err)
	}
	if res.Sent != 1 {
		t.Fatalf("expected sent=1 got %+v", res)
	}
}

func TestDispatchDue_FlagOn_EvaluatorError_Retry(t *testing.T) {
	reader := &fakeNotificationRulesReader{err: errors.New("db down")}
	eval := NewNotificationRulesEvaluator(reader, true)
	now := time.Now().UTC()
	candidates := []DispatchCandidate{{
		OccurrenceID:    "occ3",
		IdempotencyKey:  "occ3",
		RecipientEmails: []string{"user@example.com"},
		CompanyID:       "c1",
		ScheduledAt:     now,
	}}
	svc := NewService(fakeConfigRepo{}, &fakeOccurrenceRepo{
		candidates: candidates,
		occ: ReminderOccurrenceDTO{
			OccurrenceID:   "occ3",
			IdempotencyKey: "occ3",
			Status:         ReminderStatusPending,
			AttemptCount:   0,
		},
	}, &fakeAttemptRepo{},
		WithNotificationRulesFoundation(reader, eval),
	)
	res, err := svc.DispatchDueOccurrences(context.Background(), now, 10)
	if err != nil {
		t.Fatalf("DispatchDueOccurrences: %v", err)
	}
	if res.Retried != 1 {
		t.Fatalf("expected retried=1 got %+v", res)
	}
}

func TestApplyRecipientPolicies_FiltersAll(t *testing.T) {
	querier := &fakeMembershipQuerier{
		adminEmails: map[string][]string{"c1": {"admin@co.com"}},
	}
	candidates := []string{"other@co.com"}
	filtered, err := applyRecipientPolicies(context.Background(), querier, nil, nil, "c1", candidates, []string{"company_admin"}, DispatchCandidate{})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(filtered) != 0 {
		t.Fatalf("expected empty filtered got %v", filtered)
	}
}

func TestApplyRecipientPolicies_Intersect(t *testing.T) {
	querier := &fakeMembershipQuerier{
		adminEmails: map[string][]string{"c1": {"admin@co.com"}},
	}
	candidates := []string{"admin@co.com", "other@co.com"}
	filtered, err := applyRecipientPolicies(context.Background(), querier, nil, nil, "c1", candidates, []string{"company_admin"}, DispatchCandidate{})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(filtered) != 1 || filtered[0] != "admin@co.com" {
		t.Fatalf("filtered=%v", filtered)
	}
}

func TestMatchScheduleAtDispatch(t *testing.T) {
	offset7 := 7
	doc := &AlertChannelPrefsDocument{
		Schedules: []SchedulePref{{OffsetDays: &offset7, Enabled: true}},
	}
	now := time.Date(2026, 6, 30, 10, 0, 0, 0, time.UTC)
	due := now.Add(7 * 24 * time.Hour)
	ok, _ := matchScheduleAtDispatch(doc, due, now)
	if !ok {
		t.Fatal("expected schedule match")
	}
}
