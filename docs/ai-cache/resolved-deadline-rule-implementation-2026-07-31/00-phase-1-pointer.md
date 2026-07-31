# Resolved deadline rule — Phase 1 note (BE mirror)

Canonical evidence lives in sibling FE repo:

`../cobo_web_design/docs/ai-cache/resolved-deadline-rule-implementation-2026-07-31/`

## Lock summary
- Option A additive `resolved_deadline_rule` on `GET /api/v1/disclosure-types/{type_id}`
- Reuse `ResolveStructure` + `ResolveDeadlineDays` + existing calculator
- rule_code: `DEFAULT` | `has_subsidiaries` | `has_subordinate_units` | `simple_structure`
- resolution_source: `DEFAULT_TEMPLATE_RULE` | `STRUCTURE_OVERRIDE` | `STRUCTURE_FALLBACK_DEFAULT` | `NO_RULE`
- base: `CYCLE_START` (runtime cycleStart; no calculator change)
- Baseline BE HEAD at Phase 1: `661413d`

## Verdict
`PHASE_1_CONTRACT_READY` — awaiting user confirmation before Phase 2 backend implementation.
