# Risk review 2026-10-15: luồng đăng ký / đăng nhập / quên mật khẩu

## Review Scope
- Phạm vi: các luồng register, login, refresh/logout, forgot/reset password, verify/resend email, invitation validate/accept, select/switch company. Review theo luồng tại HEAD, không theo diff. Chi tiết ở `00-scope.md`.
- Commit:
  - `cobo_iam_services` `bf1b64b`, base `origin/main` `bea9376`;
  - `cobo_web_design` `c1e57037`, base `8d98b6eb`.
- Loại review: Backend Production Risk Review, có cả phía FE. Chỉ đọc, không sửa code, không ghi DB.
- 8 reviewer chạy song song: be-security, security-abuse, db-tx, idempotency, resilience-observability, concurrency-distributed, fe-security, secrets-cors.
- Main agent tự kiểm lại mọi CRITICAL/HIGH bằng code và bằng dữ liệu DEV ở chế độ chỉ đọc (`qa_ro`, `iptables -S`, GET public).
- ID finding gộp là `AUTH-xx`. ID gốc của reviewer ghi trong ngoặc.

## Executive Summary
- Critical: 1
- High: 4
- Medium: 9
- Low: 6

Top risks:
1. **AUTH-01:** form login trong bundle public được điền sẵn tài khoản seed. Cùng mật khẩu đó mở được cả tài khoản Platform Admin trên DEV. Đã xác minh bằng cách so bcrypt offline với hash trên DEV.
2. **AUTH-02:** đăng ký trước bằng email của nạn nhân thì chiếm được lời mời vào công ty sau này. Đăng ký không cần xác thực email mà vẫn thành admin doanh nghiệp.
3. **AUTH-03:** endpoint public gửi email gần như không giới hạn. Giới hạn OTP không bao giờ kích hoạt. :8080 public nên bỏ qua được rate limit của nginx. DEV gửi qua Gmail, nên hết quota là mất mọi email auth và nhắc CBTT.
4. **AUTH-04:** DEV chạy HTTP thường. Khoá RSA dùng cho login trên DEV chính là khoá đã commit vào repo.
5. **AUTH-05:** user staff kẹt ở `pending_email_verification` mà không có đường gửi lại link. DEV đang có 19 user như vậy.

Nếu chỉ sửa được 3 việc trước khi lên production: AUTH-01 (bỏ prefill, tách seed khỏi danh sách migration chính), AUTH-03 (giới hạn gửi email), AUTH-02 (bắt xác thực email trước khi gắn membership hoặc cấp quyền admin).

## Findings

### [CRITICAL] AUTH-01: Bundle FE điền sẵn tài khoản seed; cùng mật khẩu mở được Platform Admin trên DEV
- Category: security (lộ credential, chiếm tài khoản)
- Confidence: confirmed (code, bundle và dữ liệu DEV)
- Nguồn: FES-20, SC-03; C1/C4 của risk-review-2026-10-09 vẫn mở.
- Location:
  - `cobo_web_design/src/pages/public/LoginPage.tsx:21-22`: `useState('<email seed>')`, `useState('<mật khẩu seed>')`. Không có điều kiện `import.meta.env.DEV`. Comment ghi "Dev default".
  - `cobo_iam_services/migrations/0009_seed_authz_test_accounts.up.sql`, `0034_seed_org_structure_demo.up.sql`, `seed_dev_identity_authorization.sql`. Các file này nằm trong `migrations/run_dev_migrations.sh:33,42`, và đây là danh sách migration duy nhất.
  - Khoảng 14 script web và 31 script IAM dùng cùng credential (`run-phase-*-dev-e2e.mjs`, `.playwright-mcp/*.cjs`).
- Evidence (main agent kiểm lại ngày 15/10, chỉ đọc, không in giá trị):
  - bcrypt của mật khẩu điền sẵn khớp hash trên DEV của `u_admin_dn` và `u_admin_web`. Cả hai đều `active`.
  - `u_admin_dn` có role `admin_doanh_nghiep` (có `rbac.manage`) ở 2 công ty active, và có membership ở công ty thứ 3. Email chưa xác thực, nhưng login không chặn trường hợp này.
  - `u_admin_web` có `platform.cms.view`, tức Platform Admin, quản trị mọi công ty.
  - DEV `:3000` mở cho mọi nơi (`ufw: 3000/tcp ALLOW Anywhere`). `GET /login` trả 200.
- Trigger scenario:
  - Ai mở `http://<DEV>:3000/login` rồi bấm "Đăng nhập" là vào được admin doanh nghiệp của 2 công ty.
  - Mật khẩu nằm trong JS public. Login_id của platform admin theo cùng mẫu seed (có trong repo), nên đoán ra được Platform Admin.
- Impact:
  - Chiếm DEV: đọc và sửa membership, role, thông tin CBTT của mọi công ty; tạo user; đổi `verification_status`.
  - Môi trường mới nào chạy `run_dev_migrations.sh` (kể cả production nếu dựng theo script này) cũng sinh sẵn admin có mật khẩu đã biết.
  - DEV có tài khoản thật của người dùng (ví dụ `tvttth…@gmail.com`), nên dữ liệu không hoàn toàn là giả.
- Why current protection is insufficient: scan secret không bắt được literal trong `useState`. Không có guard nào chặn seed theo `ENV`.
- Recommended fix:
  1. FE: khởi tạo hai field bằng `''`. Nếu vẫn muốn prefill thì chỉ bật khi `import.meta.env.DEV` và có flag `VITE_*` opt-in. Thêm `autoComplete`.
  2. DEV (cần user duyệt vì là thao tác ghi dữ liệu): khoá hoặc đổi mật khẩu các tài khoản seed (`u_admin_dn`, `u_admin_web` và các seed cùng mật khẩu). QA dùng persona trong `~/.cobo/dev-qa.env`.
  3. BE: tách seed ra khỏi danh sách migration chính (chỉ chạy khi `ENV=development` và có flag riêng). Chuyển các script sang `devQaEnv`.
- Verification:
  - `npm run build`, sau đó grep `dist/assets/*.js` không thấy email seed.
  - Login DEV bằng cặp seed nhận 401.
  - Có test FE kiểm tra form login rỗng khi mount.

