package inmemory

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"time"

	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

type emergencyGrantRow struct {
	caapp.EmergencyAccessGrant
	requestedDurationSec int
}

func (r *AdminRepository) InsertEmergencyAccessGrant(_ context.Context, in caapp.InsertEmergencyAccessGrantInput) (*caapp.EmergencyAccessGrant, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.emergencyGrants == nil {
		r.emergencyGrants = make(map[string]*emergencyGrantRow)
	}
	for _, g := range r.emergencyGrants {
		if g.CompanyID == in.CompanyID && g.TargetMembershipID == in.TargetMembershipID && g.Status == caapp.EmergencyStatusActive {
			return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "active emergency access grant already exists for target", nil)
		}
	}
	now := time.Now().UTC()
	caps := append([]string(nil), in.CapabilitySet...)
	row := &emergencyGrantRow{
		EmergencyAccessGrant: caapp.EmergencyAccessGrant{
			SessionID:             in.SessionID,
			CompanyID:             in.CompanyID,
			TargetMembershipID:    in.TargetMembershipID,
			RequesterMembershipID: in.RequesterMembershipID,
			Reason:                in.Reason,
			Scope:                 in.Scope,
			CapabilitySet:            caps,
			RequestedDurationSeconds: in.RequestedDurationSec,
			Status:                   caapp.EmergencyStatusPendingFirst,
			RequestedAt:           now,
		},
		requestedDurationSec: in.RequestedDurationSec,
	}
	r.emergencyGrants[in.SessionID] = row
	out := row.EmergencyAccessGrant
	return &out, nil
}

func (r *AdminRepository) GetEmergencyAccessGrant(_ context.Context, companyID, sessionID string) (*caapp.EmergencyAccessGrant, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	row, ok := r.emergencyGrants[sessionID]
	if !ok || row.CompanyID != companyID {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeNotFound, "emergency access grant not found", nil)
	}
	out := row.EmergencyAccessGrant
	return &out, nil
}

func (r *AdminRepository) ListEmergencyAccessGrants(_ context.Context, companyID, status, targetMembershipID string, limit int) ([]caapp.EmergencyAccessGrant, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]caapp.EmergencyAccessGrant, 0)
	for _, row := range r.emergencyGrants {
		if row.CompanyID != companyID {
			continue
		}
		if s := strings.TrimSpace(status); s != "" && row.Status != s {
			continue
		}
		if t := strings.TrimSpace(targetMembershipID); t != "" && row.TargetMembershipID != t {
			continue
		}
		out = append(out, row.EmergencyAccessGrant)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].RequestedAt.After(out[j].RequestedAt)
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (r *AdminRepository) GetActiveEmergencyGrantForTarget(_ context.Context, companyID, targetMembershipID string) (*caapp.EmergencyAccessGrant, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now().UTC()
	for _, row := range r.emergencyGrants {
		if row.CompanyID != companyID || row.TargetMembershipID != targetMembershipID || row.Status != caapp.EmergencyStatusActive {
			continue
		}
		if row.ExpiresAt != nil && row.ExpiresAt.Before(now) {
			row.Status = caapp.EmergencyStatusExpired
			continue
		}
		out := row.EmergencyAccessGrant
		return &out, nil
	}
	return nil, nil
}

func (r *AdminRepository) HasActiveEmergencyGrantForTarget(ctx context.Context, companyID, targetMembershipID string) (bool, error) {
	g, err := r.GetActiveEmergencyGrantForTarget(ctx, companyID, targetMembershipID)
	if err != nil {
		return false, err
	}
	return g != nil, nil
}

func (r *AdminRepository) RecordEmergencyFirstApproval(_ context.Context, companyID, sessionID, approverMembershipID string) (*caapp.EmergencyAccessGrant, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	row, ok := r.emergencyGrants[sessionID]
	if !ok || row.CompanyID != companyID {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeNotFound, "emergency access grant not found", nil)
	}
	if row.Status != caapp.EmergencyStatusPendingFirst {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "grant is not pending first approval", nil)
	}
	row.ApproverMembershipID1 = approverMembershipID
	row.Status = caapp.EmergencyStatusPendingSecond
	out := row.EmergencyAccessGrant
	return &out, nil
}

