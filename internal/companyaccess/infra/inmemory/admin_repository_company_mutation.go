package inmemory

import (
	"context"
	"net/http"
	"strings"
	"time"

	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// CompanyAuditRow is an audit record kept by the in-memory repository (tests only).
type CompanyAuditRow struct {
	CompanyID string
	Actor     caapp.CompanyAuditContext
	Record    caapp.CompanyAuditRecord
}

// CompanyAuditRows returns the company audit rows written so far (tests).
func (r *AdminRepository) CompanyAuditRows() []CompanyAuditRow {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]CompanyAuditRow(nil), r.companyAudit...)
}

// FailNextCompanyAudit makes the next audit write fail, to test that the mutation rolls back.
func (r *AdminRepository) FailNextCompanyAudit() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.failCompanyAudit = true
}

func snapshotOf(c *caapp.PlatformCompanyDetail) caapp.CompanySnapshot {
	return caapp.SnapshotFromDetail(c)
}

// MutateCompanyPlatform mirrors the MySQL transaction: everything is applied under one lock, and
// nothing is applied when the audit write fails.
func (r *AdminRepository) MutateCompanyPlatform(ctx context.Context, companyID string, actor caapp.CompanyAuditContext, fn func(caapp.CompanySnapshot) (*caapp.CompanyMutation, error)) error {
	companyID = strings.TrimSpace(companyID)
	r.mu.Lock()
	c, ok := r.companies[companyID]
	if !ok {
		r.mu.Unlock()
		return perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "company not found", nil)
	}
	mutation, err := fn(snapshotOf(c))
	if err != nil || mutation == nil {
		r.mu.Unlock()
		return err
	}
	if r.failCompanyAudit && len(mutation.Audit) > 0 {
		r.failCompanyAudit = false
		r.mu.Unlock()
		return perr.NewHTTPError(http.StatusInternalServerError, perr.CodeInternal, "company audit insert failed", nil)
	}
	before := *c
	for _, rec := range mutation.Audit {
		r.companyAudit = append(r.companyAudit, CompanyAuditRow{CompanyID: companyID, Actor: actor, Record: rec})
	}
	r.mu.Unlock()

	if mutation.Update != nil {
		if err := r.UpdateCompanyPlatform(ctx, *mutation.Update); err != nil {
			r.restoreCompany(companyID, before, len(mutation.Audit))
			return err
		}
	}
	if mutation.Status != "" {
		if err := r.SetCompanyStatusPlatform(ctx, companyID, mutation.Status); err != nil {
			r.restoreCompany(companyID, before, len(mutation.Audit))
			return err
		}
	}
	r.mu.Lock()
	if cur, ok := r.companies[companyID]; ok {
		cur.UpdatedAtRFC3339 = time.Now().UTC().Format(time.RFC3339)
	}
	r.mu.Unlock()
	return nil
}

func (r *AdminRepository) restoreCompany(companyID string, before caapp.PlatformCompanyDetail, auditRows int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := before
	r.companies[companyID] = &cp
	if n := len(r.companyAudit) - auditRows; n >= 0 {
		r.companyAudit = r.companyAudit[:n]
	}
}

// CreateStandaloneCompanyAudited is not supported in memory (like CreateStandaloneCompany).
func (r *AdminRepository) CreateStandaloneCompanyAudited(ctx context.Context, displayName string, boot caapp.CreateCompanyBootstrap, _ caapp.CompanyAuditContext, _ func(companyID, companyCode string) caapp.CompanyAuditRecord) (string, string, error) {
	return r.CreateStandaloneCompany(ctx, displayName, boot)
}

// ListCompanyPrimaryAdminEmails returns login emails of active primary admins of the company.
func (r *AdminRepository) ListCompanyPrimaryAdminEmails(_ context.Context, companyID string) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0)
	for _, m := range r.memberships {
		if m.CompanyID != companyID || !m.IsPrimaryAdmin || !strings.EqualFold(strings.TrimSpace(m.Status), "active") {
			continue
		}
		if u, ok := r.users[m.UserID]; ok {
			email := strings.TrimSpace(u.Email)
			if email == "" {
				email = strings.TrimSpace(u.LoginID)
			}
			if strings.Contains(email, "@") {
				out = append(out, email)
			}
		}
	}
	return out, nil
}

// SetPrimaryAdmin flags a membership as primary admin (tests).
func (r *AdminRepository) SetPrimaryAdmin(membershipID string, primary bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if m, ok := r.memberships[membershipID]; ok {
		m.IsPrimaryAdmin = primary
		r.memberships[membershipID] = m
	}
}
