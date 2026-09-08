# Mutable Target References Revalidation

## Catalog Drift Protection
The validation token has a 15-minute TTL. During this window, target database catalogs could change (e.g. departments renamed or deleted, display groups removed).
`ConfirmTemplateImport` revalidates current target state before materialization:

## Department Revalidation
- `DEPARTMENT_REVALIDATION_ON_CONFIRM = true`
- Calls `s.repo.ListTemplateDepartments(ctx)` in bulk (1 query, zero N+1).
- Builds in-memory lookup maps by lowercase department code and lowercase department name.
- Ensures all workflow step department references and explicit department mapping targets exist in the current target catalog.
- `CONFIRM_DEPARTMENT_N_PLUS_ONE = false`

## Display Group Revalidation
- `DISPLAY_GROUP_REVALIDATION_ON_CONFIRM = true`
- Calls `s.repo.ListDisplayGroups(ctx)` in bulk (1 query, zero N+1).
- Validates that every display group code in `NormalizedTemplate.DisplayGroupCodes` exists in the current target catalog.
- If any code is missing/deleted, Confirm rejects with HTTP 400 Bad Request and ZERO DB writes.
- `CONFIRM_DISPLAY_GROUP_N_PLUS_ONE = false`

## Role Revalidation
- `ROLE_REVALIDATION_ON_CONFIRM = false`
- Assignee role IDs are compile-time static / schema-validated strings (e.g. `creator`, `approver`, `publisher`).
- Preserved directly from token-bound normalized workflow.
