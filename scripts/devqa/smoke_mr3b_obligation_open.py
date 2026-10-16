"""Smoke MR-3b (bug-mr3b-obligation-open-v12-2026-10-16) on DEV, through nginx (E2E_BASE_URL).
READ-ONLY: only GET requests (plus login) and SQL through the read-only DB user (run_sql).

1. Alerts (ENT, company E2E_COMPANY_ID, default c_001): every approved ad-hoc whose record is not
   Approved/Published/Completed/Done and whose proposal has a deadline input is listed with a due
   date and an open status (AC-11); all pages add up to `total`.
2. Dashboard: needs_action_now, pending_approval and blocked_or_exception are the same for every
   range (Q-18); pending_approval and blocked_or_exception match the alerts list.
3. "Việc của tôi": for each persona with pending tasks on open Draft records, my_tasks lists at
   least one of them (AC-13) when the persona has at most 20 pending tasks.
4. Reminder: no deadline occurrence left dispatchable on a closed obligation; worker log clean.
5. FE: the deployed bundle has the MR-3b dashboard copy.

    python3 -B scripts/devqa/smoke_mr3b_obligation_open.py [--snapshot]

--snapshot prints the figures without checking (run it before the deploy). Nothing secret is
printed. Exit code 1 when a check fails.
"""
import argparse
import re
import subprocess
import sys
import urllib.request
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
import devqa_env as env  # noqa: E402
from smoke_workflow_step_skip import call, check, data, login, results  # noqa: E402

SAFE_ID = re.compile(r"^[A-Za-z0-9_.:-]{1,128}$")
OPEN_STATUSES = {"OVERDUE", "DUE_SOON", "UPCOMING"}
KPIS = ("needs_action_now", "pending_approval", "blocked_or_exception")


def safe(value):
    if not SAFE_ID.match(value or ""):
        raise SystemExit(f"unexpected id format: {value!r}")
    return value


def sql_rows(statement):
    return [row.split("\t") for row in env.run_sql(statement)]


def all_alerts(token):
    items, page, total = [], 1, 0
    while page <= 50:
        status, body = call("GET", f"/api/v1/company/deadline-alerts?page={page}&page_size=100", token)
        if status != 200:
            return status, items, total
        d = data(body)
        total = int(d.get("total") or 0)
        items += d.get("items") or []
        if len(items) >= total or not d.get("items"):
            break
        page += 1
    return 200, items, total