### [HIGH] AUTH-02: Đăng ký trước bằng email nạn nhân để chiếm lời mời vào công ty; đăng ký chưa xác thực vẫn thành admin
- Category: security (authz, cross-tenant)
- Confidence: confirmed (main agent đọc lại toàn bộ chuỗi gọi)
- Nguồn: SEC-01 (be-security). Liên quan DB-04, SEC-04.
- Location:
  - `internal/iam/registrationmysql/register_public.go:346-349` và `:420-423`: `INSERT INTO users ... VALUES (?, ?, ?, ?, NULL, 'active')`.
  - `internal/iam/app/service.go:109-118`: login chỉ chặn `pending_email_verification`.
  - `internal/companyaccess/app/admin_service.go:444-481`: `MembershipStatus` mặc định là `"active"`. `LookupUserByLoginID` tìm thấy user `active` thì gọi luôn `CreateMembership(... Status: req.MembershipStatus)`, không có bước chấp nhận.
- Trigger scenario:
  - Kẻ tấn công `POST /auth/register` bằng `victim@corp.com` và mật khẩu của chính họ. Tài khoản được tạo `active`, login được ngay.
  - Sau đó admin công ty X mời `victim@corp.com`. Hệ thống tìm thấy user của kẻ tấn công và tạo membership `active` với role admin đã chọn.
  - Kẻ tấn công login và được chọn sẵn vào công ty X.
- Impact:
  - Đọc thông tin CBTT chưa công bố của công ty X, với role được cấp.
  - Ngoài ra, register có `company_name` thì user chưa xác thực trở thành `admin_doanh_nghiep` ngay. Role này có `admin.membership.invite`, nên dùng được Cobo để gửi email mời tới địa chỉ bất kỳ.
  - DEV hiện có 38 user `active` mà `email_verified_at IS NULL`.
- Why current protection is insufficient: điều kiện `email_verified` chỉ có ở `InitializeCompany` và `CreateSelfServiceCompany` (`admin_service.go:1903,1943`).
- Recommended fix (cần chốt sản phẩm, xem Decisions):
  - Tạo user public ở trạng thái `pending_email_verification`, hoặc chặn mọi hành động có quyền cho tới khi có `email_verified_at`.
  - Khi mời user đã tồn tại: chỉ gắn thẳng nếu email đã xác thực. Nếu chưa, gửi token mời mà người dùng phải chấp nhận từ hộp thư.
- Verification: integration test với MySQL. Đăng ký X (chưa xác thực), admin mời X, kỳ vọng không có membership `active` cho tới khi X xác thực.

### [HIGH] AUTH-03: Endpoint public gửi email gần như không giới hạn; hết quota SMTP là mất email của mọi tenant
- Category: abuse / resource
- Confidence: confirmed (main agent kiểm lại code, firewall và cấu hình DEV)
- Nguồn: SEC-01 (security-abuse), DB-02, IDEM-02, CONC-03, SEC-02 (abuse), IDEM-03; BES-42 vẫn mở.
- Mâu thuẫn giữa các reviewer: be-security ghi "5 OTP/giờ, không có vấn đề". Main agent kiểm lại và thấy giới hạn này **không bao giờ kích hoạt** (chi tiết bên dưới), nên giữ kết luận của 4 reviewer còn lại.
- Location:
  - `internal/iam/app/service.go:258-279`: hàm `issueEmailVerificationOTP` gọi `CountEmailVerificationOTPsSince`, chặn khi `n >= 5`, rồi gọi `InvalidatePendingEmailVerificationOTPs`.
  - `internal/iam/infra/mysql/auth_recovery.go:222-225`: `DELETE FROM email_verification_otps WHERE user_id = ? AND consumed_at IS NULL`.
  - `service.go:470-509` (`ForgotPassword`): mỗi lần gọi INSERT một token và publish một email, không có bất kỳ kiểm tra tần suất nào.
  - `handler.go:181-198`: resend không cần đăng nhập, chỉ cần email.
  - `deploy-artifacts/web/nginx.conf:10,54`: `api_per_ip` 20 r/s, burst 40.
- Evidence trên DEV (chỉ đọc):
  - `cobo-iam-api 0.0.0.0:8080` được publish, và `DOCKER-USER` chỉ chặn 1025/3306/8025/9090/9093. Vậy :8080 public, đi thẳng vào API và bỏ qua `limit_req` của nginx.
  - `SMTP_HOST=smtp.gmail.com`: DEV gửi email thật qua một tài khoản Gmail có quota theo ngày.
- Trigger scenario:
  - Gọi lặp `resend-verification-email {email}` với một user chưa xác thực. DEV có 38 user `active` chưa xác thực và 19 user `pending_email_verification`; kẻ tấn công cũng tự đăng ký được.
  - Mỗi lần gọi đều xoá OTP cũ, nên số OTP đếm được luôn ≤ 1 và nhánh 429 không bao giờ chạy.
  - Hoặc gọi lặp `forgot-password` với bất kỳ email nào đã tồn tại.
  - Ví dụ: ở 20 r/s qua nginx, khoảng 500 email chỉ mất chừng 25 giây. Qua :8080 thì không có giới hạn.
- Impact:
  - Hộp thư nạn nhân bị spam.
  - Hết quota Gmail làm email reset, mời, OTP và **nhắc hạn CBTT** của mọi công ty lỗi tới hết ngày. AUTH-12 cho thấy lỗi này còn không có tín hiệu nào.
  - Mỗi OTP mới reset `attempt_count`, nên giới hạn 5 lần đoán OTP cũng mất tác dụng.
