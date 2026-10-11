# PR-2a: Completion report (wf-bugfix), 2026-10-11

Trạng thái: **đã commit (`fffe055`, `8926a94`, `5c10ab6`; chưa push) và đã deploy + smoke trên DEV ngày 2026-10-11** (user: "đồng ý 1, 2, 3", smoke trên DEV thay cho Docker local). Gồm 11 mục PR-2a của `02b-plan-pr2-deadline-periodic.md`, một mục cùng loại lỗi của DLA-01 (`published_date`), và hai follow-up của PR-1a (PERF-54, SEC-29, xem `06-completion-pr1a.md`).

```text
Bug / fix (GATE R: test tái hiện fail trên 141a242 trước khi sửa):
  PER-01   FAILED xoá dấu claim cũ (materialized_at=NULL), nên worker cũ hơn cabbecf tạo Draft mỗi tick.
           Fix: COALESCE(materialized_at, NOW(3)) + migration data 0153 (có trong runner).
           Test: TestMarkPeriodicCycleFailedKeepsLegacyClaimFence, TestMigration0153InDevRunner.
  PERF-46  Release đặt PENDING kèm materialized_at, không khớp selector nào nên cycle kẹt vĩnh viễn.
           Fix: release chuyển RETRY đến hạn ngay; one-shot dùng CreateAndSubmitPeriodicRecord(cycleID) (record ID xác định theo cycle).
           Test: TestReleasePeriodicCycleClaimRequeuesAsRetry, TestEngine_UsesDeterministicRecordID.
  BES-38   Type periodic thuộc một company bị seed cho mọi company; tenant khác thấy tên type.
           Fix: PeriodicTypeRow.OwnerCompanyID; guard ở cả 2 vòng seed + seedOneCompanySlot;
                lọc (dt.company_id IS NULL OR dt.company_id = pc.company_id) ở ListPendingCycles và ListNextAlertCycles.
           Test: TestSeedPeriodicCycles_CompanyTypeOnlyForOwner (tắt guard thì test fail: seed 2 cycle cho co-2), TestPeriodicCycleListsFilterTypeOwner.
  PER-05   Lỗi build/insert milestone bị nuốt (_ = fmt.Errorf).
           Fix: slog.Error "workflow milestones not seeded" (stage, company_id, workflow_instance_id); repo task dual-authority → WARN.
           Test: TestCreateWorkflowInstance_MilestoneInsertFailureIsLogged.
  PER-04   SIGTERM giữa claim và complete làm cycle kẹt CLAIMED vĩnh viễn.
           Fix: RequeueStalePeriodicClaims (lease 10 phút, so thời gian hoàn toàn trong SQL) chạy đầu mỗi lượt;
                item đã claim chạy trên context.WithoutCancel + timeout 2 phút; không claim mới khi ctx đã huỷ;
                stop_grace_period 150s cho worker (dev + artifacts compose), dài hơn timeout của item (PERF-02).
           Test: TestMaterializePeriodic_CancelAfterClaimStillFinalizes, TestMaterializePeriodic_ReapsStaleClaims, TestPeriodicRetryAndReaperSQL.
  PER-03   RETRY mãi mãi, 1 phút/lần; lỗi 4xx vĩnh viễn vẫn retry.
           Fix: 400/403/404/422 → FAILED GATE_<status>; retry backoff 1→2→4…→60 phút; claim thứ 20 lỗi → FAILED RETRY_EXHAUSTED.
           Test: TestMaterializePeriodic_PermanentGateErrorMarksFailed (400/403/404/422 fail; 409/429 retry).
  DUE-01   Upload lịch nghỉ lưu lùi 1 ngày (bind time.Time 00:00 HCM vào cột DATE với loc=UTC).
           Fix: bind "YYYY-MM-DD"; DBProvider không cache lỗi.
           Test: TestReplaceCalendar_BindsCivilDate (HEAD: 2026-12-31 cho 2027-01-01), TestDBProvider_DoesNotCacheLoadErrors.
  DUE-03   File 2026 chỉ có 01/01, thiếu 2027.
           Fix (theo Q-06, chỉ các ngày chắc chắn): 2026 = 01/01, 16–20/02, 27/04 (bù), 30/04, 01/05, 02/09;
                2027 = 01/01, 30/04, 03/05 (bù), 02/09. Có ở cả configs/ và deploy-artifacts/.
                Các ngày còn lại (Tết/Giỗ Tổ 2027, ngày liền kề 02/09, ngày hoán đổi) nhập qua CMS khi có văn bản.
           Test: TestNonTradingDayFiles_RequiredYears/CertainDays/NextYearFromOctober/DeployArtifactsInSync.
           (Plan cũ đòi ≥9 mục/năm, mâu thuẫn với Q-06 cho năm 2027; test viết theo Q-06.)
  PER-06   Worker chỉ đọc file lịch nghỉ; thiếu năm thì bỏ seed mà không báo.
           Fix: worker dùng CompositeProvider (DB + file) với TTL 5 phút (DBProvider TTL + kiểm "có lịch DB" TTL);
                WARN "periodic seed skipped" (reason holiday_year_missing | due_date_error | company_profile_error | upsert_error),
                chống lặp theo (type, reason, year) mỗi giờ; ErrHolidayYearMissing (sentinel);
                collector API: cobo_periodic_cycles{state}, cobo_periodic_cycles_claimed_stale,
                cobo_holiday_calendar_year_available{slot=current|next, source=db|file}, cobo_periodic_metrics_scrape_errors_total.
           Test: TestSeedPeriodicCycles_MissingHolidayYearIsLogged, TestDBProvider_TTLRefreshesYear,
                 TestCompositeProvider_CachesYearCheckForTTL, TestPeriodicCollector_*.
  DLA-01   "Hôm nay" lấy theo ngày UTC (00:00–07:00 giờ VN lệch 1 ngày).
           Fix: now/today.In(loc) trong deadlinealerts status + steps, personalops RemainingDays; published_date theo giờ HCM.
           Test: TestRemainingDaysFromDue_UTCInstantEarlyHCM, TestComputeDeadlineSteps_TodayFromUTCInstant,
                 TestRemainingDays_UTCInstantEarlyHCM, TestPublishedDateIsHCMDate.
  DLA-05   Một dòng không tính được hạn làm cả danh sách và dashboard trả lỗi.
           Fix: bỏ qua dòng đó + WARN "deadline alert row skipped: due unresolved". (Phần neo created_at thuộc PR-2b.)
           Test: TestListDeadlineAlerts_OneUnresolvableRowDoesNotFailList.

Verification (GATE V, 2026-10-11):
  go build ./...                                OK
  go test ./...                                 16 fail, tên trùng khớp từng test với baseline (diff rỗng)
  go vet ./...                                  chỉ còn copylocks có sẵn ở workflowfulfillment
  gofmt (bản LF của 38 file .go đổi)            sạch
  go test -race (disclosure/app periodic, holiday, disclosure/infra, reminder/app, deadlinealerts/app)   OK
  go test -race ./internal/workflow/app         chỉ fail TestV2MultiStep_ConcurrentCompleteSingleNext (có sẵn, HEAD cũng fail)
  docker compose -f docker-compose.dev.yml build api + go build cmd/api, cmd/worker, cmd/periodic-materialize-one trong image   OK
  docker compose config (dev) có stop_grace_period 1m0s. Compose local đời cũ không parse được env_file dạng path/required
  của file artifacts (lỗi có sẵn, không do bản sửa); đoạn YAML mới đã kiểm bằng mắt.

DEV trước deploy (qa_ro, 2026-10-11):
  periodic_cycles: COMPLETED 332, PENDING 14 (6 dòng legacy có record_id); FAILED 0; RETRY 0; CLAIMED 0.
  0153 target (FAILED + materialized_at NULL) = 0; PENDING kẹt sau release = 0; CLAIMED > 10 phút = 0;
  RETRY attempt_count ≥ 20 = 0; cycle chéo tenant = 0; holiday_calendars = 0 (chưa upload);
  instance in_progress thiếu milestone = 1 (đã biết, chỉ liệt kê); Draft thiếu planned_date = 0.
  Migration mới nhất đã chạy: 0152 → 0153 sẽ chạy tiếp theo khi deploy MODE=migrate/all.
```

