# Diff scope audit

## Allowed (included)

- `internal/disclosure/app/periodic_oneshot/**`
- `cmd/periodic-materialize-one/**`
- Repo cycle helpers + calculator export
- Evidence docs under `docs/ai-cache/guarded-periodic-one-shot-materialization-2026-07-30/`

## Forbidden (not included in this feature commit)

- PERIODIC_SEEDING_ENABLED flip
- FE source changes
- Migration
- Unrelated workflow 404 / legal-basis / compose flag churn (left unstaged)
