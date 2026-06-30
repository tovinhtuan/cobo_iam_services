package app

import (
	"context"
	"testing"
	"time"
)

func testDispatchDecisionDeps(reader NotificationRulesReader, emailEnabled bool) DispatchDecisionDeps {
	doc := validPrefsDocNoPolicies("c1", emailEnabled)
	if reader == nil {
		reader = &fakeNotificationRulesReader{byCompany: map[string]*AlertChannelPrefsDocument{"c1": doc}}
	}
	return DispatchDecisionDeps{
		EvaluatorRuntime:  NewNotificationRulesEvaluator(reader, true),
		EvaluatorSimulate: NewNotificationRulesEvaluator(reader, true),
	}
}

func TestSimulateDispatchDecision_RuleMissing(t *testing.T) {
	sim := NewDispatchSimulator(DispatchDecisionDeps{
		EvaluatorSimulate: NewNotificationRulesEvaluator(&fakeNotificationRulesReader{}, true),
	})
	now := time.Now().UTC()
	res, err := sim.SimulateDispatchDecision(context.Background(), SimulateDispatchInput{
		CompanyID:       "c1",
		EventType:       "deadline",
		ScheduledAt:     now,
		DueAt:           now.Add(7 * 24 * time.Hour),
		ScopeType:       ScopeTypeDisclosure,
		ScopeID:         "disc-1",
		RecipientEmails: []string{"a@example.com"},
	})
	if err != nil {
		t.Fatalf("SimulateDispatchDecision: %v", err)
	}
	if !res.WouldSkip || res.ReasonCode != SkipReasonRuleMissing {
		t.Fatalf("would_skip=%v reason=%q", res.WouldSkip, res.ReasonCode)
	}
	if len(res.Trace) < 2 {
		t.Fatalf("expected trace steps, got %d", len(res.Trace))
	}
}

func TestSimulateDispatchDecision_ChannelDisabled(t *testing.T) {
	reader := &fakeNotificationRulesReader{byCompany: map[string]*AlertChannelPrefsDocument{
		"c1": validPrefsDoc("c1", false),
	}}
	sim := NewDispatchSimulator(testDispatchDecisionDeps(reader, false))
	now := time.Now().UTC()
	res, err := sim.SimulateDispatchDecision(context.Background(), SimulateDispatchInput{
		CompanyID:       "c1",
		EventType:       "deadline",
		ScheduledAt:     now,
		DueAt:           now.Add(7 * 24 * time.Hour),
		ScopeType:       ScopeTypeDisclosure,
		ScopeID:         "disc-1",
		RecipientEmails: []string{"a@example.com"},
	})
	if err != nil {
		t.Fatalf("SimulateDispatchDecision: %v", err)
	}
	if !res.WouldSkip || res.ReasonCode != SkipReasonChannelDisabled {
		t.Fatalf("reason=%q", res.ReasonCode)
	}
	if len(res.Trace) < 3 {
		t.Fatalf("trace too short: %+v", res.Trace)
	}
}

func TestSimulateDispatchDecision_WouldSend_ParityWithPrepareDispatch(t *testing.T) {
	alertRepo := &fakeAlertConfigRepoForDispatch{rows: []AlertTemplateConfig{
		{TypeID: "dt1", AlertKind: string(AlertKindDeadline), Enabled: true, TemplateKey: "tpl"},
	}}
	reader := &fakeNotificationRulesReader{byCompany: map[string]*AlertChannelPrefsDocument{
		"c1": validPrefsDocNoPolicies("c1", true),
	}}
	deps := testDispatchDecisionDeps(reader, true)
	deps.AlertConfigRepo = alertRepo
	sim := NewDispatchSimulator(deps)
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
	runtimeOut := svc.prepareDispatch(context.Background(), c, now)
	simRes, err := sim.SimulateDispatchDecision(context.Background(), SimulateDispatchInput{
		CompanyID:        "c1",
		EventType:        "deadline",
		ScheduledAt:      now,
		DueAt:            now.Add(7 * 24 * time.Hour),
		ScopeType:        ScopeTypeDisclosure,
		ScopeID:          "disc-1",
		DisclosureTypeID: "dt1",
		RecipientEmails:  []string{"a@example.com"},
	})
	if err != nil {
		t.Fatalf("simulate: %v", err)
	}
	if runtimeOut.skip != simRes.WouldSkip {
		t.Fatalf("parity skip runtime=%v simulate=%v reason runtime=%q sim=%q", runtimeOut.skip, simRes.WouldSkip, runtimeOut.skipReason, simRes.ReasonCode)
	}
	if !runtimeOut.skip && runtimeOut.templateCode != simRes.TemplateKey {
		t.Fatalf("template parity runtime=%q sim=%q", runtimeOut.templateCode, simRes.TemplateKey)
	}
	if !simRes.WouldSend {
		t.Fatalf("expected would_send")
	}
}

