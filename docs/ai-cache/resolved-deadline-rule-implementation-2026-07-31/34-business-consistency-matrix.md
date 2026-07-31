# Business consistency matrix

| Case | Toggle | Flags (sub/units) | Source | Code | Days | Day type | Base | Portal sentence / context | Result |
|------|--------|-------------------|--------|------|------|----------|------|---------------------------|--------|
| C1 | off | T/T | DEFAULT_TEMPLATE_RULE | DEFAULT | 23 | WORKING | CYCLE_START | 23 WD + default context; **no** criterion labels | **PASS** |
| C2 | on | T/F | STRUCTURE_OVERRIDE | has_subsidiaries | 23 | WORKING | CYCLE_START | override + Có công ty con | **PASS** |
| C3 | on | F/T | STRUCTURE_OVERRIDE | has_subordinate_units | 30 | WORKING | CYCLE_START | override + đơn vị KT | **PASS** |
| C4 | on | T/T | STRUCTURE_OVERRIDE | has_subsidiaries | 23 | WORKING | CYCLE_START | subsidiaries wins; no subordinate label | **PASS** |
| C5 | on | F/F | STRUCTURE_OVERRIDE | simple_structure | 15 | WORKING | CYCLE_START | simple label | **PASS** |
| C6 | on | T/F map miss | STRUCTURE_FALLBACK_DEFAULT | DEFAULT | 23 | WORKING | CYCLE_START | fallback copy ≠ toggle-off default | **PASS** |
| C7 | on | — / days 0 | NO_RULE | — | — | WORKING | CYCLE_START | no-rule copy; no “0 ngày” | **PASS** |
| C8 | off | — | DEFAULT… | DEFAULT | 10 | WORKING | CYCLE_START | “ngày làm việc” | **PASS** |
| C9 | off | — | DEFAULT… | DEFAULT | 10 | CALENDAR | CYCLE_START | “ngày theo lịch” | **PASS** |
| C10 | — | — | — | — | — | — | CYCLE_START | forbids period-end wording in formatter | **PASS** |
| C11 | — | — | — | — | — | — | — | due `2026-07-31` → `31/07/2026` | **PASS** |
| C12 | — | — | — | — | — | — | — | due absent → no row | **PASS** |
| C13 | DTO absent | — | — | — | — | — | — | legacy CMS text only; no FE flag match; DTO present wins over legacy bait | **PASS*** |
| C14 | unknown enums | — | — | — | — | — | — | safe labels / unavailable; no raw code | **PASS** |

\*C13 residual: legacy path passthrough CMS free-text (`deadline_rule`); FE does not invent criterion from company flags. Historical CMS copy quality is out of Phase 4 fix scope (no feature expansion without approval).
