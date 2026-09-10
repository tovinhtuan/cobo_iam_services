# Source diff

## BE
- contracts.go — DTO field
- next_applicable_slot.go — NEW resolver + validation + GenerateAt
- periodic.go — future seed + OpenAt materializer gate
- service.go — write validation
- repository.go — JSON_EXTRACT lead days
- *_test.go — coverage
- docs/ai-cache evidence + reusable-task-updates

## FE
- types.ts, cmsApi.ts, templateDefaults.ts, templateMappers.ts, TemplateEditorScreen.tsx
- periodicCycleGenerationLeadDays.test.ts
- docs pointer + reusable-task-updates

## Not changed
- Tenant portal next-alert UI
- deadlinealerts package
- DueAt/OpenAt formula helpers (beyond materializer claim window)
- TENANT_SOURCE_CHANGED=false
- UNEXPLAINED_SOURCE_FILES=0
