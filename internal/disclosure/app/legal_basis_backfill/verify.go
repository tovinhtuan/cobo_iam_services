package backfill

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	inventory "github.com/cobo/cobo_iam_services/internal/disclosure/app/legal_basis_inventory"
)

type VerifyReport struct {
	Status           string         `json:"status"` // PASS | FAIL
	GroupCounts      map[string]int `json:"groupCounts"`
	RecordsChecked   int            `json:"recordsChecked"`
	Failures         []string       `json:"failures,omitempty"`
	ProjectionHashes []string       `json:"projectionHashes,omitempty"`
	ItemIDHashes     []string       `json:"itemIdHashes,omitempty"`
}

func (e *Engine) Verify(ctx context.Context, al *AllowlistFile) (VerifyReport, error) {
	tx, err := e.Store.Begin(ctx)
	if err != nil {
		return VerifyReport{}, err
	}
	defer func() { _ = tx.Rollback() }()
	return verifyTx(ctx, tx, al)
}

func verifyTx(ctx context.Context, tx Tx, al *AllowlistFile) (VerifyReport, error) {
	rep := VerifyReport{
		GroupCounts: map[string]int{"A": 0, "B": 0, "C": 0, "D": 0, "E": 0},
	}
	idSeen := map[string]struct{}{}
	for _, rec := range al.SortedRecords() {
		row, err := tx.Get(ctx, rec.DisclosureTypeID, rec.VersionNo)
		if err != nil {
			rep.Status = "FAIL"
			rep.Failures = append(rep.Failures, err.Error())
			return rep, nil
		}
		rep.RecordsChecked++
		inv := inventory.ClassifyRecord(inventory.Record{
			TypeID: row.TypeID, VersionNo: row.VersionNo,
			LegalBasis: row.LegalBasis, LegalBasesJSON: row.LegalBasesJSON,
		})
		rep.GroupCounts[string(inv.Group)]++
		if inv.Group != inventory.GroupC {
			rep.Failures = append(rep.Failures, fmt.Sprintf("%s group=%s want C", rec.RecordID, inv.Group))
			continue
		}
		var items []disclosureapp.LegalBasisDTO
		if err := json.Unmarshal(row.LegalBasesJSON, &items); err != nil || len(items) != 1 {
			rep.Failures = append(rep.Failures, fmt.Sprintf("%s expected 1 item", rec.RecordID))
			continue
		}
		item := disclosureapp.NormalizeLegalBasisItemForRead(items[0])
		if item.Title != LegacyTitle {
			rep.Failures = append(rep.Failures, fmt.Sprintf("%s title mismatch", rec.RecordID))
		}
		if ShortHash(item.Summary) != rec.FlatHash {
			rep.Failures = append(rep.Failures, fmt.Sprintf("%s summary hash != original flat hash", rec.RecordID))
		}
		if item.Code != "" || item.Authority != "" || item.IssueDate != "" || item.Link != "" {
			rep.Failures = append(rep.Failures, fmt.Sprintf("%s metadata not empty", rec.RecordID))
		}
		if item.ID == "" || strings.Contains(item.ID, "-lb-legacy-") {
			rep.Failures = append(rep.Failures, fmt.Sprintf("%s bad id", rec.RecordID))
		}
		if _, ok := idSeen[item.ID]; ok {
			rep.Failures = append(rep.Failures, "duplicate id")
		}
		idSeen[item.ID] = struct{}{}
		proj, err := disclosureapp.ProjectLegalBasesToLegacy([]disclosureapp.LegalBasisDTO{item})
		if err != nil || proj != row.LegalBasis {
			rep.Failures = append(rep.Failures, fmt.Sprintf("%s projection!=legal_basis", rec.RecordID))
		}
		rep.ProjectionHashes = append(rep.ProjectionHashes, ShortHash(row.LegalBasis))
		rep.ItemIDHashes = append(rep.ItemIDHashes, ShortHash(item.ID))
	}
	if len(rep.Failures) > 0 || rep.GroupCounts["C"] != ExpectedRecords || rep.GroupCounts["A"] != 0 {
		rep.Status = "FAIL"
		return rep, nil
	}
	rep.Status = "PASS"
	return rep, nil
}