- Why current protection is insufficient: chỉ có giới hạn per-IP của nginx, và :8080 vượt qua được. App không có limiter theo người nhận. Test stub luôn trả count 0 (`service_test.go:202,572`).
- Recommended fix:
  - Đánh dấu OTP cũ là invalidated thay vì DELETE, hoặc đếm số lần gửi ở bảng riêng.
  - Thêm cooldown 60 giây và trần theo ngày cho mỗi người nhận, áp cho resend, forgot và register. Lưu counter trong MySQL/Redis, dùng chung giữa các replica.
  - Lấy IP client thật từ `X-Real-IP` khi request đi qua proxy tin cậy.
  - Chặn :8080 từ bên ngoài, hoặc giới hạn nguồn truy cập. Lưu ý `PUBLIC_API_BASE_URL` đang trỏ `:8080`, xem Decisions.
  - Bật `REGISTRATION_DISABLED=true` trên DEV nếu không cần đăng ký public (cờ này đã có ở `config.go:85-86,267`).
- Verification:
  - Test MySQL: resend lần thứ 6 trong 1 giờ trả 429.
  - Forgot lần thứ N+1 trong cửa sổ không tạo token và không tạo dòng outbox.
  - Từ ngoài allowlist, kết nối tới `:8080` bị DROP.

### [HIGH] AUTH-04: DEV chạy HTTP thường; khoá RSA dùng cho login đã commit trong repo và có trong mọi image
- Category: security (credential trên đường truyền)
- Confidence: confirmed (fingerprint khớp)
- Nguồn: SC-01, SC-02, FES-11, SEC-08 (BES-06).
- Location:
  - `configs/login_password_rsa_dev.pem` (commit `378faaa`), được `Makefile:152` chép lên DEV và `docker-compose.artifacts.yml:130-131` dùng.
  - `Dockerfile:10` chạy `COPY . .`, nên mọi image đều chứa khoá.
  - `internal/iam/transport/http/handler.go:129-133` vẫn nhận `password` dạng rõ.
  - `src/services/authApi.ts:298-304,334-340,417,432`: `confirm_password` và `new_password` gửi dạng rõ.
- Evidence:
  - SPKI SHA-256 của `GET /api/v1/auth/login-password-key` trên DEV (`kid=dev`) là `347191b4b817959b…`, trùng với khoá trong repo.
  - `.env` trên DEV không có `LOGIN_PASSWORD_RSA_PRIVATE_KEY_PEM` inline, nên không có khoá nào khác ghi đè.
- Trigger scenario:
  - Ai quan sát được đường mạng tới DEV (Wi-Fi công cộng, proxy) đều đọc được token và `confirm_password`/`new_password` dạng rõ. Với `password_cipher`, người có repo giải được bằng khoá đã commit.
  - Ciphertext không có nonce, nên replay được vào `/login`.
  - MITM làm endpoint key trả 5xx thì FE tự gửi mật khẩu dạng rõ (fail-open).
- Impact: lộ mật khẩu và token của người dùng DEV, trong đó có tài khoản thật và có thể dùng lại mật khẩu ở nơi khác.
  - Về production, khi thiếu env thì code không tự dùng khoá dev mà đi nhánh plaintext.
  - Nhưng `config.example.env:59-61` hướng operator trỏ tới khoá dev, và khoá có sẵn trong image.
- Why current protection is insufficient: lớp RSA chỉ là phòng thủ chiều sâu, và vô tác dụng khi khoá đã công khai trong repo. TLS mới là lớp bảo vệ thật, mà DEV chưa có.
- Recommended fix:
  - Đưa TLS lên DEV.
  - Sinh khoá chỉ nằm trên server (`cmd/genloginrsakey`). Xoá khoá khỏi tree, thêm vào `.gitignore` và `.dockerignore`. Khoá đã nằm trong history nên phải rotate.
  - Khi `ENV` khác development, từ chối khởi động nếu fingerprint trùng khoá dev.
  - Hoặc bỏ hẳn envelope và chỉ dựa vào TLS (xem Decisions).
- Verification: fingerprint khoá DEV khác khoá trong repo; có unit test cho guard lúc khởi động.

### [HIGH] AUTH-05: User staff kẹt ở `pending_email_verification`, không có đường gửi lại link
- Category: resilience / data
- Confidence: confirmed (code; DEV có 19 user ở trạng thái này)
- Nguồn: IDEM-01.
- Location:
  - `internal/companyaccess/app/admin_service.go:201-224`: `CreateUser`, các bước sau, rồi mới `IssueEmailVerificationLink`, và không nằm chung transaction.
  - `:676`, `:721`: resend-invitation chỉ chạy khi trạng thái là `invited`.
  - `internal/iam/app/service.go:109` (login 403), `:610`, `:656-661`, `:893-895`.
- Trigger scenario:
  1. Một bước sau `CreateUser` lỗi. Admin retry thì nhận 409 vì login_id đã tồn tại.
  2. Link hết hạn sau 24 giờ, hoặc outbox chuyển `failed_permanent`, hoặc publish lỗi bị nuốt (AUTH-12).
  3. `MarkEmailVerified` commit xong nhưng `Activate` lỗi.
  - Ở cả ba trường hợp: login trả 403, resend OTP đòi phải có session, resend-invitation trả 409.
- Impact: tài khoản không dùng được và cũng không tạo lại được bằng email đó. Chỉ sửa được bằng tay trong DB.
- Why current protection is insufficient: không có thao tác nào cho trạng thái `pending_email_verification`.
- Recommended fix:
  - Thêm endpoint cho admin doanh nghiệp gửi lại link xác thực khi user ở `pending_email_verification` (scope theo `sub.CompanyID`), kèm nút ở FE.
  - Gộp `MarkEmailVerified` và `Activate` vào một transaction.
  - Cho `IssueEmailVerificationLink` chạy cả khi email đã verified mà tài khoản vẫn pending.
- Verification: service test cho từng nhánh lỗi, sau đó re-issue, verify, và login thành công.

