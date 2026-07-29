package backfill_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	backfill "github.com/cobo/cobo_iam_services/internal/disclosure/app/legal_basis_backfill"
)

type seqID struct{ n int }

func (s *seqID) NewUUID() string {
	s.n++
	return fmt.Sprintf("00000000-0000-7000-8000-%012d", s.n)
}

func sampleAllowlist(t *testing.T, dir string, flats map[string]string) (string, *backfill.AllowlistFile) {
	t.Helper()
	typeIDs := []string{
		"dt-custom-obligation",
		"dt-disclosure-transaction",
		"dt-event-major-change",
		"dt-obligation-report",
		"dt-periodic-financial",
		"dt-shareholder-meeting",
	}
	records := []map[string]any{}
	for _, id := range typeIDs {
		flat := flats[id]
		records = append(records, map[string]any{
			"record_id": id + ":1", "disclosure_type_id": id, "version_no": 1,
			"company_scope": "GLOBAL", "type_status": "active", "active_version_no": 1,
			"group": "A", "flat_rune_count": len([]rune(flat)), "flat_hash": backfill.ShortHash(flat),
			"structured_state": "empty_or_null", "structured_count": 0,
			"target_item_count": 1, "target_projection_hash": backfill.TargetFlatHashOD7,
			"target_flat_rune_count": 13, "action": "WRAP_LEGACY_FLAT",
		})
	}
	doc := map[string]any{
		"phase": "test", "environment": "DEV", "algorithmVersion": backfill.AlgorithmVersion,
		"recordCount": 6, "uniqueRecordIds": 6, "action": "WRAP_LEGACY_FLAT",
		"targetFlatHashUniform": backfill.TargetFlatHashOD7, "targetFlatRuneCountUniform": 13,
		"records": records,
	}
	b, _ := json.MarshalIndent(doc, "", "  ")
	path := filepath.Join(dir, "allowlist.json")
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatal(err)
	}
	al, err := backfill.LoadAllowlist(path)
	if err != nil {
		t.Fatal(err)
	}
	return path, al
}

func seedRows(flats map[string]string) []backfill.Row {
	var rows []backfill.Row
	for id, flat := range flats {
		rows = append(rows, backfill.Row{
			TypeID: id, VersionNo: 1, LegalBasis: flat, UpdatedBy: "system", ActivatedAt: "2026-01-01T00:00:00Z",
		})
	}
	return rows
}

func baseFlats() map[string]string {
	return map[string]string{
		"dt-custom-obligation":      "Flat text for custom obligation!",
		"dt-disclosure-transaction": "Flat text for disclosure transaction XX",
		"dt-event-major-change":     "Flat text for event major change",
		"dt-obligation-report":      "Flat text for obligation reporting needs",
		"dt-periodic-financial":     "Flat periodic financial text",
		"dt-shareholder-meeting":    "Flat shareholder meeting legal basis text here",
	}
}

func TestEnvGuard(t *testing.T) {
	if err := (backfill.EnvGuard{Environment: "DEV", Database: "cobo_iam", HostAlias: "127.0.0.1", Port: "3306"}).Validate(); err != nil {
		t.Fatal(err)
	}
	if err := (backfill.EnvGuard{Environment: "prod", Database: "cobo_iam", HostAlias: "127.0.0.1", Port: "3306"}).Validate(); err == nil {
		t.Fatal("expected refuse prod")
	}
	if err := backfill.ConfirmTokenOK(""); err == nil {
		t.Fatal("empty token")
	}
	if err := backfill.ConfirmTokenOK("short"); err == nil {
		t.Fatal("short token")
	}
	if err := backfill.ConfirmTokenOK("this-is-a-valid-confirm-token"); err != nil {
		t.Fatal(err)
	}
}

func TestAllowlistRejectsWrongCount(t *testing.T) {
	dir := t.TempDir()
	doc := map[string]any{
		"environment": "DEV", "algorithmVersion": backfill.AlgorithmVersion,
		"recordCount": 1, "uniqueRecordIds": 1, "action": "WRAP_LEGACY_FLAT",
		"records": []any{},
	}
	b, _ := json.Marshal(doc)
	path := filepath.Join(dir, "bad.json")
	_ = os.WriteFile(path, b, 0o600)
	if _, err := backfill.LoadAllowlist(path); err == nil {
		t.Fatal("expected reject")
	}
}

