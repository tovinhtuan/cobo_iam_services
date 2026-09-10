# Context — Periodic Future Cycle Pre-Generation V1 (Phase A)

- Date: 2026-09-10
- Mode: IMPLEMENTATION / SOURCE-AUDIT-APPROVED / DELTA ONLY
- Environment: DEV
- Repos: cobo_iam_services (BE worker) + cobo_web_design (CMS)
- Prior audit: `periodic-future-occurrence-materialization-audit-2026-09-10/`
- Goal: CMS config `periodic_cycle_generation_lead_days` + seed future `periodic_cycle` only; materialize only when TodayHCM >= OpenAt
- Out of scope: Tenant "Cảnh báo tiếp theo", Deadline Alert changes, company override, DB migration
- NO_COMMIT / NO_PUSH / NO_MERGE / NO_PRODUCTION
