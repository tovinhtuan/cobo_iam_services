# Release DEV 2026-10-09: post-deploy và smoke QA

## Đã deploy
| Repo | Danh tính | Mục tiêu |
|---|---|---|
| cobo_iam_services | base `0609fdf` + working-tree diff `bd752415a34f99cc3c27704aad1484a23665900d` (sha1 của `git diff HEAD` + file untracked) | `make deploy-be`: api + worker, binary 2026-10-09 09:19 (giờ server) |
| cobo_web_design | base `37a799db` + src diff `3041db3288efe7b8fef7a51a28ead3381a52bf32` | `make deploy-fe`: bundle `assets/index-CkVro7nN.js` |

- User đã duyệt: deploy working tree (chưa commit), chấp nhận mang theo `dea4e59` (regression import đã biết), quy trình gồm backup, deploy-be và deploy-fe.
- Migration: không có.
- Backup trên server (rollback point): `bin/api.bak-20261009`, `bin/worker.bak-20261009`, `web/dist.bak-20261009` (bản build 2026-10-07).

## Kiểm tra sau deploy
| Kiểm tra | Kết quả |
|---|---|
| `/healthz`, `/readyz` qua nginx :3000; `/healthz` :8080 | 200 / 200 / 200 |
| Container | api, worker Up; web healthy |
| Log api/worker 15 phút sau deploy | 0 dòng ERROR/panic |
| `check_http_headers.sh` | OK: `index.html` không cache, asset hash immutable, asset cũ trả 404. Thiếu CSP/XFO/nosniff/Referrer-Policy (FES-02, có sẵn) |
| Bundle chứa code mới | `workflow-release-action-error`, `workflow.release.error.forbidden` có trong `index-CkVro7nN.js` |
| Smoke trong `deploy-fe` | healthz, login-key, fe index 200; Personal Ops V2 marker PASS |

## Smoke QA API (`smoke/api-smoke.txt`): 19/19 PASS
- **C3:**
  - `POST /api/v1/platform/cms/admin/ops/adhoc-migrate-legacy-approvals` trả 404 cả khi không có token lẫn khi có token tenant. Route đã gỡ.
  - Route tenant vẫn chạy: `GET /api/v1/company/ad-hoc-proposals` trả 200 (admin c_001). `admin-approve` vẫn đăng ký (id không tồn tại → lỗi nghiệp vụ 404 JSON, không phải 404 của mux).
- **C2, tenant admin (`admin.dn`):**
  - versions, configuration, validate, publish, activate, POST assignee-roles đều trả **403 PERMISSION_DENIED**.
  - GET assignee-roles vẫn 200. Không có token → 401.
  - Probe ghi chỉ được gửi sau khi đã xác nhận một route đọc trả 403, tức binary mới đang chạy.
- **C2, platform CMS admin (c_001):** configuration, readiness, lifecycle, versions, validate, assignee-roles đều 200.
- Không publish/activate thật trên DEV.

## Smoke QA UI (Playwright headless, `smoke/ui-results.json` + screenshot): 4/4 PASS
- Login platform CMS admin → chọn c_001 → `/cms/templates?type_id=qa-resmoke-periodic-20260904a` → tab Workflow mẫu → tab Phát hành.
- Nút "③ Phát hành" và "④ Kích hoạt" hiển thị. Không bị chuyển sang `/app/forbidden`, không có alert lỗi, không có lỗi console, không có response 403/5xx. API workflow trả 200.
- Không kiểm tra được trên DEV: alert 403 tại chỗ. Cần một CMS operator có `platform.cms.view` nhưng thiếu `cms.template.write/activate`, và seed không có tài khoản như vậy. Hành vi này đã được phủ bằng Vitest.

## Ngoài phạm vi / follow-up
- `dea4e59` (regression import: offset 0 → BLOCKER) nay đã có trên DEV. Chưa smoke luồng import template; cần bugfix riêng.
- Dùng tài khoản seed có password đã lộ trong repo (SEC-01) để smoke. Các tài khoản này vẫn đang hoạt động trên DEV.
- Rollback: copy `*.bak-20261009` về `bin/api`, `bin/worker`, `web/dist` rồi `docker compose -f docker-compose.artifacts.yml up -d --force-recreate --no-deps api worker` và restart web. Lưu ý rollback BE sẽ mở lại C2/C3.
