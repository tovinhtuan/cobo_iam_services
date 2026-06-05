# Batch 3 Report: Listed Company Lookup & Sync — Frontend

**Date:** 2026-06-05  
**Scope:** C1 (authApi) + C2 (useListedCompanyLookup) + C3 (ListedCompanyPreviewCard) + C4 (provisionErrors) + C5 (CreateCompanyModal integration)  
**Status:** ✅ COMPLETE

---

## Architecture Verification

| Component | Responsibility | State Owner | Risk | Modification |
|---|---|---|---|---|
| `CompanyCreateForm` | Controlled form UI | Props only | None | NOT modified |
| `useCompanyProvision` | Form values + idempotency + API submit | Self (`useState`) | None | NOT modified |
| `CreateCompanyModal` | Orchestrates provision hook + routing | Via useCompanyProvision | Low — additive only | MODIFIED: thêm lookup hook + preview card |
| `useListedCompanyLookup` | Lookup state machine + AbortController | Self | None | NEW |
| `ListedCompanyPreviewCard` | Checkbox-per-field sync UI + disclaimer | Props only (controlled) | None | NEW |

---

## State Flow Review

```
provision.values.registrationNumber (string)
    │ (watched by useListedCompanyLookup via useEffect)
    ▼
useListedCompanyLookup (debounce 500ms, min 8 chars)
    │ AbortController: cancels stale fetch if input changes before debounce fires
    │ or before response returns
    ▼
status: idle | loading | found | not_found | error
    │ (when found)
    ▼
ListedCompanyPreviewCard (checkbox per field)
    │ Pre-checked: fields where currentValues[field].trim() === ''
    │ User can toggle any checkbox
    ▼
onSync(patch) → provision.setValues((prev) => ({ ...prev, ...patch }))
    │ Only checked fields included in patch
    │ Only fields with API value included
    ▼
CompanyCreateForm re-renders with updated values
    │ User edits if needed
    ▼
provision.submit() (unchanged — existing provision flow)
```

**State ownership is clear:** `useListedCompanyLookup` owns only lookup state. Form state owned exclusively by `useCompanyProvision`.

---

## UX Review

### Smart Merge Decision — Checkbox per field (Option B with smart defaults)

The prompt REQUIRED IMPLEMENTATION (BẮT BUỘC) specifies checkbox per field. This is Option B. Previous execution plan specified Option D (smart merge / auto-fill empty). The prompt overrides:

- **Pre-check:** Fields where form is currently empty AND API has a value → pre-checked (smart default)
- **Pre-uncheck:** Fields where form already has content → pre-unchecked (user must opt-in to overwrite)
- **Toggle:** User can check/uncheck any field freely
- **Only sync checked fields with values:** `buildSyncPatch` skips unchecked + null API values

This prevents accidental overwrites while still being helpful.

### Stale Preview Prevention

- `AbortController` per fetch attempt
- Effect cleanup: `clearTimeout(timer); controller.abort()`
- When input changes → old timer cleared + old controller aborted → new timer starts
- If response arrives after abort → `if (controller.signal.aborted) return;` guard

### Loading States

- `loading`: spinner text "Đang tra cứu..." with `aria-live="polite"`
- `not_found`: "Không tìm thấy công ty niêm yết..." with `role="status"`
- `error` (503): "Tra cứu tạm thời không khả dụng." with `role="alert"` — form still submits

### Responsive Layout

- Preview card uses `rounded-xl border` + `p-4` consistent with modal design
- Checkbox list uses `ul` with `space-y-1.5` — scrollable in modal's `max-h-[90vh] overflow-y-auto`
- Action buttons: `flex gap-2` — both visible on mobile

---

## Smart Merge Review

| Option | Chosen | Reason |
|---|---|---|
| A — Overwrite all | No | Data loss risk |
| B — Checkbox per field | **Yes (BẮT BUỘC)** | User control, no surprises |
| C — Side-by-side compare | No | Too complex for modal context |
| D — Smart merge only (auto-fill empty) | Partially | Used as default state for checkboxes |

