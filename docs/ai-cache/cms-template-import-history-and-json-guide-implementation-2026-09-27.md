# CMS template import history and JSON guide — implementation report

Date: 2026-09-27
Branch: `fixbug/test-9`

```text
BRANCH=fixbug/test-9
GUIDE_DOWNLOAD=IMPLEMENTED
SCHEMA_PARITY=IMPLEMENTED
HISTORY_MIGRATION=0148_CREATED_NOT_APPLIED
HISTORY_API=IMPLEMENTED
HISTORY_UI=IMPLEMENTED
TARGETED_TESTS=PASS
FULL_TESTS=PARTIAL
BUILD=GO_BUILD_PASS_NPM_BUILD_PASS_DOCKER_BLOCKED
MYSQL8_ISOLATED_VALIDATION=BLOCKED
DEV_TOUCHED=false
PRODUCTION_TOUCHED=false
FLAG_DEFAULT=false
RAW_JSON_STORED=false
RAW_TOKEN_STORED=false
PURGE_JOB=NOT_IMPLEMENTED_FOLLOW_UP

IMPLEMENTATION_PHASE=CONFIRM_AND_CONCURRENCY_TESTS_ADDED
MYSQL8_VERSION=
MIGRATION_UP=NOT_RUN
MIGRATION_RERUN=NOT_RUN
MIGRATION_DOWN=NOT_RUN
FLAG_OFF_REGRESSION=PASS
FLAG_ON_CONFIRM_SUCCESS=PASS
CONCURRENT_CONFIRM=PASS
REPLAY_GUARD=PASS
DOCKER_BUILD=BLOCKED
FRONTEND_LINT=PRE_EXISTING_FAILURE
GO_BUILD=PASS
NPM_BUILD=PASS
RELEASE_STATUS=NO-GO
```

## Completion pass 2026-09-27 21:45

### Phase 1 baseline

Both repos on `fixbug/test-9`. User email/worker files remain untracked or modified and were not reverted. Highest migration before this feature was `0147`. `0148_cms_template_import_attempts` is still the next number. No extra migration was added.

### Phase 2 MySQL 8

Docker was checked once. Daemon is not running:

```text
open //./pipe/docker_engine: The system cannot find the file specified.
```

No isolated MySQL 8 server was available without loading `.env` or using schema `cobo_iam`. Those paths were not used.

```text
MYSQL_VERSION=
MYSQL_DATABASE_REDACTED=
MIGRATION_NUMBER=0148
0148_UP=NOT_RUN
0148_RERUN=NOT_RUN
0148_FIXTURE=NOT_RUN
RETENTION_QUERY=PASS_IN_MEMORY
0148_DOWN=NOT_RUN
ISOLATED_SCHEMA_CLEANUP=NOT_RUN
MYSQL8_ISOLATED_VALIDATION=BLOCKED
reason=Docker daemon is not running (//./pipe/docker_engine missing). No isolated MySQL 8 database was contacted.
```

Retention hiding of a two-year-old row, without deleting it, passed on the in-memory repository (`TestHistoryRetentionHidesOldRowWithoutDeletingIt`).

### Phase 3–5 tests that did run

`CMS_TEMPLATE_IMPORT_HISTORY_ENABLED` parser: unset, `false`, `FALSE`, `0`, `no`, `NO` are off; `true`, `TRUE`, `1`, `yes`, `YES` are on. Flag-off confirm still creates one inactive draft and does not insert an attempt row. History reads return 404 `FEATURE_DISABLED`.

Flag on: validate then confirm creates one inactive draft, stores hashes (not the raw token), sets `CONFIRMED` and `created_type_id`. Mapping-required confirm succeeds after the caller supplies a catalog target, without a second upload. Replay returns 409 `IMPORT_ATTEMPT_ALREADY_CONFIRMED` and does not create a second draft. Two goroutines confirming the same attempt produce one success and one 409, one `CONFIRMED` row. HTTP replay writes `disclosure.type.import` once.

Claim now sets internal status `CONFIRMING` before materialization so a second reader cannot claim the same attempt. That status is not a confirm success. It exists because leaving the row in `VALIDATED` after bumping `row_version` allowed a second goroutine to claim and collide on the type id.

Mismatch cases that create no draft: unknown attempt id (`IMPORT_ATTEMPT_NOT_FOUND`), attempt owned by another actor (`IMPORT_ATTEMPT_FORBIDDEN`), unknown mapping key (`INVALID_REQUEST`).

### Phase 7 lint

`npm run lint` (`tsc --noEmit`) exits 2. Filtering for `cms-core/templates/import`, `TemplatesFeatureScreen`, and `template-import` schema paths returned no errors. Those failures are outside this diff and were not edited. Classification: `PRE_EXISTING_FAILURE`. No `NEW_DIFF_FAILURE` on the import history files.

### Phase 8

