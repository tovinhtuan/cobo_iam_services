# 00 — Discovery: CMS Template single-draft overwrite

## Root cause

`internal/disclosure/infra/mysql/repository.go` → `UpsertTypeVersion`:

```go
SELECT COALESCE(MAX(version_no), 0) FROM disclosure_type_versions WHERE type_id = ?
nextVersion := max + 1
INSERT INTO disclosure_type_versions (...)
```

Mỗi lần **Lưu nháp** (FE `upsertDisclosureType` → `PUT .../disclosure-types/{typeId}`) tạo **row version mới**. Không có khái niệm overwrite draft.

## Data model hiện tại

| Entity | Role |
|--------|------|
| `disclosure_types.active_version_no` | Portal active |
| `disclosure_type_versions` | Mỗi lần save = 1 row `(type_id, version_no)` |
| `is_active` (computed) | `version_no = active_version_no` |
| Không có `state=draft` | Khác với company workflow override (đã có `state=draft`) |

## APIs

| Action | API |
|--------|-----|
| Lưu nháp | `PUT /api/v1/admin/disclosure-types/{typeId}` → UpsertTypeVersion |
| List versions | `GET .../disclosure-types/{typeId}/versions` → ListTypeVersions |
| Activate | `POST .../versions/{n}/activate` → ActivateTypeVersion |

## FE

- `useTemplateEditor.save` → `api.upsertDisclosureType`
- `TemplateVersionPanel` labels mọi non-active là "Draft" → spam v10…v2

## Constraints

- PK `(type_id, version_no)` — không unique trên "một draft"
- MySQL không partial unique dễ — dùng service-level guard

## Verdict discovery

READY_TO_IMPLEMENT — model rõ, fix tại Upsert + list filter + optional `is_released`.
