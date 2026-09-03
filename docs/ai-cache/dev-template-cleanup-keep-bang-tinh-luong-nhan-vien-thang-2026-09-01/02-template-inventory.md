# Template inventory — DEV `cobo_iam`

**Snapshot:** 2026-09-01 (read-only SQL)

```text
CURRENT_TEMPLATE_ROOT_COUNT=42
KEEP_COUNT=1
DELETE_CANDIDATE_COUNT=41
DELETE_SET_COMPLETE=true (42 = 1 + 41)
```

## Payroll-related names (similar-name audit)

| type_id | active name | decision |
|---------|-------------|----------|
| `bang-tinh-luong-nhan-vien` | Bảng tính lương nhân viên | DELETE |
| `bang-tinh-luong-nhan-vien-ban-sao` | Bảng tính lương nhân viên (bản sao) | DELETE |
| `bang-tinh-luong-nhan-vien-thang-ban-sao` | Bảng tính lương nhân viên ngày | DELETE |
| **`bang-tinh-luong-nhan-vien-ban-sao-2`** | **Bảng tính lương nhân viên tháng** | **KEEP** |
| `bang-tinh-luong-nhan-vien-thang-ban-sao-2` | Bảng tính lương nhân viên tháng (bản sao) | DELETE |

## Full inventory summary

| Category | Count |
|----------|------:|
| Active portal-visible roots (`active_version_no > 0`) | 31 |
| Hidden/invalid active pointer (`active_version_no = 0`) | 11 |
| Archived status | 1 (`bao-cao-tai-chinh-test`) |
| QA ApplicableTo clones | 22 |
| Financial/report QA templates | 8 |
| Other QA clones | 5 |

## DELETE root IDs (41)

See `_delete_root_ids.sql` output — all `disclosure_types.type_id` except `bang-tinh-luong-nhan-vien-ban-sao-2`.

**Note:** Several QA roots have `active_version_no=0` (already hidden from Portal) but rows still exist in `disclosure_types` / versions.
