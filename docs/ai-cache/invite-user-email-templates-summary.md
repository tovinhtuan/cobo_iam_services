# Invite user email templates (2026-05-25)

## Templates

| Key | Khi nào |
|-----|---------|
| `auth.user_invitation.new_user_company` | `PublishUserInvitationEmail` với `companyName != ""` |
| `auth.user_invitation.new_user_no_company` | `companyName == ""` (CMS invite orphan) |

Subject chung: **Lời mời thiết lập tài khoản CoBo Portal**

Biến: `display_name`, `setup_link`, `expiry_hours`, `support_email`, `website_url`; thêm `company_name` (company template).

## Code

- `internal/iam/app/service.go` — `PublishUserInvitationEmail` chọn template key; legacy `userInvitationNewUserEmailContent`
- Đã xóa template cũ `auth.user_invitation.new_user` (if/else một file)

## Không đổi

- `auth.user_invitation.existing_user` — user active được thêm membership (không token)

## Deploy

Cần `make deploy-be` / `deploy-dev.sh be` để embed template mới trong binary.
