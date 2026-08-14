package companyplan

import (
	"strings"
	"time"
)

// ActivateOutcome is the result of a platform-admin manual plan activation.
type ActivateOutcome struct {
	Plan          CompanyPlan
	AlreadyActive bool
	PreviousCode  PlanCode
	ClosedIDs     []string
}

type pendingClose struct {
	ID        string
	ExpiresAt *time.Time
	Status    PlanStatus
}

func prepareImmediateActivation(existing []CompanyPlan, companyID string, code PlanCode, origin RecordOrigin, newID string, now time.Time) (closes []pendingClose, create CompanyPlan, already *CompanyPlan, previous PlanCode, err error) {
	companyID = strings.TrimSpace(companyID)
	newID = strings.TrimSpace(newID)
	if companyID == "" || newID == "" {
		return nil, CompanyPlan{}, nil, "", ErrInvalidPlan
	}
	if !ValidPaidManualPlanCode(code) {
		return nil, CompanyPlan{}, nil, "", ErrUnsupportedManualPlan
	}
	if strings.TrimSpace(string(origin)) == "" {
		origin = RecordOriginPlatformAdminManual
	}
	now = now.UTC()

	covering := SelectEffectivePlan(existing, now)
	if covering != nil && covering.Status == PlanStatusActive && covering.Code == code {
		out := *covering
		return nil, CompanyPlan{}, &out, covering.Code, nil
	}

	create = CompanyPlan{
		ID:            newID,
		CompanyID:     companyID,
		Code:          code,
		Status:        PlanStatusActive,
		EffectiveFrom: now,
		ExpiresAt:     nil,
		Origin:        origin,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := ValidateCreate(create); err != nil {
		return nil, CompanyPlan{}, nil, "", err
	}

	for _, row := range existing {
		if row.CompanyID != companyID {
			continue
		}
		if !IsOccupyingStatus(row.Status) {
			continue
		}
		if !WindowsOverlap(row.EffectiveFrom, row.ExpiresAt, create.EffectiveFrom, create.ExpiresAt) {
			continue
		}
		cl := pendingClose{ID: row.ID, Status: row.Status}
		if row.EffectiveFrom.UTC().Before(now) {
			exp := now
			cl.ExpiresAt = &exp
		} else {
			cl.Status = PlanStatusCancelled
			cl.ExpiresAt = row.ExpiresAt
		}
		closes = append(closes, cl)
	}

	var remaining []CompanyPlan
	closed := map[string]pendingClose{}
	for _, c := range closes {
		closed[c.ID] = c
	}
	for _, row := range existing {
		if adj, ok := closed[row.ID]; ok {
			row.Status = adj.Status
			row.ExpiresAt = adj.ExpiresAt
		}
		remaining = append(remaining, row)
	}
	if OccupyingOverlap(remaining, create, "") {
		return nil, CompanyPlan{}, nil, "", ErrOverlap
	}

	if covering != nil {
		previous = covering.Code
	}
	return closes, create, nil, previous, nil
}
