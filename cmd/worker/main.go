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

	"github.com/cobo/cobo_iam_services/internal/platform/config"
	"github.com/cobo/cobo_iam_services/internal/platform/db"
	"github.com/cobo/cobo_iam_services/internal/platform/logger"
	platformoutbox "github.com/cobo/cobo_iam_services/internal/platform/outbox"
	outboxinmem "github.com/cobo/cobo_iam_services/internal/platform/outbox/inmemory"
	outboxmysql "github.com/cobo/cobo_iam_services/internal/platform/outbox/mysql"
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
	processor.Register("auth.email_verification_requested", platformoutbox.HandlerFunc(func(ctx context.Context, event platformoutbox.QueuedEvent) error {
		return deliverAuthEmailEvent(ctx, cfg, log, event)
	}))

	runCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

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
				tick(runCtx, log, sqlDB, processor)
			}
		}
	}()

	<-runCtx.Done()
	log.Info("worker shutting down")
	wg.Wait()
	log.Info("worker stopped")
}

func tick(ctx context.Context, log *slog.Logger, sqlDB *sql.DB, processor *platformoutbox.Processor) {
	if sqlDB != nil {
		if err := sqlDB.PingContext(ctx); err != nil {
			log.Warn("worker tick ping failed", slog.String("err", err.Error()))
			return
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
