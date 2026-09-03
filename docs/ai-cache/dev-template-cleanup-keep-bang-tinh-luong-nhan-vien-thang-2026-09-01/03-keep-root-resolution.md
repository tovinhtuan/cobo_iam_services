# KEEP root resolution

## Exact name match

```sql
WHERE TRIM(dtv.name) = 'Bảng tính lương nhân viên tháng'
  AND dtv.version_no = dt.active_version_no
```

| Field | Value |
|-------|-------|
| KEEP_TEMPLATE_NAME | Bảng tính lương nhân viên tháng |
| KEEP_TEMPLATE_ROOT_ID | `bang-tinh-luong-nhan-vien-ban-sao-2` |
| KEEP_TEMPLATE_STATUS | `active` |
| KEEP_ACTIVE_VERSION_NO | `1` |
| KEEP_VERSION_COUNT | `1` |

```text
EXACT_KEEP_NAME_MATCH_COUNT=1
KEEP_ROOT_AMBIGUOUS=false
KEEP_ROOT_ID_NOT_IN_DELETE_SET=true
```

## Rejected similar names (not exact)

- `bang-tinh-luong-nhan-vien-thang-ban-sao-2` — suffix `(bản sao)`
- `bang-tinh-luong-nhan-vien-ban-sao` — different name
- `bang-tinh-luong-nhan-vien` — shorter name (no "tháng")
- `bang-tinh-luong-nhan-vien-thang-ban-sao` — "ngày" not "tháng"

## KEEP baseline (must remain unchanged)

| Metric | Count |
|--------|------:|
| versions | 1 |
| periodic_cycles | 8 |
| disclosure_records | 8 |
| global_workflows | 0 |
| company_type_preferences | 0 |
| disclosure_template_blocks | 6 |
| template_display_groups | 2 |
| company_template_workflow_overrides | 0 |

```text
KEEP_DATA_COLLATERAL_IMPACT=false
SHARED_MASTER_DATA_DELETE_RISK=false
```
