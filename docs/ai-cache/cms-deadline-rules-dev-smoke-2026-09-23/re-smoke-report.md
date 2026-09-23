# Re-smoke report — CMS remove Deadline Rules UI (DEV)

```text
date: 2026-09-23
window: ~14:50–16:15 +07
role: Release QA Lead
environment: DEV 88.216.208.0 FE:3000 API:8080
```

## 1. Release artifact

| Item | Value |
|------|--------|
| FE commit | `e75212caa93c7aa1e1031ae6e4ca825e94932a60` |
| FE branch | `fixbug/test-9` |
| Deployed bundle | `http://88.216.208.0:3000/assets/index-D5jI2hEe.js` |
| Last-Modified | Wed, 23 Sep 2026 07:54:49 GMT |
| Rejected stale bundle | `assets/index-DWmk2Tu6.js` (22 Sep) — **not** serving |
| Deploy path | Existing `make deploy-fe` / artifacts → `/root/cobo_project/web/dist` |
| BE | Deployed earlier in session; `GET /healthz` → ok; `GET /readyz` → ready |

### Bundle content checks (browser fetch of loaded script)

| Marker | Result |
|--------|--------|
| `Deadline Rules` | Absent |
| `Deadline rule hiển thị trên Portal` | Absent |
| `Phạm vi áp dụng` | Present |
| `deadline-rules` path string | Present (legacy `?tab=` fallback) — expected |

## 2. Account / role

| Field | Value (non-secret) |
|-------|---------------------|
| Display | Platform + Tenant Admin (Dev) |
| user_id | `u_platform_tenant_admin` |
| CMS | `/cms/templates` reachable; not `/app/forbidden` |
| Companies | `c_001` Company X, `c_002` Company Y, Isolation Co |
| Secrets | Not recorded (no passwords/tokens/cookies in this report) |

## 3. CMS UI re-smoke

### Navigation

- `/cms/templates` → tabs **Templates**, **Nhóm hiển thị** only.
- No Deadline Rules menu/tab.
- No deadline-rules catalog requests from normal list browse (resources show `disclosure-types?list_mode=management`).
- Legacy `?tab=deadline-rules` → list (screenshot `06`).

### Periodic editor (`type_id=qa-import-periodic-1788864118253`)

- Section **Phạm vi áp dụng** present.
- Field **Số ngày công bố (deadline_days)** = `20`.
- Structure overrides visible: 30 / 25 / 20.
- Copy states applicability is SoT for Portal resolve; no manual Portal deadline textarea.
- **Lưu nháp** disabled while clean → no save network capture this pass.

Screenshots: `screenshots/05`, `06`, `07`, `09`.

## 4. Import cases A–E (live validate)

Method: Platform CMS session → `POST /api/v1/platform/cms/templates/import/validate` with fixtures from `import-fixtures/` (same API as Import UI). Temporary same-origin static copies used for automation then **removed** from DEV dist.

Raw tokens redacted. Full compact matrix: `evidence/import-validate-A-E.json`.

| Case | Input highlight | HTTP | parse | domain | deadline_rule result | Error | Verdict |
|------|-----------------|------|-------|--------|----------------------|-------|---------|
| A | days=20, no rule | 200 | true | true | **T+20** | — | **PASS** |
| B | rule=T+99, days=20 | 200 | true | true | **T+20** (not T+99) | — | **PASS** |
| C | days=0 + applicability | 200 | true | false | (preview may show derived) | **APPLICABILITY_DEADLINE_DAYS_REQUIRED** @ `template.applicability_rules.deadline_days` | **PASS** |
| D | legacy omit applicability, raw T+20 | 200 | true | true | **T+20** kept | — | **PASS** (soft legacy) |
| E | irregular event text | 200 | true | true | **Trong vòng 24 giờ kể từ sự kiện** (no T+N) | — | **PASS** |

Notes:

- `can_confirm=false` on A/B/D/E due to `UNRESOLVED_DEPARTMENT_MAPPING` warnings (fixture departments) — **not** a deadline derive failure; confirm not executed (avoids partial records).
- Case C correctly blocked at domain validation (no confirm).

## 5. Portal verification

### Company X — periodic `qa-import-periodic-1788864118253`

- API `resolved_deadline_rule`: `resolved_days=30`, `STRUCTURE_OVERRIDE` / `has_subsidiaries`.
- UI Detail: **Trong vòng 30 ngày theo lịch.** + **Có công ty con.**
- No raw `T+20`/`T+30` as primary copy; no structure enum codes in deadline section.
- Screenshot: `screenshots/08-portal-periodic-company-x-detail-recheck.png`.

### Company B — 20 days

- **BLOCKED:** Company B id not in `/api/v1/me/companies` for this account.

### Missing profile fallback

- **BLOCKED:** no incomplete-profile company reachable for disclosure.

### Irregular `qa-irregular-alert-20260904a`

- Event copy retained; **no** “kể từ ngày bắt đầu chu kỳ”.
- No invented `T+N` string.
- Periodicity chip still shows `event_based` (technical) — noted observation.

## 6. Console / network

- No React crash on CMS list/editor or Portal detail during re-smoke.
- Import validate responses 200 with expected domain flags.
- Avatar signed URLs observed in network — **not** stored.

## 7. Blockers & owners

| ID | Issue | Severity | Owner | Action |
|----|-------|----------|-------|--------|
| B1 | Company B not on smoke memberships | Medium (coverage) | IAM seed / DevOps | Grant simple-structure company; re-run PORT-B |
| B2 | Missing-profile Portal fixture | Medium (coverage) | QA data | Provide incomplete profile tenant; re-run PORT-MISS |
| B3 | CMS Save Case A/B network not captured | Low–Medium | QA | Dirty+save on **dedicated QA draft** type_id |

None of B1–B3 are observed incorrect Import derive or stale FE.

## 8. Final verdict

**CONDITIONAL GO**

Clears previous NO-GO (stale FE + no Platform CMS). Full **GO** deferred until PORT-B + PORT-MISS (+ optional save network) have live evidence.
