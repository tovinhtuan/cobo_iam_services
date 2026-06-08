package notification

import (
	"context"
	"fmt"
	"log/slog"

	adhocapp "github.com/cobo/cobo_iam_services/internal/adhoc/app"
	inappapp "github.com/cobo/cobo_iam_services/internal/inappnotification/app"
	notifapp "github.com/cobo/cobo_iam_services/internal/notification/app"
)

// AdhocProposalNotifier implements adhocapp.ProposalNotifier.
// In-app notifications are created immediately.
//
// Email dispatch is routed by two flags (Batch 2 cutover):
//   - both off:                  legacy DeliveryAdapter.Send only (unchanged default behavior)
//   - shadowMode on, outbox off: legacy sends for real (system of record) AND the
//     durable EmailNotificationService pipeline runs end-to-end with the recipient
//     rewritten to shadowRecipient, so real users never receive a duplicate
//   - outboxEnabled on:          durable pipeline is the sole authoritative sender
//     (real recipient); legacy is not invoked regardless of shadowMode
type AdhocProposalNotifier struct {
	inApp               inappapp.InAppNotifier
	delivery            notifapp.DeliveryAdapter
	registry            notifapp.TemplateRegistry
	renderer            notifapp.EmailRenderer
	notificationService *notifapp.EmailNotificationService
	portalURL           string
	shadowMode          bool
	outboxEnabled       bool
	shadowRecipient     string
	metrics             adhocapp.Metrics
	log                 *slog.Logger
}

func New(
	inApp inappapp.InAppNotifier,
	delivery notifapp.DeliveryAdapter,
	registry notifapp.TemplateRegistry,
	renderer notifapp.EmailRenderer,
	notificationService *notifapp.EmailNotificationService,
	portalURL string,
	shadowMode bool,
	outboxEnabled bool,
	shadowRecipient string,
	metrics adhocapp.Metrics,
	log *slog.Logger,
) *AdhocProposalNotifier {
	if log == nil {
		log = slog.Default()
	}
	if metrics == nil {
		metrics = adhocapp.NewNoopMetrics()
	}
	if shadowMode && !outboxEnabled && shadowRecipient == "" {
		log.Warn("adhoc notifier: EMAIL_SHADOW_MODE is enabled but ADHOC_EMAIL_SHADOW_RECIPIENT is empty — durable shadow dispatch will be skipped (legacy path continues unaffected)")
	}
	// R-1: warn immediately at construction when outbox is the sole delivery path
	// but the backing service was never wired (pool=nil or outboxSQL=nil at startup).
	// sendEmail will fall back to legacy in this case — the warning here signals the
	// misconfiguration so an operator does not need to wait for the first proposal
	// action to discover emails are not flowing through the durable pipeline.
	if outboxEnabled && notificationService == nil {
		log.Warn("adhoc email outbox enabled but notification service is nil; durable adhoc email dispatch disabled — check MYSQL_DSN is set and DB is reachable at startup")
	}
	return &AdhocProposalNotifier{
		inApp:               inApp,
		delivery:            delivery,
		registry:            registry,
		renderer:            renderer,
		notificationService: notificationService,
		portalURL:           portalURL,
		shadowMode:          shadowMode,
		outboxEnabled:       outboxEnabled,
		shadowRecipient:     shadowRecipient,
		metrics:             metrics,
		log:                 log,
	}
}

func (n *AdhocProposalNotifier) NotifyFocalsForReview(ctx context.Context, proposal adhocapp.ProposalDTO, focals []adhocapp.MemberInfo) {
	resType := inappapp.ResourceTypeAdHocProposal
	resID := proposal.ProposalID
	title := "Đề xuất cần duyệt"
	body := shortNote(proposal.ChangeNote)
	for _, f := range focals {
		if f.UserID != "" {
			if err := n.inApp.CreateForUser(ctx, f.UserID, proposal.CompanyID,
				inappapp.KindAdhocFocalReviewRequested, title, body, &resType, &resID); err != nil {
				n.log.Warn("adhoc notifier: create in-app (focal) failed",
					slog.String("proposal_id", proposal.ProposalID),
					slog.String("user_id", f.UserID),
					slog.String("err", err.Error()))
			}
		}
		if f.Email != "" {
			n.sendEmail(ctx, f.Email, "adhoc.focal_review_requested",
				n.focalReviewVars(proposal, f), proposal.ProposalID, "focal",
				"focal_review_requested", f.MembershipID, proposal.CompanyID)
		}
	}
}

func (n *AdhocProposalNotifier) NotifyControllerForReview(ctx context.Context, proposal adhocapp.ProposalDTO, controller adhocapp.MemberInfo) {
	resType := inappapp.ResourceTypeAdHocProposal
	resID := proposal.ProposalID
	title := "Đề xuất cần phê duyệt cuối"
	body := shortNote(proposal.ChangeNote)
	if controller.UserID != "" {
		if err := n.inApp.CreateForUser(ctx, controller.UserID, proposal.CompanyID,
			inappapp.KindAdhocControllerReviewRequested, title, body, &resType, &resID); err != nil {
			n.log.Warn("adhoc notifier: create in-app (controller) failed",
				slog.String("proposal_id", proposal.ProposalID),
				slog.String("user_id", controller.UserID),
				slog.String("err", err.Error()))
		}
	}
	if controller.Email != "" {
		n.sendEmail(ctx, controller.Email, "adhoc.controller_review_requested",
			n.controllerReviewVars(proposal, controller), proposal.ProposalID, "controller",
			"controller_review_requested", controller.MembershipID, proposal.CompanyID)
	}
}

