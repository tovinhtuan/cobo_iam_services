package httpserver

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"time"

	adhocapp "github.com/cobo/cobo_iam_services/internal/adhoc/app"
	adhocrecord "github.com/cobo/cobo_iam_services/internal/adhoc/infra/disclosure"
	adhocmysql "github.com/cobo/cobo_iam_services/internal/adhoc/infra/mysql"
	adhochttp "github.com/cobo/cobo_iam_services/internal/adhoc/transport/http"
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
	holidayapp "github.com/cobo/cobo_iam_services/internal/holiday/app"
	holidaymysql "github.com/cobo/cobo_iam_services/internal/holiday/infra/mysql"
	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	iaminmem "github.com/cobo/cobo_iam_services/internal/iam/infra/inmemory"
	iammysql "github.com/cobo/cobo_iam_services/internal/iam/infra/mysql"
	"github.com/cobo/cobo_iam_services/internal/iam/loginpassword"
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
	platformcmshttp "github.com/cobo/cobo_iam_services/internal/platformcms/transport/http"
	reminderapp "github.com/cobo/cobo_iam_services/internal/reminder/app"
	reminderemail "github.com/cobo/cobo_iam_services/internal/reminder/infra/email"
	reminderinmem "github.com/cobo/cobo_iam_services/internal/reminder/infra/inmemory"
	remindermysql "github.com/cobo/cobo_iam_services/internal/reminder/infra/mysql"
	reminderobserve "github.com/cobo/cobo_iam_services/internal/reminder/infra/observe"
	reminderhttp "github.com/cobo/cobo_iam_services/internal/reminder/transport/http"
	workflowapp "github.com/cobo/cobo_iam_services/internal/workflow/app"
	workflowinmem "github.com/cobo/cobo_iam_services/internal/workflow/infra/inmemory"
	workflowmysql "github.com/cobo/cobo_iam_services/internal/workflow/infra/mysql"
	workflowhttp "github.com/cobo/cobo_iam_services/internal/workflow/transport/http"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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
	if err := register(mux, d.Log, d.Config, d.TokenManager, sqlPing, projectionStore, outboxRepo, d.DB, outboxSQL); err != nil {
		return nil, nil, err
	}

	return corsMiddleware(d.Config, requestIDMiddleware(d.Log, mux)), cleanup, nil
}

type pingDB interface {
	PingContext(context.Context) error
}

