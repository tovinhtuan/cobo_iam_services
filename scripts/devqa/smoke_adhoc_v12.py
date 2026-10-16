"""Smoke MR-2 (bug-adhoc-v12-2026-10-16): ad-hoc vote rules of contract v1.2 §3.2 on DEV, through
nginx (E2E_BASE_URL). Proposer ENT, reviewers ENT2 and USER, company E2E_COMPANY_ID (default c_001).

1. Final values (H1) and finalize: reviewer A enters a final T0 and a note, reviewer B approves with
   an empty body. While pending the proposal shows A's values as current; after B's vote the
   proposal is approved with A's values, and its record has exactly one workflow instance whose T0
   is A's date.
2. Reject while a vote is outstanding still works (H2 regression).
3. The last voter sends two approve requests at once (double click). Finalize is serialized: the
   proposal is approved and its record has exactly one workflow instance (review DB-01).
4. The deployed FE bundle contains the MR-2 detail page ("Hoàn tất phê duyệt", the retry flag).

WRITES test-owned data: three proposals with change note "[QA smoke MR-2] ...", two approved (each
with a disclosure record and workflow) and one rejected. Instance state is read through the
read-only DB user (run_sql).

    python3 -B scripts/devqa/smoke_adhoc_v12.py [--type-id ID]

Nothing secret is printed. Exit code 1 when a check fails.
"""
import argparse
import re
import sys
import threading
import urllib.request
import uuid
from datetime import date, timedelta
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
import devqa_env as env  # noqa: E402
from smoke_workflow_step_skip import call, check, data, login, results  # noqa: E402

SAFE_ID = re.compile(r"^[A-Za-z0-9_.:-]{1,128}$")
BASE = "/api/v1/company/ad-hoc-proposals"


def sql_one(statement):
    rows = env.run_sql(statement)
    return rows[0].split("\t") if rows else None


def pick_irregular_type(token, type_id):
    if type_id:
        return type_id
    for page in range(1, 6):
        s, body = call("GET", f"/api/v1/disclosure-types?page_size=100&page={page}", token)
        items = data(body)
        items = items.get("items", []) if isinstance(items, dict) else items
        if s != 200 or not items:
            break
        for it in items:
            if (it.get("template_category") or "").lower() != "irregular":
                continue
            s, w = call("GET", f"/api/v1/disclosure-types/{it['type_id']}/effective-workflow", token)
            if s == 200 and data(w).get("workflow") and data(w).get("workflow_valid", True):
                return it["type_id"]
    raise SystemExit("no irregular type with a valid workflow; pass --type-id")


def create_and_submit(token, type_id, reviewers, case):
    t0 = (date.today() + timedelta(days=7)).isoformat()
    s, body = call("POST", BASE, token, {
        "type_id": type_id,
        "change_note": f"[QA smoke MR-2] {case} {uuid.uuid4().hex[:6]}\nQA smoke, safe to ignore.",
        "proposed_t0_date": t0,
        "proposed_deadline_days": 5,
        "reviewer_membership_ids": reviewers,
    })
    pid = (data(body) or {}).get("proposal_id")
    if not check(f"[{case}] create proposal", s in (200, 201) and pid, f"HTTP {s} {(body.get('error') or {}).get('code', '')}"):
        raise SystemExit(1)
    if not SAFE_ID.match(pid):
        raise SystemExit("unexpected proposal id format")
    s, body = call("POST", f"{BASE}/{pid}/submit", token, {})
    if not check(f"[{case}] submit (re-checks reviewers and proposer)", s == 200, f"HTTP {s} {(body.get('error') or {}).get('code', '')}"):
        raise SystemExit(1)
    return pid


def detail(token, pid):
    s, body = call("GET", f"{BASE}/{pid}", token)
    return s, data(body) or {}


