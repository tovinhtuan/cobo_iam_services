# DEV API regression — disclosure list + avatar (2026-06-29)

## disclosure-types 500
- Root cause: `ListTypes` service forced `Page:0` → repo loaded **full catalog** (tags, descriptions, workflow `config_json` batch) before in-memory applicability filter + pagination.
- On DEV (~144 templates) MySQL returned `Error 1153: Got a packet bigger than 'max_allowed_packet' bytes` → HTTP 500.
- Requests with `q=` worked because SQL filtered rows before heavy payload.

## Fix
- Two-phase list: (1) lightweight rows for applicability filter + sort; (2) full summary for current page `type_id`s only (max 20–100).
- Pagination validation: `page<=0` → 400; `page_size` outside 1–100 when provided → 400; defaults `page=1`, `page_size=20`.

## avatar 400
- **Not a regression.** Provided stale signed URL → `401 invalid content signature`.
- Fresh signed URL from session → `404 avatar not found` (user has no uploaded avatar; `/me.avatar_url` null).
- No code change required; signature validation intact.

## DEV verify
- BE+PROXY `page=1&page_size=20` → 200, 20 items, total 144.
- CMS template list loads in browser.
