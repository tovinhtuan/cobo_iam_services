# P0 Email Content Contract Validation Report

**Environment:** DEV — `88.216.208.0:21239`  
**Date:** 2026-06-09  
**Binary deployed at:** 2026-06-09T14:03:18Z  
**Scope:** Fix 1 (company_name), Fix 2 (creator_name), Fix 3 (reminder deep-link), Fix 4 (PUBLIC_WEB_BASE_URL guard)

---

## 1. Deployment Evidence

| Item | Value | Status |
|---|---|---|
| Worker start time | `2026-06-09T14:03:18Z` | ✅ |
| `PUBLIC_WEB_BASE_URL` | `http://88.216.208.0:3000` | ✅ |
| `ADHOC_EMAIL_OUTBOX_ENABLED` | `true` (durable pipeline, sole sender) | ✅ |
| SMTP backend | `smtp.gmail.com` (real delivery, not mailpit) | ✅ |
| `ENV` | `development` (startup guard passes correctly) | ✅ |
| Fix 4 guard code | `validatePublicWebBaseURL` → rejects localhost when `ENV != development` | ✅ |

---

## 2. Rendered Email Evidence — Adhoc Templates

Queried `email_notifications.variables_json_sanitized` for 4 POST-fix emails:

**`adhoc.controller_review_requested` (2 emails):**
```json
{
  "portal_url":   "http://88.216.208.0:3000",
  "company_name": "Company X",
  "creator_name": "Admin Doanh Nghiep",
  "proposal_id":  "019eacba-2e94-74d2-bd0e-fb1f8bd59ecc",
  "change_note":  "P0 Email Test Rejection - Strategy Change"
}
```

Template body (`adhoc.controller_review_requested/vi/body.txt`):
```
Công ty: {{.company_name}}
Người đề xuất: {{.creator_name}}
...
{{.portal_url}}/app/ad-hoc-proposals/{{.proposal_id}}
```

**Rendered CTA:** `http://88.216.208.0:3000/app/ad-hoc-proposals/019eacba-2e94-74d2-bd0e-fb1f8bd59ecc` — absolute deep-link ✅

---

## 3. Adhoc Email Validation (Fix 1 + Fix 2)

| Template | company_name | creator_name | Status |
|---|---|---|---|
| `adhoc.controller_review_requested` (×2) | `"Company X"` | `"Admin Doanh Nghiep"` (submitter) | ✅ |
| `adhoc.proposal_approved` | `"Company X"` | N/A (not in template) | ✅ |
| `adhoc.proposal_rejected` | `"Company X"` | N/A (not in template) | ✅ |

Pre-fix historical rows show `company_name: "08f59da2-..."` (UUID) — expected, old data before deploy.

**Fix 1 verdict:** PASS — `company_name` is `"Company X"` (not UUID `c_001`) on all post-fix emails.  
**Fix 2 verdict:** PASS — `creator_name` is `"Admin Doanh Nghiep"` (proposal submitter), not reviewer's own name.

---

## 4. Reminder Email Validation (Fix 3)

**Template change:** `reminder.disclosure_deadline/vi/body.txt` now uses `{{.portal_url}}` (was `{{.action_url}}`):
```
{{- if .portal_url }}
Action link: {{.portal_url}}
{{- end }}
```

**Service fallback** (`reminder/app/service.go`, `prepareDispatch`):
- DISCLOSURE scope → `portal_url = publicWebBaseURL + "/app/disclosures/" + scopeID`
- WORKFLOW_STEP scope → extracts `disclosure_id` from `scopeID` prefix (`disclosure_id:step_id` format)

**Live dispatch evidence (post-deploy):**

| Field | Value |
|---|---|
| Occurrence ID | `fix3-qa-live-1781014756` |
| Scope type | `DISCLOSURE` |
| Scope ID | `019e688c-8c2b-75ed-afb1-b29b1b924718` |
| Dispatched at | `2026-06-09T14:19:22Z` (after 14:03 deployment) |
| Status | `SENT` |
| Provider message ID | `smtp-1781014762177222866` |
| Recipient | `tvttthptlvh@gmail.com` |

**Computed `portal_url`:** `http://88.216.208.0:3000/app/disclosures/019e688c-8c2b-75ed-afb1-b29b1b924718` — absolute deep-link ✅

---

## 5. Click-to-Approve Evidence

CTA in `adhoc.controller_review_requested`:
- Template: `{{.portal_url}}/app/ad-hoc-proposals/{{.proposal_id}}`
- Rendered: `http://88.216.208.0:3000/app/ad-hoc-proposals/019eacba-2e94-74d2-bd0e-fb1f8bd59ecc`
- URL is absolute, routes to proposal detail page where approve/reject actions are available ✅

Approve/reject API flow validated: login → auto-select company → HTTP 200 on approve/reject endpoints.

---

## 6. Database Evidence

```
email_notifications : 4 post-fix rows, status=sent, idempotency_key UNIQUE (no duplicates)
reminder_occurrences: fix3-qa-live, status=SENT, smtp provider_message_id recorded
```

No UUID in human-readable template variables on post-fix emails:
- `company_name` → `"Company X"` (not UUID)
- `creator_name` → `"Admin Doanh Nghiep"` (not UUID)
- `proposal_id` → UUID used only in URL path, not displayed as bare text in email body

---

## 7. Regression Findings

| Finding | Severity | Introduced by P0? |
|---|---|---|
| `TestLoad_UserAvatarEnvOverride` fails on Windows (backslash path) | Low | No — pre-existing |
| `qa-step-name-test-1780373204-idem` stuck PENDING, SLA_BREACH every 5s | Low operational noise | No — bad seed data from QA script |

No new test failures introduced by P0 fixes.

Cleanup recommended:
```sql
DELETE FROM reminder_occurrences WHERE occurrence_id = 'qa-step-name-test-1780373204-idem';
```

---

## 8. Final Verdict

**PASS WITH RISKS**

| Fix | Status | Evidence |
|---|---|---|
| Fix 1: company_name = business name (not UUID) | ✅ PASS | DB: `"Company X"` in `variables_json_sanitized` |
| Fix 2: creator_name = submitter (not reviewer) | ✅ PASS | DB: `"Admin Doanh Nghiep"` in controller_review emails |
| Fix 3: reminder deep-link absolute | ✅ PASS | Live SENT at 14:19:22 + template uses `portal_url` + service builds `/app/disclosures/{id}` |
| Fix 3: adhoc CTA absolute deep-link | ✅ PASS | Template renders `portal_url + /app/ad-hoc-proposals/ + proposal_id` |
| Fix 4: PUBLIC_WEB_BASE_URL guard | ✅ PASS | `validatePublicWebBaseURL` active, dev env bypasses localhost check correctly |
| No UUID in visible email text | ✅ PASS | All human-readable fields are strings; UUIDs only in URL paths |
| All emails delivered (not queued) | ✅ PASS | All `email_notifications.status = sent`, SMTP provider_message_id recorded |

### Risks (non-blocking)

1. **Gmail inbox not directly inspectable** — DB variables used as proxy. Email body verified via template source + variables, not visual rendering.
2. **Reminder payload not logged** — worker logs no debug payload; `portal_url` value inferred from code + scope ID math, not log output.
3. **QA seed occurrence stuck** — `qa-step-name-test-*` triggers SLA_BREACH alert every 5s. See cleanup SQL above.
4. **creator_name absent on approved/rejected templates** — present in variables for controller_review only (correct per current template design).
