# CMS remove Deadline Rules — post-implementation verification (2026-09-23)

```text
task_type: post-implementation-verification + import-boundary lock
date: 2026-09-23
repos: cobo_iam_services, cobo_web_design
skills: integration-cross-repo, premerge-system-review, browser-testing-with-devtools
code_changed: true (import boundary tests + TemplatesFeatureScreen tab fallback test only in this cycle)
migration: none
commit: not performed
verdict: CONDITIONAL GO — deploy FE+BE to DEV required for full CMS/Import browser smoke
```

Related:

- `cms-remove-deadline-rules-implementation-2026-09-23.md` (implementation)
- `cms-remove-deadline-rules-ui-implementation-plan-2026-09-23.md` (plan)

---

## Phase 0 — Audit (no MISMATCH)

Checked diffs vs summary on both repos.

| Check | Result |
|-------|--------|
| Scope CMS periodic/import/Portal display only | PASS — no DisclosureTypeList / company-template / migration / authz in diff |
| Periodic save derives `T+{days}`, does not keep raw | PASS — `resolveDeadlineRuleForSave` + `EnsureCompatibilityDeadlineRule` |
| `resolved_deadline_rule` wins over compatibility | PASS — list+detail helpers |
| Irregular unchanged | PASS — derive helpers skip irregular; import still requires `deadline_rule` |
| Company-template unchanged | PASS — `CreateCompanyTemplate` does not call EnsureCompatibility |
| Catalog API/DB retained | PASS — routes + `DeadlineRuleCatalogScreen.tsx` + migrations stay; CMS tab only removed |
| No migration / auth change | PASS |

Catalog screen path (kept, not wired to tab): `cobo_web_design/src/features/cms-core/catalog/DeadlineRuleCatalogScreen.tsx`.

BE extras in same working tree (in-scope Portal semantics): `COMPANY_PROFILE_REQUIRED` + list/detail pending-profile DTO — aligns with controlled fallback PO.

---

## Phase 1 — Import boundary (locked)

### Policy implemented

| Kind | Behavior |
|------|----------|
| **Legacy** — no `applicability_rules` | Validate passes days-required check; keep raw `deadline_rule`; do **not** invent `deadline_days` |
| **New** — `applicability_rules` present | Periodic requires `deadline_days > 0` else `APPLICABILITY_DEADLINE_DAYS_REQUIRED` at `template.applicability_rules.deadline_days` |
| Raw vs days | Raw never invents days; days override raw → `T+{days}` in normalize |
| Irregular | No derive; `DEADLINE_RULE_REQUIRED` if empty |

### Tests added/strengthened (`deadline_rule_import_compat_test.go`)

1. days=20, no rule → PASS derive `T+20`
2. days=0 (+ raw T+99) → FAIL code + field path; no stack in message
3. days missing (=0) → same FAIL
4. raw `T+99` + days=20 → `T+20`
5. legacy omit rules + `T+20` → no days-required error
6. irregular omit rules → unchanged
7. irregular + applicability days → keeps free-text
8. validation DTO has no stack/goroutine strings

`go test` import/compat suite: **PASS**.

---

## Phase 2 — Browser smoke

### DEV (`http://88.216.208.0:3000`, API `:8080`)

| Flow | Result | Notes |
|------|--------|-------|
| Portal detail structure resolve | **PASS** | Fixture `qa-import-periodic-1788864118253`: “Trong vòng **30** ngày theo lịch” + căn cứ “Có công ty con” (not raw T+N) |
| CMS `/cms/templates` | **BLOCKED** | Session = company admin → `/app/forbidden` (no CMS permission) |
| CMS UI changes on DEV | **BLOCKED** | DEV FE asset `index-DWmk2Tu6.js` still contains label `Deadline Rules` (Last-Modified 2026-09-22). This branch **not deployed** (user forbade deploy) |
| Catalog API | **PASS (alive)** | `GET /api/v1/platform/cms/deadline-rules` → **401** SESSION_EXPIRED (not 404) |

### Local static bundle (`dist` after `npm run build`)

| Evidence | Result |
|----------|--------|
| Bundle has `Theo phạm vi áp dụng` + company-config fallback copy | PASS |
| Bundle has no UI label `Deadline Rules` (only legacy `deadline-rules` query fallback string) | PASS |
| Full CMS create/import in browser | **BLOCKED** | Local SPA redirects to login; no CMS+API stack with this branch without deploy |

### Vitest UI surrogate

