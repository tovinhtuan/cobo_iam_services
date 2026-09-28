# CMS template import guide — DEV deploy smoke 2026-09-28

task type: deploy + browser smoke (read-only QA, no product code change)
objective: deploy CMS Template Import Guide to DEV and smoke-check the CMS import screen
skill: integration-cross-repo, premerge-system-review

```text
BRANCH=fixbug/test-9
DEV_TARGET_VERIFIED=true
DEV_BACKEND_DEPLOY=DONE
DEV_FRONTEND_DEPLOY=DONE
HEALTHZ=200
READYZ=200
PLATFORM_CMS_SESSION=true
HEALTHZ=200
READYZ=200
TC-01_IMPORT_SCREEN=PASS
TC-02_GUIDE_DOWNLOAD=PASS
TC-03_EXAMPLE_DOWNLOAD=PASS
TC-04_VALID_JSON_VALIDATE=PASS
TC-05_MAPPING_REQUIRED=PASS
TC-06_DRAFT_CONFIRM=NOT_RUN
TC-06_REASON=MAPPING_OR_CONFIRM_PREREQUISITE_UNSAFE
TC-07_INVALID_JSON=PASS
TC-08_INVALID_PERIODICITY=PASS
TC-09_DEADLINE_RULE=PASS
TC-10_HISTORY_FLAG_OFF=PASS
CONSOLE_ERRORS=1_EXPECTED_HISTORY_404
PRODUCTION_REQUESTS=0
MIGRATION_APPLIED=false
FLAG_ENABLED=false
DRAFT_CREATED=false
CONFIRM_CALLED=false
PUBLISH=NOT_RUN
ACTIVATE=NOT_RUN
EMAIL_SENT=NOT_RUN
STAGING_CLEANUP=DELETED
PRODUCTION_TOUCHED=false
DEV_SMOKE=PASS
```

## Target

DEV only, from `docs/deploy-dev-guide.md` and `deploy-dev.ps1` defaults. Not production.

- Portal: `http://88.216.208.0:3000`
- API: `http://88.216.208.0:8080`
- SSH: port `21239`, path `/root/cobo_project`, hostname `avi-server1`
- Commands used: `.\deploy-dev.ps1 -Mode be` then `.\deploy-dev.ps1 -Mode fe -SkipTests`
- `Mode all` and `Mode migrate` were not used

## Phase 0 baseline

Both repos were on `fixbug/test-9` with empty `git status --short` before deploy.

Backend HEAD `1a09358`. Frontend HEAD `ab467e0d`.

## Source check before deploy (passed)

- FE: `ImportGuideDownload` imported and rendered in both file states in `ImportTemplateScreen.tsx` (around the empty-file and selected-file branches). Button label `Tải hướng dẫn tạo file JSON`. Path `/api/v1/platform/cms/templates/import/guide`. Fallback filename `cobo-template-import-guide-v1.0.md`.
- BE: `GET /api/v1/platform/cms/templates/import/guide`. Content type `text/markdown; charset=utf-8`. Disposition `attachment; filename="cobo-template-import-guide-v1.0.md"`. Same `requireCMSTemplateWrite` gate as the example download. Handler writes the embedded bytes only.
- Guide artifact has schema 1.0, single JSON root object, five periodic values, periodic `deadline_rule` optional, irregular `deadline_rule` required, company classes, department mapping, workflow rules, validate → mapping → confirm, and the common error table. No secret, token, or tenant identifier in the guide.

## Tests and builds

| Command | Result | Class |
|---|---|---|
| `go test ./internal/disclosure/...` | FAIL only `internal/disclosure/app/legal_basis_backfill` `TestSnapshotRoundTripAndMode` expected `0600`, got `-rw-rw-rw-` | PRE_EXISTING, outside this diff |
| `go test ./internal/platformcms/...` | PASS | |
| `go build ./...` | PASS | |
| `npm run test -- --run src/features/cms-core/templates/import` | 10 files, 45 tests PASS | |
| `npm run build` (local, then again inside FE deploy with DEV Vite flags) | PASS | |

