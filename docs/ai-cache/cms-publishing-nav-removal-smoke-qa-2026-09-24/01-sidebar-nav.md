# Flow A — Sidebar

- time_utc: 2026-09-24T08:38:26.110134+00:00
- DEV URL: http://88.216.208.0:3000/cms
- expected: nhóm “Xuất bản” ẩn mặc định; routes legacy vẫn trong bundle; deprecation notice có mặt
- actual:
  - i18n “Xuất bản” still in bundle (keys kept): True
  - publishing deep-link paths in bundle: True
  - legacy notice / publishingLegacy present: True
  - flag/helper present: False
- result: PASS
- note: Vitest CmsLayout.test proves default nav omits group; FE deploy uses same build.
- evidence: this file + CmsLayout.test.tsx
