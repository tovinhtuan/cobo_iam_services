# Feature: Tra cứu & Đồng bộ thông tin công ty niêm yết khi tạo doanh nghiệp mới

**Loại:** Enhancement — Onboarding Flow  
**Phạm vi:** Flow "Tạo doanh nghiệp mới" (Initialize + Self-service Create)  
**Trạng thái:** Đề xuất — chờ xác nhận

---

## 1. Bối cảnh & Vấn đề

Khi người dùng tạo doanh nghiệp mới trên nền tảng, họ phải nhập thủ công các thông tin như tên công ty, mã số thuế, địa chỉ, số điện thoại... Với các doanh nghiệp đã niêm yết trên sàn chứng khoán (HOSE, HNX, UPCOM), thông tin này đã có sẵn trong cơ sở dữ liệu của hệ thống (nguồn KBS). Việc bắt người dùng nhập lại toàn bộ gây ra:

- **Ma sát không cần thiết** trong quá trình onboarding — tăng thời gian hoàn thành form.
- **Nguy cơ sai sót** do nhập tay (tên công ty viết tắt, địa chỉ không đầy đủ...).
- **Trải nghiệm kém** so với kỳ vọng của người dùng doanh nghiệp.

---

## 2. Mục tiêu

Cho phép người dùng nhập mã đăng ký kinh doanh để **xem trước thông tin** công ty niêm yết tương ứng, sau đó **đồng bộ tự động** vào form nếu xác nhận đúng — giảm thời gian điền form và hạn chế sai sót.

---

## 3. Người dùng mục tiêu

- Bất kỳ người dùng nào đang thực hiện flow "Tạo doanh nghiệp mới", gồm:
  - Người dùng lần đầu đăng ký, chưa có doanh nghiệp (`/company/initialize`).
  - Người dùng đã có doanh nghiệp, muốn tạo thêm (`/company/create`).
- Không yêu cầu đăng nhập để tra cứu — tính năng tra cứu là public.

---

## 4. User Story

> **Là** người dùng đang tạo doanh nghiệp mới trên nền tảng,  
> **Tôi muốn** nhập mã đăng ký kinh doanh và thấy ngay thông tin công ty tương ứng từ hệ thống,  
> **Để** tôi không phải nhập tay toàn bộ thông tin và tránh sai sót.

---

## 5. Luồng người dùng (User Journey)

```
[Form tạo doanh nghiệp]
        │
        │ Người dùng nhập mã đăng ký kinh doanh
        │ (business_code, ví dụ: 0101234567)
        ▼
[Hệ thống tra cứu trong DB công ty niêm yết]
        │
        ├─── Không tìm thấy ──────────────────────────────────────────────────────────┐
        │                                                                             │
        │    Tìm thấy                                                                 │
        ▼                                                                             │
[Hiển thị Preview Card]                                                              │
  - Tên công ty (từ DB niêm yết)                                                     │
  - Mã chứng khoán + Sàn giao dịch                                                   │
  - Loại hình doanh nghiệp                                                           │
  - Disclaimer: "Thông tin chỉ mang tính tham khảo..."                               │
  - Nút [Đồng bộ thông tin] và [Bỏ qua]                                             │
        │                                                                             │
        ├─── Bỏ qua / Không đồng ý ───────────────────────────────────────────────── ┤
        │                                                                             │
        │    Chọn [Đồng bộ thông tin]                                                 │
        ▼                                                                             │
[Các trường trong form được điền tự động]                                            │
  - Tên công ty, Mã số thuế, Mã ĐKKD                                                 │
  - Địa chỉ, Số điện thoại, Email liên hệ                                            │
  - Người dùng vẫn có thể chỉnh sửa trước khi submit                                 │
        │                                                                             │
        ▼                                                                             │
[Người dùng submit form như bình thường] ◄───────────────────────────────────────────┘
```

---

## 6. Yêu cầu chức năng

### 6.1 Tra cứu (Lookup)