func (n *AdhocProposalNotifier) NotifyCreatorApproved(ctx context.Context, proposal adhocapp.ProposalDTO, creator adhocapp.MemberInfo) {
	resType := inappapp.ResourceTypeAdHocProposal
	resID := proposal.ProposalID
	title := "Đề xuất đã được phê duyệt"
	body := shortNote(proposal.ChangeNote)
	if creator.UserID != "" {
		if err := n.inApp.CreateForUser(ctx, creator.UserID, proposal.CompanyID,
			inappapp.KindAdhocProposalApproved, title, body, &resType, &resID); err != nil {
			n.log.Warn("adhoc notifier: create in-app (approved) failed",
				slog.String("proposal_id", proposal.ProposalID),
				slog.String("user_id", creator.UserID),
				slog.String("err", err.Error()))
		}
	}
	if creator.Email != "" {
		n.sendEmail(ctx, creator.Email, "adhoc.proposal_approved",
			n.approvedVars(proposal), proposal.ProposalID, "approved",
			"proposal_approved", creator.MembershipID, proposal.CompanyID)
	}
}

func (n *AdhocProposalNotifier) NotifyCreatorRejected(ctx context.Context, proposal adhocapp.ProposalDTO, creator adhocapp.MemberInfo) {
	resType := inappapp.ResourceTypeAdHocProposal
	resID := proposal.ProposalID
	title := "Đề xuất bị từ chối"
	body := shortNote(proposal.ChangeNote)
	if creator.UserID != "" {
		if err := n.inApp.CreateForUser(ctx, creator.UserID, proposal.CompanyID,
			inappapp.KindAdhocProposalRejected, title, body, &resType, &resID); err != nil {
			n.log.Warn("adhoc notifier: create in-app (rejected) failed",
				slog.String("proposal_id", proposal.ProposalID),
				slog.String("user_id", creator.UserID),
				slog.String("err", err.Error()))
		}
	}
	if creator.Email != "" {
		n.sendEmail(ctx, creator.Email, "adhoc.proposal_rejected",
			n.rejectedVars(proposal), proposal.ProposalID, "rejected",
			"proposal_rejected", creator.MembershipID, proposal.CompanyID)
	}
}

// sendEmail routes the email per the Batch 2 cutover flags:
//   - outboxEnabled: durable pipeline only, real recipient (sole authoritative sender)
//   - shadowMode (outbox off): legacy sends to the real recipient (system of record,
//     unchanged) AND the durable pipeline runs end-to-end against shadowRecipient
//     instead of `to`, so real users never receive a duplicate
//   - neither flag: legacy only — byte-identical to pre-Batch-2 behavior
func (n *AdhocProposalNotifier) sendEmail(ctx context.Context, to, templateKey string, vars map[string]any, proposalID, label, eventType, recipientMembershipID, companyID string) {
	switch {
	case n.outboxEnabled:
		// R-1: if the durable service was not wired (no DB at startup), fall back to
		// the legacy path so the email is delivered rather than silently dropped.
		if n.notificationService == nil {
			n.log.Warn("adhoc notifier: outbox enabled but notification service unavailable — falling back to legacy email delivery",
				slog.String("template_key", templateKey),
				slog.String("proposal_id", proposalID))
			n.sendLegacyEmail(ctx, to, templateKey, vars, proposalID, label)
			return
		}
		_, _ = n.dispatchDurable(ctx, to, templateKey, vars, proposalID, eventType, recipientMembershipID, companyID)
	case n.shadowMode:
		n.sendLegacyEmail(ctx, to, templateKey, vars, proposalID, label)
		if n.shadowRecipient != "" {
			notif, err := n.dispatchDurable(ctx, n.shadowRecipient, templateKey, vars, proposalID, eventType, recipientMembershipID, companyID)
			if err == nil && notif != nil {
				// Compare fields that are NEVER rewritten by the shadow-recipient
				// substitution (proposalID -> SourceAggregateID, templateKey) — the
				// recipient itself is intentionally rewritten to shadowRecipient and
				// is therefore not a valid collision signal here. A mismatch means
				// the computed idempotency key resolved to a *pre-existing* record
				// from a *different* notification intent — the in-app manifestation
				// of the COUNT(*) GROUP BY idempotency_key HAVING COUNT(*) > 1 > 0
				// condition §AK.5 L657 defines (made unobservable as a row-count by
				// email_notifications' own idempotency_key UNIQUE constraint).
				outcome := "match"
				if notif.SourceAggregateID != proposalID || notif.TemplateKey != templateKey {
					outcome = "mismatch"
				}
				n.metrics.RecordEmailShadowOutcome(companyID, outcome)
			}
		}
	default:
		n.sendLegacyEmail(ctx, to, templateKey, vars, proposalID, label)
	}
}