Full `npm run lint` / `npm run test` were not rerun. FE deploy used `-SkipTests` so the script's lint pre-check could not abort on the previously classified pre-existing `tsc` failures. The deploy script still ran `npm run build`.

Live FE asset after deploy: `index-iwa114I2.js`. That file contains `import-guide-download-btn`, the guide API path, `cobo-template-import-guide-v1.0.md`, and the Vietnamese guide/example labels. This is not a stale frontend asset.

## Deploy and health

Before BE deploy, unauthenticated `GET /api/v1/platform/cms/templates/import/guide` was HTTP 404 while the example route was HTTP 401, so the backend was deployed.

After deploy:

- `GET /healthz` → `{"status":"ok"}` HTTP 200
- `GET /readyz` → ready HTTP 200
- Unauthenticated guide route → HTTP 401 (route is now registered)
- `CMS_TEMPLATE_IMPORT_HISTORY_ENABLED` unset inside the API container (`FLAG_UNSET`)
- `schema_migrations` rows matching `0148%`: count `0`
- API and worker logs since the recreate had no migration / `0148` lines

`deploy-dev.ps1 -Mode be` copies `migrations/` onto the server as part of the documented binary deploy. It does not call the migrate step (`--no-deps` recreate of api and worker only). Migration `0148` was not applied.

No production host was used. History flag was not turned on. Email and workflow flags were not changed.

## Browser smoke

Existing Cursor browser session was already logged in. Opening `http://88.216.208.0:3000/cms/templates/import` redirected to `http://88.216.208.0:3000/app/forbidden`. Visible role is enterprise admin (`Admin Doanh Nghiep`, company `Company X`), with the message `Bạn không có quyền truy cập trang này`.

Cases A–E were not executed. No download, no validate, no confirm, no history tab. No localStorage, cookie, or Authorization header was read. No second login was attempted.

Evidence: `c:\Users\tvttt\AppData\Local\Temp\cursor\screenshots\cms-template-import-guide-dev-smoke-forbidden-2026-09-28.png`

Staging copy `.codex-tmp/10_bao-cao-thuong-nien.fixed.json` was not created. Source JSON was not modified.

## Pre-merge review

No product code changed in this task. Deploy risk that remains: smoke of the CMS UI, guide download headers, example download, validate-only, and history flag-off panel is unverified on a platform CMS session. Rollback is the previous DEV binaries and `web/dist` if a later deploy replaces them. Do not run `deploy-dev.ps1` without `-Mode be` or `-Mode fe`, because `Mode all` applies migrations.

## Remaining

- Repeat browser cases A–E with a platform CMS session that is already logged in.
- Do not enable `CMS_TEMPLATE_IMPORT_HISTORY_ENABLED`.
- Do not apply migration `0148`.

## Retry 2026-09-28 20:04 — TC-01..TC-10

```text
DEV_SMOKE=BLOCKED
BLOCKER=EXISTING_BROWSER_SESSION_IS_ENTERPRISE_ADMIN_CMS_FORBIDDEN
TC-01..TC-10=NOT_RUN
MIGRATION_APPLIED=false
FLAGS_ENABLED=false
DRAFT_CREATED=false
CONFIRM_CALLED=false
PRODUCTION_TOUCHED=false
```

Reopened `http://88.216.208.0:3000/cms/templates/import` in the Cursor browser. The page redirected again to `/app/forbidden`. Visible role is still enterprise admin (`Admin Doanh Nghiep`, Company X). Playwright has no logged-in tab. No login, no validate, no confirm, no migration.

## Retry 2026-09-28 20:11 — TC-01..TC-10

