# Flow G — Regression

- time_utc: 2026-09-24T08:38:26.110134+00:00
- Global CMS history API: PASS (see flow B)
- Direct publish: see flow C
- Materialize NotStarted + idempotent: see flow D
- Company queue PendingReview: see flow E
- Publishing nav default hidden: see flow A + CmsLayout.test
- Legacy entries flag unchanged: VITE_CMS_LEGACY_ENTRIES_NAV still independent
- Backend APIs reviews/schedules/releases: unchanged (kept)
- result: PASS if A–F pass
