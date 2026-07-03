package httpserver

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	adhocapp "github.com/cobo/cobo_iam_services/internal/adhoc/app"
	adhocrecord "github.com/cobo/cobo_iam_services/internal/adhoc/infra/disclosure"
	adhocmysql "github.com/cobo/cobo_iam_services/internal/adhoc/infra/mysql"
	adhocnotif "github.com/cobo/cobo_iam_services/internal/adhoc/infra/notification"
	adhocobserve "github.com/cobo/cobo_iam_services/internal/adhoc/observability"
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
	deadlinealertsapp "github.com/cobo/cobo_iam_services/internal/deadlinealerts/app"
	deadlinealertsinmem "github.com/cobo/cobo_iam_services/internal/deadlinealerts/infra/inmemory"
	deadlinealertsmysql "github.com/cobo/cobo_iam_services/internal/deadlinealerts/infra/mysql"
	deadlinealertshttp "github.com/cobo/cobo_iam_services/internal/deadlinealerts/transport/http"
	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	disclosureinmem "github.com/cobo/cobo_iam_services/internal/disclosure/infra/inmemory"
	disclosuremysql "github.com/cobo/cobo_iam_services/internal/disclosure/infra/mysql"
	disclosureworkflow "github.com/cobo/cobo_iam_services/internal/disclosure/infra/workflow"
	disclosurehttp "github.com/cobo/cobo_iam_services/internal/disclosure/transport/http"
	holidayapp "github.com/cobo/cobo_iam_services/internal/holiday/app"
	holidaymysql "github.com/cobo/cobo_iam_services/internal/holiday/infra/mysql"
	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	iaminmem "github.com/cobo/cobo_iam_services/internal/iam/infra/inmemory"
	iammysql "github.com/cobo/cobo_iam_services/internal/iam/infra/mysql"
	"github.com/cobo/cobo_iam_services/internal/iam/loginpassword"
	iamhttp "github.com/cobo/cobo_iam_services/internal/iam/transport/http"
	inappapp "github.com/cobo/cobo_iam_services/internal/inappnotification/app"
	inappmem "github.com/cobo/cobo_iam_services/internal/inappnotification/infra/inmemory"
	inappmysql "github.com/cobo/cobo_iam_services/internal/inappnotification/infra/mysql"
	marketapp "github.com/cobo/cobo_iam_services/internal/marketreference/app"
	marketmysql "github.com/cobo/cobo_iam_services/internal/marketreference/infra/mysql"
	notificationapp "github.com/cobo/cobo_iam_services/internal/notification/app"
	notificationinmem "github.com/cobo/cobo_iam_services/internal/notification/infra/inmemory"
	notificationmysql "github.com/cobo/cobo_iam_services/internal/notification/infra/mysql"
	notificationobserve "github.com/cobo/cobo_iam_services/internal/notification/infra/observe"
	notificationregistry "github.com/cobo/cobo_iam_services/internal/notification/infra/registry"
	notificationsmtp "github.com/cobo/cobo_iam_services/internal/notification/infra/smtp"
	notificationhttp "github.com/cobo/cobo_iam_services/internal/notification/transport/http"
	platformclock "github.com/cobo/cobo_iam_services/internal/platform/clock"
	"github.com/cobo/cobo_iam_services/internal/platform/config"
	"github.com/cobo/cobo_iam_services/internal/subscription/entitlement"
	"github.com/cobo/cobo_iam_services/internal/platform/db"
	"github.com/cobo/cobo_iam_services/internal/platform/httpx"
	"github.com/cobo/cobo_iam_services/internal/platform/idempotency"
	idempotencymysql "github.com/cobo/cobo_iam_services/internal/platform/idempotency/mysql"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
	"github.com/cobo/cobo_iam_services/internal/platform/mediaupload"
	platformoutbox "github.com/cobo/cobo_iam_services/internal/platform/outbox"
	outboxinmem "github.com/cobo/cobo_iam_services/internal/platform/outbox/inmemory"
	outboxmysql "github.com/cobo/cobo_iam_services/internal/platform/outbox/mysql"
	redispkg "github.com/cobo/cobo_iam_services/internal/platform/redis"
	platformcmsapp "github.com/cobo/cobo_iam_services/internal/platformcms/app"
	platformcmshttp "github.com/cobo/cobo_iam_services/internal/platformcms/transport/http"
	reminderapp "github.com/cobo/cobo_iam_services/internal/reminder/app"
	reminderemail "github.com/cobo/cobo_iam_services/internal/reminder/infra/email"
	reminderinmem "github.com/cobo/cobo_iam_services/internal/reminder/infra/inmemory"
	reminderalertmysql "github.com/cobo/cobo_iam_services/internal/reminder/infra/mysql"
	remindermysql "github.com/cobo/cobo_iam_services/internal/reminder/infra/mysql"
	reminderobserve "github.com/cobo/cobo_iam_services/internal/reminder/infra/observe"
	reminderhttp "github.com/cobo/cobo_iam_services/internal/reminder/transport/http"
	workflowapp "github.com/cobo/cobo_iam_services/internal/workflow/app"
	workflowinmem "github.com/cobo/cobo_iam_services/internal/workflow/infra/inmemory"
	workflowmysql "github.com/cobo/cobo_iam_services/internal/workflow/infra/mysql"
	workflownotif "github.com/cobo/cobo_iam_services/internal/workflow/infra/notification"
	workflowhttp "github.com/cobo/cobo_iam_services/internal/workflow/transport/http"
	wfcapp "github.com/cobo/cobo_iam_services/internal/workflowconfig/app"
	wfcmysql "github.com/cobo/cobo_iam_services/internal/workflowconfig/infra/mysql"
	wfchttp "github.com/cobo/cobo_iam_services/internal/workflowconfig/transport/http"
	"github.com/prometheus/client_golang/prometheus"
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

	var vnstockDB *sql.DB
	if d.Config.VnstockMarketEnabled && d.Config.VnstockMySQLDSN != "" {
		pool, err := db.OpenMySQL(ctx, d.Config.VnstockMySQLDSN)
		if err != nil {
			return nil, nil, fmt.Errorf("vnstock mysql: %w", err)
		}
		vnstockDB = pool
		prev := cleanup
		cleanup = func() {
			prev()
			_ = vnstockDB.Close()
		}
		d.Log.Info("vnstock market MySQL read-only pool enabled")
	} else if d.Config.VnstockMarketEnabled {
		d.Log.Warn("VNSTOCK_MARKET_ENABLED=true but VNSTOCK_MYSQL_DSN empty; listed-companies API will return 503 until configured")
	}

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
	if err := register(mux, d.Log, d.Config, d.TokenManager, sqlPing, projectionStore, outboxRepo, d.DB, outboxSQL, vnstockDB); err != nil {
		return nil, nil, err
	}

	return corsMiddleware(d.Config, requestIDMiddleware(d.Log, mux)), cleanup, nil
}