### [MEDIUM] AUTH-06: Rotate refresh token không có CAS, không phát hiện reuse, hạn phiên trượt mãi
- Category: concurrency / security
- Confidence: confirmed
- Nguồn: DB-01 (reviewer xếp HIGH, main agent hạ xuống MEDIUM), CONC-02, SEC-03 (be-security). H6 vẫn mở.
- Lý do hạ mức: hậu quả trực tiếp là logout ngẫu nhiên giữa các tab. Để chiếm phiên lâu dài thì trước đó phải trộm được refresh token, và FE đã single-flight refresh trong từng tab.
- Location: `internal/iam/infra/mysql/sessions.go:122-128`, `internal/iam/app/service.go:417-451`.
- Evidence: `UPDATE sessions SET refresh_token_hash=?, refresh_expires_at=? WHERE session_id=? AND revoked_at IS NULL`. Câu lệnh không so hash cũ.
- Trigger scenario:
  - Hai tab gửi cùng refresh token R0 thì cả hai đều nhận 200. Tab giữ token bị ghi đè sẽ nhận 401 ở lần sau.
  - Kẻ trộm dùng R trước thì nạn nhân nhận 401, còn phiên của kẻ trộm không bị thu hồi. Mỗi lần rotate lại cộng thêm 720 giờ.
- Impact: người dùng bị logout ngẫu nhiên. Token bị lộ dẫn tới chiếm phiên kéo dài.
- Recommended fix:
  - Thêm `AND refresh_token_hash = <hash cũ>` vào WHERE; `RowsAffected=0` thì trả 401.
  - Lưu hash trước đó; nếu hash cũ bị dùng lại ngoài thời gian grace thì revoke phiên.
  - Thêm hạn tuyệt đối cho phiên.
  - FE dùng `navigator.locks` để khoá refresh giữa các tab.
- Verification: test 2 goroutine refresh cùng token, chỉ đúng 1 request thành công; dùng lại token cũ thì phiên bị revoke.

### [MEDIUM] AUTH-07: Opaque access token và pre-company token chỉ nằm trong RAM, không hết hạn, không bao giờ bị xoá
- Category: distributed / resource
- Confidence: confirmed (code). Mức HIGH nếu chạy hơn 1 replica; số replica production chưa rõ.
- Nguồn: CONC-01, SEC-03 (một phần).
- Location: `internal/iam/infra/token/opaque/manager.go:17,32,52`; `jwt/manager.go:81-83,119-120` (pre-company token đi qua fallback opaque ngay cả ở mode jwt); `docker-compose.artifacts.yml:110` (DEV dùng opaque).
- Trigger scenario:
  - Từ 2 replica trở lên: token `ptk_` được cấp ở replica A, request select-company rơi vào B và nhận 401. User thuộc nhiều công ty không login được.
  - Với 1 replica: RAM tăng theo mỗi lần login/refresh/switch. Token "900 giây" thực tế sống tới khi phiên hết hạn.
  - Restart thì mọi access token mất hiệu lực.
- Impact: không scale ngang được; access token sống lâu hơn thiết kế; có nguy cơ OOM nếu process chạy dài.
- Recommended fix: lưu opaque/pre-company token vào MySQL/Redis kèm `expires_at`, hoặc dùng JWT ngắn hạn. Kiểm tra exp và prune. Trước mắt ghi rõ ràng buộc chỉ chạy single-replica.
- Verification: unit test token quá 900 giây bị từ chối; smoke 2 API sau LB cho luồng select-company.

### [MEDIUM] AUTH-08: Link reset/mời và OTP dạng thô nằm vĩnh viễn trong `outbox_events.payload_json`
- Category: security
- Confidence: confirmed (code và dữ liệu DEV)
- Nguồn: SC-04 (reviewer xếp HIGH, main agent hạ xuống MEDIUM), SEC-05, RES-05, IDEM-03. BES-04/OBX-04/SEC-29 vẫn mở.
- Lý do hạ mức: đã kiểm trên DEV và thấy Mailpit `:8025`/`:1025` và MySQL `:3306` chỉ mở cho một IP trong allowlist (`DOCKER-USER`). DEV gửi email qua Gmail, không qua Mailpit. Vậy còn lại đường đọc DB (`qa_ro`, backup).
- Location: `internal/iam/app/service.go:304-314,493-508,852-873`; `internal/platform/outbox/mysql/repository.go:128-141` (`MarkProcessed` chỉ đổi status, giữ nguyên payload).
- Evidence trên DEV: 67/80 dòng outbox `auth.*` có chuỗi `token=` trong payload. Hiện không còn reset token nào chưa hết hạn.
- Trigger scenario: người có quyền đọc DB lấy link còn hạn (reset 30 phút, mời 72 giờ) và dùng trước chủ tài khoản.
- Impact: chiếm tài khoản hoặc nhận lời mời thay chủ.
- Recommended fix: xoá `body` trong cùng lệnh UPDATE của `MarkProcessed`/`MarkFailedPermanent` cho event `auth.*`; thêm job retention. Lâu dài, chỉ lưu token id trong outbox và render link lúc gửi.
- Verification: `JSON_EXTRACT(payload_json,'$.body')` của các dòng `auth.*` đã processed là NULL.

### [MEDIUM] AUTH-09: Đổi email không cần xác thực; tra cứu khi khôi phục mật khẩu nhập nhằng vì email trùng
- Category: security / data
- Confidence: confirmed (DEV có một cặp user trùng email)
- Nguồn: SEC-04 (be-security), DB-04. BES-03 vẫn mở.
- Location:
  - `internal/companyaccess/infra/mysql/admin_repository_notification_account.go:177-181`, `internal/iam/transport/http/me_handler_profile.go:56`: đổi email không cần mật khẩu hiện tại, không cần xác thực email mới, và vẫn giữ `email_verified_at`.
  - `internal/iam/infra/mysql/auth_recovery.go:47-54`: `WHERE LOWER(TRIM(email)) = ? OR LOWER(TRIM(login_id)) = ? LIMIT 1`, không có ORDER BY, và `users.email` không có UNIQUE.
- Trigger scenario:
  - (a) Có access token bị trộm: đổi email sang hộp thư của kẻ tấn công, rồi forgot-password, link reset gửi tới kẻ tấn công.
  - (b) User A đặt email trùng login_id của B, forgot cho B có thể trả về dòng của A.
- Impact: một lần lộ token ngắn hạn thành chiếm tài khoản vĩnh viễn; người dùng bị khoá khỏi luồng khôi phục. Mỗi lần login/forgot đều full scan bảng `users`.
- Recommended fix: đổi email phải nhập mật khẩu hiện tại và xác thực địa chỉ mới; chỉ gửi reset tới email đã xác thực; tra `login_id = ?` trước; dedup rồi thêm cột email chuẩn hoá có index unique.
- Verification: test đổi email chưa xác thực thì link reset vẫn gửi tới email cũ. Operator chạy (chỉ đọc) `SELECT LOWER(TRIM(email)) e, COUNT(*) c FROM users GROUP BY e HAVING c>1;`.

