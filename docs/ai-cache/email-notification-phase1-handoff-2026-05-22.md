# HANDOFF NOTE CHO SESSION TIẾP THEO

## 1. Bối cảnh tổng quan

- Dự án đang làm: triển khai **Phase 1** của email notification system trong repo `cobo_iam_services`.
- Nguồn sự thật của scope:  
  `/home/icom/go/src/myself/backend_api_cobo/cobo_iam_services/docs/email-notification-system-implementation-plan.md`
- User yêu cầu: **bám sát plan, không tự đoán**, triển khai logic phase 1 theo đúng roadmap.
- Logic nằm trong:
  - `internal/iam/app`
  - `internal/reminder/infra/email`
  - `internal/notification/*`
  - runtime wiring ở `internal/httpserver/server.go`, `cmd/worker/main.go`
- Kết quả cuối cùng mong muốn của phase 1:
  - tách subject/body email hardcoded sang `embed.FS`
  - render qua template mới khi bật `EMAIL_TEMPLATE_SOURCE=embed`
  - fallback về legacy nếu resolve/render lỗi
  - **không đổi behavior nghiệp vụ**
  - **không đổi outbox payload auth**
  - **không đổi token/OTP/TTL**
  - **không đổi reminder retry/delivery path**

## 2. Trạng thái hiện tại

### Đã triển khai

- Thêm template registry + renderer:
  - `internal/notification/app/email_contracts.go`
  - `internal/notification/app/email_renderer.go`
  - `internal/notification/infra/registry/embed_registry.go`
- Thêm template tree:
  - `internal/notification/templates/*`
- Thêm config:
  - `internal/platform/config/config.go`
  - env mới: `EMAIL_TEMPLATE_SOURCE=legacy|embed`
- Wire IAM auth email sang template layer:
  - `internal/iam/app/hooks.go`
  - `internal/iam/app/service.go`
- Wire reminder SMTP sender sang template layer:
  - `internal/reminder/infra/email/smtp_sender.go`
- Wire dependency injection:
  - `internal/httpserver/server.go`
  - `cmd/worker/main.go`
- Thêm test:
  - `internal/notification/app/email_renderer_test.go`
  - `internal/notification/infra/registry/embed_registry_test.go`
  - `internal/reminder/infra/email/smtp_sender_test.go`
  - `internal/iam/app/service_test.go`

### Đã verify

- `go test ./internal/notification/app ./internal/notification/infra/registry ./internal/reminder/infra/email ./internal/iam/app` ✅
- `go test ./internal/httpserver ./cmd/worker` ✅
- `docker compose -f docker-compose.dev.yml build api` ✅

### Đã đúng / đã chốt

- Phase 1 chỉ extract template + wire flag-based rendering.
- Auth outbox event vẫn publish payload `to`, `subject`, `body`.
- Reminder vẫn direct SMTP như cũ.
- Fallback legacy là bắt buộc.

### Mới chỉ là giả định hoặc chưa mở rộng

- Chỉ có locale `vi`; locale fallback đã có nhưng chưa có locale khác.
- Chưa thêm HTML templates thực sự.
- Chưa làm `NotificationService`, `email.dispatch`, DB tables của phase 2 trở đi.

### Đang dang dở

- Nếu tiếp tục roadmap thì bước tiếp theo phải là **Phase 2**, không phải sửa kiến trúc lại Phase 1.
- CHƯA CHẮC có cần thêm HTML placeholder trước Phase 2 hay không; plan phase 1 không bắt buộc.

## 3. Luồng logic chính cần giữ nguyên

### 3.1 IAM auth email flow

1. Input:
   - business logic hiện tại vẫn tạo OTP/token/link/TTL như cũ
   - vars render được tạo tại chỗ trong `internal/iam/app/service.go`
2. Xử lý:
   - code build `legacySubject`, `legacyBody`
   - gọi helper `renderEmailContent(ctx, templateKey, vars, legacySubject, legacyBody)`
