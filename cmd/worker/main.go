package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/smtp"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	adhocrecord "github.com/cobo/cobo_iam_services/internal/adhoc/infra/disclosure"
	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	disclosuremysql "github.com/cobo/cobo_iam_services/internal/disclosure/infra/mysql"
	notificationapp "github.com/cobo/cobo_iam_services/internal/notification/app"
	notificationmysql "github.com/cobo/cobo_iam_services/internal/notification/infra/mysql"
	notificationobserve "github.com/cobo/cobo_iam_services/internal/notification/infra/observe"
	notificationregistry "github.com/cobo/cobo_iam_services/internal/notification/infra/registry"
	notificationsmtp "github.com/cobo/cobo_iam_services/internal/notification/infra/smtp"
	"github.com/cobo/cobo_iam_services/internal/platform/config"
	"github.com/cobo/cobo_iam_services/internal/platform/db"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
	"github.com/cobo/cobo_iam_services/internal/platform/logger"
	platformoutbox "github.com/cobo/cobo_iam_services/internal/platform/outbox"
	outboxinmem "github.com/cobo/cobo_iam_services/internal/platform/outbox/inmemory"
	outboxmysql "github.com/cobo/cobo_iam_services/internal/platform/outbox/mysql"
	reminderapp "github.com/cobo/cobo_iam_services/internal/reminder/app"
	reminderemail "github.com/cobo/cobo_iam_services/internal/reminder/infra/email"
	remindermysql "github.com/cobo/cobo_iam_services/internal/reminder/infra/mysql"
	reminderobserve "github.com/cobo/cobo_iam_services/internal/reminder/infra/observe"
	"github.com/cobo/cobo_iam_services/internal/subscription/entitlement"
	workflowapp "github.com/cobo/cobo_iam_services/internal/workflow/app"
	workflowmysql "github.com/cobo/cobo_iam_services/internal/workflow/infra/mysql"
)

