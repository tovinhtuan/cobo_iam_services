package app

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"time"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/events"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
	"github.com/cobo/cobo_iam_services/internal/platform/outbox"
	outboxmysql "github.com/cobo/cobo_iam_services/internal/platform/outbox/mysql"
)

type service struct {
	repo      Repository
	auth      authapp.Service
	idg       idgen.Generator
	outbox    outbox.Publisher
	sqlDB     *sql.DB
	outboxSQL *outboxmysql.Repository
}

// ServiceOption configures notification.service (e.g. transactional enqueue with MySQL outbox).
type ServiceOption func(*service)

// WithTransactionalEnqueue commits notification_jobs + outbox_events in one DB transaction when repo implements TxJobRepository.
func WithTransactionalEnqueue(db *sql.DB, ob *outboxmysql.Repository) ServiceOption {
	return func(s *service) {
		s.sqlDB = db
		s.outboxSQL = ob
	}
}

func NewService(repo Repository, auth authapp.Service, idg idgen.Generator, pub outbox.Publisher, opts ...ServiceOption) Service {
	s := &service{repo: repo, auth: auth, idg: idg, outbox: pub}
	for _, o := range opts {
		if o != nil {
			o(s)
		}
	}
	return s
}

func (s *service) ResolveRecipients(ctx context.Context, req ResolveRecipientsRequest) (*ResolveRecipientsResponse, error) {
	if err := s.authorize(ctx, req.Subject, "notification.resolve_recipients", req.ResourceID); err != nil {
		return nil, err
	}
	// Skeleton strategy: current membership as fallback recipient.
	return &ResolveRecipientsResponse{Recipients: []string{req.Subject.MembershipID}}, nil
}

func (s *service) EnqueueNotification(ctx context.Context, req EnqueueNotificationRequest) (*NotificationJobDTO, error) {
	if strings.TrimSpace(req.EventType) == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "event_type is required", nil)
	}
	if err := s.authorize(ctx, req.Subject, "notification.enqueue", req.ResourceID); err != nil {
		return nil, err
	}
	job := NotificationJobDTO{NotificationJobID: s.idg.NewUUID(), CompanyID: req.Subject.CompanyID, EventType: req.EventType, ResourceType: req.ResourceType, ResourceID: req.ResourceID, Payload: req.Payload, Status: "pending"}

	if s.sqlDB != nil && s.outboxSQL != nil {
		if tjr, ok := s.repo.(TxJobRepository); ok {
			tx, err := s.sqlDB.BeginTx(ctx, nil)
			if err != nil {
				return nil, fmt.Errorf("begin notification tx: %w", err)
			}
			defer func() { _ = tx.Rollback() }()
			created, err := tjr.CreateJobTx(ctx, tx, job)
			if err != nil {
				return nil, err
			}
			if err := s.outboxSQL.PublishEventTx(ctx, tx, toOutboxEvent(*created, s.idg.NewUUID())); err != nil {
				return nil, err
			}
			if err := tx.Commit(); err != nil {
				return nil, fmt.Errorf("commit notification tx: %w", err)
			}
			return created, nil
		}
	}

	created, err := s.repo.CreateJob(ctx, job)
	if err != nil {
		return nil, err
	}
	_ = s.outbox.Publish(ctx, toOutboxEvent(*created, s.idg.NewUUID()))
	return created, nil
}

func (s *service) DispatchPending(ctx context.Context, req DispatchPendingRequest) (*DispatchPendingResponse, error) {
	if err := s.authorize(ctx, req.Subject, "notification.dispatch", ""); err != nil {
		return nil, err
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 50
	}
	jobs, err := s.repo.ListPendingJobs(ctx, req.Subject.CompanyID, limit)
	if err != nil {
		return nil, err
	}
	dispatched := 0
	for _, j := range jobs {
		recipients, _ := s.ResolveRecipients(ctx, ResolveRecipientsRequest{Subject: req.Subject, EventType: j.EventType, ResourceType: j.ResourceType, ResourceID: j.ResourceID})
		for _, r := range recipients.Recipients {
			_, _ = s.repo.CreateDelivery(ctx, NotificationDeliveryDTO{NotificationDeliveryID: s.idg.NewUUID(), NotificationJobID: j.NotificationJobID, Recipient: r, Status: "sent"})
		}
		_ = s.repo.UpdateJobStatus(ctx, j.CompanyID, j.NotificationJobID, "dispatched")
		dispatched++
	}
	return &DispatchPendingResponse{Dispatched: dispatched}, nil
}

func (s *service) authorize(ctx context.Context, sub Subject, action, resourceID string) error {
	decision, err := s.auth.Authorize(ctx, authapp.AuthorizeRequest{Subject: authapp.SubjectRef{UserID: sub.UserID, MembershipID: sub.MembershipID, CompanyID: sub.CompanyID}, Action: action, Resource: authapp.ResourceRef{Type: "notification_job", ID: resourceID, Attributes: map[string]any{}}})
	if err != nil {
		return fmt.Errorf("authorize notification action: %w", err)
	}
	if decision.Decision != authapp.DecisionAllow {
		code := perr.CodePermissionDenied
		if decision.DenyReasonCode != nil {
			code = *decision.DenyReasonCode
		}
		return perr.NewHTTPError(http.StatusForbidden, code, "access denied", nil)
	}
	return nil
}

func toOutboxEvent(job NotificationJobDTO, eventID string) events.Event {
	return events.Event{EventID: eventID, AggregateType: "notification_job", AggregateID: job.NotificationJobID, EventType: "notification.dispatch", Payload: map[string]any{"company_id": job.CompanyID, "job_id": job.NotificationJobID}, OccurredAt: time.Now().UTC()}
}
