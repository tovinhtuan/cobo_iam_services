#!/usr/bin/env python3
"""Find DEV deadline suitable for browser sequential-unlock smoke."""
from __future__ import annotations

import json
import urllib.error
import urllib.request
from datetime import date

BASE = "http://88.216.208.0:8080"


def req(method, path, token=None, body=None):
    data = None
    hdrs = {"Content-Type": "application/json", "Accept": "application/json"}
    if token:
        hdrs["Authorization"] = f"Bearer {token}"
    if body is not None:
        data = json.dumps(body).encode()
    r = urllib.request.Request(BASE + path, data=data, headers=hdrs, method=method)
    try:
        with urllib.request.urlopen(r, timeout=30) as resp:
            raw = resp.read().decode()
            return resp.status, json.loads(raw) if raw.strip() else None
    except urllib.error.HTTPError as e:
        raw = e.read().decode(errors="replace")
        try:
            payload = json.loads(raw) if raw.strip().startswith("{") else {"raw": raw[:200]}
        except json.JSONDecodeError:
            payload = {"raw": raw[:200]}
        return e.code, payload


def unwrap(payload):
    if isinstance(payload, dict) and "data" in payload and payload["data"] is not None:
        return payload["data"]
    return payload


def main():
    st, body = req("POST", "/api/v1/auth/login", body={"email": "admin.dn@example.com", "password": "secret", "remember_me": False})
    d = unwrap(body) or {}
    tok = d["session"]["pre_company_token"]
    st, body = req("POST", "/api/v1/auth/select-company", token=tok, body={"company_id": "c_001", "membership_id": "m_102"})
    d = unwrap(body) or {}
    token = d.get("access_token") or (d.get("session") or {}).get("access_token")
    st, body = req("GET", "/api/v1/company/deadline-alerts?limit=100", token=token)
    data = unwrap(body)
    # envelope variants
    if isinstance(body, dict) and isinstance(body.get("items"), list):
        items = body["items"]
    elif isinstance(data, dict):
        items = data.get("items") or data.get("alerts") or []
    elif isinstance(data, list):
        items = data
    else:
        items = []
    print(f"alerts={len(items)}")
    today = date.today().isoformat()
    found = []
    for it in items:
        rid = it.get("record_id") or it.get("id") or it.get("alert_id")
        if not rid:
            continue
        st, body = req("GET", f"/api/v1/company/deadlines/{rid}/steps", token=token)
        if st != 200:
            continue
        steps_resp = unwrap(body) or {}
        ss = steps_resp.get("steps") or []
        if len(ss) < 2:
            continue
        s1, s2 = ss[0], ss[1]
        if s1.get("is_completed"):
            continue
        if "complete" not in (s1.get("available_actions") or []):
            continue
        start = (s2.get("planned_start_date") or "")[:10]
        found.append(
            {
                "record_id": rid,
                "current": steps_resp.get("current_step_code"),
                "s1": s1.get("step_code"),
                "s2": s2.get("step_code"),
                "s2_start": start,
                "s2_future": s2.get("is_future"),
                "s2_status": s2.get("status"),
                "future_ok": bool(start and start > today and s2.get("is_future")),
            }
        )
    print(json.dumps(found[:15], indent=2, ensure_ascii=False))
    futures = [f for f in found if f["future_ok"]]
    print(f"actionable={len(found)} with_future_s2={len(futures)}")
    if futures:
        print("PICK", futures[0]["record_id"])
    elif found:
        print("PICK", found[0]["record_id"])


if __name__ == "__main__":
    main()
