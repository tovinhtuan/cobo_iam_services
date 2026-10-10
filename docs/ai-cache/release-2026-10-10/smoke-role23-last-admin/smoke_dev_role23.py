"""DEV smoke for ROLE-23 (BES-22): a company keeps at least one active admin, also under concurrency.

QA company only (E2E_QA_COMPANY_ID). Fixture: `provision_qa_company.py --no-primary` leaves the QA
company without a primary admin (ENT and ENT2 are plain admins, MEMBER is user_thuong), the case
where nothing else keeps an admin. Plain `provision_qa_company.py` restores ENT as owner.
- MEMBER cannot deactivate an admin (403); a membership of another company is 403/404.
- ENT may deactivate ENT2 while ENT stays admin (200).
- ENT as the only admin cannot remove-admin-role / deactivate / delete itself (409; old binary:
  deactivate and delete succeeded and left the company with no admin). revoke-company-admin must not
  remove it either (that route answers 500 on tenant companies: a separate, pre-existing bug).
- Concurrent mutual deactivate and mutual admin-role removal (ENT <-> ENT2), 5 rounds each: exactly
  one change wins, one active admin stays, no 5xx (old binary: both deactivations won).
"""
import json
import subprocess
import sys
import threading
import urllib.error
import urllib.request
from pathlib import Path

DEVQA = Path(__file__).resolve().parents[4] / "scripts" / "devqa"
sys.path.insert(0, str(DEVQA))
from devqa_env import base_url, optional, persona, require, run_sql  # noqa: E402

BASE = base_url()
QA = require("E2E_QA_COMPANY_ID")
OTHER = optional("E2E_OTHER_COMPANY_ID", "c_002")
A = "/api/v1/admin"
LAST = "last_admin_role_change_blocked"
ROUNDS = 5
results = []


def call(method, path, token=None, body=None):
    req = urllib.request.Request(BASE + path, method=method, data=json.dumps(body).encode() if body is not None else None,
                                 headers={"Content-Type": "application/json"})
    if token:
        req.add_header("Authorization", "Bearer " + token)
    try:
        with urllib.request.urlopen(req, timeout=25) as r:
            return r.status, r.read()
    except urllib.error.HTTPError as e:
        return e.code, e.read()


def bodyj(raw):
    try:
        d = json.loads(raw)
    except Exception:
        return {}
    return d["data"] if isinstance(d, dict) and d.get("data") is not None else d


def code_of(raw):
    try:
        e = json.loads(raw).get("error") or {}
        return f"{e.get('code', '')} {e.get('message', '')}".strip()
    except Exception:
        return ""


def record(ok, name, detail=""):
    results.append((bool(ok), name, str(detail)[:240]))


def login(p, company):
    email, password = persona(p)
    st, raw = call("POST", "/api/v1/auth/login", body={"email": email, "password": password})
    if st != 200:
        raise SystemExit(f"login failed for persona {p}: HTTP {st}")
    body = bodyj(raw)
    sess, ctx = body.get("session", {}), body.get("current_context") or {}
    mids = {m["company_id"]: m["membership_id"] for m in body.get("memberships", [])}
    if sess.get("access_token") and ctx.get("company_id") == company:
        return sess["access_token"], ctx.get("membership_id")
    st, raw = call("POST", "/api/v1/auth/select-company", sess.get("pre_company_token") or sess.get("access_token"), {"company_id": company})
    if st != 200:
        raise SystemExit(f"select-company failed for persona {p}: HTTP {st}")
    return bodyj(raw)["access_token"], mids.get(company)


def q(v):
    return "'" + str(v).replace("'", "''") + "'"


def state():
    return run_sql("SELECT m.membership_id, m.membership_status, m.is_primary_admin, "
                   "IFNULL(GROUP_CONCAT(r.role_code ORDER BY r.role_code), '') FROM memberships m "
                   "LEFT JOIN membership_roles mr ON mr.membership_id=m.membership_id AND mr.status='active' "
                   f"LEFT JOIN roles r ON r.role_id=mr.role_id WHERE m.company_id={q(QA)} "
                   "GROUP BY m.membership_id ORDER BY m.membership_id")


def active_admins():
    return [r.split("\t")[0] for r in state() if r.split("\t")[1] == "active" and "admin_doanh_nghiep" in r.split("\t")[3]]


def provision(no_primary):
    args = [sys.executable, "-I", str(DEVQA / "provision_qa_company.py")] + (["--no-primary"] if no_primary else [])
    return subprocess.run(args, capture_output=True, timeout=300).returncode


def admin_role_id():
    return run_sql(f"SELECT role_id FROM roles WHERE company_id={q(QA)} AND role_code='admin_doanh_nghiep'")[0]


