"""DEV API smoke for GET /api/v1/platform/cms/dashboard/overview (2026-10-11).

Checks route gate (CMS 200, ENT 403, no token 401), envelope, per-section status, forbidden sections
without data, cache / refresh rate limit, numbers cross-checked against MySQL (read-only), legacy
summary still served, audit feed. Personas come from ~/.cobo/dev-qa.env (never printed). Read-only.
"""
import json
import sys
import time
import urllib.error
import urllib.request
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[3] / "scripts" / "devqa"))
from devqa_env import base_url, optional, persona, run_sql  # noqa: E402

BASE = base_url()
OWN = optional("E2E_COMPANY_ID", "c_001")
SECTIONS = ["companies", "subscriptions", "users", "templates", "disclosures", "system_health", "growth"]
results = []


def call(method, path, token=None, body=None):
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(BASE + path, method=method, data=data, headers={"Content-Type": "application/json"})
    if token:
        req.add_header("Authorization", "Bearer " + token)
    started = time.time()
    try:
        with urllib.request.urlopen(req, timeout=40) as r:
            return r.status, r.read(), time.time() - started
    except urllib.error.HTTPError as e:
        return e.code, e.read(), time.time() - started


def data_of(raw):
    try:
        d = json.loads(raw)
    except Exception:
        return {}
    return d.get("data") if isinstance(d, dict) and isinstance(d.get("data"), (dict, list)) else d


def record(ok, name, detail=""):
    results.append((bool(ok), name, str(detail)[:240]))


def login(p, company):
    email, password = persona(p)
    st, raw, _ = call("POST", "/api/v1/auth/login", body={"email": email, "password": password})
    if st != 200:
        raise SystemExit(f"login failed for persona {p}: HTTP {st}")
    body = data_of(raw)
    sess, ctx = body.get("session", {}), body.get("current_context") or {}
    if sess.get("access_token") and (ctx.get("company_id") == company or not sess.get("pre_company_token")):
        return sess["access_token"]
    st, raw, _ = call("POST", "/api/v1/auth/select-company", sess.get("pre_company_token") or sess.get("access_token"), {"company_id": company})
    if st != 200:
        raise SystemExit(f"select-company failed for persona {p}: HTTP {st}")
    return data_of(raw)["access_token"]


def scalar(sql):
    rows = run_sql(sql)
    return int(rows[0].split("\t")[0]) if rows else -1


def main():
    cms = login("CMS", OWN)
    ent = login("ENT", OWN)

    st, raw, _ = call("GET", "/api/v1/platform/cms/dashboard/overview")
    record(st == 401, "no token -> 401", st)
    st, raw, _ = call("GET", "/api/v1/platform/cms/dashboard/overview", ent)
    record(st == 403, "ENT (enterprise admin) -> 403", st)

    st, raw, elapsed = call("GET", "/api/v1/platform/cms/dashboard/overview?refresh=true", cms)
    body = data_of(raw)
    sections = body.get("sections") or {}
    record(st == 200, "CMS overview 200", f"HTTP {st} in {elapsed*1000:.0f} ms")
    record(body.get("timezone") == "Asia/Ho_Chi_Minh" and body.get("generated_at"), "envelope fields", {k: body.get(k) for k in ("generated_at", "cached", "timezone")})
    statuses = {k: (sections.get(k) or {}).get("status") for k in SECTIONS + ["attention"]}
    record(all(statuses[k] in ("ok", "forbidden", "error") for k in SECTIONS), "section statuses valid", statuses)
    record(not any(statuses[k] == "error" for k in SECTIONS), "no section error", statuses)
    leaked = [k for k in SECTIONS if statuses[k] == "forbidden" and "data" in (sections.get(k) or {})]
    record(not leaked, "forbidden sections carry no data", leaked or "none")
    record(elapsed < 2.0, "uncached response < 2 s", f"{elapsed*1000:.0f} ms")
    st_list, _, _ = call("GET", "/api/v1/platform/cms/admin/companies?page=1&limit=1", cms)
    company_sections = {k: statuses[k] for k in ("companies", "subscriptions", "growth")}
    expected = "ok" if st_list == 200 else "forbidden"
    record(all(v == expected for v in company_sections.values()), "company sections follow companies screen gate", f"companies API {st_list} -> {company_sections}")

    comp = (sections.get("companies") or {}).get("data")
    if comp is not None:
        db_total = scalar("SELECT COUNT(*) FROM companies")
        db_unverified = scalar("SELECT COUNT(*) FROM companies WHERE verification_status='unverified'")
        record(comp.get("total") == db_total and comp.get("unverified") == db_unverified, "companies match DB", f"api {comp.get('total')}/{comp.get('unverified')} db {db_total}/{db_unverified}")
    disc = (sections.get("disclosures") or {}).get("data")
    if disc is not None:
        db_records = scalar("SELECT COUNT(*) FROM disclosure_records dr JOIN companies c ON c.company_id=dr.company_id AND c.status='active'")
        by = disc.get("by_status") or {}
        record(sum(by.values()) == db_records, "disclosure status buckets sum = records at active companies", f"api {by} db {db_records}")
        record(len(disc.get("top_overdue_companies") or []) <= 5, "top overdue <= 5", len(disc.get("top_overdue_companies") or []))
        items = (sections.get("attention") or {}).get("items") or []
        overdue_item = next((i for i in items if i.get("code") == "disclosures_overdue"), None)
        record((disc.get("overdue", 0) == 0) == (overdue_item is None), "attention overdue consistent with section", f"overdue {disc.get('overdue')} item {overdue_item}")
    growth = (sections.get("growth") or {}).get("data")
    if growth is not None:
        days = growth.get("days") or []
        record(len(days) == 30, "growth has 30 days", len(days))
        total = sum(d.get("disclosures", 0) for d in days)
        record(total == (growth.get("totals") or {}).get("disclosures"), "growth totals = sum of days", total)

    st, raw, elapsed2 = call("GET", "/api/v1/platform/cms/dashboard/overview", cms)
    record(st == 200 and data_of(raw).get("cached") is True, "second call served from cache", f"cached={data_of(raw).get('cached')} {elapsed2*1000:.0f} ms")
    st, raw, _ = call("GET", "/api/v1/platform/cms/dashboard/overview?refresh=true", cms)
    record(st == 200 and data_of(raw).get("cached") is True, "refresh within 10 s is rate-limited (cache)", f"cached={data_of(raw).get('cached')}")

    st, raw, _ = call("GET", "/api/v1/platform/cms/dashboard/summary", cms)
    record(st == 200, "legacy summary still served (compat)", st)
    st, raw, _ = call("GET", "/api/v1/platform/cms/ops/audit?limit=8", cms)
    items = (data_of(raw) or {}).get("items") or []
    record(st == 200 and len(items) <= 8, "audit feed limit=8", f"HTTP {st}, {len(items)} items")

    out = Path(__file__).with_suffix(".out")
    lines = [f"{'PASS' if ok else 'FAIL'}  {name}  {detail}" for ok, name, detail in results]
    lines.append(f"\n{sum(ok for ok, _, _ in results)}/{len(results)} PASS")
    out.write_text("\n".join(lines) + "\n", encoding="utf-8")
    print("\n".join(lines))
    return 0 if all(ok for ok, _, _ in results) else 1


if __name__ == "__main__":
    sys.exit(main())
