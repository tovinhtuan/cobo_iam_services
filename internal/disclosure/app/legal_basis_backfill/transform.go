package backfill

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

// TransformResult is the wrap payload without logging legal text.
type TransformResult struct {
	TypeID           string
	VersionNo        int
	OriginalFlatHash string
	Projection       string
	ProjectionHash   string
	JSONBytes        []byte
	ItemIDHash       string // hash of UUID only
	ItemCount        int
}

func TransformWrapLegacy(typeID string, versionNo int, originalFlat string, idg idgen.Generator) (TransformResult, error) {
	flat := strings.TrimSpace(originalFlat)
	if flat == "" {
		return TransformResult{}, fmt.Errorf("%s:%d empty flat", typeID, versionNo)
	}
	if idg == nil {
		idg = idgen.UUIDv7Generator{}
	}
	id := idg.NewUUID()
	if id == "" || strings.Contains(id, "-lb-legacy-") {
		return TransformResult{}, fmt.Errorf("invalid persistence id")
	}
	item := disclosureapp.LegalBasisDTO{
		ID:        id,
		Title:     LegacyTitle,
		Code:      "",
		Authority: "",
		IssueDate: "",
		Summary:   flat,
		Link:      "",
	}
	validated, err := disclosureapp.ValidateLegalBasesForWrite([]disclosureapp.LegalBasisDTO{item}, idg)
	if err != nil {
		return TransformResult{}, err
	}
	if len(validated) != 1 {
		return TransformResult{}, fmt.Errorf("expected 1 validated item")
	}
	if strings.Contains(validated[0].ID, "-lb-legacy-") {
		return TransformResult{}, fmt.Errorf("display id rejected")
	}
	proj, err := disclosureapp.ProjectLegalBasesToLegacy(validated)
	if err != nil {
		return TransformResult{}, err
	}
	if ShortHash(proj) != TargetFlatHashOD7 {
		// Still assert equality with live projector; golden may drift only if title changes.
		if utf8.RuneCountInString(proj) != utf8.RuneCountInString(LegacyTitle) || proj != LegacyTitle {
			return TransformResult{}, fmt.Errorf("projection mismatch contract: got hash=%s", ShortHash(proj))
		}
	}
	b, err := json.Marshal(validated)
	if err != nil {
		return TransformResult{}, err
	}
	return TransformResult{
		TypeID:           typeID,
		VersionNo:        versionNo,
		OriginalFlatHash: ShortHash(flat),
		Projection:       proj,
		ProjectionHash:   ShortHash(proj),
		JSONBytes:        b,
		ItemIDHash:       ShortHash(validated[0].ID),
		ItemCount:        1,
	}, nil
}

func AssertProjectionMatchesAllowlist(tr TransformResult, rec AllowlistRecord) error {
	if tr.ProjectionHash != rec.TargetProjectionHash {
		return fmt.Errorf("%s: projection hash %s != allowlist %s", rec.RecordID, tr.ProjectionHash, rec.TargetProjectionHash)
	}
	if tr.OriginalFlatHash != rec.FlatHash {
		return fmt.Errorf("%s: original flat hash mismatch (stale input)", rec.RecordID)
	}
	return nil
}
