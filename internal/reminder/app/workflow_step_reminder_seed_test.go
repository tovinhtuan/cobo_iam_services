package app

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeMilestoneScanner struct {
	due     []DueMilestone
	marked  []string
	listErr error
	markErr error
}

func (f *fakeMilestoneScanner) ListDueMilestones(context.Context, time.Time, int) ([]DueMilestone, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	out := make([]DueMilestone, 0, len(f.due))
	sent := map[string]struct{}{}
	for _, id := range f.marked {
		sent[id] = struct{}{}
	}
	for _, m := range f.due {
		if _, ok := sent[m.MilestoneID]; ok {
			continue
		}
		out = append(out, m)
	}
	return out, nil
}

func (f *fakeMilestoneScanner) MarkMilestoneSent(_ context.Context, milestoneID, _ string) error {
	if f.markErr != nil {
		return f.markErr
	}
	f.marked = append(f.marked, milestoneID)
	return nil
}

type seedOccurrenceRepo struct {
	fakeOccurrenceRepo
	byKey map[string]ReminderOccurrenceDTO
	seeds []ReminderOccurrenceDTO
}

func (f *seedOccurrenceRepo) SeedOccurrence(_ context.Context, in ReminderOccurrenceDTO) (*ReminderOccurrenceDTO, error) {
	if f.byKey == nil {
		f.byKey = map[string]ReminderOccurrenceDTO{}
	}
	if _, ok := f.byKey[in.IdempotencyKey]; ok {
		return nil, errors.New("duplicate idempotency_key")
	}
	f.byKey[in.IdempotencyKey] = in
	f.seeds = append(f.seeds, in)
	f.occ = in
	return &in, nil
}

type fakeStepTaskStateReader struct {
	states map[string]WorkflowStepTaskState
}

func (f fakeStepTaskStateReader) StepTaskState(_ context.Context, _, _, stepID string) (WorkflowStepTaskState, error) {
	if f.states == nil {
		return WorkflowStepTaskAbsent, nil
	}
	if st, ok := f.states[stepID]; ok {
		return st, nil
	}
	return WorkflowStepTaskAbsent, nil
}

func TestSeedOccurrencesFromDueMilestones_DueMinusOnlyActiveStepDedup(t *testing.T) {
	now := time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)
	scanner := &fakeMilestoneScanner{due: []DueMilestone{
		{MilestoneID: "m-start", CompanyID: "c1", WorkflowInstanceID: "inst-1", StepID: "step-a", MilestoneType: "step_start", ScheduledDate: now},
		{MilestoneID: "m-before", CompanyID: "c1", WorkflowInstanceID: "inst-1", StepID: "step-a", MilestoneType: "before_start_3d", ScheduledDate: now},
		{MilestoneID: "m-a7", CompanyID: "c1", WorkflowInstanceID: "inst-1", StepID: "step-a", MilestoneType: "due_minus_7d", ScheduledDate: now},
		{MilestoneID: "m-b3", CompanyID: "c1", WorkflowInstanceID: "inst-1", StepID: "step-b", MilestoneType: "due_minus_3d", ScheduledDate: now},
		{MilestoneID: "m-c5", CompanyID: "c1", WorkflowInstanceID: "inst-1", StepID: "step-c", MilestoneType: "due_minus_5d", ScheduledDate: now},
	}}
	occ := &seedOccurrenceRepo{}
	svc := NewService(fakeConfigRepo{}, occ, &fakeAttemptRepo{},
		WithMilestoneScanner(scanner),
		WithWorkflowStepTaskStateReader(fakeStepTaskStateReader{states: map[string]WorkflowStepTaskState{
			"step-a": WorkflowStepTaskPending,
			"step-b": WorkflowStepTaskAbsent,
			"step-c": WorkflowStepTaskCompleted,
		}}),
	)
	n, err := svc.SeedOccurrencesFromDueMilestones(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("seeded=%d want 1 (only active step-a due-minus)", n)
	}
	if len(occ.seeds) != 1 || occ.seeds[0].ScopeID != "step-a" {
		t.Fatalf("seeds=%+v", occ.seeds)
	}
	if occ.seeds[0].IdempotencyKey != WorkflowStepReminderIdempotencyKey("inst-1", "step-a", "due_minus_7d") {
		t.Fatalf("key=%s", occ.seeds[0].IdempotencyKey)
	}

	n2, err := svc.SeedOccurrencesFromDueMilestones(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	if n2 != 0 {
		t.Fatalf("second scan seeded=%d want 0", n2)
	}
	if len(occ.seeds) != 1 {
		t.Fatalf("dedup failed, seeds=%d", len(occ.seeds))
	}
}

