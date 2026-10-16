"""Smoke MR-1 (bug-workflow-step-skip-2026-10-16) on DEV through nginx (E2E_BASE_URL), as persona ENT.

WF-01: a record submitted on a template workflow gets a task for step 1 only, assigned to the
submitter. That task must not complete the workflow, nor approve the record, while steps are open:

- after submit, available_actions offers no approve/review/confirm;
- the completing action -> 409 WORKFLOW_STEPS_INCOMPLETE listing every open step; record and
  instance unchanged;
- after completing step 1 on the deadline screen, it still lists the remaining steps;
- after completing every step, the action is offered and completes the instance (approve also
  approves the record; review/confirm do not, see WF-07).

--action picks the completing action (default review): on DEV the ENT persona holds
workflow.review/confirm but not workflow.approve, which is refused by policy before the gate.

WRITES test-owned data in company E2E_COMPANY_ID (default c_001): one record titled
"[QA smoke MR-1] ...", submitted, its steps completed and the record approved. The type is
--type-id, or the first visible type whose effective workflow has 2..6 steps and no required
document. Instance state is read through the read-only DB user (run_sql).

    python3 -B scripts/devqa/smoke_workflow_step_skip.py [--type-id ID] [--action review|confirm|approve]

Nothing secret is printed. Exit code 1 when a check fails.
"""
import argparse
import json
import re
import sys
import urllib.error
import urllib.request
import uuid
from datetime import date
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
import devqa_env as env  # noqa: E402

COMPLETING = ("approve", "review", "confirm")
SAFE_ID = re.compile(r"^[A-Za-z0-9_.:-]{1,128}$")

results = []


def call(method, path, token=None, body=None, headers=None):
    """Returns (status, json body); status None when the connection dropped without a response."""
    req = urllib.request.Request(env.base_url() + path, method=method,
                                 data=json.dumps(body).encode() if body is not None else None,
                                 headers={"Content-Type": "application/json", **(headers or {})})
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


def data(body):
    return body.get("data", body) if isinstance(body, dict) else body


def check(name, ok, detail):
    results.append(bool(ok))
    print(f"{'PASS' if ok else 'FAIL'}  {name}  -> {detail}")
    return ok


def login(p):
    email, password = env.persona(p)
    status, body = call("POST", "/api/v1/auth/login", body={"email": email, "password": password})
    if status != 200:
        raise SystemExit(f"{p} persona login failed: HTTP {status}")
    d = data(body)
    sess = d.get("session", {})
    company = env.optional("E2E_COMPANY_ID", "c_001")
    if sess.get("access_token") and (d.get("current_context") or {}).get("company_id") == company:
        return sess["access_token"], d["current_context"]["membership_id"]
    if sess.get("pre_company_token"):
        status, sel = call("POST", "/api/v1/auth/select-company", sess["pre_company_token"], {"company_id": company})
    else:
        status, sel = call("POST", "/api/v1/auth/switch-company", sess.get("access_token"), {"company_id": company})
    if status != 200:
        raise SystemExit(f"{p} persona could not enter {company}: HTTP {status}")
    sel = data(sel)
    return sel["access_token"], (sel.get("current_context") or {}).get("membership_id", "")


def pick_type(token, type_id):
    if type_id:
        return type_id
    status, body = call("GET", "/api/v1/disclosure-types?page_size=100", token)
    items = data(body)
    items = items.get("items", []) if isinstance(items, dict) else items
    for it in items or []:
        s, w = call("GET", f"/api/v1/disclosure-types/{it['type_id']}/effective-workflow", token)
        if s != 200 or not data(w).get("workflow_valid", True):
            continue
        steps = data(w).get("workflow") or []
        required = [d for st in steps for d in (st.get("documents") or []) if d.get("required")]
        if 2 <= len(steps) <= 6 and not required:
            return it["type_id"]
    raise SystemExit("no visible type with a 2..6-step workflow and no required document; pass --type-id")


