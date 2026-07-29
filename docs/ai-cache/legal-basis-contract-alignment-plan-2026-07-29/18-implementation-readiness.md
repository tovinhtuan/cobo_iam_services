# 18 — Implementation readiness (post Phase 12.1A)

## Gate: Phase 12.2 may start when

| Prerequisite | Status |
| --- | --- |
| DTO locked | ✅ |
| read/write precedence locked | ✅ |
| projection locked | ✅ |
| divergence locked | ✅ |
| validation locked | ✅ |
| migration A–E locked | ✅ |
| OD-1…OD-7 LOCKED | ✅ |
| User confirmation | ✅ assumed for 12.2 execution |
| **Phase 12.2 implementation** | ✅ **PASS_WITH_NON_BLOCKING_GAPS** (`phase-12-2-backend-implementation-handoff.md`) |

## Phase dependency map

| Phase | Name | Depends on locked decisions | Ready? |
| --- | --- | --- | --- |
| **12.2** | Backend compatibility | OD-1,2,5,6,7 + read/write/divergence/validation | **DONE** |
| **12.3** | CMS structured editor | OD-1,5,6 + DTO + flags CMS | **DONE** (`PASS_WITH_NON_BLOCKING_GAPS`) |
| **12.4** | Tenant normalization | OD-2 + read precedence | **DONE** (`PASS_WITH_NON_BLOCKING_GAPS` — harness visual; stop before 12.5) |
| **12.5** | Lifecycle ID semantics | OD-3 + ID matrix | **DONE** (`PASS_WITH_NON_BLOCKING_GAPS` — new-version deep-copy+regen; clone/copy N/A) |
| **12.6A** | Inventory + dry-run (read-only) | OD-4 A–E | **CLOSED** — operational `PASS_READ_ONLY_DRY_RUN`; governance `FAIL_SCOPE_CREEP` (see `phase-12-6a-scope-exception.md`) |
| **12.6B-Plan** | Controlled DEV backfill design | 12.6A inventory VALID | **BACKFILL_PLAN_READY** (`phase-12-6b-plan-handoff.md`) — docs only; mutations=0 |
| **12.6B** | Backfill apply | Plan READY + Approvals 3–4 + explicit user phrase | **NOT STARTED** |
| **12.6** | Backfill | OD-4 + A–E matrix | YES after dry-run tooling (await user ACK) |
| **12.7** | DEV E2E | golden cases 16 | after 12.3–12.5 |
| **12.8** | Deprecation | fallback metric ~0 | later |

## Phase 12.4 notes

- Helper: `src/pages/portal/legalBasis/normalizeLegalBasesViewModel.ts`
- Wire: `normalizers.ts` delegates legal bases mapping
- UI: labels, date-only display, empty copy, safe links (`data:` blocked)
- Evidence: `phase-12-4-*`

## Deferred non-blocking (do not block 12.2)

| Item | Owner | Phase |
| --- | --- | --- |
| `LEGAL_BASIS_REQUIRE_ON_PUBLISH` | Product | future |
| `source_legal_basis_id` | BE | future |
| `is_legacy_projection` API field | FE/BE | never required; FE-only ok |
| Company create `legal_bases[]` request field | BE | 12.3+ / with company API extend |
| Soft-warn same-code BE enforce | BA | optional post-MVP |
| Projection include code/link | Product | only with new ADR + golden tests |
| Clone/version ID regen hardening | BE | **12.5** |
| Top-level `VALIDATION_ERROR` code alias | BE | optional; currently `INVALID_REQUEST` + field_errors |

## Scope reminder

Phase 12.2: backend only. No CMS/tenant UI, no migration, no DEV deploy.
