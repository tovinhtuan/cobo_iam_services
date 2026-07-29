# Phase 12.6A — Schema map

## Tables

### `disclosure_types`

| Column | Type (migration) | Notes |
| --- | --- | --- |
| type_id | VARCHAR(64) PK | Business id |
| company_id | VARCHAR(64) NULL | NULL = global |
| group_id | VARCHAR(64) | FK groups |
| active_version_no | INT | Portal pointer |
| status | VARCHAR(32) | `active` / `archived` / … |

### `disclosure_type_versions`

| Column | Type | Notes |
| --- | --- | --- |
| type_id, version_no | composite PK | |
| legal_basis | TEXT NULL | Compatibility flat |
| legal_bases_json | JSON NULL | Canonical structured |
| is_released | TINYINT(1) | Snapshot vs open draft |
| change_note, updated_by, … | … | Out of LB inventory |

## DEV vs source

Source migrations define the above. **Live DEV metadata SELECT not executed** (no RO session).  
When RO access is available, CLI must verify columns via `information_schema` before scan; mismatch → `BLOCKED_SCHEMA_CONFLICT`.

## Indexes used for inventory

Keyset: `ORDER BY type_id, version_no` on primary key — efficient pagination.
