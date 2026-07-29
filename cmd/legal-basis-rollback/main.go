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
	envName := flag.String("environment", "", "DEV")
	database := flag.String("database", "", "cobo_iam")
	host := flag.String("host", "127.0.0.1", "")
	port := flag.String("port", "3306", "")
	allowlistPath := flag.String("allowlist", "", "")
	snapshotPath := flag.String("snapshot", "", "")
	confirmToken := flag.String("confirm-token", "", "")
	apply := flag.Bool("apply", false, "required for mutation")
	fixture := flag.String("fixture-json", "", "current rows (memory)")
	postState := flag.String("post-state-json", "", "expected post-backfill flat/json by record_id")
	flag.Parse()
	if !*apply {
		fmt.Fprintln(os.Stderr, "default refuse: pass --apply explicitly (still memory-only in 12.6B-I)")
		os.Exit(2)
	}
	if err := backfill.ConfirmTokenOK(*confirmToken); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	env := backfill.EnvGuard{Environment: *envName, Database: *database, HostAlias: *host, Port: *port}
	if err := env.Validate(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if *allowlistPath == "" || *snapshotPath == "" || *fixture == "" || *postState == "" {
		fmt.Fprintln(os.Stderr, "required: allowlist snapshot fixture post-state")
		os.Exit(2)
	}
	al, err := backfill.LoadAllowlist(*allowlistPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	snap, err := backfill.LoadSnapshot(*snapshotPath, al)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	var rows []backfill.Row
	b, _ := os.ReadFile(*fixture)
	if err := json.Unmarshal(b, &rows); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	var post struct {
		Flat map[string]string          `json:"flat"`
		JSON map[string]json.RawMessage `json:"json"`
	}
	pb, _ := os.ReadFile(*postState)
	if err := json.Unmarshal(pb, &post); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	postJSON := map[string][]byte{}
	for k, v := range post.JSON {
		postJSON[k] = []byte(v)
	}
	eng := &backfill.Engine{Env: env, Store: backfill.NewMemoryStore(rows)}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	rep, err := eng.Rollback(ctx, al, snap, post.Flat, postJSON)
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(rep)
	if err != nil {
		os.Exit(2)
	}
}