### [MEDIUM] AUTH-10: Reset password không atomic: đốt token trước khi validate, bỏ qua lỗi revoke, token khác vẫn còn hiệu lực
- Category: transaction / security
- Confidence: confirmed
- Nguồn: DB-03, RES-03, IDEM-03, IDEM-04, SEC-07 (be-security), SEC-06 (abuse), CONC-06, FES-22. BES-08 vẫn mở.
- Location: `internal/iam/app/service.go:557-573`; `auth_recovery.go:68-77,150-160`; FE `ResetPasswordPage.tsx:34` (FE đếm độ dài chưa trim).
- Trigger scenario:
  - Mật khẩu chỉ đủ 12 ký tự nhờ khoảng trắng: FE cho qua, BE consume token rồi trả 422.
  - `RevokeAllByUser` lỗi: API vẫn trả 200, phiên của kẻ tấn công còn sống.
  - Hai link reset T1/T2: sau khi reset bằng T1, T2 vẫn reset được trong 30 phút.
  - Lỗi DB lúc consume trả 401 "token invalid" và không có log.
- Impact: reset password không chắc đẩy được kẻ chiếm tài khoản ra; người dùng bị kẹt với link đã chết.
- Recommended fix:
  - Validate mật khẩu trước khi consume token.
  - Trong 1 transaction: khoá token, update credential (kiểm `RowsAffected = 1`), set `used_at` cho mọi token còn mở của user, revoke session, rồi commit.
  - Trả lỗi nếu revoke lỗi.
  - Áp tương tự cho `ChangeAccountPassword`. FE dùng `trim().length`.
- Verification: mật khẩu 11 ký tự thì token vẫn dùng được; reset bằng T1 thì T2 trả 401; revoke lỗi thì API trả lỗi.

### [MEDIUM] AUTH-11: Không có lockout login; login sai không được ghi; IP ghi lại là của proxy; audit bị xoá được qua `X-Request-Id`
- Category: security / observability
- Confidence: confirmed
- Nguồn: SEC-02 (be-security), SEC-03 (abuse), CONC-04, RES-02. BES-42 và H4 vẫn mở.
- Location:
  - `internal/iam/app/service.go:388-394`: FE gửi `email`, nhưng ghi nhận lấy theo `req.LoginID`, nên login sai không được ghi.
  - `internal/iam/infra/mysql/login_attempts.go:20-42`: chỉ insert, không bao giờ đọc.
  - `handler.go:116`: `req.IP = r.RemoteAddr`, tức IP của nginx.
  - `internal/httpserver/server.go:1110-1114`: `X-Request-Id` không được validate, trong khi cột `request_id` là `VARCHAR(64)`.
- Trigger scenario:
  - Credential stuffing: khoảng 72k lần thử/giờ/IP qua nginx, không giới hạn qua :8080, không để lại dấu vết.
  - Gửi `X-Request-Id` dài hơn 64 ký tự thì INSERT audit lỗi (strict mode). Lỗi bị nuốt và dòng `login_failure` biến mất.
- Impact: không phát hiện và không chặn được brute force; mất bằng chứng điều tra.
- Recommended fix:
  - Ghi theo login_id đã chuẩn hoá, lấy từ email nếu thiếu.
  - Lấy IP thật từ `X-Real-IP` khi peer là proxy tin cậy.
  - Backoff tăng dần theo login_id và theo IP, dùng counter chung (Redis `INCR` có TTL, hoặc upsert MySQL), không hard lock.
  - Validate `X-Request-Id` (≤64 ký tự, charset an toàn), sai thì sinh mới; log warn khi audit lỗi.
  - Thêm metric `cobo_auth_login_total{outcome}`.
- Verification: login sai qua field `email` tạo một dòng có login_id; header dài vẫn còn dòng audit; lần sai thứ N bị 429 hoặc bị trễ.

### [MEDIUM] AUTH-12: Email auth (reset, xác thực, mời) bị mất mà không ai biết
- Category: observability / resilience
- Confidence: confirmed
- Nguồn: RES-01. OBX-02 vẫn mở.
- Location: `internal/iam/app/service.go:1018` (`_ = s.outbox.Publish(...)`), `:479-483`; `cmd/worker/main.go:383,409`; `internal/platform/outbox/processor.go:65-90` (`maxOutboxRetries = 10`, backoff 1→32 giây).
- Trigger scenario:
  - SMTP lỗi khoảng 3-4 phút (hoặc hết quota như ở AUTH-03): event chuyển `failed_permanent`, không có log hay metric nào.
  - INSERT outbox lỗi mà API vẫn trả 200.
  - `SMTP_HOST` rỗng: event bị coi như đã gửi.
- Impact: người dùng không nhận được link; on-call không có tín hiệu. Đây cũng là đường dẫn tới AUTH-05.
- Recommended fix:
  - Không nuốt lỗi publish.
  - Cho email auth đi qua cơ chế retry của `email.dispatch`.
  - Thêm counter `cobo_auth_email_total{outcome}`, gauge outbox `failed_permanent` và alert.
  - Đặt timeout cho SMTP (`boundedSendMail`).
  - Bắt buộc `SMTP_HOST` khi env không phải dev.
- Verification: test processor với handler luôn lỗi thì metric tăng; publisher lỗi khi gọi ForgotPassword thì có log hoặc trả lỗi.

### [MEDIUM] AUTH-13: Endpoint public chạy bcrypt trước khi validate, khuếch đại tải CPU
- Category: resource
- Confidence: confirmed về thứ tự gọi; mức CPU là plausible, chưa đo.
- Nguồn: SEC-04 (abuse).
- Location: `internal/iam/app/service.go:733-738` (accept-invitation), `:346-351` (register, 3 lần bcrypt).
- Trigger scenario: gửi token rác kèm mật khẩu 12 ký tự, mỗi request tốn khoảng 50-100 ms CPU. Một IP ở 20 r/s chiếm khoảng 1-2 core; qua :8080 thì không giới hạn.
- Recommended fix: kiểm tra token hoặc email trước rồi mới hash; thêm semaphore giới hạn số bcrypt chạy song song.
- Verification: benchmark với token sai thì không gọi bcrypt.

