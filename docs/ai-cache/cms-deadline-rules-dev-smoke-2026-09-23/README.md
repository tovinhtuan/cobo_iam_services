# Smoke QA DEV — CMS Deadline Rules (2026-09-23) — Re-smoke

```text
task_type: qa-smoke-dev-resmoke
timestamp: 2026-09-23 ~14:50–16:15 +07
repos: cobo_iam_services (evidence/docs), cobo_web_design (runtime FE on DEV)
code_changed: false (app source untouched in this QA cycle)
data_changed: QA-only — temporary static fixtures under FE dist (removed after validate)
commit/push: not performed
deploy: FE (+ prior BE) performed via existing make deploy-* workflow
```

Evidence dir: `docs/ai-cache/cms-deadline-rules-dev-smoke-2026-09-23/`  
Detailed matrix: [`re-smoke-report.md`](./re-smoke-report.md)

---

## Release artifact

| Field | Value |
|-------|--------|
| FE repo commit | `e75212caa93c7aa1e1031ae6e4ca825e94932a60` (`fixbug/test-9`, message: `fix bug test 9`) |
| FE bundle | **`assets/index-D5jI2hEe.js`** (NOT stale `index-DWmk2Tu6.js`) |
| Bundle Last-Modified | **Wed, 23 Sep 2026 07:54:49 GMT** |
| Bundle markers | **No** UI string `Deadline Rules`; **no** `Deadline rule hiển thị trên Portal`; **has** `Phạm vi áp dụng`; `deadline-rules` string remains only for legacy URL fallback path |
| API DEV | `http://88.216.208.0:8080` — `/healthz` 200, `/readyz` 200 |
| FE DEV | `http://88.216.208.0:3000` |
| Account / role | **Platform + Tenant Admin (Dev)** (`u_platform_tenant_admin` / seed `platform.tenant.admin@example.com`) — Platform CMS OK |
| Companies on session | Company X (`c_001`), Company Y (`c_002`), Isolation Co — **Company B (`bd1f…`) not granted** |

---

## Scenario matrix (re-smoke)

| ID | Scenario | Expected | Actual | Status | Evidence |
|----|----------|----------|--------|--------|----------|
| P0 | FE artifact + health | New bundle; BE healthy | `index-D5jI2hEe.js` 23/09; healthz/readyz OK | **PASS** | `evidence/bundle-headers.txt` |
| AUTH | Platform CMS | `/cms/templates` not forbidden | Opens Templates + Nhóm hiển thị | **PASS** | `05-*.png`, live DOM |
| CMS-1 | No Deadline Rules tab | Tabs = Templates / Nhóm hiển thị | Confirmed; no catalog CRUD from normal CMS browse | **PASS** | `05`, `09` |
| CMS-2 | `?tab=deadline-rules` fallback | List templates | Fallback to list (prior + recheck) | **PASS** | `06-*.png` |
| CMS-3 | Periodic editor | Phạm vi áp dụng; no Portal textarea; `deadline_days` | `deadline_days=20`; structure 30/25/20; copy explains resolve | **PASS** | `07`, `09` |
| CMS-4A | Save Case A (days=20) | Payload days=20; derive `T+20` | Editor shows 20; **Lưu nháp disabled** (clean); **no new save network** this pass — derive proven via Import A | **PARTIAL** | editor DOM + IMP-A |
| CMS-4B | Save Case B structure | Structure map + Company B 20d Portal | Structure inputs 30/25/20 in editor; **Portal Company B BLOCKED** (no membership) | **PARTIAL / BLOCKED** | `09` + company list |
| IMP-A | Import missing rule | Derive `T+20`; domain OK | `deadline_rule=T+20`; `domain_valid=true` | **PASS** | `evidence/import-validate-A-E.json` |
| IMP-B | Raw `T+99` + days=20 | Overwrite → `T+20` | Preview/normalized `T+20` (not T+99) | **PASS** | same |
| IMP-C | `deadline_days<=0` | Reject `APPLICABILITY_DEADLINE_DAYS_REQUIRED` | Exact code + field path; `domain_valid=false` | **PASS** | same |
| IMP-D | Legacy omit applicability | Soft compat; keep raw rule | `domain_valid=true`; rule `T+20` kept | **PASS** | same |
| IMP-E | Irregular | No periodic `T+N` derive | Rule stays event text; category irregular | **PASS** | same |
| PORT-A | Company X structure | 30 days; no raw T+N; no enum | UI: “Trong vòng 30 ngày theo lịch.” + “Có công ty con.” | **PASS** | `08-*.png` |
| PORT-B | Company B → 20 days | Simple structure 20 | Company B **not** on account | **BLOCKED** | `/api/v1/me/companies` |
| PORT-MISS | Missing profile fallback | Friendly copy; no 500 / no T+N | No incomplete-profile Portal fixture on session | **BLOCKED** | — |
| IRR | Irregular Portal | No “bắt đầu chu kỳ”; no invented T+N | Event text present; no cycle sentence; `event_based` label still visible (pre-existing) | **PASS*** | live detail text |

\*Irregular still shows technical `event_based` in periodicity chip — noted; not introduced as Deadline Rules authoring regression.

---

## PASS / BLOCKED / FAIL summary

### PASS

- New FE artifact deployed and verified in browser.
- Platform CMS access.
- CMS UI: no Deadline Rules tab; legacy tab fallback; periodic applicability editor.
- Import validate A–E against live DEV API with Platform session (same endpoint as Import UI).
- Portal Company X periodic resolve 30 days (List API + Detail UI).
- Irregular: no periodic cycle-start sentence; no raw `T+N` invent.

### BLOCKED

- **PORT-B** Company B (simple / 20 days) — membership not on smoke account.
- **PORT-MISS** missing-profile fallback UI — no reachable incomplete profile fixture.
- **CMS-4 save network write** — form dirty/save automation unstable; avoided mutating Active template without clean capture. Derive contract covered by Import A/B.

### FAIL

- None that meet hard NO-GO product defects for this cycle (stale FE cleared; Platform CMS available; Import A–E match expectations).

---

## Final verdict

**CONDITIONAL GO**

Rationale:

1. Prior **NO-GO** blockers (stale FE `DWmk2Tu6`, no Platform CMS) are **cleared**.
2. CMS removal + Import soft-compat A–E have **live DEV evidence**.
3. Remaining gaps are **fixture/account coverage** (Company B, missing-profile) and **save write network capture**, with owners below — not observed wrong import derive / not stale FE / not missing CMS auth.

**Not GO:** Company B 20-day Portal resolve and missing-profile fallback lack live evidence required by full GO checklist.

### Owners / next actions

| Gap | Owner | Action | Severity |
|-----|-------|--------|----------|
| Grant Company B (`bd1f…`) or equivalent simple-structure tenant to Platform CMS smoke user | DevOps / IAM seed | Membership + re-run PORT-B | Medium (coverage) |
| Provide incomplete company profile fixture for fallback copy | QA data | Re-run PORT-MISS | Medium (coverage) |
| Capture CMS Save Case A/B network (dirty→Lưu nháp) on dedicated QA draft type_id | QA | Avoid writing Active BCTC fixture | Low–Medium |
| Optional: hide `event_based` raw chip on irregular Portal | FE | Polish; not release blocker for this change | Low |

---

## Related docs

- `docs/ai-cache/cms-remove-deadline-rules-implementation-2026-09-23.md`
- `docs/ai-cache/cms-remove-deadline-rules-postverify-2026-09-23.md`
- `docs/ai-cache/cms-remove-deadline-rules-ui-implementation-plan-2026-09-23.md`