## Review (Phase 6, 2026-10-11): be-security, api-compat, perf-reliability

Không có CRITICAL/HIGH. Đã sửa:
- **PERF-01 (MEDIUM):** create quá timeout thì các bước ghi trạng thái chạy cùng context đã hết hạn, nên cycle kẹt CLAIMED và reaper đưa lại mãi (không kiểm giới hạn).
  Sửa: các bước ghi trạng thái chạy trên context riêng 10s; reaper áp dụng backoff + giới hạn 20 lần (RETRY_EXHAUSTED).
  Test: `TestMaterializePeriodic_TimeoutStillRecordsRetry` (trước khi sửa: "mark periodic cycle cy-1 retry: context deadline exceeded") + SQL guard.
- **API-01 (MEDIUM):** DLA-05 nuốt cả lỗi hạ tầng, trả 200 với danh sách thiếu dòng. Sửa: chỉ bỏ qua lỗi riêng của dòng (`ErrHolidayYearMissing`, JSON config lỗi, HTTPError 4xx); request bị huỷ hoặc lỗi khác thì trả lỗi như trước. Test: `TestListDeadlineAlerts_InfrastructureErrorStillFails`.
- **PERF-04:** log gộp thành một WARN mỗi request (count, first_record_id) thay vì mỗi dòng một WARN.
- **BES-40:** `ListActivePeriodicTypes` loại type có `company_id` rỗng (fail closed ở nguồn). DEV: 0 dòng như vậy.
- **PERF-02:** `stop_grace_period` 150s > item timeout 2m + finalize 10s.
- **PERF-03:** one-shot xoá luôn cycle do chính nó tạo, không nhả trước (tránh race với worker).
- **API-04:** label đổi thành `slot` (current|next) cho khỏi nhầm với số năm.
- **BES-42:** `.gitignore` thêm `__pycache__/`, `*.pyc`; đã xoá `deploy-artifacts/__pycache__/`.

