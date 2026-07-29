package backfill

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const SnapshotSchemaVersion = "1"

// SnapshotFile is the secure raw snapshot (must not be committed).
type SnapshotFile struct {
	SchemaVersion             string           `json:"schemaVersion"`
	Environment               string           `json:"environment"`
	DatabaseName              string           `json:"databaseName"`
	HostAlias                 string           `json:"hostAlias"`
	SnapshotTimestamp         string           `json:"snapshotTimestamp"`
	AllowlistAlgorithmVersion string           `json:"allowlistAlgorithmVersion"`
	RecordCount               int              `json:"recordCount"`
	AllowlistFileChecksum     string           `json:"allowlistFileChecksum"`
	Records                   []SnapshotRecord `json:"records"`
	SnapshotChecksum          string           `json:"snapshotChecksum"`
}

type SnapshotRecord struct {
	RecordID         string  `json:"record_id"`
	DisclosureTypeID string  `json:"disclosure_type_id"`
	VersionNo        int     `json:"version_no"`
	LegalBasis       string  `json:"legal_basis"`
	LegalBasesJSON   *string `json:"legal_bases_json"` // null => SQL NULL
	UpdatedBy        string  `json:"updated_by"`
	ActivatedAt      string  `json:"activated_at"`
	RowChecksum      string  `json:"row_checksum"`
}

func rowChecksum(r SnapshotRecord) string {
	jsonPart := "<null>"
	if r.LegalBasesJSON != nil {
		jsonPart = *r.LegalBasesJSON
	}
	payload := fmt.Sprintf("%s\n%d\n%s\n%s\n%s\n%s",
		r.DisclosureTypeID, r.VersionNo, r.LegalBasis, jsonPart, r.UpdatedBy, r.ActivatedAt)
	return ShortHash(payload)
}

func BuildSnapshot(env EnvGuard, al *AllowlistFile, rows []SnapshotRecord) (*SnapshotFile, error) {
	if len(rows) != ExpectedRecords {
		return nil, fmt.Errorf("snapshot rows=%d want %d", len(rows), ExpectedRecords)
	}
	for i := range rows {
		want := fmt.Sprintf("%s:%d", rows[i].DisclosureTypeID, rows[i].VersionNo)
		if rows[i].RecordID == "" {
			rows[i].RecordID = want
		}
		rows[i].RowChecksum = rowChecksum(rows[i])
	}
	s := &SnapshotFile{
		SchemaVersion:             SnapshotSchemaVersion,
		Environment:               env.Environment,
		DatabaseName:              env.Database,
		HostAlias:                 env.HostAlias,
		SnapshotTimestamp:         time.Now().UTC().Format(time.RFC3339),
		AllowlistAlgorithmVersion: AlgorithmVersion,
		RecordCount:               ExpectedRecords,
		AllowlistFileChecksum:     al.FileChecksum,
		Records:                   rows,
	}
	if err := ValidateSnapshot(s, al); err != nil {
		return nil, err
	}
	s.SnapshotChecksum = computeSnapshotChecksum(s)
	return s, nil
}

func computeSnapshotChecksum(s *SnapshotFile) string {
	cp := *s
	cp.SnapshotChecksum = ""
	b, _ := json.Marshal(cp)
	return FileSHA256(b)
}

func ValidateSnapshot(s *SnapshotFile, al *AllowlistFile) error {
	if s == nil {
		return fmt.Errorf("nil snapshot")
	}
	if s.SchemaVersion != SnapshotSchemaVersion {
		return fmt.Errorf("snapshot schemaVersion")
	}
	if s.Environment != "DEV" || s.DatabaseName != "cobo_iam" {
		return fmt.Errorf("snapshot env/db mismatch")
	}
	if s.RecordCount != ExpectedRecords || len(s.Records) != ExpectedRecords {
		return fmt.Errorf("snapshot recordCount")
	}
	alIDs := map[string]struct{}{}
	for _, r := range al.Records {
		alIDs[r.RecordID] = struct{}{}
	}
	seen := map[string]struct{}{}
	for _, r := range s.Records {
		if _, ok := alIDs[r.RecordID]; !ok {
			return fmt.Errorf("snapshot record not in allowlist: %s", r.RecordID)
		}
		if _, ok := seen[r.RecordID]; ok {
			return fmt.Errorf("duplicate snapshot record %s", r.RecordID)
		}
		seen[r.RecordID] = struct{}{}
		if r.RowChecksum == "" || r.RowChecksum != rowChecksum(r) {
			return fmt.Errorf("bad row checksum for %s", r.RecordID)
		}
		if strings.TrimSpace(r.LegalBasis) == "" {
			return fmt.Errorf("snapshot missing legal_basis for %s", r.RecordID)
		}
	}
	if len(seen) != ExpectedRecords {
		return fmt.Errorf("snapshot unique count")
	}
	return nil
}

// WriteSnapshotAtomic writes mode 0600 with temp+rename. Refuses overwrite unless force.
func WriteSnapshotAtomic(path string, s *SnapshotFile, force bool) error {
	if s.SnapshotChecksum == "" {
		s.SnapshotChecksum = computeSnapshotChecksum(s)
	}
	if _, err := os.Stat(path); err == nil && !force {
		return fmt.Errorf("snapshot exists (refuse overwrite): %s", path)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.Write(b); err != nil {
		f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Chmod(tmp, 0o600); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

func LoadSnapshot(path string, al *AllowlistFile) (*SnapshotFile, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s SnapshotFile
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, err
	}
	if err := ValidateSnapshot(&s, al); err != nil {
		return nil, err
	}
	want := computeSnapshotChecksum(&s)
	if s.SnapshotChecksum != want {
		return nil, fmt.Errorf("snapshot checksum mismatch (tamper?)")
	}
	return &s, nil
}
