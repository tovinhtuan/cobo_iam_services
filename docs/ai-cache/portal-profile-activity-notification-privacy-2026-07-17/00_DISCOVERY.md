# 00 — Discovery

## FE
- Route: `/app/profile` → `UserProfile` → `PersonalOpsScreen` when `VITE_PERSONAL_OPS_V2=true`
- **Bug:** tab `activity` và right rail cùng render `PersonalActivityRail` với `viewModel.activities` (in-app notifications)
- Copy cũ: rail title “Hoạt động gần đây”; tab label “Lịch sử hoạt động” nhưng nội dung trùng notification
- Không có route `/app/profile/:userId`

## BE
- `GET /api/v1/me/operational-overview` → `activities` từ in-app notifications (mọi kind)
- In-app: `GET /api/v1/me/in-app-notifications`
- Audit: `audit_logs` + `ListFiltered` (company-scoped admin); thiếu actor filter trước task này
- Profile: chỉ `/api/v1/me` / `/api/v1/me/profile` (self) — không có personal profile-by-id cho member
