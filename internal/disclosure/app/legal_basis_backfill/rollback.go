package backfill

import (
	"context"
	"fmt"
)

type RollbackReport struct {
	Status       string   `json:"status"`
	RowsRestored int      `json:"rowsRestored"`
	ErrorCode    string   `json:"errorCode,omitempty"`
	Failures     []string `json:"failures,omitempty"`
}

// Rollback restores exact snapshot values when current state still matches post-backfill.
func (e *Engine) Rollback(ctx context.Context, al *AllowlistFile, snap *SnapshotFile, postFlatByID map[string]string, postJSONByID map[string][]byte) (RollbackReport, error) {
	rep := RollbackReport{}
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
	tx, err := e.Store.Begin(ctx)
	if err != nil {
		rep.Status = "ERROR"
		return rep, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	snapByID := map[string]SnapshotRecord{}
	for _, r := range snap.Records {
		snapByID[r.RecordID] = r
	}

	restored := 0
	for _, rec := range al.SortedRecords() {
		expectFlat, ok := postFlatByID[rec.RecordID]
		if !ok {
			rep.Status = "REFUSED_STALE"
			rep.ErrorCode = "MISSING_POST_HASH"
			return rep, fmt.Errorf("missing post flat for %s", rec.RecordID)
		}
		expectJSON := postJSONByID[rec.RecordID]
		sr := snapByID[rec.RecordID]
		var jsonPtr *string
		if sr.LegalBasesJSON != nil {
			jsonPtr = sr.LegalBasesJSON
		}
		var snapJSON []byte
		if jsonPtr != nil {
			snapJSON = []byte(*jsonPtr)
		}
		snapRow := Row{
			TypeID: sr.DisclosureTypeID, VersionNo: sr.VersionNo,
			LegalBasis: sr.LegalBasis, LegalBasesJSON: snapJSON,
			UpdatedBy: sr.UpdatedBy, ActivatedAt: sr.ActivatedAt,
		}
		n, err := tx.RestoreExact(ctx, rec.DisclosureTypeID, rec.VersionNo, expectFlat, expectJSON, snapRow)
		if err != nil {
			rep.Status = "ROLLBACK"
			rep.ErrorCode = "RESTORE_ERR"
			return rep, err
		}
		if n != 1 {
			rep.Status = "REFUSED_STALE"
			rep.ErrorCode = "STALE_ROLLBACK"
			rep.Failures = append(rep.Failures, fmt.Sprintf("%s RowsAffected=%d", rec.RecordID, n))
			return rep, fmt.Errorf("STALE_ROLLBACK %s", rec.RecordID)
		}
		restored++
	}
	if err := tx.Commit(); err != nil {
		rep.Status = "ERROR"
		rep.ErrorCode = "COMMIT"
		return rep, err
	}
	committed = true
	rep.Status = "RESTORED"
	rep.RowsRestored = restored
	return rep, nil
}
