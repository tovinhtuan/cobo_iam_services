package periodic_oneshot

import (
	"context"
	"fmt"
	"sync"
	"time"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	"github.com/cobo/cobo_iam_services/internal/disclosure/app/applicability"
)

// MemoryDomain is an in-memory Domain for unit/integration tests.
type MemoryDomain struct {
	mu         sync.Mutex
	Type       TypeSnapshot
	Profile    applicability.CompanyApplicabilityProfile
	Cycles     map[string]disclosureapp.PeriodicCycleRow // key type|company|label
	Claimed    map[string]bool
	Records    map[string]string // cycleID -> recordID
	FailCreate bool
	Seq        int
	Loc        *time.Location
	Calc       *disclosureapp.DeadlineCalculator
}

func NewMemoryDomain(t TypeSnapshot, profile applicability.CompanyApplicabilityProfile) *MemoryDomain {
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	return &MemoryDomain{
		Type:    t,
		Profile: profile,
		Cycles:  map[string]disclosureapp.PeriodicCycleRow{},
		Claimed: map[string]bool{},
		Records: map[string]string{},
		Loc:     loc,
		Calc:    disclosureapp.NewDeadlineCalculator(nil),
	}
}

func cycleKey(typeID, companyID, label string) string {
	return typeID + "|" + companyID + "|" + label
}

func (m *MemoryDomain) Location() *time.Location { return m.Loc }

func (m *MemoryDomain) NewCycleID() string {
	m.Seq++
	return fmt.Sprintf("cycle-%d", m.Seq)
}

func (m *MemoryDomain) ComputeDue(ctx context.Context, cycleStart time.Time, deadlineDays int, durationType string) (time.Time, error) {
	return m.Calc.AddDurationInclusive(ctx, cycleStart, deadlineDays, durationType)
}

func (m *MemoryDomain) LoadType(context.Context, string, string) (TypeSnapshot, error) {
	return m.Type, nil
}

func (m *MemoryDomain) LoadCompanyProfile(context.Context, string) (applicability.CompanyApplicabilityProfile, error) {
	return m.Profile, nil
}

func (m *MemoryDomain) LoadCycle(_ context.Context, typeID, companyID, cycleLabel string) (CycleSnapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	row, ok := m.Cycles[cycleKey(typeID, companyID, cycleLabel)]
	if !ok {
		return CycleSnapshot{Exists: false}, nil
	}
	out := CycleSnapshot{Exists: true, CycleID: row.CycleID, CycleLabel: row.CycleLabel, RecordID: row.RecordID}
	if !row.CycleStart.IsZero() {
		out.CycleStart = row.CycleStart.Format("2006-01-02")
	}
	if !row.DueDate.IsZero() {
		out.DueDate = row.DueDate.Format("2006-01-02")
	}
	return out, nil
}

func (m *MemoryDomain) InsertCycle(_ context.Context, row disclosureapp.PeriodicCycleRow) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := cycleKey(row.TypeID, row.CompanyID, row.CycleLabel)
	if _, ok := m.Cycles[k]; ok {
		return fmt.Errorf("duplicate cycle")
	}
	m.Cycles[k] = row
	return nil
}

func (m *MemoryDomain) DeleteUnmaterializedCycle(_ context.Context, cycleID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, row := range m.Cycles {
		if row.CycleID == cycleID && row.RecordID == "" {
			delete(m.Cycles, k)
			delete(m.Claimed, cycleID)
		}
	}
	return nil
}

func (m *MemoryDomain) ClaimCycle(_ context.Context, cycleID string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Claimed[cycleID] {
		return false, nil
	}
	m.Claimed[cycleID] = true
	return true, nil
}

func (m *MemoryDomain) ReleaseCycle(_ context.Context, cycleID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.Claimed, cycleID)
	return nil
}

func (m *MemoryDomain) UpdateCycleRecord(_ context.Context, cycleID, recordID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, row := range m.Cycles {
		if row.CycleID == cycleID {
			row.RecordID = recordID
			m.Cycles[k] = row
			m.Records[cycleID] = recordID
			return nil
		}
	}
	return fmt.Errorf("cycle not found")
}

func (m *MemoryDomain) CreateAndSubmitRecordWithPlannedDate(context.Context, string, string, string, string, *time.Time, string) (string, string, error) {
	if m.FailCreate {
		return "", "", fmt.Errorf("forced create failure")
	}
	m.Seq++
	return fmt.Sprintf("rec-%d", m.Seq), fmt.Sprintf("wf-%d", m.Seq), nil
}