func TestWorkflowStepReminderIdempotencyKey_AcceptsArbitraryDueMinus(t *testing.T) {
	for _, mtype := range []string{
		"due_minus_1d", "due_minus_2d", "due_minus_3d", "due_minus_5d", "due_minus_7d", "due_minus_90d",
	} {
		key := WorkflowStepReminderIdempotencyKey("inst-1", "step-a", mtype)
		got, ok := workflowStepReminderMilestoneType(key)
		if !ok || got != mtype {
			t.Fatalf("key=%s got=%s ok=%v", key, got, ok)
		}
		if !IsDueMinusReminderMilestone(mtype) {
			t.Fatalf("%s not accepted as due-minus", mtype)
		}
	}
	if IsDueMinusReminderMilestone("due_minus_d") || IsDueMinusReminderMilestone("step_start") {
		t.Fatal("invalid format must not be treated as due-minus")
	}
}

func TestSeedOccurrencesFromDueMilestones_NextStepGetsOwnRule(t *testing.T) {
	now := time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)
	scanner := &fakeMilestoneScanner{due: []DueMilestone{
		{MilestoneID: "m-b3", CompanyID: "c1", WorkflowInstanceID: "inst-1", StepID: "step-b", MilestoneType: "due_minus_3d", ScheduledDate: now},
	}}
	occ := &seedOccurrenceRepo{}
	svc := NewService(fakeConfigRepo{}, occ, &fakeAttemptRepo{},
		WithMilestoneScanner(scanner),
		WithWorkflowStepTaskStateReader(fakeStepTaskStateReader{states: map[string]WorkflowStepTaskState{
			"step-b": WorkflowStepTaskPending,
		}}),
	)
	n, err := svc.SeedOccurrencesFromDueMilestones(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 || occ.seeds[0].ScopeID != "step-b" {
		t.Fatalf("n=%d seeds=%+v", n, occ.seeds)
	}
	if occ.seeds[0].IdempotencyKey != WorkflowStepReminderIdempotencyKey("inst-1", "step-b", "due_minus_3d") {
		t.Fatalf("key=%s", occ.seeds[0].IdempotencyKey)
	}
}

func TestPrepareDispatch_WorkflowStepReminderContext(t *testing.T) {
	svc := NewService(fakeConfigRepo{}, &fakeOccurrenceRepo{}, &fakeAttemptRepo{}).(*service)
	now := time.Date(2026, 6, 13, 0, 0, 0, 0, time.UTC)
	out := svc.prepareDispatch(context.Background(), DispatchCandidate{
		OccurrenceID:       "occ-1",
		IdempotencyKey:     WorkflowStepReminderIdempotencyKey("inst-1", "step-a", "due_minus_7d"),
		ScopeType:          ScopeTypeWorkflowStep,
		ScopeID:            "step-a",
		WorkflowInstanceID: "inst-1",
		ScheduledAt:        now,
		RecipientEmails:    []string{"a@co.com"},
	}, now)
	if out.payload["step_id"] != "step-a" {
		t.Fatalf("step_id=%v", out.payload["step_id"])
	}
	if out.payload["reminder_offset_days"] != 7 {
		t.Fatalf("offset=%v", out.payload["reminder_offset_days"])
	}
	if out.payload["reminder_milestone_type"] != "due_minus_7d" {
		t.Fatalf("type=%v", out.payload["reminder_milestone_type"])
	}
	if out.payload["step_due_date"] == nil || out.payload["step_due_date"] == "" {
		t.Fatalf("step_due_date missing: %#v", out.payload)
	}
}

func TestResolveForWorkflowStep_NoPreviousStepLeak(t *testing.T) {
	stepReader := &fakeStepReader{steps: map[string]*WorkflowStepConfig{
		"step-a": {StepID: "step-a", AssigneeRoleIDs: []string{"role-a"}},
		"step-b": {StepID: "step-b", AssigneeRoleIDs: []string{"role-b"}},
	}}
	querier := &fakeMembershipQuerier{
		roleEmails: map[string][]string{
			"c1:role-a": {"a@co.com"},
			"c1:role-b": {"b@co.com"},
		},
	}
	r := NewRecipientResolver(nil, stepReader, querier, nil, nil)
	a, err := r.ResolveForWorkflowStep(context.Background(), "c1", "inst-1", "step-a")
	if err != nil {
		t.Fatal(err)
	}
	b, err := r.ResolveForWorkflowStep(context.Background(), "c1", "inst-1", "step-b")
	if err != nil {
		t.Fatal(err)
	}
	if len(a) != 1 || a[0] != "a@co.com" {
		t.Fatalf("step-a=%v", a)
	}
	if len(b) != 1 || b[0] != "b@co.com" {
		t.Fatalf("step-b=%v", b)
	}
}
