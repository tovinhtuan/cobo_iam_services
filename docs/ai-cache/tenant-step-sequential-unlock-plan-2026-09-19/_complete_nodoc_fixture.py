#!/usr/bin/env python3
"""Complete first no-doc-gate fixture for browser UI unlock verify."""
from __future__ import annotations

import json
import urllib.error
import urllib.request
import uuid
from datetime import date

BASE = "http://88.216.208.0:8080"


def req(method, path, token=None, body=None, headers=None):
    data = None
    hdrs = {"Content-Type": "application/json", "Accept": "application/json"}
    if token:
        hdrs["Authorization"] = f"Bearer {token}"
    if headers:
        hdrs.update(headers)
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
            payload = json.loads(raw) if raw.strip().startswith("{") else {"raw": raw[:300]}
        except json.JSONDecodeError:
            payload = {"raw": raw[:300]}
        return e.code, payload


def unwrap(p):
    if isinstance(p, dict) and p.get("data") is not None:
        return p["data"]
    return p


def main():
    st, body = req(
        "POST",
        "/api/v1/auth/login",
        body={"email": "admin.dn@example.com", "password": "secret", "remember_me": False},
    )
    d = unwrap(body) or {}
    tok = d["session"]["pre_company_token"]
    st, body = req(
        "POST",
        "/api/v1/auth/select-company",
        token=tok,
        body={"company_id": "c_001", "membership_id": "m_102"},
    )
    d = unwrap(body) or {}
    token = d.get("access_token") or d["session"]["access_token"]
    st, body = req("GET", "/api/v1/company/deadline-alerts?limit=100", token=token)
    items = body.get("items") if isinstance(body, dict) and "items" in body else []
    today = date.today().isoformat()
    for it in items:
        rid = it.get("record_id") or it.get("alert_id")
        st, body = req("GET", f"/api/v1/company/deadlines/{rid}/steps", token=token)
        if st != 200:
            continue
        steps = unwrap(body) or {}
        ss = steps.get("steps") or []
        if len(ss) < 2 or ss[0].get("is_completed"):
            continue
        if "complete" not in (ss[0].get("available_actions") or []):
            continue
        start = (ss[1].get("planned_start_date") or "")[:10]
        if not (start > today and ss[1].get("is_future")):
            continue
        code = ss[0]["step_code"]
        st, body = req(
            "POST",
            f"/api/v1/company/deadlines/{rid}/steps/{code}/complete",
            token=token,
            body={},
            headers={"Idempotency-Key": str(uuid.uuid4())},
        )
        err = None
        if isinstance(body, dict) and st >= 400:
            err = (body.get("error") or {}).get("code")
        print(f"try {rid} http={st} err={err}")
        if st in (200, 201, 204):
            st2, after = req("GET", f"/api/v1/company/deadlines/{rid}/steps", token=token)
            ad = unwrap(after) or {}
            s2 = ad["steps"][1]
            print(
                "PICK",
                json.dumps(
                    {
                        "record_id": rid,
                        "current": ad.get("current_step_code"),
                        "s2_status": s2.get("status"),
                        "is_future": s2.get("is_future"),
                        "is_locked": s2.get("is_locked"),
                        "actions": s2.get("available_actions"),
                        "planned_start": s2.get("planned_start_date"),
                        "timeliness": s2.get("timeliness_status"),
                    },
                    ensure_ascii=False,
                ),
            )
            return
    print("NO_FIXTURE")


if __name__ == "__main__":
    main()
