# Touchpoint reconfirmation

| Touchpoint | Path | Action |
|------------|------|--------|
| Worker entry | cmd/worker/main.go | unchanged (Seed→Materialize) |
| SeedPeriodicCycles | internal/disclosure/app/periodic.go | additive future slot |
| MaterializePeriodicDisclosures | periodic.go | bufferDays=0 OpenAt gate |
| ResolveLogicalSlot / NextLogicalSlot | existing | reused |
| ResolveNextApplicableLogicalSlot | next_applicable_slot.go | NEW |
| ResolveEffectiveAnchor / ResolveOccurrenceT | existing | reused for future T |
| AF/AT eligibility | existing Evaluate* | reused |
| UpsertPeriodicCycle | mysql repository | reused |
| ListPendingCycles | mysql | bufferDays=0 |
| deadline_config DTO | contracts.go + FE types | field added |
| CMS editor | TemplateEditorScreen | field + copy |
| Clone/import | existing deadline_config copy | field rides along |