3. Rẽ nhánh:
   - nếu `EMAIL_TEMPLATE_SOURCE != "embed"`: trả thẳng legacy
   - nếu registry hoặc renderer nil: trả legacy
   - nếu resolve template lỗi: trả legacy
   - nếu render lỗi: trả legacy
   - nếu thành công: dùng output từ template
4. Output:
   - vẫn gọi `publishEmail(...)`
   - payload event **không đổi shape**
5. Case đặc biệt:
   - missing required var không được fail business path; phải fallback legacy

### 3.2 Reminder email flow

1. Input:
   - `templateCode`
   - `payload`
   - `recipients`
2. Xử lý:
   - `SendReminderEmail(...)` gọi `renderReminderEmailContent(...)`
   - nếu `templateCode == REMINDER_DISCLOSURE_DUE` thì map sang key `reminder.disclosure_deadline`
3. Rẽ nhánh:
   - nếu `EMAIL_TEMPLATE_SOURCE=embed` và registry/renderer có đủ:
     - resolve + render template
     - normalize body từ `\n` sang `\r\n`
   - nếu lỗi hoặc không map được: fallback `renderReminderEmail(...)` legacy
4. Output:
   - vẫn dựng MIME text/plain và gọi `smtp.SendMail(...)` như cũ
5. Case đặc biệt:
   - không đổi retry classification hiện tại
   - không đổi SMTP mock/no-smtp behavior

## 4. Các quyết định kỹ thuật đã chốt

- Chọn **extend `internal/notification` hiện có** thay vì tạo module mới.
- Chọn `EMAIL_TEMPLATE_SOURCE` làm feature flag để rollback dễ.
- Chọn render ở IAM service / reminder sender thay vì worker auth path, vì plan phase 1 yêu cầu giữ delivery path cũ.
- Chọn giữ outbox auth payload `to/subject/body`, không chuyển sang `template_key + vars` ở phase 1.
- Chọn metadata `trailing_lf` trong `meta.yaml` để giữ đúng legacy output, vì test đã fail do newline không đồng nhất.
- Chọn map reminder template code hiện tại sang template key mới, thay vì đổi contract reminder cũ.

### Naming/convention/rule phải giữ

- Template key:
  - `auth.email_verification`
  - `auth.password_reset.user`
  - `auth.password_reset.admin`
  - `auth.user_invitation.new_user`
  - `auth.user_invitation.existing_user`
  - `reminder.disclosure_deadline`
- Config:
  - `EMAIL_TEMPLATE_SOURCE=legacy|embed`
- Output phase 1 phải ưu tiên **khớp legacy**, không ưu tiên “đẹp hơn”.

### Giải pháp đã loại bỏ

- Rewrite auth worker delivery path trong phase 1
- Đổi payload auth outbox
- DB-driven template
- HTML-first implementation
- Refactor mạnh reminder pipeline

### Giới hạn cần tôn trọng

- Không chạm business rule OTP/reset/invite/reminder
- Không chạm token generation / TTL source of truth
- Không gộp phase 1 và phase 2

## 5. Những điểm dễ bị hiểu sai

- Không được hiểu `embed` là thay thế hoàn toàn `legacy`; đây là **optional render path có fallback**.
- Không được đổi event type hoặc payload của auth email ở phase 1.
- Không được dời logic generate token/OTP/link sang notification layer.
- Không được thay đổi reminder dispatch/retry semantics.
- Không được chỉnh nội dung template theo ý riêng; phải khớp output cũ.
- Không được bỏ xử lý newline/trailing newline.
- Không được refactor worker auth SMTP path sang shared adapter nếu chưa có yêu cầu phase tiếp theo.

## 6. Dữ liệu / state / model liên quan

### `EMAIL_TEMPLATE_SOURCE`

- Ý nghĩa: chọn source render email phase 1
- Kiểu: `string`
- Values:
  - `legacy`
  - `embed`
