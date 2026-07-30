package periodic_oneshot

import (
	"fmt"
	"net/url"
	"strings"
)

// EnvGuard fail-closes outside approved DEV fingerprint.
type EnvGuard struct {
	Environment string
	Database    string
	HostAlias   string
	Port        string
}

func (e EnvGuard) Validate() error {
	if strings.TrimSpace(e.Environment) != "DEV" {
		return fmt.Errorf("environment must be DEV (got %q)", e.Environment)
	}
	if strings.TrimSpace(e.Database) != "cobo_iam" {
		return fmt.Errorf("database must be cobo_iam (got %q)", e.Database)
	}
	host := strings.TrimSpace(e.HostAlias)
	switch host {
	case "127.0.0.1", "localhost", "cobo-iam-mysql", "mysql":
		// approved DEV fingerprints (host-mapped MySQL or compose service name)
	default:
		return fmt.Errorf("host alias not approved for Controlled DEV (got %q)", e.HostAlias)
	}
	if strings.TrimSpace(e.Port) != "3306" {
		return fmt.Errorf("port must be 3306 (got %q)", e.Port)
	}
	return nil
}

// ParseDSNFingerprint extracts database/host/port without logging credentials.
func ParseDSNFingerprint(dsn string) (database, host, port string, err error) {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return "", "", "", fmt.Errorf("empty DSN")
	}
	// Go MySQL DSN: user:pass@tcp(host:port)/dbname?params
	at := strings.LastIndex(dsn, "@")
	if at < 0 {
		return "", "", "", fmt.Errorf("invalid DSN shape")
	}
	rest := dsn[at+1:]
	slash := strings.Index(rest, "/")
	if slash < 0 {
		return "", "", "", fmt.Errorf("invalid DSN: missing database")
	}
	addr := rest[:slash]
	dbPart := rest[slash+1:]
	if q := strings.Index(dbPart, "?"); q >= 0 {
		dbPart = dbPart[:q]
	}
	database = dbPart
	addr = strings.TrimPrefix(addr, "tcp(")
	addr = strings.TrimSuffix(addr, ")")
	host, port, err = splitHostPort(addr)
	if err != nil {
		return "", "", "", err
	}
	return database, host, port, nil
}

func splitHostPort(addr string) (string, string, error) {
	addr = strings.TrimSpace(addr)
	if strings.HasPrefix(addr, "[") {
		u, err := url.Parse("tcp://" + addr)
		if err != nil {
			return "", "", err
		}
		return u.Hostname(), u.Port(), nil
	}
	host, port, ok := strings.Cut(addr, ":")
	if !ok {
		return addr, "3306", nil
	}
	return host, port, nil
}
