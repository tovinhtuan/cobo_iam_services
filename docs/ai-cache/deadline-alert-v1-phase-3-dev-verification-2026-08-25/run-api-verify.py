#!/usr/bin/env python3
"""Phase 3 DEV verification helper — Deadline Alert V1.
No tokens/passwords written to evidence files.
"""
from __future__ import annotations

import json
import os
import subprocess
import sys
import urllib.request
import urllib.error
from datetime import date

API = os.environ.get("API_BASE", "http://88.216.208.0:8080").rstrip("/")
COMPANY = "c_001"
EMAIL = os.environ.get("QA_EMAIL", "admin.dn@example.com")
PASSWORD = os.environ.get("QA_PASSWORD", "secret")
TYPE_ID = os.environ.get("QA_TYPE_ID", "bao-cao-tuan-test")
DEPT = os.environ.get("QA_DEPT_ID", "019f1734-20bf-79e9-abf8-9ca5bc7cdf98")
SSH_HOST = os.environ.get("DEV_HOST", "88.216.208.0")
SSH_PORT = os.environ.get("DEV_PORT", "21239")
SSH_USER = os.environ.get("DEV_USER", "root")
TAG = "qa-dav1-20260825"
TODAY = "2026-08-25"

results = {}


def mark(k, v, note=""):
    results[k] = {"status": v, "note": note}
    print(f"{v}: {k}" + (f" — {note}" if note else ""))


def http(method, path, token=None, body=None):
    data = None
    headers = {"Content-Type": "application/json"}
    if token:
        headers["Authorization"] = f"Bearer {token}"
    if body is not None:
        data = json.dumps(body).encode()
    req = urllib.request.Request(API + path, data=data, headers=headers, method=method)
    try:
        with urllib.request.urlopen(req, timeout=60) as resp:
            raw = resp.read().decode()
            return resp.status, json.loads(raw) if raw else {}
    except urllib.error.HTTPError as e:
        raw = e.read().decode()
        try:
            payload = json.loads(raw) if raw else {}
        except Exception:
            payload = {"raw": raw[:500]}
        return e.code, payload


def ssh_mysql(sql: str) -> str:
    # Pass SQL via stdin to mysql to avoid shell quoting issues.
    remote = (
        "docker exec -i cobo-iam-mysql mysql -uroot -proot cobo_iam -N"
    )
    cmd = ["ssh", "-p", SSH_PORT, f"{SSH_USER}@{SSH_HOST}", remote]
    proc = subprocess.run(
        cmd,
        input=sql,
        capture_output=True,
        text=True,
        check=False,
    )
    out = (proc.stdout or "") + (proc.stderr or "")
    if proc.returncode != 0:
        raise RuntimeError(f"mysql rc={proc.returncode}: {out[:800]}")
    lines = [ln for ln in out.splitlines() if "Using a password" not in ln]
    return "\n".join(lines).strip()


def login():
    st, d = http("POST", "/api/v1/auth/login", body={"login_id": EMAIL, "password": PASSWORD})
    if st != 200:
        raise SystemExit(f"login failed {st}")
    pre = (d.get("session") or {}).get("pre_company_token") or (d.get("session") or {}).get("access_token")
    st2, d2 = http(
        "POST",
        "/api/v1/auth/select-company",
        token=pre,
        body={"company_id": COMPANY},
    )
    if st2 != 200:
        raise SystemExit(f"select-company failed {st2} {d2}")
    token = d2.get("access_token") or (d2.get("session") or {}).get("access_token")
    if not token:
        raise SystemExit(f"no access token after select-company keys={list(d2.keys())}")
    return token


def create_record(token, title, planned):
    st, d = http(
        "POST",
        "/api/v1/disclosures",
        token=token,
        body={
            "type_id": TYPE_ID,
            "department_id": DEPT,
            "title": title,
            "summary": TAG,
            "content": f"{TAG} content",
            "planned_date": planned,
        },
    )
    if st not in (200, 201):
        raise SystemExit(f"create failed {st} {d}")
    rid = d.get("record_id") or (d.get("data") or {}).get("record_id")
    if not rid:
        # sometimes nested
        raise SystemExit(f"no record_id in create resp keys={list(d.keys())} sample={str(d)[:300]}")
    return rid


