package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	backfill "github.com/cobo/cobo_iam_services/internal/disclosure/app/legal_basis_backfill"
)

func main() {
	envName := flag.String("environment", "DEV", "")
	database := flag.String("database", "cobo_iam", "")
	host := flag.String("host", "127.0.0.1", "")
	port := flag.String("port", "3306", "")
	allowlistPath := flag.String("allowlist", "", "")
	fixture := flag.String("fixture-json", "", "memory rows (Phase 12.6B-I)")
	flag.Parse()
	if *allowlistPath == "" || *fixture == "" {
		fmt.Fprintln(os.Stderr, "--allowlist and --fixture-json required (no DEV wiring in 12.6B-I)")
		os.Exit(2)
	}
	al, err := backfill.LoadAllowlist(*allowlistPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	b, err := os.ReadFile(*fixture)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	var rows []backfill.Row
	if err := json.Unmarshal(b, &rows); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	eng := &backfill.Engine{
		Env:   backfill.EnvGuard{Environment: *envName, Database: *database, HostAlias: *host, Port: *port},
		Store: backfill.NewMemoryStore(rows),
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	rep, err := eng.Verify(ctx, al)
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(rep)
	if err != nil || rep.Status != "PASS" {
		os.Exit(2)
	}
}
