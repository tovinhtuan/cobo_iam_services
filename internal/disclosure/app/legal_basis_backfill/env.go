package backfill

import (
	"fmt"
	"strings"
)

// EnvGuard is the fail-closed environment fingerprint for Controlled DEV execution.
type EnvGuard struct {
	Environment string // must be DEV
	Database    string // must be cobo_iam
	HostAlias   string // approved: 127.0.0.1
	Port        string // approved: 3306
}

func (e EnvGuard) Validate() error {
	if strings.TrimSpace(e.Environment) != "DEV" {
		return fmt.Errorf("environment must be DEV (got %q)", e.Environment)
	}
	if strings.TrimSpace(e.Database) != "cobo_iam" {
		return fmt.Errorf("database must be cobo_iam (got %q)", e.Database)
	}
	host := strings.TrimSpace(e.HostAlias)
	if host != "127.0.0.1" && host != "localhost" {
		return fmt.Errorf("host alias not approved for Controlled DEV (got %q)", e.HostAlias)
	}
	if strings.TrimSpace(e.Port) != "3306" {
		return fmt.Errorf("port must be 3306 (got %q)", e.Port)
	}
	return nil
}

// ConfirmTokenOK rejects empty/default tokens. Token itself must never be logged.
func ConfirmTokenOK(token string) error {
	t := strings.TrimSpace(token)
	if t == "" {
		return fmt.Errorf("confirm token required")
	}
	if t == "default" || t == "TODO" || t == "changeme" {
		return fmt.Errorf("confirm token rejected")
	}
	if len(t) < 16 {
		return fmt.Errorf("confirm token too short")
	}
	return nil
}