type pingDB interface {
	PingContext(context.Context) error
}

func buildListedCompaniesService(cfg config.Config, vnstockDB *sql.DB, log *slog.Logger) *marketapp.Service {
	if cfg.VnstockMarketEnabled && cfg.VnstockMySQLDSN != "" && vnstockDB != nil {
		repo := marketmysql.NewRepository(vnstockDB)
		log.Info("cms listed companies market reference enabled")
		return marketapp.NewService(repo, vnstockDB.PingContext)
	}
	return marketapp.NewDisabledService()
}

func register(mux *http.ServeMux, log *slog.Logger, cfg config.Config, tokenMgr TokenManager, sqlDB pingDB, projectionStore authprojection.SnapshotStore, outboxRepo platformoutbox.Repository, pool *sql.DB, outboxSQL *outboxmysql.Repository, vnstockDB *sql.DB) error {
	listedCompaniesSvc := buildListedCompaniesService(cfg, vnstockDB, log)
	id := idgen.UUIDv7Generator{}
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
	tokenManager := tokenMgr
	if tokenManager == nil {
		tokenManager = buildTokenManager(log, cfg, id, sessionRepo)
	}
	var iamOpts []iamapp.ServiceOption
	emailTemplateRegistry := notificationregistry.NewEmbedRegistry()
	emailRenderer := notificationapp.NewEmailRenderer()
	smtpDelivery := notificationsmtp.NewAdapter(notificationsmtp.Config{
		Host: cfg.SMTPHost,
		Port: cfg.SMTPPort,
		User: cfg.SMTPUser,
		Pass: cfg.SMTPPassword,
		From: cfg.SMTPFrom,
	}, nil)
	var workflowEmailNotificationService *notificationapp.EmailNotificationService
	if pool != nil && outboxSQL != nil {
		workflowEmailNotificationService = notificationapp.NewEmailNotificationService(
			notificationmysql.NewEmailNotificationRepository(pool),
			emailTemplateRegistry,
			emailRenderer,
			id,
			nil,
			notificationapp.WithTransactionalDispatch(pool, outboxSQL),
		)
	}
	iamOpts = append(iamOpts, iamapp.WithAuthFlowConfig(iamapp.AuthFlowConfig{
		WebBaseURL:              cfg.PublicWebBaseURL,
		SupportEmail:            cfg.SupportEmail,
		UserInvitationTokenTTL:  cfg.UserInvitationTokenTTL,
		EmailVerificationOTPTTL: cfg.EmailVerificationOTPTTL,
		EmailTemplateSource:     cfg.EmailTemplateSource,
		EmailTemplateRegistry:   emailTemplateRegistry,
		EmailRenderer:           emailRenderer,
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
	} else if pemFile := strings.TrimSpace(os.Getenv("LOGIN_PASSWORD_RSA_PRIVATE_KEY_PEM_FILE")); pemFile != "" {
		log.Warn("login password RSA PEM file not loaded; using plaintext login fallback",
			slog.String("path", pemFile))
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
	var adminRepo companyaccessapp.AdminRepository = cainmem.NewAdminRepository()
	if pool != nil {
		adminRepo = camysql.NewAdminRepository(pool)
		log.Info("admin access APIs using MySQL")
	}
	avatarDisk, err := mediaupload.NewDiskStorage(cfg.ResolveUserAvatarStorageDir())
	if err != nil {
		return fmt.Errorf("user avatar storage: %w", err)
	}
	avatarSigner := mediaupload.NewSigner(cfg.UserAvatarUploadSigningSecret, cfg.UserAvatarSignedURLTTL)
	var avatarRepo iamapp.AvatarRepository = iaminmem.NewAvatarRepository()
	if pool != nil {
		avatarRepo = iammysql.NewAvatarRepository(pool)
	}
	avatarSvc := iamapp.NewAvatarService(iamapp.AvatarServiceConfig{
		Repo:          avatarRepo,
		Storage:       avatarDisk,
		Signer:        avatarSigner,
		MaxBytes:      cfg.UserAvatarMaxBytes,
		AllowedTypes:  cfg.UserAvatarAllowedContentTypesSet(),
		UploadTTL:     cfg.UserAvatarSignedURLTTL,
		PublicBaseURL: cfg.PublicAPIBaseURL,
	})
	// In-app notification service
	var inAppRepo inappapp.Repository = inappmem.NewRepository()
	var inAppUserIDQuerier inappapp.UserIDQuerier
	if pool != nil {
		inAppRepo = inappmysql.NewRepository(pool)
		inAppUserIDQuerier = inappmysql.NewUserIDQuerier(pool)
	}
	inAppSvc := inappapp.NewService(inAppRepo, inAppUserIDQuerier, log)

	// Wire in-app notifier into IAM service after both are created.
	if n, ok := iamSvc.(iamapp.InAppNotifierSetter); ok {
		n.SetInAppNotifier(inAppSvc)
	}

	meHandler := iamhttp.NewMeHandler(iamHandler, identity, memberQuery, authSvc, adminRepo, pool, iamSvc, loginPWD, avatarSvc, cfg.PublicAPIBaseURL)
	meHandler.WithInAppNotifications(inAppSvc)

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
	holidayProvider := disclosureapp.HolidayCalendarProvider(fileHoliday)
	var disclosureOpts []disclosureapp.ServiceOption
	disclosureOpts = append(disclosureOpts, disclosureapp.WithWorkflowGroupsEnabled(cfg.WorkflowGroupsEnabled))
	disclosureOpts = append(disclosureOpts, disclosureapp.WithTemplateApplicabilityStrictFilter(cfg.TemplateApplicabilityStrictFilter))
	disclosureOpts = append(disclosureOpts, disclosureapp.WithDeadlineEngineV2Shadow(cfg.DeadlineEngineV2Shadow))
	tierLookup := func(ctx context.Context, userID string) string {
		if identity == nil {
			return ""
		}
		u, err := identity.GetByUserID(ctx, userID)
		if err != nil || u == nil {
			return ""
		}
		return u.SubscriptionTier
	}
	if identity != nil {
		disclosureOpts = append(disclosureOpts, disclosureapp.WithSubscriptionTierLookup(tierLookup))
	}
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
		holidayProvider = composite
		holidaySvc = holidayapp.NewService(holidayRepo, dbHoliday, id)
	}
	disclosureSvc := disclosureapp.NewService(disclosureRepo, authSvc, id, disclosureOpts...)
	deadlineCalc := disclosureapp.NewDeadlineCalculator(holidayProvider)
	var deadlineAlertsRepo deadlinealertsapp.Repository
	if pool != nil {
		deadlineAlertsRepo = deadlinealertsmysql.NewRepository(pool)
	} else if dr, ok := disclosureRepo.(*disclosureinmem.Repository); ok {
		deadlineAlertsRepo = deadlinealertsinmem.NewRepository(dr)
	} else {
		deadlineAlertsRepo = deadlinealertsinmem.NewRepository(disclosureinmem.NewRepository())
	}
	deadlineAlertsSvc := deadlinealertsapp.NewService(deadlineAlertsRepo, authSvc, deadlineCalc)
	deadlineAlertsHandler := deadlinealertshttp.NewHandler(log, deadlineAlertsSvc, tokenManager)
	var idemStore idempotency.Store
	if pool != nil {
		idemStore = idempotencymysql.NewStore(pool)
		log.Info("disclosure submit/confirm idempotency enabled (Idempotency-Key header)")
	}
	disclosureHandler := disclosurehttp.NewHandler(disclosureSvc, tokenManager, idemStore, auditSvc)
	workflowOpts := []workflowapp.ServiceOption{
		workflowapp.WithFlags(workflowapp.Flags{
			SnapshotEnabled:           cfg.WorkflowSnapshotEnabled,
			TimelineEnabled:           cfg.WorkflowTimelineEnabled,
			AssigneeResolutionEnabled: cfg.WorkflowAssigneeResolutionEnabled,
		}),
	}
	if pool != nil && cfg.WorkflowTimelineEnabled {
		workflowOpts = append(workflowOpts, workflowapp.WithMilestoneRepository(workflowmysql.NewMilestoneRepository(pool)))
	}
	workflowOpts = append(workflowOpts, workflowapp.WithRecordStatusUpdater(disclosureworkflow.NewRecordStatusAdapter(disclosureRepo)))
	var workflowMembershipLookup workflownotif.MembershipEmailLookup
	if pool != nil {
		workflowMembershipLookup = &workflownotif.SQLMembershipLookup{DB: pool}
	}
	workflowOpts = append(workflowOpts, workflowapp.WithWorkflowNotifier(workflownotif.NewWorkflowNotifier(
		smtpDelivery,
		emailTemplateRegistry,
		emailRenderer,
		workflowEmailNotificationService,
		workflowMembershipLookup,
		cfg.PublicWebBaseURL,
		cfg.AdhocEmailOutboxEnabled,
		log,
	)))
	// Wire strict assignee resolution (Sprint 1 / Batch 1). Inject only when a SQL pool exists; the
	// workflow service still gates actual use by Flags.AssigneeResolutionEnabled (default OFF). When
	// the resolver is absent or disabled, ResolveAssignees returns controlled-unresolved — never the
	// current user. No schema change; no task-creation change.
	if pool != nil {
		assigneeResolver := wfcapp.WorkflowAssigneeResolverAdapter{
			Service: wfcapp.NewAssigneeResolutionService(wfcapp.DefaultRoleRegistry(), wfcmysql.NewDirectory(pool)),
		}
		workflowOpts = append(workflowOpts, workflowapp.WithAssigneeResolver(assigneeResolver))
	}
	workflowSvc := workflowapp.NewService(workflowRepo, authSvc, id, workflowOpts...)
	if setter, ok := disclosureSvc.(interface {
		SetWorkflowBootstrap(disclosureapp.WorkflowBootstrapper)
	}); ok {
		setter.SetWorkflowBootstrap(disclosureworkflow.NewBootstrap(disclosureSvc, workflowSvc, true))
	}
	workflowHandler := workflowhttp.NewHandler(workflowSvc, tokenManager)
	// Global workflow versioning lifecycle (publish ≠ activate). Registered ONLY when the flag is ON
	// and a SQL pool exists; existing GET/PUT workflow APIs are unaffected. Versioning touches global
	// tables only — never tenant override tables, never runtime instances.
	if pool != nil && cfg.WorkflowVersioningEnabled {
		versionSvc := wfcapp.NewVersionService(wfcmysql.NewVersionRepository(pool), nil)
		readinessSvc := wfcapp.NewReadinessService(versionSvc, wfcapp.DefaultRoleRegistry())
		configSvc := wfcapp.NewConfigService(versionSvc, readinessSvc)
		wfchttp.NewHandler(versionSvc, configSvc, tokenManager).Register(mux)
	}
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
		}, reminderemail.WithTemplateRendering(cfg.EmailTemplateSource, emailTemplateRegistry, emailRenderer))),
		reminderapp.WithMetrics(reminderobserve.NewPromMetrics()),
		reminderapp.WithAuditor(reminderobserve.AuditRecorder{Svc: auditSvc, IDG: id}),
		reminderapp.WithAlertHook(reminderobserve.AlertLogger{Log: log}),
	)
	var adminOpts []companyaccessapp.AdminOption
	adminOpts = append(adminOpts, companyaccessapp.WithInviteDefaultRoleCode(cfg.InviteDefaultRoleCode))
	if pool != nil {
		alertCfgRepo := reminderalertmysql.NewAlertConfigRepository(pool)
		membershipQuerier := reminderalertmysql.NewMembershipEmailQuerier(pool)
		stepReader := reminderalertmysql.NewGlobalWorkflowStepReader(pool)
		resolver := reminderapp.NewRecipientResolver(reminderConfigRepo, stepReader, membershipQuerier, membershipQuerier, log)
		rulesReader := remindermysql.NewNotificationRulesReader(pool)
		rulesEvaluator := reminderapp.NewNotificationRulesEvaluator(rulesReader, cfg.NotificationRulesConsumerEnabled)
		tierChecker := &entitlement.Checker{
			Enabled:            cfg.SubscriptionTierEnforcementEnabled,
			ResolveUserTier:    tierLookup,
			ResolveCompanyTier: entitlement.NewMySQLCompanyTierResolver(pool),
		}
		reminderSvcOpts = append(reminderSvcOpts,
			reminderapp.WithAlertConfigRepo(alertCfgRepo),
			reminderapp.WithRecipientResolver(resolver),
			reminderapp.WithRecipientPolicyDeps(membershipQuerier, membershipQuerier),
			reminderapp.WithDispatchLogger(log),
			reminderapp.WithNotificationRulesFoundation(rulesReader, rulesEvaluator),
			reminderapp.WithTierEnforcement(tierChecker),
		)
		simDeps := reminderapp.DispatchDecisionDeps{
			EvaluatorRuntime:   rulesEvaluator,
			EvaluatorSimulate:  reminderapp.NewNotificationRulesEvaluator(rulesReader, true),
			AlertConfigRepo:    alertCfgRepo,
			RecipientResolver:  resolver,
			MembershipQuerier:  membershipQuerier,
			TaskAssigneeReader: membershipQuerier,
			StepReader:         stepReader,
			TierEnforcement:    tierChecker,
		}
		adminOpts = append(adminOpts, companyaccessapp.WithDispatchSimulator(newReminderDispatchSimulatorAdapter(reminderapp.NewDispatchSimulator(simDeps))))
	}
	reminderSvcOpts = append(reminderSvcOpts, reminderapp.WithInAppCreator(&reminderInAppBridge{svc: inAppSvc}))
	reminderSvc := reminderapp.NewService(reminderConfigRepo, reminderOccurrenceRepo, reminderAttemptRepo, reminderSvcOpts...)
	reminderHandler := reminderhttp.NewHandler(reminderSvc, tokenManager, "", cfg.Env)
	if pool != nil {
		mysqlAdmin := adminRepo.(*camysql.AdminRepository)
		adminOpts = append(adminOpts,
			companyaccessapp.WithInvitationMailer(&iamInvitationMailer{iam: iamSvc}),
			companyaccessapp.WithEmailVerificationIssuer(&iamEmailVerificationIssuer{iam: iamSvc}),
			companyaccessapp.WithInvitationTTL(cfg.UserInvitationTokenTTL),
			companyaccessapp.WithConflictSnapshotReader(camysql.NewConflictSnapshotReader(mysqlAdmin)),
			companyaccessapp.WithDependencyReader(camysql.NewDependencyReader(mysqlAdmin)),
			companyaccessapp.WithCompanyTierLookup(entitlement.NewMySQLCompanyTierResolver(pool)),
		)
	} else if memRepo, ok := adminRepo.(*cainmem.AdminRepository); ok {
		adminOpts = append(adminOpts,
			companyaccessapp.WithConflictSnapshotReader(cainmem.NewConflictSnapshotReader(memRepo)),
			companyaccessapp.WithDependencyReader(cainmem.NewDependencyReader(memRepo)),
		)
	}
	adminOpts = append(adminOpts, companyaccessapp.WithSubscriptionTierLookup(tierLookup))
	adminOpts = append(adminOpts, companyaccessapp.WithNotificationRulesConsumerEnabled(cfg.NotificationRulesConsumerEnabled))
	adminOpts = append(adminOpts, companyaccessapp.WithSubscriptionTierEnforcementEnabled(cfg.SubscriptionTierEnforcementEnabled))
	adminOpts = append(adminOpts, companyaccessapp.WithAuditRepository(auditRepo))
	adminOpts = append(adminOpts, companyaccessapp.WithEffectiveAccessCache(projectionStore))
	adminSvc := companyaccessapp.NewAdminService(adminRepo, authSvc, id, adminOpts...)
	adminHandler := companyaccesshttp.NewAdminHandler(adminSvc, tokenManager, auditSvc)
	adminHandler.WithTokenIssuer(tokenManager, sessionRepo)
	adminHandler.WithAccountPasswordChange(iamSvc, loginPWD)
	if idemStore != nil {
		adminHandler.WithIdempotency(idemStore, cfg.CompanyProvisionIdempotencyRequired)
	}
	adminHandler.WithSelfCreateEnabled(cfg.CompanySelfCreateEnabled)
	adminHandler.WithListedCompaniesLookup(listedCompaniesSvc)
	var alertConfigSvc platformcmsapp.AlertConfigService
	if pool != nil {
		alertConfigSvc = platformcmsapp.NewAlertConfigService(
			reminderalertmysql.NewAlertConfigRepository(pool),
			notificationregistry.NewEmbedRegistry(),
			pool,
		)
	}
	platformCMSHandler := platformcmshttp.NewHandler(tokenManager, authSvc, adminSvc, iamSvc, auditSvc, auditRepo, disclosureSvc, disclosureRepo, holidaySvc, listedCompaniesSvc, alertConfigSvc, platformcmshttp.MediaOptions{
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

		// Durable email pipeline (Batch 2 cutover): only constructible when a DB +
		// outbox repo are available. nil notificationService disables the durable
		// branch in adhocnotif (it falls back to legacy-only, matching Case A).
		var adhocEmailNotificationService *notificationapp.EmailNotificationService
		if pool != nil && outboxSQL != nil {
			adhocEmailNotificationService = notificationapp.NewEmailNotificationService(
				notificationmysql.NewEmailNotificationRepository(pool),
				emailTemplateRegistry,
				emailRenderer,
				id,
				nil,
				notificationapp.WithTransactionalDispatch(pool, outboxSQL),
			)
		}
		var adhocMetrics adhocapp.Metrics = adhocapp.NewNoopMetrics()
		if cfg.AdhocEmailMetricsEnabled {
			adhocMetrics = adhocobserve.NewPromMetrics()
		}
		var proposalNotifier adhocapp.ProposalNotifier = adhocnotif.New(
			inAppSvc, smtpDelivery, emailTemplateRegistry, emailRenderer, adhocEmailNotificationService,
			cfg.PublicWebBaseURL, cfg.EmailShadowMode, cfg.AdhocEmailOutboxEnabled, cfg.AdhocEmailShadowRecipient,
			adhocMetrics, log,
		)
		adhocSvc := adhocapp.NewService(adhocRepo, recordCreator, typeCatalog, id, cfg.WorkflowAdhocAutoApproveEnabled, authSvc, membershipValidator, proposalNotifier, adhocMetrics)
		adhocHandler = adhochttp.NewHandler(log, adhocSvc, tokenManager, idemStore)
		log.Info("ad-hoc proposal module enabled")
	}

	// Batch 2B Observability (Option B): expose DB-derived email-pipeline gauges
	// on the API's existing /metrics. The worker (where EmailDispatchHandler runs)
	// has no metrics endpoint; the API + shared MySQL are the authoritative alert
	// source for failed_permanent / backlog / stale-processing. Scrape-driven, so
	// no new poller/scheduler. prometheus.Register (not MustRegister) tolerates
	// repeated New() builds in tests without panicking.
	if pool != nil {
		collector := notificationobserve.NewEmailDeliveryCollector(
			notificationobserve.NewDBCountSource(pool), cfg.OutboxVisibilityTimeout)
		if err := prometheus.Register(collector); err != nil {
			var already prometheus.AlreadyRegisteredError
			if !errors.As(err, &already) {
				log.Warn("email delivery metrics collector registration failed", slog.String("err", err.Error()))
			}
		} else {
			log.Info("email delivery metrics collector registered")
		}
	}

	// Reminder Reliability Hardening — Observability: same Option B pattern for the
	// reminder pipeline. DB-derived gauges (backlog / failed / stuck-dispatching) on
	// the API /metrics; the worker reminder loop has no metrics endpoint.
	if pool != nil {
		reminderCollector := reminderobserve.NewReminderObservabilityCollector(
			reminderobserve.NewDBCountSource(pool), cfg.ReminderVisibilityTimeout)
		if err := prometheus.Register(reminderCollector); err != nil {
			var already prometheus.AlreadyRegisteredError
			if !errors.As(err, &already) {
				log.Warn("reminder metrics collector registration failed", slog.String("err", err.Error()))
			}
		} else {
			log.Info("reminder metrics collector registered")
		}
	}

	return muxRegisterHealthAndIAM(mux, log, sqlDB, iamHandler, meHandler, authHandler, disclosureHandler, workflowHandler, notificationHandler, reminderHandler, adminHandler, platformCMSHandler, adhocHandler, deadlineAlertsHandler)
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
	deadlineAlertsHandler *deadlinealertshttp.Handler,
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
	deadlineAlertsHandler.Register(mux)
	return nil
}