- `TemplatesFeatureScreen.import.test.tsx`: `?tab=deadline-rules` → template list; no Deadline Rules button — **PASS**

---

## Phase 3 — Consumer audit (do not delete)

### KEEP

- BE column `deadline_rule`, matrix/`validatePortalDeadlineRule`, activate/archive/restore
- Catalog table + CRUD API + reference-data + `DeadlineRuleCatalogScreen` / hooks / cmsApi
- Portal `resolved_deadline_rule` enrichment + FE formatters
- Compatibility derive helpers + import schema optional `deadline_rule`
- Company-template / `DisclosureTypeList` modal `deadlineRule` field

### DEPRECATE LATER

- CMS tab wiring to catalog (already removed; screen orphaned)
- Admin free-text authoring of periodic `deadline_rule` (FE textarea removed; column still written as `T+N`)
- Showing compatibility `T+N` as Portal primary (suppressed when applicability-driven)

### OUT OF SCOPE

- Company-template modal / `CreateCompanyTemplate` free-text catalog contract
- Irregular FIXED_DATE / DYNAMIC_RULE authoring
- Dropping catalog DB / column / API

---

## Phase 4 — Precedence (verified, no fix needed)

```text
Periodic Portal:
  resolved (OK) → company-config / NO_RULE / COMPANY_PROFILE_REQUIRED fallback
  → raw only if non-compatibility free-text AND no controlled model
  → never show T+N as primary

Irregular:
  raw / event copy; no applicability derive

Company-template:
  unchanged
```

Sources: `DisclosureDeadlineSection.tsx`, `deadlineDisplayHelpers.ts`, `resolvedDeadlineRuleDisplay.ts`.

---

## Phase 5 — Full verification

| Command | Result | Classification |
|---------|--------|----------------|
| Focused FE vitest (deadline/CMS screens) | **108 PASS** | this change |
| `npm run lint` (`tsc --noEmit`) | exit **0** | pre-existing noise in unrelated `.test.tsx` still printed historically |
| `npm test` (full vitest) | **21 failed / 342 passed** | **pre-existing** (e.g. `adhoc-display.p0.test.tsx`); **none** in this diff’s feature files |
| `npm run build` | **PASS** | |
| `git diff --check` FE | **PASS** | |
| `go test ./internal/disclosure/...` | **PASS** | |
| `go build ./...` | **PASS** | |
| `docker compose -f docker-compose.dev.yml build api` | **PASS** | |
| `git diff --check` BE | **PASS** | |

---

## Phase 6 — Pre-merge system review

### Critical

- None in code for scoped PO. **Rollout gap:** FE/BE not on DEV → CMS/Import browser AC incomplete until deploy.

### Important

1. Soft import: omit `applicability_rules` still allows legacy raw-only periodic — intentional until PO hard-require + fixture migration.
2. `DefaultGlobalRules(periodic)` still has `DeadlineDays=0` — confirm omit path does not derive `T+N` until CMS/admin sets days.
3. Full `npm test` red (pre-existing adhoc/other) — do not treat as introduced by this change; track separately.
4. Catalog screen orphaned but reachable only if re-linked — OK for phase.

### Nice-to-have

- Wire catalog screen behind admin tooling or delete in later PO.
- Align `DefaultGlobalRules` with default `deadline_days=20` after fixture updates.
- Deploy smoke checklist for platform CMS user.

### Regression risks

- Dual derive FE+BE: keep in sync if days semantics change.
- Partial save paths that omit applicability but send stale `deadline_rule` — BE overwrites only when days>0.
- Company-template still authors free-text — intentional out of scope.

### Remaining PO decisions

1. Hard-require `deadline_days` for **all** periodic imports (including omit rules)?
2. When to drop catalog UI files / API / table?
3. Company-template modal alignment task schedule?
4. Approve DEV deploy of this branch for full browser AC?

### Go / No-go

**CONDITIONAL GO** for merge of code as implemented, **after**:

1. Human ack of soft legacy import boundary, and  
2. DEV deploy + CMS/Import smoke with platform CMS session (blocked in this task by no-deploy rule).

**NO-GO for production rollout** until Phase 2 CMS/Import browser AC is closed on DEV/staging.

---

## Files touched this verification cycle

- `internal/disclosure/app/deadline_rule_import_compat_test.go` — boundary cases
- `cobo_web_design/.../TemplatesFeatureScreen.import.test.tsx` — tab fallback
- this summary + `reusable-task-updates.md`

No commit / push / deploy.