### [MEDIUM] AUTH-14: Phiên tạo khi accept-invite thiếu company context; lỗi tạo phiên bị bỏ qua
- Category: transaction / functional
- Confidence: plausible (chưa chạy thực tế)
- Nguồn: DB-05; ghi chú "invitation auto-login" của be-security.
- Location: `internal/iam/app/service.go:763-789`: `IssueAccessToken` chạy trước `sessions.Create`, và gặp `_ = s.sessions.Create(...)` không truyền `MembershipID`/`CompanyID`. `invitation_mysql.go:120-127`.
- Trigger scenario: sau khi accept, access token có claim company nhưng dòng session lưu NULL. Khi token hết hạn, `Refresh` trả 422 và người được mời bị logout. Nếu `Create` lỗi, client nhận token trỏ tới một session không tồn tại.
- Recommended fix: tạo session trước khi phát token, truyền membership/company đã qua `usableMemberships`; `Create` lỗi thì trả lỗi.
- Verification: test accept rồi refresh bằng token nhận được, kỳ vọng 200.

### [LOW] AUTH-15: Dò được user qua register 409, độ lệch thời gian login và 429 của resend
- Nguồn: SEC-06 (be-security), SEC-05 (abuse). Finding về timing từ review trước vẫn mở.
- Location: `credentials.go:38-45` (user không tồn tại thì bỏ qua bcrypt); `register_public.go:321-325`; `service.go:274-276`.
- Fix: so với một hash bcrypt giả khi không có user; resend luôn trả 200; cân nhắc register trả phản hồi chung (202).
- Confidence: confirmed.

### [LOW] AUTH-16: FE: login sai ở một tab làm refresh rồi xoá phiên của tab khác; không xử lý hết phiên giữa các tab
- Nguồn: FES-21, FES-10 (vẫn mở, phạm vi rộng hơn).
- Location: `src/services/authApi.ts:142-145,210-219,307-310`; `src/App.tsx:213-229,696-698`; `ResetPasswordPage.tsx:44-45`. Không có listener `storage`.
- Trigger scenario: tab A đã đăng nhập, tab B nhập sai mật khẩu. B nhận 401 nên gọi refresh bằng token của A, rồi `resetSession()`. Kết quả là A bị logout ngầm, hoặc A hiển thị lẫn ngữ cảnh của hai tài khoản.
- Fix: dùng client không gắn token cho login/register; thêm handler auth-failure chung (`resetSession()` + `hardNavigate('/login')`) và listener `storage`.
- Confidence: confirmed.

### [LOW] AUTH-17: Token dùng một lần nằm trong URL, log nginx và lịch sử trình duyệt
- Nguồn: FES-12 (vẫn mở, phạm vi rộng hơn).
- Location: `authApi.ts:393` (`invitations/validate?token=`); `ResetPasswordPage.tsx:12`, `VerifyEmailTokenPage.tsx:17`, `AcceptInvitationPage.tsx:52` (không gọi `replaceState`); `nginx.conf` không đặt `Referrer-Policy`.
- Fix: gọi `history.replaceState` ngay sau khi đọc token; đổi validate sang POST; thêm `Referrer-Policy: no-referrer`.
- Confidence: confirmed (giả định nginx dùng log format mặc định).

### [LOW] AUTH-18: Handler auth trả 500 khi JSON sai, không giới hạn body, log lỗi thiếu request_id
- Nguồn: RES-04.
- Location: `handler.go:108-110` (cùng mẫu ở các handler auth khác); `internal/platform/httpx/json.go`. :8080 public nên bỏ qua được giới hạn 5 MB của nginx.
- Fix: lỗi decode trả 400; bọc body bằng `MaxBytesReader` 16 KB; log kèm request_id, route và Cause.
- Confidence: confirmed.

### [LOW] AUTH-19: Mặc định không an toàn khi thiếu `ENV`; độ mạnh token
- Nguồn: SC-05, SEC-08 (be-security), BES-06.
- Location: `internal/platform/config/config.go:234,533-534`: thiếu `ENV` thì mặc định là `development` (CORS mở cho loopback, secret avatar đã biết, link reset localhost). `jwt/manager.go:96-110,129-134`: HS256 nhận secret dài bao nhiêu cũng được; không bắt buộc `exp`; `typ` rỗng vẫn được chấp nhận. Refresh token và opaque token là UUIDv7 (khoảng 62 bit ngẫu nhiên).
- Fix: không khởi động nếu thiếu `ENV`; HS256 bắt buộc secret ≥32 byte; thêm `WithExpirationRequired` và `typ=access`; token 32 byte từ crypto/rand.
- Confidence: confirmed.

### [LOW] AUTH-20: Register-user-only trả 500 khi race; bảng auth không có retention; goroutine fire-and-forget dùng ctx của request
- Nguồn: DB-06, CONC-05. OBX-06 vẫn mở.
- Location: `register_public.go:420-425`; `login_attempts.go:37-40`; `service.go:316-319,845-847`.
- Fix: map lỗi duplicate thành 409; thêm job xoá theo lô cho dữ liệu đã hết hạn; dùng `context.WithoutCancel` kèm timeout và `recover`, hoặc chuyển sang outbox.
- Lưu ý (từ fix AUTH-03): job retention của `email_verification_otps`/`password_reset_tokens` phải xoá theo `created_at` cũ hơn 24h, **không** theo `expires_at`. Lý do: OTP bị thay được cho hết hạn ngay mà vẫn phải được tính vào giới hạn gửi.
- Confidence: confirmed (500/retention); plausible (mất notification in-app).

