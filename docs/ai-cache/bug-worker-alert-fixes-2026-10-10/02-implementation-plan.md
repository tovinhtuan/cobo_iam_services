# Kế hoạch triển khai: sửa bug worker/alert (wf-bugfix Phase 3–6, trình bày theo quy ước `spec-authoring`)

> Trạng thái: **KẾ HOẠCH, chưa triển khai.** Chưa sửa code, chưa viết test, chưa ghi DB, chưa deploy.
>
> **Khuyến nghị PO/BA để chốt Phase 0: `05-po-ba-decision-recommendations.md`** (đối chiếu contract, mâu thuẫn C-1..C-6, đề xuất Contract v1.2).
>
> Tài liệu chi tiết theo PR:
> - `02a-plan-pr1-reminder.md`
> - `02b-plan-pr2-deadline-periodic.md`
> - `02c-plan-pr3-adhoc.md`
> - `02d-plan-pr4-pr5-list-dashboard-status.md`
>
> Test: `03-test-plan.md`. Deploy / sửa dữ liệu: `04-rollout-data-repair.md`. Nguyên nhân gốc: `01-root-cause.md`.

## 1. Mục tiêu và ràng buộc
- **Mục tiêu:**
  - Nhắc việc đúng người, đúng bước, đúng giờ, không spam.
  - CBTT bất thường đã duyệt hiển thị đúng.
  - Hạn pháp lý tính đúng.
  - Danh sách hạn và dashboard có số liệu đúng.
  - Worker không thể tái tạo sự cố 4 triệu bản ghi.
- **Ràng buộc:**
  - Ưu tiên bản sửa **không đổi schema**, vì `make deploy-be` không chạy migration (H19). Migration duy nhất trong kế hoạch là `0153` (data-only, PER-01).
  - Mỗi bug chỉ sửa ở đúng nguyên nhân gốc, không refactor ngoài phạm vi.
  - Mọi bản sửa phải có test tái hiện fail trước (GATE R), trừ lỗi dữ liệu/cấu hình (AHA-13, DUE-03), vốn được tái hiện bằng query/test file.
  - Tenant isolation do server bảo đảm. Sàn rollback worker là `cabbecf`.
- **Hạn chót:**
  - **2026-10-12**: PR-1a phải lên DEV, vì REM-06 sẽ chặn scanner quanh 13–14/10.
  - **2026-12-01**: PR-2a (PER-06, DUE-03) phải xong, vì cycle tính ngày làm việc vắt sang 2027 sẽ bắt đầu hỏng.

## 2. Phase 0: chốt quyết định (HARD STOP)

> **2026-10-11: PO "đồng ý toàn bộ"** các khuyến nghị trong `05-po-ba-decision-recommendations.md`. Contract đã cập nhật lên **v1.2** (`docs/ai-cache/business-contract-workflow-deadline-final.md` §2.2); WEB `06-approved-kpi-formulas.md` có amendment.
>
> Mọi Q-xx bên dưới đã chốt theo cột khuyến nghị. Hai việc còn lại:
> - **Q-06:** BA/HR đối chiếu văn bản cho các ngày nghỉ còn TBC. Không chặn, vì file chỉ chứa ngày chắc chắn.
> - **Q-25:** từng thao tác trên server DEV vẫn cần user bấm thực hiện lúc deploy.

PR nào còn quyết định "chặn" chưa chốt thì không được code. Các quyết định "mặc định" sẽ dùng khuyến nghị nếu không có phản hồi.

