# Implementation

## BE
- `ListTypesParams.Scope` / request + handler query `scope`
- MySQL + inmemory predicates; invalid scope → 400

## FE
- `cmsApi.listDisclosureTypes({ scope, page, pageSize })`
- `useTemplatesList` → `scope: 'global'`
- Empty copy: Không có template toàn hệ thống phù hợp.
- Wire `successMessage` banner in TemplateEditorScreen

## Unchanged
- Portal listTypes (no scope)
- Runtime effective-workflow
- No DELETE / ownership rewrite / migration