def sql_one(statement):
    rows = env.run_sql(statement)
    return rows[0].split("\t") if rows else None


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--type-id", default="")
    ap.add_argument("--action", default="review", choices=COMPLETING)
    args = ap.parse_args()

    for path in ("/healthz", "/readyz"):
        s, _ = call("GET", path)
        check(f"GET {path}", s == 200, f"HTTP {s}")

    token, membership = login("ENT")
    type_id = pick_type(token, args.type_id)
    print(f"type_id={type_id}")

    title = f"[QA smoke MR-1] template step skip {date.today().isoformat()} {uuid.uuid4().hex[:6]}"
    s, body = call("POST", "/api/v1/disclosures", token, {
        "type_id": type_id, "title": title, "content": "QA smoke, safe to ignore.",
        "planned_date": date.today().isoformat(),
    })
    record_id = (data(body) or {}).get("record_id") or (data(body) or {}).get("id")
    if not check("create record", s in (200, 201) and record_id, f"HTTP {s}"):
        return
    if not SAFE_ID.match(record_id):
        raise SystemExit("unexpected record id format")
    print(f"record_id={record_id}")

    s, body = call("POST", f"/api/v1/disclosures/{record_id}/submit", token, {},
                   headers={"Idempotency-Key": str(uuid.uuid4())})
    if not check("submit record", s == 200, f"HTTP {s} {(body.get('error') or {}).get('code', '')}"):
        return
    row = sql_one(f"SELECT workflow_instance_id, workflow_source, status FROM workflow_instances WHERE record_id = '{record_id}'")
    if not check("template instance created", row and row[1] not in ("proposal_snapshot_v2", "proposal_snapshot_v3"),
                 f"source={row and row[1]} status={row and row[2]}"):
        return
    instance_id = row[0]

    s, steps_body = call("GET", f"/api/v1/company/deadlines/{record_id}/steps", token)
    step_codes = [st["step_code"] for st in sorted(data(steps_body).get("steps") or [], key=lambda x: x.get("order", 0))]
    check("deadline screen lists the steps", s == 200 and len(step_codes) >= 2, f"HTTP {s} steps={step_codes}")

    def task_and_actions():
        s, b = call("GET", f"/api/v1/workflows/instances/{instance_id}/tasks", token)
        tasks = data(b)
        tasks = tasks.get("items", tasks) if isinstance(tasks, dict) else tasks
        pending = [t for t in tasks or [] if t.get("status") == "pending"]
        return s, pending

    s, pending = task_and_actions()
    task = pending[0] if len(pending) == 1 else {}
    assignees = {task.get("assignee_membership_id")} | set(task.get("assignee_membership_ids") or [])
    check("one pending step-1 task, assigned to the submitter",
          s == 200 and task.get("step_code") == step_codes[0] and membership in assignees,
          f"HTTP {s} pending={len(pending)} step={task.get('step_code')} submitter_is_assignee={membership in assignees}")
    actions = task.get("available_actions") or []
    check("available_actions hides completing actions while steps are open",
          not any(a in actions for a in COMPLETING), f"available_actions={actions}")

    def act():
        return call("POST", f"/api/v1/workflows/tasks/{task['task_id']}/{args.action}", token, {})

    def record_status():
        s, b = call("GET", f"/api/v1/disclosures/{record_id}", token)
        d = data(b) or {}
        return d.get("status") or (d.get("record") or {}).get("status")

    s, body = act()
    err = body.get("error") or {}
    missing = (err.get("details") or {}).get("missing_step_codes")
    check(f"{args.action} with every step open -> 409 WORKFLOW_STEPS_INCOMPLETE",
          s == 409 and err.get("code") == "WORKFLOW_STEPS_INCOMPLETE" and missing == step_codes,
          f"HTTP {s} {err.get('code')} missing={missing}")
    inst = sql_one(f"SELECT status FROM workflow_instances WHERE workflow_instance_id = '{instance_id}'")
    check("instance still in_progress, record not approved",
          inst and inst[0] == "in_progress" and record_status() != "approved",
          f"instance={inst and inst[0]} record={record_status()}")

    for i, code in enumerate(step_codes):
        s, body = call("POST", f"/api/v1/company/deadlines/{record_id}/steps/{code}/complete", token, {})
        check(f"complete step {code} on the deadline screen", s == 200,
              f"HTTP {s} {(body.get('error') or {}).get('code', '')}")
        if i == 0 and len(step_codes) > 1:
            s, body = act()
            err = body.get("error") or {}
            missing = (err.get("details") or {}).get("missing_step_codes")
            check(f"{args.action} with step 1 done -> 409 listing the remaining steps",
                  s == 409 and missing == step_codes[1:], f"HTTP {s} {err.get('code')} missing={missing}")

    s, pending = task_and_actions()
    actions = (pending[0] if pending else {}).get("available_actions") or []
    check(f"{args.action} offered once every step is done", args.action in actions, f"available_actions={actions}")
    s, body = act()
    check(f"{args.action} after every step is done", s == 200, f"HTTP {s} {(body.get('error') or {}).get('code', '')}")
    inst = sql_one(f"SELECT status FROM workflow_instances WHERE workflow_instance_id = '{instance_id}'")
    rec = record_status()
    if args.action == "approve":
        check("instance approved and record approved", inst and inst[0] == "approved" and rec == "approved",
              f"instance={inst and inst[0]} record={rec}")
    else:
        check("instance completed (record left as is, WF-07)", inst and inst[0] == "approved",
              f"instance={inst and inst[0]} record={rec}")


if __name__ == "__main__":
    main()
    print(f"\n{sum(results)}/{len(results)} checks passed")
    sys.exit(0 if results and all(results) else 1)