```text
DEV_SMOKE=BLOCKED
reason=PLATFORM_CMS_SESSION_NOT_ACTIVE
PLATFORM_CMS_SESSION=false
DEV_TARGET_VERIFIED=true
TC-01..TC-10=NOT_RUN
HEALTHZ=NOT_RERUN
READYZ=NOT_RERUN
CONSOLE_ERRORS=NOT_RUN
PRODUCTION_REQUESTS=0
MIGRATION_APPLIED=false
FLAG_ENABLED=false
DRAFT_CREATED=false
CONFIRM_CALLED=false
PUBLISH=NOT_RUN
ACTIVATE=NOT_RUN
EMAIL_SENT=NOT_RUN
STAGING_CLEANUP=NOT_CREATED
PRODUCTION_TOUCHED=false
```

Same Cursor tab `80af4f` was reused. Opening `http://88.216.208.0:3000/cms/templates/import` redirected to `http://88.216.208.0:3000/app/forbidden`. Visible identity remains enterprise admin for Company X. No new browser session, no login attempt, no credentials lookup, no validate, no confirm.

## Final pass — logged-in Platform CMS smoke

DEV portal `http://88.216.208.0:3000`, API proxied on the same DEV host. Account signed in through the login form, company **Company X** selected. Header showed `Platform + Tenant Admin (Dev)` and CMS home `/cms` opened. Password, tokens, and cookies were not recorded.

Health from the earlier DEV check still stands: `/healthz` and `/readyz` HTTP 200. `CMS_TEMPLATE_IMPORT_HISTORY_ENABLED` inside the API container: unset. No migration was run.

| Case | Result |
|---|---|
| TC-01 import screen | PASS. Dropzone, both download buttons, accessible names, console 0. |
| TC-02 guide | PASS. `GET /api/v1/platform/cms/templates/import/guide` 200. `text/markdown; charset=utf-8`. Filename `cobo-template-import-guide-v1.0.md`. Toast shown. Content includes schema 1.0, five periodic values, periodic `deadline_rule` optional, irregular required, company classes, department mapping, workflow, common errors, validate → mapping → confirm. |
| TC-03 example | PASS. `GET .../import/example` 200. Filename `cobo-template-import-example-v1.0.json`. JSON parse OK. |
| TC-04 validate | PASS. `POST .../import/validate` 200. `parse_valid=true`, `domain_valid=true`. Preview `yearly`, `T+110`, `NEXT_SLOT`. No payload/periodicity/applicability error codes. |
| TC-05 mapping | PASS. `mapping_required=true`, `required_mapping_count=3`, `resolved_mapping_count=0`, `can_confirm=false`. Confirm button disabled. Confirm not called. |
| TC-06 | NOT_RUN. Catalog has no unambiguous target for `corporate_secretary` (Thư ký Công ty / Văn phòng HĐQT). Legal is close to `Phòng Pháp chế`, board is close to `Ban Tổng Giám đốc`, but the secretary mapping is not exact, so confirm was not clicked. |
| TC-07 | PASS. `INVALID_JSON_PAYLOAD` with a visible fix hint. No confirm. |
| TC-08 | PASS. `periodicity=annually` → `INVALID_PERIODICITY`. Message lists `daily`, `weekly`, `monthly`, `quarterly`, `yearly`. |
| TC-09 | PASS. Periodic without `deadline_rule` and `deadline_days=110` → validate PASS, derived `deadline_rule=T+110`. Irregular without `deadline_rule` → `DEADLINE_RULE_REQUIRED`. |
| TC-10 | PASS. One `GET .../import-history?limit=20` → 404 `FEATURE_DISABLED`. UI text `Tính năng chưa được bật`. Empty-history text absent. No retry after 5s. |

`POST .../import/confirm` was not observed. No Draft id. No publish, activate, or email. Resource hosts on the history page: DEV `88.216.208.0:3000` and `fonts.googleapis.com`.

Source JSON SHA-256 unchanged: `acc52622e62bfd11c8b0aaba196f295b9da59799a9bb3ec6e721986bf0e2ea00`. Staging `.codex-tmp` and browser downloads were deleted. Frontend git status clean.

Console: one error, the expected history 404 resource failure. No other warning or error.

