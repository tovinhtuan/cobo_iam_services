# CMS remove Deadline Rules — implementation summary (2026-09-23)

```text
task_type: implementation
date: 2026-09-23
repos: cobo_iam_services, cobo_web_design
skills: integration-cross-repo, backend-api-contract, premerge-system-review
code_changed: true
migration: none
commit: not performed
verdict: READY FOR USER REVIEW (no commit/push)
```

## PO policy locked

- Periodic SoT: `applicability_rules.deadline_days` (+ company profile for Portal resolve).
- CMS does not author `deadline_rule`; BE/FE derive compatibility `T+{deadline_days}`.
- Portal: `resolved_deadline_rule` wins; do not show compatibility `T+N` as primary; missing profile → controlled fallback copy.
- Scope: CMS templates feature only (not DisclosureTypeList / company-template modal).
- Keep DB column, catalog API, catalog helpers.

## Slices delivered

### Slice 1 — CMS UI
- Removed Deadline Rules tab from `TemplatesFeatureScreen`; `?tab=deadline-rules` falls back to templates list.
- Removed periodic textarea from `TemplateEditorScreen`; copy points to Phạm vi áp dụng.
- CMS list shows `Theo phạm vi áp dụng` for periodic (no inventing days from list API).

### Slice 2 — FE save
- `templateValidation`: periodic no longer requires free-text `deadlineRule`; requires `applicabilityRules.deadlineDays > 0`.
- `resolveDeadlineRuleForSave`: always `T+{days}` for periodic; irregular unchanged.

### Slice 3 — Portal fallback
- Compatibility `T+N` not used as Portal primary when applicability-driven.
- Controlled fallback: “Thời hạn được áp dụng theo cấu hình doanh nghiệp.”
- Legacy free-text only when no applicability-driven display model.

### Slice 4 — BE compatibility
- `EnsureCompatibilityDeadlineRule` on CMS upsert before matrix validation.
- Structure override does not change compatibility string (still default days).

### Slice 5 — Import
- Schema: `deadline_rule` optional.
- Normalize calls `DeriveImportCompatibilityDeadlineRule` before domain validation.
- Periodic + present `applicability_rules` with `deadline_days <= 0` → `APPLICABILITY_DEADLINE_DAYS_REQUIRED`.
- Periodic + omitted rules: still allowed at validate (confirm injects `DefaultGlobalRules`); does **not** invent days from raw `deadline_rule`.
- Irregular: still requires `deadline_rule`; no derive from applicability.

## Boundary note (soft import rule)

Strict “every periodic import must have deadline_days” would break existing fixtures that omit `applicability_rules` and only send `deadline_rule`. Implemented as: **fail when applicability_rules is present and days missing**; omitted rules remain confirm-default path. Full hard-require needs fixture migration + DefaultGlobalRules.DeadlineDays — deferred.

## Verification

| Check | Result |
|-------|--------|
| FE focused vitest (mappers/validation/list/editor/portal deadline) | PASS (117) |
| `npm run lint` | exit 0 (pre-existing tsc noise in unrelated tests) |
| `npm run build` (cobo_web_design) | PASS |
| `go test ./internal/disclosure/...` | PASS |
| `docker compose -f docker-compose.dev.yml build api` | PASS |
| `git diff --check` (both repos) | PASS |
| commit/push/deploy | not done |

## Out of scope (unchanged)

- `DisclosureTypeList` / company-template modal
- Dropping `deadline_rule` column / catalog API / DeadlineRuleCatalogScreen files
- `ResolveDeadlineRule` calculation semantics
- Mass rewrite of legacy DB rows

## 5-axis review

### Correctness
- Periodic save/import derive `T+N` from days when days > 0; raw overridden.
- Portal precedence: resolved → company-config fallback → legacy raw.
- Irregular authoring/import contract preserved.

### Architecture
- Compatibility derive at FE mapper + BE upsert/import normalize boundaries; Portal resolve unchanged.
- Catalog UI removed without deleting catalog storage/API (Phase later).

### Backward compatibility
- Column + catalog retained.
- Old periodic rows keep raw on read; re-save normalizes.
- Import omit-rules path still validates (soft days rule).

### Security / data integrity
- No secret logging changes.
- No schema migration; derive is deterministic overwrite of display compatibility only.
- Matrix/activate still require non-empty rule after derive when days present.

### Performance
- Negligible (string format + validation branch). No extra DB round-trips.

## Merge recommendation

**Conditional go** after human review of soft import validation vs PO “always require days”. Recommend follow-up task: hard-require `deadline_days` on all periodic imports + update fixtures + set default days on omitted-rules defaults.

## Post-verify addendum (same day)

See `cms-remove-deadline-rules-postverify-2026-09-23.md` for Phase 0–6 evidence, consumer KEEP/DEPRECATE map, browser BLOCKED items, and CONDITIONAL GO / production NO-GO until DEV CMS deploy smoke.
