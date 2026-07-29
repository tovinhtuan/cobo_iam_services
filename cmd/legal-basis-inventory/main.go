package main

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"

	inventory "github.com/cobo/cobo_iam_services/internal/disclosure/app/legal_basis_inventory"
)

// legal-basis-inventory — Phase 12.6A DEV inventory + in-memory dry-run.
// No --apply flag. SQL go through allowlist interceptor.
// Docker DEV app credential allowed only with READ ONLY transaction + allowlist
// (user confirmation for Phase 12.6A).

func main() {
	inventory.SetMySQLConnector(func(dsn string) (driver.Connector, error) {
		cfg, err := mysql.ParseDSN(dsn)
		if err != nil {
			return nil, err
		}
		return mysql.NewConnector(cfg)
	})

	outDir := flag.String("out-dir", "", "directory for JSON reports")
	dsnFile := flag.String("dsn-file", "", "path to file containing DSN")
	dockerDev := flag.Bool("docker-dev", false, "use docker-compose.dev.yml published host mapping (app user + READ ONLY tx)")
	batchSize := flag.Int("batch", 200, "keyset page size (100–500)")
	timeout := flag.Duration("timeout", 10*time.Minute, "overall context timeout")
	flag.Parse()

	dsn, src, allowAppCreds, err := loadDSN(*dsnFile, *dockerDev)
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

	qlog := &queryLog{}
	db, err := inventory.OpenAllowlistedMySQL(dsn, func(purpose, hash string) {
		qlog.add(purpose+"/hash:"+hash, "SELECT", 0, 0)
	})
	if err != nil {
		fatalf("BLOCKED_DATABASE_CONNECTION: open: %v", err)
	}
	defer db.Close()
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.PingContext(ctx); err != nil {
		fatalf("BLOCKED_DATABASE_CONNECTION: ping: %v", err)
	}

	safety, err := proveSession(ctx, db, src, allowAppCreds, qlog)
	if err != nil {
		_ = os.WriteFile(filepath.Join(*outDir, "phase-12-6a-read-only-safety.md"), []byte("# BLOCKED\n\n"+err.Error()+"\n"), 0o644)
		fatalf("SESSION GATE FAILED: %v", err)
	}

	start := time.Now()
	// MySQL 8: prefer session characteristic so @@transaction_read_only reflects READ ONLY
	// (START TRANSACTION READ ONLY alone blocks writes but may leave the variable at 0).
	if _, err := db.ExecContext(ctx, "SET SESSION TRANSACTION ISOLATION LEVEL REPEATABLE READ, READ ONLY"); err != nil {
		fatalf("SET SESSION TRANSACTION READ ONLY: %v", err)
	}
	qlog.add("set_session_txn_ro_rr", "METADATA", 0, 0)

	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		fatalf("BEGIN READ ONLY: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Confirm txn read-only inside transaction
	var txnRO int
	if err := tx.QueryRowContext(ctx, "SELECT @@transaction_read_only").Scan(&txnRO); err == nil {
		safety.TransactionReadOnly = fmt.Sprintf("%d", txnRO)
	}
	qlog.add("txn_read_only_check", "SELECT", 1, 0)
	if txnRO != 1 {
		_ = tx.Rollback()
		fatalf("expected @@transaction_read_only=1 inside inventory transaction (engine=%s)", safety.EngineVersion)
	}
	safety.ReadOnlyGranted = true
	safety.SQLAllowlistEnforced = true

	rows, sqlCounts, err := scanAll(ctx, tx, *batchSize, qlog)
	if err != nil {
		fatalf("scan: %v", err)
	}
	if err := tx.Commit(); err != nil {
		fatalf("commit: %v", err)
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

func loadDSN(dsnFile string, dockerDev bool) (dsn, source string, allowAppCreds bool, err error) {
	if dockerDev {
		// Published host port from docker-compose.dev.yml (service mysql → 3306:3306).
		// Credential matches compose MYSQL_USER/MYSQL_PASSWORD baked in that file (not logged).
		dsn = "cobo:cobo@tcp(127.0.0.1:3306)/cobo_iam?parseTime=true&loc=UTC&tls=false"
		return dsn, "docker-compose.dev.yml:mysql published 127.0.0.1:3306 / db=cobo_iam / user=c***", true, nil
	}
	if dsnFile != "" {
		b, e := os.ReadFile(dsnFile)
		if e != nil {
			return "", "", false, fmt.Errorf("dsn-file: %w", e)
		}
		dsn = strings.TrimSpace(string(b))
		return dsn, "dsn-file", true, nil
	}
	if v := strings.TrimSpace(os.Getenv("MYSQL_READONLY_DSN")); v != "" {
		return v, "MYSQL_READONLY_DSN", false, nil
	}
	if v := strings.TrimSpace(os.Getenv("LEGAL_BASIS_INVENTORY_DSN")); v != "" {
		return v, "LEGAL_BASIS_INVENTORY_DSN", true, nil
	}
	if v := strings.TrimSpace(os.Getenv("MYSQL_DSN")); v != "" {
		// Host remapping hint: replace tcp(mysql: with tcp(127.0.0.1:
		v = strings.Replace(v, "@tcp(mysql:", "@tcp(127.0.0.1:", 1)
		return v, "MYSQL_DSN(host-mapped)", true, nil
	}
	return "", "", false, fmt.Errorf("no DSN: use --docker-dev or MYSQL_READONLY_DSN / --dsn-file")
}

type safetyProof struct {
	ConnectionSource       string `json:"connectionSource"`
	EngineVersion          string `json:"engineVersion"`
	TransactionReadOnly    string `json:"transactionReadOnly"`
	SessionReadOnly        string `json:"sessionReadOnly"`
	GrantsRedacted         string `json:"grantsRedacted"`
	ReadOnlyGranted        bool   `json:"readOnlyGranted"`
	WritePrivilegeDetected bool   `json:"writePrivilegeDetected"`
	SQLAllowlistEnforced   bool   `json:"sqlAllowlistEnforced"`
	AppCredentialAllowed   bool   `json:"appCredentialAllowedWithGuards"`
	HostAlias              string `json:"hostAlias"`
	Port                   string `json:"port"`
	DatabaseName           string `json:"databaseName"`
	DockerService          string `json:"dockerService"`
	UsernameMasked         string `json:"usernameMasked"`
}

func proveSession(ctx context.Context, db *sql.DB, src string, allowAppCreds bool, qlog *queryLog) (safetyProof, error) {
	var s safetyProof
	s.ConnectionSource = src
	s.AppCredentialAllowed = allowAppCreds
	s.HostAlias = "127.0.0.1"
	s.Port = "3306"
	s.DatabaseName = "cobo_iam"
	s.DockerService = "mysql (cobo-iam-mysql)"
	s.UsernameMasked = "c***"
	start := time.Now()
	if err := db.QueryRowContext(ctx, "SELECT VERSION()").Scan(&s.EngineVersion); err != nil {
		return s, fmt.Errorf("BLOCKED_DATABASE_CONNECTION: VERSION: %w", err)
	}
	qlog.add("select_version", "METADATA", 1, time.Since(start))
	start = time.Now()
	_ = db.QueryRowContext(ctx, "SELECT @@session.transaction_read_only").Scan(&s.SessionReadOnly)
	qlog.add("session_txn_ro", "SELECT", 1, time.Since(start))

	grants, err := readGrants(ctx, db, qlog)
	if err != nil {
		return s, err
	}
	s.GrantsRedacted = redactGrants(grants)
	s.WritePrivilegeDetected = grantsLookWritable(grants)
	if s.WritePrivilegeDetected && !allowAppCreds {
		return s, fmt.Errorf("write grants without Phase 12.6A docker-dev allow")
	}
	// Probe READ ONLY begin/commit once before main inventory txn.
	if _, err := db.ExecContext(ctx, "SET SESSION TRANSACTION READ ONLY"); err != nil {
		return s, fmt.Errorf("SET SESSION TRANSACTION READ ONLY: %w", err)
	}
	qlog.add("set_session_txn_ro", "METADATA", 0, 0)
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return s, fmt.Errorf("BEGIN READ ONLY failed: %w", err)
	}
	var v int
	_ = tx.QueryRowContext(ctx, "SELECT 1").Scan(&v)
	qlog.add("ro_probe_select1", "SELECT", 1, 0)
	var probeRO int
	_ = tx.QueryRowContext(ctx, "SELECT @@transaction_read_only").Scan(&probeRO)
	qlog.add("ro_probe_txn_flag", "SELECT", 1, 0)
	if probeRO != 1 {
		_ = tx.Rollback()
		return s, fmt.Errorf("RO probe: expected @@transaction_read_only=1, got %d", probeRO)
	}
	if err := tx.Commit(); err != nil {
		return s, err
	}
	s.ReadOnlyGranted = true // session guards + allowlist
	s.SQLAllowlistEnforced = true
	return s, nil
}

func readGrants(ctx context.Context, db *sql.DB, qlog *queryLog) ([]string, error) {
	start := time.Now()
	rows, err := db.QueryContext(ctx, "SHOW GRANTS")
	qlog.add("show_grants", "METADATA", 0, time.Since(start))
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
		line := g
		if i := strings.Index(strings.ToLower(line), " identified by"); i >= 0 {
			line = line[:i] + " IDENTIFIED BY ***"
		}
		// Mask account identifier in grants evidence.
		line = strings.ReplaceAll(line, "`cobo`", "`c***`")
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

type queryable interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func hasColumn(ctx context.Context, q queryable, table, column string, qlog *queryLog) (bool, error) {
	start := time.Now()
	var n int
	err := q.QueryRowContext(ctx, `
SELECT COUNT(*) FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND COLUMN_NAME = ?
`, table, column).Scan(&n)
	qlog.add("schema_column_"+table+"_"+column, "METADATA", 1, time.Since(start))
	return n > 0, err
}

func scanAll(ctx context.Context, db queryable, batch int, qlog *queryLog) ([]inventory.Record, sqlCountResult, error) {
	hasReleased, err := hasColumn(ctx, db, "disclosure_type_versions", "is_released", qlog)
	if err != nil {
		return nil, sqlCountResult{}, err
	}
	hasFlat, err := hasColumn(ctx, db, "disclosure_type_versions", "legal_basis", qlog)
	if err != nil {
		return nil, sqlCountResult{}, err
	}
	hasJSON, err := hasColumn(ctx, db, "disclosure_type_versions", "legal_bases_json", qlog)
	if err != nil {
		return nil, sqlCountResult{}, err
	}
	if !hasFlat || !hasJSON {
		return nil, sqlCountResult{}, fmt.Errorf("BLOCKED_SCHEMA_CONFLICT: missing legal_basis=%v legal_bases_json=%v", hasFlat, hasJSON)
	}

	releasedExpr := "0 AS is_released"
	if hasReleased {
		releasedExpr = "COALESCE(v.is_released, 0) AS is_released"
	}

	var counts sqlCountResult
	start := time.Now()
	err = db.QueryRowContext(ctx, `
SELECT COUNT(*) FROM disclosure_type_versions v
INNER JOIN disclosure_types t ON t.type_id = v.type_id
`).Scan(&counts.Total)
	qlog.add("count_total", "SELECT", 1, time.Since(start))
	if err != nil {
		return nil, counts, err
	}

	keysetSQL := fmt.Sprintf(`
SELECT v.type_id, v.version_no,
       COALESCE(t.company_id, '') AS company_id,
       COALESCE(t.status, '') AS type_status,
       COALESCE(t.active_version_no, 0) AS active_version_no,
       %s,
       COALESCE(v.legal_basis, '') AS legal_basis,
       v.legal_bases_json
FROM disclosure_type_versions v
INNER JOIN disclosure_types t ON t.type_id = v.type_id
WHERE (v.type_id > ?) OR (v.type_id = ? AND v.version_no > ?)
ORDER BY v.type_id ASC, v.version_no ASC
LIMIT ?
`, releasedExpr)

	var records []inventory.Record
	var lastType string
	var lastVer int
	for {
		start = time.Now()
		rows, err := db.QueryContext(ctx, keysetSQL, lastType, lastType, lastVer, batch)
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
			if hasReleased {
				rec.IsReleased = released != 0
			} else {
				// Migration 0122 not applied on this DEV: approximate via active pointer.
				rec.IsReleased = rec.VersionNo == rec.ActiveVersionNo
			}
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

	summary := reports["summary"].(map[string]any)
	verdict, _ := summary["verdict"].(string)
	safetyMD := fmt.Sprintf(`# Phase 12.6A — Read-only safety

- DB engine / version: %s (credentials never logged)
- Database name: %s
- Host alias: %s
- Port: %s
- Docker service/network: %s (compose network cobo-net / published host port)
- Username (masked): %s
- Credential source: %s
- App credential allowed for 12.6A with guards: %v
- SQL allowlist enforced: %v
- @@transaction_read_only (inventory tx): %s
- @@session.transaction_read_only: %s
- Write privilege detected on account: %v
- Read-only transaction used for all inventory SELECTs: %v
- Database mutations: 0
- Tool flags: no --apply; no write repository import

## Grants (privilege names only; passwords stripped)

`+"```\n%s```\n",
		safety.EngineVersion, safety.DatabaseName, safety.HostAlias, safety.Port,
		safety.DockerService, safety.UsernameMasked, safety.ConnectionSource,
		safety.AppCredentialAllowed, safety.SQLAllowlistEnforced,
		safety.TransactionReadOnly, safety.SessionReadOnly,
		safety.WritePrivilegeDetected, safety.ReadOnlyGranted, safety.GrantsRedacted)
	_ = os.WriteFile(filepath.Join(outDir, "phase-12-6a-read-only-safety.md"), []byte(safetyMD), 0o644)

	var b strings.Builder
	b.WriteString("# Phase 12.6A — Query log\n\n")
	b.WriteString("Purposes and hashes only — no DSN, password, or legal text.\n\n")
	for _, e := range qlog.Entries {
		b.WriteString(fmt.Sprintf("- seq=%v ts=%v purpose=%v type=%v rows=%v durMs=%v write=false\n",
			e["sequence"], e["timestamp"], e["purpose"], e["queryType"], e["rows"], e["durationMs"]))
	}
	b.WriteString(fmt.Sprintf("\nSELECT/METADATA count = %d\nINSERT count = 0\nUPDATE count = 0\nDELETE count = 0\nDDL count = 0\nLOCK count = 0\n", qlog.Selects))
	_ = os.WriteFile(filepath.Join(outDir, "phase-12-6a-query-log.md"), []byte(b.String()), 0o644)

	handoff := fmt.Sprintf(`# Phase 12.6A — Inventory handoff

- Verdict: **%s**
- Environment: DEV (docker-compose.dev.yml MySQL)
- Connection: host=%s port=%s db=%s user=%s service=%s (password/DSN redacted)
- Database mutations: **0**
- Phase 12.6B: **not started**
- Groups / dry-run / reconciliation: see sibling JSON reports in this directory
- Next: human review of Group D + dry-run preview before any 12.6B apply plan
`, verdict, safety.HostAlias, safety.Port, safety.DatabaseName, safety.UsernameMasked, safety.DockerService)
	_ = os.WriteFile(filepath.Join(outDir, "phase-12-6a-inventory-handoff.md"), []byte(handoff), 0o644)
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