Implementation: Option B with Option D defaults (empty fields pre-checked, filled fields pre-unchecked).

---

## Files Changed

| File | Change |
|---|---|
| `src/services/authApi.ts` | C1: Added `lookupListedCompany`, `parseListedCompanyLookupPayload`, `ListedCompanyLookupResult` types |
| `src/features/company/provisionErrors.ts` | C4: Updated `COMPANY_ALREADY_EXISTS` message (tax_code + membership conflict) |
| `src/features/company/CreateCompanyModal.tsx` | C5: Added `useListedCompanyLookup` + status indicators + `ListedCompanyPreviewCard` integration |

## Files Created

| File | Description |
|---|---|
| `src/features/company/useListedCompanyLookup.ts` | C2: Debounce hook + AbortController + state machine + `buildSyncPatch` + `defaultSelectedFields` |
| `src/features/company/ListedCompanyPreviewCard.tsx` | C3: Checkbox-per-field sync UI + disclaimer + accessibility |
| `src/features/company/useListedCompanyLookup.test.ts` | Tests: 14 cases (buildSyncPatch + defaultSelectedFields + hook behavior) |
| `src/features/company/ListedCompanyPreviewCard.test.tsx` | Tests: 9 cases (render + checkbox logic + accessibility) |
| `src/features/company/provisionErrors.test.ts` | Tests: 5 cases (error message mapping) |

---

## Tests Added

### useListedCompanyLookup (14 tests)

| Test | Purpose |
|---|---|
| buildSyncPatch: returns only selected fields | Core sync logic |
| buildSyncPatch: returns empty patch when nothing selected | Edge case |
| buildSyncPatch: does not apply deselected fields | Checkbox off = not applied |
| defaultSelectedFields: pre-checks empty fields | Smart default |
| defaultSelectedFields: does not pre-check filled fields | No unintended overwrite |
| defaultSelectedFields: empty set when sync empty | Edge case |
| Hook: stays idle for short input | Min length guard |
| Hook: does not call API before debounce | Debounce prevention |
| Hook: transitions to found after debounce | Happy path |
| Hook: transitions to not_found | Miss path |
| Hook: stays not_found on non-ok response | 503 graceful |
| Hook: resets to idle immediately when input short | Stale prevention |
| Hook: resets to idle when input shortens during loading | Loading → idle |
| Hook: dismiss() resets to idle | User dismiss |

### ListedCompanyPreviewCard (9 tests)

| Test | Purpose |
|---|---|
| Renders company name, symbol, exchange | Header content |
| Disclaimer before action buttons | DOM order + accessibility |
| Pre-checks empty fields by default | Smart default |
| Does not pre-check filled fields | No overwrite |
| onSync called with only checked fields | Partial sync |
| Sync button disabled when no fields selected | Guard |
| Does not call onSync when disabled | Safety |
| onDismiss called on Bỏ qua | Dismiss flow |
| No checkboxes when sync empty | Empty state |
| All checkboxes have labels | Accessibility |

### provisionErrors (5 tests)

| Test | Purpose |
|---|---|
| COMPANY_ALREADY_EXISTS (via code) → tax_code message | Q-BLOCK-2 mapping |
| COMPANY_ALREADY_EXISTS (via message) → tax_code message | Backend variant |
| QUOTA_EXCEEDED still works | Regression |
| STATE_CONFLICT with other message still works | Regression |
| Non-ApiError handled gracefully | Safety |

---

## Tests Run

```
npm run test -- src/features/company/useListedCompanyLookup.test.ts \
               src/features/company/ListedCompanyPreviewCard.test.tsx \
               src/features/company/provisionErrors.test.ts

npm run test -- src/features/company/   (full company feature regression)

npm run build
npm run lint (excluding pre-existing DisclosureForm.fe004.test.tsx errors)
```

---

## Test Results

