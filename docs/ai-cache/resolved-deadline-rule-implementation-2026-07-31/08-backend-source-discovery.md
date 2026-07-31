# Phase 2 — Backend source discovery

## Baseline
- Branch: `recovery/lost-changes-audit-20260717-153324`
- HEAD before implement: `661413d`
- Dirty at start: Phase 1 docs only (`PHASE_1_DOCS`) — **CLEAN enough** (not BLOCKED_DIRTY_BASELINE)

## Exact helpers
| Helper | Package | File |
|--------|---------|------|
| `ResolveStructure` | `applicability` | `internal/disclosure/app/applicability/rules.go` |
| `ResolveDeadlineDays` | `applicability` | same — now delegates to `ResolveDeadlineRule` |
| `ResolveDeadlineRule` | `applicability` | **added** — same branches as former ResolveDeadlineDays + semantic source |
| `ResolveDeadlineDurationType` | `applicability` | same |
| `resolveEffectiveN` | `deadlineengine` | `resolve_n.go` (V2 twin; unchanged) |
| `calculatePeriodic` | `app` | `deadline_calculator.go` — cycleStart + N (unchanged) |
| `GetTypeDetail` | `app` | `service.go` |

## Call chain (actual after Phase 2)
```text
GET /api/v1/disclosure-types/{type_id}
→ handler.getTypeDetail
→ service.GetTypeDetail(Subject.CompanyID)
→ repo.GetTypeDetail
→ repo.GetCompanyApplicabilityProfile(companyID)
→ buildResolvedDeadlineRuleDTO
     → applicability.ResolveDeadlineRule (uses ResolveStructure)
     → ResolveDeadlineDurationType
     → BaseDateSourceCycleStart if PERIODIC
→ [existing] ResolveDeadlineDays for DeadlineConfig override
→ CalculateDeadlineSummary / calculatePeriodic
→ attachResolvedDueDate from deadline_summary.deadline_date
```

## Fallback (production — confirmed)
Toggle on + map miss/invalid days + `deadline_days>0` → days = deadline_days (now labeled `STRUCTURE_FALLBACK_DEFAULT`).

## Worker / one-shot
Still call `ResolveDeadlineDays` — behavior preserved via thin wrapper over `ResolveDeadlineRule`.
