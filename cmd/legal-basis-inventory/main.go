package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"

	inventory "github.com/cobo/cobo_iam_services/internal/disclosure/app/legal_basis_inventory"
)

// legal-basis-inventory — Phase 12.6A read-only DEV inventory + in-memory dry-run.
//
// Required env (prefer):
//
//	MYSQL_READONLY_DSN or LEGAL_BASIS_INVENTORY_DSN
//
// Optional:
//
//	--dsn-file PATH   (file contains DSN only; not committed)
//	--out-dir PATH    (default docs/ai-cache/... under cwd)
//
// Safety: refuses to proceed unless session/grants look read-only.
// No --apply flag exists.

func main() {
	outDir := flag.String("out-dir", "", "directory for JSON reports")
	dsnFile := flag.String("dsn-file", "", "path to file containing read-only DSN")
	batchSize := flag.Int("batch", 200, "keyset page size (100–500)")
	timeout := flag.Duration("timeout", 10*time.Minute, "overall context timeout")
	flag.Parse()

	dsn, src, err := loadDSN(*dsnFile)
	if err != nil {
		fatalf("%v", err)
	}
	if *batchSize < 100 || *batchSize > 500 {
		fatalf("batch must be 100–500")
	}
	if *outDir == "" {
		*outDir = "docs/ai-cache/legal-basis-contract-alignment-plan-2026-07-29"
	}
	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		fatalf("mkdir: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		fatalf("open: %v", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(2)
	db.SetConnMaxLifetime(5 * time.Minute)

	safety, err := proveReadOnly(ctx, db, src)
	if err != nil {
		_ = writeJSON(filepath.Join(*outDir, "phase-12-6a-read-only-safety.md"), []byte("# BLOCKED\n\n"+err.Error()+"\n"))
		fatalf("READ-ONLY GATE FAILED: %v", err)
	}

	qlog := &queryLog{}
	start := time.Now()

	rows, sqlCounts, err := scanAll(ctx, db, *batchSize, qlog)
	if err != nil {
		fatalf("scan: %v", err)
	}

	results := make([]inventory.Result, 0, len(rows))
	for _, rec := range rows {
		results = append(results, inventory.ClassifyRecord(rec))
	}
	recon := inventory.Reconcile(results)
	recon.AnalyzerMatch = matchSQLApprox(sqlCounts, recon)
	if !recon.AnalyzerMatch || recon.SumGroups != recon.Total || recon.Unclassified != 0 {
		_ = writeJSON(filepath.Join(*outDir, "phase-12-6a-group-reconciliation.json"), mustJSON(recon))
		fatalf("FAIL_RECONCILIATION: analyzer=%+v sqlApprox=%+v match=%v", recon, sqlCounts, recon.AnalyzerMatch)
	}

	idempo := runIdempotency(rows)
	reports := buildReports(results, recon, sqlCounts, safety, qlog, start, *batchSize, idempo)

	writeAll(*outDir, reports, safety, qlog)
	fmt.Fprintf(os.Stderr, "OK inventory total=%d A=%d B=%d C=%d D=%d E=%d writes=0 out=%s\n",
		recon.Total, recon.Groups[inventory.GroupA], recon.Groups[inventory.GroupB],
		recon.Groups[inventory.GroupC], recon.Groups[inventory.GroupD], recon.Groups[inventory.GroupE], *outDir)
}

func loadDSN(dsnFile string) (dsn, source string, err error) {
	if dsnFile != "" {
		b, e := os.ReadFile(dsnFile)
		if e != nil {
			return "", "", fmt.Errorf("dsn-file: %w", e)
		}
		dsn = strings.TrimSpace(string(b))
		source = "dsn-file"
	} else if v := strings.TrimSpace(os.Getenv("MYSQL_READONLY_DSN")); v != "" {
		dsn, source = v, "MYSQL_READONLY_DSN"
	} else if v := strings.TrimSpace(os.Getenv("LEGAL_BASIS_INVENTORY_DSN")); v != "" {
		dsn, source = v, "LEGAL_BASIS_INVENTORY_DSN"
	} else {
		return "", "", fmt.Errorf("BLOCKED_READ_ONLY_ACCESS: set MYSQL_READONLY_DSN or LEGAL_BASIS_INVENTORY_DSN (or --dsn-file); refusing MYSQL_DSN write-capable default")
	}
	if dsn == "" {
		return "", "", fmt.Errorf("empty DSN")
	}
	// Reject obvious root DSNs
	lower := strings.ToLower(dsn)
	if strings.HasPrefix(lower, "root:") {
		return "", "", fmt.Errorf("BLOCKED_READ_ONLY_ACCESS: root DSN refused")
	}
	return dsn, source, nil
}

type safetyProof struct {
	ConnectionSource       string `json:"connectionSource"`
	EngineVersion          string `json:"engineVersion"`
	TransactionReadOnly    string `json:"transactionReadOnly"`
	SessionReadOnly        string `json:"sessionReadOnly"`
	GrantsRedacted         string `json:"grantsRedacted"`
	ReadOnlyGranted        bool   `json:"readOnlyGranted"`
	WritePrivilegeDetected bool   `json:"writePrivilegeDetected"`
}

func proveReadOnly(ctx context.Context, db *sql.DB, src string) (safetyProof, error) {
	var s safetyProof
	s.ConnectionSource = src
	if err := db.QueryRowContext(ctx, "SELECT VERSION()").Scan(&s.EngineVersion); err != nil {
		return s, err
	}
	_ = db.QueryRowContext(ctx, "SELECT @@transaction_read_only").Scan(&s.TransactionReadOnly)
	_ = db.QueryRowContext(ctx, "SELECT @@session.transaction_read_only").Scan(&s.SessionReadOnly)

	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return s, fmt.Errorf("START TRANSACTION READ ONLY failed: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return s, err
	}

	grants, err := readGrants(ctx, db)
	if err != nil {
		return s, err
	}
	s.GrantsRedacted = redactGrants(grants)
	s.WritePrivilegeDetected = grantsLookWritable(grants)
	s.ReadOnlyGranted = !s.WritePrivilegeDetected
	if s.WritePrivilegeDetected {
		return s, fmt.Errorf("BLOCKED_READ_ONLY_ACCESS: grants appear to allow INSERT/UPDATE/DELETE/DDL; use a read-only MySQL user")
	}
	return s, nil
}

func readGrants(ctx context.Context, db *sql.DB) ([]string, error) {
	rows, err := db.QueryContext(ctx, "SHOW GRANTS")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var g string
		if err := rows.Scan(&g); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func grantsLookWritable(grants []string) bool {
	joined := strings.ToUpper(strings.Join(grants, "\n"))
	// ALL PRIVILEGES on *.* or on schema is write
	if strings.Contains(joined, "ALL PRIVILEGES") {
		return true
	}
	for _, w := range []string{"INSERT", "UPDATE", "DELETE", "CREATE", "DROP", "ALTER", "TRUNCATE", "REPLACE", "GRANT"} {
		if strings.Contains(joined, w+",") || strings.Contains(joined, w+" ") || strings.Contains(joined, ","+w) {
			// SELECT-only users sometimes have GRANT USAGE — ignore bare grant option usage
			if w == "GRANT" && strings.Contains(joined, "USAGE") && !strings.Contains(joined, "GRANT OPTION") {
				continue
			}
			return true
		}
	}
	return false
}

func redactGrants(grants []string) string {
	var b strings.Builder
	for _, g := range grants {
		// strip identified by password remnants if any
		line := g
		if i := strings.Index(strings.ToLower(line), " identified by"); i >= 0 {
			line = line[:i] + " IDENTIFIED BY ***"
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

type sqlCountResult struct {
	Total int `json:"total"`
	A     int `json:"A"`
	B     int `json:"B"`
	C     int `json:"C"`
	D     int `json:"D"`
	E     int `json:"E"`
}

func (s sqlCountResult) Match(r inventory.Reconciliation) bool {
	return matchSQLApprox(s, r)
}

func matchSQLApprox(sql sqlCountResult, recon inventory.Reconciliation) bool {
	bothSQL := sql.C // BOTH(C+D) stored in C by scanAll
	bothAna := recon.Groups[inventory.GroupC] + recon.Groups[inventory.GroupD]
	return sql.Total == recon.Total &&
		sql.A == recon.Groups[inventory.GroupA] &&
		sql.B == recon.Groups[inventory.GroupB] &&
		sql.E == recon.Groups[inventory.GroupE] &&
		bothSQL == bothAna
}

type queryLog struct {
	Entries []map[string]any `json:"entries"`
	Selects int              `json:"selectStatements"`
	Writes  int              `json:"writeStatements"`
}

func (q *queryLog) add(purpose, qtype string, rows int, dur time.Duration) {
	q.Entries = append(q.Entries, map[string]any{
		"sequence":   len(q.Entries) + 1,
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
		"purpose":    purpose,
		"queryType":  qtype,
		"rows":       rows,
		"durationMs": dur.Milliseconds(),
		"write":      false,
	})
	if qtype == "SELECT" || qtype == "METADATA" {
		q.Selects++
	}
}

func scanAll(ctx context.Context, db *sql.DB, batch int, qlog *queryLog) ([]inventory.Record, sqlCountResult, error) {
	// Independent SQL aggregate approximating groups using same predicate shapes where SQL-feasible.
	// Full C/D equality of projection is evaluated in analyzer; SQL assigns
	// structured+flat both non-empty to bucket "BOTH" then analyzer splits C/D.
	var counts sqlCountResult
	start := time.Now()
	err := db.QueryRowContext(ctx, `
SELECT COUNT(*) FROM disclosure_type_versions v
INNER JOIN disclosure_types t ON t.type_id = v.type_id
`).Scan(&counts.Total)
	qlog.add("count_total", "SELECT", 1, time.Since(start))
	if err != nil {
		return nil, counts, err
	}

	var records []inventory.Record
	var lastType string
	var lastVer int
	for {
		start = time.Now()
		rows, err := db.QueryContext(ctx, `
SELECT v.type_id, v.version_no,
       COALESCE(t.company_id, '') AS company_id,
       COALESCE(t.status, '') AS type_status,
       COALESCE(t.active_version_no, 0) AS active_version_no,
       COALESCE(v.is_released, 0) AS is_released,
       COALESCE(v.legal_basis, '') AS legal_basis,
       v.legal_bases_json
FROM disclosure_type_versions v
INNER JOIN disclosure_types t ON t.type_id = v.type_id
WHERE (v.type_id > ?) OR (v.type_id = ? AND v.version_no > ?)
ORDER BY v.type_id ASC, v.version_no ASC
LIMIT ?
`, lastType, lastType, lastVer, batch)
		if err != nil {
			return nil, counts, err
		}
		n := 0
		var page []inventory.Record
		for rows.Next() {
			var rec inventory.Record
			var released int
			var jsonRaw sql.NullString
			if err := rows.Scan(
				&rec.TypeID, &rec.VersionNo, &rec.CompanyID, &rec.TypeStatus,
				&rec.ActiveVersionNo, &released, &rec.LegalBasis, &jsonRaw,
			); err != nil {
				rows.Close()
				return nil, counts, err
			}
			rec.IsReleased = released != 0
			if jsonRaw.Valid {
				rec.LegalBasesJSON = []byte(jsonRaw.String)
			}
			page = append(page, rec)
			n++
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, counts, err
		}
		rows.Close()
		qlog.add("keyset_page", "SELECT", n, time.Since(start))
		if n == 0 {
			break
		}
		records = append(records, page...)
		last := page[len(page)-1]
		lastType, lastVer = last.TypeID, last.VersionNo
		if n < batch {
			break
		}
	}

	if len(records) != counts.Total {
		return nil, counts, fmt.Errorf("scan row count %d != SQL COUNT %d", len(records), counts.Total)
	}

	// Second-pass SQL-ish buckets on already scanned data using only flat emptiness + JSON non-null length heuristics
	// True A–E for C/D requires analyzer; we rebuild sqlCounts from analyzer after classify —
	// here we set sqlCounts by re-walking with Classify locally? Contract requires TWO independent ways:
	// 1) SQL aggregate 2) local analyzer.
	// For C vs D we run a pure SQL approximation that still partitions 5 groups by computing projection in Go is NOT independent.
	// Independent SQL approach:
	// - flat_empty / flat_nonempty
	// - json_null_or_empty_array vs non-empty array via JSON_LENGTH
	// Then map:
	//   flat+ && (json empty) => A
	//   flat- && (json nonempty) => B
	//   flat- && (json empty) => E
	//   flat+ && (json nonempty) => BOTH (C+D) — then SQL cannot split without projection UDFs.
	// To keep independence for A/B/E and (C+D), and require analyzer for C vs D split that still reconciles A+B+C+D+E=total:
	// We compute A,B,E via SQL and require analyzer A,B,E match; C+D count must equal SQL BOTH.

	start = time.Now()
	err = db.QueryRowContext(ctx, `
SELECT
  SUM(CASE
    WHEN TRIM(COALESCE(v.legal_basis,'')) <> ''
     AND NOT (
       v.legal_bases_json IS NOT NULL
       AND JSON_VALID(v.legal_bases_json)
       AND JSON_TYPE(v.legal_bases_json)='ARRAY'
       AND EXISTS (
         SELECT 1 FROM JSON_TABLE(v.legal_bases_json, '$[*]' COLUMNS (
           title VARCHAR(1024) PATH '$.title' NULL ON ERROR,
           summary TEXT PATH '$.summary' NULL ON ERROR
         )) jt
         WHERE TRIM(COALESCE(jt.title,'')) <> '' OR TRIM(COALESCE(jt.summary,'')) <> ''
       )
     ) THEN 1 ELSE 0 END) AS a_cnt,
  SUM(CASE
    WHEN TRIM(COALESCE(v.legal_basis,'')) = ''
     AND v.legal_bases_json IS NOT NULL
     AND JSON_VALID(v.legal_bases_json)
     AND JSON_TYPE(v.legal_bases_json)='ARRAY'
     AND EXISTS (
       SELECT 1 FROM JSON_TABLE(v.legal_bases_json, '$[*]' COLUMNS (
         title VARCHAR(1024) PATH '$.title' NULL ON ERROR,
         summary TEXT PATH '$.summary' NULL ON ERROR
       )) jt
       WHERE TRIM(COALESCE(jt.title,'')) <> '' OR TRIM(COALESCE(jt.summary,'')) <> ''
     ) THEN 1 ELSE 0 END) AS b_cnt,
  SUM(CASE
    WHEN TRIM(COALESCE(v.legal_basis,'')) = ''
     AND NOT (
       v.legal_bases_json IS NOT NULL
       AND JSON_VALID(v.legal_bases_json)
       AND JSON_TYPE(v.legal_bases_json)='ARRAY'
       AND EXISTS (
         SELECT 1 FROM JSON_TABLE(v.legal_bases_json, '$[*]' COLUMNS (
           title VARCHAR(1024) PATH '$.title' NULL ON ERROR,
           summary TEXT PATH '$.summary' NULL ON ERROR
         )) jt
         WHERE TRIM(COALESCE(jt.title,'')) <> '' OR TRIM(COALESCE(jt.summary,'')) <> ''
       )
     ) THEN 1 ELSE 0 END) AS e_cnt,
  SUM(CASE
    WHEN TRIM(COALESCE(v.legal_basis,'')) <> ''
     AND v.legal_bases_json IS NOT NULL
     AND JSON_VALID(v.legal_bases_json)
     AND JSON_TYPE(v.legal_bases_json)='ARRAY'
     AND EXISTS (
       SELECT 1 FROM JSON_TABLE(v.legal_bases_json, '$[*]' COLUMNS (
         title VARCHAR(1024) PATH '$.title' NULL ON ERROR,
         summary TEXT PATH '$.summary' NULL ON ERROR
       )) jt
       WHERE TRIM(COALESCE(jt.title,'')) <> '' OR TRIM(COALESCE(jt.summary,'')) <> ''
     ) THEN 1 ELSE 0 END) AS both_cd
FROM disclosure_type_versions v
INNER JOIN disclosure_types t ON t.type_id = v.type_id
`).Scan(&counts.A, &counts.B, &counts.E, &counts.C)
	qlog.add("sql_group_approx", "SELECT", 1, time.Since(start))
	if err != nil {
		return nil, counts, err
	}
	counts.D = 0 // C holds BOTH(C+D); D unused at SQL layer
	return records, counts, nil
}

func runIdempotency(rows []inventory.Record) map[string]any {
	autoEligibleSecondMutations := 0
	stable := 0
	for _, rec := range rows {
		r1 := inventory.ClassifyRecord(rec)
		sim := inventory.SimulateApply(rec, r1)
		r2 := inventory.ClassifyRecord(sim)
		sim2 := inventory.SimulateApply(sim, r2)
		r3 := inventory.ClassifyRecord(sim2)
		if r2.TargetFlatHash == r3.TargetFlatHash && r2.TargetStructHash == r3.TargetStructHash {
			stable++
		}
		switch r2.ProposedAction {
		case inventory.ActionWrapLegacyFlat, inventory.ActionProjectStructured:
			autoEligibleSecondMutations++
		}
	}
	return map[string]any{
		"records":                 len(rows),
		"stableHashCount":         stable,
		"secondPassAutoMutations": autoEligibleSecondMutations,
		"idempotent":              autoEligibleSecondMutations == 0 && stable == len(rows),
		"note":                    "Run2 proposed WRAP/PROJECT must be 0; C/D/E/MANUAL/BLOCKED stable",
	}
}

func buildReports(
	results []inventory.Result,
	recon inventory.Reconciliation,
	sql sqlCountResult,
	safety safetyProof,
	qlog *queryLog,
	start time.Time,
	batch int,
	idempo map[string]any,
) map[string]any {
	recon.AnalyzerMatch = matchSQLApprox(sql, recon)

	malformed := []map[string]any{}
	violations := map[string]int{}
	overflow := []map[string]any{}
	groupD := []map[string]any{}
	preview := []map[string]any{}
	actionCounts := map[string]int{}

	for _, r := range results {
		actionCounts[string(r.ProposedAction)]++
		preview = append(preview, map[string]any{
			"typeId": r.TypeID, "versionNo": r.VersionNo, "group": r.Group,
			"companyMarker": r.CompanyMarker, "typeStatus": r.TypeStatus,
			"structuredCount": r.StructuredCount, "targetStructuredCount": r.TargetStructCount,
			"flatRuneCount": r.FlatRuneCount, "targetFlatRuneCount": r.TargetFlatRuneCount,
			"flatHash": r.FlatHash, "projectionHash": r.ProjectionHash,
			"targetFlatHash": r.TargetFlatHash, "proposedAction": r.ProposedAction,
			"warnings": r.Warnings,
		})
		for _, a := range r.Anomalies {
			if a == inventory.AnomalyMalformedJSON {
				malformed = append(malformed, map[string]any{
					"typeId": r.TypeID, "versionNo": r.VersionNo, "companyMarker": r.CompanyMarker,
					"errorCode": r.ParseError, "jsonByteLength": r.JSONByteLength, "jsonHash": r.JSONHash,
					"group": r.Group,
				})
			}
			if a == inventory.AnomalyProjectionOverflow {
				overflow = append(overflow, map[string]any{
					"typeId": r.TypeID, "versionNo": r.VersionNo, "itemCount": r.StructuredCount,
					"projectionRuneCount": r.ProjectionRuneCount, "projectionHash": r.ProjectionHash, "group": r.Group,
				})
			}
		}
		for _, c := range r.ViolationCodes {
			violations[c]++
		}
		if r.Group == inventory.GroupD {
			groupD = append(groupD, map[string]any{
				"typeId": r.TypeID, "versionNo": r.VersionNo, "typeStatus": r.TypeStatus,
				"companyMarker": r.CompanyMarker, "structuredCount": r.StructuredCount,
				"flatRuneCount": r.FlatRuneCount, "projectionRuneCount": r.ProjectionRuneCount,
				"flatHash": r.FlatHash, "projectionHash": r.ProjectionHash,
				"differenceClassification": r.DivergenceClass,
			})
		}
	}

	total := recon.Total
	pct := func(n int) float64 {
		if total == 0 {
			return 0
		}
		return float64(n) * 100 / float64(total)
	}
	summary := map[string]any{
		"phase":       "Phase 12.6A - Legacy Legal Basis Inventory",
		"environment": "DEV",
		"dataset": map[string]any{
			"table":  "disclosure_type_versions JOIN disclosure_types",
			"filter": "all non-deleted versions (types.status retained; no soft-delete column)",
			"cutoff": start.UTC().Format(time.RFC3339),
			"total":  total,
		},
		"groups": map[string]any{
			"A": map[string]any{"count": recon.Groups[inventory.GroupA], "percent": pct(recon.Groups[inventory.GroupA])},
			"B": map[string]any{"count": recon.Groups[inventory.GroupB], "percent": pct(recon.Groups[inventory.GroupB])},
			"C": map[string]any{"count": recon.Groups[inventory.GroupC], "percent": pct(recon.Groups[inventory.GroupC])},
			"D": map[string]any{"count": recon.Groups[inventory.GroupD], "percent": pct(recon.Groups[inventory.GroupD])},
			"E": map[string]any{"count": recon.Groups[inventory.GroupE], "percent": pct(recon.Groups[inventory.GroupE])},
		},
		"anomalies": map[string]any{
			"malformedJson":      len(malformed),
			"invalidItems":       sumViolations(violations),
			"projectionOverflow": len(overflow),
			"exactDuplicates":    violations["EXACT_DUPLICATE"],
			"duplicateIds":       violations["DUPLICATE_ID"],
		},
		"dryRun": map[string]any{
			"autoEligible": actionCounts[string(inventory.ActionWrapLegacyFlat)] + actionCounts[string(inventory.ActionProjectStructured)] + actionCounts[string(inventory.ActionNormalizeMatched)],
			"manualReview": actionCounts[string(inventory.ActionManualReview)],
			"blocked":      actionCounts[string(inventory.ActionBlockedMalformed)] + actionCounts[string(inventory.ActionBlockedOverflow)],
			"noOp":         actionCounts[string(inventory.ActionNoOp)],
			"idempotent":   idempo["idempotent"],
			"actionCounts": actionCounts,
		},
		"safety": map[string]any{
			"readOnlyConnection": safety.ReadOnlyGranted,
			"selectStatements":   qlog.Selects,
			"writeStatements":    0,
			"databaseMutations":  0,
			"migrationExecuted":  false,
			"deploymentExecuted": false,
			"connectionSource":   safety.ConnectionSource,
			"engineVersion":      safety.EngineVersion,
		},
		"sqlApprox": map[string]any{
			"total": sql.Total, "A": sql.A, "B": sql.B, "E": sql.E, "both_CD": sql.C,
			"analyzerMatchApprox": recon.AnalyzerMatch,
		},
		"verdict": verdictOf(recon, idempo, safety),
	}

	return map[string]any{
		"reconciliation": recon,
		"summary":        summary,
		"malformed":      malformed,
		"violations":     violations,
		"overflow":       overflow,
		"groupD":         groupD,
		"preview":        preview,
		"idempotency":    idempo,
		"performance": map[string]any{
			"batchSize":  batch,
			"durationMs": time.Since(start).Milliseconds(),
			"queryCount": qlog.Selects,
			"rowsPerSec": float64(total) / maxFloat(time.Since(start).Seconds(), 0.001),
		},
		"safety": safety,
		"qlog":   qlog,
	}
}

func verdictOf(recon inventory.Reconciliation, idempo map[string]any, safety safetyProof) string {
	if !safety.ReadOnlyGranted {
		return "BLOCKED_READ_ONLY_ACCESS"
	}
	if !recon.AnalyzerMatch || recon.SumGroups != recon.Total {
		return "FAIL_RECONCILIATION"
	}
	if idempo["idempotent"] == true {
		return "PASS_READ_ONLY_DRY_RUN"
	}
	return "PASS_WITH_NON_BLOCKING_GAPS"
}

func sumViolations(m map[string]int) int {
	n := 0
	for _, v := range m {
		n += v
	}
	return n
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func writeAll(outDir string, reports map[string]any, safety safetyProof, qlog *queryLog) {
	mustWrite(filepath.Join(outDir, "phase-12-6a-group-reconciliation.json"), reports["reconciliation"])
	mustWrite(filepath.Join(outDir, "phase-12-6a-group-summary.json"), reports["summary"])
	mustWrite(filepath.Join(outDir, "phase-12-6a-malformed-json-report.json"), map[string]any{"items": reports["malformed"]})
	mustWrite(filepath.Join(outDir, "phase-12-6a-contract-violation-report.json"), map[string]any{"counts": reports["violations"]})
	mustWrite(filepath.Join(outDir, "phase-12-6a-projection-overflow-report.json"), map[string]any{"items": reports["overflow"]})
	mustWrite(filepath.Join(outDir, "phase-12-6a-group-d-divergence-report.json"), map[string]any{"items": reports["groupD"]})
	mustWrite(filepath.Join(outDir, "phase-12-6a-dry-run-preview.json"), map[string]any{"items": reports["preview"]})
	mustWrite(filepath.Join(outDir, "phase-12-6a-idempotency-results.json"), reports["idempotency"])
	mustWrite(filepath.Join(outDir, "phase-12-6a-performance-results.json"), reports["performance"])

	safetyMD := fmt.Sprintf(`# Phase 12.6A — Read-only safety

- Connection source: %s (credentials redacted)
- Engine version: %s
- @@transaction_read_only: %s
- @@session.transaction_read_only: %s
- Write privilege detected: %v
- Read-only granted: %v

## Grants (redacted)

`+"```\n%s```\n",
		safety.ConnectionSource, safety.EngineVersion, safety.TransactionReadOnly, safety.SessionReadOnly,
		safety.WritePrivilegeDetected, safety.ReadOnlyGranted, safety.GrantsRedacted)
	_ = os.WriteFile(filepath.Join(outDir, "phase-12-6a-read-only-safety.md"), []byte(safetyMD), 0o644)

	var b strings.Builder
	b.WriteString("# Phase 12.6A — Query log\n\n")
	for _, e := range qlog.Entries {
		b.WriteString(fmt.Sprintf("- seq=%v ts=%v purpose=%v type=%v rows=%v durMs=%v write=false\n",
			e["sequence"], e["timestamp"], e["purpose"], e["queryType"], e["rows"], e["durationMs"]))
	}
	b.WriteString(fmt.Sprintf("\nSELECT count = %d\nINSERT count = 0\nUPDATE count = 0\nDELETE count = 0\nDDL count = 0\nLOCK count = 0\n", qlog.Selects))
	_ = os.WriteFile(filepath.Join(outDir, "phase-12-6a-query-log.md"), []byte(b.String()), 0o644)
}

func mustWrite(path string, v any) {
	if err := writeJSON(path, mustJSON(v)); err != nil {
		fatalf("write %s: %v", path, err)
	}
}

func mustJSON(v any) []byte {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		panic(err)
	}
	return append(b, '\n')
}

func writeJSON(path string, b []byte) error {
	return os.WriteFile(path, b, 0o644)
}

func fatalf(f string, args ...any) {
	fmt.Fprintf(os.Stderr, f+"\n", args...)
	os.Exit(2)
}
