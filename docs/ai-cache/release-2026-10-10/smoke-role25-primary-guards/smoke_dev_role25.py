"""DEV smoke for ROLE-25: primary-admin invariants hold in the write itself (MySQL).

QA company only (E2E_QA_COMPANY_ID): ENT owner, ENT2 admin, MEMBER user. All changes are reverted.
1. Remove-role SQL (new guarded DELETE ... JOIN) still removes an ordinary role; re-added after.
2. Primary admin guards: ENT2 cannot remove ENT's admin role, deactivate or delete ENT (409).
3. Race: ENT transfers ownership to ENT2 while (concurrently) deactivating ENT2, 10 rounds.
   Allowed: one of the two wins. Forbidden: both win (ENT2 primary but inactive). After every
   round the DB must have exactly one primary admin, and it must be active.
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
from devqa_env import base_url, persona, require, run_sql  # noqa: E402

BASE = base_url()
QA = require("E2E_QA_COMPANY_ID")
A = "/api/v1/admin"
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
    results.append((bool(ok), name, str(detail)[:220]))


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
    return run_sql(f"SELECT membership_id, membership_status, is_primary_admin FROM memberships WHERE company_id={q(QA)} ORDER BY membership_id")


def primaries_active():
    return run_sql(f"SELECT membership_id, membership_status FROM memberships WHERE company_id={q(QA)} AND is_primary_admin=1")


def role_of(mid, code):
    rows = run_sql(f"SELECT r.role_id FROM membership_roles mr JOIN roles r ON r.role_id=mr.role_id WHERE mr.membership_id={q(mid)} "
                   f"AND mr.status='active' AND r.role_code={q(code)}")
    return rows[0] if rows else None


def report():
    print()
    for ok, name, detail in results:
        print(f"{'PASS' if ok else 'FAIL'}  {name}  {detail}")
    print(f"\n{sum(r[0] for r in results)}/{len(results)} passed")


def restore(ent, m_ent, m_ent2):
    """ENT owner, everyone active; falls back to the fixture script when the API path cannot."""
    try:
        p = primaries_active()
        if p and p[0].split("\t")[0] == m_ent2:
            tok2, _ = login("ENT2", QA)
            call("POST", f"{A}/company/transfer-ownership", tok2, {"target_membership_id": m_ent})
        for row in state():
            mid, status, _ = row.split("\t")
            if status != "active":
                call("PATCH", f"{A}/memberships/{mid}", ent, {"status": "active"})
    except SystemExit:
        pass
    if primaries_active() != [f"{m_ent}\tactive"] or any(r.split("\t")[1] != "active" for r in state()):
        subprocess.run([sys.executable, "-I", str(DEVQA / "provision_qa_company.py")], capture_output=True, timeout=300)


def main():
    ent, m_ent = login("ENT", QA)
    ent2, m_ent2 = login("ENT2", QA)
    _, m_mem = login("MEMBER", QA)
    before = state()
    record(primaries_active() == [f"{m_ent}\tactive"], "fixture: ENT is the only (active) primary admin", primaries_active())
    try:
        # 1. guarded DELETE still removes an ordinary role
        r_user = role_of(m_mem, "user_thuong")
        st, raw = call("DELETE", f"{A}/memberships/{m_mem}/roles/{r_user}", ent)
        record(st == 200 and role_of(m_mem, "user_thuong") is None, "remove an ordinary role works (guarded DELETE on MySQL)", f"{st} {code_of(raw)}")
        st, raw = call("POST", f"{A}/memberships/{m_mem}/roles", ent, {"role_id": r_user})
        record(st in (200, 201) and role_of(m_mem, "user_thuong") == r_user, "restore: role re-assigned", f"{st} {code_of(raw)}")

        # 2. primary-admin guards
        r_admin = role_of(m_ent, "admin_doanh_nghiep")
        st, raw = call("DELETE", f"{A}/memberships/{m_ent}/roles/{r_admin}", ent2)
        record(st == 409 and "PRIMARY_ADMIN" in code_of(raw), "ENT2 cannot remove the owner's admin role (409, not 403)", f"{st} {code_of(raw)}")
        st, raw = call("PATCH", f"{A}/memberships/{m_ent}", ent2, {"status": "inactive"})
        record(st == 409, "ENT2 cannot deactivate the owner", f"{st} {code_of(raw)}")
        st, raw = call("DELETE", f"{A}/memberships/{m_ent}", ent2)
        record(st == 409, "ENT2 cannot delete the owner", f"{st} {code_of(raw)}")
        record(state() == before, "DB unchanged after refused guards", "")

        # 2b. regression (PR-B ROLE-13 guard): the owner can demote another admin via primary-role
        st, raw = call("PUT", f"{A}/memberships/{m_ent2}/primary-role", ent, {"role_id": r_user})
        record(st == 200 and role_of(m_ent2, "user_thuong") == r_user and role_of(m_ent2, "admin_doanh_nghiep") is None,
               "owner demotes ENT2 to user_thuong via primary-role", f"{st} {code_of(raw)}")
        r = subprocess.run([sys.executable, "-I", str(DEVQA / "provision_qa_company.py")], capture_output=True, timeout=300)
        record(r.returncode == 0 and role_of(m_ent2, "admin_doanh_nghiep") is not None and role_of(m_ent2, "user_thuong") is None,
               "restore: ENT2 is admin again (fixture script)", r.returncode)

        # 3. race: transfer to ENT2 vs deactivate ENT2, concurrently
        both_won, errors_5xx, rounds = 0, 0, 10
        outcomes = []
        for _ in range(rounds):
            res = {}
            barrier = threading.Barrier(2)

            def transfer():
                barrier.wait()
                res["transfer"] = call("POST", f"{A}/company/transfer-ownership", ent, {"target_membership_id": m_ent2})

            def deactivate():
                barrier.wait()
                res["deactivate"] = call("PATCH", f"{A}/memberships/{m_ent2}", ent, {"status": "inactive"})

            ts = [threading.Thread(target=transfer), threading.Thread(target=deactivate)]
            for t in ts:
                t.start()
            for t in ts:
                t.join()
            tr, de = res["transfer"][0], res["deactivate"][0]
            outcomes.append((tr, de))
            both_won += tr == 200 and de == 200
            errors_5xx += tr >= 500 or de >= 500
            p = primaries_active()
            if len(p) != 1 or not p[0].endswith("\tactive"):
                record(False, "invariant after a race round: one active primary admin", p)
                break
            restore(ent, m_ent, m_ent2)
        record(both_won == 0, "race: never both transfer and deactivate succeed", f"outcomes={outcomes}")
        record(errors_5xx == 0, "race: no 5xx (no deadlock surfaced)", errors_5xx)
    finally:
        restore(ent, m_ent, m_ent2)
        if primaries_active() != [f"{m_ent}\tactive"]:
            subprocess.run([sys.executable, "-I", str(DEVQA / "provision_qa_company.py")], capture_output=True, timeout=300)
        record(state() == before, "cleanup: QA company back to its starting state", state())


if __name__ == "__main__":
    try:
        main()
    finally:
        report()
    sys.exit(0 if results and all(r[0] for r in results) else 1)
