# 05 — Alert reproduction

- `/app/deadlines` search “QA - Báo cáo hoạt động tháng”
- Card: UPCOMING / current step “Tiếp nhận thông tin” / due 2026-07-31 / Ban Tổng Giám đốc
- Detail route: UI shows “Không tìm thấy cảnh báo / Request failed: 500” because `GET .../workflows/instances/{id}/tasks` → **500**
- Steps API still **200** with 4 steps — used for semantic comparison (`05-alert-workflow-steps.png` rebuilt from API table + note)
- Screenshots: `04-alert-card.png`, `05-alert-workflow-steps.png`, `06-alert-network.png`