Ghi nhận, chưa làm (follow-up):
- BES-41/PERF-05: migration dọn cycle chéo tenant đã có trước fix (DEV = 0) và các dòng PENDING có `materialized_at` (DEV: 6 dòng legacy, đều đã có record_id); thêm điều kiện chủ sở hữu vào `GetPeriodicCycle`.
- API-02: **không** dùng `deploy-init` để cập nhật compose. Nó ghi đè cả file trên server, mất các chỉnh tay. Chỉ thêm dòng `stop_grace_period` vào bản trên server (có backup), hoặc diff trước.
- API-03: rollback worker xuống dưới bản này sẽ mở lại BES-38. Sau rollback, chạy lại query kiểm cycle chéo tenant.
- API-05: release note: trạng thái hạn 00:00–07:00 giờ VN đổi sớm hơn 1 ngày (đúng); `published_date` theo giờ HCM; danh sách "Cảnh báo tiếp theo" không còn type của tenant khác.
- PERF-06: cycle FAILED `GATE_4xx` chỉ đưa lại hàng đợi bằng SQL tay (runbook ở comment `periodicMaxAttempts`). Nên alert khi `cobo_periodic_cycles{state="FAILED"}` tăng.
- BES-44: hardening `/metrics` (không publish 8080 ra host, so token constant-time). Code cũ, không thuộc PR này.

Verify lại sau khi sửa theo review: build OK; `go test ./...` 16 fail trùng khớp baseline; vet sạch (trừ copylocks có sẵn); gofmt sạch; race OK cho các package đổi; Docker build + compile 3 binary trong image OK; compose dev `stop_grace_period: 2m30s`.

## Rollout (cần user cho phép)

1. Commit.
2. `deploy-dev.ps1 -Mode migrate` (0153, data-only) rồi `-Mode be`. Hoặc `-Mode all`, nhưng mode này deploy cả FE.
3. Trên server: backup `docker-compose.artifacts.yml`, chỉ chèn dòng `stop_grace_period: 150s` vào service worker (không dùng `deploy-init`, xem API-02), rồi recreate worker.
4. Sau deploy: `/healthz`, `/readyz`; `/metrics` có `cobo_holiday_calendar_year_available{slot,source}` (file = 1 cho current/next, db = 0 cho tới khi BA upload qua CMS `/cms/settings/holiday-calendar`); log worker không có ERROR; danh sách hạn và dashboard trả 200 cho company QA.
5. Rollback: binary `*.rollback.<ts>` (tạo trước khi deploy); 0153 là forward-only và vô hại.

## Follow-up