def insert_cycle(cycle_id, label, record_id, open_at, cycle_start, due_date):
    open_sql = "NULL" if open_at is None else f"'{open_at}'"
    start_sql = "NULL" if cycle_start is None else f"'{cycle_start}'"
    sql = f"""
INSERT INTO periodic_cycles
(cycle_id, type_id, company_id, cycle_label, cycle_start, due_date, record_id, materialized_at, open_at)
VALUES
('{cycle_id}','{TYPE_ID}','{COMPANY}','{label}',{start_sql},'{due_date}','{record_id}',NOW(3),{open_sql})
ON DUPLICATE KEY UPDATE record_id=VALUES(record_id), open_at=VALUES(open_at), cycle_start=VALUES(cycle_start), due_date=VALUES(due_date);
"""
    ssh_mysql(sql)


def list_alerts(token, page_size=100):
    st, d = http("GET", f"/api/v1/company/deadline-alerts?page=1&page_size={page_size}", token=token)
    return st, d


def alert_ids(d):
    items = d.get("items") or d.get("data", {}).get("items") or []
    return {it.get("record_id") or it.get("alert_id"): it for it in items}


def db_row(record_id):
    sql = f"""
SELECT CONCAT_WS('|',
  COALESCE(dr.status,''),
  IF(dr.submitted_at IS NULL,'NULL',DATE_FORMAT(dr.submitted_at,'%Y-%m-%d')),
  COALESCE(DATE_FORMAT(pc.open_at,'%Y-%m-%d'),'NULL'),
  COALESCE(DATE_FORMAT(pc.cycle_start,'%Y-%m-%d'),'NULL'),
  COALESCE(DATE_FORMAT(dr.planned_date,'%Y-%m-%d'),'NULL'),
  IF(pc.record_id IS NULL,'NO_PC','HAS_PC')
)
FROM disclosure_records dr
LEFT JOIN periodic_cycles pc ON pc.record_id=dr.record_id
WHERE dr.record_id='{record_id}' LIMIT 1;
"""
    return ssh_mysql(sql)


