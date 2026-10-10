package mysql

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"database/sql/driver"
	"encoding/hex"
	"fmt"
	"net/http"
	"sync"
	"time"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

const companyAdminLockWaitSeconds = 5

// companyAdminLockName stays within MySQL's 64-character lock name limit.
func companyAdminLockName(companyID string) string {
	name := "cobo_iam:company_admins:" + companyID
	if len(name) <= 64 {
		return name
	}
	sum := sha256.Sum256([]byte(companyID))
	return "cobo_iam:company_admins:" + hex.EncodeToString(sum[:])[:40]
}

// LockCompanyAdmins takes a MySQL named lock on a dedicated connection, so it serializes the
// changes of every API instance. The checks and writes run on other connections while it is held;
// they autocommit, so the next holder reads what the previous one wrote.
func (r *AdminRepository) LockCompanyAdmins(ctx context.Context, companyID string) (func(), error) {
	conn, err := r.db.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("lock company admins: %w", err)
	}
	name := companyAdminLockName(companyID)
	var got sql.NullInt64
	if err := conn.QueryRowContext(ctx, `SELECT GET_LOCK(?, ?)`, name, companyAdminLockWaitSeconds).Scan(&got); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("lock company admins: %w", err)
	}
	if !got.Valid || got.Int64 != 1 {
		_ = conn.Close()
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict,
			"another admin change for this company is in progress; retry", nil)
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			rctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
			defer cancel()
			if _, err := conn.ExecContext(rctx, `DO RELEASE_LOCK(?)`, name); err != nil {
				// Never return a session that may still hold the lock to the pool: closing the
				// session releases it.
				_ = conn.Raw(func(any) error { return driver.ErrBadConn })
			}
			_ = conn.Close()
		})
	}, nil
}
