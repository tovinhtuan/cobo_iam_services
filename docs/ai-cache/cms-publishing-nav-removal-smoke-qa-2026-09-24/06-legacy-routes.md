# Flow F — Legacy routes

- time_utc: 2026-09-24T08:38:26.110134+00:00
- expected: deep links serve SPA (no crash/404 blank); pages show deprecation notice (client); APIs kept
- actual FE: [{"path": "/cms/publishing/review", "http": 200, "spa": true, "blank": false}, {"path": "/cms/publishing/schedule", "http": 200, "spa": true, "blank": false}, {"path": "/cms/publishing/releases", "http": 200, "spa": true, "blank": false}]
- actual API: reviews=200 schedules=200 releases=200
- policy: keep legacy pages + deprecation banner (not unsafe redirect)
- result: PASS
