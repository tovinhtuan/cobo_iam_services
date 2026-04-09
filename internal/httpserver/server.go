package httpserver

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"time"

	auditapp "github.com/cobo/cobo_iam_services/internal/audit/app"
	auditappimpl "github.com/cobo/cobo_iam_services/internal/audit/appimpl"
	auditinmem "github.com/cobo/cobo_iam_services/internal/audit/infra/inmemory"
	auditmysql "github.com/cobo/cobo_iam_services/internal/audit/infra/mysql"
	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	authinmem "github.com/cobo/cobo_iam_services/internal/authorization/infra/inmemory"
	authmysql "github.com/cobo/cobo_iam_services/internal/authorization/infra/mysql"
	authprojection "github.com/cobo/cobo_iam_services/internal/authorization/infra/projection"
	authhttp "github.com/cobo/cobo_iam_services/internal/authorization/transport/http"
	companyaccessapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	cainmem "github.com/cobo/cobo_iam_services/internal/companyaccess/infra/inmemory"
	camysql "github.com/cobo/cobo_iam_services/internal/companyaccess/infra/mysql"
	companyaccesshttp "github.com/cobo/cobo_iam_services/internal/companyaccess/transport/http"
	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	disclosureinmem "github.com/cobo/cobo_iam_services/internal/disclosure/infra/inmemory"
	disclosuremysql "github.com/cobo/cobo_iam_services/internal/disclosure/infra/mysql"
	disclosurehttp "github.com/cobo/cobo_iam_services/internal/disclosure/transport/http"
	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	iaminmem "github.com/cobo/cobo_iam_services/internal/iam/infra/inmemory"
	iammysql "github.com/cobo/cobo_iam_services/internal/iam/infra/mysql"
	iamhttp "github.com/cobo/cobo_iam_services/internal/iam/transport/http"
	notificationapp "github.com/cobo/cobo_iam_services/internal/notification/app"
	notificationinmem "github.com/cobo/cobo_iam_services/internal/notification/infra/inmemory"
	notificationmysql "github.com/cobo/cobo_iam_services/internal/notification/infra/mysql"
	notificationhttp "github.com/cobo/cobo_iam_services/internal/notification/transport/http"
	platformclock "github.com/cobo/cobo_iam_services/internal/platform/clock"
	"github.com/cobo/cobo_iam_services/internal/platform/config"
	"github.com/cobo/cobo_iam_services/internal/platform/httpx"
	"github.com/cobo/cobo_iam_services/internal/platform/idempotency"
	idempotencymysql "github.com/cobo/cobo_iam_services/internal/platform/idempotency/mysql"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
	platformoutbox "github.com/cobo/cobo_iam_services/internal/platform/outbox"
	outboxinmem "github.com/cobo/cobo_iam_services/internal/platform/outbox/inmemory"
	outboxmysql "github.com/cobo/cobo_iam_services/internal/platform/outbox/mysql"
	redispkg "github.com/cobo/cobo_iam_services/internal/platform/redis"
	workflowapp "github.com/cobo/cobo_iam_services/internal/workflow/app"
	workflowinmem "github.com/cobo/cobo_iam_services/internal/workflow/infra/inmemory"
	workflowmysql "github.com/cobo/cobo_iam_services/internal/workflow/infra/mysql"
	workflowhttp "github.com/cobo/cobo_iam_services/internal/workflow/transport/http"
)

// Deps wires HTTP API dependencies.
type Deps struct {
	Log    *slog.Logger
	Config config.Config
	DB     *sql.DB // optional; when set: IAM + membership + authz + audit + admin + P1 repos + MySQL outbox
	// Optional token manager override (useful for integration tests).
	TokenManager TokenManager
}

// New builds the full API http.Handler and an optional cleanup (e.g. close Redis).
func New(ctx context.Context, d Deps) (http.Handler, func(), error) {
	cleanup := func() {}

	projectionStore := authprojection.NewInMemoryStore(d.Config.EffectiveAccessCacheTTL)
	if d.Config.RedisAddr != "" {
		rdb, err := redispkg.Open(ctx, d.Config)
		if err != nil {
			d.Log.Warn("redis unavailable; using in-memory effective-access cache", slog.String("err", err.Error()))
		} else if rdb != nil {
			prev := cleanup
			cleanup = func() {
				prev()
				_ = rdb.Close()
			}
			projectionStore = authprojection.NewRedisStore(rdb, d.Config.EffectiveAccessCacheTTL)
			d.Log.Info("redis effective-access cache enabled", slog.String("addr", d.Config.RedisAddr))
		}
	}

	var outboxRepo platformoutbox.Repository
	var outboxSQL *outboxmysql.Repository
	if d.DB != nil {
		outboxSQL = outboxmysql.NewRepository(d.DB)
		outboxRepo = outboxSQL
		d.Log.Info("outbox using MySQL")
	} else {
		outboxRepo = outboxinmem.NewRepository()
		d.Log.Warn("outbox using in-memory (lost on restart; set MYSQL_DSN for durable outbox)")
	}

	var sqlPing pingDB
	if d.DB != nil {
		sqlPing = d.DB
	}

	mux := http.NewServeMux()
	register(mux, d.Log, d.Config, d.TokenManager, sqlPing, projectionStore, outboxRepo, d.DB, outboxSQL)

	return requestIDMiddleware(d.Log, mux), cleanup, nil
}

type pingDB interface {
	PingContext(context.Context) error
}

