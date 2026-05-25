# Dev email links — PUBLIC_WEB_BASE_URL (2026-05-25)

## Vấn đề

Mail reset password trên dev server `88.216.208.0` chứa `http://localhost:3000/reset-password?token=...` — user bên ngoài không mở được.

## Nguyên nhân

- Link ghép từ `s.webBaseURL` ← `PUBLIC_WEB_BASE_URL` (`internal/iam/app/buildActionLink`).
- `docker-compose.artifacts.yml` hard-code `localhost:3000`.

## Sửa đã áp dụng

| File | Thay đổi |
|------|----------|
| `docker-compose.artifacts.yml` | `api` + `worker`: `PUBLIC_WEB_BASE_URL=http://88.216.208.0:3000`, `PUBLIC_API_BASE_URL=http://88.216.208.0:8080` |
| `configs/config.example.env` | Comment local vs dev server |
| `docs/deploy-dev-guide.md` | §6.4 biến môi trường + recreate api/worker |
| `docs/windows-dev-guide.md` | Ghi chú artifacts vs local |

## Vận hành sau deploy compose mới

```bash
docker compose -f docker-compose.artifacts.yml up -d --force-recreate api worker
```

Override tùy chọn: `/root/cobo_project/.env`.

## Local không đổi

`docker-compose.dev.yml` vẫn `localhost:3000` cho máy dev.

## Nội dung mail reset (user) — 2026-05-25

Template `auth.password_reset.user` (vi): subject «Yêu cầu đặt lại mật khẩu CoBo Portal»; body formal VN với `full_name`, `reset_link`, `expiry_minutes`, `support_email`, `website_url`. Legacy fallback: `passwordResetUserEmailContent` trong `service.go`.

## CMS admin «Gửi email đặt lại MK» — 2026-05-25

`AdminRequestPasswordReset` → template `auth.password_reset.admin` — cùng nội dung/subject với user forgot-password; event `auth.admin_password_reset_requested`.