func main() {
	ctx := context.Background()
	cfg, err := config.Load()
	if err != nil {
		slog.Default().Error("config load failed", slog.String("err", err.Error()))
		os.Exit(1)
	}
	log := logger.New(cfg.LogLevel).With(
		slog.String("service", cfg.ServiceName+"-worker"),
		slog.String("env", cfg.Env),
	)

	var sqlDB *sql.DB
	if cfg.MySQLDSN != "" {
		sqlDB, err = db.OpenMySQL(ctx, cfg.MySQLDSN)
		if err != nil {
			log.Error("mysql connect failed", slog.String("err", err.Error()))
			os.Exit(1)
		}
		defer sqlDB.Close()
	} else {
		log.Warn("MYSQL_DSN empty; worker uses in-memory outbox (not shared with API)")
	}

	var outboxRepo platformoutbox.Repository
	if sqlDB != nil {
		outboxRepo = outboxmysql.NewRepository(sqlDB)
		log.Info("outbox using MySQL")
	} else {
		outboxRepo = outboxinmem.NewRepository()
		if err := outboxinmem.SeedBootstrapEvents(ctx, outboxRepo); err != nil {
			log.Warn("outbox seed skipped", slog.String("err", err.Error()))
		}
	}
	processor := platformoutbox.NewProcessor(outboxRepo, 50)
	processor.Register("notification.dispatch", platformoutbox.HandlerFunc(func(ctx context.Context, event platformoutbox.QueuedEvent) error {
		payload := map[string]any{}
		_ = json.Unmarshal(event.PayloadJSON, &payload)
		log.Info("dispatch notification event", slog.String("event_id", event.EventID), slog.String("event_type", event.EventType), slog.Any("payload", payload))
		return nil
	}))
	processor.Register("auth.password_reset_requested", platformoutbox.HandlerFunc(func(ctx context.Context, event platformoutbox.QueuedEvent) error {
		return deliverAuthEmailEvent(ctx, cfg, log, event)
	}))
	processor.Register("auth.admin_password_reset_requested", platformoutbox.HandlerFunc(func(ctx context.Context, event platformoutbox.QueuedEvent) error {
		return deliverAuthEmailEvent(ctx, cfg, log, event)
	}))
	processor.Register("auth.user_invitation_sent", platformoutbox.HandlerFunc(func(ctx context.Context, event platformoutbox.QueuedEvent) error {
		return deliverAuthEmailEvent(ctx, cfg, log, event)
	}))
	processor.Register("auth.email_verification_requested", platformoutbox.HandlerFunc(func(ctx context.Context, event platformoutbox.QueuedEvent) error {
		return deliverAuthEmailEvent(ctx, cfg, log, event)
	}))
	if sqlDB != nil {
		emailTemplateRegistry := notificationregistry.NewEmbedRegistry()
		emailRenderer := notificationapp.NewEmailRenderer()
		emailNotificationRepo := notificationmysql.NewEmailNotificationRepository(sqlDB)
		emailAttemptRepo := notificationmysql.NewEmailDeliveryAttemptRepository(sqlDB)
		emailDeliveryAdapter := notificationsmtp.NewAdapter(notificationsmtp.Config{
			Host: cfg.SMTPHost,
			Port: cfg.SMTPPort,
			User: cfg.SMTPUser,
			Pass: cfg.SMTPPassword,
			From: cfg.SMTPFrom,
		}, nil)
		emailDispatchHandler := notificationapp.NewEmailDispatchHandler(
			emailNotificationRepo,
			emailAttemptRepo,
			emailTemplateRegistry,
			emailRenderer,
			emailDeliveryAdapter,
			idgen.UUIDv7Generator{},
			nil,
			0,
		).WithMetrics(notificationobserve.NewPromDeliveryMetrics()).WithLogger(log)
		processor.Register(notificationapp.EmailDispatchOutboxEventType, platformoutbox.HandlerFunc(func(ctx context.Context, event platformoutbox.QueuedEvent) error {
			return emailDispatchHandler.Handle(ctx, event.PayloadJSON)
		}))
	}

	runCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	var (
		disclosureSvc   disclosureapp.Service
		periodicCreator disclosureapp.PeriodicRecordCreator
	)
	if sqlDB != nil && cfg.PeriodicSeedingEnabled {
		disclosureRepo := disclosuremysql.NewRepository(sqlDB)
		disclosureSvc = disclosureapp.NewService(
			disclosureRepo,
			nil, /* no auth: worker mode */
			idgen.UUIDv7Generator{},
			disclosureapp.WithTemplateApplicabilityStrictFilter(cfg.TemplateApplicabilityStrictFilter),
			disclosureapp.WithDeadlineEngineV2Shadow(cfg.DeadlineEngineV2Shadow),
			disclosureapp.WithLegalBasisStructuredWriteEnabled(cfg.LegalBasisStructuredWriteEnabled),
			disclosureapp.WithLegalBasisLegacyFallbackEnabled(cfg.LegalBasisLegacyFallbackEnabled),
			disclosureapp.WithLegalBasisDivergenceWarningEnabled(cfg.LegalBasisDivergenceWarningEnabled),
		)
		var workflowSvc workflowapp.Service
		if cfg.WorkflowSnapshotEnabled {
			workflowRepo := workflowmysql.NewRepository(sqlDB)
			workflowOpts := []workflowapp.ServiceOption{
				workflowapp.WithFlags(workflowapp.Flags{
					SnapshotEnabled: true,
					TimelineEnabled: cfg.WorkflowTimelineEnabled,
				}),
			}
			if cfg.WorkflowTimelineEnabled {
				workflowOpts = append(workflowOpts, workflowapp.WithMilestoneRepository(workflowmysql.NewMilestoneRepository(sqlDB)))
			}
			workflowSvc = workflowapp.NewService(workflowRepo, nil, idgen.UUIDv7Generator{}, workflowOpts...)
		}
		periodicCreator = adhocrecord.NewRecordCreatorAdapter(disclosureSvc, workflowSvc, workflowSvc != nil)
	}

	var reminderScheduler reminderapp.Service
	if sqlDB != nil {
		emailTemplateRegistry := notificationregistry.NewEmbedRegistry()
		emailRenderer := notificationapp.NewEmailRenderer()
		reminderRepo := remindermysql.NewRepository(sqlDB)
		alertCfgRepo := remindermysql.NewAlertConfigRepository(sqlDB)
		membershipQuerier := remindermysql.NewMembershipEmailQuerier(sqlDB)
		stepReader := remindermysql.NewGlobalWorkflowStepReader(sqlDB)
		recipientResolver := reminderapp.NewRecipientResolver(reminderRepo, stepReader, membershipQuerier, membershipQuerier, log)
		rulesReader := remindermysql.NewNotificationRulesReader(sqlDB)
		rulesEvaluator := reminderapp.NewNotificationRulesEvaluator(rulesReader, cfg.NotificationRulesConsumerEnabled)
		tierChecker := &entitlement.Checker{
			Enabled:            cfg.SubscriptionTierEnforcementEnabled,
			ResolveCompanyTier: entitlement.NewMySQLCompanyTierResolver(sqlDB),
		}
		reminderOpts := []reminderapp.ServiceOption{
			reminderapp.WithEmailSender(reminderemail.NewSMTPSender(reminderemail.SMTPConfig{
				Host: cfg.SMTPHost,
				Port: cfg.SMTPPort,
				User: cfg.SMTPUser,
				Pass: cfg.SMTPPassword,
				From: cfg.SMTPFrom,
			}, reminderemail.WithTemplateRendering(cfg.EmailTemplateSource, emailTemplateRegistry, emailRenderer))),
			reminderapp.WithAlertConfigRepo(alertCfgRepo),
			reminderapp.WithRecipientResolver(recipientResolver),
			reminderapp.WithRecipientPolicyDeps(membershipQuerier, membershipQuerier),
			reminderapp.WithStepReader(stepReader),
			reminderapp.WithPublicWebBaseURL(cfg.PublicWebBaseURL),
			reminderapp.WithMetrics(reminderobserve.NewPromMetrics()),
			reminderapp.WithAlertHook(reminderobserve.AlertLogger{Log: log}),
			reminderapp.WithDispatchLogger(log),
			reminderapp.WithNotificationRulesFoundation(rulesReader, rulesEvaluator),
			reminderapp.WithTierEnforcement(tierChecker),
		}
		if cfg.WorkflowRemindersEnabled {
			reminderOpts = append(reminderOpts, reminderapp.WithMilestoneScanner(remindermysql.NewMilestoneScanner(sqlDB)))
			log.Info("workflow milestone reminder bridge enabled")
		}
		reminderScheduler = reminderapp.NewService(reminderRepo, reminderRepo, reminderRepo, reminderOpts...)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		t := time.NewTicker(cfg.WorkerTickInterval)
		defer t.Stop()
		for {
			select {
			case <-runCtx.Done():
				return
			case <-t.C:
				tick(runCtx, log, sqlDB, processor, reminderScheduler, disclosureSvc, periodicCreator, cfg.OutboxVisibilityTimeout, cfg.ReminderDispatchEnabled, cfg.ReminderVisibilityTimeout)
			}
		}
	}()

	<-runCtx.Done()
	log.Info("worker shutting down")
	wg.Wait()
	log.Info("worker stopped")
}