| Mã | Quyết định | Khuyến nghị | Người chốt | Chặn PR | Loại |
|---|---|---|---|---|---|
| Q-01 | Ngưỡng nhắc: grace 2 ngày sau `step_end`; trễ quá 48h thì hết hạn; khung gửi 07:00–21:00 HCM; khi trễ chỉ gửi 1 thông báo quá hạn cho mỗi bước | Như nêu | PO | PR-1b (RMC-02) | mặc định |
| Q-02 | REM-04: Defer 15 phút + hết hạn 48h (B), hay FAILED ở lần thứ 5 (A) | B | Tech lead | PR-1b | mặc định |
| Q-03 | Tách PR-1a (commit 1–5) để kịp 2026-10-12 | Có | Tech lead | — | mặc định |
| Q-04 | DUE-02: nguồn loại ngày khi tính hạn | **Đã định hướng theo Q-05 (2026-10-10):** tính theo đúng loại ngày template khai báo. Resolver đọc `rules.deadline_day_type`, rồi đến `deadline_config.deadline_duration_type`. Chỉ khi template không khai báo gì mới dùng fallback ngày lịch, kèm validate bắt buộc khai báo khi tạo/publish. DEV: 12/12 template active đều đã khai báo, nên dự kiến không phải sửa Draft (dry-run xác nhận) | User (đã chốt nguyên tắc) | PR-2b | **mặc định** (hết chặn) |
| Q-05 | DUE-10: hạn trùng ngày nghỉ có dời sang ngày làm việc kế tiếp không | **ĐÃ CHỐT (user, 2026-10-10):**<br>- **Không tự dời.** Template khai báo hạn theo ngày làm việc hay ngày lịch thì tính đúng theo đó; kết quả ngày lịch rơi vào cuối tuần/ngày nghỉ vẫn giữ nguyên.<br>- Hạn là **ngày cụ thể** (FIXED_DATE) thì đó là ngày bắt buộc. Code hiện đã giữ ngày mặc định (`WARN_ONLY_KEEP_DATE`). Tuỳ chọn `MOVE_TO_NEXT_WORKING_DAY` chỉ áp dụng khi người tạo template chọn tường minh trên CMS, vì đó là "quy định áp dụng ở template".<br>- DUE-10 đóng: không cần flag, không đổi code | User | — | **đã chốt** |
| Q-06 | DUE-03: danh sách ngày nghỉ 2026/2027 | **(05 §2)** File chỉ chứa ngày chắc chắn. Ngày TBC nhập qua CMS theo văn bản chính thức (cần DUE-01). BA cập nhật lịch năm sau vào tháng 10 | **BA** | PR-2a commit 8 | **chặn** |
| Q-07 | DUE-07: T0 của workflow khi submit thủ công | Hôm nay theo HCM, `system_date` | PO | PR-2b | **chặn** |
| Q-08 | DUE-08: timeline vượt hạn | Chỉ phát hiện (cờ + log) | PO | PR-2b | mặc định |
| Q-09 | DUE-09: tạo kỳ trước tại `max(lead, open_days)` | Có | PO | PR-2b | mặc định |
| Q-10 | DLA-05: Draft có `planned_date` NULL thì neo vào `created_at` | Có | PO | PR-2b | mặc định |
| Q-11 | PER-03: 400/403/404/422 → FAILED; tối đa 20 lần, backoff 1→60 phút | Có | Tech lead | PR-2a | mặc định |
| Q-12 | AHA-01: cách đưa ad-hoc đã duyệt vào tab Hạn | **(a′, 05 §2):** hiện từ khi duyệt tới khi record Approved/Published/Completed (nhánh OR + điều kiện chưa terminal). Cần sửa contract C-3 | **PO** | PR-3 commit 7 | **chặn** |
| Q-13 | AHA-03: quy tắc ngày chốt khi nhiều người duyệt | **Giữ A4 (contract v3) = giá trị được nhập cuối cùng** (ô trống không xoá giá trị trước) + sửa DUE-06 + hiện giá trị hiện hành trong modal. **Huỷ** khuyến nghị "sớm nhất" (trái A4 và cảnh báo UI đã phát hành) | **BA** | PR-3 commit 8 | **chặn** |
| Q-14 | AHA-05: F2 (kiểm quyền người tạo lúc submit) hay F1 (materialize nội bộ) | F2 | Tech lead + bảo mật | PR-3 commit 9 | mặc định |
| Q-15 | AHA-06: B (bù trừ) hay A (chặn Reject/Cancel khi đủ phiếu) | **A + B** | PO | PR-3 commit 10 | mặc định |
| Q-16 | AHA-09b: in-app khi được giao bước | **P1 ngay sau PR-3**, sau flag | PO | — | mặc định |
| Q-17 | DLA-02: nhân viên phòng ban (không phải người được giao) có thấy cảnh báo của bước thuộc phòng mình không | Chưa (follow-up) | PO | — | mặc định |
| Q-18 | DLA-03: KPI mở theo trạng thái hiện tại; range chỉ áp cho KPI hoàn thành | Có | **PO** | PR-4 commit 5 | **chặn** |
| Q-19 | DLA-04: định nghĩa hoàn thành, PENDING_CONFIRM, ngưỡng màu | **(05 §1–2):**<br>- `CompletedAt` = `confirmed_at` → `submitted_at` (định kỳ/thủ công) → thời điểm hoàn tất (ad-hoc); query trực tiếp `disclosure_records`.<br>- **Bỏ PENDING_CONFIRM** khỏi contract; giữ confirm cho trường hợp "đã công bố ngoài hệ thống".<br>- Ngưỡng màu ≥95 / 85–95 / <85 | **PO** | PR-4 commit 6 | **chặn** |
| Q-20 | DLA-11: confirm lần 2 trả 200 với người xác nhận gốc, hay 409 | 200 | PO | PR-4 | mặc định |
| Q-21 | DLA-12: được phép sửa `src/types.ts` (thêm `Unknown`) | Có | **Tech lead FE** | PR-4 WEB W1 | **chặn** |
| Q-22 | RMC-05: inactive không nhắc, không materialize; suspended vẫn seed và nhắc | Như nêu | **PO** | PR-5 | **chặn** |
| Q-23 | RMC-07: thiếu row prefs thì dùng mặc định | Có | **PO** | PR-5 | **chặn** |
| Q-24 | RMC-08: lịch gửi của company chỉ gate reminder hạn nộp | Có | PO | PR-5 | mặc định |
| Q-25 | **Vận hành DEV (cần cho phép rõ ràng):**<br>(a) cách ly 6 worker + ~7 api cũ;<br>(b) tắt 6 template QA chu kỳ hằng ngày;<br>(c) bật allowlist/redirect email trước khi PR-1 làm tăng lượng email (SEC-27);<br>(d) rotate `INTERNAL_REMINDER_TOKEN`;<br>(e) các bước sửa dữ liệu trong `04` | (a)(b)(c) trước PR-1a | **User** | PR-1a (khuyến nghị) | **chặn vận hành** |
| PO-01 | **Mới:** "Việc của tôi" lọc `<> 'draft'`, nên task định kỳ bị ẩn (`personalops/infra/mysql/repository.go:115,246`) | Dùng chung `ObligationOpen`; làm trong PR-4 cùng DLA-02 | PO | PR-4 | mặc định |
| Q-26 | Có dựng MySQL test (`MYSQL_TEST_DSN`) cho integration test không | Có (docker compose local) | Tech lead | GATE R/V của nhiều mục | khuyến nghị mạnh |