def kpi(token, rng):
    status, body = call("GET", f"/api/v1/company/dashboard/overview?range={rng}", token)
    kpis = (data(body) or {}).get("kpis") or {}
    return status, {k: (kpis.get(k) or {}).get("value") for k in KPIS}


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--snapshot", action="store_true")
    snap = ap.parse_args().snapshot
    company = safe(env.optional("E2E_COMPANY_ID", "c_001"))

    # 1. Alerts
    ent, _ = login("ENT")
    status, alerts, total = all_alerts(ent)
    by_id = {a.get("record_id"): a for a in alerts}
    # Approved ad-hoc still open whose proposal carries a deadline input: the list shows
    # obligations with a due date only (v1.2 §5.2); the others must stay out of list and count.
    adhoc = [r[0] for r in sql_rows(f"""SELECT dr.record_id FROM ad_hoc_proposals p
        JOIN disclosure_records dr ON dr.company_id = p.company_id AND dr.record_id = p.record_id
        WHERE p.company_id = '{company}' AND p.status = 'approved'
          AND LOWER(TRIM(dr.status)) NOT IN ('approved', 'published', 'completed', 'done')
          AND (p.final_deadline_date IS NOT NULL OR (p.proposed_t0_date IS NOT NULL AND p.proposed_deadline_days > 0)
               OR p.proposed_deadline_date >= '2000-01-01')""")]
    listed = [rid for rid in adhoc if rid in by_id]
    print(f"alerts: total={total} items={len(alerts)} approved_open_adhoc={len(adhoc)} listed={len(listed)}")
    counts = {s: sum(1 for a in alerts if a.get("status") == s) for s in ("OVERDUE", "DUE_SOON", "UPCOMING", "PENDING_CONFIRM", "DONE")}
    print(f"alert statuses: {counts}")

    # 2. Dashboard
    ranges = {}
    for rng in ("7d", "30d", "90d"):
        s, values = kpi(ent, rng)
        ranges[rng] = (s, values)
        print(f"dashboard {rng}: HTTP {s} {values}")

    # 3. My tasks
    personas = []
    for p in ("ENT", "ENT2", "MEMBER", "USER"):
        token, membership = login(p)
        membership = safe(membership)
        rows = sql_rows(f"""SELECT
            (SELECT COUNT(*) FROM workflow_tasks wt
               JOIN workflow_instances wi ON wi.workflow_instance_id = wt.workflow_instance_id
               JOIN disclosure_records dr ON dr.record_id = wi.record_id AND dr.company_id = wi.company_id
             WHERE LOWER(TRIM(wt.status)) = 'pending' AND LOWER(TRIM(dr.status)) = 'draft' AND dr.submitted_at IS NULL
               AND (wt.assignee_membership_id = '{membership}'
                    OR EXISTS (SELECT 1 FROM workflow_task_assignees a WHERE a.task_id = wt.task_id AND a.membership_id = '{membership}'))),
            (SELECT COUNT(*) FROM workflow_tasks wt
             WHERE LOWER(TRIM(wt.status)) = 'pending'
               AND (wt.assignee_membership_id = '{membership}'
                    OR EXISTS (SELECT 1 FROM workflow_task_assignees a WHERE a.task_id = wt.task_id AND a.membership_id = '{membership}')))""")
        draft_tasks, all_tasks = (int(rows[0][0]), int(rows[0][1])) if rows else (0, 0)
        s, body = call("GET", "/api/v1/me/operational-overview", token)
        tasks = (data(body) or {}).get("my_tasks") or []
        ids = [safe(t.get("alert_id")) for t in tasks if t.get("alert_id")]
        draft_listed = 0
        if ids:
            in_list = ",".join(f"'{i}'" for i in ids)
            draft_listed = int(sql_rows(f"SELECT COUNT(DISTINCT record_id) FROM disclosure_records WHERE record_id IN ({in_list}) AND LOWER(TRIM(status)) = 'draft'")[0][0])
        print(f"my tasks {p}: HTTP {s} items={len(tasks)} draft_records_listed={draft_listed} db_pending_on_draft={draft_tasks} db_pending_all={all_tasks}")
        personas.append((p, s, len(tasks), draft_listed, draft_tasks, all_tasks))

    # 4. Reminder
    closed = sql_rows("SELECT COUNT(*) FROM reminder_occurrences WHERE last_error_code = 'OBLIGATION_CLOSED'")[0][0]
    stale = sql_rows("""SELECT COUNT(*) FROM reminder_occurrences o JOIN disclosure_records dr ON dr.record_id = o.disclosure_id
        WHERE o.scope_type = 'DISCLOSURE' AND o.status IN ('PENDING', 'RETRY_SCHEDULED') AND dr.submitted_at IS NOT NULL
          AND NOT EXISTS (SELECT 1 FROM ad_hoc_proposals p WHERE p.record_id = dr.record_id AND p.status = 'approved')""")[0][0]
    print(f"reminder: obligation_closed={closed} dispatchable_on_closed={stale}")

    if snap:
        return

    check("[Alerts] list 200, all pages add up to total", status == 200 and len(alerts) == total, f"items={len(alerts)} total={total}")
    check("[Alerts] approved open ad-hoc are listed (AC-11)", len(listed) == len(adhoc) and len(adhoc) > 0, f"{len(listed)}/{len(adhoc)}")
    bad = [rid for rid in listed if not by_id[rid].get("due_date") or by_id[rid].get("status") not in OPEN_STATUSES]
    check("[Alerts] ad-hoc rows carry a due date and an open status", not bad, f"bad={len(bad)}")
    check("[Alerts] no PENDING_CONFIRM left (v1.2)", counts["PENDING_CONFIRM"] == 0, f"pending_confirm={counts['PENDING_CONFIRM']}")

    statuses = {s for s, _ in ranges.values()}
    values = {tuple(v[k] for k in KPIS) for _, v in ranges.values()}
    check("[Dashboard] open KPIs are the same for 7d/30d/90d (Q-18)", statuses == {200} and len(values) == 1, f"{values}")
    v30 = ranges["30d"][1]
    open_total = counts["OVERDUE"] + counts["DUE_SOON"] + counts["UPCOMING"]
    check("[Dashboard] pending_approval = open alerts, blocked_or_exception = overdue alerts",
          v30["pending_approval"] == open_total and v30["blocked_or_exception"] == counts["OVERDUE"],
          f"pending_approval={v30['pending_approval']}/{open_total} blocked={v30['blocked_or_exception']}/{counts['OVERDUE']}")

    check("[My tasks] operational overview 200 for every persona", all(p[1] == 200 for p in personas),
          ", ".join(f"{p[0]}={p[1]}" for p in personas))
    eligible = [p for p in personas if p[4] > 0 and p[5] <= 20]
    for p, s, n, draft_listed, draft_tasks, all_tasks in eligible:
        check(f"[My tasks] {p}: tasks of open Draft records are listed (AC-13)", s == 200 and draft_listed > 0,
              f"items={n} draft_records_listed={draft_listed} db_pending_on_draft={draft_tasks}")
    if not eligible:
        # On DEV every pending task of an open Draft record belongs to m_system_worker (no user):
        # AC-13 is covered by TestMineObligation_TasksOfOpenDraftRecordsAreMine on MySQL 8.
        print("SKIP  [My tasks] AC-13: no persona has pending tasks on open Draft records")

    check("[Reminder] no deadline occurrence dispatchable on a closed obligation", stale == "0", f"dispatchable_on_closed={stale}")
    r = subprocess.run(env.ssh_cmd() + ["docker logs --since 15m cobo-iam-worker 2>&1 | grep -ciE 'panic|fatal'; docker logs --since 15m cobo-iam-api 2>&1 | grep -ciE 'panic|fatal'"],
                       capture_output=True, timeout=60)
    nums = [int(x) for x in r.stdout.decode().split() if x.isdigit()]
    check("[Logs] api/worker: no panic or fatal in 15 minutes", r.returncode in (0, 1) and sum(nums) == 0, f"counts={nums}")

    with urllib.request.urlopen(env.base_url() + "/", timeout=60) as resp:
        html = resp.read().decode("utf-8", "replace")
    bundles = re.findall(r'src="(/assets/index-[^"]+\.js)"', html)
    found = False
    for path in bundles:
        with urllib.request.urlopen(env.base_url() + path, timeout=120) as resp:
            if "không phụ thuộc khoảng thời gian đang chọn" in resp.read().decode("utf-8", "replace"):
                found = True
    check("[FE] deployed bundle has the MR-3b dashboard copy", found, f"bundles={bundles}")

    print(f"\n{sum(results)}/{len(results)} checks passed")
    sys.exit(0 if all(results) else 1)


if __name__ == "__main__":
    main()