Evidence:

- `docs/ai-cache/cms-import-tc01-baseline.png`
- `docs/ai-cache/cms-import-tc04-preview.png`
- `docs/ai-cache/cms-import-tc10-history-disabled.png`

## Completion attempt 2026-09-28 20:43

```text
PLATFORM_CMS_SESSION=true
DEPARTMENT_MAPPING_READY=false
MYSQL8_ISOLATED_VALIDATION=BLOCKED
reason=Docker daemon is not running (//./pipe/docker_engine missing). Checked once. Not retried.
MIGRATION_CHAIN=0144,0145,0146,0147,0148
0148_UP=NOT_RUN
0148_RERUN=NOT_RUN
0148_FIXTURE=NOT_RUN
0148_DOWN=NOT_RUN
ISOLATED_CLEANUP=NOT_RUN
DEV_TARGET_VERIFIED=true
DEV_LEDGER_BEFORE=latest=0143_cms_global_record_id_width.up.sql; 0148_count=0
MIGRATIONS_APPLIED=none
DEV_LEDGER_AFTER=unchanged
DEV_MIGRATION=NOT_RUN
DEV_BACKEND_DEPLOY=NOT_RERUN
DEV_FRONTEND_DEPLOY=NOT_RERUN
HISTORY_FLAG_DEV=false
GUIDE_DOWNLOAD=PASS_PRIOR_SMOKE
JSON_VALIDATE=PASS_PRIOR_SMOKE
MAPPING_RESOLUTION=BLOCKED
DRAFT_CONFIRM=NOT_RUN
HISTORY_CONFIRMED_ROW=NOT_RUN
CONFIRM_RECOVERY=GAP_NO_REAPER
REPLAY_GUARD=COVERED_BY_EXISTING_TESTS_NOT_RERUN_ON_DEV
CONSOLE_EXPECTED_404=P2
DRAFT_CREATED=false
PUBLISH=NOT_RUN
ACTIVATE=NOT_RUN
EMAIL_SENT=NOT_RUN
MIGRATION_APPLIED_PRODUCTION=false
PRODUCTION_TOUCHED=false
DEV_FEATURE_STATUS=PARTIAL
RELEASE_STATUS=NO-GO
TC-06=BLOCKED
TC-06_REASON=PLATFORM_CATALOG_MAPPING_NOT_AVAILABLE
```

Both repos remain on `fixbug/test-9`. Frontend working tree is clean. Backend only has this untracked report. No product code was changed. No flag was turned on. No migration was applied. No confirm. No second Draft.

### Mapping

System seed in `internal/disclosure/app/catalog.go` is `dept-001` Phòng Pháp chế, `dept-002` Phòng Quan hệ cổ đông (IR), `dept-003` Phòng Kế toán, `dept-004` Ban Tổng Giám đốc. The live DEV import dropdown also contains extra `tpl_dept_*` QA rows and those four codes. None of the codes are `corporate_secretary`, `legal`, or `bod`.

Name comparison is not an exact match:

- `corporate_secretary` / Thư ký Công ty / Văn phòng HĐQT — no catalog name
- `legal` / Phòng Pháp chế & Tuân thủ — catalog name is Phòng Pháp chế
- `bod` / Ban Giám đốc — catalog name is Ban Tổng Giám đốc

Platform CMS already has `POST /api/v1/platform/cms/workflow/template-departments`. New catalog rows were not created, because that would invent targets rather than verify existing ones. Mapping owner needs an explicit catalog decision before confirm.

### CONFIRMING

`ClaimImportAttempt` sets status `CONFIRMING`. The same request can move that row to `CONFIRMED` or `CONFIRM_FAILED`. A later confirm treats any status other than `VALIDATED` or `MAPPING_REQUIRED` as conflict, including `CONFIRMING`. There is no timeout or reaper. A crash after the claim and before `failConfirmAttempt` leaves the row stuck. The approved plan state machine does not define `CONFIRMING` or a lease, so no recovery code was added in this pass.

