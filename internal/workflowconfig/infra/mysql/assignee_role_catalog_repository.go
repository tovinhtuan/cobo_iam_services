package mysql

import (
	"context"
	"database/sql"
	"fmt"

	wfcapp "github.com/cobo/cobo_iam_services/internal/workflowconfig/app"
)

type AssigneeRoleCatalogRepository struct {
	db *sql.DB
}

func NewAssigneeRoleCatalogRepository(db *sql.DB) *AssigneeRoleCatalogRepository {
	return &AssigneeRoleCatalogRepository{db: db}
}

func (r *AssigneeRoleCatalogRepository) List(ctx context.Context) ([]wfcapp.AssigneeRoleCatalogItem, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT role_code, role_name, COALESCE(description, ''), is_system
		FROM workflow_assignee_role_catalog
		WHERE status = 'active'
		ORDER BY role_name ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list workflow assignee role catalog: %w", err)
	}
	defer rows.Close()
	out := make([]wfcapp.AssigneeRoleCatalogItem, 0)
	for rows.Next() {
		var item wfcapp.AssigneeRoleCatalogItem
		var isSystem int
		if err := rows.Scan(&item.RoleCode, &item.RoleName, &item.Description, &isSystem); err != nil {
			return nil, fmt.Errorf("scan workflow assignee role catalog: %w", err)
		}
		item.IsSystem = isSystem == 1
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *AssigneeRoleCatalogRepository) Create(ctx context.Context, item wfcapp.AssigneeRoleCatalogItem) (*wfcapp.AssigneeRoleCatalogItem, error) {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO workflow_assignee_role_catalog (role_code, role_name, description, status, is_system)
		VALUES (?, ?, ?, 'active', 0)
	`, item.RoleCode, item.RoleName, item.Description)
	if err != nil {
		return nil, fmt.Errorf("create workflow assignee role catalog: %w", err)
	}
	return r.getByCode(ctx, item.RoleCode)
}

func (r *AssigneeRoleCatalogRepository) getByCode(ctx context.Context, code string) (*wfcapp.AssigneeRoleCatalogItem, error) {
	var item wfcapp.AssigneeRoleCatalogItem
	var isSystem int
	err := r.db.QueryRowContext(ctx, `
		SELECT role_code, role_name, COALESCE(description, ''), is_system
		FROM workflow_assignee_role_catalog
		WHERE role_code = ?
	`, code).Scan(&item.RoleCode, &item.RoleName, &item.Description, &isSystem)
	if err != nil {
		return nil, fmt.Errorf("get workflow assignee role catalog: %w", err)
	}
	item.IsSystem = isSystem == 1
	return &item, nil
}

var _ wfcapp.AssigneeRoleCatalogRepository = (*AssigneeRoleCatalogRepository)(nil)