## 3. Thứ tự PR và phụ thuộc

```text
Phase 1 (≤2026-10-12)  PR-1a Reminder hotfix ── cần Q-25(a,c) trước khi deploy
Phase 2                PR-2a Periodic/date reliability (DUE-03 chờ Q-06)
Phase 3                PR-3  Ad-hoc ── chờ Q-12, Q-13
Phase 4                PR-4  List/dashboard (rebase lên PR-3 commit 5) ── chờ Q-18, Q-19, Q-21
Phase 5                PR-1b Reminder phần còn lại (REM-05, RMC-02, REM-03/04, RMC-03, REM-10, RMC-09)
Phase 6                PR-5  Company status + prefs (cần PR-1b/REM-05) ── chờ Q-22, Q-23
Phase 7                PR-2b Hạn pháp lý ── chờ Q-07 (Q-04 đã định hướng, Q-05 đã chốt ngày 2026-10-10)
Phụ lục                PR-W3 outbox/SMTP, PR-W4 H1, PR-W5 ops, PR-W6 retention (xem §6)
```

**Xung đột giữa các PR đã được hoà giải:**
1. **AHA-07 (PR-3) và DLA-02 (PR-4)** cùng sửa `listCurrentStepMeta` và cách chọn instance. Thống nhất dùng `internal/platform/workflowsel` (PR-3 commit 5); PR-4 không tự thêm `ORDER BY … ASC`.
2. **AHA-01 (PR-3) và DLA-07 (PR-4)** sửa cùng file SQL membership/listRows. PR-3 vào trước, PR-4 rebase.
3. **DLA-04 (PR-4):** PENDING_CONFIRM chỉ đạt được sau AHA-01 phương án (a). KPI hoàn thành dùng `COALESCE(completed_at, confirmed_at)` cho cả hai đường.
4. **RMC-05 (PR-5)** cần cơ chế hết hạn của REM-05 (PR-1b), để dòng PENDING của company inactive không bị gửi dồn khi company được kích hoạt lại.
5. **REM-01 (PR-1a commit 3)** thêm kiểm `step.ID` trùng ở `adhoc/runtime_materialize.go`. **AHA-05 (PR-3)** thêm gọi `PrepareV3Materialization` ở cùng vùng code; PR-3 rebase.
6. **DUE-07 (PR-2b)** đổi T0 trong `EnsureOnSubmit`. **AHA-07 (PR-3)** bỏ bootstrap cho ad-hoc, nên DUE-07 chỉ còn ảnh hưởng submit thủ công.
7. **RMC-01 (PR-1a):** reader runtime tự truyền ngày HCM, không phụ thuộc bản sửa DLA-01 (PR-2a).
8. **PER-01 trước PER-03/PER-04** (API-38): FAILED phải giữ dấu claim trước khi tạo thêm dòng FAILED.

