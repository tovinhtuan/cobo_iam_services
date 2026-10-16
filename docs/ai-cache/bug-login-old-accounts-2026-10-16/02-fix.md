# Sửa lỗi và kiểm chứng (16/10/2026, đã deploy DEV)

User duyệt "làm cả 3 theo khuyến nghị" (BUG-1, BUG-2, đổi email seed trên DEV).

## BUG-1: lần đăng nhập thất bại không được ghi
- `LoginRequest.Identifier()` (`internal/iam/app/contracts.go`): `login_id` đã trim; nếu rỗng thì dùng `email` (form web gửi `email`).
- `LoginRequest.RecordedIdentifier()`: Identifier viết thường, cắt 191 rune (review BES-03: audit trước đây không giới hạn độ dài).
- Dùng ở `Login`, `recordLoginAttempt` (`service.go`) và audit `login_failure` (`transport/http/handler.go`).
- `login_attempts.go`: `truncStr` chuyển chuỗi sang UTF-8 hợp lệ và cắt theo rune cho login_id, IP, User-Agent (review BES-04). Trước đây byte lạ hoặc rune bị cắt đôi làm INSERT lỗi, và lần thất bại không được ghi.
- Không có code nào đọc `login_attempts`, nên ghi thêm lần thất bại không kéo theo khoá tài khoản.

## BUG-2: quên mật khẩu chọn tài khoản ngẫu nhiên
- **Thiết kế đã duyệt ban đầu:** ưu tiên `login_id`, từ chối khi trùng. `be-security-reviewer` (BES-01, MEDIUM) chỉ ra thiết kế này làm việc chặn khôi phục thành chắc chắn:
  - kẻ tấn công đăng ký `login_id` = email nạn nhân (khi nạn nhân có `login_id` ≠ email) rồi đổi email của mình;
  - hoặc đặt email của mình trùng email nạn nhân, thành "trùng" nên bị từ chối.
- **Thiết kế đã làm (thay thế):** `FindRecoveryUsers(ctx, address, limit)` thay cho `FindUserByEmail`.
  - Gom mọi tài khoản có hộp thư (`COALESCE(NULLIF(TRIM(email),''), login_id)`) bằng địa chỉ đã nhập, cũ nhất trước, tối đa `limit`.
  - Nếu không có tài khoản nào, mới tìm theo `login_id` (thư về hộp thư của chính tài khoản đó).
  - `ForgotPassword` phát một link cho **mỗi** tài khoản (tối đa 5; log `password_reset_address_shared` khi nhiều hơn). Thư luôn tới đúng địa chỉ đã nhập, nên email trùng không còn chặn được ai.
  - Lỗi tra cứu ghi log `password_reset_lookup_failed`; trước đây bị bỏ qua không log. Phản hồi luôn là `{success:true}`.
- MySQL (`infra/mysql/auth_recovery.go`) và in-memory (`infra/inmemory/credentials.go`) theo cùng quy tắc. Bản in-memory sắp theo user_id, không còn phụ thuộc thứ tự lặp map.

## Test
| Test | Bằng chứng |
|---|---|
| `TestLogin_RecordsFailedAttemptsSentWithTheEmailAlias`, `TestLoginRequest_*` | Fail với code cũ (`got: []`), pass sau khi sửa |
| `TestFindRecoveryUsers_EveryAccountOfTheMailboxThenLoginIDOwner` (in-memory, lặp 30 lần) | Bản trước dùng chung kịch bản: code cũ trả `u_a` cho email trùng |
| `TestAuthRecovery_FindRecoveryUsers_*` (MySQL 8.0 thật, `MYSQL_TEST_DSN`) | Pass. Với code cũ, cùng kịch bản trả về tài khoản giữ email (`a-holder`) thay vì đúng tài khoản, giống DEV |
| `TestForgotPassword_*` (3 test service) | Mỗi tài khoản một token + một thư tới địa chỉ đã nhập; giới hạn 5; lỗi tra cứu hoặc không có tài khoản thì không gửi gì |
| `TestTruncStr_KeepsValidUTF8WithinTheRuneLimit` | Byte lạ thành U+FFFD, cắt theo rune |

## Lệnh
| Lệnh | Kết quả |
|---|---|
| `go test ./internal/iam/...` (kèm `-race` + MySQL) | ok |
| `go test ./...` so với HEAD (`git archive`) | 0 fail mới (HEAD 21, sau sửa 20; `TestSchemaParityConditionalPeriodicity` chỉ fail ở bản archive) |
| `go vet ./internal/iam/...` | sạch (cảnh báo ở `internal/workflowfulfillment` có từ trước) |
| `docker compose -f docker-compose.dev.yml build api` | Built |
| gofmt các file đã sửa | sạch (`email_baseline_test.go` lệch gofmt từ trước, không đụng) |
| Test AUTH-03 `AuthRecoveryLimits` | Pass với schema 0001/0007/0027. Fail khi DB có thêm 0011, vì seed của test xoá `users` trước `user_subscription_tiers` (FK). Đây là giới hạn có sẵn của test |

