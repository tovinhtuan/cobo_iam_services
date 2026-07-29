# Phase 12.6A — Schema map (live DEV)

## Tables (verified via `information_schema` + DESCRIBE on DEV)

### `disclosure_types`

| Column | Notes |
| --- | --- |
| type_id | PK |
| company_id | NULL = global |
| status | e.g. active |
| active_version_no | portal pointer |

### `disclosure_type_versions`

| Column | Live DEV |
| --- | --- |
| type_id, version_no | composite PK |
| legal_basis | TEXT NULL — present |
| legal_bases_json | JSON NULL — present |
| is_released | **missing** (0122 not applied) |

## Inventory adaptation

- Required LB columns present → proceed.
- Missing `is_released` → SELECT literal `0 AS is_released`, set `IsReleased = (version_no == active_version_no)` in analyzer input with schema note.
- No schema mutation / migration run in 12.6A.

## Indexes

Keyset pagination: `ORDER BY type_id, version_no` on primary key.
