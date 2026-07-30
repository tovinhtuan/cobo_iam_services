package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	adhocrecord "github.com/cobo/cobo_iam_services/internal/adhoc/infra/disclosure"
	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	oneshot "github.com/cobo/cobo_iam_services/internal/disclosure/app/periodic_oneshot"
	disclosuremysql "github.com/cobo/cobo_iam_services/internal/disclosure/infra/mysql"
	"github.com/cobo/cobo_iam_services/internal/platform/config"
	"github.com/cobo/cobo_iam_services/internal/platform/db"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
	workflowapp "github.com/cobo/cobo_iam_services/internal/workflow/app"
	workflowmysql "github.com/cobo/cobo_iam_services/internal/workflow/infra/mysql"
)

// Guarded DEV CLI: default dry-run preview. Apply requires --apply + --confirm-token.
func main() {
	var (
		mode         = flag.String("mode", "preview", "preview|apply|verify")
		environment  = flag.String("environment", "", "must be DEV")
		database     = flag.String("database", "", "must be cobo_iam")
		hostAlias    = flag.String("host", "127.0.0.1", "approved host alias")
		port         = flag.String("port", "3306", "approved port")
		typeID       = flag.String("type-id", "", "exact allowlisted type_id")
		companyID    = flag.String("company-id", "", "exact allowlisted company_id")
		period       = flag.String("period", "", "exact allowlisted period YYYY-MM")
		apply        = flag.Bool("apply", false, "explicit mutation switch")
		confirmToken = flag.String("confirm-token", "", "token from preview (required for apply)")
	)
	flag.Parse()

	wantApply := *apply || strings.EqualFold(*mode, "apply")
	wantVerify := strings.EqualFold(*mode, "verify")

	cfg, err := config.Load()
	if err != nil {
		fatalf("config: %v", err)
	}
	if strings.TrimSpace(cfg.MySQLDSN) == "" {
		fatalf("MYSQL_DSN required")
	}
	dbName, dsnHost, dsnPort, err := oneshot.ParseDSNFingerprint(cfg.MySQLDSN)
	if err != nil {
		fatalf("dsn fingerprint: %v", err)
	}
	// Prefer explicit flags; fall back to DSN fingerprint for database check.
	if strings.TrimSpace(*database) == "" {
		*database = dbName
	}
	if strings.TrimSpace(*hostAlias) == "" {
		*hostAlias = dsnHost
	}
	if strings.TrimSpace(*port) == "" {
		*port = dsnPort
	}
	env := oneshot.EnvGuard{Environment: *environment, Database: *database, HostAlias: *hostAlias, Port: *port}
	if err := env.Validate(); err != nil {
		fatalf("env: %v", err)
	}
	if dbName != "cobo_iam" {
		fatalf("dsn database fingerprint must be cobo_iam (got %q)", dbName)
	}

	scope := oneshot.Scope{TypeID: *typeID, CompanyID: *companyID, Period: *period}
	if err := oneshot.ValidateAllowlist(scope); err != nil {
		fatalf("%v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	sqlDB, err := db.OpenMySQL(ctx, cfg.MySQLDSN)
	if err != nil {
		fatalf("mysql: %v", err)
	}
	defer sqlDB.Close()

	eng, err := newEngine(ctx, cfg, sqlDB, env)
	if err != nil {
		fatalf("engine: %v", err)
	}

	if wantVerify {
		rep, err := eng.Preview(ctx, scope)
		encode(rep)
		if err != nil {
			os.Exit(2)
		}
		existingCycle, _ := rep.Existing["cycle"].(bool)
		existingRecord, _ := rep.Existing["record"].(bool)
		if !existingCycle || !existingRecord {
			fatalf("verify failed: cycle/record missing")
		}
		if rep.Resolved["due_date"] != oneshot.ExpectedDueDate {
			fatalf("verify failed: due_date mismatch")
		}
		return
	}

	if !wantApply {
		rep, err := eng.Preview(ctx, scope)
		encode(rep)
		if err != nil {
			os.Exit(2)
		}
		return
	}

	if strings.TrimSpace(*confirmToken) == "" {
		fatalf("--confirm-token required for apply")
	}
	rep, err := eng.Apply(ctx, scope, *confirmToken)
	encode(rep)
	if err != nil {
		os.Exit(2)
	}
}

func newEngine(ctx context.Context, cfg config.Config, sqlDB *sql.DB, env oneshot.EnvGuard) (*oneshot.Engine, error) {
	_ = ctx
	repo := disclosuremysql.NewRepository(sqlDB)
	calc := disclosureapp.NewDeadlineCalculator(nil)
	disclosureSvc := disclosureapp.NewService(
		repo,
		nil, // worker/CLI mode: auth nil
		idgen.UUIDv7Generator{},
		disclosureapp.WithTemplateApplicabilityStrictFilter(cfg.TemplateApplicabilityStrictFilter),
		disclosureapp.WithDeadlineEngineV2Shadow(cfg.DeadlineEngineV2Shadow),
	)
	var workflowSvc workflowapp.Service
	if cfg.WorkflowSnapshotEnabled {
		workflowRepo := workflowmysql.NewRepository(sqlDB)
		workflowSvc = workflowapp.NewService(workflowRepo, nil, idgen.UUIDv7Generator{}, workflowapp.WithFlags(workflowapp.Flags{
			SnapshotEnabled: true,
			TimelineEnabled: cfg.WorkflowTimelineEnabled,
		}))
	}
	creator := adhocrecord.NewRecordCreatorAdapter(disclosureSvc, workflowSvc, workflowSvc != nil)
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		loc = time.FixedZone("Asia/Ho_Chi_Minh", 7*3600)
	}
	domain := &oneshot.ProductionDomain{
		Repo:    repo,
		Creator: creator,
		Calc:    calc,
		IDGen:   idgen.UUIDv7Generator{},
		Loc:     loc,
	}
	return &oneshot.Engine{Env: env, Domain: domain}, nil
}

func encode(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