- `go build ./...` PASS.
- `go test ./internal/disclosure/... ./internal/platformcms/... ./internal/audit/... ./migrations/` : PASS except `internal/disclosure/app/legal_basis_backfill` `TestSnapshotRoundTripAndMode` expected mode `0600`, got `-rw-rw-rw-`. That package is not part of this diff. `PRE_EXISTING_FAILURE`.
- `npm run build` PASS on the previous frontend implementation pass. Frontend source was not changed in this completion pass.
- `npm run test`: 346 files passed, 21 failed, 76 failed tests, 2386 passed, 7 skipped. No failure under `src/features/cms-core/templates/import` or `TemplatesFeatureScreen`. One failure is `templateEditorViewModel.test.ts`, which expects four editor tabs and receives five, including `Lịch sử bản ghi CMS`. That file was not edited. Classification: `PRE_EXISTING_FAILURE`.

### Decision

`RELEASE_STATUS=NO-GO` because MySQL 8 isolated up/rerun/down did not run. Flag stays off. DEV and production were not touched.


## Implemented

- Guide artifact `internal/disclosure/app/artifacts/cobo-template-import-guide-v1.0.md`, embedded. `GET /api/v1/platform/cms/templates/import/guide` with filename `cobo-template-import-guide-v1.0.md` and `text/markdown; charset=utf-8`. Same write permission as the example. No database write.
- Frontend button `Tải hướng dẫn tạo file JSON` beside the example download. Independent of the history flag.
- Conditional schema in both `docs/schema/template-import-v1.schema.json` files: periodic periodicity is five values and required; irregular `deadline_rule` required; irregular periodicity if present is `event_based` or `ad_hoc`. No shared seven-value enum. Import upload still does not schema-check before POST.
- Irregular periodicity validator rejects values other than `event_based` and `ad_hoc` when the field is present. Periodic five-value rule and deadline overwrite are unchanged.
- Migration `migrations/0148_cms_template_import_attempts.up.sql` and down that only drops `cms_template_import_attempts`. Number chosen after listing the ledger: highest existing file was `0147`. Appended to `run_dev_migrations.sh`. Not applied.
- Flag `CMS_TEMPLATE_IMPORT_HISTORY_ENABLED`: `true` / `1` / `yes` on; otherwise off. Default off.
- Flag off: validate/confirm do not write history; missing `import_attempt_id` is still optional; history reads return HTTP 404 `FEATURE_DISABLED`.
- Flag on: validate returns `import_attempt_id`; confirm requires it (`IMPORT_ATTEMPT_REQUIRED`). Rows store `file_sha256`, `canonical_payload_sha256`, and `validation_token_sha256`. No raw JSON or raw token columns.
- History list `GET /api/v1/platform/cms/templates/import-history` and detail `GET /api/v1/platform/cms/templates/import-history/items/{id}`. The `{id}` path is one segment deeper than the plan’s `import-history/:id` because Go’s ServeMux conflicts `templates/import-history/{id}` with `templates/{type_id}/workflow`.
- Tab `/cms/templates?tab=import-history`, label `Lịch sử import file`. `Lịch sử bản ghi CMS` was not renamed.
- Disabled panel text `Tính năng chưa được bật` is separate from the empty state.
- Reads hide rows older than 365 days. No purge job and no archive table.

## Tested

- `go test ./internal/disclosure/app/ ./internal/disclosure/transport/http/ ./internal/disclosure/infra/inmemory/ ./internal/disclosure/infra/mysql/ ./migrations/` PASS.
- `go test ./internal/disclosure/...` : app, transport, mysql, inmemory, migrations PASS. `internal/disclosure/app/legal_basis_backfill` FAIL `TestSnapshotRoundTripAndMode` expected file mode `0600`, got `-rw-rw-rw-`. That package was not edited. Classified pre-existing (Windows file mode).
- `go build ./...` PASS.
- Frontend targeted Vitest (guide, history, feature tab, import screen, example, schema contract): 34 PASS.
- `npm run build` PASS.
- `npm run lint` (`tsc --noEmit`) FAIL on existing files (`CompanyProfile.departmentCount`, template channel types, portal disclosure tests). No diagnostic on the new import-history files.

## Not run / blocked

- `docker compose -f docker-compose.dev.yml build api`: BLOCKED. Docker pipe is not available (`docker_engine` missing).
- Isolated MySQL 8 up/rerun/down: NOT_RUN. No isolated DSN. `.env` was not loaded. DEV schema was not migrated.
- `go test ./internal/platformcms/...` and `go test ./internal/audit/...`: NOT_RUN. Those packages were not changed.
- `npm run test` for the whole frontend: NOT_RUN. Targeted import tests were run.
- Browser and DEV confirm: NOT_RUN.
- Concurrent confirm race test: not executed. Claim uses `row_version` before `UpsertTypeVersion`, but there is no two-goroutine test in this pass.
- End-to-end confirm success with the flag on (Draft + `CONFIRMED` row): not covered by an executed test. Covered cases are flag-off validate, flag-on malformed JSON row, missing attempt id, and cross-company detail 404.

## Limitations

- Detail route is `/import-history/items/{id}`.
- Schema download endpoint/button is not in this slice. Both schema files are updated for authors and tests.
- Purge after 365 days is a follow-up. Until then, older rows can remain in the table while the API does not return them.
- Flag stays off in code. Nothing was enabled on a running environment.
