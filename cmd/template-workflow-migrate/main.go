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

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	disclosuremysql "github.com/cobo/cobo_iam_services/internal/disclosure/infra/mysql"
	"github.com/cobo/cobo_iam_services/internal/platform/config"
	"github.com/cobo/cobo_iam_services/internal/platform/db"
)

type inventoryRow struct {
	TypeID            string `json:"type_id"`
	CompanyID         string `json:"company_id,omitempty"`
	TemplateVersionNo int    `json:"template_version_no"`
	Classification    string `json:"classification"`
	LegacySource      string `json:"legacy_source"`
	LegacyVersionNo   int    `json:"legacy_version_no"`
	SemanticHash      string `json:"semantic_hash"`
	CandidateHash     string `json:"publication_candidate_hash"`
	Result            string `json:"result"`
}

func main() {
	mode := flag.String("mode", "dry-run", "dry-run|apply|rollback")
	environment := flag.String("environment", "", "must be DEV")
	confirm := flag.String("confirm", "", "apply/rollback requires MIGRATE_TEMPLATE_WORKFLOW_DEV")
	batchSize := flag.Int("batch-size", 25, "1..100")
	flag.Parse()

	if !strings.EqualFold(strings.TrimSpace(*environment), "DEV") {
		fatalf("--environment DEV is required")
	}
	if *batchSize < 1 || *batchSize > 100 {
		fatalf("--batch-size must be 1..100")
	}
	normalizedMode := strings.ToLower(strings.TrimSpace(*mode))
	if normalizedMode != "dry-run" && normalizedMode != "apply" && normalizedMode != "rollback" {
		fatalf("invalid --mode")
	}
	if normalizedMode != "dry-run" && *confirm != "MIGRATE_TEMPLATE_WORKFLOW_DEV" {
		fatalf("mutation requires --confirm MIGRATE_TEMPLATE_WORKFLOW_DEV")
	}

	cfg, err := config.Load()
	if err != nil {
		fatalf("config: %v", err)
	}
	if strings.TrimSpace(cfg.MySQLDSN) == "" {
		fatalf("MYSQL_DSN required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	sqlDB, err := db.OpenMySQL(ctx, cfg.MySQLDSN)
	if err != nil {
		fatalf("mysql: %v", err)
	}
	defer sqlDB.Close()

	if normalizedMode == "rollback" {
		result, err := sqlDB.ExecContext(ctx, `
			UPDATE disclosure_type_versions v
			INNER JOIN disclosure_types t ON t.type_id = v.type_id AND t.active_version_no = v.version_no
			SET v.workflow_authority_mode = 'LEGACY_COMPAT'
			WHERE v.workflow_authority_mode = 'TEMPLATE_PINNED'
		`)
		if err != nil {
			fatalf("rollback: %v", err)
		}
		n, _ := result.RowsAffected()
		writeJSON(map[string]any{"mode": normalizedMode, "rows": n, "pinned_manifest_deleted": false})
		return
	}

	rows, err := activeTemplates(ctx, sqlDB)
	if err != nil {
		fatalf("inventory: %v", err)
	}
	repo := disclosuremysql.NewRepository(sqlDB)
	report := make([]inventoryRow, 0, len(rows))
	failed := 0
	for index, row := range rows {
		detail, err := repo.GetTypeVersionDetail(ctx, row.CompanyID, row.TypeID, row.TemplateVersionNo)
		if err != nil {
			fatalf("detail %s: %v", row.TypeID, err)
		}
		globalSteps, globalVersion, globalOK, err := repo.GetActiveGlobalWorkflow(ctx, row.TypeID)
		if err != nil {
			fatalf("global %s: %v", row.TypeID, err)
		}
		enterpriseSteps := disclosureapp.ExtractTemplateWorkflow(detail.Blocks)
		classification := classify(globalSteps, globalOK, enterpriseSteps)
		selected := enterpriseSteps
		source := disclosureapp.WorkflowPublicationSourceTemplate
		sourceVersion := row.TemplateVersionNo
		if globalOK {
			selected = globalSteps
			source = disclosureapp.WorkflowPublicationSourceGlobal
			sourceVersion = globalVersion
		}
		item := inventoryRow{
			TypeID: row.TypeID, CompanyID: row.CompanyID, TemplateVersionNo: row.TemplateVersionNo,
			Classification: classification, LegacySource: source, LegacyVersionNo: sourceVersion,
		}
		if len(selected) == 0 {
			item.Result = "FAILED_REQUIRED_WORKFLOW_NONE"
			failed++
			report = append(report, item)
			continue
		}
		candidate, err := disclosureapp.BuildLegacyMigrationPublicationCandidate(*detail, selected, source, sourceVersion)
		if err != nil {
			fatalf("candidate %s: %v", row.TypeID, err)
		}
		item.SemanticHash = candidate.ManifestHash
		item.CandidateHash = candidate.CandidateHash
		item.Result = "DRY_RUN_OK"
		if normalizedMode == "apply" {
			if err := applyOne(ctx, sqlDB, row.TypeID, row.TemplateVersionNo, candidate); err != nil {
				item.Result = "APPLY_FAILED: " + err.Error()
				failed++
			} else {
				item.Result = "MIGRATED_OK"
			}
		}
		report = append(report, item)
		if normalizedMode == "apply" && (index+1)%*batchSize == 0 && failed > 0 {
			break
		}
	}
	writeJSON(map[string]any{
		"mode": normalizedMode, "active_templates": len(rows), "migrated_failed": failed,
		"items": report,
	})
	if failed > 0 {
		os.Exit(2)
	}
}

type activeTemplate struct {
	TypeID            string
	CompanyID         string
	TemplateVersionNo int
}

func activeTemplates(ctx context.Context, db *sql.DB) ([]activeTemplate, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT t.type_id, COALESCE(t.company_id, ''), t.active_version_no
		FROM disclosure_types t
		WHERE t.status = 'active' AND t.active_version_no > 0
		ORDER BY t.type_id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []activeTemplate
	for rows.Next() {
		var row activeTemplate
		if err := rows.Scan(&row.TypeID, &row.CompanyID, &row.TemplateVersionNo); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func classify(global []disclosureapp.WorkflowStepDTO, globalOK bool, enterprise []disclosureapp.WorkflowStepDTO) string {
	if !globalOK && len(enterprise) == 0 {
		return "NONE"
	}
	if globalOK && len(enterprise) == 0 {
		return "GLOBAL_ONLY"
	}
	if !globalOK {
		return "ENTERPRISE_ONLY"
	}
	_, _, globalHash, _ := disclosureapp.CanonicalWorkflowPublication(global)
	_, _, enterpriseHash, _ := disclosureapp.CanonicalWorkflowPublication(enterprise)
	if globalHash == enterpriseHash {
		return "BOTH_EQUAL"
	}
	return "BOTH_DIFFERENT"
}

func applyOne(ctx context.Context, db *sql.DB, typeID string, versionNo int, candidate disclosureapp.TemplatePublicationCandidate) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var currentMode string
	if err := tx.QueryRowContext(ctx, `
		SELECT workflow_authority_mode FROM disclosure_type_versions
		WHERE type_id = ? AND version_no = ? FOR UPDATE
	`, typeID, versionNo).Scan(&currentMode); err != nil {
		return err
	}
	if currentMode == disclosureapp.WorkflowAuthorityTemplatePinned {
		var existingHash string
		if err := tx.QueryRowContext(ctx, `
			SELECT COALESCE(workflow_semantic_hash, '') FROM disclosure_type_versions
			WHERE type_id = ? AND version_no = ?
		`, typeID, versionNo).Scan(&existingHash); err != nil {
			return err
		}
		if existingHash != candidate.ManifestHash {
			return fmt.Errorf("already pinned with different semantic hash")
		}
		return tx.Commit()
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE disclosure_type_versions
		SET workflow_authority_mode = 'TEMPLATE_PINNED',
		    workflow_manifest_json = CAST(? AS JSON),
		    workflow_manifest_schema_version = ?,
		    workflow_source = ?,
		    workflow_source_version_no = NULLIF(?, 0),
		    workflow_semantic_hash = ?,
		    publication_candidate_hash = ?
		WHERE type_id = ? AND version_no = ?
	`, string(candidate.ManifestJSON), disclosureapp.WorkflowManifestSchemaVersion,
		candidate.Source, candidate.SourceVersionNo, candidate.ManifestHash, candidate.CandidateHash,
		typeID, versionNo); err != nil {
		return err
	}
	var readback string
	if err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(workflow_semantic_hash, '') FROM disclosure_type_versions
		WHERE type_id = ? AND version_no = ?
	`, typeID, versionNo).Scan(&readback); err != nil {
		return err
	}
	if readback != candidate.ManifestHash {
		return fmt.Errorf("read-back semantic hash mismatch")
	}
	return tx.Commit()
}

func writeJSON(value any) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(value)
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(2)
}
