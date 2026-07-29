package backfill

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Row is the mutable legal-basis state for a version PK.
type Row struct {
	TypeID         string
	VersionNo      int
	LegalBasis     string
	LegalBasesJSON []byte // nil => SQL NULL
	UpdatedBy      string
	ActivatedAt    string
}

func (r Row) RecordID() string { return fmt.Sprintf("%s:%d", r.TypeID, r.VersionNo) }

func StructuredEmpty(jsonRaw []byte) bool {
	if len(jsonRaw) == 0 {
		return true
	}
	s := strings.TrimSpace(string(jsonRaw))
	return s == "" || s == "null" || s == "[]"
}

// Store abstracts persistence for synthetic tests and future SQL wiring.
type Store interface {
	Begin(ctx context.Context) (Tx, error)
}

type Tx interface {
	Get(ctx context.Context, typeID string, versionNo int) (Row, error)
	// CASUpdate applies new flat+json only if current flat exact-match and structured empty.
	// updated_by is preserved (policy locked Phase 12.6B-I).
	CASUpdate(ctx context.Context, typeID string, versionNo int, expectFlat string, newFlat string, newJSON []byte) (rowsAffected int64, err error)
	// RestoreExact restores snapshot values if current flat+json hashes match expected post-state.
	RestoreExact(ctx context.Context, typeID string, versionNo int, expectFlat string, expectJSON []byte, snap Row) (rowsAffected int64, err error)
	Commit() error
	Rollback() error
}

// MemoryStore is an in-memory all-or-nothing store for tests.
type MemoryStore struct {
	mu   sync.Mutex
	rows map[string]Row
}

func NewMemoryStore(rows []Row) *MemoryStore {
	m := &MemoryStore{rows: map[string]Row{}}
	for _, r := range rows {
		m.rows[r.RecordID()] = r
	}
	return m
}

func (m *MemoryStore) Begin(ctx context.Context) (Tx, error) {
	m.mu.Lock()
	cp := make(map[string]Row, len(m.rows))
	for k, v := range m.rows {
		cp[k] = cloneRow(v)
	}
	return &memoryTx{parent: m, draft: cp, open: true}, nil
}

func cloneRow(r Row) Row {
	out := r
	if r.LegalBasesJSON != nil {
		out.LegalBasesJSON = append([]byte(nil), r.LegalBasesJSON...)
	}
	return out
}

type memoryTx struct {
	parent *MemoryStore
	draft  map[string]Row
	open   bool
}

func (t *memoryTx) Get(ctx context.Context, typeID string, versionNo int) (Row, error) {
	id := fmt.Sprintf("%s:%d", typeID, versionNo)
	r, ok := t.draft[id]
	if !ok {
		return Row{}, fmt.Errorf("missing %s", id)
	}
	return cloneRow(r), nil
}

func (t *memoryTx) CASUpdate(ctx context.Context, typeID string, versionNo int, expectFlat string, newFlat string, newJSON []byte) (int64, error) {
	id := fmt.Sprintf("%s:%d", typeID, versionNo)
	r, ok := t.draft[id]
	if !ok {
		return 0, nil
	}
	if r.LegalBasis != expectFlat {
		return 0, nil
	}
	if !StructuredEmpty(r.LegalBasesJSON) {
		return 0, nil
	}
	r.LegalBasis = newFlat
	r.LegalBasesJSON = append([]byte(nil), newJSON...)
	// preserve UpdatedBy / ActivatedAt
	t.draft[id] = r
	return 1, nil
}

func (t *memoryTx) RestoreExact(ctx context.Context, typeID string, versionNo int, expectFlat string, expectJSON []byte, snap Row) (int64, error) {
	id := fmt.Sprintf("%s:%d", typeID, versionNo)
	r, ok := t.draft[id]
	if !ok {
		return 0, nil
	}
	if r.LegalBasis != expectFlat {
		return 0, nil
	}
	if string(r.LegalBasesJSON) != string(expectJSON) {
		return 0, nil
	}
	t.draft[id] = cloneRow(snap)
	return 1, nil
}

func (t *memoryTx) Commit() error {
	if !t.open {
		return fmt.Errorf("tx closed")
	}
	t.parent.rows = t.draft
	t.open = false
	t.parent.mu.Unlock()
	return nil
}

func (t *memoryTx) Rollback() error {
	if !t.open {
		return nil
	}
	t.open = false
	t.parent.mu.Unlock()
	return nil
}

func (m *MemoryStore) Snapshot() []Row {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Row, 0, len(m.rows))
	for _, r := range m.rows {
		out = append(out, cloneRow(r))
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].TypeID == out[j].TypeID {
			return out[i].VersionNo < out[j].VersionNo
		}
		return out[i].TypeID < out[j].TypeID
	})
	return out
}
