# 40 — USER COMMIT MANIFEST (INFORMATIONAL ONLY)
STAGING_COMMAND_COUNT=0
COMMIT_COMMAND_COUNT=0
Do NOT execute git add/commit. User decides.

## BE PRODUCT
- PATH=cobo_iam_services/internal/workflowstepevidence/audit.go
  CLASSIFICATION=A
  WHY=Generic audit helpers
  COMMIT_POLICY=INCLUDE

- PATH=cobo_iam_services/internal/workflowstepevidence/concurrency_test.go
  CLASSIFICATION=D
  WHY=Concurrency tests
  COMMIT_POLICY=INCLUDE

- PATH=cobo_iam_services/internal/workflowstepevidence/domain.go
  CLASSIFICATION=A
  WHY=Domain model
  COMMIT_POLICY=INCLUDE

- PATH=cobo_iam_services/internal/workflowstepevidence/dto.go
  CLASSIFICATION=A
  WHY=API DTOs
  COMMIT_POLICY=INCLUDE

- PATH=cobo_iam_services/internal/workflowstepevidence/foundation.go
  CLASSIFICATION=A
  WHY=Storage foundation
  COMMIT_POLICY=INCLUDE

- PATH=cobo_iam_services/internal/workflowstepevidence/lifecycle.go
  CLASSIFICATION=A
  WHY=Lifecycle
  COMMIT_POLICY=INCLUDE

- PATH=cobo_iam_services/internal/workflowstepevidence/memory/repository.go
  CLASSIFICATION=A
  WHY=Memory repo
  COMMIT_POLICY=INCLUDE

- PATH=cobo_iam_services/internal/workflowstepevidence/mysql/repository.go
  CLASSIFICATION=A
  WHY=MySQL repo
  COMMIT_POLICY=INCLUDE

- PATH=cobo_iam_services/internal/workflowstepevidence/repository.go
  CLASSIFICATION=A
  WHY=Repo contract
  COMMIT_POLICY=INCLUDE

- PATH=cobo_iam_services/internal/workflowstepevidence/repository_test.go
  CLASSIFICATION=D
  WHY=Repo/B3 tests
  COMMIT_POLICY=INCLUDE

- PATH=cobo_iam_services/internal/workflowstepevidence/service.go
  CLASSIFICATION=A
  WHY=Service
  COMMIT_POLICY=INCLUDE

- PATH=cobo_iam_services/internal/workflowstepevidence/service_g2b_test.go
  CLASSIFICATION=D
  WHY=G2B API tests
  COMMIT_POLICY=INCLUDE

- PATH=cobo_iam_services/internal/workflowstepevidence/service_g2c_test.go
  CLASSIFICATION=D
  WHY=G2C hardening tests
  COMMIT_POLICY=INCLUDE

- PATH=cobo_iam_services/internal/workflowstepevidence/storage_key.go
  CLASSIFICATION=A
  WHY=Server-side key namespace
  COMMIT_POLICY=INCLUDE

- PATH=cobo_iam_services/internal/workflowstepevidence/transport/http/handler.go
  CLASSIFICATION=A
  WHY=5 routes
  COMMIT_POLICY=INCLUDE

- PATH=cobo_iam_services/internal/httpserver/server.go
  CLASSIFICATION=A
  WHY=Wire evidence handler
  COMMIT_POLICY=INCLUDE

- PATH=cobo_iam_services/internal/platform/errors/errors.go
  CLASSIFICATION=A
  WHY=Evidence error codes
  COMMIT_POLICY=INCLUDE

- PATH=cobo_iam_services/internal/workflowfulfillment/deadline_bridge.go
  CLASSIFICATION=A
  WHY=Shared EvaluateStepAuthority
  COMMIT_POLICY=INCLUDE

## DB MIGRATION
- PATH=cobo_iam_services/migrations/0137_workflow_step_evidence_files.up.sql
  CLASSIFICATION=C
  WHY=Create table
  COMMIT_POLICY=INCLUDE

- PATH=cobo_iam_services/migrations/0137_workflow_step_evidence_files.down.sql
  CLASSIFICATION=C
  WHY=Drop table
  COMMIT_POLICY=INCLUDE

- PATH=cobo_iam_services/migrations/run_dev_migrations.sh
  CLASSIFICATION=C
  WHY=Dev migration list
  COMMIT_POLICY=INCLUDE

## BE TESTS
(included under workflowstepevidence *_test.go above)

## FE PRODUCT
- PATH=cobo_web_design/src/services/workflowStepEvidenceApi.ts
  CLASSIFICATION=B
  WHY=API types/client
  COMMIT_POLICY=INCLUDE

- PATH=cobo_web_design/src/services/createWorkflowStepEvidenceApi.ts
  CLASSIFICATION=B
  WHY=Factory
  COMMIT_POLICY=INCLUDE

- PATH=cobo_web_design/src/pages/portal/deadlines/evidence/StepGenericEvidencePanel.tsx
  CLASSIFICATION=B
  WHY=UI panel
  COMMIT_POLICY=INCLUDE

- PATH=cobo_web_design/src/pages/portal/deadlines/evidence/useStepGenericEvidence.ts
  CLASSIFICATION=B
  WHY=Hook
  COMMIT_POLICY=INCLUDE

- PATH=cobo_web_design/src/pages/portal/DeadlineDetail.tsx
  CLASSIFICATION=B
  WHY=Mount Generic section
  COMMIT_POLICY=INCLUDE

- PATH=cobo_web_design/src/pages/portal/deadlines/evidence/StepDocumentRequirementsPanel.tsx
  CLASSIFICATION=B
  WHY=Empty-state copy polish
  COMMIT_POLICY=INCLUDE

