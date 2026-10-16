"""Smoke MR-3 (mr3-obligation-open-archive-2026-10-16) on DEV, through nginx (E2E_BASE_URL).
READ-ONLY: only GET requests (plus login), and SQL through the read-only DB user (run_sql).

1. DB: migration 0154 is in the ledger; no archived global template keeps a PENDING/RETRY cycle
   without a record.
2. Portal (ENT, company E2E_COMPANY_ID, default c_001):
   - every alert carries a bool template_archived; the flagged alerts are exactly the open
     obligations (shared ObligationOpen predicate) of archived templates in the company, and no
     alert of an active template is flagged;
   - /deadline-alerts/next lists no cycle of an archived template;
   - the dashboard overview and "Việc của tôi" answer 200 and carry the flag as a bool.
3. CMS (CMS persona): archive-impact of the archived template with the most cycles without a
   record matches the DB (open records, missed + auto-resumed cycles, list not cut); an active
   template answers 200 with status active; a company template answers 404; the ENT persona (no
   platform.cms.view) gets 403. The CMS dashboard overview answers 200.
4. FE: the deployed bundle contains the MR-3 strings.

    python3 -B scripts/devqa/smoke_mr3_obligation_open.py

Nothing secret is printed. Exit code 1 when a check fails.
"""
import re
import sys
import urllib.request
from datetime import datetime
from pathlib import Path
from zoneinfo import ZoneInfo

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
import devqa_env as env  # noqa: E402
from smoke_workflow_step_skip import call, check, data, login, results  # noqa: E402

SAFE_ID = re.compile(r"^[A-Za-z0-9_.:-]{1,128}$")
TODAY = datetime.now(ZoneInfo("Asia/Ho_Chi_Minh")).strftime("%Y-%m-%d")
ARCHIVED_SQL = "(LOWER(COALESCE(dt.status, '')) = 'archived' OR (dt.company_id IS NOT NULL AND LOWER(COALESCE(dt.review_status, '')) = 'archived'))"
OPEN_SQL = f"""
  AND LOWER(TRIM(dr.status)) = 'draft' AND dr.submitted_at IS NULL
  AND (NOT EXISTS (SELECT 1 FROM periodic_cycles pc_ir WHERE pc_ir.record_id = dr.record_id)
       OR EXISTS (SELECT 1 FROM periodic_cycles pc WHERE pc.record_id = dr.record_id
                  AND COALESCE(pc.open_at, pc.cycle_start) IS NOT NULL
                  AND COALESCE(pc.open_at, pc.cycle_start) <= '{TODAY}'))"""


def sql_rows(statement):
    return [row.split("\t") for row in env.run_sql(statement)]


def sql_int(statement):
    rows = sql_rows(statement)
    return int(rows[0][0]) if rows else 0


def safe(value):
    if not SAFE_ID.match(value or ""):
        raise SystemExit(f"unexpected id format: {value!r}")
    return value


def cms_token():
    email, password = env.persona("CMS")
    status, body = call("POST", "/api/v1/auth/login", body={"email": email, "password": password})
    if status != 200:
        raise SystemExit(f"CMS persona login failed: HTTP {status}")
    sess = data(body).get("session", {})
    if sess.get("access_token"):
        return sess["access_token"]
    status, sel = call("POST", "/api/v1/auth/select-company", sess.get("pre_company_token"),
                       {"company_id": env.optional("E2E_COMPANY_ID", "c_001")})
    if status != 200:
        raise SystemExit(f"CMS persona select-company failed: HTTP {status}")
    return data(sel)["access_token"]


def all_alerts(token):
    items, page = [], 1
    while True:
        status, body = call("GET", f"/api/v1/company/deadline-alerts?page={page}&page_size=100", token)
        if status != 200:
            return status, items, 0
        d = data(body)
        items += d.get("items") or []
        if len(items) >= int(d.get("total") or 0) or not d.get("items") or page >= 50:
            return status, items, int(d.get("total") or 0)
        page += 1


