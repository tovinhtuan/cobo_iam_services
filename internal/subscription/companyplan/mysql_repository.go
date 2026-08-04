package companyplan

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

type mysqlRepository struct {
	db *sql.DB
}

// NewMySQLRepository constructs a Case C company_subscriptions repository.
func NewMySQLRepository(db *sql.DB) Repository {
	return &mysqlRepository{db: db}
}

func (r *mysqlRepository) GetEffectivePlan(ctx context.Context, companyID string, at time.Time) (*CompanyPlan, error) {
	companyID = strings.TrimSpace(companyID)
	if companyID == "" {
		return nil, nil
	}
	rows, err := r.listCovering(ctx, r.db, []string{companyID}, at)
	if err != nil {
		return nil, err
	}
	return SelectEffectivePlan(rows, at), nil
}

func (r *mysqlRepository) GetEffectivePlans(ctx context.Context, companyIDs []string, at time.Time) (map[string]*CompanyPlan, error) {
	out := make(map[string]*CompanyPlan, len(companyIDs))
	ids := uniqueNonEmpty(companyIDs)
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := r.listCovering(ctx, r.db, ids, at)
	if err != nil {
		return nil, err
	}
	byCompany := map[string][]CompanyPlan{}
	for _, row := range rows {
		byCompany[row.CompanyID] = append(byCompany[row.CompanyID], row)
	}
	for _, id := range ids {
		if p := SelectEffectivePlan(byCompany[id], at); p != nil {
			out[id] = p
		}
	}
	return out, nil
}

func (r *mysqlRepository) ListOccupyingByCompany(ctx context.Context, companyID string) ([]CompanyPlan, error) {
	companyID = strings.TrimSpace(companyID)
	if companyID == "" {
		return nil, nil
	}
	q := `
		SELECT id, company_id, plan_code, status, effective_from, expires_at, origin, created_at, updated_at
		FROM company_subscriptions
		WHERE company_id = ?
		  AND status IN ('ACTIVE', 'TRIAL', 'SUSPENDED')
		ORDER BY effective_from ASC, id ASC`
	rows, err := r.db.QueryContext(ctx, q, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPlans(rows)
}

func (r *mysqlRepository) Create(ctx context.Context, plan CompanyPlan) error {
	plan = NormalizeUTC(plan)
	if err := ValidateCreate(plan); err != nil {
		return err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// Serialize writers per company: lock occupying rows (and a sentinel scan).
	lockQ := `
		SELECT id FROM company_subscriptions
		WHERE company_id = ?
		  AND status IN ('ACTIVE', 'TRIAL', 'SUSPENDED')
		ORDER BY id
		FOR UPDATE`
	lockRows, err := tx.QueryContext(ctx, lockQ, plan.CompanyID)
	if err != nil {
		return err
	}
	_ = lockRows.Close()

	existing, err := listOccupyingTx(ctx, tx, plan.CompanyID)
	if err != nil {
		return err
	}
	if OccupyingOverlap(existing, plan, "") {
		return ErrOverlap
	}

	now := NowUTC()
	if plan.CreatedAt.IsZero() {
		plan.CreatedAt = now
	}
	if plan.UpdatedAt.IsZero() {
		plan.UpdatedAt = now
	}
	var expires any
	if plan.ExpiresAt != nil {
		expires = *plan.ExpiresAt
	} else {
		expires = nil
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO company_subscriptions (
			id, company_id, plan_code, status, effective_from, expires_at, origin, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		plan.ID, plan.CompanyID, string(plan.Code), string(plan.Status),
		plan.EffectiveFrom, expires, string(plan.Origin), plan.CreatedAt, plan.UpdatedAt,
	)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (r *mysqlRepository) DeleteByIDs(ctx context.Context, ids []string) error {
	ids = uniqueNonEmpty(ids)
	if len(ids) == 0 {
		return nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	q := fmt.Sprintf(`DELETE FROM company_subscriptions WHERE id IN (%s)`, strings.Join(placeholders, ","))
	_, err := r.db.ExecContext(ctx, q, args...)
	return err
}

func (r *mysqlRepository) DeleteByOrigin(ctx context.Context, origin RecordOrigin) error {
	if strings.TrimSpace(string(origin)) == "" {
		return ErrInvalidPlan
	}
	_, err := r.db.ExecContext(ctx, `DELETE FROM company_subscriptions WHERE origin = ?`, string(origin))
	return err
}

func (r *mysqlRepository) listCovering(ctx context.Context, db queryable, companyIDs []string, at time.Time) ([]CompanyPlan, error) {
	at = at.UTC()
	placeholders := make([]string, len(companyIDs))
	args := make([]any, 0, len(companyIDs)+1)
	for i, id := range companyIDs {
		placeholders[i] = "?"
		args = append(args, id)
	}
	args = append(args, at, at)
	q := fmt.Sprintf(`
		SELECT id, company_id, plan_code, status, effective_from, expires_at, origin, created_at, updated_at
		FROM company_subscriptions
		WHERE company_id IN (%s)
		  AND effective_from <= ?
		  AND (expires_at IS NULL OR expires_at > ?)
		ORDER BY company_id ASC, effective_from DESC, id DESC`, strings.Join(placeholders, ","))
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPlans(rows)
}

type queryable interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

func listOccupyingTx(ctx context.Context, tx *sql.Tx, companyID string) ([]CompanyPlan, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT id, company_id, plan_code, status, effective_from, expires_at, origin, created_at, updated_at
		FROM company_subscriptions
		WHERE company_id = ?
		  AND status IN ('ACTIVE', 'TRIAL', 'SUSPENDED')
		ORDER BY effective_from ASC, id ASC`, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPlans(rows)
}

func scanPlans(rows *sql.Rows) ([]CompanyPlan, error) {
	var out []CompanyPlan
	for rows.Next() {
		var p CompanyPlan
		var code, status, origin string
		var expires sql.NullTime
		if err := rows.Scan(
			&p.ID, &p.CompanyID, &code, &status, &p.EffectiveFrom, &expires, &origin, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, err
		}
		p.Code = PlanCode(code)
		p.Status = PlanStatus(status)
		p.Origin = RecordOrigin(origin)
		if expires.Valid {
			t := expires.Time.UTC()
			p.ExpiresAt = &t
		}
		p.EffectiveFrom = p.EffectiveFrom.UTC()
		p.CreatedAt = p.CreatedAt.UTC()
		p.UpdatedAt = p.UpdatedAt.UTC()
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func uniqueNonEmpty(ids []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

// Ensure compile-time interface satisfaction.
var (
	_ Repository = (*mysqlRepository)(nil)
	_            = errors.Is
)
