# 03 — Active periodic template inventory

SEED_TEMPLATE_SELECTION_PREDICATE (ListActivePeriodicTypes):

```text
dt.status = 'active'
AND template_category IN ('periodic','custom')
AND frequency_unit IN (daily|weekly|monthly|quarterly|yearly + aliases)
AND join active_version_no
```

```text
ACTIVE_PERIODIC_TEMPLATE_COUNT=14
ACTIVE_DAILY=1
ACTIVE_WEEKLY=0
ACTIVE_MONTHLY=4
ACTIVE_QUARTERLY=4
ACTIVE_YEARLY=5
```

All 14 are global (`company_id IS NULL`).

| type_id | name | freq | AF mode/slot |
| --- | --- | --- | --- |
| bang-tinh-luong-nhan-vien-thang-ban-sao | Bảng tính lương nhân viên ngày | daily | CURRENT_SLOT/2026-08-25 |
| bang-tinh-luong-nhan-vien | Bảng tính lương nhân viên | monthly | legacy empty |
| bang-tinh-luong-nhan-vien-ban-sao | Bảng tính lương nhân viên (bản sao) | monthly | legacy; anchor_day=31 |
| bang-tinh-luong-nhan-vien-ban-sao-2 | Bảng tính lương nhân viên tháng | monthly | CURRENT_SLOT/2026-08 |
| bao-cao-tai-chinh-thang | Báo cáo tài chính tháng | monthly | legacy |
| bao-cao-tai-chinh-quy | Báo cáo tài chính quý | quarterly | NEXT_SLOT/2026-Q4 |
| qa-model-a-financial-20260820-1509 | QA Model A … | quarterly | legacy |
| qa-template-real-user-smoke-20260821-0941 | QA Template … | quarterly | legacy |
| qa-ui-clone-20260821-1115 | QA UI Clone … | quarterly | legacy |
| bao-cao-tcq1 | Báo cáo tcq1 | yearly | legacy; month=9 |
| bao-cao-test-final (+ copies / tuần test) | yearly test set | yearly | legacy; month=9 |

```text
ACTIVE_COMPANIES=20
TEMPLATE_APPLICABILITY_STRICT_FILTER=true
COMPANIES_PASSING_APPLICABILITY≈4
  (have class flags + sectors): cskh_9bea, ctcp_nh_a_an_ph_t_xanh_5239, qa_manual_qr_pkg_…, c001
```

Existing `periodic_cycles` total=5 (all QA labels on bao-cao-tuan-test / c_001 — **not** current canonical slots).