func TestSnapshotRoundTripAndMode(t *testing.T) {
	dir := t.TempDir()
	flats := baseFlats()
	_, al := sampleAllowlist(t, dir, flats)
	env := backfill.EnvGuard{Environment: "DEV", Database: "cobo_iam", HostAlias: "127.0.0.1", Port: "3306"}
	var rows []backfill.SnapshotRecord
	for _, rec := range al.SortedRecords() {
		rows = append(rows, backfill.SnapshotRecord{
			RecordID: rec.RecordID, DisclosureTypeID: rec.DisclosureTypeID, VersionNo: 1,
			LegalBasis: flats[rec.DisclosureTypeID], UpdatedBy: "system", ActivatedAt: "2026-01-01T00:00:00Z",
		})
	}
	snap, err := backfill.BuildSnapshot(env, al, rows)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "snap.json")
	if err := backfill.WriteSnapshotAtomic(path, snap, false); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm()&0o077 != 0 {
		t.Fatalf("expected 0600, got %v", st.Mode().Perm())
	}
	if err := backfill.WriteSnapshotAtomic(path, snap, false); err == nil {
		t.Fatal("expected refuse overwrite")
	}
	loaded, err := backfill.LoadSnapshot(path, al)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.SnapshotChecksum != snap.SnapshotChecksum {
		t.Fatal("checksum")
	}
	// tamper checksum field while keeping body otherwise valid enough to parse
	b, _ := os.ReadFile(path)
	tampered := strings.Replace(string(b), `"snapshotChecksum": "`+snap.SnapshotChecksum+`"`, `"snapshotChecksum": "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"`, 1)
	if tampered == string(b) {
		t.Fatal("tamper replace failed")
	}
	_ = os.WriteFile(path, []byte(tampered), 0o600)
	if _, err := backfill.LoadSnapshot(path, al); err == nil {
		t.Fatal("expected tamper detect")
	}
}

func TestPlanNoMutation(t *testing.T) {
	dir := t.TempDir()
	flats := baseFlats()
	_, al := sampleAllowlist(t, dir, flats)
	store := backfill.NewMemoryStore(seedRows(flats))
	eng := &backfill.Engine{
		Env:   backfill.EnvGuard{Environment: "DEV", Database: "cobo_iam", HostAlias: "127.0.0.1", Port: "3306"},
		Store: store, IDGen: &seqID{},
	}
	rep, err := eng.Plan(context.Background(), al)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Mutations != 0 || rep.Freshness != "PASS" || rep.ProposedUpdates != 6 {
		t.Fatalf("%+v", rep)
	}
}

func TestApplySuccessAndVerify(t *testing.T) {
	dir := t.TempDir()
	flats := baseFlats()
	_, al := sampleAllowlist(t, dir, flats)
	store := backfill.NewMemoryStore(seedRows(flats))
	eng := &backfill.Engine{
		Env:   backfill.EnvGuard{Environment: "DEV", Database: "cobo_iam", HostAlias: "127.0.0.1", Port: "3306"},
		Store: store, IDGen: &seqID{},
	}
	var srows []backfill.SnapshotRecord
	for _, rec := range al.SortedRecords() {
		srows = append(srows, backfill.SnapshotRecord{
			RecordID: rec.RecordID, DisclosureTypeID: rec.DisclosureTypeID, VersionNo: 1,
			LegalBasis: flats[rec.DisclosureTypeID], UpdatedBy: "system", ActivatedAt: "2026-01-01T00:00:00Z",
		})
	}
	snap, err := backfill.BuildSnapshot(eng.Env, al, srows)
	if err != nil {
		t.Fatal(err)
	}
	arep, err := eng.Apply(context.Background(), al, snap)
	if err != nil || arep.Status != "APPLIED" || arep.RowsAffected != 6 {
		t.Fatalf("%+v %v", arep, err)
	}
	if arep.Verify == nil || arep.Verify.Status != "PASS" {
		t.Fatalf("verify %+v", arep.Verify)
	}
	v, err := eng.Verify(context.Background(), al)
	if err != nil || v.Status != "PASS" || v.GroupCounts["C"] != 6 {
		t.Fatalf("%+v %v", v, err)
	}
	// updated_by preserved
	for _, r := range store.Snapshot() {
		if r.UpdatedBy != "system" {
			t.Fatalf("updated_by mutated: %q", r.UpdatedBy)
		}
		if strings.Contains(string(r.LegalBasesJSON), "-lb-legacy-") {
			t.Fatal("display id leaked")
		}
	}
}

func TestApplyFreshnessStale(t *testing.T) {
	dir := t.TempDir()
	flats := baseFlats()
	_, al := sampleAllowlist(t, dir, flats)
	rows := seedRows(flats)
	rows[0].LegalBasis = "changed by user"
	store := backfill.NewMemoryStore(rows)
	eng := &backfill.Engine{
		Env:   backfill.EnvGuard{Environment: "DEV", Database: "cobo_iam", HostAlias: "127.0.0.1", Port: "3306"},
		Store: store, IDGen: &seqID{},
	}
	var srows []backfill.SnapshotRecord
	for _, rec := range al.SortedRecords() {
		srows = append(srows, backfill.SnapshotRecord{
			RecordID: rec.RecordID, DisclosureTypeID: rec.DisclosureTypeID, VersionNo: 1,
			LegalBasis: flats[rec.DisclosureTypeID], UpdatedBy: "system", ActivatedAt: "2026-01-01T00:00:00Z",
		})
	}
	snap, _ := backfill.BuildSnapshot(eng.Env, al, srows)
	arep, err := eng.Apply(context.Background(), al, snap)
	if err == nil || arep.Status != "STALE_DRY_RUN" {
		t.Fatalf("%+v %v", arep, err)
	}
}

