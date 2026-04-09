package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"os"
	"os/signal"
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
