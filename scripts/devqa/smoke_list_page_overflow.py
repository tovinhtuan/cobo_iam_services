"""Smoke a huge ?page= on DEV list endpoints (regression for 61eb75c, pagination.Window / Offset).

Handlers parse page with strconv.Atoi (an out-of-range value clamps to MaxInt) and services only
clamp page from below, so (page-1)*page_size overflowed. Before 61eb75c: in-memory list paths
panicked (nginx 502, connection dropped), SQL paths got a negative OFFSET (500).

For each endpoint, through nginx (E2E_BASE_URL) as personas ENT (company) and CMS (platform):
page=1 baseline, then page=500000000000000000 and page=100000000000000000000 must return 200,
no items and the baseline total, exactly like a normal page just past the end (some endpoints
return items null there, others []). Path ids (record, step, type, template, global record) are
discovered from list endpoints; an endpoint without an id is reported as SKIP.

    python3 -B scripts/devqa/smoke_list_page_overflow.py

Read-only (GET + login). Nothing secret is printed. Exit code 1 when a check fails.
"""
import json
import sys
import urllib.error
import urllib.parse
import urllib.request
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
import devqa_env as env  # noqa: E402

HUGE_PAGE = "500000000000000000"  # (page-1)*20 overflows int64
OUT_OF_INT_PAGE = "100000000000000000000"  # Atoi clamps to MaxInt
PAGE_SIZE = 20

results = []


def call(path, token=None, method="GET", body=None):
    """Returns (status, json body); status None when the connection dropped without a response."""
    req = urllib.request.Request(env.base_url() + path, method=method,
                                 data=json.dumps(body).encode() if body is not None else None,
                                 headers={"Content-Type": "application/json"})
    if token:
        req.add_header("Authorization", "Bearer " + token)
    try:
        with urllib.request.urlopen(req, timeout=60) as r:
            return r.status, json.loads(r.read() or b"{}")
    except urllib.error.HTTPError as e:
        raw = e.read()
        try:
            return e.code, json.loads(raw or b"{}")
        except ValueError:
            return e.code, {}
    except Exception as e:  # noqa: BLE001 - connection reset / remote closed
        return None, {"_exc": type(e).__name__}


def persona_token(p):
    email, password = env.persona(p)
    status, body = call("/api/v1/auth/login", method="POST", body={"email": email, "password": password})
    if status != 200:
        raise SystemExit(f"{p} persona login failed: HTTP {status}")
    sess = body.get("data", body).get("session", {})
    if sess.get("access_token"):
        return sess["access_token"]
    status, sel = call("/api/v1/auth/select-company", sess.get("pre_company_token"), "POST",
                       {"company_id": env.optional("E2E_COMPANY_ID", "c_001")})
    if status != 200:
        raise SystemExit(f"{p} persona select-company failed: HTTP {status}")
    return sel.get("data", sel)["access_token"]


def page_info(body):
    """(items, total) across flat {items|comments, total} and envelope {data, meta} shapes."""
    if not isinstance(body, dict):
        return None, None
    layers = [body] + ([body["data"]] if isinstance(body.get("data"), dict) else [])
    items = total = None
    for layer in layers:
        for key in ("items", "comments"):
            if items is None and isinstance(layer.get(key), list):
                items = layer[key]
        if total is None and isinstance(layer.get("total"), int):
            total = layer["total"]
        for meta_key in ("meta", "pagination"):
            meta = layer.get(meta_key)
            if total is None and isinstance(meta, dict) and isinstance(meta.get("total"), int):
                total = meta["total"]
    if items is None and isinstance(body.get("data"), list):
        items = body["data"]
    return items, total


def find_first(obj, key):
    """First non-empty value of key anywhere in a JSON tree."""
    if isinstance(obj, dict):
        if obj.get(key):
            return obj[key]
        obj = list(obj.values())
    if isinstance(obj, list):
        for value in obj:
            found = find_first(value, key)
            if found:
                return found
    return None


def status_text(status, body):
    return f"HTTP {status}" if status is not None else f"NO RESPONSE ({body.get('_exc')})"


def items_text(items):
    return "null" if items is None else str(len(items))


def check(name, ok, detail):
    results.append(ok)
    print(f"{'PASS' if ok else 'FAIL'}  {name}  -> {detail}")