def instances_of(record_id):
    if not record_id or not SAFE_ID.match(record_id):
        return None
    return sql_one(f"SELECT COUNT(*), COALESCE(MIN(DATE_FORMAT(t0_date, '%Y-%m-%d')), '') FROM workflow_instances WHERE record_id = '{record_id}'")


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--type-id", default="")
    args = ap.parse_args()

    for path in ("/healthz", "/readyz"):
        s, _ = call("GET", path)
        check(f"GET {path}", s == 200, f"HTTP {s}")

    ent, _ = login("ENT")
    rev_a, a_id = login("ENT2")
    rev_b, b_id = login("USER")
    type_id = pick_irregular_type(ent, args.type_id)
    print(f"type_id={type_id}")
    final_t0 = (date.today() + timedelta(days=10)).isoformat()
    note = "Dời theo lịch họp HĐQT"

    # 1. H1: last entered wins, a blank keeps it; finalize with one workflow on A's T0.
    pid = create_and_submit(ent, type_id, [a_id, b_id], "H1 final values")
    s, body = call("POST", f"{BASE}/{pid}/approve", rev_a, {"final_t0_date": final_t0, "adjustment_note": note})
    check("[H1] A approves with a final T0 and a note", s == 200 and not body.get("finalized"), f"HTTP {s} finalized={body.get('finalized')}")
    s, p = detail(rev_b, pid)
    check("[H1] pending proposal shows A's values as current",
          s == 200 and (p.get("final_t0_date") or "")[:10] == final_t0 and p.get("adjustment_note") == note
          and not p.get("finalize_retry_available"),
          f"HTTP {s} final_t0={(p.get('final_t0_date') or '')[:10]} note_ok={p.get('adjustment_note') == note} retry={p.get('finalize_retry_available')}")
    s, body = call("POST", f"{BASE}/{pid}/approve", rev_b)
    check("[H1] B approves with an empty body and finalizes", s == 200 and body.get("finalized") is True,
          f"HTTP {s} finalized={body.get('finalized')} {(body.get('error') or {}).get('code', '')}")
    s, p = detail(ent, pid)
    record_id = p.get("record_id") or p.get("disclosure_record_id")
    check("[H1] approved with A's final T0 and note kept",
          p.get("status") == "approved" and (p.get("final_t0_date") or "")[:10] == final_t0 and p.get("adjustment_note") == note,
          f"status={p.get('status')} final_t0={(p.get('final_t0_date') or '')[:10]} note_ok={p.get('adjustment_note') == note}")
    inst = instances_of(record_id)
    check("[H1] one workflow instance on A's T0", inst and inst[0] == "1" and inst[1] == final_t0,
          f"instances={inst and inst[0]} t0={inst and inst[1]}")

    # 2. H2 regression: reject while a vote is outstanding.
    pid2 = create_and_submit(ent, type_id, [a_id, b_id], "H2 reject")
    s, _ = call("POST", f"{BASE}/{pid2}/approve", rev_a, {})
    check("[H2] A approves", s == 200, f"HTTP {s}")
    s, body = call("POST", f"{BASE}/{pid2}/reject", rev_b, {"reject_reason": "QA smoke MR-2"})
    check("[H2] B rejects while a vote is outstanding", s == 200 and (data(body) or {}).get("status") == "rejected",
          f"HTTP {s} status={(data(body) or {}).get('status')}")

    # 3. DB-01: the last voter double-clicks; one finalize, one workflow instance.
    pid3 = create_and_submit(ent, type_id, [a_id, b_id], "double approve")
    s, _ = call("POST", f"{BASE}/{pid3}/approve", rev_a, {})
    check("[DB-01] A approves", s == 200, f"HTTP {s}")
    outcomes = []
    barrier = threading.Barrier(2)

    def fire():
        barrier.wait()
        st, b = call("POST", f"{BASE}/{pid3}/approve", rev_b)
        outcomes.append((st, b.get("finalized"), ((b.get("error") or {}).get("details") or {}).get("reason")))

    threads = [threading.Thread(target=fire) for _ in range(2)]
    for th in threads:
        th.start()
    for th in threads:
        th.join()
    ok = all(st == 200 or (st == 409 and reason == "finalize_in_progress") for st, _, reason in outcomes)
    check("[DB-01] both concurrent approves end cleanly", ok and any(st == 200 and fin for st, fin, _ in outcomes), f"outcomes={outcomes}")
    s, p = detail(ent, pid3)
    inst = instances_of(p.get("record_id") or p.get("disclosure_record_id"))
    check("[DB-01] approved with exactly one workflow instance", p.get("status") == "approved" and inst and inst[0] == "1",
          f"status={p.get('status')} instances={inst and inst[0]}")

    # 4. The deployed FE carries the MR-2 detail page.
    with urllib.request.urlopen(env.base_url() + "/", timeout=30) as r:
        html = r.read().decode("utf-8", "replace")
    bundles = re.findall(r'src="(/assets/index-[^"]+\.js)"', html)
    found = False
    for path in bundles:
        with urllib.request.urlopen(env.base_url() + path, timeout=60) as r:
            js = r.read().decode("utf-8", "replace")
        if "finalize_retry_available" in js and "Hoàn tất phê duyệt" in js:
            found = True
    check("[FE] deployed bundle has the MR-2 detail page", found, f"bundles={bundles}")


if __name__ == "__main__":
    try:
        main()
    except SystemExit as e:
        if e.code not in (None, 0, 1):
            print(e)
    print(f"\n{sum(results)}/{len(results)} checks passed")
    sys.exit(0 if results and all(results) else 1)
