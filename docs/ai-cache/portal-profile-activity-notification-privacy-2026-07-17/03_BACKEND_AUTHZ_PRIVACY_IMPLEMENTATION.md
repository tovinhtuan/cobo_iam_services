# 03 — Backend authz / privacy

- `ListFilter.ActorUserID` + mysql/inmemory filter
- personalops: `buildReportActivities` (filter kinds) + `buildActivityLog` (actor audit)
- Overview adds `activity_log[]`
- Wire `WithAuditLister(auditRepo)` in httpserver
- Profile privacy: only `/me` endpoints; no member personal-profile-by-id API
- Tests: `TestOperationalOverview_SeparatesReportActivitiesAndActivityLog`