## Trạng thái finding cũ
| ID cũ | Trạng thái 15/10 | Finding hiện tại |
|---|---|---|
| C1/C4 (risk-review-2026-10-09: credential seed, prefill) | **Mở**, đã xác minh khai thác được trên DEV | AUTH-01 |
| C6 (Mailpit `0.0.0.0:8025`) | **Đã giảm thiểu** bằng `DOCKER-USER` (chỉ một IP); publish vẫn là `0.0.0.0` | AUTH-08 |
| H23 (`:8080` publish ra ngoài) | **Mở**, :8080 không có trong `DOCKER-USER` | AUTH-03, 11, 13, 18 |
| H4 / BES-42 (không lockout, không giới hạn) | **Mở** | AUTH-03, AUTH-11 |
| H6 (refresh không CAS) | **Mở** | AUTH-06 |
| BES-04 / OBX-04 / SEC-29 (token trong outbox) | **Mở** | AUTH-08 |
| BES-03 (email trùng, đổi email) | **Mở** | AUTH-09 |
| BES-06 (độ mạnh token, replay RSA) | **Mở** | AUTH-04, AUTH-19 |
| BES-08 (reset đốt token) | **Mở** | AUTH-10 |
| BES-33 (membership inactive) | **Sửa một phần**: resolver và Refresh đã chặn, session chưa bị revoke nên `/me` vẫn chạy (LOW) | — |
| FES-10, FES-11, FES-12 | **Mở**; FES-10 và FES-12 rộng hơn trước | AUTH-16, AUTH-04, AUTH-17 |
| Dò email qua timing | **Mở** | AUTH-15 |
| OBX-02 (SMTP không timeout), OBX-06 | **Mở** | AUTH-12, AUTH-20 |

## Failure Scenarios Checked
- Brute force và credential stuffing qua nginx và qua :8080.
- Spam email qua resend, forgot và register; ảnh hưởng tới quota SMTP dùng chung.
- Hai tab hoặc hai request refresh cùng lúc; refresh token bị trộm.
- Reset password khi mật khẩu sai policy, khi revoke lỗi, khi có nhiều token cùng tồn tại.
- Accept invitation: chạy hai lần, token sai, phiên sau khi accept.
- Đăng ký trước bằng email nạn nhân rồi bị mời vào công ty; đổi email rồi forgot.
- Retry sau lỗi giữa chừng khi admin tạo staff; link hết hạn; outbox `failed_permanent`.
- Nhiều replica với opaque token; restart API.
- Lộ token qua outbox, log nginx, URL, Mailpit.
- Open redirect sau login, XSS trên trang public, CSRF.

## Tool-assisted verification (lệnh thực sự đã chạy)
- `go vet ./internal/iam/...` → rc=0. `go test ./internal/iam/... -count=1` → 8 package ok. `staticcheck ./internal/iam/...` → 1 U1000 trong test.
- Reviewer: `go vet` cho iam/httpserver/outbox/httpx sạch; `go test ./internal/iam/app/ ./internal/iam/loginpassword/` pass; `go test -race` cho `token/...`, `loginpassword`, `inmemory` pass.
- `diff_hygiene.py` (BE/FE) và `scan_secrets.py`: xem `00-scope.md`.
- Main agent kiểm trên DEV, chỉ đọc:
  - Python `bcrypt.checkpw` offline, so literal trong `LoginPage.tsx` với hash đọc qua `run_sql` (`qa_ro`). Kết quả: khớp `u_admin_dn` và `u_admin_web`. Không in giá trị.
  - Đếm qua `qa_ro`: 38 user active chưa xác thực; 67/80 dòng outbox `auth.*` có `token=`; 0 reset token còn hạn; quyền của 2 tài khoản seed.
  - `curl` status: `:3000/login` 200, `:8025/` 200 (từ IP trong allowlist), `GET /api/v1/auth/login-password-key` 200. So fingerprint SPKI với `openssl pkey -pubout`.
  - TCP connect từ workstation tới 8025/1025/3306/8080: đều mở. Workstation nằm trong allowlist nên kết quả này chưa nói được gì về truy cập từ ngoài.
  - SSH chỉ đọc: `ufw status`, `iptables -S INPUT`, `iptables -S DOCKER-USER`, `docker ps` (cột Ports); kiểm tra có biến RSA inline trong `.env` hay không (rc=0), đọc `SMTP_HOST`.
- Không chạy: `govulncheck`, `npm audit` (cần mạng); test MySQL tích hợp (không có `MYSQL_TEST_DSN`); đo timing; tải thật; mọi thao tác ghi.

## Areas reviewed with no material finding
- Token reset/verify/mời: 32 byte từ crypto/rand, lưu SHA-256, dùng một lần với `FOR UPDATE` + `used_at`, kiểm hết hạn trong SQL.
- Verify OTP: `UserID` lấy từ bearer, `max_attempts=5` có khoá. Lưu ý: giới hạn này bị vô hiệu bởi AUTH-03.
- Select/switch company: user và session lấy từ token, sau đó `GetActiveMembership` và kiểm status công ty. Không có IDOR.
- Logout và revoke session đều scope theo user. Đổi hoặc reset mật khẩu revoke mọi session (trừ trường hợp lỗi ở AUTH-10). Không có session fixation.
- JWT: allowlist thuật toán, bắt buộc issuer/audience, leeway 60 giây.
- CORS: allowlist chính xác, có `Vary: Origin`, từ chối origin `null`, không có `Allow-Credentials`. Không dùng cookie.
- Link reset/verify build từ `PUBLIC_WEB_BASE_URL`, không từ header Host.
- Không có open redirect sau login; không có HTML sink trên trang public; không có CSRF vì chỉ dùng bearer.
- Không có log nào ghi mật khẩu, token hay link. Response lỗi không lộ SQL.
- `http.Server` có timeout và graceful shutdown.
- Register dùng 1 transaction, có `uk_users_login_id`, không có mass assignment.

## Residual risks / unknowns
- Số replica production và token mode production: quyết định mức của AUTH-07.
- Quota thực tế của tài khoản Gmail trên DEV và mức CPU host: ảnh hưởng tới thời gian để AUTH-03 và AUTH-13 gây sự cố.
- MySQL strict mode thật, cùng với AUTH-11 (`X-Request-Id`).
- Visibility của repo `cobo_iam_services`: chỉ thấy repo FE là private. Ảnh hưởng tới mức lộ của AUTH-01 và AUTH-04.
- Có CDN/LB phía trước nginx hay không: nếu có thì mọi client dùng chung một bucket per-IP.
- Chưa review: OAuth consent redirect (`TemplateBuilderOAuthAuthorizePage.tsx:50`), SSO/MFA bridge.