def main():
    token = login()
    mark("AUTH_LOGIN_SELECT_COMPANY", "PASS")

    # Cleanup prior tag cycles/labels if re-run
    ssh_mysql(f"DELETE FROM periodic_cycles WHERE cycle_id LIKE '{TAG}%' OR cycle_label LIKE '{TAG}%';")
    # Leave prior disclosure_records (safe); titles include TAG for identification.

    ids = {}
    ids["pre"] = create_record(token, f"{TAG} PRE-OPENAT", "2026-09-15")
    ids["overdue"] = create_record(token, f"{TAG} OVERDUE", "2026-08-20")
    ids["today"] = create_record(token, f"{TAG} DUE-TODAY", TODAY)
    ids["future"] = create_record(token, f"{TAG} FUTURE", "2026-09-10")
    ids["irregular"] = create_record(token, f"{TAG} IRREGULAR", "2026-08-18")
    ids["submit"] = create_record(token, f"{TAG} SUBMIT-ME", "2026-08-15")
    mark("QA_RECORDS_CREATED", "PASS", f"n={len(ids)}")

    insert_cycle(f"{TAG}-pre", f"{TAG}-pre", ids["pre"], "2026-09-01", "2026-08-01", "2026-09-15")
    insert_cycle(f"{TAG}-ov", f"{TAG}-ov", ids["overdue"], "2026-08-01", "2026-07-01", "2026-08-20")
    insert_cycle(f"{TAG}-td", f"{TAG}-td", ids["today"], "2026-08-25", "2026-08-01", TODAY)
    insert_cycle(f"{TAG}-fu", f"{TAG}-fu", ids["future"], "2026-08-10", "2026-08-01", "2026-09-10")
    insert_cycle(f"{TAG}-sub", f"{TAG}-sub", ids["submit"], "2026-08-01", "2026-07-01", "2026-08-15")
    # irregular: no cycle
    mark("QA_PERIODIC_CYCLES_LINKED", "PASS")

    # DB inventory
    for k, rid in ids.items():
        results[f"DB_{k}"] = {"record_id": rid, "row": db_row(rid)}

    st, alerts = list_alerts(token)
    mark("API_LIST_HTTP", "PASS" if st == 200 else "FAIL", f"status={st}")
    if st != 200:
        print(json.dumps(alerts)[:500])
        raise SystemExit(1)
    by = alert_ids(alerts)
    total = alerts.get("total")
    if total is None:
        total = len(alerts.get("items") or [])
    mark("API_TOTAL", "INFO", f"total={total} items={len(by)}")

    # duplicates
    items = alerts.get("items") or []
    seen = {}
    dup = 0
    for it in items:
        rid = it.get("record_id")
        seen[rid] = seen.get(rid, 0) + 1
    dup = sum(1 for v in seen.values() if v > 1)
    mark("DUPLICATE_ALERT_ROWS", "PASS" if dup == 0 else "FAIL", f"dup_keys={dup}")

    def expect(name, rid, present, status=None):
        got = rid in by
        ok = got == present
        note = f"record={rid[:8]}… present={got}"
        if got and status:
            note += f" status={by[rid].get('status')}"
            ok = ok and by[rid].get("status") == status
        mark(name, "PASS" if ok else "FAIL", note)
        return ok

    expect("PRE_OPENAT_HIDDEN", ids["pre"], False)
    expect("API_RETURNS_ACTIONABLE_PERIODIC_DRAFT", ids["future"], True)  # open reached + future due
    expect("PERIODIC_FUTURE_DUE_STATUS", ids["future"], True, "UPCOMING")
    expect("DUE_TODAY_STATUS", ids["today"], True, "DUE_SOON")
    expect("PERIODIC_UNSUBMITTED_OVERDUE", ids["overdue"], True, "OVERDUE")
    expect("IRREGULAR_ALERT_REGRESSION", ids["irregular"], True)
    expect("SUBMIT_BEFORE_VISIBLE", ids["submit"], True)

    # Cross-company: login c_002 if membership exists — optional soft
    # Post-submit
    st_sub, sub_body = http("POST", f"/api/v1/disclosures/{ids['submit']}/submit", token=token)
    mark("SUBMIT_ACTION_HTTP", "PASS" if st_sub in (200, 201) else "FAIL", f"http={st_sub}")
    if st_sub not in (200, 201):
        print("submit body", str(sub_body)[:400])
    after = db_row(ids["submit"])
    results["SUBMIT_AFTER_DB"] = after
    st2, alerts2 = list_alerts(token)
    by2 = alert_ids(alerts2)
    gone = ids["submit"] not in by2
    mark("SUBMIT_REMOVES_DEADLINE_ALERT", "PASS" if gone else "FAIL")
    # PendingReview should not appear as company overdue
    mark(
        "INTERNAL_WORKFLOW_DELAY_IS_NOT_COMPANY_OVERDUE",
        "PASS" if gone else "FAIL",
        f"after_db={after}",
    )

    # PendingReview existing fixtures should not be in list
    legacy_pr = "01a0201e-120c-72ab-a30a-8a807e73a530"
    mark(
        "LEGACY_PENDING_REVIEW_NOT_LISTED",
        "PASS" if legacy_pr not in by2 else "FAIL",
    )

    # write machine summary without secrets
    out = {
        "today_hcm_assumed": TODAY,
        "type_id": TYPE_ID,
        "company_id": COMPANY,
        "record_ids": ids,
        "results": results,
        "api_sample_statuses": {
            rid: (by2.get(rid) or by.get(rid) or {}).get("status")
            for rid in [ids["overdue"], ids["today"], ids["future"], ids["irregular"]]
        },
    }
    print("===SUMMARY_JSON===")
    print(json.dumps(out, indent=2, ensure_ascii=False))
    fails = [k for k, v in results.items() if isinstance(v, dict) and v.get("status") == "FAIL"]
    sys.exit(1 if fails else 0)


if __name__ == "__main__":
    main()