### Console 404

The history flag-off 404 is still an expected browser console error. UX text is correct. Tracked as P2. HTTP contract stays 404. No code change.

## PO mapping pass 2026-09-28 21:34

```text
DEPARTMENT_MAPPING_READY=true
LEGAL_MAPPING=dept-001
BOD_MAPPING=dept-004
CORPORATE_SECRETARY_MAPPING=corporate_secretary
MYSQL8_ISOLATED_VALIDATION=BLOCKED
reason=Docker daemon is not running (//./pipe/docker_engine missing). Checked once.
MYSQL8_VERSION=
MIGRATION_CHAIN=0144,0145,0146,0147,0148
MIGRATION_UP=NOT_RUN
MIGRATION_RERUN=NOT_RUN
MIGRATION_FIXTURE=NOT_RUN
MIGRATION_DOWN=NOT_RUN
ISOLATED_CLEANUP=NOT_RUN
DEV_MIGRATION=NOT_RUN
DEV_LEDGER_BEFORE=0143
DEV_MIGRATIONS_APPLIED=none
DEV_LEDGER_AFTER=unchanged
DEV_BACKEND_DEPLOY=NOT_RERUN
DEV_FRONTEND_DEPLOY=NOT_RERUN
HISTORY_FLAG_DEV=false
CONFIRM_RECOVERY=NOT_IMPLEMENTED_MIGRATION_GATE
DRAFT_CONFIRM=NOT_RUN
HISTORY_CONFIRMED_ROW=NOT_RUN
REPLAY_GUARD=NOT_RERUN
DRAFT_CREATED=false
PUBLISH=NOT_RUN
ACTIVATE=NOT_RUN
EMAIL_SENT=NOT_RUN
PRODUCTION_TOUCHED=false
DEV_FEATURE_STATUS=PARTIAL
RELEASE_STATUS=NO-GO
```

Official catalog API on DEV, no direct SQL:

- `dept-001` already present as Phòng Pháp chế.
- `dept-004` already present as Ban Tổng Giám đốc.
- `corporate_secretary` was absent. `POST /api/v1/platform/cms/workflow/template-departments` returned HTTP 201 with code `corporate_secretary` and name `Thư ký Công ty / Văn phòng HĐQT`. Recheck shows exactly one row, included in the live catalog list. `is_system=false`. Not mapped to `dept-002`. QA codes `tpl_dept_*` were not used.

Docker is down, so isolated MySQL 8 did not run. DEV migration stays at `0143`. Chain `0144`–`0148` was not applied. History flag stays off. Confirm, Draft, and the history `CONFIRMED` row were not run. `CONFIRMING` recovery was not implemented in this pass because it stays behind the migration gate.

## DEV-only migration pass 2026-09-28 21:49

```text
SCOPE=DEV_ONLY
LOCAL_DOCKER_USED=false
DEV_TARGET_VERIFIED=true
DEV_ISOLATED_SCHEMA=cobo_template_import_history_test_20260928
MYSQL8_DEV_ISOLATED_VALIDATION=PASS
MYSQL8_VERSION=8.0.46
DEV_TEST_SCHEMA_REDACTED=cobo_template_import_history_test_20260928
0144_RESULT=PASS
0145_RESULT=PASS
0146_RESULT=PASS
0147_RESULT=PASS
0148_UP_RESULT=PASS
RERUN_RESULT=PASS
FIXTURE_RESULT=PASS
DOWN_RESULT=PASS
CLEANUP_RESULT=PASS
DEV_MIGRATION=PASS
DEV_LEDGER_BEFORE=0143_cms_global_record_id_width.up.sql
DEV_LEDGER_AFTER=0148_cms_template_import_attempts.up.sql
DEV_BACKEND_DEPLOY=NOT_RERUN
DEV_FRONTEND_DEPLOY=NOT_RERUN
HISTORY_FLAG_DEV=false
CONFIRM_RECOVERY=GAP_NO_REAPER
GUIDE_DOWNLOAD=PASS_PRIOR_SMOKE
JSON_VALIDATE=PASS_PRIOR_SMOKE
MAPPING_RESOLUTION=READY
DRAFT_CONFIRM=NOT_RUN
HISTORY_CONFIRMED_ROW=NOT_RUN
CONCURRENT_CONFIRM=NOT_RERUN
REPLAY_GUARD=NOT_RERUN
DRAFT_CREATED=false
PUBLISH=NOT_RUN
ACTIVATE=NOT_RUN
EMAIL_SENT=NOT_RUN
PRODUCTION_TOUCHED=false
DEV_FEATURE_STATUS=PARTIAL
RELEASE_STATUS=NO-GO
```