## 4. Các phase triển khai (checkbox + CHECKPOINT)

### Phase 1: PR-1a hotfix reminder (gấp)
- [ ] Q-25(a) cách ly binary cũ; Q-25(c) bật allowlist email (hoặc tạm `REMINDER_DISPATCH_ENABLED=false`).
- [ ] Commit 1: fake PERF-47 (repo occurrence 2 unique key; scanner tôn trọng limit/cursor).
- [ ] Commit 2: REM-01 `EnsureOccurrence`.
- [ ] Commit 3: REM-01 kiểm step ID trùng ở `ValidateSnapshot`, override, CMS facade, ad-hoc.
- [ ] Commit 4: RMC-01 (A)+(B) + REM-06 obsolete/phân trang + RMC-04 ngày seed.
- [ ] Commit 5: RMC-06 trạng thái reminder trong API.
- [ ] **CHECKPOINT:**
  - `go test ./internal/reminder/... ./internal/workflow/... ./internal/adhoc/app/ ./internal/disclosure/app/ -count=1`
  - `go test -race ./internal/reminder/... -count=1`
  - `go vet ./...`
  - `docker compose -f docker-compose.dev.yml build api worker`
- [ ] Review song song: perf-reliability, api-compat, be-security → `premerge-system-review` rút gọn.
- [ ] Deploy theo `04` §2 (dark launch).
- [ ] Verify DEV theo `03` §5.

### Phase 2: PR-2a độ tin cậy periodic + ngày
- [ ] Commit 1–11 theo `02b` (PER-01 + 0153, PERF-46, BES-38, PER-05, PER-04, PER-03, DUE-01, DUE-03 [Q-06], PER-06, DLA-01, DLA-05 phần skip).
- [ ] **CHECKPOINT:**
  - `go test ./internal/holiday/... ./internal/deadlinealerts/... ./internal/personalops/... ./internal/disclosure/... ./internal/workflow/... ./internal/portaldashboard/... -count=1`
  - `grep 0153 migrations/run_dev_migrations.sh`
  - `diff -r configs/non_trading_days deploy-artifacts/backend/configs/non_trading_days`
  - build Docker.