func tick(ctx context.Context, log *slog.Logger, sqlDB *sql.DB, processor *platformoutbox.Processor, reminderScheduler reminderapp.Service, disclosureSvc disclosureapp.Service, periodicCreator disclosureapp.PeriodicRecordCreator, outboxVisibilityTimeout time.Duration, reminderDispatchEnabled bool, reminderVisibilityTimeout time.Duration) {
	if sqlDB != nil {
		if err := sqlDB.PingContext(ctx); err != nil {
			log.Warn("worker tick ping failed", slog.String("err", err.Error()))
			return
		}
	}
	now := time.Now().UTC()
	// Reaper (Batch 2B): recover outbox rows orphaned in `processing` by a
	// crashed/restarted worker before the processor locks the next batch, so the
	// requeued rows are picked up in the same tick. Cheap indexed UPDATE; usually
	// matches 0 rows. A non-zero count is alert-grade (worker instability).
	if outboxVisibilityTimeout > 0 {
		if requeued, err := processor.RequeueStaleProcessing(ctx, now.Add(-outboxVisibilityTimeout)); err != nil {
			log.Warn("outbox reaper failed", slog.String("err", err.Error()))
		} else if requeued > 0 {
			log.Warn("outbox reaper requeued stale processing events",
				slog.Int("requeued", requeued),
				slog.Duration("visibility_timeout", outboxVisibilityTimeout),
			)
		}
	}
	if disclosureSvc != nil {
		seeded, err := disclosureSvc.SeedPeriodicCycles(ctx, now)
		if err != nil {
			log.Warn("periodic cycle seed failed", slog.String("err", err.Error()))
		} else if seeded > 0 {
			log.Info("periodic cycles seeded", slog.Int("seeded", seeded))
		}
		if periodicCreator != nil {
			materialized, err := disclosureSvc.MaterializePeriodicDisclosures(ctx, now, periodicCreator)
			if err != nil {
				log.Warn("periodic disclosure materialize failed", slog.String("err", err.Error()))
			} else if materialized > 0 {
				log.Info("periodic disclosures materialized", slog.Int("materialized", materialized))
			}
		}
	}
	if reminderScheduler != nil {
		seeded, err := reminderScheduler.SeedOccurrencesFromDueMilestones(ctx, time.Now().UTC())
		if err != nil {
			log.Warn("milestone bridge tick failed", slog.String("err", err.Error()))
		} else if seeded > 0 {
			log.Info("workflow milestone occurrences seeded", slog.Int("seeded", seeded))
		}
		inserted, err := reminderScheduler.MaterializeDueOccurrences(ctx, time.Now().UTC())
		if err != nil {
			log.Warn("reminder scheduler tick failed", slog.String("err", err.Error()))
		} else if inserted > 0 {
			log.Info("reminder occurrences materialized", slog.Int("inserted", inserted))
		}
		// Rollback switch (Reminder Reliability Hardening): when disabled, Seed +
		// Materialize above still run so occurrences accrue accurately, but no email
		// is sent and the reaper is skipped — a no-data-loss kill switch.
		if reminderDispatchEnabled {
			dispatchRes, err := reminderScheduler.DispatchDueOccurrences(ctx, time.Now().UTC(), 50)
			if err != nil {
				log.Warn("reminder dispatch tick failed", slog.String("err", err.Error()))
			} else if dispatchRes != nil && dispatchRes.Processed > 0 {
				log.Info("reminder dispatch tick summary",
					slog.Int("processed", dispatchRes.Processed),
					slog.Int("sent", dispatchRes.Sent),
					slog.Int("retried", dispatchRes.Retried),
					slog.Int("failed", dispatchRes.Failed),
					slog.Int("skipped", dispatchRes.Skipped),
				)
			}
			// Reaper: recover occurrences orphaned in DISPATCHING by a crashed worker
			// so they re-dispatch this/next tick. Mirrors the outbox reaper above;
			// usually matches 0 rows, a non-zero count is alert-grade.
			if reminderVisibilityTimeout > 0 {
				if requeued, err := reminderScheduler.RequeueStaleDispatching(ctx, time.Now().UTC().Add(-reminderVisibilityTimeout)); err != nil {
					log.Warn("reminder reaper failed", slog.String("err", err.Error()))
				} else if requeued > 0 {
					log.Warn("reminder reaper requeued stale dispatching occurrences",
						slog.Int("requeued", requeued),
						slog.Duration("visibility_timeout", reminderVisibilityTimeout),
					)
				}
			}
		}
	}
	if err := processor.Tick(ctx); err != nil {
		log.Warn("outbox processor tick failed", slog.String("err", err.Error()))
		return
	}
	log.Debug("worker tick ok")
}

