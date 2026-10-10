# DEV deploy BE — 2026-10-10 (PR-A + PR-B)

User yêu cầu rõ trong session: "deploy BE và smoke QA Dev". User chọn commit PR-B trước rồi mới deploy.

```text
Repos and SHAs deployed:
  cobo_iam_services c6b4ab6 (PR-B), cha 2996f00 (PR-A), cha ff5ada2 (ROLE-01 v2, đã chạy trên DEV từ 2026-10-09).
  Working tree còn file chưa commit, nhưng không thuộc build Go: CLAUDE.md, .claude/skills/deploy-dev-release,
  4 smoke script trong release-2026-10-09, scripts/devqa/ (của phiên khác), và các file secret-scan.
Mode / targets: deploy-dev.ps1 -Mode be -SkipTests -> recreate api + worker (--no-deps). Web, MySQL, Redis không đổi.
Migrations applied: không có. DEV schema_migrations = 148, khớp 148 file *.up.sql ở local.
Checks before deploy:
  go build ./... OK. go vet ./...: chỉ còn lỗi có sẵn (PERF-25).
  go test ./...: tập test fail giống hệt HEAD gốc (11 test có sẵn). 80 package ok.
  -race (companyaccess/app, authorization, iam/app): không có race.
  Secret scan diff staged: 0. Review song song cho PR-A và PR-B: mọi HIGH/MEDIUM đã sửa.
  Docker build: BLOCKED (daemon local không chạy). Build Linux bằng go build đã OK.
Post-deploy checks:
  SHA-256 trên server khớp bản build local: api fd52e46f38671916070c83850ac06713c3462801897c87bca228faef673a4c6e,
                                            worker 545258325e784ead9a44fce5797a87dbed14f7a44461f24ef633a627d779cb53.
  /healthz 200, /readyz 200 (qua nginx :3000). /api/v1/me và /api/v1/admin/roles không có token -> 401.
  Log 3 phút sau khi restart: 0 dòng panic/fatal/ERROR ở api và worker.
  "redis effective-access cache enabled". Worker seed periodic cycles bình thường.
Rollback point:
  /root/cobo_project/bin/api.rollback.20261010T042845Z    (sha256 0e3d051f79dbae786763db199916bb9a1f28d5378081645cc5c5bc5c3b36970b)
  /root/cobo_project/bin/worker.rollback.20261010T042845Z (sha256 b5b845bdaf99e8b7b5ea9c0ba6c732155303d209c9b3bafa386f188d89290fa9)
  Cách làm: dừng worker, cp các file .rollback về bin/api và bin/worker, rồi
  `docker compose -f docker-compose.artifacts.yml up -d --force-recreate --no-deps api worker`.
  Lưu ý khi rollback:
    - Cache Redis của bản mới dùng namespace v2. Bản cũ dùng key cũ, không thấy invalidate của bản mới, nên quyền có thể cũ tối đa 5 phút.
      Sau khi rollback, xoá `cobo_iam:effective_access:*` hoặc chờ hết TTL.
    - Nếu deploy lại bản mới trong vòng TTL thì xoá thêm `cobo_iam:effective_access_gen:v2:*`.
    - Trước khi chạy bản cũ, huỷ các approval rbac_matrix đang pending (API-13).
Issues / follow-ups:
  - Smoke QA có đăng nhập: chờ user tạo ~/.cobo/dev-qa.env.
    Script: smoke-pr-ab/smoke_dev_pr_ab.py, chạy bằng `python smoke_dev_pr_ab.py`.
  - FE PR-F: suppressForbiddenNavigation cho 403 mới (API-17/18).
```

## Smoke 2026-10-10 (không đăng nhập) + audit DB chỉ đọc

Lý do không đăng nhập: chưa có `~/.cobo/dev-qa.env` có mật khẩu persona. AI đã tạo sẵn file đó với các giá trị không bí mật; các ô `*_PASSWORD` để trống cho user điền.

**HTTP qua nginx :3000**
- `/healthz`, `/readyz`: 200.
- Không có token: `/api/v1/me`, `/me/effective-access`, `/admin/roles`, `/admin/invite-roles`, `/platform/cms/admin/users`, PATCH membership → 401.
- Refresh với token sai → 401.

**Log từ 04:30Z (lúc deploy)**
- api: 0 panic/fatal/ERROR, 0 response 5xx, 0 log "effective access cache invalidation failed".
- worker: 0 panic/fatal/ERROR.

**Redis**
- 0 key `effective_access:v2`, 0 key `effective_access_gen:v2`, 0 key namespace cũ.
- Chưa có request nào đã đăng nhập kể từ deploy; key cũ đã hết hạn.

**Audit DB (chỉ đọc)**

| Mục | Kết quả |
|---|---|
| A1 approval đang pending | 0 |
| A2 rbac thiếu `plan_digest` (BES-08/API-13) | 0, rollback BE an toàn về mặt này |
| A3 pending direct-remove quyền platform (ROLE-19) | 0 |
| A4 resource-scope rule có subject ở company khác (ROLE-02) | 0, không thấy dấu vết khai thác |
| A5 direct grant còn hiệu lực trên membership không active | 18 (16 `pending_verification`: quyền cấp lúc mời; 2 `inactive`). Sau H3 không còn tác dụng; không cần sửa. |
| A6 giá trị `membership_status` | `active` 84, `inactive` 2, `pending_verification` 12. Không có giá trị lạ, nên BES-12 không ảnh hưởng dữ liệu cũ. |
| A7 người giữ `rbac.manage` không có `platform.cms.view` ở c_001 | 13. Điều kiện của ROLE-13 **có thật** trên DEV (trước PR-B là khai thác được). |
| A8 người có quyền mời nhưng không có `rbac.manage` | 0. ROLE-03 phương án A hiện không chặn ai trên DEV. |
| A9 company không có primary admin đang active | 11 (thông tin; guard ROLE-05 dựa trên đếm admin cuối cùng) |
| A10 rule `alert_channel_prefs` trên DEV | 0. Smoke ROLE-12/BES-18 có đăng nhập sẽ SKIP nếu không tạo rule trước. |

**Cập nhật:** smoke có đăng nhập đã chạy với persona QA (`qa.persona.*@cobo.test`): **29/29 PASS**. Xem `06-release-verify.md`.
