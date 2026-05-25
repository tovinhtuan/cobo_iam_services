# Unicode mojibake — FE disclosure detail & DB repair

## Symptom

`/app/history/:id` (`DisclosureDetail`) showed garbled Vietnamese labels (`Lá»‹ch sá»­`, `KhÃ´ng`, …) while other screens were fine.

## Root cause

Hard-coded strings in `DisclosureDetail.tsx` / `DisclosureTypeDetail.tsx` were saved as **UTF-8 misread as Latin-1** (file-level mojibake), not an API encoding bug.

## Fixes (2026-05-25)

| Layer | Change |
|-------|--------|
| FE | Restored UTF-8 Vietnamese in both TSX files |
| CI guard | `cobo_web_design/scripts/check-mojibake.sh` + `npm run check:mojibake` + `make fe-check-mojibake` |
| DB (draft) | `0075_fix_unicode_mojibake_disclosure_text.up.sql` — repair garbled `disclosure_records`, active `disclosure_type_versions`, `ad_hoc_proposals`, `global_workflow_steps` |

## Run checks

```bash
cd cobo_web_design && npm run check:mojibake
# or from cobo_iam_services:
make fe-check-mojibake
```

## Apply DB repair (optional)

```bash
make push-migration FILE=0075_fix_unicode_mojibake_disclosure_text.up.sql
```

Review on staging before production; down migration is no-op.