def report():
    print()
    for ok, name, detail in results:
        print(f"{'PASS' if ok else 'FAIL'}  {name}  {detail}")
    print(f"\n{sum(r[0] for r in results)}/{len(results)} passed")


def race(name, change):
    """Runs ENT->ENT2 and ENT2->ENT at once; returns the two HTTP statuses and error codes."""
    ent, m_ent = login("ENT", QA)
    ent2, m_ent2 = login("ENT2", QA)
    barrier, res = threading.Barrier(2), {}

    def run(key, token, target):
        barrier.wait()
        res[key] = change(token, target)

    ts = [threading.Thread(target=run, args=("a", ent, m_ent2)), threading.Thread(target=run, args=("b", ent2, m_ent))]
    for t in ts:
        t.start()
    for t in ts:
        t.join()
    return [(res[k][0], code_of(res[k][1]).split(" ")[0]) for k in ("a", "b")]


def main():
    before = state()
    record(provision(True) == 0, "fixture: QA company without a primary admin", "")
    try:
        ent, m_ent = login("ENT", QA)
        ent2, m_ent2 = login("ENT2", QA)
        member, _ = login("MEMBER", QA)
        record(sorted(active_admins()) == sorted([m_ent, m_ent2]) and all(r.split("\t")[2] == "0" for r in state()),
               "fixture: ENT and ENT2 active admins, nobody primary", state())

        st, raw = call("PATCH", f"{A}/memberships/{m_ent2}", member, {"status": "inactive"})
        record(st == 403, "MEMBER cannot deactivate an admin", f"{st} {code_of(raw)}")
        other = run_sql(f"SELECT membership_id FROM memberships WHERE company_id={q(OTHER)} AND membership_status='active' LIMIT 1")
        if other:
            st, raw = call("PATCH", f"{A}/memberships/{other[0]}", ent, {"status": "inactive"})
            still = run_sql(f"SELECT membership_status FROM memberships WHERE membership_id={q(other[0])}")
            record(st in (403, 404) and still == ["active"], "membership of another company: 403/404, unchanged", f"{st} {code_of(raw)}")

        st, raw = call("PATCH", f"{A}/memberships/{m_ent2}", ent, {"status": "inactive"})
        record(st == 200 and active_admins() == [m_ent], "ENT deactivates ENT2 while ENT stays admin", f"{st} {code_of(raw)}")

        # Least destructive first; on the first change that goes through (missing guard), stop and
        # restore instead of stacking more damage (the fixture script also re-creates a deleted ENT).
        role = admin_role_id()
        for label, method, path, body in (
            ("remove admin role", "DELETE", f"{A}/memberships/{m_ent}/roles/{role}", None),
            ("revoke company admin", "DELETE", f"{A}/company/admins/{m_ent}", None),
            ("deactivate", "PATCH", f"{A}/memberships/{m_ent}", {"status": "inactive"}),
            ("delete", "DELETE", f"{A}/memberships/{m_ent}", None),
        ):
            st, raw = call(method, path, ent, body)
            if label == "revoke company admin":
                # Pre-existing, separate bug: this route looks up role code company_admin, which
                # tenant companies do not have (500). Asserted here: it never removes the last admin.
                ok = st != 200 and active_admins() == [m_ent]
            else:
                ok = st == 409 and code_of(raw).startswith(LAST) and active_admins() == [m_ent]
            record(ok, f"sole admin cannot leave: {label}", f"{st} {code_of(raw)}")
            if not ok:
                break
        provision(True)

        changes = {
            "deactivate": lambda tok, target: call("PATCH", f"{A}/memberships/{target}", tok, {"status": "inactive"}),
            "remove admin role": lambda tok, target: call("DELETE", f"{A}/memberships/{target}/roles/{role}", tok),
        }
        for name, change in changes.items():
            outcomes, bad = [], 0
            for _ in range(ROUNDS):
                out = race(name, change)
                outcomes.append(out)
                wins = sum(st == 200 for st, _ in out)
                # The loser gets 409 when it reaches the last-admin check, or 403 when the winner had
                # already taken its access before its authorization ran; both keep one admin.
                refused = sum((st == 409 and code == LAST) or st == 403 for st, code in out)
                bad += not (wins == 1 and refused == 1 and len(active_admins()) == 1)
                provision(True)
            record(bad == 0, f"race ({name}): one wins, the other is refused, one admin stays", f"bad={bad} outcomes={outcomes}")
            record(all(st < 500 for out in outcomes for st, _ in out), f"race ({name}): no 5xx", "")
    finally:
        rc = provision(False)
        record(rc == 0 and state() == before, "cleanup: QA company back to its starting state (ENT owner)", state())


if __name__ == "__main__":
    try:
        main()
    finally:
        report()
    sys.exit(0 if results and all(r[0] for r in results) else 1)