- Đọc ở:
  - `internal/platform/config/config.go`
- Ảnh hưởng:
  - IAM auth render path
  - reminder render path

### `notificationapp.ResolvedTemplate`

- File: `internal/notification/app/email_contracts.go`
- Fields:
  - `Key string`
  - `Locale string`
  - `Subject string`
  - `TextBody string`
  - `HTMLBody string`
  - `RequiredVars []string`
  - `TrailingLF bool`
- Ý nghĩa:
  - `TrailingLF` dùng để tái tạo đúng legacy body behavior

### `meta.yaml`

- Ý nghĩa: metadata của mỗi template
- Fields đang dùng:
  - `default_locale`
  - `variables[].name`
  - `variables[].required`
  - `trailing_lf`

### Auth email event payload

- Kiểu: `map[string]any`
- Fields:
  - `to`
  - `subject`
  - `body`
- Ghi ở:
  - `internal/iam/app/service.go`
- Đọc ở:
  - `cmd/worker/main.go`
- Ảnh hưởng:
  - worker auth email delivery path hiện tại

### Reminder template mapping

- `REMINDER_DISCLOSURE_DUE` -> `reminder.disclosure_deadline`

## 7. Code hiện tại hoặc pseudo-code

### IAM helper

```go
func (s *service) renderEmailContent(ctx context.Context, key string, vars map[string]any, legacySubject, legacyBody string) (string, string) {
	if s.emailTemplateSource != "embed" || s.emailTemplateRegistry == nil || s.emailRenderer == nil {
		return legacySubject, legacyBody
	}
	resolved, err := s.emailTemplateRegistry.Resolve(ctx, key, "vi")
	if err != nil {
		return legacySubject, legacyBody
	}
	rendered, err := s.emailRenderer.Render(resolved, vars)
	if err != nil {
		return legacySubject, legacyBody
	}
	return rendered.Subject, rendered.TextBody
}
```

### Reminder helper

```go
func (s *Sender) renderReminderEmailContent(templateCode string, payload map[string]any) (string, string, error) {
	if s.templateSource == "embed" && s.registry != nil && s.renderer != nil {
		if key, ok := reminderTemplateKey(templateCode); ok {
			resolved, err := s.registry.Resolve(context.Background(), key, "vi")
			if err == nil {
				rendered, renderErr := s.renderer.Render(resolved, payload)
				if renderErr == nil {
					return rendered.Subject, strings.ReplaceAll(rendered.TextBody, "\n", "\r\n"), nil
				}
			}
		}
	}
	return renderReminderEmail(templateCode, payload)
}
```

### TODO cụ thể

- Nếu tiếp tục roadmap:
  - bắt đầu **Phase 2**
  - thiết kế `NotificationService`
  - vẫn giữ compatibility với phase 1 render path
- Không có TODO phase 1 bắt buộc còn đỏ tại thời điểm kết thúc session này.

## 8. Các bug / vấn đề đang gặp

### Đã gặp trong session này

- `go:embed` pattern fail vì dùng `*/vi/*.html` nhưng chưa có file html nào.
- Golden test fail do:
  - subject bị newline cuối
  - body newline không đồng nhất giữa các template
  - invitation template spacing lệch so với legacy

### Đã thử

- Normalize newline cứng cho mọi template: **không đủ chính xác**
- Chỉnh template whitespace + thêm metadata `trailing_lf`: **hiệu quả**

### Hiện còn bug gì?

- Không còn bug đang mở trong phạm vi phase 1 đã implement.

### Nghi ngờ/nguy cơ

- CHƯA CHẮC phase sau có muốn auth worker cũng dùng shared SMTP helper hay không.
- CHƯA CHẮC có cần HTML body sớm hơn phase 7 hay không; plan hiện tại không yêu cầu cho phase 1.

## 9. Việc session tiếp theo cần làm ngay

