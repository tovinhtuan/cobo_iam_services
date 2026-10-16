# 04: Deploy DEV và smoke (16/10/2026)

## Bản deploy
| Mục | Giá trị |
|---|---|
| BE | `45b4a70`, sau đó `de3da9a` (sửa phép đếm sau smoke đầu). Lệnh: `bash deploy-dev.sh be --skip-tests` (api và worker). `/healthz` ok, `/readyz` ready |
| FE | `eec188c6`, bundle `index-Cj9fssGR.js`. Lệnh: `VITE_LEGAL_BASIS_STRUCTURED_CMS_ENABLED=true VITE_STEP_COMMENT_MENTIONS_V11=true sh deploy-dev.sh fe --skip-tests` |
| Migration | Không có |
| Thứ tự | BE, rồi FE |

## Trước và sau (c_001, persona ENT)
| | Trước deploy | Sau deploy |
|---|---|---|
| Tab Cảnh báo | 83 mục, 0/5 ad-hoc đã duyệt | 85 mục (`total` = 85). 2/2 ad-hoc có thông tin hạn được liệt kê, trạng thái UPCOMING. 3 ad-hoc QA cũ không có hạn nằm ngoài cả danh sách lẫn phép đếm |
| KPI dashboard 7d | 39 / 38 / 38 | 84 / 85 / 83 |
| KPI dashboard 30d, 90d | 84 / 83 / 83 | 84 / 85 / 83 (giống nhau ở mọi range) |
| "Việc của tôi" (4 persona) | 200 | 200, không đổi. Trên DEV, mọi task đang chờ của record Draft đang mở (332) thuộc `m_system_worker`, không thuộc người dùng nào, nên AC-13 không có thay đổi nhìn thấy được. AC-13 được kiểm bằng test MySQL |
| Reminder | 0 bản nhắc DISCLOSURE | 0 bản nhắc DISCLOSURE, 0 `OBLIGATION_CLOSED` (DEV có 0 `reminder_configs`) |

KPI ghi theo thứ tự `needs_action_now` / `pending_approval` / `blocked_or_exception`.

## Smoke
- **Lần đầu (`45b4a70`): 8/10.** `total=88` nhưng danh sách chỉ 85 mục, và chỉ 2/5 ad-hoc được liệt kê. Ba ad-hoc QA cũ không có thông tin hạn nên danh sách bỏ chúng, nhưng phép đếm vẫn tính. Đã sửa ở `de3da9a` (`02-fix.md`).
- **`scripts/devqa/smoke_mr3b_obligation_open.py`** (chỉ GET và đọc DB): **10/10**, cộng 1 SKIP có lý do (AC-13, xem trên).
- **Hồi quy MR-3:**
  - `scripts/devqa/smoke_mr3_obligation_open.py` **15/15**;
  - `cobo_web_design/docs/ai-cache/mr3-obligation-open-archive-2026-10-16/smoke-mr3-ui.mjs` **8/8** (không có lệnh ghi API nào). Hai lỗi 404 của `disclosure-types/{id}` trên trang chi tiết là follow-up API-06 đã biết.
- **Log api/worker 15 phút:** 0 panic, 0 fatal.

## Rollback
- **BE:** deploy lại `3031d8f` (MR-3). Các read model không lưu trạng thái, nên quay về hành vi cũ ngay.
- **FE:** deploy lại `98ffbd5c` (bundle `index--q4crObz.js`), cùng bộ cờ.
- Sau rollback, các bản nhắc đã thành `OBLIGATION_CLOSED` vẫn ở trạng thái kết thúc. DEV hiện có 0 bản như vậy.
