package backfill

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

const (
	ExpectedRecords   = 6
	AlgorithmVersion  = "v1-wrap-legacy-flat"
	ActionWrapLegacy  = "WRAP_LEGACY_FLAT"
	LegacyTitle       = "Cơ sở pháp lý"
	TargetFlatHashOD7 = "06a6705c2f2481b5" // projection of title-only legacy wrap
)

// AllowlistFile is the committed redacted allowlist schema.
type AllowlistFile struct {
	Phase                      string            `json:"phase"`
	Environment                string            `json:"environment"`
	AlgorithmVersion           string            `json:"algorithmVersion"`
	RecordCount                int               `json:"recordCount"`
	UniqueRecordIDs            int               `json:"uniqueRecordIds"`
	Action                     string            `json:"action"`
	TargetFlatHashUniform      string            `json:"targetFlatHashUniform"`
	TargetFlatRuneCountUniform int               `json:"targetFlatRuneCountUniform"`
	Records                    []AllowlistRecord `json:"records"`
	FileChecksum               string            `json:"-"` // filled after load
}

type AllowlistRecord struct {
	RecordID             string `json:"record_id"`
	DisclosureTypeID     string `json:"disclosure_type_id"`
	VersionNo            int    `json:"version_no"`
	CompanyScope         string `json:"company_scope"`
	TypeStatus           string `json:"type_status"`
	ActiveVersionNo      int    `json:"active_version_no"`
	Group                string `json:"group"`
	FlatRuneCount        int    `json:"flat_rune_count"`
	FlatHash             string `json:"flat_hash"`
	StructuredState      string `json:"structured_state"`
	StructuredCount      int    `json:"structured_count"`
	TargetItemCount      int    `json:"target_item_count"`
	TargetProjectionHash string `json:"target_projection_hash"`
	TargetFlatRuneCount  int    `json:"target_flat_rune_count"`
	Action               string `json:"action"`
}

func LoadAllowlist(path string) (*AllowlistFile, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var a AllowlistFile
	if err := json.Unmarshal(b, &a); err != nil {
		return nil, fmt.Errorf("allowlist json: %w", err)
	}
	a.FileChecksum = FileSHA256(b)
	if err := ValidateAllowlist(&a); err != nil {
		return nil, err
	}
	return &a, nil
}

func ValidateAllowlist(a *AllowlistFile) error {
	if a == nil {
		return fmt.Errorf("nil allowlist")
	}
	if strings.TrimSpace(a.Environment) != "DEV" {
		return fmt.Errorf("allowlist environment must be DEV")
	}
	if a.AlgorithmVersion != AlgorithmVersion {
		return fmt.Errorf("algorithmVersion want %s got %s", AlgorithmVersion, a.AlgorithmVersion)
	}
	if a.RecordCount != ExpectedRecords || len(a.Records) != ExpectedRecords {
		return fmt.Errorf("recordCount must be %d (got count=%d len=%d)", ExpectedRecords, a.RecordCount, len(a.Records))
	}
	if a.UniqueRecordIDs != ExpectedRecords {
		return fmt.Errorf("uniqueRecordIds must be %d", ExpectedRecords)
	}
	if a.Action != ActionWrapLegacy {
		return fmt.Errorf("top-level action must be %s", ActionWrapLegacy)
	}
	seen := map[string]struct{}{}
	for i, r := range a.Records {
		if r.RecordID == "" || r.DisclosureTypeID == "" {
			return fmt.Errorf("record[%d]: missing ids", i)
		}
		wantID := fmt.Sprintf("%s:%d", r.DisclosureTypeID, r.VersionNo)
		if r.RecordID != wantID {
			return fmt.Errorf("record[%d]: record_id %q != %q", i, r.RecordID, wantID)
		}
		if _, ok := seen[r.RecordID]; ok {
			return fmt.Errorf("duplicate record_id %s", r.RecordID)
		}
		seen[r.RecordID] = struct{}{}
		if r.Action != ActionWrapLegacy {
			return fmt.Errorf("%s: action must be %s", r.RecordID, ActionWrapLegacy)
		}
		if r.Group != "A" {
			return fmt.Errorf("%s: group must be A", r.RecordID)
		}
		if r.FlatHash == "" || r.TargetProjectionHash == "" {
			return fmt.Errorf("%s: missing hashes", r.RecordID)
		}
		if r.StructuredCount != 0 || r.TargetItemCount != 1 {
			return fmt.Errorf("%s: structured/target counts invalid", r.RecordID)
		}
		if r.StructuredState != "empty_or_null" {
			return fmt.Errorf("%s: structured_state must be empty_or_null", r.RecordID)
		}
		if r.TargetProjectionHash != a.TargetFlatHashUniform && a.TargetFlatHashUniform != "" {
			return fmt.Errorf("%s: target hash mismatch uniform", r.RecordID)
		}
	}
	if len(seen) != ExpectedRecords {
		return fmt.Errorf("unique ids=%d want %d", len(seen), ExpectedRecords)
	}
	return nil
}

func (a *AllowlistFile) SortedRecords() []AllowlistRecord {
	out := append([]AllowlistRecord(nil), a.Records...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].DisclosureTypeID == out[j].DisclosureTypeID {
			return out[i].VersionNo < out[j].VersionNo
		}
		return out[i].DisclosureTypeID < out[j].DisclosureTypeID
	})
	return out
}

func (a *AllowlistFile) RecordIDs() []string {
	ids := make([]string, 0, len(a.Records))
	for _, r := range a.SortedRecords() {
		ids = append(ids, r.RecordID)
	}
	return ids
}
