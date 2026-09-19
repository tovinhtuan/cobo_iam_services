#!/usr/bin/env python3
"""DEV smoke: sequential unlock after predecessor completion. DEV only. No secrets logged."""
from __future__ import annotations

import json
import sys
import urllib.error
import urllib.request
import uuid
from datetime import date

BASE = "http://88.216.208.0:8080"
EMAIL = "admin.dn@example.com"
PASSWORD = "secret"
COMPANY_ID = "c_001"
MEMBERSHIP_ID = "m_102"

results: list[tuple[str, str, str]] = []


def log(name: str, status: str, detail: str = "") -> None:
    results.append((name, status, detail))
    print(f"[{status}] {name}" + (f" — {detail}" if detail else ""))


def req(method: str, path: str, token: str | None = None, body: dict | None = None, headers: dict | None = None):
    data = None
    hdrs = {"Content-Type": "application/json", "Accept": "application/json"}
    if token:
        hdrs["Authorization"] = f"Bearer {token}"
    if headers:
        hdrs.update(headers)
    if body is not None:
        data = json.dumps(body).encode("utf-8")
    url = BASE + path if path.startswith("/") else path
    r = urllib.request.Request(url, data=data, headers=hdrs, method=method)
    try:
        with urllib.request.urlopen(r, timeout=30) as resp:
            raw = resp.read().decode("utf-8")
            payload = json.loads(raw) if raw.strip() else None
            return resp.status, payload, dict(resp.headers)
    except urllib.error.HTTPError as e:
        raw = e.read().decode("utf-8", errors="replace")
        try:
            payload = json.loads(raw) if raw.strip() else {"raw": raw}
        except json.JSONDecodeError:
            payload = {"raw": raw}
        return e.code, payload, dict(e.headers)


def unwrap(payload):
    if isinstance(payload, dict) and "data" in payload:
        return payload["data"]
    return payload


