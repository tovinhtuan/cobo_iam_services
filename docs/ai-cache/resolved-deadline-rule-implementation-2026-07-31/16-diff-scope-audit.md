# Diff scope audit

## TASK_SCOPE
- `internal/disclosure/app/applicability/{types,rules}.go`
- `internal/disclosure/app/applicability/rules_deadline_rule_test.go`
- `internal/disclosure/app/{contracts,deadline_calculator,service,resolved_deadline_rule}.go`
- `internal/disclosure/app/resolved_deadline_rule_test.go`
- `docs/ai-cache/resolved-deadline-rule-implementation-2026-07-31/*`
- `docs/ai-cache/reusable-task-updates.md`

## PHASE_1_DOCS
- `docs/ai-cache/resolved-deadline-rule-implementation-2026-07-31/00-phase-1-pointer.md` (pre-existing untracked)

## PRE_EXISTING_UNRELATED
- none in working tree at commit time (notification test fail is in tree history, not a dirty file)

## Forbidden (verified absent)
- FE app source
- migrations/
- worker semantic changes
- calculator algorithm changes
- deploy scripts
