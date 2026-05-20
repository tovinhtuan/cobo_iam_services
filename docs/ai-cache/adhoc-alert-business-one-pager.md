# Ad-Hoc Alert Proposal Flow — Business One-Pager

## Tính năng này là gì?

Đây là tính năng hỗ trợ doanh nghiệp xử lý các trường hợp **công bố thông tin bất thường / đột xuất**.

Thay vì tạo hồ sơ công bố trực tiếp ngay từ đầu, người dùng sẽ tạo một **đề xuất cảnh báo bất thường**, gửi qua các vòng duyệt nội bộ, rồi hệ thống mới tự động khởi tạo hồ sơ công bố chính thức khi đề xuất được duyệt cuối.

Nói ngắn gọn:

**Đây là quy trình đề xuất và phê duyệt cho một case công bố bất thường.**

---

## Tính năng này giải quyết bài toán gì?

Trong thực tế có nhiều sự kiện không nằm trong kế hoạch công bố định kỳ, ví dụ:

- thay đổi nhân sự quan trọng
- giao dịch lớn phát sinh đột xuất
- sự kiện ảnh hưởng đến hoạt động doanh nghiệp
- quyết định cần công bố gấp

Những tình huống này cần:

- phản ứng nhanh
- có kiểm soát nội bộ
- có người chịu trách nhiệm duyệt cuối
- có thể khởi tạo ngay quy trình công bố chính thức sau khi được thông qua

---

## Người dùng nào tham gia?

### 1. Người đề xuất

Người khởi tạo đề xuất cảnh báo bất thường.

Họ có thể:

- tạo đề xuất
- lưu nháp
- gửi đề xuất
- rút lại đề xuất khi chưa có quyết định cuối

### 2. Focal Point

Người duyệt vòng đầu.

Họ có thể:

- duyệt
- từ chối

### 3. Người kiểm soát quy trình

Đây là người được chỉ định ngay khi tạo đề xuất.

Họ là người:

- duyệt cuối
- hoặc từ chối ở vòng cuối
- có thể điều chỉnh thông tin cuối trước khi hệ thống tạo hồ sơ công bố thật

Điểm quan trọng:

**Người duyệt cuối không phải là “bất kỳ admin nào”, mà là đúng người được chỉ định trong đề xuất.**

---

## Luồng nghiệp vụ hiện tại

1. Người dùng tạo đề xuất mới
2. Chọn loại công bố bất thường
3. Chỉ định người kiểm soát quy trình
4. Lưu nháp hoặc gửi đề xuất
5. Focal Point duyệt hoặc từ chối
6. Người kiểm soát quy trình duyệt hoặc từ chối
7. Nếu được duyệt cuối:
   - hệ thống tự động tạo hồ sơ công bố
   - hệ thống tự động tạo workflow xử lý tiếp theo

---

## Các trạng thái đang có

- `Nháp`
- `Chờ Focal duyệt`
- `Chờ Kiểm soát duyệt`
- `Đã duyệt`
- `Từ chối`
- `Đã hủy`

Ba trạng thái cuối:

- `Đã duyệt`
- `Từ chối`
- `Đã hủy`

là trạng thái kết thúc.

---

## Người dùng hiện có thể làm gì trên giao diện?

### Đã có sẵn

- xem danh sách đề xuất
- lọc theo trạng thái
- tạo đề xuất mới
- xem chi tiết từng đề xuất
- gửi đề xuất
- duyệt vòng focal
- duyệt vòng cuối
- từ chối
- rút lại đề xuất

### Lưu ý quan trọng

Chức năng hiện tại là cho **proposal / đề xuất**.

Nó **không đồng nghĩa** với việc đã có “CRUD hoàn chỉnh cho một cảnh báo cuối cùng”.

---

## Những điểm mạnh có thể truyền thông nội bộ

- Có quy trình phê duyệt rõ ràng cho tình huống bất thường
- Phân vai rõ giữa người đề xuất, người duyệt vòng 1 và người duyệt cuối
- Có thể chỉ định đúng người chịu trách nhiệm duyệt cuối
- Có lịch sử trạng thái trên từng đề xuất
- Sau khi được duyệt, hệ thống tự động sinh hồ sơ công bố và workflow tiếp theo

---

## Những điểm cần nói cẩn thận khi mô tả

### Nên nói

- “đề xuất cảnh báo bất thường”
- “quy trình phê duyệt cảnh báo bất thường”
- “sau khi duyệt, hệ thống tạo hồ sơ công bố chính thức”

### Không nên nói quá mức

- “đã có CRUD hoàn chỉnh cho cảnh báo bất thường”
- “người dùng có thể chỉnh sửa proposal nháp thoải mái về sau”
- “mọi admin đều có thể duyệt cuối”
- “đã có full notification/audit cho toàn bộ flow”

---

## Các giới hạn hiện tại BA cần biết

### 1. “Lưu nháp” chưa có nghĩa là chỉnh sửa nháp đầy đủ về sau

Hiện hệ thống có trạng thái nháp, nhưng trải nghiệm “mở lại và sửa draft cũ” chưa hoàn chỉnh như một module draft editor đầy đủ.

### 2. Đây là flow của proposal, không phải final alert management

Sau khi duyệt xong, hệ thống mới sinh ra hồ sơ công bố thật.

### 3. Một số phần contract nội bộ còn đang được engineering làm sạch

Điều này không cản việc demo flow chính, nhưng có nghĩa là tài liệu nghiệp vụ nên bám theo outcome thực tế, không hứa thêm tính năng chưa hoàn thiện.

---

## Câu mô tả ngắn gọn dùng cho slide / note

“Ad-Hoc Alert Proposal là luồng cho phép doanh nghiệp tạo đề xuất công bố bất thường, gửi qua các vòng phê duyệt nội bộ, chỉ định người kiểm soát duyệt cuối, và khi được duyệt thì hệ thống tự động tạo hồ sơ công bố cùng workflow thực thi tiếp theo.”

---

## Câu mô tả 1 dòng cho marketing nội bộ

“Một quy trình kiểm soát nội bộ cho các case công bố thông tin đột xuất, từ đề xuất đến phê duyệt và khởi tạo hồ sơ công bố chính thức.”
