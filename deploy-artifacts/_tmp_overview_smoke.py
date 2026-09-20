#!/usr/bin/env python3
"""DEV smoke: login + dashboard overview + assert new notification shape."""
from __future__ import annotations

import json
import urllib.request
import urllib.error

BASE = "http://88.216.208.0:8080"
EMAIL = "admin.dn@example.com"
PASSWORD = "secret"
COMPANY_ID = "c_001"
MEMBERSHIP_ID = "m_102"
NEEDLE_TITLE = "QA Resmoke Irregular Alert 20260904A"
NEEDLE_RECORD = "52698f3f-53c0-53e7-9e40-d99f90774f41"


def req(method, path, token=None, body=None):
    data = None if body is None else json.dumps(body).encode()
    hdrs = {"Content-Type": "application/json"}
    if token:
        hdrs["Authorization"] = "Bearer " + token
    r = urllib.request.Request(BASE + path, data=data, headers=hdrs, method=method)
    try:
        with urllib.request.urlopen(r, timeout=60) as resp:
            raw = resp.read()
            return resp.status, json.loads(raw.decode() or "null") if raw else None
    except urllib.error.HTTPError as e:
        raw = e.read()
        try:
            return e.code, json.loads(raw.decode() or "null")
        except Exception:
            return e.code, raw.decode(errors="replace")


def main():
    st, login = req("POST", "/api/v1/auth/login", body={"email": EMAIL, "password": PASSWORD, "remember_me": False})
    print("LOGIN", st)
    if st != 200:
        print(login)
        raise SystemExit(1)
    token = login.get("access_token") or login.get("accessToken")
    if not token and isinstance(login.get("tokens"), dict):
        token = login["tokens"].get("access_token")
    # select company
    st2, sel = req("POST", "/api/v1/auth/select-company", token=token, body={"company_id": COMPANY_ID, "membership_id": MEMBERSHIP_ID})
    print("SELECT_COMPANY", st2)
    if st2 == 200:
        token = sel.get("access_token") or sel.get("accessToken") or token
        if isinstance(sel.get("tokens"), dict):
            token = sel["tokens"].get("access_token") or token

    st3, ov = req("GET", "/api/v1/company/dashboard/overview?range=30d", token=token)
    print("OVERVIEW", st3)
    if st3 != 200:
        print(ov)
        raise SystemExit(1)

    acts = ov.get("recent_activities") or []
    print("RECENT_COUNT", len(acts))
    titles = []
    needle = None
    for a in acts:
        titles.append(a.get("title"))
        if a.get("title") == NEEDLE_TITLE or (a.get("target_url") or "").endswith(NEEDLE_RECORD):
            needle = a
    print("TITLES", json.dumps(titles, ensure_ascii=False))
    if not needle:
        # find by title contains QA Resmoke
        for a in acts:
            if "QA Resmoke" in (a.get("title") or ""):
                needle = a
                break
    print("NEEDLE", json.dumps(needle, ensure_ascii=False, indent=2))
    if not needle:
        raise SystemExit(2)

    title = needle.get("title") or ""
    summary = needle.get("summary") or ""
    url = needle.get("target_url") or ""
    assert "Bước phê duyệt đến hạn" not in title, "legacy title still used for new row"
    assert NEEDLE_TITLE in title or "QA Resmoke" in title
    assert "Bước:" in summary and "Hạn:" in summary
    assert "Deadline:" not in summary
    assert url == f"/app/deadlines/{NEEDLE_RECORD}"
    print("API_ASSERT_PASS=true")

    # aggregate frequent late
    flows = ov.get("frequent_late_workflows") or []
    print("AGGREGATE_ROWS", len(flows))
    names = [r.get("workflow_name") for r in flows]
    print("AGGREGATE_NAMES", json.dumps(names, ensure_ascii=False))
    raw_periodic = any((n or "").strip().lower() == "periodic" for n in names)
    print("RAW_PERIODIC_ENUM_VISIBLE", raw_periodic)


if __name__ == "__main__":
    main()