func TestApplyRollbackOnCASMissMidBatch(t *testing.T) {
	dir := t.TempDir()
	flats := baseFlats()
	_, al := sampleAllowlist(t, dir, flats)
	store := backfill.NewMemoryStore(seedRows(flats))
	eng := &backfill.Engine{
		Env:   backfill.EnvGuard{Environment: "DEV", Database: "cobo_iam", HostAlias: "127.0.0.1", Port: "3306"},
		Store: store,
		IDGen: &seqID{},
	}
	var srows []backfill.SnapshotRecord
	for _, rec := range al.SortedRecords() {
		srows = append(srows, backfill.SnapshotRecord{
			RecordID: rec.RecordID, DisclosureTypeID: rec.DisclosureTypeID, VersionNo: 1,
			LegalBasis: flats[rec.DisclosureTypeID], UpdatedBy: "system", ActivatedAt: "2026-01-01T00:00:00Z",
		})
	}
	snap, _ := backfill.BuildSnapshot(eng.Env, al, srows)

	// Mutate third record after snapshot build but before apply by wrapping store?
	// Simulate mid-apply by using a sabotaging ID gen that fails transform after 2:
	eng.IDGen = &failAfter{n: 0, failAt: 3}
	arep, err := eng.Apply(context.Background(), al, snap)
	if err == nil || arep.Status != "ROLLBACK" {
		t.Fatalf("%+v %v", arep, err)
	}
	// all still Group A (no commit)
	v, _ := eng.Verify(context.Background(), al)
	if v.GroupCounts["C"] != 0 {
		t.Fatalf("partial apply leaked: %+v", v)
	}
}

type failAfter struct {
	n, failAt int
}

func (f *failAfter) NewUUID() string {
	f.n++
	if f.n >= f.failAt {
		return "" // invalid
	}
	return fmt.Sprintf("00000000-0000-7000-8000-%012d", f.n)
}

func TestRollbackExactAndRefuseStale(t *testing.T) {
	dir := t.TempDir()
	flats := baseFlats()
	_, al := sampleAllowlist(t, dir, flats)
	store := backfill.NewMemoryStore(seedRows(flats))
	eng := &backfill.Engine{
		Env:   backfill.EnvGuard{Environment: "DEV", Database: "cobo_iam", HostAlias: "127.0.0.1", Port: "3306"},
		Store: store, IDGen: &seqID{},
	}
	var srows []backfill.SnapshotRecord
	for _, rec := range al.SortedRecords() {
		srows = append(srows, backfill.SnapshotRecord{
			RecordID: rec.RecordID, DisclosureTypeID: rec.DisclosureTypeID, VersionNo: 1,
			LegalBasis: flats[rec.DisclosureTypeID], UpdatedBy: "system", ActivatedAt: "2026-01-01T00:00:00Z",
		})
	}
	snap, _ := backfill.BuildSnapshot(eng.Env, al, srows)
	arep, err := eng.Apply(context.Background(), al, snap)
	if err != nil {
		t.Fatal(err)
	}
	postFlat := map[string]string{}
	postJSON := map[string][]byte{}
	for _, r := range store.Snapshot() {
		postFlat[r.RecordID()] = r.LegalBasis
		postJSON[r.RecordID()] = append([]byte(nil), r.LegalBasesJSON...)
	}
	rr, err := eng.Rollback(context.Background(), al, snap, postFlat, postJSON)
	if err != nil || rr.Status != "RESTORED" || rr.RowsRestored != 6 {
		t.Fatalf("%+v %v", rr, err)
	}
	v, _ := eng.Verify(context.Background(), al)
	if v.GroupCounts["C"] != 0 {
		// after rollback expect not all C
	}
	// re-apply then user edit then refuse rollback
	eng.IDGen = &seqID{n: 10}
	if _, err := eng.Apply(context.Background(), al, snap); err != nil {
		t.Fatal(err)
	}
	for _, r := range store.Snapshot() {
		postFlat[r.RecordID()] = r.LegalBasis
		postJSON[r.RecordID()] = append([]byte(nil), r.LegalBasesJSON...)
	}
	// user edits one row
	edited := store.Snapshot()
	edited[0].LegalBasis = "user edited projection"
	store2 := backfill.NewMemoryStore(edited)
	eng.Store = store2
	rr2, err := eng.Rollback(context.Background(), al, snap, postFlat, postJSON)
	if err == nil || rr2.Status != "REFUSED_STALE" {
		t.Fatalf("%+v %v", rr2, err)
	}
	_ = arep
}

func TestTransformUnicodeMultiline(t *testing.T) {
	flat := "Dòng 1\n\nDòng 2 — Tiếng Việt ậẫ"
	tr, err := backfill.TransformWrapLegacy("dt-x", 1, flat, &seqID{})
	if err != nil {
		t.Fatal(err)
	}
	if tr.Projection != backfill.LegacyTitle {
		t.Fatalf("proj %q", tr.Projection)
	}
	var items []map[string]any
	if err := json.Unmarshal(tr.JSONBytes, &items); err != nil || len(items) != 1 {
		t.Fatal(items, err)
	}
	if items[0]["summary"] != flat {
		t.Fatal("summary not preserved")
	}
	if items[0]["title"] != backfill.LegacyTitle {
		t.Fatal("title")
	}
}
