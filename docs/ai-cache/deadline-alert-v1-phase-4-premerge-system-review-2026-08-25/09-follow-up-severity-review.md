# 09 — Follow-up severity review

| Item | Severity | Release blocker? | Next |
|------|----------|------------------|------|
| confirm-on-Draft (DONE ack while still Draft obligation) | P1 | No | UX/product clarification phase |
| UNIQUE(periodic_cycles.record_id) | P1 | No | data-integrity migration phase |
| DONE enum debt (ack vs obligation) | P2 | No | enum redesign later |
| reopen/return lifecycle | NOT_IN_SCOPE | No | future domain |
| index / EXPLAIN at scale (`LOWER(TRIM(status))`) | P1 | No | performance optimization phase |
| QA cleanup qa-dav1-20260825 | OPERATIONAL | No | housekeeping |
| list card click→detail polish | P2 | No | FE UX (detail route already works) |
| helper queries still `status<>draft` for enrichment | P2 | No | optional enrichment follow-up |
| FE leftover `_tmp_dav1_browser.mjs` | P2 | hygiene | delete before FE docs merge |
| Historical commits include binaries | P1 hygiene | merge hygiene | clean PR/squash excluding artifacts |

```text
OPEN_P0=0
OPEN_P1=4
OPEN_P2=4
RETURN_REOPEN_SUPPORTED=false
```
