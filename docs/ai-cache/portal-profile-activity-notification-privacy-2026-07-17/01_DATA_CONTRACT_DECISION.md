# 01 — Data contract decision

| Surface | Source | Notes |
|---------|--------|-------|
| Right rail “Hoạt động báo cáo gần đây” | `activities` in operational-overview | Filtered report-related in-app notifs across user’s companies |
| Tab “Lịch sử hoạt động” | `activity_log` in operational-overview | `audit_logs` where `actor_user_id = current user` |
| Fake data | Never | Empty states if empty/unavailable |

Report-related kinds: `reminder.*`, `adhoc.*`, `disclosure.*`, `workflow.*`, `deadline.*` (+ disclosure/ad_hoc resource types).

Privacy: self `/me` only; `GET /api/v1/users/{id}/profile` not a product route → 404/405.

Verdict: **READY_TO_IMPLEMENT** (done).