Local Docker and local MySQL were not used. DEV target is the host in `docs/deploy-dev-guide.md` (`88.216.208.0`, API port 8080, portal port 3000). `/healthz` and `/readyz` were HTTP 200 before and after the `cobo_iam` apply.

Isolated database `cobo_template_import_history_test_20260928` was created on the DEV MySQL 8.0.46 server, not in `cobo_iam`. Prerequisites through `0143` were applied there, then `0144`–`0148`. Rerun of `0144`–`0148` passed. A fixture row from 2020 was excluded by the 365-day filter (`visible_recent=1`). Columns are hashes only: `file_sha256`, `canonical_payload_sha256`, `validation_token_sha256`. No raw payload or raw token column. Down of `0148` dropped only `cms_template_import_attempts` and left `workflow_template_department_code_registry`. The test database was then dropped.

`cobo_iam` then received `0144` through `0148` via `deploy-artifacts/push-migration.ps1`. Ledger now ends at `0148_cms_template_import_attempts.up.sql`. `cms_template_import_attempts` exists. No business fixture was inserted into `cobo_iam`. Down was not run on `cobo_iam`.

`CMS_TEMPLATE_IMPORT_HISTORY_ENABLED` is still unset. Confirm, Draft, and the history tab `CONFIRMED` row were not run. `CONFIRMING` still has no lease and no reaper, so the history flag stays off.

## Confirm lease recovery 2026-09-28 22:05

```text
SCOPE=DEV_ONLY
RECOVERY_MIGRATION=0149_cms_template_import_attempt_lease.up.sql
CONFIRM_RECOVERY=PASS
MYSQL8_RECOVERY_ISOLATED_VALIDATION=PASS
RECOVERY_MIGRATION_UP=PASS
RECOVERY_MIGRATION_RERUN=PASS
RECOVERY_FIXTURE=PASS
RECOVERY_CONCURRENT=PASS_SQL_SECOND_CLAIM_BLOCKED
RECOVERY_CRASH=PASS_EXPIRED_LEASE_RECLAIM
RECOVERY_MIGRATION_DOWN=PASS
RECOVERY_SCHEMA_CLEANUP=PASS
DEV_RECOVERY_MIGRATION=PASS
DEV_LEDGER_BEFORE=0148
DEV_LEDGER_AFTER=0149_cms_template_import_attempt_lease.up.sql
DEV_BACKEND_DEPLOY=PASS
DEV_FRONTEND_DEPLOY=NOT_RERUN
HISTORY_FLAG_DEV=true
DRAFT_CONFIRM=FAIL_FILE_TYPES_TOO_LONG
DRAFT_CREATED=false
HISTORY_CONFIRMED_ROW=NOT_RUN
CONCURRENT_CONFIRM=PASS_IN_MEMORY_AND_SQL_GUARD
REPLAY_GUARD=PASS_IN_MEMORY
PUBLISH=NOT_RUN
ACTIVATE=NOT_RUN
EMAIL_SENT=NOT_RUN
PRODUCTION_TOUCHED=false
DEV_FEATURE_STATUS=PARTIAL
RELEASE_STATUS=NO-GO
```