- [ ] Review: perf-reliability, api-compat, be-security (BES-38); tự kiểm `cobo-domain-cbtt` cho DUE-03.
- [ ] Deploy theo `04` §3: `MODE=all` hoặc `migrate` + `be`; cần copy `deploy-artifacts` cho file ngày nghỉ và compose.

### Phase 3: PR-3 ad-hoc
- [ ] Q-12, Q-13 đã chốt.
- [ ] Commit 1–11 theo `02c`.
- [ ] **CHECKPOINT:**
  - `go test ./internal/adhoc/... ./internal/deadlinealerts/... ./internal/workflow/... ./internal/disclosure/... ./internal/personalops/... ./internal/portaldashboard/...`
  - `-race` adhoc/workflow.
  - Integration test nếu có DSN.
  - build Docker.
- [ ] Review: be-security, admin-role, api-compat, perf-reliability.
- [ ] Deploy + sửa dữ liệu theo `04` §4 (AHA-13 → AHA-07 → AHA-02 → AHA-06).

### Phase 4: PR-4 danh sách hạn/dashboard (BE trước, WEB sau)
- [ ] Q-18, Q-19, Q-21 đã chốt. Rebase lên PR-3 commit 5.
- [ ] BE commit 1–8, WEB W1–W2 theo `02d`.
- [ ] **CHECKPOINT:**
  - `go test ./internal/deadlinealerts/... ./internal/portaldashboard/...`
  - WEB: `npx vitest run src/services src/pages/portal/deadlines src/features/portal-dashboard && npm run lint && npm run build`
- [ ] Review: be-security + admin-role, perf-reliability, api-compat.

### Phase 5: PR-1b reminder phần còn lại
- [ ] Commit 6–11 theo `02a` (REM-05/04, RMC-02 [Q-01], REM-03, RMC-03, REM-10, RMC-09).
- [ ] **CHECKPOINT:**
  - `go test ./internal/reminder/... ./internal/inappnotification/... ./internal/notification/app/ -count=1` (chỉ được còn fail có sẵn)
  - `-race`
  - build Docker.

### Phase 6: PR-5 trạng thái company + kênh thông báo
- [ ] Q-22, Q-23 đã chốt; PR-1b đã có trên DEV.
- [ ] Commit 1–5 theo `02d`.
- [ ] **CHECKPOINT:**
  - `go test ./internal/reminder/... ./internal/disclosure/... ./internal/companyaccess/...`
  - `EXPLAIN` query dispatch qua qa_ro.

### Phase 7: PR-2b hạn pháp lý
- [x] Q-05 đã chốt (không tự dời; theo quy định của template; ngày cụ thể là bắt buộc), ghi ở `00-report.md` §Nhật ký quyết định.
- [ ] Q-07 đã chốt. Q-04 dùng hướng đã định (theo template, fallback ngày lịch + validate).
- [ ] Chạy dry-run DUE-02. Kỳ vọng 0 dòng trên DEV vì mọi template active đều đã khai báo; nếu khác 0 thì gửi danh sách cho BA.
- [ ] Commit 12–14 theo `02b`. DUE-10 đã đóng, không làm. Thêm test khoá quyết định Q-05.
- [ ] Deploy, rồi sửa dữ liệu Draft đang mở (đã được duyệt).
- [ ] **CHECKPOINT:** kỳ mới thỏa `due = cycle_start + N - 1` (ngày lịch); dry-run trả 0 dòng.

## 5. Phạm vi ảnh hưởng tổng (Phase 5 của wf-bugfix)
- **Hành vi người dùng thay đổi:**
  - nhiều email hơn (bước 2..N, email "đã duyệt" cho bước cuối được Confirm);
  - ít email rác hơn (không gửi đêm, không gửi bản trễ vô nghĩa);
  - danh sách hạn và dashboard có thêm dòng và số liệu khác.

  → Cần thông báo trước cho người dùng DEV/QA và ghi vào release notes.
- **Contract (giữ shape, đổi giá trị):**
  - Status reminder trong API workflow;
  - KPI dashboard;
  - `workflow_instance_id` (instance proposal);
  - `deadline_summary.duration_type` (PR-2b);
  - lỗi 400/422 mới khi step ID trùng hoặc khi submit ad-hoc chặt hơn.

  Cập nhật `docs/api-contracts-json.md` qua `api-postman-sync`.
