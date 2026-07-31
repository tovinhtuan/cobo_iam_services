# Phase 2 — Backend implementation

## Changed files (TASK_SCOPE)
| File | Change |
|------|--------|
| `internal/disclosure/app/applicability/types.go` | Rule/source constants |
| `internal/disclosure/app/applicability/rules.go` | `ResolveDeadlineRule`, label key; `ResolveDeadlineDays` wraps it |
| `internal/disclosure/app/applicability/rules_deadline_rule_test.go` | Matrix A–H unit tests |
| `internal/disclosure/app/contracts.go` | `ResolvedDeadlineRuleDTO` + field on `DisclosureTypeDTO` |
| `internal/disclosure/app/deadline_calculator.go` | `BaseDateSourceCycleStart` const only |
| `internal/disclosure/app/resolved_deadline_rule.go` | DTO builder + due_date attach |
| `internal/disclosure/app/resolved_deadline_rule_test.go` | GetTypeDetail integration cases |
| `internal/disclosure/app/service.go` | Wire into GetTypeDetail |

## Unchanged
Calculator body, ResolveStructure precedence, worker/oneshot logic (call sites unchanged), migrations, FE.