## Quick wins (≤1h)
1. **FE:** bỏ prefill ở `LoginPage.tsx:21-22`, thêm test form rỗng (AUTH-01, phần code).
2. **DEV ops (cần user duyệt):** khoá hoặc đổi mật khẩu các tài khoản seed trên DEV (AUTH-01). Bật `REGISTRATION_DISABLED=true` nếu DEV không cần đăng ký public (AUTH-02, AUTH-03).
3. **DEV ops (cần user duyệt):** thêm `DOCKER-USER` DROP cho `:8080` từ ngoài allowlist, hoặc bind `127.0.0.1` (AUTH-03, 11, 13, 18). Kiểm trước mọi chỗ dùng `PUBLIC_API_BASE_URL=…:8080`.
4. **BE:** sửa limiter OTP, đổi DELETE thành đánh dấu invalidated, kèm test MySQL (AUTH-03, một phần).
5. **BE:** `ResetPassword` validate trước khi consume; trả lỗi khi revoke lỗi (AUTH-10, một phần).
6. **FE:** login/register dùng client không gắn token (AUTH-16).
7. **BE:** validate `X-Request-Id`; lỗi decode trả 400; thêm `MaxBytesReader` (AUTH-11, AUTH-18).

## Decisions needed from the team
1. **Đăng ký public (AUTH-02):** có bắt xác thực email trước khi login, hoặc trước khi thành admin/được gắn vào công ty không? Đây là thay đổi UX cho luồng đăng ký.
2. **Mời user đã tồn tại (AUTH-02):** gắn thẳng như hiện nay, hay bắt chấp nhận qua email khi email chưa xác thực?
3. **TLS cho DEV và envelope RSA (AUTH-04):** đưa DEV lên HTTPS (domain + cert)? Sau đó bỏ envelope RSA, hay giữ và rotate khoá?
4. **Seed migration (AUTH-01):** tách khỏi danh sách chính hay chỉ chạy khi có cờ dev? Các script QA đang phụ thuộc vào seed thì chuyển sang persona.
5. **Production:** số replica và token mode (AUTH-07); có Redis cho limiter dùng chung không (AUTH-03, AUTH-11).
6. **Chính sách giới hạn:** cụ thể cooldown, trần/ngày, backoff hay hard lock.
7. **SMTP:** tiếp tục dùng Gmail cá nhân cho DEV hay chuyển sang relay riêng có quota và alert (AUTH-03, AUTH-12)?
8. **`:8080` public:** còn dùng `PUBLIC_API_BASE_URL=…:8080` cho việc gì không, có đóng được không?

## Suggested fix plan (grouped by MR)
| MR | Repo | Nội dung | Finding |
|---|---|---|---|
| OPS-1 (cần duyệt) | DEV server | Khoá/đổi mật khẩu tài khoản seed; chặn :8080 từ ngoài; `REGISTRATION_DISABLED` | AUTH-01, 02, 03 |
| MR-1 | FE | Bỏ prefill; login/register không gắn token; `trim().length`; `replaceState` token; xử lý auth-failure và listener `storage` | AUTH-01, 10, 16, 17 |
| MR-2 | BE | Limiter gửi email (OTP, forgot, resend, register) dùng counter chung; IP thật | AUTH-03, 11 (một phần) |
| MR-3 | BE | Reset password atomic; vô hiệu mọi token của user; trả lỗi khi revoke lỗi; `ChangeAccountPassword` | AUTH-10 |
| MR-4 | BE | Refresh có CAS, phát hiện reuse, hạn tuyệt đối; opaque token có TTL và prune, hoặc lưu chung | AUTH-06, 07 |
| MR-5 | BE (+FE nhỏ) | Xác thực email khi đăng ký và khi mời user đã tồn tại; đổi email phải xác thực (sau quyết định 1-2) | AUTH-02, 09 |
| MR-6 | BE | Xoá payload outbox và thêm retention; metric/alert cho email auth; không nuốt lỗi publish; timeout SMTP | AUTH-08, 12 |
| MR-7 | BE + FE | Endpoint admin gửi lại link xác thực cho `pending_email_verification`; verify và activate cùng transaction | AUTH-05 |
| MR-8 | BE + infra | Gỡ khoá RSA khỏi repo/image, rotate, guard lúc khởi động; TLS DEV (sau quyết định 3); seed chỉ cho dev | AUTH-01, 04, 19 |
| MR-9 | BE | Lockout/backoff, ghi nhận login sai, `X-Request-Id`, 400/`MaxBytesReader`, bcrypt sau validate, bcrypt giả, session khi accept-invite, 409 khi race | AUTH-11, 13, 14, 15, 18, 20 |

Thứ tự đề xuất: OPS-1 và MR-1 (trong ngày) → MR-2 và MR-3 → MR-7 → MR-4, MR-6 → MR-5 và MR-8 sau khi chốt quyết định → MR-9.

## Cập nhật 15/10/2026: trạng thái sửa (user đồng ý sửa AUTH-01 và AUTH-03)
- **AUTH-01:** phần FE đã sửa (form trống, có test, bundle sạch). Xem `cobo_web_design/docs/ai-cache/bug-auth01-login-prefill-2026-10-15/`. Finding **vẫn mở** cho tới khi xong OPS-1 (khoá/đổi mật khẩu seed trên DEV), MR-8 (tách seed) và deploy FE.
- **AUTH-03:** BE đã sửa. Xem `docs/ai-cache/bug-auth03-auth-email-limits-2026-10-15/`.
  - Trần theo user được kiểm trong tx có lock.
  - Resend ẩn danh không gửi nữa.
  - nginx giới hạn 3 req/phút/IP.
  - Còn mở một phần: chưa có trần tổng/tách quota SMTP (chờ quyết định), và `:8080` public (OPS-1).
- Chưa commit, chưa deploy.
