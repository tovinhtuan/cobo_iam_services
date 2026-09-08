# Write Authority & Transaction Boundary

## Authoritative Write Path
- **Authority**: `s.UpsertTypeVersion(ctx, upsert)` with `CreateOnly = true`.
- Reused directly from existing CMS template service.
- Manages:
  1. Root row insertion in `disclosure_types`.
  2. Version row insertion in `disclosure_type_versions` (`version_no=1`).
  3. Template blocks insertion in `disclosure_template_blocks`.
  4. Template display group associations in `template_display_groups`.
  5. Workflow manifest serialization and Model A hash computation (`workflow_semantic_hash`, `publication_candidate_hash`).
  6. Legal bases normalization and association.
  7. Atomic database transaction (`BeginTx` -> operations -> `Commit` / `Rollback`).

## Hard Invariants
- `EXISTING_ROOT_UPDATE_SUPPORTED = false`: Fails if template root already exists.
- `AUTO_ACTIVATE = false`: `active_version_no = 0`, `activated_at = NULL`.
- `AUTO_PUBLISH = false`: `is_released = false`.
- `PORTAL_STATE = not_active`.
- `PORTAL_ACTIVE_AFTER_IMPORT = false`: Imported template does not appear in Portal active lists.
- `TEMPLATE_AGGREGATE_MATERIALIZATION_ATOMIC = true`.
- Zero parallel manual SQL insert paths.
