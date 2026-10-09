---
name: vi-text-encoding
description: Dùng khi gặp hoặc phòng tránh lỗi tiếng Việt bị mojibake (KhÃ´ng, láº, Ä'ang, ký tự box-drawing) trong code, docs, template email, Excel, SQL seed của Cobo - phát hiện, decode, sửa an toàn, chuẩn hóa Unicode NFC, check:mojibake.
---

# Vietnamese Text Encoding

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
