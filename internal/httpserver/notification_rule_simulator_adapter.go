package httpserver

import (
	"context"

	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	reminderapp "github.com/cobo/cobo_iam_services/internal/reminder/app"
)

type reminderDispatchSimulatorAdapter struct {
	sim *reminderapp.DispatchSimulator
}

func newReminderDispatchSimulatorAdapter(sim *reminderapp.DispatchSimulator) caapp.NotificationDispatchSimulator {
	if sim == nil {
		return nil
	}
	return &reminderDispatchSimulatorAdapter{sim: sim}
}

func (a *reminderDispatchSimulatorAdapter) SimulateDispatch(ctx context.Context, in caapp.NotificationDispatchSimulateInput) (*caapp.NotificationDispatchSimulateResult, error) {
	res, err := a.sim.SimulateDispatchDecision(ctx, reminderapp.SimulateDispatchInput{
		CompanyID:          in.CompanyID,
		EventType:          in.EventType,
		Channel:            in.Channel,
		DueAt:              in.DueAt,
		ScheduledAt:        in.ScheduledAt,
		AsOf:               in.AsOf,
		ScopeType:          reminderapp.ScopeType(in.ScopeType),
		ScopeID:            in.ScopeID,
		DisclosureTypeID:   in.DisclosureTypeID,
		WorkflowInstanceID: in.WorkflowInstanceID,
		RecipientEmails:    in.RecipientEmails,
		TemplateCode:       in.TemplateCode,
		SimulationID:       in.SimulationID,
		SubscriptionTier:   in.SubscriptionTier,
	})
	if err != nil {
		return nil, err
	}
	out := &caapp.NotificationDispatchSimulateResult{
		SimulationID:    res.SimulationID,
		WouldSend:       res.WouldSend,
		WouldSkip:       res.WouldSkip,
		Outcome:         res.Outcome,
		ReasonCode:      res.ReasonCode,
		Channel:         res.Channel,
		DispatchPath:    res.DispatchPath,
		MatchedRuleCode: res.MatchedRuleCode,
		TemplateKey:     res.TemplateKey,
		Warnings:        append([]string(nil), res.Warnings...),
		EvaluatedAt:     res.EvaluatedAt,
		RecipientSummary: caapp.NotificationDispatchRecipientSummary{
			Count:         res.RecipientSummary.Count,
			MaskedSamples: append([]string(nil), res.RecipientSummary.MaskedSamples...),
			PolicyApplied: res.RecipientSummary.PolicyApplied,
		},
	}
	for _, m := range res.MatchedSchedules {
		out.MatchedSchedules = append(out.MatchedSchedules, caapp.NotificationDispatchScheduleMatch{
			OffsetDays:  m.OffsetDays,
			Kind:        m.Kind,
			PremiumOnly: m.PremiumOnly,
		})
	}
	for _, t := range res.Trace {
		out.Trace = append(out.Trace, caapp.NotificationDispatchTraceStep{
			Step:     t.Step,
			Status:   t.Status,
			Detail:   t.Detail,
			Code:     t.Code,
			Metadata: t.Metadata,
		})
	}
	return out, nil
}
