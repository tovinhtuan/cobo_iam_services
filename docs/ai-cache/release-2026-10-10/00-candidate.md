# Release candidate DEV — 2026-10-10 (wf-release, lần 2 trong ngày)

User duyệt trước toàn bộ quy trình: check → risk review → deploy từ working tree → smoke tự động. Không tự rollback.

| Repo | Branch | HEAD | Working tree so với bản đang chạy trên DEV |
|---|---|---|---|
| cobo_iam_services | improve/ui-theme | `c6b4ab6` (đã deploy lên DEV lúc 04:30Z; xem `00-deploy-be.md`) | **Không có thay đổi code ứng dụng.** Chỉ đổi `CLAUDE.md`, `.claude/skills/{deploy-dev-release,wf-release}`, 4 smoke script `release-2026-10-09/*` (bỏ IP và mật khẩu gắn cứng, chuyển sang loader), `scripts/devqa/` (loader, script provision persona QA, tool bcrypt `scripts/devqa/bcrypthash`: tool rời, không thuộc `cmd/api` hay `cmd/worker`), và docs ai-cache. |
| cobo_web_design | improve/ui-theme | `453a3494` (bản FE đang chạy, không đổi) | **Không đổi `src/`.** Chỉ đổi script smoke/debug (chuyển sang `scripts/lib/devQaEnv.mjs`), skill, `docs/ai-cache/dev-qa/`. |

- **Bản DEV trước release:** api `fd52e46f…`, worker `54525832…` (build từ `c6b4ab6`), web `453a3494`.
- **Kết luận:** không có binary hay bundle mới để deploy. Mode được chọn là `verify`.