def main():
    company = safe(env.optional("E2E_COMPANY_ID", "c_001"))
    archived_types = {r[0] for r in sql_rows(f"SELECT dt.type_id FROM disclosure_types dt WHERE {ARCHIVED_SQL}")}

    # 1. DB
    check("[DB] 0154 in schema_migrations",
          sql_int("SELECT COUNT(*) FROM schema_migrations WHERE file_name = '0154_periodic_cycles_suspend_archived_templates.up.sql'") == 1, "ledger row")
    leftover = sql_int("""SELECT COUNT(*) FROM periodic_cycles pc JOIN disclosure_types dt ON dt.type_id = pc.type_id
        WHERE dt.company_id IS NULL AND LOWER(dt.status) = 'archived' AND pc.record_id IS NULL
          AND pc.materialization_state IN ('PENDING', 'RETRY')""")
    check("[DB] no PENDING/RETRY cycle left on an archived global template", leftover == 0, f"leftover={leftover}")

    # 2. Portal
    ent, _ = login("ENT")
    status, alerts, total = all_alerts(ent)
    check("[Portal] deadline-alerts 200, all pages read", status == 200 and len(alerts) == total, f"HTTP {status} items={len(alerts)} total={total}")
    check("[Portal] every alert carries a bool template_archived",
          all(isinstance(a.get("template_archived"), bool) for a in alerts), f"{len(alerts)} alerts")
    flagged = [a for a in alerts if a.get("template_archived")]
    expected = sql_int(f"""SELECT COUNT(*) FROM disclosure_records dr JOIN disclosure_types dt ON dt.type_id = dr.type_id
        WHERE dr.company_id = '{company}' AND {ARCHIVED_SQL} {OPEN_SQL}""")
    check("[Portal] open obligations of archived templates are listed and flagged (Q1-B)",
          len(flagged) == expected and expected > 0, f"flagged={len(flagged)} db_open_on_archived={expected}")
    wrong = [a.get("record_id") for a in alerts if bool(a.get("template_archived")) != (a.get("type_id") in archived_types)]
    check("[Portal] the flag matches the template state for every alert", not wrong, f"mismatches={len(wrong)}")

    status, body = call("GET", "/api/v1/company/deadline-alerts/next", ent)
    nxt = data(body)
    nxt = nxt.get("items", []) if isinstance(nxt, dict) else (nxt or [])
    on_archived = [n for n in nxt if n.get("type_id") in archived_types]
    check("[Portal] /next lists no cycle of an archived template", status == 200 and not on_archived, f"HTTP {status} items={len(nxt)} on_archived={len(on_archived)}")

    status, body = call("GET", "/api/v1/company/dashboard/overview?range=30d", ent)
    actions = (data(body) or {}).get("immediate_actions") or []
    check("[Portal] dashboard overview 200, immediate_actions flag is a bool when present",
          status == 200 and all(isinstance(a.get("template_archived", False), bool) for a in actions),
          f"HTTP {status} actions={len(actions)} flagged={sum(1 for a in actions if a.get('template_archived'))}")

    status, body = call("GET", "/api/v1/me/operational-overview", ent)
    tasks = (data(body) or {}).get("my_tasks") or []
    check("[Portal] operational overview 200, my_tasks flag is a bool when present",
          status == 200 and all(isinstance(t.get("template_archived", False), bool) for t in tasks),
          f"HTTP {status} tasks={len(tasks)} flagged={sum(1 for t in tasks if t.get('template_archived'))}")

    # 3. CMS
    cms = cms_token()
    row = sql_rows("""SELECT dt.type_id, COUNT(pc.cycle_id) n FROM disclosure_types dt
        LEFT JOIN periodic_cycles pc ON pc.type_id = dt.type_id AND pc.record_id IS NULL
        WHERE dt.company_id IS NULL AND LOWER(dt.status) = 'archived'
        GROUP BY dt.type_id ORDER BY n DESC, dt.type_id LIMIT 1""")
    if row:
        tid = safe(row[0][0])
        status, body = call("GET", f"/api/v1/platform/cms/templates/{tid}/archive-impact", cms)
        imp = data(body) or {}
        cycles = imp.get("cycles") or {}
        db_open = sql_int(f"SELECT COUNT(*) FROM disclosure_records dr WHERE dr.type_id = '{tid}' {OPEN_SQL}")
        db_cycles = sql_int(f"""SELECT COUNT(*) FROM periodic_cycles WHERE type_id = '{tid}' AND record_id IS NULL
            AND materialization_state IN ('SUSPENDED', 'PENDING', 'RETRY')""")
        suspended = cycles.get("suspended") or []
        ok = (status == 200 and imp.get("status") == "archived"
              and (imp.get("open_records") or {}).get("total") == db_open
              and cycles.get("suspended_total", -1) + cycles.get("auto_resume", -1) == db_cycles
              and len(suspended) == cycles.get("suspended_total"))
        check("[CMS] archive-impact of an archived template matches the DB", ok,
              f"HTTP {status} status={imp.get('status')} open={(imp.get('open_records') or {}).get('total')}/{db_open} "
              f"missed={cycles.get('suspended_total')} auto_resume={cycles.get('auto_resume')} db_cycles={db_cycles} listed={len(suspended)}")
        status, _ = call("GET", f"/api/v1/platform/cms/templates/{tid}/archive-impact", ent)
        check("[CMS] archive-impact without platform.cms.view (ENT) is 403", status == 403, f"HTTP {status}")
    else:
        check("[CMS] archived global template to read", False, "none on DEV")

    row = sql_rows("SELECT type_id FROM disclosure_types WHERE company_id IS NULL AND LOWER(status) = 'active' AND active_version_no > 0 ORDER BY type_id LIMIT 1")
    if row:
        status, body = call("GET", f"/api/v1/platform/cms/templates/{safe(row[0][0])}/archive-impact", cms)
        imp = data(body) or {}
        check("[CMS] archive-impact of an active template", status == 200 and imp.get("status") == "active",
              f"HTTP {status} status={imp.get('status')} open={(imp.get('open_records') or {}).get('total')} pending={(imp.get('cycles') or {}).get('pending')}")
    row = sql_rows("SELECT type_id FROM disclosure_types WHERE company_id IS NOT NULL ORDER BY type_id LIMIT 1")
    if row:
        status, _ = call("GET", f"/api/v1/platform/cms/templates/{safe(row[0][0])}/archive-impact", cms)
        check("[CMS] archive-impact of a company template is 404", status == 404, f"HTTP {status}")
    status, _ = call("GET", "/api/v1/platform/cms/dashboard/overview", cms)
    check("[CMS] dashboard overview 200 (shared predicate)", status == 200, f"HTTP {status}")

    # 4. FE bundle
    with urllib.request.urlopen(env.base_url() + "/", timeout=60) as r:
        html = r.read().decode("utf-8", "replace")
    bundles = re.findall(r'src="(/assets/index-[^"]+\.js)"', html)
    found = False
    for path in bundles:
        with urllib.request.urlopen(env.base_url() + path, timeout=120) as r:
            js = r.read().decode("utf-8", "replace")
        if "archive-impact" in js and "auto_resume" in js and "Mẫu đã ngừng áp dụng" in js:
            found = True
    check("[FE] deployed bundle has the MR-3 label and dialogs", found, f"bundles={bundles}")

    print(f"\n{sum(results)}/{len(results)} checks passed")
    sys.exit(0 if all(results) else 1)


if __name__ == "__main__":
    main()