func deliverAuthEmailEvent(_ context.Context, cfg config.Config, log *slog.Logger, event platformoutbox.QueuedEvent) error {
	payload := map[string]any{}
	if err := json.Unmarshal(event.PayloadJSON, &payload); err != nil {
		return fmt.Errorf("decode auth email payload: %w", err)
	}
	to := strings.TrimSpace(fmt.Sprint(payload["to"]))
	subject := strings.TrimSpace(fmt.Sprint(payload["subject"]))
	body := strings.TrimSpace(fmt.Sprint(payload["body"]))
	if to == "" || subject == "" || body == "" {
		return fmt.Errorf("invalid auth email payload")
	}
	if strings.TrimSpace(cfg.SMTPHost) == "" {
		log.Info("smtp not configured; auth email payload logged",
			slog.String("event_id", event.EventID),
			slog.String("to", to),
			slog.String("subject", subject),
		)
		return nil
	}
	return sendSMTPMail(cfg, to, subject, body)
}

func sendSMTPMail(cfg config.Config, to, subject, body string) error {
	addr := fmt.Sprintf("%s:%d", cfg.SMTPHost, cfg.SMTPPort)
	from := cfg.SMTPFrom
	if strings.TrimSpace(from) == "" {
		from = "no-reply@cobo.local"
	}
	msg := "From: " + from + "\r\n" +
		"To: " + to + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n\r\n" +
		body + "\r\n"
	var auth smtp.Auth
	if strings.TrimSpace(cfg.SMTPUser) != "" {
		auth = smtp.PlainAuth("", cfg.SMTPUser, cfg.SMTPPassword, cfg.SMTPHost)
	}
	if err := smtp.SendMail(addr, auth, from, []string{to}, []byte(msg)); err != nil {
		return fmt.Errorf("smtp send to %s via %s: %w", to, addr, err)
	}
	return nil
}
