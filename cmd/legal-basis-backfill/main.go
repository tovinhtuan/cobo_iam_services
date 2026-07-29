package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	backfill "github.com/cobo/cobo_iam_services/internal/disclosure/app/legal_basis_backfill"
)

// Controlled DEV backfill CLI. Default is --mode=plan (no mutation).
// --apply is implemented for future Phase 12.6B-E and still requires guards;
// this package's MemoryStore path is for tests. SQL DEV wiring is intentionally
// not connected here — refuse --apply without --store=memory for Phase 12.6B-I safety.
func main() {
	mode := flag.String("mode", "plan", "plan|apply")
	envName := flag.String("environment", "", "must be DEV")
	database := flag.String("database", "", "must be cobo_iam")
	host := flag.String("host", "127.0.0.1", "approved host alias")
	port := flag.String("port", "3306", "approved port")
	allowlistPath := flag.String("allowlist", "", "path to exact allowlist json")
	snapshotPath := flag.String("snapshot", "", "secure snapshot path (apply)")
	confirmToken := flag.String("confirm-token", "", "one-time confirm token (never logged)")
	expected := flag.Int("expected-records", 6, "must be 6")
	apply := flag.Bool("apply", false, "explicit mutation switch (also requires mode=apply)")
	storeKind := flag.String("store", "", "memory fixture only in 12.6B-I; sql not wired")
	fixture := flag.String("fixture-json", "", "memory store seed json for synthetic apply tests")
	flag.Parse()

	if *expected != backfill.ExpectedRecords {
		fatalf("expected-records must be %d", backfill.ExpectedRecords)
	}
	if *allowlistPath == "" {
		fatalf("--allowlist required")
	}
	al, err := backfill.LoadAllowlist(*allowlistPath)
	if err != nil {
		fatalf("allowlist: %v", err)
	}
	env := backfill.EnvGuard{Environment: *envName, Database: *database, HostAlias: *host, Port: *port}
	if err := env.Validate(); err != nil {
		fatalf("env: %v", err)
	}

	wantApply := *apply || strings.EqualFold(*mode, "apply")
	if wantApply {
		if err := backfill.ConfirmTokenOK(*confirmToken); err != nil {
			fatalf("confirm: %v", err)
		}
		if *snapshotPath == "" {
			fatalf("--snapshot required for apply")
		}
		if *storeKind != "memory" {
			fatalf("Phase 12.6B-I refuses non-memory store apply (no DEV write wiring)")
		}
		if *fixture == "" {
			fatalf("--fixture-json required for memory apply")
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if !wantApply {
		// plan mode does not need a store mutation backend — use empty memory for structure
		eng := &backfill.Engine{Env: env, Store: backfill.NewMemoryStore(nil)}
		// Without rows, freshness fails — plan against fixture if provided
		if *fixture != "" {
			rows, err := loadFixture(*fixture)
			if err != nil {
				fatalf("fixture: %v", err)
			}
			eng.Store = backfill.NewMemoryStore(rows)
		} else {
			fmt.Fprintf(os.Stderr, "plan without fixture: emitting allowlist/env gate only\n")
			out := map[string]any{
				"mode": "plan", "environment": env.Environment, "database": env.Database,
				"allowlistCount": len(al.Records), "allowlistChecksum": al.FileChecksum,
				"freshness": "NOT_CHECKED_NO_FIXTURE", "mutations": 0,
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			_ = enc.Encode(out)
			return
		}
		rep, err := eng.Plan(ctx, al)
		if err != nil {
			_ = json.NewEncoder(os.Stdout).Encode(rep)
			fatalf("plan: %v", err)
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(rep)
		return
	}

	rows, err := loadFixture(*fixture)
	if err != nil {
		fatalf("fixture: %v", err)
	}
	snap, err := backfill.LoadSnapshot(*snapshotPath, al)
	if err != nil {
		fatalf("snapshot: %v", err)
	}
	eng := &backfill.Engine{Env: env, Store: backfill.NewMemoryStore(rows)}
	rep, err := eng.Apply(ctx, al, snap)
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(rep)
	if err != nil {
		os.Exit(2)
	}
}

func loadFixture(path string) ([]backfill.Row, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var rows []backfill.Row
	if err := json.Unmarshal(b, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

func fatalf(f string, a ...any) {
	fmt.Fprintf(os.Stderr, f+"\n", a...)
	os.Exit(2)
}