func TestSimulateDispatchDecision_SkipParity_ScheduleMismatch(t *testing.T) {
	reader := &fakeNotificationRulesReader{byCompany: map[string]*AlertChannelPrefsDocument{
		"c1": validPrefsDoc("c1", true),
	}}
	deps := testDispatchDecisionDeps(reader, true)
	sim := NewDispatchSimulator(deps)
	eval := NewNotificationRulesEvaluator(reader, true)
	svc := NewService(nil, nil, nil, WithNotificationRulesFoundation(reader, eval)).(*service)
	now := time.Now().UTC()
	c := DispatchCandidate{
		CompanyID:       "c1",
		RecipientEmails: []string{"a@example.com"},
		ScheduledAt:     now,
		DeadlineAt:      now.Add(3 * 24 * time.Hour),
	}
	runtimeOut := svc.prepareDispatch(context.Background(), c, now)
	simRes, err := sim.SimulateDispatchDecision(context.Background(), SimulateDispatchInput{
		CompanyID:       "c1",
		EventType:       "deadline",
		ScheduledAt:     now,
		DueAt:           now.Add(3 * 24 * time.Hour),
		ScopeType:       ScopeTypeDisclosure,
		ScopeID:         "disc-1",
		RecipientEmails: []string{"a@example.com"},
	})
	if err != nil {
		t.Fatalf("simulate: %v", err)
	}
	if runtimeOut.skipReason != simRes.ReasonCode {
		t.Fatalf("reason parity runtime=%q sim=%q", runtimeOut.skipReason, simRes.ReasonCode)
	}
}

func TestMaskEmailSamples(t *testing.T) {
	masked := maskEmailSamples([]string{"alice@example.com", "bob@test.org"}, 2)
	if len(masked) != 2 || masked[0] != "a***@example.com" {
		t.Fatalf("masked=%v", masked)
	}
}

func TestSimulateUsesLayer1RegardlessOfRuntimeFlag(t *testing.T) {
	reader := &fakeNotificationRulesReader{byCompany: map[string]*AlertChannelPrefsDocument{
		"c1": validPrefsDocNoPolicies("c1", true),
	}}
	deps := DispatchDecisionDeps{
		EvaluatorRuntime:  NewNotificationRulesEvaluator(reader, false),
		EvaluatorSimulate: NewNotificationRulesEvaluator(reader, true),
	}
	sim := NewDispatchSimulator(deps)
	now := time.Now().UTC()
	res, err := sim.SimulateDispatchDecision(context.Background(), SimulateDispatchInput{
		CompanyID:       "c1",
		EventType:       "deadline",
		ScheduledAt:     now,
		DueAt:           now.Add(7 * 24 * time.Hour),
		ScopeType:       ScopeTypeDisclosure,
		ScopeID:         "disc-1",
		RecipientEmails: []string{"a@example.com"},
	})
	if err != nil {
		t.Fatalf("simulate: %v", err)
	}
	if res.WouldSkip {
		t.Fatalf("simulate should apply layer1 even when runtime consumer off, reason=%q", res.ReasonCode)
	}
}