// reminderInAppBridge adapts inappapp.Service to reminderapp.InAppNotificationCreator.
type reminderInAppBridge struct {
	svc inappapp.Service
}

func (b *reminderInAppBridge) CreateForReminderDispatch(ctx context.Context, c reminderapp.DispatchCandidate) error {
	kind := inappapp.KindReminderDeadline
	if c.ScopeType == reminderapp.ScopeTypeWorkflowStep {
		kind = inappapp.KindReminderWorkflow
	}
	title := "Nhắc nhở CBTT"
	if v, ok := c.TemplatePayload["disclosure_title"].(string); ok && v != "" {
		if c.ScopeType == reminderapp.ScopeTypeWorkflowStep {
			step := ""
			if s, ok2 := c.TemplatePayload["step_name"].(string); ok2 && s != "" {
				step = s
			}
			if step != "" {
				title = "Bước phê duyệt đến hạn: " + step
			} else {
				title = "Bước phê duyệt đến hạn: " + v
			}
		} else {
			title = "Sắp đến hạn CBTT: " + v
		}
	}
	body := ""
	if v, ok := c.TemplatePayload["due_date"].(string); ok && v != "" {
		body = "Deadline: " + v
	}
	// resource_id is disclosure_id for DISCLOSURE scope; empty for WORKFLOW_STEP (no direct link)
	resourceID := ""
	if c.ScopeType == reminderapp.ScopeTypeDisclosure {
		resourceID = c.ScopeID
	}
	return b.svc.CreateForReminder(ctx, inappapp.ReminderInAppRequest{
		CompanyID:       c.CompanyID,
		Kind:            kind,
		Title:           title,
		Body:            body,
		ResourceType:    inappapp.ResourceTypeDisclosure,
		ResourceID:      resourceID,
		RecipientEmails: c.RecipientEmails,
	})
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
