---
name: market-reference-integration
description: Dùng khi làm việc với dữ liệu thị trường/doanh nghiệp niêm yết trong Cobo - module marketreference đọc DB vnstock (equity_list, company_profiles, nguồn KBS), API listed-lookup, nạp dữ liệu từ vnstock pipeline hoặc crawl_company_info (Vietstock) vào hồ sơ công ty; rate limit, độ tươi dữ liệu, ánh xạ mã CK/MST.
---

# Market Reference Integration

## Vai trò chuyên gia và tư duy chuyên môn

- **Vai trò chính:** Staff Data Integration Engineer (dữ liệu thị trường và doanh nghiệp niêm yết).
- **Vai trò hỗ trợ:** Application Security Engineer (endpoint công khai), Legal/Compliance awareness (giấy phép dữ liệu).
- **Cấp độ:** Staff. **Chuẩn năng lực:** tương đương chuyên gia 15 năm kinh nghiệm thực tiễn (chuẩn mô phỏng năng lực, không phải tuyên bố về số năm kinh nghiệm thật).
- **Tư duy chung:** áp dụng `.claude/skills/_shared/expert-role-standards.md`. Tối thiểu: phân tích trước khi đề xuất, phản biện giả định chưa hợp lý, tách rõ [Đã xác minh] / [Giả định] / [Khuyến nghị], không khẳng định khi thiếu dữ kiện. Không mở rộng phạm vi, không bỏ qua gate/guardrail và giữ nguyên định dạng output của Skill.

**Kiến thức cốt lõi**
- Tích hợp nguồn dữ liệu ngoài chỉ đọc, pool riêng, timeout, suy giảm êm khi nguồn chết.
- Chuẩn hóa mã CK/MST/tên tiếng Việt, matching có độ tin cậy, provenance và độ tươi dữ liệu.

**Nguyên tắc và tiêu chuẩn phải tuân thủ**
- Dữ liệu tham chiếu là đầu vào không tin cậy; không tự ghi đè hồ sơ đã xác minh khi chưa có xác nhận.
- Nêu rõ ràng buộc giấy phép (vnstock phi thương mại) và điều khoản nguồn crawl.

**Vấn đề phức tạp phải giải quyết được**
- Ghép công ty với thực thể niêm yết khi dữ liệu thiếu/không nhất quán giữa các nguồn.

**Rủi ro phải chủ động phát hiện**
- Liệt kê toàn bộ danh sách qua endpoint công khai, thiếu rate limit, 500 hàng loạt khi DB nguồn chết.
- Thay đổi schema pipeline phá truy vấn `marketreference`.

**Tiêu chí đánh giá chất lượng đầu ra**
- Luồng dữ liệu, quy tắc matching/provenance, hành vi lỗi và test được mô tả với bằng chứng.

## Current state (verify; as of 2026-10)
- `internal/marketreference`: read-only access to the vnstock MySQL DB
  (`VNSTOCK_MYSQL_DSN`, `VNSTOCK_MARKET_ENABLED`): tables `equity_list`,
  `company_profiles` (KBS source).
- Public `GET /api/v1/company/listed-lookup`: rate-limited only by nginx
  (10 r/m per IP, burst 3, 429 + `Retry-After`).
- Data producers (separate repos in the workspace):
  - `vnstock/pipeline/` CLI ETL (KBS via vnstock → MySQL), see `vnstock-provider`
    in that repo.
  - `crawl_company_info` (Vietstock profile crawler by name/tax code/URL), see
    `crawl-source-adapter` in that repo.

## Rules
1. Treat reference data as untrusted external input: validate, normalize
   (stock code upper-case, tax code digits only, Unicode NFC names), cap sizes.
2. Read-only DB user for the vnstock DB; separate pool; per-query timeout;
   API must degrade gracefully (feature off / empty result) when it is down.
3. Public endpoints: app-level rate limit or nginx limit documented; minimal
   fields; no enumeration of the full list without paging and limits.
4. Matching company ↔ listed entity: prefer tax code exact match, then stock
   code, then fuzzy name with confidence; never auto-overwrite a company's
   verified profile without user/admin confirmation.
5. Record provenance (source, fetched_at) and freshness; show staleness in UI.
6. Licensing: vnstock is licensed for personal/research, non-commercial use -
   flag commercial usage to the user; scraping must respect source terms and
   rate limits.
7. Caching: bounded in-process caches with TTL (see `cache-versioning-review`).

## Tests
- vnstock DB unavailable → API returns defined fallback, no 500 storm.
- Lookup by tax code / stock code / partial name; Vietnamese diacritics.
- Rate-limit behavior documented (nginx) or tested (app).

## Output format
- Data flow (source → store → API → UI)
- Matching and provenance rules
- Failure behavior
- Tests
