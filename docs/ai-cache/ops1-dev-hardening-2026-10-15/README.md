# OPS-1 trên DEV (15/10/2026): khoá tài khoản seed, đóng `:8080`

- Nguồn: `docs/ai-cache/risk-review-auth-flows-2026-10-15/10-risk-report.md` (AUTH-01, AUTH-03; Quick wins 2-3).
- User yêu cầu: "làm OPS-1 trên DEV".
- Không làm `REGISTRATION_DISABLED=true`: đây là quyết định sản phẩm, user chưa chọn.

## 1. Đổi mật khẩu tài khoản seed (AUTH-01)
- Cách tìm: so bcrypt (offline, đọc hash qua `qa_ro`) giữa mật khẩu mà bundle cũ điền sẵn và 77 credential đang active. Có **16** tài khoản khớp.
- Đã đổi **15** tài khoản seed:
  - `u_admin_dn`, `u_admin_web`, `u_cms_operator`, `u_platform_tenant_admin`, `u_123`, `u_single`, `u_no_company`;
  - `u_truong_phong`, `u_truong_nhom`, `u_nhan_vien`;
  - `usr_org_admin_001`, `usr_org_nv_nhom_001`, `usr_org_nv_phong_001`, `usr_org_tn_001`, `usr_org_tp_001`.
- Cách đổi:
  - một mật khẩu ngẫu nhiên mới; bcrypt bằng `scripts/devqa/bcrypthash`;
  - `UPDATE credentials ... WHERE credential_id=? AND password_hash=<cũ>` trong một transaction, chạy bằng root của container theo cách `provision_qa_accounts.py` đang làm;
  - thu hồi mọi session còn mở của 15 user này (2.600 session, `revoked_reason='ops1_seed_password_rotation'`).
- **Không đổi** tài khoản `019e6d62-…` (`tua…@gmail.com`, có 2 membership). Tài khoản này trông như của một người thật mà lại dùng mật khẩu seed. Cần chủ tài khoản tự đổi mật khẩu, hoặc user quyết định đổi hộ.
- Mật khẩu mới: chỉ lưu ở `~/.cobo/dev-qa.env`, key `DEV_SEED_ACCOUNTS_PASSWORD`, quyền 600. Template `dev-qa.env.example` và `dev-qa/README.md` đã cập nhật.
- Sao lưu và rollback:
  - `~/.cobo/backups/ops1-seed-credentials-20261015T082139Z.json` (0600, ngoài git) chứa hash cũ và danh sách session_id đã thu hồi;
  - rollback bằng cách `UPDATE credentials SET password_hash=<cũ>` theo `credential_id`. Không nên rollback, vì mật khẩu cũ đã công khai trong bundle.
- Kiểm chứng:
  - mật khẩu cũ → 401 cho cả 15 tài khoản;
  - mật khẩu mới với `u_admin_dn` → 200;
  - persona ENT, CMS, USER → 200.
- Ảnh hưởng: các script QA cũ còn gắn cứng mật khẩu seed (khoảng 31 file IAM, 14 file web) sẽ nhận 401. Cần chuyển sang `devQaEnv` (MR-8).

## 2. Đóng `:8080` từ ngoài (AUTH-03, H23)
Trước khi chặn, link ảnh đại diện và link upload media CMS đều được tạo từ `PUBLIC_API_BASE_URL=http://88.216.208.0:8080`, tức trình duyệt gọi thẳng `:8080`. Vì vậy phải chuyển chúng sang nginx trước.

| Bước | Thay đổi | Kiểm chứng |
|---|---|---|
| a | `nginx.conf`: `location ^~ /api/v1/platform/cms/media/upload/` với `client_max_body_size 21m`, vì media CMS tối đa 20MB; các đường khác giữ 5m. Commit `59a0eb0`, deploy FE | Container cục bộ: upload 12MB vào đường media → 200, vào `/api` khác → 413. Trên DEV, `nginx -T` thấy location mới |
| b | `PUBLIC_API_BASE_URL` → `http://88.216.208.0:3000` trong compose, cả repo (`59a0eb0`) lẫn server; recreate api + worker | `printenv` trong api/worker → `:3000`. `/me` của `u_admin_dn` trả `avatar_url` bắt đầu bằng `http://88.216.208.0:3000`; tải link đó → 200 `image/png` |
| c | Thêm `8080` vào `/root/firewall-backup/phase2/apply-docker-user.sh` (script của `cobo-docker-firewall.service`, enabled), `systemctl restart` | `iptables -S DOCKER-USER` có `! -s 14.160.70.230/32 ... --ctorigdstport 8080 -j DROP`; tổng 6 rule DROP. Từ IP trong allowlist, `:8080` và `:3000` đều 200 |

- Không có dữ liệu nào lưu URL `:8080` (đã quét 88 cột url/text/json). DEV chưa có media CMS nào lớn hơn 5MB.
- Chưa kiểm được: request từ IP ngoài allowlist có thực sự bị DROP không, vì không có máy ngoài để thử. Rule dùng đúng mẫu đang chặn 3306, 8025 và các cổng khác.
- Sao lưu trên server (hậu tố `pre-ops1-20261015T082245Z`):
  - `web/releases/pre-ops1-….tar.gz` (dist và nginx.conf);
  - `docker-compose.artifacts.yml.pre-ops1-…`;
  - `apply-docker-user.sh.pre-ops1-…`;
  - `docker-user-before-ops1-….txt`.
- Rollback:
  1. Chép lại script firewall cũ rồi `systemctl restart cobo-docker-firewall`. Lưu ý script chỉ thêm rule: muốn bỏ hẳn rule 8080 phải chạy `iptables -D DOCKER-USER ... 8080 ...`.
  2. Chép lại compose cũ rồi `up -d --force-recreate --no-deps api worker`.
  3. nginx: giải nén bản `pre-ops1`.
- Vẫn còn gọi `:8080`: các script và tài liệu cũ (`deploy-artifacts/t5-login.mjs`, `scripts/qa-account-settings-smoke.sh`, `run-workflow-*.mjs`, `configs/config.example.env`). Chúng vẫn chạy được từ máy trong allowlist; `docs/deploy-dev-guide.md` đã cập nhật.

## 3. Smoke sau OPS-1
- `admin-shell-smoke` 9/9 và `admin-step3-smoke` 12/12 (`refs/admin-shell`, `refs/admin-step3`): 0 lỗi console, 0 request lỗi.
- Login trống 2/2.
- Log api/worker 15 phút sau: 0 dòng panic/ERROR. Outbox `failed_permanent` trong 30 phút: 0.