func register(mux *http.ServeMux, log *slog.Logger, cfg config.Config, tokenMgr TokenManager, sqlDB pingDB, projectionStore authprojection.SnapshotStore, outboxRepo platformoutbox.Repository, pool *sql.DB, outboxSQL *outboxmysql.Repository) {
	id := idgen.UUIDv7Generator{}
	tokenManager := tokenMgr
	if tokenManager == nil {
		tokenManager = buildTokenManager(log, cfg, id)
	}
	var auditRepo auditapp.Repository = auditinmem.NewRepository()
	if pool != nil {
		auditRepo = auditmysql.NewRepository(pool)
		log.Info("audit logs using MySQL (audit_logs)")
	}
	auditSvc := auditappimpl.NewService(auditRepo, platformclock.System{}, id)
	outboxPublisher := platformoutbox.NewPublisher(outboxRepo)

	var memberQuery companyaccessapp.MembershipQueryService
	var sessionRepo iamapp.SessionRepository
	var credVerifier iamapp.CredentialVerifier
	var identity iamapp.IdentityQueryService
	if pool != nil {
		memberQuery = camysql.NewMembershipQueryService(pool)
		sessionRepo = iammysql.NewSessionRepository(pool, 720*time.Hour)
		cv := iammysql.NewCredentialVerifier(pool)
		credVerifier = cv
		identity = cv
		log.Info("iam using MySQL sessions + credentials; membership query from DB")
	} else {
		memberQuery = cainmem.NewMembershipQueryService()
		sessionRepo = iaminmem.NewSessionRepository()
		static := &iaminmem.StaticCredentialVerifier{
			Users: map[string]iaminmem.StaticUser{
				"user@example.com":   {UserID: "u_123", LoginID: "user@example.com", Password: "secret", FullName: "Nguyen Van A", Status: "active"},
				"single@example.com": {UserID: "u_single", LoginID: "single@example.com", Password: "secret", FullName: "Single Company User", Status: "active"},
			},
		}
		credVerifier = static
		identity = static
	}
	var iamOpts []iamapp.ServiceOption
	if pool != nil {
		iamOpts = append(iamOpts, iamapp.WithLoginAttemptRecorder(iammysql.NewLoginAttemptRecorder(pool)))
		log.Info("login_attempts writes enabled (MySQL)")
	}
	iamSvc := iamapp.NewService(credVerifier, sessionRepo, tokenManager, memberQuery, id, iamOpts...)
	iamHandler := iamhttp.NewHandler(log, iamSvc, tokenManager, auditSvc, outboxPublisher, id)
	var authRepo authapp.Repository = authinmem.NewRepository()
	if pool != nil {
		authRepo = authmysql.NewRepository(pool)
		log.Info("authorization effective-access reads from MySQL (roles/permissions/assignments + projection responsibilities)")
	}
	baseAuthResolver := authinmem.NewResolver(authRepo)
	authResolver := authprojection.NewCachedResolver(baseAuthResolver, projectionStore)
	authChecker := authinmem.NewChecker()
	authSvc := authapp.NewService(authResolver, authChecker)
	authHandler := authhttp.NewHandler(authSvc, tokenManager)
	meHandler := iamhttp.NewMeHandler(iamHandler, identity, memberQuery, authSvc)

	var disclosureRepo disclosureapp.Repository = disclosureinmem.NewRepository()
	var workflowRepo workflowapp.Repository = workflowinmem.NewRepository()
	var notificationRepo notificationapp.Repository = notificationinmem.NewRepository()
	var notifOpts []notificationapp.ServiceOption
	if pool != nil {
		disclosureRepo = disclosuremysql.NewRepository(pool)
		workflowRepo = workflowmysql.NewRepository(pool)
		notificationRepo = notificationmysql.NewRepository(pool)
		if outboxSQL != nil {
			notifOpts = append(notifOpts, notificationapp.WithTransactionalEnqueue(pool, outboxSQL))
		}
	}
	disclosureSvc := disclosureapp.NewService(disclosureRepo, authSvc, id)
	var idemStore idempotency.Store
	if pool != nil {
		idemStore = idempotencymysql.NewStore(pool)
		log.Info("disclosure submit/confirm idempotency enabled (Idempotency-Key header)")
	}
	disclosureHandler := disclosurehttp.NewHandler(disclosureSvc, tokenManager, idemStore)
	workflowSvc := workflowapp.NewService(workflowRepo, authSvc, id)
	workflowHandler := workflowhttp.NewHandler(workflowSvc, tokenManager)
	notificationSvc := notificationapp.NewService(notificationRepo, authSvc, id, outboxPublisher, notifOpts...)
	notificationHandler := notificationhttp.NewHandler(notificationSvc, tokenManager)
	var adminRepo companyaccessapp.AdminRepository = cainmem.NewAdminRepository()
	if pool != nil {
		adminRepo = camysql.NewAdminRepository(pool)
		log.Info("admin access APIs using MySQL")
	}
	adminSvc := companyaccessapp.NewAdminService(adminRepo, authSvc, id)
	adminHandler := companyaccesshttp.NewAdminHandler(adminSvc, tokenManager, auditSvc)

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"status": "ok"})
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
			httpx.WriteJSON(w, http.StatusServiceUnavailable, map[string]any{"status": "not_ready"})
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

func requestIDMiddleware(_ *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		id := r.Header.Get(httpx.RequestIDHeader)
		if id == "" {
			ctx, id = httpx.EnsureRequestID(ctx)
		} else {
			ctx = httpx.WithRequestID(ctx, id)
		}
		w.Header().Set(httpx.RequestIDHeader, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
