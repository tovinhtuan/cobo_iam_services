# Kế hoạch sửa theo risk review 2026-10-10

Nguồn: `10-risk-report.md`. Thực hiện qua `wf-bugfix`, mỗi bước là một nhóm nhỏ có test hồi quy. Chưa commit/push/deploy nếu chưa có yêu cầu rõ.

Tiến độ từng bước ghi tại `docs/ai-cache/bug-revoke-access-effective-cache-2026-10-10/`.

## PR-A (iam) — thu hồi quyền có hiệu lực ngay (ưu tiên 1)

**Trạng thái 2026-10-10: A1-A5 FIXED IN WORKING TREE (chưa commit/deploy).** Phát sinh từ review và đã sửa luôn: PERF-26/CACHE-15/BES-14, BES-12, BES-13, PERF-12, PERF-30, PERF-31/CACHE-17, BES-15. Chi tiết và follow-up: `../bug-revoke-access-effective-cache-2026-10-10/05-completion.md`.

| Bước | Finding | Thay đổi | Test hồi quy |
|---|---|---|---|
| A1 | H3 | Resolver trả quyền rỗng khi membership không `active`. Kiểm qua interface tuỳ chọn `MembershipStatusReader`; MySQL đọc `memberships.membership_status`. | `authorization/infra/inmemory` resolver test: membership inactive → không có permission/scope |
| A2 | CACHE-11, CACHE-12, PERF-21, CACHE-04 | Store cache dùng generation theo company. Key có version `v2`. `Get` trả generation đã đọc; `Put` ghi kèm generation đó; generation lệch thì coi là miss. Thêm `InvalidateCompany` (SET token ngẫu nhiên, O(1); thiếu key generation → không cache). Helper invalidate chuyển sang `InvalidateCompany`, chạy trên `context.WithoutCancel` + timeout, log khi lỗi. Bỏ `ListMembershipsByCompany` khỏi đường invalidate. | `projection` test: Put với generation cũ sau khi invalidate thì không được đọc lại (race CACHE-11); invalidate company xoá mọi member |
| A3 | H17 / ROLE-06 | Gọi invalidate sau mọi mutation ảnh hưởng effective access trong `companyaccess`: role của membership (assign/remove/primary), quyền của role, direct permission, trạng thái/xoá membership, phòng ban, chức danh, team/org unit, custom role (create/clone/update/delete). | Test bảng: mỗi route → cache bị invalidate |
| A4 | BES-10 | Restore direct grant chỉ cho membership `active`. | Test restore/plan bỏ qua membership inactive |
| A5 | H3 (refresh) | Refresh token thất bại khi membership của phiên không còn `active`. | `iam/app` test refresh với membership inactive |

Ngoài phạm vi PR-A (ghi follow-up): invalidate khi `assignments` đổi ở module workflow/disclosure, và ở route platform CMS gán company/thêm member.

## Các PR tiếp theo (theo thứ tự)

- **PR-B (quyền admin):** **Trạng thái 2026-10-10: FIXED IN WORKING TREE** (chưa commit). ROLE-03 theo phương án A. Review phát sinh và đã sửa luôn: BES-18 (HIGH, approve notification trên MySQL), BES-19, BES-20, BES-21, ROLE-19, API-17 (lọc picker), API-19, API-20, BES-23, BES-24. Chi tiết và follow-up: `../bug-admin-escalation-2026-10-10/05-completion.md`.
  - ROLE-02 (ép `company_id` theo token)
  - ROLE-13 (guard gỡ role/quyền platform)
  - ROLE-12 (rollback/xoá rule prefs qua approval)
  - ROLE-03 (role khi mời: chọn phương án — cần quyết định)
  - ROLE-05, BES-09, ROLE-14, RP-07
- **PR-C (hậu kỳ ROLE-01):**
  - BES-08 (từ chối row không có `plan_digest`)
  - PERF-19 (thứ tự khoá, không nuốt lỗi capture), retry 1213/1205
  - PERF-23, PERF-22, PERF-20
- **PR-D (perf/worker):** PERF-10, PERF-11 (PERF-12 đã sửa ở PR-A), PERF-27/28/29/32 (follow-up của PR-A), PERF-14, H8–H16, CACHE-08.
- **PR-E (deploy/docs):**
  - H18, H19
  - API-15, API-09
  - runbook API-13
  - SEC-12, SEC-13, PERF-25
- **PR-F (web):** FES-15, API-14, CACHE-14, FES-16, FES-07/09/10, API-10/11, SEC-08.
- **Hạ tầng (cần người có quyền server):** C1, C6, H20–H23, rotate credential.

## Quyết định mặc định đã áp dụng (đổi được)

- **H17/H3:** kết hợp hai cách.
  - Thứ nhất: resolver lọc trạng thái membership.
  - Thứ hai: invalidate theo generation của company ở mọi mutation.
  - Không giảm TTL. Lý do: generation biến invalidate thành O(1), nên invalidate cả company khi có mutation là rẻ và an toàn hơn việc tính chính xác danh sách membership bị ảnh hưởng.
- **Key Redis đổi namespace sang `cobo_iam:effective_access:v2`.**
  - Lý do: tránh binary cũ đọc nhầm format mới khi deploy cuốn chiếu.
  - Hệ quả: cache cũ bị bỏ qua và tự hết hạn theo TTL.