| # | Yêu cầu |
|---|---------|
| F-01 | Hệ thống cho phép tra cứu thông tin công ty niêm yết bằng mã đăng ký kinh doanh (`business_code`). |
| F-02 | Tính năng tra cứu **không yêu cầu đăng nhập** — áp dụng cho mọi người dùng. |
| F-03 | Nếu tìm thấy công ty khớp, hệ thống trả về thông tin để hiển thị preview và payload để đồng bộ. |
| F-04 | Nếu **không tìm thấy**, hệ thống thông báo "Không tìm thấy thông tin công ty niêm yết phù hợp" — form vẫn hoạt động bình thường, người dùng tự nhập. |
| F-05 | Nếu công ty niêm yết tìm thấy nhưng **không có hồ sơ chi tiết** (chỉ có mã chứng khoán, không có thông tin đầy đủ), hệ thống coi như không tìm thấy và không hiển thị preview. |

### 6.2 Đồng bộ (Sync)

| # | Yêu cầu |
|---|---------|
| F-06 | Người dùng xem preview và tự quyết định có đồng bộ hay không — hệ thống **không tự động điền** mà không có thao tác xác nhận. |
| F-07 | Khi đồng bộ, **toàn bộ các field** có thể ánh xạ được điền vào form, gồm: tên công ty, mã số thuế, mã ĐKKD, địa chỉ, số điện thoại, email liên hệ. |
| F-08 | Sau khi đồng bộ, người dùng **vẫn có thể chỉnh sửa** bất kỳ trường nào trước khi submit — dữ liệu đồng bộ chỉ là gợi ý điền sẵn. |
| F-09 | Đồng bộ là **hành động một lần** tại thời điểm tạo doanh nghiệp. Sau khi tạo xong, hệ thống **không lưu liên kết** giữa doanh nghiệp và công ty niêm yết. |
| F-10 | Dữ liệu từ công ty niêm yết **không ghi đè** thông tin đã tạo — chỉ điền vào form trước khi submit. |

### 6.3 Disclaimer

| # | Yêu cầu |
|---|---------|
| F-11 | Preview card phải hiển thị rõ disclaimer: *"Thông tin được lấy từ dữ liệu công ty niêm yết công khai, chỉ mang tính tham khảo. Nền tảng không chịu trách nhiệm pháp lý về tính chính xác của thông tin."* |
| F-12 | Disclaimer phải xuất hiện trước khi người dùng thực hiện đồng bộ — không được ẩn sau bước xác nhận. |

---

## 7. Yêu cầu phi chức năng

| # | Yêu cầu |
|---|---------|
| NF-01 | Thời gian phản hồi của API tra cứu ≤ 500ms trong điều kiện bình thường. |
| NF-02 | Nếu nguồn dữ liệu công ty niêm yết (vnstock DB) không khả dụng, hệ thống trả lỗi 503 rõ ràng — form tạo doanh nghiệp vẫn hoạt động bình thường (tính năng lookup là optional). |
| NF-03 | API tra cứu là public — cần được cân nhắc rate-limit ở tầng gateway để tránh bị lạm dụng dò quét dữ liệu. |
| NF-04 | Không cần thay đổi database schema — dữ liệu công ty niêm yết đã có sẵn. |

---

## 8. Quy tắc nghiệp vụ

| # | Quy tắc |
|---|---------|
| BR-01 | Tra cứu theo **mã đăng ký kinh doanh** (`business_code`) — không phải mã số thuế (`tax_code`) hay mã chứng khoán (`symbol`). |
| BR-02 | Mỗi mã đăng ký kinh doanh là duy nhất cho một công ty. Nếu DB có nhiều hơn một bản ghi khớp (lỗi dữ liệu nguồn), hệ thống lấy bản ghi đầu tiên. |
| BR-03 | Hệ thống **không xác thực** rằng công ty niêm yết preview có thực sự là công ty của người dùng — đây là trách nhiệm của người dùng khi xác nhận đồng bộ. |
| BR-04 | Việc đồng bộ không ảnh hưởng đến trạng thái `verification_status` của doanh nghiệp mới tạo — vẫn là `unverified` như bình thường. |
| BR-05 | Thông tin đồng bộ vào form không được đánh dấu hay phân biệt với thông tin người dùng tự nhập — sau khi submit, dữ liệu được lưu như nhau. |

---

## 9. Thông tin đồng bộ — Ánh xạ field