// sendLegacyEmail resolves the template, renders it, and sends via DeliveryAdapter.
// This is byte-identical to the pre-Batch-2 sendEmail body — the legacy path must
// remain unchanged regardless of which routing branch invokes it.
func (n *AdhocProposalNotifier) sendLegacyEmail(ctx context.Context, to, templateKey string, vars map[string]any, proposalID, label string) {
	if n.delivery == nil || n.registry == nil || n.renderer == nil {
		return
	}
	resolved, err := n.registry.Resolve(ctx, templateKey, "vi")
	if err != nil {
		n.log.Warn("adhoc notifier: resolve template failed",
			slog.String("template_key", templateKey),
			slog.String("err", err.Error()))
		return
	}
	rendered, err := n.renderer.Render(resolved, vars)
	if err != nil {
		n.log.Warn("adhoc notifier: render template failed",
			slog.String("template_key", templateKey),
			slog.String("err", err.Error()))
		return
	}
	if _, err := n.delivery.Send(ctx, notifapp.DeliveryMessage{
		NotificationID: fmt.Sprintf("adhoc.%s.%s", label, proposalID),
		To:             to,
		Subject:        rendered.Subject,
		TextBody:       rendered.TextBody,
		HTMLBody:       rendered.HTMLBody,
	}); err != nil {
		n.log.Warn("adhoc notifier: send email failed",
			slog.String("template_key", templateKey),
			slog.String("recipient", to),
			slog.String("err", err.Error()))
	}
}

// dispatchDurable enqueues the email through the durable EmailNotificationService
// pipeline (transactional publish -> outbox -> worker -> EmailDispatchHandler ->
// real DeliveryAdapter — proven end-to-end by Batch 2A, untouched here). It
// returns the persisted record so the Shadow Mode caller can compare it against
// the request and emit cobo_adhoc_email_shadow_total (§AK.5 L657).
//
// `to` is the recipient the message is actually addressed to: the real recipient
// when outboxEnabled (Cases C/D), or the configured shadow sink when running the
// shadow comparison (Case B) — the rewrite happens at the call site in sendEmail,
// never here, and never touches recipientMembershipID or the idempotency key, so
// the audit trail continues to identify the real intended recipient.
func (n *AdhocProposalNotifier) dispatchDurable(ctx context.Context, to, templateKey string, vars map[string]any, proposalID, eventType, recipientMembershipID, companyID string) (*notifapp.EmailNotification, error) {
	if n.notificationService == nil {
		return nil, nil
	}
	idempotencyKey := fmt.Sprintf("adhoc.%s.%s.%s", eventType, proposalID, recipientMembershipID)
	notif, err := n.notificationService.DispatchEmail(ctx, notifapp.DispatchEmailRequest{
		To:                  to,
		TemplateKey:         templateKey,
		Locale:              "vi",
		Variables:           vars,
		IdempotencyKey:      idempotencyKey,
		TriggeredByUserID:   "system",
		SourceEventType:     "adhoc." + eventType,
		SourceAggregateType: "ad_hoc_proposal",
		SourceAggregateID:   proposalID,
		CompanyID:           companyID,
	})
	if err != nil {
		n.log.Warn("adhoc notifier: durable dispatch failed",
			slog.String("template_key", templateKey),
			slog.String("idempotency_key", idempotencyKey),
			slog.String("err", err.Error()))
		return nil, err
	}
	return notif, nil
}

func (n *AdhocProposalNotifier) focalReviewVars(p adhocapp.ProposalDTO, creator adhocapp.MemberInfo) map[string]any {
	return map[string]any{
		"proposal_id":  p.ProposalID,
		"change_note":  p.ChangeNote,
		"company_name": p.CompanyID,
		"creator_name": creator.FullName,
		"portal_url":   n.portalURL,
	}
}

func (n *AdhocProposalNotifier) controllerReviewVars(p adhocapp.ProposalDTO, creator adhocapp.MemberInfo) map[string]any {
	return map[string]any{
		"proposal_id":  p.ProposalID,
		"change_note":  p.ChangeNote,
		"company_name": p.CompanyID,
		"creator_name": creator.FullName,
		"portal_url":   n.portalURL,
	}
}

func (n *AdhocProposalNotifier) approvedVars(p adhocapp.ProposalDTO) map[string]any {
	return map[string]any{
		"proposal_id":  p.ProposalID,
		"change_note":  p.ChangeNote,
		"company_name": p.CompanyID,
		"record_id":    p.RecordID,
		"portal_url":   n.portalURL,
	}
}

func (n *AdhocProposalNotifier) rejectedVars(p adhocapp.ProposalDTO) map[string]any {
	return map[string]any{
		"proposal_id":   p.ProposalID,
		"change_note":   p.ChangeNote,
		"company_name":  p.CompanyID,
		"reject_reason": p.RejectReason,
		"portal_url":    n.portalURL,
	}
}

func shortNote(note string) string {
	if len(note) > 120 {
		return note[:117] + "..."
	}
	return note
}
