# 00 — Target identification

MODE: SOURCE_AND_DEV_DATA_READ_ONLY  
DATE: 2026-08-25 (investigation ~16:28 HCM)

## Worktree

```text
CURRENT_WORKTREE_STATE=
cobo_iam_services: dirty deploy-artifacts web dist only (unrelated); no application source edits this audit
APPLICATION_SOURCE_CHANGED=false
DEV_DATA_MUTATED=false
```

## Exact name match

```text
EXACT_NAME_MATCH_COUNT=1
TARGET_TEMPLATE_AMBIGUOUS=false
TARGET_TEMPLATE_NAME=Bảng tính lương nhân viên ngày
TARGET_TEMPLATE_ROOT_ID=bang-tinh-luong-nhan-vien-thang-ban-sao
TARGET_ACTIVE_VERSION_NO=1
TARGET_ACTIVE_VERSION_ID=(version_no=1 on type_id above; no separate version UUID in schema)
TARGET_COMPANY_SCOPE=NULL (global / platform type)
TARGET_ACTIVATED_AT=2026-08-25 09:13:31 UTC ≈ 2026-08-25 16:13:31 HCM
ACTIVATION_TIME_SOURCE=disclosure_type_versions.created_at (active version row; MySQL NOW()≡UTC)
```

Note: `type_id` slug contains `thang` (month) but persisted `frequency_unit=daily` and UI name is ngày — identity is by exact name + unique type_id.