- [ ] Mở lại `docs/email-notification-system-implementation-plan.md`
- [ ] Xác nhận đang tiếp tục **Phase 2**, không redesign lại Phase 1
- [ ] Đọc file này trước khi sửa code
- [ ] Đọc `docs/ai-cache/reusable-task-updates.md` mục ngày `2026-05-22 - Email notification system phase 1 embed template extraction`
- [ ] Kiểm tra worktree hiện tại có thay đổi local nào khác ngoài phase 1 này không
- [ ] Nếu làm Phase 2: định nghĩa rõ contract `NotificationService` trước khi code
- [ ] Giữ nguyên fallback `legacy`
- [ ] Giữ nguyên outbox auth payload shape
- [ ] Sau khi sửa tiếp, rerun:
  - [ ] `go test ./internal/notification/app ./internal/notification/infra/registry ./internal/reminder/infra/email ./internal/iam/app`
  - [ ] `go test ./internal/httpserver ./cmd/worker`
  - [ ] `docker compose -f docker-compose.dev.yml build api`

## 10. Test cases cần giữ

### Case bình thường

- Email verification
  - Input: `full_name=Nguyen Van A`, `otp_code=123456`, `expiry_minutes=15`
  - Expected: subject/body khớp legacy
  - Lý do: auth path quan trọng

- User password reset
  - Input: valid `reset_link`
  - Expected: subject/body khớp legacy
  - Lý do: security-sensitive flow

- Admin password reset
  - Input: valid `reset_link`
  - Expected: giữ trailing newline đúng legacy
  - Lý do: từng fail do newline

- Invitation new user
  - Input: `display_name`, `company_name`, `setup_link`, `expiry_hours`
  - Expected: spacing/newline đúng legacy
  - Lý do: từng fail do whitespace

- Invitation existing user
  - Input: `display_name`, `company_name`
  - Expected: không có setup link, body đúng legacy
  - Lý do: branch riêng

- Reminder disclosure due
  - Input: `title`, `deadline_date`, `disclosure_id`, optional `status`, `action_url`
  - Expected: embed output == legacy output
  - Lý do: không được đổi reminder behavior

### Case lỗi / edge

- Missing required var
  - Input: thiếu `otp_code` hoặc field bắt buộc khác
  - Expected: renderer error, caller fallback legacy
  - Lý do: phase 1 không được fail business path

- Locale không tồn tại
  - Input: locale `en`
  - Expected: fallback `vi`
  - Lý do: registry fallback contract

- Broken registry
  - Input: mock registry trả lỗi
  - Expected: fallback legacy
  - Lý do: rollback safety

- Reminder template code không map
  - Input: template code khác `REMINDER_DISCLOSURE_DUE`
  - Expected: behavior cũ
  - Lý do: scope hiện tại chỉ cover 1 reminder template

## 11. Ràng buộc phong cách triển khai

- Không viết lại toàn bộ nếu không cần.
- Ưu tiên sửa tiếp từ logic hiện tại.
- Nếu thiếu thông tin, đọc lại plan/docs trước khi đoán.
- Nếu muốn đổi kiến trúc, phải giải thích trade-off trước.
- Giữ naming, flow, key, config đã thống nhất.
- Chỉ refactor khi giúp giữ behavior đúng hoặc giảm lỗi.
- Không tự thêm behavior ngoài phase đang làm.

## 12. Tóm tắt cực ngắn cho session sau

Đang làm email notification system phase 1 trong `cobo_iam_services`. Phase 1 đã xong phần extract template sang `embed.FS`, thêm registry/renderer, thêm `EMAIL_TEMPLATE_SOURCE`, và wire IAM + reminder dùng `embed` nhưng fallback `legacy` khi lỗi. Không đổi outbox payload auth, không đổi token/OTP/TTL, không đổi reminder retry path. Test hẹp, compile-level verify, và Docker build api đều pass. Nếu làm tiếp thì sang **Phase 2 NotificationService**, không quay lại redesign Phase 1, và không được làm hỏng fallback legacy hay contract hiện tại.
