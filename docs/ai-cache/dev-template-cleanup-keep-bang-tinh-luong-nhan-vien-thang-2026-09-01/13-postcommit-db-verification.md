# 13 — Post-commit DB verification (fresh session)

## Roots

```text
FINAL_TEMPLATE_ROOT_COUNT=1
FINAL_TEMPLATE_ROOT_ID=bang-tinh-luong-nhan-vien-ban-sao-2
FINAL_TEMPLATE_NAME=Bảng tính lương nhân viên tháng
FINAL_TEMPLATE_NAME_MATCH=PASS
```

## KEEP baseline preserved

| Metric | Value |
|--------|------:|
| versions | 1 |
| cycles | 8 |
| records | 8 |
| template_blocks | 6 |
| display_groups | 2 |
| status | active |
| active_version_no | 1 |

Same cycle/record/display-group IDs as pre-delete snapshot.

```text
KEEP_TEMPLATE_ROOT_PRESERVED=true
KEEP_TEMPLATE_ACTIVE_VERSION_UNCHANGED=true
KEEP_VERSION_BASELINE_UNCHANGED=true
KEEP_EXISTING_DATA_PRESERVED=true
ORPHAN_ROWS_AFTER_DELETE=0
GLOBAL_MASTER_DATA_CHANGED=false
```
