package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	auditappimpl "github.com/cobo/cobo_iam_services/internal/audit/appimpl"
	auditinmem "github.com/cobo/cobo_iam_services/internal/audit/infra/inmemory"
	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	authinmem "github.com/cobo/cobo_iam_services/internal/authorization/infra/inmemory"
	authprojection "github.com/cobo/cobo_iam_services/internal/authorization/infra/projection"
	authhttp "github.com/cobo/cobo_iam_services/internal/authorization/transport/http"
	companyaccessapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	cainmem "github.com/cobo/cobo_iam_services/internal/companyaccess/infra/inmemory"
	companyaccesshttp "github.com/cobo/cobo_iam_services/internal/companyaccess/transport/http"
	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	disclosureinmem "github.com/cobo/cobo_iam_services/internal/disclosure/infra/inmemory"
	disclosurehttp "github.com/cobo/cobo_iam_services/internal/disclosure/transport/http"
	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	iaminmem "github.com/cobo/cobo_iam_services/internal/iam/infra/inmemory"
	iamhttp "github.com/cobo/cobo_iam_services/internal/iam/transport/http"
	notificationapp "github.com/cobo/cobo_iam_services/internal/notification/app"
	notificationinmem "github.com/cobo/cobo_iam_services/internal/notification/infra/inmemory"
	notificationhttp "github.com/cobo/cobo_iam_services/internal/notification/transport/http"
	platformclock "github.com/cobo/cobo_iam_services/internal/platform/clock"
	"github.com/cobo/cobo_iam_services/internal/platform/config"
	"github.com/cobo/cobo_iam_services/internal/platform/db"
	"github.com/cobo/cobo_iam_services/internal/platform/httpx"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
	"github.com/cobo/cobo_iam_services/internal/platform/logger"
	platformoutbox "github.com/cobo/cobo_iam_services/internal/platform/outbox"
	outboxinmem "github.com/cobo/cobo_iam_services/internal/platform/outbox/inmemory"
	redispkg "github.com/cobo/cobo_iam_services/internal/platform/redis"
	workflowapp "github.com/cobo/cobo_iam_services/internal/workflow/app"
	workflowinmem "github.com/cobo/cobo_iam_services/internal/workflow/infra/inmemory"
	workflowhttp "github.com/cobo/cobo_iam_services/internal/workflow/transport/http"
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

	var sqlDB httpHandlerDeps
	if cfg.MySQLDSN != "" {
		pool, err := db.OpenMySQL(ctx, cfg.MySQLDSN)
		if err != nil {
			log.Error("mysql connect failed", slog.String("err", err.Error()))
			os.Exit(1)
		}
		defer pool.Close()
		sqlDB = pool
	} else {
		log.Warn("MYSQL_DSN empty; API runs without database (bootstrap only)")
	}

	projectionStore := authprojection.NewInMemoryStore(cfg.EffectiveAccessCacheTTL)
	if cfg.RedisAddr != "" {
		rdb, err := redispkg.Open(ctx, cfg)
		if err != nil {
			log.Warn("redis unavailable; using in-memory effective-access cache", slog.String("err", err.Error()))
		} else if rdb != nil {
			defer func() { _ = rdb.Close() }()
			projectionStore = authprojection.NewRedisStore(rdb, cfg.EffectiveAccessCacheTTL)
			log.Info("redis effective-access cache enabled", slog.String("addr", cfg.RedisAddr))
		}
	}

	mux := http.NewServeMux()
	registerRoutes(mux, log, sqlDB, projectionStore)

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           requestIDMiddleware(log, mux),
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

type httpHandlerDeps interface {
	PingContext(context.Context) error
}

func registerRoutes(mux *http.ServeMux, log *slog.Logger, sqlDB httpHandlerDeps, projectionStore authprojection.SnapshotStore) {
	// P0 bootstrap wiring with in-memory adapters before MySQL repositories are added.
	id := idgen.UUIDv7Generator{}
	memberQuery := cainmem.NewMembershipQueryService()
	tokenManager := iaminmem.NewTokenManager(id)
	sessionRepo := iaminmem.NewSessionRepository()
	auditRepo := auditinmem.NewRepository()
	auditSvc := auditappimpl.NewService(auditRepo, platformclock.System{}, id)
	outboxRepo := outboxinmem.NewRepository()
	outboxPublisher := platformoutbox.NewPublisher(outboxRepo)
	credVerifier := &iaminmem.StaticCredentialVerifier{
		Users: map[string]iaminmem.StaticUser{
			"user@example.com":   {UserID: "u_123", LoginID: "user@example.com", Password: "secret", FullName: "Nguyen Van A", Status: "active"},
			"single@example.com": {UserID: "u_single", LoginID: "single@example.com", Password: "secret", FullName: "Single Company User", Status: "active"},
		},
	}
	iamSvc := iamapp.NewService(credVerifier, sessionRepo, tokenManager, memberQuery, id)
	iamHandler := iamhttp.NewHandler(log, iamSvc, tokenManager, auditSvc, outboxPublisher, id)
	authRepo := authinmem.NewRepository()
	baseAuthResolver := authinmem.NewResolver(authRepo)
	authResolver := authprojection.NewCachedResolver(baseAuthResolver, projectionStore)
	authChecker := authinmem.NewChecker()
	authSvc := authapp.NewService(authResolver, authChecker)
	authHandler := authhttp.NewHandler(authSvc, tokenManager)
	meHandler := iamhttp.NewMeHandler(iamHandler, credVerifier, memberQuery, authSvc)
	disclosureRepo := disclosureinmem.NewRepository()
	disclosureSvc := disclosureapp.NewService(disclosureRepo, authSvc, id)
	disclosureHandler := disclosurehttp.NewHandler(disclosureSvc, tokenManager)
	workflowRepo := workflowinmem.NewRepository()
	workflowSvc := workflowapp.NewService(workflowRepo, authSvc, id)
	workflowHandler := workflowhttp.NewHandler(workflowSvc, tokenManager)
	notificationRepo := notificationinmem.NewRepository()
	notificationSvc := notificationapp.NewService(notificationRepo, authSvc, id, outboxPublisher)
	notificationHandler := notificationhttp.NewHandler(notificationSvc, tokenManager)
	adminRepo := cainmem.NewAdminRepository()
	adminSvc := companyaccessapp.NewAdminService(adminRepo, authSvc, id)
	adminHandler := companyaccesshttp.NewAdminHandler(adminSvc, tokenManager, auditSvc)

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{
			"status": "ok",
		})
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		if sqlDB == nil {
			httpx.WriteJSON(w, http.StatusServiceUnavailable, map[string]any{
				"status": "not_ready",
				"reason": "database not configured",
			})
			return
		}
		if err := sqlDB.PingContext(r.Context()); err != nil {
			log.Warn("readyz ping failed", slog.String("err", err.Error()))
			httpx.WriteJSON(w, http.StatusServiceUnavailable, map[string]any{
				"status": "not_ready",
			})
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"status": "ready"})
	})
	iamHandler.Register(mux)
	meHandler.Register(mux)
	authHandler.Register(mux)
	disclosureHandler.Register(mux)
	workflowHandler.Register(mux)
	notificationHandler.Register(mux)
	adminHandler.Register(mux)
}

func requestIDMiddleware(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		id := r.Header.Get(httpx.RequestIDHeader)
		if id == "" {
			ctx, id = httpx.EnsureRequestID(ctx)
		} else {
			ctx = httpx.WithRequestID(ctx, id)
		}
		w.Header().Set(httpx.RequestIDHeader, id)
		sr := r.WithContext(ctx)
		next.ServeHTTP(w, sr)
	})
}
