package backfill

import (
	"context"
	"fmt"
	"strings"

	inventory "github.com/cobo/cobo_iam_services/internal/disclosure/app/legal_basis_inventory"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

// PlanReport is a redacted non-mutating plan result.
type PlanReport struct {
	Environment       string   `json:"environment"`
	Database          string   `json:"database"`
	AllowlistCount    int      `json:"allowlistCount"`
	AllowlistChecksum string   `json:"allowlistChecksum"`
	Freshness         string   `json:"freshness"` // PASS | STALE_DRY_RUN
	ProposedUpdates   int      `json:"proposedUpdates"`
	RecordIDs         []string `json:"recordIds"`
	BlockedReasons    []string `json:"blockedReasons,omitempty"`
	Mutations         int      `json:"mutations"`
}

// ApplyReport is a redacted apply outcome (no legal text / UUID plaintext).
type ApplyReport struct {
	Status            string        `json:"status"`
	Environment       string        `json:"environment"`
	AllowlistChecksum string        `json:"allowlistChecksum"`
	RowsAffected      int           `json:"rowsAffected"`
	ItemIDHashes      []string      `json:"itemIdHashes"`
	ProjectionHashes  []string      `json:"projectionHashes"`
	Verify            *VerifyReport `json:"verify,omitempty"`
	ErrorCode         string        `json:"errorCode,omitempty"`
}

type Engine struct {
	Env   EnvGuard
	Store Store
	IDGen idgen.Generator
}

func (e *Engine) idg() idgen.Generator {
	if e.IDGen != nil {
		return e.IDGen
	}
	return idgen.UUIDv7Generator{}
}

func (e *Engine) Plan(ctx context.Context, al *AllowlistFile) (PlanReport, error) {
	rep := PlanReport{
		Environment:       e.Env.Environment,
		Database:          e.Env.Database,
		AllowlistCount:    len(al.Records),
		AllowlistChecksum: al.FileChecksum,
		RecordIDs:         al.RecordIDs(),
		Mutations:         0,
	}
	if err := e.Env.Validate(); err != nil {
		rep.Freshness = "BLOCKED"
		rep.BlockedReasons = append(rep.BlockedReasons, err.Error())
		return rep, err
	}
	tx, err := e.Store.Begin(ctx)
	if err != nil {
		return rep, err
	}
	defer func() { _ = tx.Rollback() }()

	if err := freshnessCheck(ctx, tx, al); err != nil {
		rep.Freshness = "STALE_DRY_RUN"
		rep.BlockedReasons = append(rep.BlockedReasons, err.Error())
		return rep, err
	}
	rep.Freshness = "PASS"
	rep.ProposedUpdates = ExpectedRecords
	return rep, nil
}

func freshnessCheck(ctx context.Context, tx Tx, al *AllowlistFile) error {
	for _, rec := range al.SortedRecords() {
		row, err := tx.Get(ctx, rec.DisclosureTypeID, rec.VersionNo)
		if err != nil {
			return fmt.Errorf("STALE_DRY_RUN: missing %s: %w", rec.RecordID, err)
		}
		flat := strings.TrimSpace(row.LegalBasis)
		if ShortHash(flat) != rec.FlatHash {
			return fmt.Errorf("STALE_DRY_RUN: flat hash mismatch %s", rec.RecordID)
		}
		if !StructuredEmpty(row.LegalBasesJSON) {
			return fmt.Errorf("STALE_DRY_RUN: structured present %s", rec.RecordID)
		}
		inv := inventory.ClassifyRecord(inventory.Record{
			TypeID: row.TypeID, VersionNo: row.VersionNo,
			LegalBasis: row.LegalBasis, LegalBasesJSON: row.LegalBasesJSON,
		})
		if inv.Group != inventory.GroupA {
			return fmt.Errorf("STALE_DRY_RUN: %s group=%s want A", rec.RecordID, inv.Group)
		}
		for _, a := range inv.Anomalies {
			if a == inventory.AnomalyMalformedJSON || a == inventory.AnomalyProjectionOverflow {
				return fmt.Errorf("STALE_DRY_RUN: anomaly %s on %s", a, rec.RecordID)
			}
		}
	}
	return nil
}

// Apply runs all-or-nothing CAS updates. Requires confirm token already validated by caller.
func (e *Engine) Apply(ctx context.Context, al *AllowlistFile, snap *SnapshotFile) (ApplyReport, error) {
	rep := ApplyReport{
		Environment:       e.Env.Environment,
		AllowlistChecksum: al.FileChecksum,
	}
	if err := e.Env.Validate(); err != nil {
		rep.Status = "BLOCKED"
		rep.ErrorCode = "ENV_GUARD"
		return rep, err
	}
	if err := ValidateSnapshot(snap, al); err != nil {
		rep.Status = "BLOCKED"
		rep.ErrorCode = "SNAPSHOT_INVALID"
		return rep, err
	}
	if snap.AllowlistFileChecksum != "" && snap.AllowlistFileChecksum != al.FileChecksum {
		rep.Status = "BLOCKED"
		rep.ErrorCode = "ALLOWLIST_CHECKSUM_MISMATCH"
		return rep, fmt.Errorf("snapshot allowlist checksum mismatch")
	}

	tx, err := e.Store.Begin(ctx)
	if err != nil {
		rep.Status = "ERROR"
		rep.ErrorCode = "BEGIN"
		return rep, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	if err := freshnessCheck(ctx, tx, al); err != nil {
		rep.Status = "STALE_DRY_RUN"
		rep.ErrorCode = "STALE_DRY_RUN"
		return rep, err
	}

	// Snapshot must match pre-state flats
	snapByID := map[string]SnapshotRecord{}
	for _, r := range snap.Records {
		snapByID[r.RecordID] = r
	}

	var idHashes []string
	var projHashes []string
	affected := 0

	for _, rec := range al.SortedRecords() {
		row, err := tx.Get(ctx, rec.DisclosureTypeID, rec.VersionNo)
		if err != nil {
			rep.Status = "ROLLBACK"
			rep.ErrorCode = "MISSING"
			return rep, err
		}
		sr, ok := snapByID[rec.RecordID]
		if !ok {
			rep.Status = "ROLLBACK"
			rep.ErrorCode = "SNAPSHOT_GAP"
			return rep, fmt.Errorf("snapshot missing %s", rec.RecordID)
		}
		if row.LegalBasis != sr.LegalBasis {
			rep.Status = "ROLLBACK"
			rep.ErrorCode = "SNAPSHOT_STALE"
			return rep, fmt.Errorf("snapshot flat mismatch %s", rec.RecordID)
		}

		tr, err := TransformWrapLegacy(rec.DisclosureTypeID, rec.VersionNo, row.LegalBasis, e.idg())
		if err != nil {
			rep.Status = "ROLLBACK"
			rep.ErrorCode = "TRANSFORM"
			return rep, err
		}
		if err := AssertProjectionMatchesAllowlist(tr, rec); err != nil {
			rep.Status = "ROLLBACK"
			rep.ErrorCode = "PROJECTION"
			return rep, err
		}
		n, err := tx.CASUpdate(ctx, rec.DisclosureTypeID, rec.VersionNo, row.LegalBasis, tr.Projection, tr.JSONBytes)
		if err != nil {
			rep.Status = "ROLLBACK"
			rep.ErrorCode = "CAS_ERR"
			return rep, err
		}
		if n != 1 {
			rep.Status = "ROLLBACK"
			rep.ErrorCode = "CAS_ROWS"
			return rep, fmt.Errorf("%s RowsAffected=%d", rec.RecordID, n)
		}
		affected++
		idHashes = append(idHashes, tr.ItemIDHash)
		projHashes = append(projHashes, tr.ProjectionHash)
	}

	// uniqueness of id hashes
	seen := map[string]struct{}{}
	for _, h := range idHashes {
		if _, ok := seen[h]; ok {
			rep.Status = "ROLLBACK"
			rep.ErrorCode = "UUID_DUP"
			return rep, fmt.Errorf("duplicate uuid hash")
		}
		seen[h] = struct{}{}
	}

	// in-txn verify Group C
	vr, err := verifyTx(ctx, tx, al)
	if err != nil || vr.Status != "PASS" {
		rep.Status = "ROLLBACK"
		rep.ErrorCode = "IN_TXN_VERIFY"
		if err == nil {
			err = fmt.Errorf("in-txn verify %s", vr.Status)
		}
		return rep, err
	}

	if err := tx.Commit(); err != nil {
		rep.Status = "ROLLBACK"
		rep.ErrorCode = "COMMIT"
		return rep, err
	}
	committed = true
	rep.Status = "APPLIED"
	rep.RowsAffected = affected
	rep.ItemIDHashes = idHashes
	rep.ProjectionHashes = projHashes
	rep.Verify = &vr
	return rep, nil
}