def smoke(label, token, path, size_param="page_size", extra=""):
    status, body = call(f"{path}?page=1&{size_param}={PAGE_SIZE}{extra}", token)
    items, total = page_info(body)
    ok = status == 200 and total is not None and (items is not None or total == 0)
    check(f"{label} baseline", ok, f"{status_text(status, body)} items={items_text(items)} total={total}"
          + ("" if ok else f" keys={list(body)[:6]}"))
    if not ok:
        return
    past_end = total // PAGE_SIZE + 2
    _, ref = call(f"{path}?page={past_end}&{size_param}={PAGE_SIZE}{extra}", token)
    ref_items, _ = page_info(ref)
    for page in (HUGE_PAGE, OUT_OF_INT_PAGE):
        status, body = call(f"{path}?page={page}&{size_param}={PAGE_SIZE}{extra}", token)
        h_items, h_total = page_info(body)
        check(f"{label} page={page}",
              status == 200 and not h_items and h_items == ref_items and h_total == total,
              f"{status_text(status, body)} items={items_text(h_items)} total={h_total} "
              f"(page {past_end}: items={items_text(ref_items)})")


def skip(label, reason):
    print(f"SKIP  {label}  -> {reason}")


def main():
    print(f"target: {env.base_url()}")
    ent, cms = persona_token("ENT"), persona_token("CMS")
    company_id = env.optional("E2E_COMPANY_ID", "c_001")
    q = urllib.parse.quote

    # Discover path ids (read-only).
    _, alerts = call("/api/v1/company/deadline-alerts?page=1&page_size=50", ent)
    record_id = find_first(alerts, "record_id")
    step_code = None
    if record_id:
        _, steps = call(f"/api/v1/company/deadlines/{q(record_id)}/steps", ent)
        step_code = find_first(steps, "step_code")
    _, types = call("/api/v1/disclosure-types?page=1&page_size=1", ent)
    type_id = find_first(types, "type_id")
    _, cms_types = call("/api/v1/disclosure-types?list_mode=management&page=1&page_size=100", cms)
    template_id = global_record_id = None
    for t in (cms_types.get("items") or [])[:40]:
        status, recs = call(f"/api/v1/platform/cms/templates/{q(t['type_id'])}/records?page=1&page_size=1", cms)
        _, rec_total = page_info(recs)
        if status == 200 and rec_total:
            template_id, global_record_id = t["type_id"], find_first(recs, "id")
            break
        template_id = template_id or (t["type_id"] if status == 200 else None)

    smoke("deadline-alerts (SQL, no filter)", ent, "/api/v1/company/deadline-alerts")
    smoke("deadline-alerts (memory, start_date filter)", ent, "/api/v1/company/deadline-alerts",
          extra="&start_date=2000-01-01")
    smoke("admin memberships (memory)", ent, f"/api/v1/admin/companies/{q(company_id)}/memberships")
    smoke("ad-hoc proposals (SQL)", ent, "/api/v1/company/ad-hoc-proposals")
    if type_id:
        smoke("workflow override versions (SQL)", ent,
              f"/api/v1/company/disclosure-types/{q(type_id)}/workflow-override/versions")
    else:
        skip("workflow override versions", "no disclosure type visible to ENT")
    if record_id:
        smoke("reminder history (SQL)", ent, f"/api/v1/disclosures/{q(record_id)}/reminders/history")
    else:
        skip("reminder history", "no deadline record for ENT")
    if record_id and step_code:
        base = f"/api/v1/company/deadlines/{q(record_id)}/steps/{q(step_code)}"
        smoke("step comments (SQL)", ent, f"{base}/comments")
        smoke("mention candidates (SQL)", ent, f"{base}/comment-mention-candidates", extra="&q=an")
    else:
        skip("step comments / mention candidates", "no deadline record step for ENT")
    smoke("cms admin users (memory)", cms, "/api/v1/platform/cms/admin/users", extra=f"&company_id={q(company_id)}")
    smoke("cms companies (SQL)", cms, "/api/v1/platform/cms/admin/companies", size_param="limit")
    smoke("cms listed companies (SQL)", cms, "/api/v1/platform/cms/market/listed-companies", size_param="limit")
    if template_id:
        smoke("cms global records by template (SQL)", cms, f"/api/v1/platform/cms/templates/{q(template_id)}/records")
    else:
        skip("cms global records by template", "no CMS template")
    if global_record_id:
        smoke("cms global record company-records (SQL)", cms,
              f"/api/v1/platform/cms/records/{q(global_record_id)}/company-records")
    else:
        skip("cms global record company-records", "no CMS template with global records")

    passed = sum(results)
    print(f"\n{passed}/{len(results)} passed")
    sys.exit(0 if passed == len(results) else 1)


if __name__ == "__main__":
    main()
