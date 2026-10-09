# CLAUDE.md - cobo_iam_services

@AGENTS.md

## Claude Code skills

Project skills live in `.claude/skills/<name>/SKILL.md` (Cursor copies remain
in `.cursor/skills/`; keep both in sync when editing shared skills).

- Start any non-trivial task with `cobo-task-workflow`; it selects the minimal
  skill chain. Do not load every skill.
- Skills tagged `[web]` in the workflow live in `../cobo_web_design/.claude/skills/`;
  read them directly for cross-repo work.
- The `[ai-cache]` answer marker's `skill:` field should name
  `.claude/skills/<name>` when running under Claude Code.
- Critical invariant: platform admin vs company admin - see `cobo-admin-role-guard`.

Baseline checks: `go test ./...`, `go vet ./...`,
`docker compose -f docker-compose.dev.yml build api` (or `BLOCKED: <reason>`).
New migrations must be appended to `migrations/run_dev_migrations.sh`.

### Workflows, agents, commands, hooks (Claude Code)

- Workflow skills: `wf-feature`, `wf-bugfix`, `wf-risk-review`, `wf-pr-review`,
  `wf-release` (phases + gates + artifacts in `docs/ai-cache/`).
- Commands: `/task`, `/feature`, `/bugfix`, `/risk-review`, `/pr-review`,
  `/release-check` (`.claude/commands/`).
- Read-only reviewer subagents (`.claude/agents/`): fe-security, be-security,
  admin-role, perf-reliability, cache-versioning, api-compat, secrets-cors.
  Launch independent reviewers in one message so they run in parallel.
- Hooks (`.claude/settings.json` → `.claude/hooks/cobo_*.py`): secret-file
  guard, ask-before push/deploy/destructive commands, post-edit checks
  (secrets, mojibake, gofmt, migration list), Stop reminder for ai-cache
  summary (`COBO_HOOK_STOP=0` disables it).
- Source of all of the above: `../claude-skills/`; reinstall with
  `bash ../claude-skills/install.sh`.
