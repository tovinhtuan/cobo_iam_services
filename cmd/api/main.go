package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/cobo/cobo_iam_services/internal/httpserver"
	"github.com/cobo/cobo_iam_services/internal/platform/config"
	"github.com/cobo/cobo_iam_services/internal/platform/db"
	"github.com/cobo/cobo_iam_services/internal/platform/logger"
)

func main() {
	ctx := context.Background()
	cfg, err := config.Load()
	if err != nil {
		slog.Default().Error("config load failed", slog.String("err", err.Error()))
		os.Exit(1)
	}
	log := logger.New(cfg.LogLevel).With(
		slog.String("service", cfg.ServiceName),
		slog.String("env", cfg.Env),
	)

	sqlPool := openMySQLMaybe(ctx, log, cfg)
	if sqlPool != nil {
		defer func() { _ = sqlPool.Close() }()
	}

	h, cleanup, err := httpserver.New(ctx, httpserver.Deps{Log: log, Config: cfg, DB: sqlPool})
	if err != nil {
		log.Error("httpserver build failed", slog.String("err", err.Error()))
		os.Exit(1)
	}
	defer cleanup()

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           h,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       cfg.HTTPReadTimeout,
		WriteTimeout:      cfg.HTTPWriteTimeout,
		IdleTimeout:       cfg.HTTPIdleTimeout,
	}

	shutdownCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Info("api listening", slog.String("addr", cfg.HTTPAddr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("http server error", slog.String("err", err.Error()))
			stop()
		}
	}()

	<-shutdownCtx.Done()
	log.Info("shutting down api")

	ctxShutdown, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctxShutdown); err != nil {
		log.Error("http shutdown error", slog.String("err", err.Error()))
	}
	wg.Wait()
	log.Info("api stopped")
}

func openMySQLMaybe(ctx context.Context, log *slog.Logger, cfg config.Config) *sql.DB {
	if cfg.MySQLDSN == "" {
		log.Warn("MYSQL_DSN empty; API runs without database (bootstrap only)")
		return nil
	}
	pool, err := db.OpenMySQL(ctx, cfg.MySQLDSN)
	if err != nil {
		log.Error("mysql connect failed", slog.String("err", err.Error()))
		os.Exit(1)
	}
	// Note: pool is closed by OS exit on shutdown; explicit Close could be added with defer in main if desired.
	return pool
}