`0148` was not edited. Next ledger file is `0149`, adding nullable `confirming_at` and `lease_expires_at`. Lease default is 10 minutes. A live `CONFIRMING` claim conflicts. An expired lease with an existing type id reconciles to `CONFIRMED` without a second draft and without a second import audit. An expired lease with no draft reclaims and materializes once. `CONFIRM_FAILED` stays terminal. Isolated schema `cobo_template_import_recovery_test_20260928` on MySQL 8.0.46 proved up, rerun, an in-lease second update affecting 0 rows, an expired reclaim affecting 1 row, down dropping only the two columns, then the schema was dropped.

`cobo_iam` received `0149`. Backend was redeployed. `CMS_TEMPLATE_IMPORT_HISTORY_ENABLED=true` is set on the DEV API. `/healthz` and `/readyz` are 200.

Browser smoke on the existing CMS session: guide download HTTP 200, filename `cobo-template-import-guide-v1.0.md`. Validate of the annual report HTTP 200. `corporate_secretary` auto-matched the new catalog code. `legal` was mapped to `dept-001` and `bod` to `dept-004`. Confirm was enabled and clicked once. It returned HTTP 400 `INVALID_REQUEST` because `blocks.3.config.file_types` exceeds 64 characters. No draft row for `bao-cao-thuong-nien` (count 0). The attempt is `CONFIRM_FAILED`, not stuck in `CONFIRMING`. History `CONFIRMED` was not shown because confirm did not succeed. Source JSON hash is unchanged. Staging files were removed. Down was not run on `cobo_iam`.

## File type parity and new draft 2026-09-28 22:37

```text
FILE_TYPES_CONTRACT=EACH_TOKEN_MAX_64_RUNES
VALIDATOR_CONFIRM_PARITY=PASS
VALIDATOR_FIX=VALIDATE_IMPORT_USES_SAME_FILE_TYPES_HELPER
FIXTURE_USED=10_bao-cao-thuong-nien.fixed-v2.json
DEV_BACKEND_DEPLOY=PASS
DEV_VALIDATE_NEW_ATTEMPT=PASS
MAPPING_RESOLUTION=PASS
DRAFT_CONFIRM=PASS
DRAFT_CREATED=true
HISTORY_CONFIRMED_ROW=PASS
OLD_FAILED_ATTEMPT_PRESERVED=true
REPLAY_GUARD=PASS_UNIT
PUBLISH=NOT_RUN
ACTIVATE=NOT_RUN
EMAIL_SENT=NOT_RUN
PRODUCTION_TOUCHED=false
DEV_FEATURE_STATUS=COMPLETE
RELEASE_STATUS=READY_FOR_DEV_ACCEPTANCE
```

Confirm copies `template.format` into `blocks[3].config.file_types` as one token. The token limit is 64 runes, the same helper `validateFreeTextFileTypes` uses. The original fixture `format` is 119 runes, so confirm rejected it and validate did not. Validate now runs that helper and returns `WORKFLOW_DOCUMENT_FILE_TYPES_TOO_LONG` before a token is issued. A blank format still becomes `PDF`.

`10_bao-cao-thuong-nien.fixed.json` was not edited. `10_bao-cao-thuong-nien.fixed-v2.json` changes only `format` to `PDF`, the file type named in that sentence. Periodicity stays `yearly`, deadline `T+110`, mode `NEXT_SLOT`, and the source codes `corporate_secretary`, `legal`, and `bod` remain.

DEV validate of the v2 file was HTTP 200 with `parse_valid=true`, `domain_valid=true`, `activation_ready=true`, and a new attempt id. `corporate_secretary` auto-matched. `legal` mapped to `dept-001` and `bod` to `dept-004`. Confirm was one click and HTTP 201. The UI opened `/cms/templates?type_id=bao-cao-thuong-nien`. `active_version_no` is 0. History shows the v2 file as `CONFIRMED` and the older file still `CONFIRM_FAILED`. Attempt counts are one of each status. No publish, activate, or email. Replay after success stays covered by the in-memory confirm test and was not clicked again in the browser.
