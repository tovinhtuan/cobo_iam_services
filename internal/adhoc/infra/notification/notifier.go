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
// Emails are rendered from templates and dispatched directly via DeliveryAdapter (SMTP).
type AdhocProposalNotifier struct {
	inApp     inappapp.InAppNotifier
	delivery  notifapp.DeliveryAdapter
	registry  notifapp.TemplateRegistry
	renderer  notifapp.EmailRenderer
	portalURL string
	log       *slog.Logger
}

func New(
	inApp inappapp.InAppNotifier,
	delivery notifapp.DeliveryAdapter,
	registry notifapp.TemplateRegistry,
	renderer notifapp.EmailRenderer,
	portalURL string,
	log *slog.Logger,
) *AdhocProposalNotifier {
	if log == nil {
		log = slog.Default()
	}
	return &AdhocProposalNotifier{
		inApp:     inApp,
		delivery:  delivery,
		registry:  registry,
		renderer:  renderer,
		portalURL: portalURL,
		log:       log,
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
				n.focalReviewVars(proposal, f), proposal.ProposalID, "focal")
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
			n.controllerReviewVars(proposal, controller), proposal.ProposalID, "controller")
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
			n.approvedVars(proposal), proposal.ProposalID, "approved")
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
			n.rejectedVars(proposal), proposal.ProposalID, "rejected")
	}
}

// sendEmail resolves the template, renders it, and sends via DeliveryAdapter.
func (n *AdhocProposalNotifier) sendEmail(ctx context.Context, to, templateKey string, vars map[string]any, proposalID, label string) {
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
