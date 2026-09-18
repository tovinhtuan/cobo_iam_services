// QA-only helper for G4 release verification.
// Calls production CreateAndSubmitRecordWithPlannedDate (SkipCompanySubmit → Draft + B1 snaps).
// NOT product runtime code. Do not ship in production image.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	adhocrecord "github.com/cobo/cobo_iam_services/internal/adhoc/infra/disclosure"
	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	disclosuremysql "github.com/cobo/cobo_iam_services/internal/disclosure/infra/mysql"
	"github.com/cobo/cobo_iam_services/internal/platform/db"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
	workflowapp "github.com/cobo/cobo_iam_services/internal/workflow/app"
	workflowmysql "github.com/cobo/cobo_iam_services/internal/workflow/infra/mysql"
)

func main() {
	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		fatalf("MYSQL_DSN required")
	}
	typeID := envOr("TYPE_ID", "qa-evidence-recovery-e2e-20260918")
	companyID := envOr("COMPANY_ID", "c_001")
	planned := envOr("PLANNED_DATE", time.Now().Add(7*24*time.Hour).Format("2006-01-02"))
	title := envOr("TITLE", "G4 Fixture B "+time.Now().UTC().Format("20060102T150405Z"))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	sqlDB, err := db.OpenMySQL(ctx, dsn)
	if err != nil {
		fatalf("mysql: %v", err)
	}
	defer sqlDB.Close()

	repo := disclosuremysql.NewRepository(sqlDB)
	disclosureSvc := disclosureapp.NewService(repo, nil, idgen.UUIDv7Generator{})
	workflowRepo := workflowmysql.NewRepository(sqlDB)
	workflowSvc := workflowapp.NewService(workflowRepo, nil, idgen.UUIDv7Generator{}, workflowapp.WithFlags(workflowapp.Flags{
		SnapshotEnabled: true,
	}))
	creator := adhocrecord.NewRecordCreatorAdapter(disclosureSvc, workflowSvc, true)

	t0 := time.Now()
	recordID, wiID, err := creator.CreateAndSubmitRecordWithPlannedDate(
		ctx, companyID, typeID, "m_system_worker", title, &t0, planned,
	)
	if err != nil {
		fatalf("materialize: %v", err)
	}
	fmt.Printf("RECORD_ID=%s\nWORKFLOW_INSTANCE_ID=%s\nPLANNED_DATE=%s\nTYPE_ID=%s\nPATH=CreateAndSubmitRecordWithPlannedDate\n", recordID, wiID, planned, typeID)
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
