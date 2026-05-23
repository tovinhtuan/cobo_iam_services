# Portal Template CMS Phase 1 Backend Foundation Summary

- Created: 2026-05-23
- Updated: 2026-05-23
- Created by: Codex
- Valid for: Team reuse, Phase 2 handoff, backend authz/validation review

## Objective

Land the safe backend foundation slice for CMS Phase 1 without jumping into Phase 2 editor/save-contract refactors.

## Implemented

- Added additive migration `0058_cms_template_permissions` for canonical CMS template permissions:
  - `cms.template.read`
  - `cms.template.write`
  - `cms.template.activate`
  - `cms.template.archive`
  - `cms.template.config.write`
- Added CMS authz helpers in disclosure service so admin endpoints no longer depend on scattered raw checks.
- Rewired admin template endpoints to use canonical permissions with legacy fallback:
  - version detail
  - reference data
  - upsert
  - version list
  - activate
  - config read
  - config update
- Added `deadline_rule_catalog` to template reference data and DB/in-memory loaders for it.
- Hardened company-defined template create/update validation with the target `periodic|irregular` matrix while preserving update-merge behavior for omitted fields.

## Files

- `internal/disclosure/app/service.go`
- `internal/disclosure/app/contracts.go`
- `internal/disclosure/app/cms_template_permissions.go`
- `internal/disclosure/app/cms_template_reference_data.go`
- `internal/disclosure/app/cms_template_authz_test.go`
- `internal/disclosure/app/cms_template_validation_test.go`
- `internal/disclosure/infra/mysql/repository.go`
- `internal/disclosure/infra/inmemory/repository.go`
- `migrations/0058_cms_template_permissions.up.sql`
- `migrations/0058_cms_template_permissions.down.sql`

## Verification

- `go test ./internal/disclosure/...`: pass
- `go build ./cmd/api`: pass
- `docker compose -f docker-compose.dev.yml build api`: blocked because Docker daemon was not running

## Constraints / Decisions

- Did not flip legacy CMS admin save validation to the strict new vocabulary yet; that belongs to Phase 2 with FE/editor alignment.
- Used compatibility authz to avoid breaking current operators before role assignments are migrated.
- Kept config read under CMS read capability, while config update uses config-write compatibility gate.

## Next Recommended Step

Move to Phase 2 only after explicit user confirmation, then tackle:

- `CMS-BE-TEMPLATE`
- `CMS-BE-CATALOG`
- `CMS-BE-DEADLINE`
- `CMS-FE-ARCH`
- `CMS-FE-AUTH`
- `CMS-FE-PERM`
- `CMS-FE-LIST`
- `CMS-FE-EDITOR`
