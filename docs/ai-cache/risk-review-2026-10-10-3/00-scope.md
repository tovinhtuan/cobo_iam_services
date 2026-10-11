# Risk review 2026-10-10 (lần 5) - scope: rủi ro worker/alert

- Mode: read-only (`/risk-review review kỹ các rủi ro trên`). Không sửa code, không ghi DB, không thao tác server.
- Repo: `cobo_iam_services` branch `improve/ui-theme` HEAD `a8acb51` (merge-base với `main`: `bea9376`). Không chạm `cobo_web_design`.
- Đầu vào: `../worker-alert-spam-audit-2026-10-10/00-report.md`, gồm PER-01..08, REM-01..11, OBX-01..09, DATA-01 và bằng chứng DEV.

## Phạm vi (paths)
- `cmd/worker/main.go`
- `internal/reminder/**` (app, infra/mysql, infra/email, infra/inapp, transport/http)
- `internal/disclosure/app/periodic*.go`, `periodic_oneshot/**`, `timeline.go`, `template_validation.go`; `internal/disclosure/infra/mysql/repository.go` (phần periodic)
- `internal/adhoc/app/runtime_materialize.go`, `internal/adhoc/infra/disclosure/record_creator.go`, `internal/adhoc/infra/notification/**`
- `internal/workflow/app/{service.go,milestones.go}`, `internal/workflow/infra/mysql/{repository.go,milestones.go}`
- `internal/platform/outbox/**`, `internal/notification/**`, `internal/inappnotification/**`, `internal/workflowdept/**`
- Migration liên quan: `0039` (periodic_cycles), `0085` (in-app), `0151` (materialization state), các bảng reminder/milestone
- Vận hành: `docker-compose.artifacts.yml` (env worker), `Makefile` (`deploy-be`), các binary `bin/worker.*` trên DEV, runbook rollback trong `.claude/skills/deploy-dev-release`, `.claude/skills/wf-release`

## Reviewers
| Reviewer | Lý do |
|---|---|
| perf-reliability | `cmd/worker`, outbox, reminder, notification, periodic: chạy trùng, retry, lease, starvation, retention |
| be-security | app/infra + handler reminder: H1/REM-09 spam relay, nội dung email, PII trong log |
| api-compat | migration cần cho bản sửa (unique milestone, enum trạng thái mới), mixed-version worker/API, rollback (PER-01) |
| secrets-cors | `.env`/compose/Makefile của worker, binary rollback trên DEV, SMTP thật trên DEV |

Không chọn:
- fe-security, admin-role: không chạm FE hay quyền admin.
- cache-versioning: không chạm cache hay nginx.

## Secret scan (không ghi giá trị; không lưu output thô, xem SEC-24)
- 78 hit trên các path phạm vi.
- Tập trung ở test fixture (`template_import_*_test.go` 33, `workflowdept_test.go` 6), `docker-compose.artifacts.yml` (11, đã biết: H20/H23), `workflowdept/classify.go` (6), `periodic_oneshot/env.go` (4).
- Reviewer secrets-cors sẽ phân loại các hit không phải test.
- File nhạy cảm đang được track (không đổi): `.env.example`, `configs/login_password_rsa_dev.pem`.
