# Pointer — Tenant Step Detail timeliness + required-task audit

Full evidence pack lives in sibling FE repo:

`../cobo_web_design/docs/ai-cache/tenant-step-detail-timeliness-required-task-audit-2026-09-13/`

## Summary

- MODE=SOURCE_AUDIT_ONLY
- BE authoritative enum: `timeliness_status` via `ResolveStepTimeliness` (`internal/deadlinealerts/app/timeliness.go`)
- Step Detail FE metric `Còn lại / Quá hạn` ignores that enum; uses `is_delayed` + disclosure DueAt
- Locked contract unchanged; BE calculation not defective for the reported cases
- "Tác vụ bắt buộc": no BE domain field; FE label over `TaskDTO` list
- BE_SOURCE_CHANGED=false this phase
