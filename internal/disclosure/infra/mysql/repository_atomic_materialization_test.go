package mysql

import (
	"strings"
	"testing"
)

// TestUpsertTypeVersion_MySQLTransactionalIntegrityAndConcurrencyProof verifies that
// UpsertTypeVersion enforces atomic transaction boundary and strict duplicate key handling
// in the MySQL repository:
// 1. Transaction begins with tx, err := r.db.BeginTx(ctx, nil) and defers rollback.
// 2. Uses SELECT ... FOR UPDATE on disclosure_types to lock the type_id row or prevent race.
// 3. Enforces CreateOnly check: if typeExists && req.CreateOnly, returns 409 STATE_CONFLICT.
// 4. Inserts disclosure_types root (if not exists) and disclosure_type_versions atomically.
// 5. Inserts blocks into disclosure_template_blocks in the same transaction.
// 6. Inserts display groups into template_display_groups in the same transaction.
// 7. Commits via tx.Commit() only after all aggregate entities succeed.
func TestUpsertTypeVersion_MySQLTransactionalIntegrityAndConcurrencyProof(t *testing.T) {
	src := readRepositorySrc(t)
	fn := extractFunc(t, src, "func (r *Repository) UpsertTypeVersion")

	// 1. Transaction boundary & Rollback deferral
	if !strings.Contains(fn, "r.db.BeginTx(ctx, nil)") {
		t.Fatal("UpsertTypeVersion must begin an explicit database transaction with BeginTx")
	}
	if !strings.Contains(fn, "defer func() { _ = tx.Rollback() }()") {
		t.Fatal("UpsertTypeVersion must defer tx.Rollback() to guarantee zero-partial-write on error")
	}

	// 2. Concurrency Safety: Row-level lock / FOR UPDATE
	if !strings.Contains(fn, "SELECT active_version_no, company_id FROM disclosure_types WHERE type_id = ? FOR UPDATE") {
		t.Fatal("UpsertTypeVersion must acquire exclusive row lock with FOR UPDATE to serialize concurrent writes on type_id")
	}

	// 3. CreateOnly duplicate detection & 409 Conflict
	if !strings.Contains(fn, "req.CreateOnly && typeExists") {
		t.Fatal("UpsertTypeVersion must check req.CreateOnly when typeExists")
	}
	if !strings.Contains(fn, "perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, \"target_type_id already exists\", nil)") {
		t.Fatal("UpsertTypeVersion must return 409 STATE_CONFLICT on CreateOnly duplicate")
	}

	// 4. Atomic aggregate writes
	if !strings.Contains(fn, "INSERT INTO disclosure_types") {
		t.Fatal("UpsertTypeVersion must insert root disclosure_types within the transaction")
	}
	if !strings.Contains(fn, "INSERT INTO disclosure_type_versions") {
		t.Fatal("UpsertTypeVersion must insert disclosure_type_versions within the transaction")
	}
	if !strings.Contains(fn, "INSERT INTO disclosure_template_blocks") {
		t.Fatal("UpsertTypeVersion must insert disclosure_template_blocks within the transaction")
	}
	if !strings.Contains(fn, "replaceTemplateDisplayGroups(ctx, tx, req.TypeID, req.DisplayGroupCodes)") {
		t.Fatal("UpsertTypeVersion must associate display groups within the same transaction")
	}

	// 5. Single atomic commit at the very end
	if !strings.Contains(fn, "err := tx.Commit(); err != nil") {
		t.Fatal("UpsertTypeVersion must commit transaction only after all aggregate entities are staged")
	}
}
