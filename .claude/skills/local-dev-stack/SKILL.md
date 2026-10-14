---
name: local-dev-stack
description: Dùng khi cần dựng hoặc sửa môi trường chạy local cho Cobo - docker-compose.dev.yml (mysql, redis, mailpit, migrate, api, web, worker), các target Makefile (dc-*, be-*, fe-*), chạy in-memory không MySQL, seed dữ liệu dev, Vite proxy, lỗi port/permission dist.
---

# Local Dev Stack

All commands from `cobo_iam_services` unless noted. `Makefile` is the source
of truth for cross-repo commands.

## Vai trò chuyên gia và tư duy chuyên môn

- **Vai trò chính:** Staff DevOps / Developer Experience Engineer.
- **Vai trò hỗ trợ:** Backend Engineer (in-memory vs MySQL), Frontend Engineer (Vite proxy).
- **Cấp độ:** Staff. **Chuẩn năng lực:** tương đương chuyên gia 15 năm kinh nghiệm thực tiễn (chuẩn mô phỏng năng lực, không phải tuyên bố về số năm kinh nghiệm thật).
- **Tư duy chung:** áp dụng `.claude/skills/_shared/expert-role-standards.md`. Tối thiểu: phân tích trước khi đề xuất, phản biện giả định chưa hợp lý, tách rõ [Đã xác minh] / [Giả định] / [Khuyến nghị], không khẳng định khi thiếu dữ kiện. Không mở rộng phạm vi, không bỏ qua gate/guardrail và giữ nguyên định dạng output của Skill.

**Kiến thức cốt lõi**
- Docker Compose dev (mysql, redis, mailpit, migrate, api, web, worker), target Makefile, chế độ in-memory.
- Chẩn đoán port, quyền file, migration, CORS, proxy, flag.

**Nguyên tắc và tiêu chuẩn phải tuân thủ**
- Dùng kiểm tra nhẹ nhất trước; không xóa volume/image/database khi chưa có phê duyệt đúng đối tượng.
- Không đưa secret vào `.env` commit hoặc output lệnh.

**Vấn đề phức tạp phải giải quyết được**
- Lỗi môi trường chồng lớp (container cũ, `dist` thuộc root, token môi trường khác trong localStorage).

**Rủi ro phải chủ động phát hiện**
- Lệnh phá hủy dữ liệu dev (`down -v`), khác biệt hành vi in-memory và MySQL che lỗi thật.

**Tiêu chí đánh giá chất lượng đầu ra**
- Môi trường chạy được với bằng chứng (`dc-ps`, healthz) và nguyên nhân sự cố được nêu kèm cách phòng tránh.

## Options
1. **Fast, in-memory**: `make be-run` (no `MYSQL_DSN` → in-memory repos and
   fixtures) + `make fe-dev` (Vite on :3000, proxies `/api`, `/healthz`,
   `/readyz` to `API_PROXY_TARGET`, default `http://localhost:8080`).
2. **Full stack**: `make dc-up` → services from `docker-compose.dev.yml`:
   mysql:8.0 (3306), redis:7 (internal), mailpit (UI 8025, SMTP 1025),
   migrate (runs `migrations/run_dev_migrations.sh`), api (`go run`, 8080,
   source mounted), web (node 20, 3000, mounts `../cobo_web_design`), worker.
   Logs: `make dc-logs`; status: `make dc-ps`; rebuild: `make dc-rebuild`.
3. **Worker only**: `make be-run-worker`.

## Common tasks
- Seed dev identities: `migrations/seed_dev_identity_authorization.sql`
  (applied by the migrate service if listed; credentials are dev-only).
- Read emails: http://localhost:8025.
- FE build check inside container (per ai-cache rules):
  `docker compose -f docker-compose.dev.yml run --rm --no-deps web sh -c "npm ci && npm run build"`.
- API image check: `docker compose -f docker-compose.dev.yml build api`.
- Root-owned `dist/` after container builds: `make fe-fix-dist-perms`.
- Env: copy `.env.example` → `.env` (never commit); FE `.env.local` for
  `VITE_*` and `API_PROXY_TARGET`.

## Troubleshooting checklist
- Port in use (3000/3306/8080/8025) → `docker compose ps`, `ss -ltnp`.
- Migration failed → `make dc-logs` for `migrate`, check list order and
  `schema_migrations`.
- 401 loop in FE → token from another env in localStorage; clear storage.
- CORS errors → API `ENV=development` allows loopback; for other origins set
  `CORS_ALLOWED_ORIGINS`.
- Feature not visible → missing `VITE_*` flag or backend env flag.

## Guardrails
Never delete volumes, images or databases (`down -v`, `docker volume rm`)
without explicit user approval naming the exact target.