func register(mux *http.ServeMux, log *slog.Logger, cfg config.Config, tokenMgr TokenManager, sqlDB pingDB, projectionStore authprojection.SnapshotStore, outboxRepo platformoutbox.Repository, pool *sql.DB, outboxSQL *outboxmysql.Repository) error {
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
	var recoveryRepo iamapp.AuthRecoveryRepository
	if pool != nil {
		memberQuery = camysql.NewMembershipQueryService(pool)
		sessionRepo = iammysql.NewSessionRepository(pool, 720*time.Hour)
		cv := iammysql.NewCredentialVerifier(pool)
		credVerifier = cv
		identity = cv
		recoveryRepo = iammysql.NewAuthRecoveryRepository(pool)
		log.Info("iam using MySQL sessions + credentials; membership query from DB")
	} else {
		memberQuery = cainmem.NewMembershipQueryService()
		sessionRepo = iaminmem.NewSessionRepository()
		static := &iaminmem.StaticCredentialVerifier{
			Users: map[string]iaminmem.StaticUser{
				"user@example.com":   {UserID: "u_123", LoginID: "user@example.com", Password: "secret", FullName: "Nguyen Van A", Status: "active", SubscriptionTier: "Free"},
				"single@example.com": {UserID: "u_single", LoginID: "single@example.com", Password: "secret", FullName: "Single Company User", Status: "active", SubscriptionTier: "Free"},
				// Same membership/roles as admin@cobo.vn (u_admin) — password `secret` for local smoke tests.
				"admin.dn@example.com":     {UserID: "u_admin", LoginID: "admin.dn@example.com", Password: "secret", FullName: "Enterprise Admin (DN)", Status: "active", SubscriptionTier: "Enterprise"},
				"admin@cobo.vn":            {UserID: "u_admin", LoginID: "admin@cobo.vn", Password: "password123", FullName: "Enterprise Admin", Status: "active", SubscriptionTier: "Enterprise"},
				"cms.operator@example.com": {UserID: "u_cms", LoginID: "cms.operator@example.com", Password: "secret", FullName: "CMS Operator", Status: "active", SubscriptionTier: "Enterprise"},
			},
		}
		credVerifier = static
		identity = static
	}
	var iamOpts []iamapp.ServiceOption
	iamOpts = append(iamOpts, iamapp.WithAuthFlowConfig(iamapp.AuthFlowConfig{
		WebBaseURL:              cfg.PublicWebBaseURL,
		UserInvitationTokenTTL:  cfg.UserInvitationTokenTTL,
		EmailVerificationOTPTTL: cfg.EmailVerificationOTPTTL,
	}))
	if pool != nil {
		iamOpts = append(iamOpts,
			iamapp.WithUserInvitationExecutor(&iammysql.UserInvitationStore{DB: pool}),
			iamapp.WithPublicRegistration(pool),
		)
	}
	if cfg.RegistrationDisabled {
		iamOpts = append(iamOpts, iamapp.WithRegistrationDisabled(true))
	}
	if pool != nil {
		iamOpts = append(iamOpts, iamapp.WithLoginAttemptRecorder(iammysql.NewLoginAttemptRecorder(pool)))
		log.Info("login_attempts writes enabled (MySQL)")
	}
	if recoveryRepo != nil {
		iamOpts = append(iamOpts,
			iamapp.WithAuthRecoveryRepository(recoveryRepo),
			iamapp.WithOutboxPublisher(outboxPublisher),
		)
	}
	var loginPWD *loginpassword.Service
	if cfg.LoginPasswordRSAPrivateKeyPEM != "" {
		lp, err := loginpassword.NewFromPEM(cfg.LoginPasswordRSAPrivateKeyPEM, cfg.LoginPasswordRSAKeyID)
		if err != nil {
			return fmt.Errorf("LOGIN_PASSWORD_RSA_PRIVATE_KEY_PEM: %w", err)
		}
		loginPWD = lp
		log.Info("login password RSA-OAEP transport encryption enabled", slog.String("kid", loginPWD.KeyID()))
	}

	iamSvc := iamapp.NewService(credVerifier, sessionRepo, tokenManager, memberQuery, id, iamOpts...)
	iamHandler := iamhttp.NewHandler(log, iamSvc, tokenManager, auditSvc, outboxPublisher, id, loginPWD)
	var authRepo authapp.Repository = authinmem.NewRepository()
	if pool != nil {
		authRepo = authmysql.NewRepository(pool)
		log.Info("authorization effective-access reads from MySQL (roles/permissions/assignments + projection responsibilities)")
	}
	baseAuthResolver := authinmem.NewResolver(authRepo)
	authResolver := authprojection.NewCachedResolver(baseAuthResolver, projectionStore)
	authChecker := authinmem.NewChecker()
	authSvc := authapp.NewService(authResolver, authChecker, authRepo)
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
	fileHoliday := disclosureapp.NewHolidayCalendarFileProvider(filepath.Join("configs", "non_trading_days"))
	var disclosureOpts []disclosureapp.ServiceOption
	disclosureOpts = append(disclosureOpts, disclosureapp.WithWorkflowGroupsEnabled(cfg.WorkflowGroupsEnabled))
	var holidaySvc holidayapp.Service
	if pool != nil {
		holidayRepo := holidaymysql.NewRepository(pool)
		dbHoliday := holidaymysql.NewDBProvider(holidayRepo)
		composite := &holidaymysql.CompositeProvider{
			Repo: holidayRepo,
			DB:   dbHoliday,
			File: fileHoliday,
		}
		disclosureOpts = append(disclosureOpts, disclosureapp.WithHolidayCalendarProvider(composite))
		holidaySvc = holidayapp.NewService(holidayRepo, dbHoliday, id)
	}
	disclosureSvc := disclosureapp.NewService(disclosureRepo, authSvc, id, disclosureOpts...)
	var idemStore idempotency.Store
	if pool != nil {
		idemStore = idempotencymysql.NewStore(pool)
		log.Info("disclosure submit/confirm idempotency enabled (Idempotency-Key header)")
	}
	disclosureHandler := disclosurehttp.NewHandler(disclosureSvc, tokenManager, idemStore, auditSvc)
	workflowOpts := []workflowapp.ServiceOption{
		workflowapp.WithFlags(workflowapp.Flags{
			SnapshotEnabled: cfg.WorkflowSnapshotEnabled,
			TimelineEnabled: cfg.WorkflowTimelineEnabled,
		}),
	}
	if pool != nil && cfg.WorkflowTimelineEnabled {
		workflowOpts = append(workflowOpts, workflowapp.WithMilestoneRepository(workflowmysql.NewMilestoneRepository(pool)))
	}
	workflowSvc := workflowapp.NewService(workflowRepo, authSvc, id, workflowOpts...)
	workflowHandler := workflowhttp.NewHandler(workflowSvc, tokenManager)
	notificationSvc := notificationapp.NewService(notificationRepo, authSvc, id, outboxPublisher, notifOpts...)
	notificationHandler := notificationhttp.NewHandler(notificationSvc, tokenManager)
	reminderRepo := reminderinmem.NewRepository()
	var reminderConfigRepo reminderapp.ConfigRepository = reminderRepo
	var reminderOccurrenceRepo reminderapp.OccurrenceRepository = reminderRepo
	var reminderAttemptRepo reminderapp.AttemptRepository = reminderRepo
	var reminderSvcOpts []reminderapp.ServiceOption
	if pool != nil {
		reminderMySQLRepo := remindermysql.NewRepository(pool)
		reminderConfigRepo = reminderMySQLRepo
		reminderOccurrenceRepo = reminderMySQLRepo
		reminderAttemptRepo = reminderMySQLRepo
		log.Info("reminder module using MySQL repository")
		if cfg.WorkflowRemindersEnabled {
			reminderSvcOpts = append(reminderSvcOpts, reminderapp.WithMilestoneScanner(remindermysql.NewMilestoneScanner(pool)))
			log.Info("workflow milestone reminder bridge enabled")
		}
	}
	reminderSvcOpts = append(reminderSvcOpts,
		reminderapp.WithEmailSender(reminderemail.NewSMTPSender(reminderemail.SMTPConfig{
			Host: cfg.SMTPHost,
			Port: cfg.SMTPPort,
			User: cfg.SMTPUser,
			Pass: cfg.SMTPPassword,
			From: cfg.SMTPFrom,
		})),
		reminderapp.WithMetrics(reminderobserve.NewPromMetrics()),
		reminderapp.WithAuditor(reminderobserve.AuditRecorder{Svc: auditSvc, IDG: id}),
		reminderapp.WithAlertHook(reminderobserve.AlertLogger{Log: log}),
	)
	reminderSvc := reminderapp.NewService(reminderConfigRepo, reminderOccurrenceRepo, reminderAttemptRepo, reminderSvcOpts...)
	reminderHandler := reminderhttp.NewHandler(reminderSvc, tokenManager, "", cfg.Env)
	var adminRepo companyaccessapp.AdminRepository = cainmem.NewAdminRepository()
	if pool != nil {
		adminRepo = camysql.NewAdminRepository(pool)
		log.Info("admin access APIs using MySQL")
	}
	var adminOpts []companyaccessapp.AdminOption
	adminOpts = append(adminOpts, companyaccessapp.WithInviteDefaultRoleCode(cfg.InviteDefaultRoleCode))
	if pool != nil {
		adminOpts = append(adminOpts,
			companyaccessapp.WithInvitationMailer(&iamInvitationMailer{iam: iamSvc}),
			companyaccessapp.WithInvitationTTL(cfg.UserInvitationTokenTTL),
		)
	}
	adminSvc := companyaccessapp.NewAdminService(adminRepo, authSvc, id, adminOpts...)
	adminHandler := companyaccesshttp.NewAdminHandler(adminSvc, tokenManager, auditSvc)
	platformCMSHandler := platformcmshttp.NewHandler(tokenManager, authSvc, adminSvc, iamSvc, auditSvc, auditRepo, disclosureSvc, disclosureRepo, holidaySvc, platformcmshttp.MediaOptions{
		DB:                  pool,
		UploadSigningSecret: cfg.CMSMediaUploadSigningSecret,
		UploadURLTTL:        cfg.CMSMediaUploadURLTTL,
		StorageDir:          cfg.CMSMediaStorageDir,
		PublicAPIBaseURL:    cfg.PublicAPIBaseURL,
	})

	// Ad-hoc proposal module (WORKFLOW_ADHOC_ENABLED flag).
	var adhocHandler *adhochttp.Handler
	if cfg.WorkflowAdhocEnabled {
		var adhocRepo adhocapp.Repository
		if pool != nil {
			adhocRepo = adhocmysql.NewRepository(pool)
		} else {
			adhocRepo = adhocmysql.NewRepository(nil) // will panic on use; acceptable in no-DB mode
		}
		recordCreator := adhocrecord.NewRecordCreatorAdapter(disclosureSvc, workflowSvc, true)
		typeCatalog := adhocrecord.NewTypeCatalogAdapter(disclosureRepo)
		membershipValidator := adhocmysql.NewMembershipValidator(pool)
		adhocSvc := adhocapp.NewService(adhocRepo, recordCreator, typeCatalog, id, cfg.WorkflowAdhocAutoApproveEnabled, authSvc, membershipValidator)
		adhocHandler = adhochttp.NewHandler(adhocSvc, tokenManager, idemStore)
		log.Info("ad-hoc proposal module enabled")
	}

	return muxRegisterHealthAndIAM(mux, log, sqlDB, iamHandler, meHandler, authHandler, disclosureHandler, workflowHandler, notificationHandler, reminderHandler, adminHandler, platformCMSHandler, adhocHandler)
}

func muxRegisterHealthAndIAM(
	mux *http.ServeMux,
	log *slog.Logger,
	sqlDB pingDB,
	iamHandler *iamhttp.Handler,
	meHandler *iamhttp.MeHandler,
	authHandler *authhttp.Handler,
	disclosureHandler *disclosurehttp.Handler,
	workflowHandler *workflowhttp.Handler,
	notificationHandler *notificationhttp.Handler,
	reminderHandler *reminderhttp.Handler,
	adminHandler *companyaccesshttp.AdminHandler,
	platformCMSHandler *platformcmshttp.Handler,
	adhocHandler *adhochttp.Handler,
) error {
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	})
	mux.Handle("/metrics", promhttp.Handler())
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
	reminderHandler.Register(mux)
	adminHandler.Register(mux)
	platformCMSHandler.Register(mux)
	if adhocHandler != nil {
		adhocHandler.Register(mux)
	}
	return nil
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
