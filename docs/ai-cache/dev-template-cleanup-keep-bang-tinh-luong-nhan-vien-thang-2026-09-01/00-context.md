# Context — DEV template cleanup

**Date:** 2026-09-01

## GOAL

Xóa tất cả template DEV và chỉ giữ exact business name:

```text
Bảng tính lương nhân viên tháng
```

## Flags

```text
DESTRUCTIVE_OPERATION=true
DEV_ONLY=true
PRODUCTION=false
DELETE_EXECUTED=false
```

## Phase

Audit + dry-run + delete plan only. **No DB mutation in this cycle.**