| Field hiển thị với người dùng | Field trong form tạo DN | Nguồn dữ liệu từ DB công ty niêm yết |
|-------------------------------|------------------------|--------------------------------------|
| Tên công ty                   | `company_name`         | `equity_list.company_name`           |
| Mã số thuế                    | `tax_code`             | `company_profiles.info → tax_id`     |
| Mã đăng ký kinh doanh         | `registration_number`  | `company_profiles.info → business_code` |
| Địa chỉ                       | `address`              | `company_profiles.info → address`    |
| Số điện thoại                 | `phone`                | `company_profiles.info → phone`      |
| Email liên hệ                 | `contact_email`        | `company_profiles.info → email`      |

**Thông tin chỉ hiển thị trong preview (không điền vào form):**

| Field hiển thị với người dùng | Nguồn dữ liệu |
|-------------------------------|---------------|
| Mã chứng khoán                | `equity_list.symbol` |
| Sàn giao dịch                 | `equity_list.exchange` |
| Loại hình doanh nghiệp        | `company_profiles.info → company_type` |
| Ngày niêm yết                 | `company_profiles.info → listing_date` |

---

## 10. Ngoài phạm vi (Out of Scope)

- Không hỗ trợ tra cứu công ty **chưa niêm yết** (công ty tư nhân, TNHH không niêm yết...).
- Không lưu lịch sử người dùng đã tra cứu công ty nào.
- Không có cơ chế báo cáo thông tin sai — người dùng chỉ được thông báo qua disclaimer.
- Không tự động cập nhật thông tin doanh nghiệp nếu thông tin công ty niêm yết thay đổi về sau.
- Không liên kết doanh nghiệp với công ty niêm yết sau khi tạo xong (không có tính năng "theo dõi công ty niêm yết").

---

## 11. Câu hỏi mở / Cần xác nhận thêm

| # | Câu hỏi | Mặc định nếu chưa có câu trả lời |
|---|---------|-----------------------------------|
| Q-01 | Nếu vnstock DB không khả dụng, có hiển thị thông báo lỗi cho người dùng hay im lặng (ẩn tính năng lookup)? | Hiển thị thông báo nhẹ "Tính năng tra cứu tạm thời không khả dụng", form vẫn dùng được. |
| Q-02 | Có giới hạn số lần tra cứu trong một phiên không? | Không giới hạn — người dùng có thể tra cứu lại nếu nhập sai mã. |
| Q-03 | Frontend debounce bao nhiêu ms trước khi gọi API tra cứu? | 500ms sau khi người dùng ngừng gõ. |

---

## 12. Tiêu chí chấp nhận (Acceptance Criteria)

**AC-01** — Khi người dùng nhập đúng mã ĐKKD của một công ty niêm yết có đầy đủ hồ sơ:
- Preview card xuất hiện với tên công ty, mã CK, sàn, loại hình DN, disclaimer.
- Nút "Đồng bộ thông tin" và "Bỏ qua" hiển thị rõ ràng.

**AC-02** — Khi người dùng bấm "Đồng bộ thông tin":
- Các trường `company_name`, `tax_code`, `registration_number`, `address`, `phone`, `contact_email` được điền vào form.
- Người dùng có thể chỉnh sửa các trường đó.
- Submit form thành công → doanh nghiệp được tạo với dữ liệu đã đồng bộ (hoặc đã chỉnh sửa).

**AC-03** — Khi nhập mã ĐKKD không có trong DB niêm yết:
- Không hiện preview card.
- Form không bị ảnh hưởng, người dùng tự nhập bình thường.

**AC-04** — Khi bấm "Bỏ qua" hoặc không thao tác với preview:
- Form không bị điền gì.
- Người dùng submit form với dữ liệu tự nhập → tạo doanh nghiệp bình thường.

**AC-05** — Khi vnstock DB không khả dụng:
- API tra cứu trả lỗi (503).
- Form tạo doanh nghiệp vẫn hoạt động bình thường — tính năng lookup là optional.

**AC-06** — Disclaimer hiển thị trong preview card trước khi người dùng xác nhận đồng bộ.

**AC-07** — Sau khi tạo doanh nghiệp thành công, không có dữ liệu nào lưu lại mối liên kết giữa doanh nghiệp mới và công ty niêm yết.
