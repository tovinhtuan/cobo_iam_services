package mysql

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"

	_ "github.com/go-sql-driver/mysql"
)

// MR-3: ListMineOpenTasks reads the template's archive state (LEFT JOIN disclosure_types). Runs the
// real query against a migrated MySQL 8.0 so a column or syntax slip fails here, not on DEV:
//
//	MYSQL_TEST_DSN='<user>:<pass>@tcp(127.0.0.1:3306)/cobo_iam?parseTime=true&loc=UTC' go test ./internal/personalops/infra/mysql/ -run OpenTasksQuery
func TestOpenTasksQuery_runsOnMySQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("MYSQL_TEST_DSN"))
	if dsn == "" {
		t.Skip("MYSQL_TEST_DSN not set")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	tasks, err := NewRepository(db).ListMineOpenTasks(context.Background(), []string{"m-mr3-none"}, 5)
	if err != nil {
		t.Fatalf("ListMineOpenTasks: %v", err)
	}
	if len(tasks) != 0 {
		t.Fatalf("tasks=%v for an unknown membership", tasks)
	}
}
