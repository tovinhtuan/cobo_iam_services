package mysql

import (
	"context"
	"database/sql"
)

// queryer is the part of *sql.DB and *sql.Tx the read helpers need, so the same query code runs
// on the pool or inside a transaction.
type queryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}