| Suite | Tests | Result |
|---|---|---|
| `useListedCompanyLookup.test.ts` | 14/14 | ✅ PASS |
| `ListedCompanyPreviewCard.test.tsx` | 9/9 | ✅ PASS |
| `provisionErrors.test.ts` | 5/5 | ✅ PASS |
| `CompanyCreateForm.test.tsx` (regression) | 2/2 | ✅ PASS — no regression |
| `CreateCompanyModal.test.tsx` (regression) | 7/7 | ✅ PASS — no regression |
| **Company feature total** | 37/37 | ✅ PASS |
| `npm run build` | — | ✅ Clean |
| `npm run lint` (new files only) | — | ✅ No new errors |

**Pre-existing lint errors:** `src/pages/portal/DisclosureForm.fe004.test.tsx` — 4 type errors (missing `description`, `deadlineRule` fields). Confirmed pre-existing, unrelated to Batch 3.

**Pre-existing build warning:** Chunk size > 500 kB — pre-existing, unrelated to Batch 3.

---

## Accessibility Verification

| Aspect | Implementation | Verified by |
|---|---|---|
| Fieldset + legend for checkbox group | `<fieldset><legend>Chọn thông tin muốn đồng bộ...</legend>` | Component code |
| Explicit `htmlFor` ↔ `id` on checkboxes | `id="sync-field-${key}"`, `htmlFor={id}` | Test: "all checkboxes have labels" |
| Keyboard navigation | Checkboxes + buttons are all native elements — Tab-navigable | Native browser behavior |
| Sync button `aria-disabled` | `aria-disabled={!hasSelection}` alongside `disabled` | Component code |
| Loading state ARIA | `aria-live="polite"` on loading text | Component code |
| Error state ARIA | `role="alert"` on error text | Component code |
| Not found state ARIA | `role="status"` on not found text | Component code |
| Region label | `role="region" aria-label="Kết quả tra cứu công ty niêm yết"` | Component code |
| Disclaimer as note | `role="note"` | Component code |

---

## Risks

| Risk | Severity | Notes |
|---|---|---|
| `createAuthApi({})` called with empty options inside hook → no token | Intentional | Lookup endpoint is public — no auth needed |
| `AbortController` cleanup when component unmounts | Handled | Effect cleanup: `controller.abort()` in return |
| Checkbox state resets if parent re-renders | Acceptable | `useState` with `() => defaultSelectedFields(...)` initializer — stable on mount |
| Slow API (>500ms) shows loading for extended time | Low | 500ms debounce + spinner text shown |

---

## Technical Debt

| Item | Priority |
|---|---|
| `createAuthApi` called with `{}` inside `useListedCompanyLookup` creates a new instance per render cycle (not memoized) | Low — hook is ephemeral, created when modal opens |
| Execution plan Phase C vs D labeling (C3=provisionErrors, C3 in prompt=PreviewCard) | Documentation debt only |

---

## Blockers

None.

---

## Ready For Batch 4

**YES** — All Batch 3 acceptance criteria met:
- ✅ C1: `lookupListedCompany` + types + `parseListedCompanyLookupPayload` in `authApi.ts`
- ✅ C2: `useListedCompanyLookup` with debounce, AbortController, state machine, `buildSyncPatch`, `defaultSelectedFields`
- ✅ C3: `ListedCompanyPreviewCard` with checkbox-per-field, disclaimer, pre-check logic, accessibility
- ✅ C4: `provisionErrors.ts` `COMPANY_ALREADY_EXISTS` message updated (Q-BLOCK-2)
- ✅ C5: `CreateCompanyModal.tsx` integrated — lookup hook + status indicators + preview card
- ✅ 37/37 company feature tests pass (including regression)
- ✅ `npm run build` clean
- ✅ `npm run lint` clean (no new errors)

Batch 4 scope (per execution plan): Phase E — Integration Testing (E1–E11 E2E scenarios) + Phase F (Dev Deploy).
