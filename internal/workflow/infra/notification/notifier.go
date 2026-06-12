package notification

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"

	notifapp "github.com/cobo/cobo_iam_services/internal/notification/app"
)

type MembershipEmailLookup interface {
	LookupCreatorEmail(ctx context.Context, companyID, recordID string) (email, title, companyName string, err error)
}

type WorkflowNotifier struct {
	delivery            notifapp.DeliveryAdapter
	registry            notifapp.TemplateRegistry
	renderer            notifapp.EmailRenderer
	notificationService *notifapp.EmailNotificationService
	membershipLookup    MembershipEmailLookup
	portalURL           string
	outboxEnabled       bool
	log                 *slog.Logger
}

func NewWorkflowNotifier(
	delivery notifapp.DeliveryAdapter,
	registry notifapp.TemplateRegistry,
	renderer notifapp.EmailRenderer,
	notificationService *notifapp.EmailNotificationService,
	membershipLookup MembershipEmailLookup,
	portalURL string,
	outboxEnabled bool,
	log *slog.Logger,
) *WorkflowNotifier {
	if log == nil {
		log = slog.Default()
	}
	return &WorkflowNotifier{
		delivery:            delivery,
		registry:            registry,
		renderer:            renderer,
		notificationService: notificationService,
		membershipLookup:    membershipLookup,
		portalURL:           strings.TrimRight(portalURL, "/"),
		outboxEnabled:       outboxEnabled,
		log:                 log,
	}
}

func (n *WorkflowNotifier) NotifyWorkflowApproved(ctx context.Context, companyID, recordID, workflowInstanceID, actorMembershipID string) error {
	if n.membershipLookup == nil {
		return nil
	}
	email, title, companyName, err := n.membershipLookup.LookupCreatorEmail(ctx, companyID, recordID)
	if err != nil || email == "" {
		return err
	}
	vars := map[string]any{
		"disclosure_title":       title,
		"company_name":           companyName,
		"record_id":              recordID,
		"portal_url":             n.portalURL,
		"workflow_instance_id":   workflowInstanceID,
	}
	n.sendEmail(ctx, email, "workflow.approved", vars, recordID, actorMembershipID, companyID)
	return nil
}

func (n *WorkflowNotifier) sendEmail(ctx context.Context, to, templateKey string, vars map[string]any, recordID, actorMembershipID, companyID string) {
	if n.outboxEnabled && n.notificationService != nil {
		idempotencyKey := fmt.Sprintf("workflow.approved.%s.%s", recordID, actorMembershipID)
		if _, err := n.notificationService.DispatchEmail(ctx, notifapp.DispatchEmailRequest{
			To:                  to,
			TemplateKey:         templateKey,
			Locale:              "vi",
			Variables:           vars,
			IdempotencyKey:      idempotencyKey,
			TriggeredByUserID:   "system",
			SourceEventType:     "workflow.approved",
			SourceAggregateType: "disclosure_record",
			SourceAggregateID:   recordID,
			CompanyID:           companyID,
		}); err != nil {
			n.log.Warn("workflow notifier: durable dispatch failed",
				slog.String("template_key", templateKey),
				slog.String("record_id", recordID),
				slog.String("err", err.Error()))
		}
		return
	}
	if n.delivery == nil || n.registry == nil || n.renderer == nil {
		return
	}
	resolved, err := n.registry.Resolve(ctx, templateKey, "vi")
	if err != nil {
		n.log.Warn("workflow notifier: resolve template failed", slog.String("err", err.Error()))
		return
	}
	rendered, err := n.renderer.Render(resolved, vars)
	if err != nil {
		n.log.Warn("workflow notifier: render template failed", slog.String("err", err.Error()))
		return
	}
	if _, err := n.delivery.Send(ctx, notifapp.DeliveryMessage{
		NotificationID: fmt.Sprintf("workflow.approved.%s", recordID),
		To:             to,
		Subject:        rendered.Subject,
		TextBody:       rendered.TextBody,
		HTMLBody:       rendered.HTMLBody,
	}); err != nil {
		n.log.Warn("workflow notifier: send email failed", slog.String("err", err.Error()))
	}
}

type SQLMembershipLookup struct {
	DB *sql.DB
}

func (q *SQLMembershipLookup) LookupCreatorEmail(ctx context.Context, companyID, recordID string) (string, string, string, error) {
	const query = `
SELECT COALESCE(u.email, ''), COALESCE(dr.title, ''), COALESCE(c.company_name, '')
FROM disclosure_records dr
JOIN companies c ON c.company_id = dr.company_id
LEFT JOIN users u ON u.user_id = dr.created_by
WHERE dr.company_id = ? AND dr.record_id = ?
LIMIT 1`
	var email, title, companyName string
	if err := q.DB.QueryRowContext(ctx, query, companyID, recordID).Scan(&email, &title, &companyName); err != nil {
		return "", "", "", err
	}
	return email, title, companyName, nil
}
