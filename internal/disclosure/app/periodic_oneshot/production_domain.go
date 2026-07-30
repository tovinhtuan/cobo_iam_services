package periodic_oneshot

import (
	"context"
	"fmt"
	"strings"
	"time"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	"github.com/cobo/cobo_iam_services/internal/disclosure/app/applicability"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

// ProductionDomain wires MySQL disclosure repos + calculator + periodic record creator.
type ProductionDomain struct {
	Repo    disclosureapp.Repository
	Creator disclosureapp.PeriodicRecordCreator
	Calc    *disclosureapp.DeadlineCalculator
	IDGen   idgen.Generator
	Loc     *time.Location
}

func (d *ProductionDomain) Location() *time.Location {
	if d.Loc != nil {
		return d.Loc
	}
	return time.UTC
}

func (d *ProductionDomain) NewCycleID() string {
	return d.IDGen.NewUUID()
}

func (d *ProductionDomain) ComputeDue(ctx context.Context, cycleStart time.Time, deadlineDays int, durationType string) (time.Time, error) {
	return d.Calc.AddDurationInclusive(ctx, cycleStart, deadlineDays, durationType)
}

func (d *ProductionDomain) LoadType(ctx context.Context, typeID, companyID string) (TypeSnapshot, error) {
	detail, err := d.Repo.GetTypeDetail(ctx, companyID, typeID)
	if err != nil {
		return TypeSnapshot{}, err
	}
	if detail == nil {
		return TypeSnapshot{}, fmt.Errorf("type not found")
	}
	cfg := detail.DeadlineConfig
	mode := ""
	days := 0
	freq := ""
	if cfg != nil {
		mode = strings.TrimSpace(cfg.DeadlineMode)
		days = cfg.DeadlineDays
		freq = strings.TrimSpace(cfg.FrequencyUnit)
	}
	dayType := ""
	if detail.ApplicabilityRules != nil {
		dayType = strings.TrimSpace(detail.ApplicabilityRules.DeadlineDayType)
	}
	hasWF := detail.HasWorkflow
	if !hasWF {
		flag, err := d.Repo.HasActiveEnterpriseWorkflow(ctx, companyID, typeID)
		if err != nil {
			return TypeSnapshot{}, err
		}
		hasWF = flag
	}
	status := "inactive"
	types, err := d.Repo.ListActivePeriodicTypes(ctx)
	if err != nil {
		return TypeSnapshot{}, err
	}
	for _, t := range types {
		if t.TypeID == typeID {
			status = "active"
			if days == 0 {
				days = t.DeadlineDays
			}
			if freq == "" {
				freq = t.FrequencyUnit
			}
			if detail.ApplicabilityRules == nil {
				detail.ApplicabilityRules = t.ApplicabilityRules
			}
			break
		}
	}
	return TypeSnapshot{
		TypeID:             detail.TypeID,
		TypeName:           detail.Name,
		Status:             status,
		ActiveVersionNo:    detail.VersionNo,
		DeadlineMode:       mode,
		DeadlineDays:       days,
		FrequencyUnit:      freq,
		DeadlineDayType:    dayType,
		IsGlobal:           strings.TrimSpace(detail.OwnerCompanyID) == "" && (detail.Scope == "" || detail.Scope == "global"),
		ApplicabilityRules: detail.ApplicabilityRules,
		HasWorkflow:        hasWF,
	}, nil
}

func (d *ProductionDomain) LoadCompanyProfile(ctx context.Context, companyID string) (applicability.CompanyApplicabilityProfile, error) {
	return d.Repo.GetCompanyApplicabilityProfile(ctx, companyID)
}

func (d *ProductionDomain) LoadCycle(ctx context.Context, typeID, companyID, cycleLabel string) (CycleSnapshot, error) {
	row, err := d.Repo.GetPeriodicCycle(ctx, typeID, companyID, cycleLabel)
	if err != nil {
		return CycleSnapshot{}, err
	}
	if row == nil {
		return CycleSnapshot{Exists: false}, nil
	}
	out := CycleSnapshot{
		Exists:     true,
		CycleID:    row.CycleID,
		CycleLabel: row.CycleLabel,
		RecordID:   strings.TrimSpace(row.RecordID),
	}
	if !row.CycleStart.IsZero() {
		out.CycleStart = row.CycleStart.Format("2006-01-02")
	}
	if !row.DueDate.IsZero() {
		out.DueDate = row.DueDate.Format("2006-01-02")
	}
	return out, nil
}

func (d *ProductionDomain) InsertCycle(ctx context.Context, row disclosureapp.PeriodicCycleRow) error {
	return d.Repo.InsertPeriodicCycle(ctx, row)
}

func (d *ProductionDomain) DeleteUnmaterializedCycle(ctx context.Context, cycleID string) error {
	return d.Repo.DeleteUnmaterializedPeriodicCycle(ctx, cycleID)
}

func (d *ProductionDomain) ClaimCycle(ctx context.Context, cycleID string) (bool, error) {
	return d.Repo.TryClaimPeriodicCycle(ctx, cycleID)
}

func (d *ProductionDomain) ReleaseCycle(ctx context.Context, cycleID string) error {
	return d.Repo.ReleasePeriodicCycleClaim(ctx, cycleID)
}

func (d *ProductionDomain) UpdateCycleRecord(ctx context.Context, cycleID, recordID string) error {
	return d.Repo.UpdateCycleRecord(ctx, cycleID, recordID)
}

func (d *ProductionDomain) CreateAndSubmitRecordWithPlannedDate(ctx context.Context, companyID, typeID, createdByMembershipID, title string, t0Date *time.Time, plannedDate string) (string, string, error) {
	return d.Creator.CreateAndSubmitRecordWithPlannedDate(ctx, companyID, typeID, createdByMembershipID, title, t0Date, plannedDate)
}
