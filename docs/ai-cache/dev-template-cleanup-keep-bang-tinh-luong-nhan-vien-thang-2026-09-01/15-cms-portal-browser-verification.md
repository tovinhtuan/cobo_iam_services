# 15 — CMS / Portal browser verification

## Portal

| Check | Result |
|-------|--------|
| List URL | `/app/disclosure-types` |
| Visible cards | **1** |
| Name | Bảng tính lương nhân viên tháng |
| Detail | 200 / opens / guidance present |
| Authenticated API list count | **1** |
| Authenticated API detail | **200**, correct id+name |

Screenshots:
- `screenshots/portal-list-one-root.png`
- `screenshots/portal-keep-detail.png`

```text
PORTAL_BROWSER_VERIFY=PASS
KEEP_TEMPLATE_API_DETAIL=PASS
KEEP_EXISTING_RECORD_ACCESS=PASS (detail page healthy; 8 Draft records preserved)
NEW_BROWSER_CONSOLE_P0_P1=0
```

## CMS

Current browser session = enterprise admin (`Admin Doanh Nghiep`) → `/cms` redirected to forbidden (no `platform.cms.view`).

```text
CMS_BROWSER_VERIFY=NOT_RUN
CMS_TEMPLATE_ROOT_COUNT=1 (proven via fresh DB session + Portal/API list)
```

DB authority: `SELECT COUNT(*) FROM disclosure_types` = 1 after cleanup.