## FE TESTS
- PATH=cobo_web_design/src/services/workflowStepEvidence.g3.test.tsx
  CLASSIFICATION=E
  WHY=G3 vitest
  COMMIT_POLICY=INCLUDE

## RELEASE SMOKE
- PATH=cobo_web_design/.playwright-mcp/generic-evidence-v1-1-release-smoke.cjs
  CLASSIFICATION=F
  WHY=Canonical G4 release smoke
  COMMIT_POLICY=INCLUDE

- PATH=cobo_web_design/.playwright-mcp/g3-generic-evidence-browser-smoke.cjs
  CLASSIFICATION=F
  WHY=G3 browser smoke
  COMMIT_POLICY=INCLUDE

- PATH=cobo_iam_services/.playwright-mcp/g2c-generic-evidence-audit-smoke.cjs
  CLASSIFICATION=F
  WHY=G2C audit smoke
  COMMIT_POLICY=INCLUDE

- PATH=cobo_iam_services/.playwright-mcp/g3-generic-evidence-browser-smoke.cjs
  CLASSIFICATION=F
  WHY=BE-side G3 smoke helper if present
  COMMIT_POLICY=INCLUDE

- PATH=cobo_iam_services/.playwright-mcp/g4-materialize-draft/main.go
  CLASSIFICATION=F
  WHY=QA draft materializer source
  COMMIT_POLICY=INCLUDE

## GENERATED FILES
(none recommended for INCLUDE — see exclusions)

## EVIDENCE DOCS
- PATH=cobo_web_design/docs/ai-cache/tenant-generic-step-evidence-v1-1-g1-contract-source-audit-2026-09-18/** (already tracked — no new)
- PATH=cobo_web_design/docs/ai-cache/tenant-generic-step-evidence-v1-1-g2a-runtime-domain-2026-09-18/**
  CLASSIFICATION=G COMMIT_POLICY=INCLUDE
- PATH=cobo_web_design/docs/ai-cache/tenant-generic-step-evidence-v1-1-g2b-crud-api-2026-09-18/**
  CLASSIFICATION=G COMMIT_POLICY=INCLUDE
- PATH=cobo_web_design/docs/ai-cache/tenant-generic-step-evidence-v1-1-g2c-audit-security-hardening-2026-09-18/** (pointer)
  CLASSIFICATION=G COMMIT_POLICY=INCLUDE
- PATH=cobo_web_design/docs/ai-cache/tenant-generic-step-evidence-v1-1-g3-tenant-ux-2026-09-18/** + screenshots/
  CLASSIFICATION=G/H COMMIT_POLICY=INCLUDE
- PATH=cobo_web_design/docs/ai-cache/tenant-generic-step-evidence-v1-1-g4-full-release-verification-2026-09-18/** + screenshots/ + release-smoke-result.json
  CLASSIFICATION=G/H/I COMMIT_POLICY=INCLUDE
- PATH=cobo_web_design/docs/ai-cache/tenant-generic-step-evidence-v1-1-g5-final-precommit-review-2026-09-18/**
  CLASSIFICATION=G COMMIT_POLICY=INCLUDE
- PATH=cobo_web_design/docs/ai-cache/README.md
  CLASSIFICATION=G COMMIT_POLICY=INCLUDE
- PATH=cobo_iam_services/docs/ai-cache/tenant-generic-step-evidence-v1-1-g2a-runtime-domain-2026-09-18/** (pointer or full as present)
  CLASSIFICATION=G COMMIT_POLICY=INCLUDE
- PATH=cobo_iam_services/docs/ai-cache/tenant-generic-step-evidence-v1-1-g2b-crud-api-2026-09-18/**
  CLASSIFICATION=G COMMIT_POLICY=INCLUDE
- PATH=cobo_iam_services/docs/ai-cache/tenant-generic-step-evidence-v1-1-g2c-audit-security-hardening-2026-09-18/**
  CLASSIFICATION=G COMMIT_POLICY=INCLUDE
- PATH=cobo_iam_services/docs/ai-cache/tenant-generic-step-evidence-v1-1-g3-tenant-ux-2026-09-18/00-pointer.md
  CLASSIFICATION=G COMMIT_POLICY=INCLUDE
- PATH=cobo_iam_services/docs/ai-cache/tenant-generic-step-evidence-v1-1-g4-full-release-verification-2026-09-18/00-pointer.md
  CLASSIFICATION=G COMMIT_POLICY=INCLUDE
- PATH=cobo_iam_services/docs/ai-cache/tenant-generic-step-evidence-v1-1-g5-final-precommit-review-2026-09-18/**
  CLASSIFICATION=G COMMIT_POLICY=INCLUDE
- PATH=cobo_iam_services/docs/ai-cache/README.md
  CLASSIFICATION=G COMMIT_POLICY=INCLUDE

## SCREENSHOTS
- PATH=cobo_web_design/docs/ai-cache/tenant-generic-step-evidence-v1-1-g3-tenant-ux-2026-09-18/screenshots/*.png
  CLASSIFICATION=H COMMIT_POLICY=INCLUDE
- PATH=cobo_web_design/docs/ai-cache/tenant-generic-step-evidence-v1-1-g4-full-release-verification-2026-09-18/screenshots/*.png
  CLASSIFICATION=H COMMIT_POLICY=INCLUDE

USER_COMMIT_MANIFEST_COMPLETE=true
UNRELATED_CHANGE_INCLUDED_IN_USER_COMMIT_MANIFEST=false