- BA upload lịch nghỉ chính thức 2026/2027 qua CMS (sau khi có DUE-01 trên DEV).
- PR-2b (ngày pháp lý): DUE-02, DUE-09, DUE-07, DUE-08, DLA-05 phần neo created_at, test khoá Q-05.
- Repair 1 instance thiếu milestone (PER-05): chỉ liệt kê, cần quyết định.

## Nhật ký rollout DEV 2026-10-11 (UTC)

1. **Trước deploy (03:56, qa_ro):** periodic COMPLETED 332, PENDING 14; occurrence max id 1416798; milestone obsolete 18. Danh sách migration chờ (so runner với `schema_migrations`) chỉ có `0153`.
2. **Backup:** `bin/{api,worker}.rollback.20261011T035605Z`, `docker-compose.artifacts.yml.bak-20261011T035605Z`.
3. **Migration 0153 (03:57):**
   - `deploy-dev.ps1 -Mode migrate` đã sync `migrations/` rồi dừng: PowerShell 5.1 coi dòng cảnh báo mysql trên stderr là lỗi (lỗi có sẵn của script).
   - Không dùng service `migrate` vì runner chạy `ALTER USER root/cobo ... BY '<mặc định>'`, tức đặt lại mật khẩu DB.
   - Áp dụng đúng file 0153 bằng root trong container DB, mật khẩu lấy từ biến môi trường của container (không gõ ra dòng lệnh), rồi `INSERT IGNORE INTO schema_migrations`. Hash file trên server khớp local. Số dòng phải sửa trước/sau: 0/0.
4. **Deploy BE (03:59):** `deploy-dev.ps1 -Mode be`; `/healthz` và `/readyz` OK; hash binary trên server khớp bản build.
5. **Compose (04:00):** file trên server giống hệt bản local trước khi đổi (không có chỉnh tay). Chèn 3 dòng (2 comment + `stop_grace_period: 150s`) bằng awk, xử lý CRLF; sau đó file trên server khớp bản local mới. Recreate worker: `StopTimeout=150`.
6. **Smoke DEV (04:01–04:03):**
   - `/metrics` (Prometheus đang scrape): `cobo_holiday_calendar_year_available{slot=current|next, source=file}` = 1, `source=db` = 0 (chưa upload); `cobo_periodic_cycles{COMPLETED}` 332, `{PENDING}` 14; `claimed_stale` 0; scrape errors 0. API log có "periodic metrics collector registered".
   - API (script `smoke_pr2a.py`, persona ENT/c_001, USER/c_001, USER/c_002): `deadline-alerts` (tất cả + OVERDUE/DUE_SOON/UPCOMING), `deadline-alerts/next`, `dashboard/overview`, `me/operational-overview`, `deadlines/{record}/steps` đều 200 (c_001: 5 cảnh báo = 3 OVERDUE + 2 UPCOMING; steps 6). CMS `holiday-calendars/2026|2027` = 404 (chưa có lịch DB, đúng kỳ vọng).
   - DB: CLAIMED/FAILED/RETRY = 0; không có cycle/record mới ngoài dự kiến; reminder: 0 occurrence mới, 0 đến hạn.
   - Log: worker 27 INFO, API 28 INFO, 0 WARN/ERROR.
7. **Chưa smoke được trực tiếp:** DLA-01 (cần khung 00:00–07:00 giờ VN; đã có unit test); DUE-01 trên DB thật (cần BA upload qua CMS); PER-04 reaper (cần dừng worker giữa một item).

Rollback: copy `bin/{api,worker}.rollback.20261011T035605Z` đè lên `bin/{api,worker}` rồi recreate api + worker; compose: khôi phục từ `.bak-20261011T035605Z`. 0153 là forward-only và vô hại. Rollback worker sẽ mở lại BES-38 (API-03).

Follow-up vận hành phát sinh:
- `deploy-dev.ps1 -Mode migrate` lỗi trên PowerShell 5.1 (stderr của mysql). `push-migration.*` gõ mật khẩu root ngay trên dòng lệnh.
- `run_dev_migrations.sh` (service `migrate`) đặt lại mật khẩu root/cobo về mặc định mỗi lần chạy: không dùng trên DEV có mật khẩu đã rotate.
- Sau khi BA upload lịch qua CMS: kiểm `MIN/MAX(holiday_date)` của năm nằm đúng trong năm, và metric `source=db` lên 1.
