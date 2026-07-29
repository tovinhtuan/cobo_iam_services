package inventory

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"regexp"
	"strings"
	"sync"
)

var leadingComment = regexp.MustCompile(`(?s)^\s*((/\*.*?\*/|--[^\n]*\n)\s*)*`)

// ValidateReadOnlySQL fails closed unless the single statement is allowlisted.
func ValidateReadOnlySQL(query string) error {
	q := strings.TrimSpace(query)
	q = leadingComment.ReplaceAllString(q, "")
	q = strings.TrimSpace(q)
	if q == "" {
		return fmt.Errorf("empty SQL rejected")
	}
	trimmed := strings.TrimRight(q, " \t\r\n;")
	if strings.Contains(trimmed, ";") {
		return fmt.Errorf("multi-statement SQL rejected")
	}
	upper := strings.ToUpper(trimmed)
	for _, bad := range []string{
		" FOR UPDATE", " FOR SHARE", " LOCK IN SHARE MODE",
		"INSERT ", "UPDATE ", "DELETE ", "REPLACE ", "UPSERT ",
		"ALTER ", "CREATE ", "DROP ", "TRUNCATE ", "LOCK TABLES",
		"UNLOCK TABLES", "CALL ", "INTO OUTFILE", "INTO DUMPFILE",
		"LOAD DATA", "GRANT ", "REVOKE ",
		"TEMPORARY ", "CREATE TEMPORARY",
	} {
		if strings.Contains(upper, bad) {
			return fmt.Errorf("forbidden SQL token: %s", strings.TrimSpace(bad))
		}
	}
	first := firstKeyword(upper)
	switch first {
	case "SELECT", "SHOW", "DESCRIBE", "DESC", "EXPLAIN":
		return nil
	case "SET":
		if strings.HasPrefix(upper, "SET SESSION TRANSACTION") ||
			strings.HasPrefix(upper, "SET TRANSACTION") ||
			strings.Contains(upper, "TRANSACTION READ ONLY") {
			return nil
		}
		return fmt.Errorf("SET statement not allowlisted for inventory")
	case "START", "BEGIN", "COMMIT", "ROLLBACK":
		return nil
	default:
		return fmt.Errorf("SQL verb not allowlisted: %s", first)
	}
}

func firstKeyword(upper string) string {
	fields := strings.Fields(upper)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

// QueryLogHook records validated query hashes (no params/secrets/legal text).
type QueryLogHook func(purpose, hash string)

// AllowlistConnector wraps a driver.Connector with SQL validation.
type AllowlistConnector struct {
	Parent driver.Connector
	Hook   QueryLogHook
	mu     sync.Mutex
	seen   int
}

func (c *AllowlistConnector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := c.Parent.Connect(ctx)
	if err != nil {
		return nil, err
	}
	return &allowlistConn{Conn: conn, hook: c.Hook, parent: c}, nil
}

func (c *AllowlistConnector) Driver() driver.Driver { return c.Parent.Driver() }

type allowlistConn struct {
	driver.Conn
	hook   QueryLogHook
	parent *AllowlistConnector
}

func (c *allowlistConn) validate(query string) error {
	if err := ValidateReadOnlySQL(query); err != nil {
		return err
	}
	if c.hook != nil {
		c.parent.mu.Lock()
		c.parent.seen++
		n := c.parent.seen
		c.parent.mu.Unlock()
		c.hook(fmt.Sprintf("q%d", n), fmt.Sprintf("%08x", hash32(query)))
	}
	return nil
}

func hash32(s string) uint32 {
	var h uint32 = 2166136261
	for i := 0; i < len(s); i++ {
		h ^= uint32(s[i])
		h *= 16777619
	}
	return h
}

func (c *allowlistConn) Prepare(query string) (driver.Stmt, error) {
	if err := c.validate(query); err != nil {
		return nil, err
	}
	return c.Conn.Prepare(query)
}

func (c *allowlistConn) Begin() (driver.Tx, error) { return c.Conn.Begin() }
func (c *allowlistConn) Close() error              { return c.Conn.Close() }

func (c *allowlistConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	if err := c.validate(query); err != nil {
		return nil, err
	}
	if pc, ok := c.Conn.(driver.ConnPrepareContext); ok {
		return pc.PrepareContext(ctx, query)
	}
	return c.Conn.Prepare(query)
}

func (c *allowlistConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if err := c.validate(query); err != nil {
		return nil, err
	}
	if ec, ok := c.Conn.(driver.ExecerContext); ok {
		return ec.ExecContext(ctx, query, args)
	}
	return nil, driver.ErrSkip
}

func (c *allowlistConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if err := c.validate(query); err != nil {
		return nil, err
	}
	if qc, ok := c.Conn.(driver.QueryerContext); ok {
		return qc.QueryContext(ctx, query, args)
	}
	return nil, driver.ErrSkip
}

func (c *allowlistConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	if b, ok := c.Conn.(driver.ConnBeginTx); ok {
		return b.BeginTx(ctx, opts)
	}
	return c.Conn.Begin()
}

func (c *allowlistConn) Ping(ctx context.Context) error {
	if p, ok := c.Conn.(driver.Pinger); ok {
		return p.Ping(ctx)
	}
	return nil
}

func (c *allowlistConn) ResetSession(ctx context.Context) error {
	if r, ok := c.Conn.(driver.SessionResetter); ok {
		return r.ResetSession(ctx)
	}
	return nil
}

func (c *allowlistConn) IsValid() bool {
	if v, ok := c.Conn.(driver.Validator); ok {
		return v.IsValid()
	}
	return true
}

func (c *allowlistConn) CheckNamedValue(nv *driver.NamedValue) error {
	if checker, ok := c.Conn.(driver.NamedValueChecker); ok {
		return checker.CheckNamedValue(nv)
	}
	return driver.ErrSkip
}

var mysqlConnector = func(dsn string) (driver.Connector, error) {
	return nil, fmt.Errorf("mysql connector not wired")
}

// SetMySQLConnector wires the MySQL driver connector factory from main.
func SetMySQLConnector(fn func(dsn string) (driver.Connector, error)) {
	mysqlConnector = fn
}

// OpenAllowlistedMySQL opens a DSN through the SQL allowlist connector.
func OpenAllowlistedMySQL(dsn string, hook QueryLogHook) (*sql.DB, error) {
	connector, err := mysqlConnector(dsn)
	if err != nil {
		return nil, err
	}
	db := sql.OpenDB(&AllowlistConnector{Parent: connector, Hook: hook})
	db.SetMaxOpenConns(2)
	return db, nil
}
