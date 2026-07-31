# Phase 2 — Backend design

## Approach
Single production resolution path: `ResolveDeadlineRule` owns branches; `ResolveDeadlineDays` returns `(days, ok)` from it — **no second resolver**.

## Layers
| Layer | Responsibility |
|-------|----------------|
| `applicability.ResolveDeadlineRule` | Semantic code + source + days |
| `app.buildResolvedDeadlineRuleDTO` | Map to transport DTO + day_type + CYCLE_START + label key |
| `app.GetTypeDetail` | Company profile once; attach DTO; attach due_date from summary |
| `contracts.ResolvedDeadlineRuleDTO` | Additive JSON field |

## Constants
- Rule codes: existing `StructureCriterion` + `RuleCodeDefault`
- Sources: `DEFAULT_TEMPLATE_RULE`, `STRUCTURE_OVERRIDE`, `STRUCTURE_FALLBACK_DEFAULT`, `NO_RULE`
- Base: `BaseDateSourceCycleStart` (`CYCLE_START`)

## Non-goals
No calculator change, no precedence change, no migration, no FE, no worker rewrite.