## Review (`be-security-reviewer`)
| ID | Mức | Xử lý |
|---|---|---|
| BES-01 | MEDIUM | Đã sửa bằng thiết kế mới (trên) |
| BES-02 | MEDIUM, có từ trước | **Chưa sửa (follow-up):** `PATCH /api/v1/me/profile` đổi email không cần mật khẩu, không xác minh, không giữ trạng thái `email_verified_at`. Có token truy cập của người khác là chiếm được tài khoản qua reset |
| BES-03 | MEDIUM | Đã sửa: định danh trong audit cắt 191 rune. Còn lại: `http.MaxBytesReader` cho login/register/forgot (follow-up) |
| BES-04 | LOW | Đã sửa trong `login_attempts`. Repo audit cũng cần tương tự (follow-up) |
| BES-05 | LOW | Chấp nhận: lần thất bại lưu định danh đã gõ (có thể là mật khẩu dán nhầm ô). Không API nào trả ra; cần retention cho `login_failure` (follow-up) |
| BES-06 | LOW | Đã thêm test service. Nhánh "trùng" không còn |
| BES-07, BES-08 | INFO | Ghi nhận: chênh lệch thời gian phản hồi có từ trước; IP ghi được là IP của nginx |

## Mục 3: đổi email tài khoản seed trên DEV
- **Chưa làm.** Lệnh ghi DB DEV bị bộ kiểm tra quyền tự động của Claude Code chặn ("Modify Shared Resources").
- Đã soạn `refs/set_seed_emails_dev.py`:
  - chỉ cho 15 tài khoản seed của OPS-1;
  - kiểm trùng địa chỉ; backup giá trị cũ vào `~/.cobo/backups` (quyền 600);
  - cập nhật trong transaction có điều kiện chặn; kiểm tra lại bằng `qa_ro`.
- Dry run đã chạy và đúng.

## Dọn dẹp
- Container MySQL tạm `cobo-bugfue-mysql` (cổng 127.0.0.1:33991) đang dừng, chưa xoá (lệnh xoá bị kiểm tra an toàn chặn).

## Deploy DEV 16/10/2026
- BE `d32cd22` (docs `81eacdd`). Lệnh: `sh deploy-dev.sh be --skip-tests`, exit 0. Container api và worker được tạo lại. `/healthz` ok, `/readyz` ready, login-key qua `:3000` 200.
- Binary trên server trùng bản build cục bộ: api `826637c2…`, worker `56010cce…`.
- Không có migration mới.
- **Rollback:** `/root/cobo_project/releases/pre-bugfix-login-old-accounts-20261016T020229Z.tar.gz` (quyền 600, gồm `bin/api`, `bin/worker`, `configs`; api `1e34916f…`, worker `a98b4be6…`).
  - Giải nén đè vào `/root/cobo_project`, rồi `docker compose -f docker-compose.artifacts.yml up -d --force-recreate --no-deps api worker`.
- Log api/worker sau khởi động: 0 dòng error/panic.

## Smoke DEV (`refs/smoke_bugfix_dev.py` → `refs/smoke-bugfix-dev-result.json`): 8/8
| Bước | Kết quả |
|---|---|
| Đăng nhập sai với tài khoản không tồn tại, gửi qua `email` | 401. `login_attempts` có dòng (success=0, `INVALID_CREDENTIALS`). Audit `login_failure` có `login_id` |
| Đăng nhập sai mật khẩu với persona USER | 401; có dòng `login_attempts` |
| User-Agent chứa byte 0xFF | Có dòng `login_attempts`, UA đã chuyển thành U+FFFD. **Audit không có dòng**: repo audit chưa làm sạch chuỗi (follow-up BES-04, đúng như reviewer dự đoán) |
| Đăng nhập đúng (persona ENT) | 200, có thêm dòng thành công |
| Quên mật khẩu, persona USER (một hộp thư, một tài khoản) | 200; thêm 1 token; sự kiện outbox `processed`, `to` = hộp thư của tài khoản |
| Quên mật khẩu, địa chỉ không tồn tại | 200; không phát token nào |
| Log API | 0 dòng `password_reset_lookup_failed`/panic |

- Trường hợp nhiều tài khoản chung một hộp thư không smoke trên DEV, vì sẽ gửi thư thật vào gmail của người dùng. Đã có test với MySQL 8.0 thật và test service.
- Hồi quy trình duyệt (cobo_web_design):
  - AUTH-01 `smoke-login-empty-dev.mjs` 2/2;
  - S1 `smoke-s1-dev.mjs` 3/3, với 5 lỗi 401 console như baseline.
- Tác dụng phụ: 1 email reset gửi tới địa chỉ persona USER (`@cobo.test`); 4 dòng `login_attempts`, 3 dòng audit.

## Còn lại
- Mục 3 (đổi email 3 tài khoản seed) **vẫn chưa chạy**: lúc deploy, `u_admin_dn` vẫn dùng gmail của người dùng, `u_platform_tenant_admin` vẫn `@example.com`, `u_cms_operator` chưa có email. Người dùng chạy `refs/set_seed_emails_dev.py ... --apply`.