func (r *AdminRepository) ActivateEmergencyGrant(_ context.Context, companyID, sessionID, approverMembershipID string, expiresAt time.Time) (*caapp.EmergencyAccessGrant, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	row, ok := r.emergencyGrants[sessionID]
	if !ok || row.CompanyID != companyID {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeNotFound, "emergency access grant not found", nil)
	}
	if row.Status != caapp.EmergencyStatusPendingSecond {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "grant is not pending second approval", nil)
	}
	now := time.Now().UTC()
	row.ApproverMembershipID2 = approverMembershipID
	row.Status = caapp.EmergencyStatusActive
	row.ActivatedAt = &now
	exp := expiresAt.UTC()
	row.ExpiresAt = &exp
	out := row.EmergencyAccessGrant
	return &out, nil
}

func (r *AdminRepository) DenyEmergencyGrant(_ context.Context, companyID, sessionID string) (*caapp.EmergencyAccessGrant, error) {
	return r.setEmergencyTerminalStatus(companyID, sessionID, caapp.EmergencyStatusDenied,
		caapp.EmergencyStatusPendingFirst, caapp.EmergencyStatusPendingSecond)
}

func (r *AdminRepository) CancelEmergencyGrant(_ context.Context, companyID, sessionID string) (*caapp.EmergencyAccessGrant, error) {
	return r.setEmergencyTerminalStatus(companyID, sessionID, caapp.EmergencyStatusCancelled,
		caapp.EmergencyStatusPendingFirst, caapp.EmergencyStatusPendingSecond)
}

func (r *AdminRepository) RevokeEmergencyGrant(_ context.Context, companyID, sessionID string) (*caapp.EmergencyAccessGrant, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	row, ok := r.emergencyGrants[sessionID]
	if !ok || row.CompanyID != companyID {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeNotFound, "emergency access grant not found", nil)
	}
	if row.Status != caapp.EmergencyStatusActive {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "grant is not active", nil)
	}
	now := time.Now().UTC()
	row.Status = caapp.EmergencyStatusRevoked
	row.RevokedAt = &now
	out := row.EmergencyAccessGrant
	return &out, nil
}

func (r *AdminRepository) ExpireEmergencyGrant(_ context.Context, companyID, sessionID string) (*caapp.EmergencyAccessGrant, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	row, ok := r.emergencyGrants[sessionID]
	if !ok || row.CompanyID != companyID {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeNotFound, "emergency access grant not found", nil)
	}
	if row.Status != caapp.EmergencyStatusActive {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "grant is not active", nil)
	}
	row.Status = caapp.EmergencyStatusExpired
	out := row.EmergencyAccessGrant
	return &out, nil
}

func (r *AdminRepository) ExpireDueEmergencyGrants(_ context.Context, companyID string) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now().UTC()
	n := 0
	for _, row := range r.emergencyGrants {
		if row.CompanyID != companyID || row.Status != caapp.EmergencyStatusActive {
			continue
		}
		if row.ExpiresAt != nil && !row.ExpiresAt.After(now) {
			row.Status = caapp.EmergencyStatusExpired
			n++
		}
	}
	return n, nil
}

func (r *AdminRepository) setEmergencyTerminalStatus(companyID, sessionID, terminal string, allowed ...string) (*caapp.EmergencyAccessGrant, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	row, ok := r.emergencyGrants[sessionID]
	if !ok || row.CompanyID != companyID {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeNotFound, "emergency access grant not found", nil)
	}
	okStatus := false
	for _, s := range allowed {
		if row.Status == s {
			okStatus = true
			break
		}
	}
	if !okStatus {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "grant is not pending approval", nil)
	}
	row.Status = terminal
	out := row.EmergencyAccessGrant
	return &out, nil
}

// GetEmergencyGrantRequestedDuration is test-only helper for in-memory activation TTL.
func (r *AdminRepository) GetEmergencyGrantRequestedDuration(sessionID string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	if row, ok := r.emergencyGrants[sessionID]; ok {
		return row.requestedDurationSec
	}
	return 0
}
