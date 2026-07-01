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

type delegationGrantRow struct {
	caapp.DelegatedAdminGrant
}

func (r *AdminRepository) delegationGrants() map[string]*delegationGrantRow {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.delegatedGrants == nil {
		r.delegatedGrants = make(map[string]*delegationGrantRow)
	}
	return r.delegatedGrants
}

func (r *AdminRepository) InsertDelegationGrant(_ context.Context, in caapp.InsertDelegationGrantInput) (*caapp.DelegatedAdminGrant, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.delegatedGrants == nil {
		r.delegatedGrants = make(map[string]*delegationGrantRow)
	}
	for _, g := range r.delegatedGrants {
		if g.CompanyID == in.CompanyID &&
			g.DelegateeMembershipID == in.DelegateeMembershipID &&
			g.ScopeType == in.ScopeType &&
			g.ScopeID == in.ScopeID &&
			g.Status == caapp.DelegationStatusActive {
			return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "active delegation grant already exists for delegatee and scope", nil)
		}
	}
	now := time.Now().UTC()
	perms := append([]string(nil), in.PermissionSet...)
	row := &delegationGrantRow{
		DelegatedAdminGrant: caapp.DelegatedAdminGrant{
			DelegationID:          in.ID,
			CompanyID:             in.CompanyID,
			DelegateeMembershipID: in.DelegateeMembershipID,
			DelegatorMembershipID: in.DelegatorMembershipID,
			ScopeType:             in.ScopeType,
			ScopeID:               in.ScopeID,
			PermissionSet:         perms,
			Status:                caapp.DelegationStatusActive,
			CreatedAt:             now,
			CreatedBy:             in.CreatedBy,
			UpdatedAt:             now,
			UpdatedBy:             in.CreatedBy,
		},
	}
	r.delegatedGrants[in.ID] = row
	out := row.DelegatedAdminGrant
	return &out, nil
}

func (r *AdminRepository) GetDelegationGrant(_ context.Context, companyID, delegationID string) (*caapp.DelegatedAdminGrant, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	row, ok := r.delegatedGrants[delegationID]
	if !ok || row.CompanyID != companyID {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeNotFound, "delegation not found", nil)
	}
	out := row.DelegatedAdminGrant
	return &out, nil
}

func (r *AdminRepository) ListDelegationGrants(_ context.Context, companyID, status, delegateeMembershipID, scopeID string, limit int) ([]caapp.DelegatedAdminGrant, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []caapp.DelegatedAdminGrant
	for _, row := range r.delegatedGrants {
		if row.CompanyID != companyID {
			continue
		}
		if s := strings.TrimSpace(status); s != "" && row.Status != s {
			continue
		}
		if d := strings.TrimSpace(delegateeMembershipID); d != "" && row.DelegateeMembershipID != d {
			continue
		}
		if sid := strings.TrimSpace(scopeID); sid != "" && row.ScopeID != sid {
			continue
		}
		out = append(out, row.DelegatedAdminGrant)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (r *AdminRepository) ListActiveDelegationsForDelegatee(ctx context.Context, companyID, delegateeMembershipID string) ([]caapp.DelegatedAdminGrant, error) {
	return r.ListDelegationGrants(ctx, companyID, caapp.DelegationStatusActive, delegateeMembershipID, "", 100)
}

func (r *AdminRepository) HasActiveDelegationGrant(_ context.Context, companyID, delegateeMembershipID, scopeType, scopeID string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, row := range r.delegatedGrants {
		if row.CompanyID == companyID &&
			row.DelegateeMembershipID == delegateeMembershipID &&
			row.ScopeType == scopeType &&
			row.ScopeID == scopeID &&
			row.Status == caapp.DelegationStatusActive {
			return true, nil
		}
	}
	return false, nil
}

func (r *AdminRepository) UpdateDelegationGrantPermissions(_ context.Context, companyID, delegationID string, permissionSet []string, updatedBy string) (*caapp.DelegatedAdminGrant, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	row, ok := r.delegatedGrants[delegationID]
	if !ok || row.CompanyID != companyID {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeNotFound, "delegation not found", nil)
	}
	if row.Status != caapp.DelegationStatusActive {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "delegation is not active", nil)
	}
	row.PermissionSet = append([]string(nil), permissionSet...)
	row.UpdatedAt = time.Now().UTC()
	row.UpdatedBy = updatedBy
	out := row.DelegatedAdminGrant
	return &out, nil
}

func (r *AdminRepository) RevokeDelegationGrant(_ context.Context, companyID, delegationID, updatedBy string) (*caapp.DelegatedAdminGrant, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	row, ok := r.delegatedGrants[delegationID]
	if !ok || row.CompanyID != companyID {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeNotFound, "delegation not found", nil)
	}
	if row.Status == caapp.DelegationStatusRevoked {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "delegation already revoked", nil)
	}
	row.Status = caapp.DelegationStatusRevoked
	row.UpdatedAt = time.Now().UTC()
	row.UpdatedBy = updatedBy
	out := row.DelegatedAdminGrant
	return &out, nil
}