def main() -> int:
    # Health
    st, body, _ = req("GET", "/healthz")
    log("healthz", "PASS" if st == 200 else "FAIL", f"http={st} body={body}")
    st, body, _ = req("GET", "/readyz")
    log("readyz", "PASS" if st == 200 else "FAIL", f"http={st} body={body}")

    # Login + select company
    st, body, _ = req("POST", "/api/v1/auth/login", body={"email": EMAIL, "password": PASSWORD, "remember_me": False})
    if st != 200:
        log("login", "FAIL", f"http={st}")
        return 1
    data = unwrap(body) or {}
    session = data.get("session") or {}
    token = (
        data.get("access_token")
        or data.get("accessToken")
        or session.get("access_token")
        or session.get("pre_company_token")
    )
    if not token:
        log("login", "FAIL", f"no token keys={list(data.keys())} session={list(session.keys())}")
        return 1
    log("login", "PASS", f"next={data.get('next_action')} memberships={len(data.get('memberships') or [])}")

    st, body, _ = req(
        "POST",
        "/api/v1/auth/select-company",
        token=token,
        body={"company_id": COMPANY_ID, "membership_id": MEMBERSHIP_ID},
    )
    if st != 200:
        log("select_company", "FAIL", f"http={st} body={str(body)[:300]}")
        return 1
    data = unwrap(body) or {}
    session = data.get("session") or {}
    token = data.get("access_token") or data.get("accessToken") or session.get("access_token") or token
    log("select_company", "PASS", f"token_len={len(token or '')}")

    # List deadlines — find candidate with >=2 steps, step2 start in future, step1 incomplete
    today = date.today().isoformat()
    candidates = []
    for path in (
        "/api/v1/company/deadlines?limit=50",
        "/api/v1/company/deadline-alerts?limit=50",
        "/api/v1/company/disclosures?limit=50&status=submitted",
    ):
        st, body, _ = req("GET", path, token=token)
        if st != 200:
            continue
        data = unwrap(body) or {}
        items = data if isinstance(data, list) else data.get("items") or data.get("records") or data.get("alerts") or []
        log("list_probe", "INFO", f"{path} http={st} n={len(items) if isinstance(items, list) else type(data)}")
        if isinstance(items, list):
            for it in items:
                rid = it.get("record_id") or it.get("id") or it.get("disclosure_record_id")
                if rid:
                    candidates.append(str(rid))

    # Dedup preserve order
    seen = set()
    record_ids = []
    for r in candidates:
        if r not in seen:
            seen.add(r)
            record_ids.append(r)

    chosen = None
    baseline = None
    for rid in record_ids[:40]:
        st, body, _ = req("GET", f"/api/v1/company/deadlines/{rid}/steps", token=token)
        if st != 200:
            continue
        steps_resp = unwrap(body) or {}
        steps = steps_resp.get("steps") or []
        if len(steps) < 2:
            continue
        s1, s2 = steps[0], steps[1]
        # Prefer: s1 actionable incomplete, s2 planned_start > today, locked
        s1_complete = bool(s1.get("is_completed"))
        s2_start = (s2.get("planned_start_date") or "")[:10]
        actions = s1.get("available_actions") or []
        if s1_complete:
            continue
        if "complete" not in actions:
            continue
        if not s2_start or s2_start <= today:
            # still usable if we can prove unlock; prefer future start
            continue
        if not s2.get("is_future") and s2.get("status") != "not_started":
            continue
        chosen = rid
        baseline = steps_resp
        break

    if not chosen:
        # Fallback: any with s1 completeable and >=2 steps
        for rid in record_ids[:40]:
            st, body, _ = req("GET", f"/api/v1/company/deadlines/{rid}/steps", token=token)
            if st != 200:
                continue
            steps_resp = unwrap(body) or {}
            steps = steps_resp.get("steps") or []
            if len(steps) < 2:
                continue
            s1 = steps[0]
            if s1.get("is_completed"):
                continue
            if "complete" not in (s1.get("available_actions") or []):
                continue
            chosen = rid
            baseline = steps_resp
            log("fixture_note", "WARN", "no future planned_start for s2; using best available")
            break

    if not chosen or not baseline:
        log("fixture", "FAIL", f"no suitable deadline among {len(record_ids)} ids")
        print(json.dumps({"record_ids_sample": record_ids[:10]}, indent=2))
        return 1

    steps = baseline["steps"]
    s1, s2 = steps[0], steps[1]
    s1_code = s1["step_code"]
    s2_code = s2["step_code"]
    s2_start_before = s2.get("planned_start_date")
    s2_end_before = s2.get("planned_end_date")
    log(
        "BASELINE",
        "PASS",
        f"record={chosen} current={baseline.get('current_step_code')} "
        f"s1={s1.get('status')}/{s1.get('available_actions')} "
        f"s2={s2.get('status')} future={s2.get('is_future')} locked={s2.get('is_locked')} "
        f"start={s2_start_before}",
    )

    if "complete" not in (s1.get("available_actions") or []):
        log("BASELINE_STEP1", "FAIL", "missing complete")
        return 1
    log("BASELINE_STEP1", "PASS", "complete available")

    s2_ok_baseline = (
        s2.get("status") in ("not_started", "future")
        or s2.get("is_future") is True
        or s2.get("is_locked") is True
    ) and "complete" not in (s2.get("available_actions") or [])
    log("BASELINE_STEP2", "PASS" if s2_ok_baseline else "FAIL", f"status={s2.get('status')} actions={s2.get('available_actions')}")

    # Complete step 1
    idem = str(uuid.uuid4())
    st, body, hdrs = req(
        "POST",
        f"/api/v1/company/deadlines/{chosen}/steps/{s1_code}/complete",
        token=token,
        body={},
        headers={"Idempotency-Key": idem},
    )
    data = unwrap(body) or body
    log(
        "STEP1_COMPLETE",
        "PASS" if st in (200, 201, 204) else "FAIL",
        f"http={st} current={ (data or {}).get('current_step_code') if isinstance(data, dict) else None } "
        f"req_id={hdrs.get('X-Request-Id') or hdrs.get('x-request-id')}",
    )
    if st not in (200, 201, 204):
        print(json.dumps(body, indent=2, ensure_ascii=False)[:2000])
        return 1

    # Replay idempotency
    st2, body2, _ = req(
        "POST",
        f"/api/v1/company/deadlines/{chosen}/steps/{s1_code}/complete",
        token=token,
        body={},
        headers={"Idempotency-Key": idem},
    )
    log("IDEMPOTENCY_REPLAY", "PASS" if st2 in (200, 201, 204, 409) else "WARN", f"http={st2}")

    # GET steps after
    st, body, _ = req("GET", f"/api/v1/company/deadlines/{chosen}/steps", token=token)
    after = unwrap(body) or {}
    steps = after.get("steps") or []
    s1a = next((x for x in steps if x["step_code"] == s1_code), None)
    s2a = next((x for x in steps if x["step_code"] == s2_code), None)
    if not s1a or not s2a:
        log("STEP2_EARLY_UNLOCK", "FAIL", "missing steps after complete")
        return 1

    unlock_ok = (
        after.get("current_step_code") == s2_code
        and s2a.get("status") == "current"
        and s2a.get("is_future") is False
        and s2a.get("is_locked") is False
        and "complete" in (s2a.get("available_actions") or [])
        and s1a.get("status") == "completed"
        and s1a.get("is_completed") is True
        and not (s1a.get("available_actions") or [])
    )
    dates_ok = s2a.get("planned_start_date") == s2_start_before and s2a.get("planned_end_date") == s2_end_before
    tl = s2a.get("timeliness_status")
    tl_ok = tl != "OVERDUE"

    log(
        "STEP2_EARLY_UNLOCK",
        "PASS" if unlock_ok else "FAIL",
        f"current={after.get('current_step_code')} s2_status={s2a.get('status')} "
        f"future={s2a.get('is_future')} locked={s2a.get('is_locked')} actions={s2a.get('available_actions')} "
        f"s1={s1a.get('status')} dates_ok={dates_ok} tl={tl}",
    )
    log("PLANNED_DATES_PRESERVED", "PASS" if dates_ok else "FAIL", f"{s2_start_before}->{s2a.get('planned_start_date')}")
    log("TIMELINESS", "PASS" if tl_ok else "FAIL", f"timeliness={tl}")

    # Complete step 2
    st, body, _ = req(
        "POST",
        f"/api/v1/company/deadlines/{chosen}/steps/{s2_code}/complete",
        token=token,
        body={},
        headers={"Idempotency-Key": str(uuid.uuid4())},
    )
    log("STEP2_COMPLETE", "PASS" if st in (200, 201, 204) else "FAIL", f"http={st}")
    if st not in (200, 201, 204):
        print(json.dumps(body, indent=2, ensure_ascii=False)[:1500])

    st, body, _ = req("GET", f"/api/v1/company/deadlines/{chosen}/steps", token=token)
    after2 = unwrap(body) or {}
    steps = after2.get("steps") or []
    s2b = next((x for x in steps if x["step_code"] == s2_code), None)
    s3 = steps[2] if len(steps) > 2 else None
    s3_ok = True
    if s3:
        s3_ok = (
            after2.get("current_step_code") == s3["step_code"]
            and s3.get("status") == "current"
            and s3.get("is_future") is False
            and "complete" in (s3.get("available_actions") or [])
        )
        # later steps if any must stay locked
        for later in steps[3:]:
            if later.get("status") == "current" or "complete" in (later.get("available_actions") or []):
                s3_ok = False
    log(
        "STEP3_UNLOCK",
        "PASS" if (s2b and s2b.get("status") == "completed" and s3_ok) else ("SKIP" if not s3 else "FAIL"),
        f"s2={s2b.get('status') if s2b else None} current={after2.get('current_step_code')} "
        f"s3={s3.get('status') if s3 else None}",
    )

    # Authz: no token
    st, _, _ = req("GET", f"/api/v1/company/deadlines/{chosen}/steps")
    log("AUTHZ_NO_TOKEN", "PASS" if st in (401, 403) else "FAIL", f"http={st}")

    # Cross-tenant: try wrong company header if any; select other company if available
    st, body, _ = req(
        "POST",
        f"/api/v1/company/deadlines/{chosen}/steps/{s2_code}/complete",
        token=token,
        body={},
        headers={"Idempotency-Key": str(uuid.uuid4()), "X-Company-Id": "c_other_fake"},
    )
    # Already completed — expect 409/400/403 not success opening wrong tenant
    log("CROSS_TENANT_PROBE", "INFO", f"http={st}")

    # Evidence / comments list if endpoints exist
    for label, path in (
        ("EVIDENCE", f"/api/v1/company/deadlines/{chosen}/steps/{s2_code}/evidence-files"),
        ("DISCUSSION", f"/api/v1/company/deadlines/{chosen}/steps/{s2_code}/comments"),
        ("HISTORY", f"/api/v1/company/deadlines/{chosen}/history"),
    ):
        st, body, _ = req("GET", path, token=token)
        log(label, "PASS" if st in (200, 404) else "WARN", f"http={st}")  # 404 ok if route absent for history

    # FE asset fingerprint
    try:
        with urllib.request.urlopen(BASE.replace(":8080", ":3000") + "/", timeout=15) as resp:
            html = resp.read().decode("utf-8", errors="replace")
        has_new = "index-" in html and ("Chỉnh sửa" not in html)  # label is in JS bundle
        # fetch main js and search label
        import re

        m = re.search(r'/assets/(index-[A-Za-z0-9_-]+\.js)', html)
        label_ok = False
        bundle = ""
        if m:
            bundle = m.group(1)
            with urllib.request.urlopen(BASE.replace(":8080", ":3000") + "/assets/" + bundle, timeout=30) as resp:
                js = resp.read().decode("utf-8", errors="replace")
            label_ok = "Chỉnh sửa tin công bố" in js and "Cập nhật cảnh báo" not in js
        log("FE_BUNDLE_LABEL", "PASS" if label_ok else "FAIL", f"asset={bundle} label_ok={label_ok}")
    except Exception as e:
        log("FE_BUNDLE_LABEL", "FAIL", str(e)[:120])

    fails = [r for r in results if r[1] == "FAIL"]
    print("\n=== SUMMARY ===")
    for name, status, detail in results:
        print(f"{status:5} {name}: {detail}")
    print(f"\nRECORD_ID={chosen}")
    print(f"FAIL_COUNT={len(fails)}")
    return 1 if fails else 0


if __name__ == "__main__":
    sys.exit(main())
