package app

import (
	"context"
	"time"
)

// Subject is the authenticated caller for personal operational overview.
type Subject struct {
	UserID       string
	MembershipID string
	CompanyID    string
}

// MineRecord is a disclosure record matched by mine semantics for one membership.
type MineRecord struct {
	CompanyID      string
	CompanyName    string
	MembershipID   string
	RecordID       string
	Title          string
	RecordStatus   string
	PlannedDate    string // YYYY-MM-DD from planned_date
	AdHocDueDate   string // YYYY-MM-DD from approved ad-hoc (may be empty)
	Confirmed      bool
	MatchedViaTask bool
	MatchedViaAsgn bool
	// CompletedAt is disclosure_records.completed_at (forward-only). Nil when absent.
	CompletedAt *time.Time
}

// MineTask is an open workflow task assigned to one of the user's memberships.
type MineTask struct {
	TaskID       string
	CompanyID    string
	CompanyName  string
	MembershipID string
	RecordID     string
	Title        string
	StepCode     string
	TaskStatus   string
	PlannedDate  string
	AdHocDueDate string
	RecordStatus string
}

// MineRepository loads mine-scoped operational rows. Implementations MUST NOT
// expand via rbac.manage / company-wide / department visibility.
type MineRepository interface {
	ListMineRecords(ctx context.Context, membershipIDs []string) ([]MineRecord, error)
	ListMineOpenTasks(ctx context.Context, membershipIDs []string, limit int) ([]MineTask, error)
	// ListApprovedAdHocDues returns map key "companyID|recordID" → due YYYY-MM-DD.
	ListApprovedAdHocDues(ctx context.Context, companyIDs []string) (map[string]string, error)
}

// EmptyMineRepository returns empty exact results (used when DB is unavailable).
type EmptyMineRepository struct{}

func (EmptyMineRepository) ListMineRecords(context.Context, []string) ([]MineRecord, error) {
	return nil, nil
}
func (EmptyMineRepository) ListMineOpenTasks(context.Context, []string, int) ([]MineTask, error) {
	return nil, nil
}
func (EmptyMineRepository) ListApprovedAdHocDues(context.Context, []string) (map[string]string, error) {
	return map[string]string{}, nil
}

// Clock abstracts time for tests.
type Clock interface {
	Now() time.Time
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now().UTC() }
