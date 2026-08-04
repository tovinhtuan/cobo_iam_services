package companyplan

import "time"

// IntervalEnd returns expires_at or a sentinel far-future when open-ended.
// Used only for overlap comparison; callers should not persist the sentinel.
func IntervalEnd(expiresAt *time.Time) time.Time {
	if expiresAt == nil {
		return time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC)
	}
	return expiresAt.UTC()
}

// WindowsOverlap reports whether half-open intervals [fromA, endA) and [fromB, endB) intersect.
func WindowsOverlap(fromA time.Time, expiresA *time.Time, fromB time.Time, expiresB *time.Time) bool {
	a0 := fromA.UTC()
	a1 := IntervalEnd(expiresA)
	b0 := fromB.UTC()
	b1 := IntervalEnd(expiresB)
	if !a1.After(a0) || !b1.After(b0) {
		// Degenerate / empty window does not occupy space.
		return false
	}
	return a0.Before(b1) && b0.Before(a1)
}

// OccupyingOverlap reports whether candidate overlaps an existing occupying-status plan
// for the same company (excluding excludeID when updating).
func OccupyingOverlap(existing []CompanyPlan, candidate CompanyPlan, excludeID string) bool {
	if !IsOccupyingStatus(candidate.Status) {
		return false
	}
	for _, row := range existing {
		if excludeID != "" && row.ID == excludeID {
			continue
		}
		if row.CompanyID != candidate.CompanyID {
			continue
		}
		if !IsOccupyingStatus(row.Status) {
			continue
		}
		if WindowsOverlap(row.EffectiveFrom, row.ExpiresAt, candidate.EffectiveFrom, candidate.ExpiresAt) {
			return true
		}
	}
	return false
}

// Covers reports whether the plan's time window covers instant at (UTC).
func (p CompanyPlan) Covers(at time.Time) bool {
	t := at.UTC()
	if t.Before(p.EffectiveFrom.UTC()) {
		return false
	}
	if p.ExpiresAt != nil && !t.Before(p.ExpiresAt.UTC()) {
		return false
	}
	return true
}

var statusPrecedence = map[PlanStatus]int{
	PlanStatusActive:    40,
	PlanStatusTrial:     30,
	PlanStatusSuspended: 20,
	PlanStatusExpired:   10,
	PlanStatusCancelled: 5,
}

// SelectEffectivePlan picks the deterministic plan covering at among candidates.
// Prefer higher status precedence, then latest EffectiveFrom, then larger ID.
// Returns nil when none cover at.
func SelectEffectivePlan(candidates []CompanyPlan, at time.Time) *CompanyPlan {
	var best *CompanyPlan
	for i := range candidates {
		p := &candidates[i]
		if !p.Covers(at) {
			continue
		}
		if best == nil {
			best = p
			continue
		}
		bp, cp := statusPrecedence[best.Status], statusPrecedence[p.Status]
		if cp != bp {
			if cp > bp {
				best = p
			}
			continue
		}
		if p.EffectiveFrom.After(best.EffectiveFrom) {
			best = p
			continue
		}
		if p.EffectiveFrom.Equal(best.EffectiveFrom) && p.ID > best.ID {
			best = p
		}
	}
	if best == nil {
		return nil
	}
	out := *best
	return &out
}