- **FE:** chỉ PR-4 WEB (nhãn, `types.ts`, tooltip). Mọi PR khác: FE cũ chạy được với BE mới.
- **Dữ liệu cần sửa trên DEV:** AHA-13, AHA-07, AHA-02, AHA-06, REM-06 (chỉ khi trễ hạn), DUE-02 (sau Q-04), PERF-46. Xem `04` §6.

## 6. Phụ lục: các PR còn lại từ risk review lần 5 (tóm tắt; chi tiết ở `../risk-review-2026-10-10-3/10-risk-report.md`)
- **PR-W3 outbox/SMTP/email:**
  - OBX-02/H8: dùng `boundedSendMail` cho `cmd/worker/main.go:392`, `notification/infra/smtp/adapter.go:42`, `binding_mailer.go:35`.
  - OBX-01/H9: claim đặt `available_at=NOW()`, các lệnh `Mark*` thêm `AND status='processing'`.
  - OBX-03: claim `MarkSending` có điều kiện.
  - OBX-07: cắt `last_error` theo rune; log lỗi của `Mark*`.
  - BES-30: `html/template` cho `body.html`.
  - BES-39/40: dùng `BuildMessage` + gửi riêng từng người; che email trong log.
  - BES-42: giới hạn tần suất quên mật khẩu (3/giờ).
  - REM-11.
  - Không đổi schema.
- **PR-W4 H1 / BES-36:**
  - 5 route reminder-config: `FindByID(company)` + `authorize`.
  - Giới hạn mảng (≤10, 0..365); người nhận là member active; `MaxBytesReader` 16 KB.
  - Lúc dispatch, loại người nhận không phải member.
  - Lịch sử reminder lọc theo company.
- **PR-W5 vận hành/deploy:**
  - H19/API-34: preflight `deploy-be` chặn khi còn migration chưa chạy.
  - H18: thêm 0038/0098/0099/0150 vào runner, viết lại cho idempotent trước.
  - API-43: runbook sàn rollback (worker ≥ `cabbecf`).
  - Vệ sinh binary: `deploy-be` tự tạo `bin/<svc>.rollback.<sha>`, chỉ giữ 3 bản.
  - SEC-28: compose dùng `${VAR:-default}` + runbook kill switch.
  - PERF-43: gauge worker / heartbeat.
  - SEC-27: wrapper allowlist email.
  - SEC-26: `-tags prod`, lấy người nhận ở server, đóng cổng :8080.
- **PR-W6 retention + ràng buộc:**
  - OBX-04/SEC-29: xoá `body` khỏi payload khi `MarkProcessed`; job dọn theo lô có `LIMIT`, loại trừ theo API-41.
  - API-36: migration dedupe + UNIQUE `(workflow_instance_id, step_id, milestone_type)`, sau PR-1a.
  - `0153` đã dùng cho PER-01, nên PR-W6 lấy số kế tiếp.
  - Index `(reminder_sent, scheduled_date)`.
  - API-42: contract doc cho reminder/in-app.

## 7. Định nghĩa "xong" cho mỗi PR
- Test tái hiện fail trên commit cha và pass sau khi sửa; revert bản sửa thì fail lại.
- Baseline `go test ./...` chỉ còn đúng các fail có sẵn:
  - 6 test template-import (`disclosure/app`);
  - `notification/app TestContract_VariableParity/workflow.approved`;
  - vet của `workflowfulfillment`.
- `docker compose -f docker-compose.dev.yml build api worker` thành công, hoặc ghi `BLOCKED:` kèm lý do.
- Reviewer subagent đã chạy, không còn HIGH; `premerge-system-review` rút gọn.
- Đã deploy DEV (theo yêu cầu rõ ràng của user) và có smoke/SQL verify trong `03` §5.
- Ghi `05-completion-<pr>.md` trong thư mục này và một mục trong `docs/ai-cache/reusable-task-updates.md`.
