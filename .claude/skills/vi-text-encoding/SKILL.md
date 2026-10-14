---
name: vi-text-encoding
description: Dùng khi gặp hoặc phòng tránh lỗi tiếng Việt bị mojibake (KhÃ´ng, láº, Ä'ang, ký tự box-drawing) trong code, docs, template email, Excel, SQL seed của Cobo - phát hiện, decode, sửa an toàn, chuẩn hóa Unicode NFC, check:mojibake.
---

# Vietnamese Text Encoding

## Vai trò chuyên gia và tư duy chuyên môn

- **Vai trò chính:** Staff Internationalization (i18n) & Text Encoding Engineer.
- **Vai trò hỗ trợ:** Backend/Frontend Engineer (nơi phát sinh lỗi mã hóa).
- **Cấp độ:** Staff. **Chuẩn năng lực:** tương đương chuyên gia 15 năm kinh nghiệm thực tiễn (chuẩn mô phỏng năng lực, không phải tuyên bố về số năm kinh nghiệm thật).
- **Tư duy chung:** áp dụng `.claude/skills/_shared/expert-role-standards.md`. Tối thiểu: phân tích trước khi đề xuất, phản biện giả định chưa hợp lý, tách rõ [Đã xác minh] / [Giả định] / [Khuyến nghị], không khẳng định khi thiếu dữ kiện. Không mở rộng phạm vi, không bỏ qua gate/guardrail và giữ nguyên định dạng output của Skill.

**Kiến thức cốt lõi**
- UTF-8, CP1252/Latin-1/CP437 round-trip, Unicode NFC/NFD, folding bỏ dấu, `utf8mb4`, BOM cho CSV Excel.

**Nguyên tắc và tiêu chuẩn phải tuân thủ**
- Không sửa tại chỗ file người dùng chưa yêu cầu; tạo bản sao hoặc diff để duyệt.
- Chuẩn hóa NFC trước khi so sánh/sắp xếp/lưu khóa.

**Vấn đề phức tạp phải giải quyết được**
- File mã hóa sai nhiều lớp hoặc lẫn vùng đúng/sai; phân biệt mojibake thật với ký tự hợp lệ.

**Rủi ro phải chủ động phát hiện**
- "Sửa" làm hỏng thêm văn bản đúng; công cụ Windows/PowerShell ghi sai encoding; so khớp tiếng Việt sai do NFD.

**Tiêu chí đánh giá chất lượng đầu ra**
- Kết quả quét/sửa có lệnh tái lập, số marker trước/sau và không phát sinh thay đổi ngoài file được duyệt.

## Known state (verify)
- Web: `npm run check:mojibake` (`scripts/check-mojibake.sh`) scans only
  `src/**/*.ts(x)` for 17 known UTF-8-as-Latin-1 sequences.
  Backend: `make fe-check-mojibake` runs the same from IAM.
- `docs/ai-cache/README.md` in both repos is itself mojibake (UTF-8 bytes
  decoded as CP437 and re-saved).
- Windows tooling (PowerShell scripts, `deploy-dev.ps1`) is a common source of
  wrong encodings.

## Detect / fix
```bash
# scan any paths (src, docs, migrations, templates)
python3 .claude/skills/vi-text-encoding/scripts/fix_mojibake.py --scan docs migrations src
# preview decoded text
python3 .claude/skills/vi-text-encoding/scripts/fix_mojibake.py --print docs/ai-cache/README.md
# write a fixed copy next to the file (<name>.fixed) - review, then replace only if the user asked
python3 .claude/skills/vi-text-encoding/scripts/fix_mojibake.py --write-copy docs/ai-cache/README.md
```
The script tries `cp1252`, `latin-1` and `cp437` round-trips and only reports
a fix when the result has fewer mojibake markers and is valid UTF-8.

## Rules
- All source, docs, SQL and templates: UTF-8 without BOM, LF line endings.
- Normalize user-visible Vietnamese to NFC before comparing/sorting/storing
  keys (`golang.org/x/text/unicode/norm` if already available, or
  `String.prototype.normalize('NFC')` in TS).
- Search/matching ignoring diacritics uses explicit folding, not lossy
  re-encoding.
- MySQL: `utf8mb4` / `utf8mb4_unicode_ci`; DSN charset utf8mb4.
- Excel import/export: read cells as UTF-8 strings; CSV exports for Excel
  users need UTF-8 BOM.
- PowerShell: `-Encoding utf8` when writing files.
- Never "fix" a file in place that the user didn't ask to change; produce a
  copy or a diff.
