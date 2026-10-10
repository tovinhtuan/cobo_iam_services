"""DEV smoke for ROLE-10 / RP-11: adding a team member checks the team's own department.

QA company only (E2E_QA_COMPANY_ID), ENT as owner. Fixture (all removed at the end):
departments X and Y, team T in Y, MEMBER in X.
- body department_id = X (the member's department, not the team's) -> 400 (old binary: added);
- no department_id -> member not in the team's department -> 409;
- MEMBER added to Y, body department_id = Y -> 200 (positive).
"""
import json
import sys
import time
import urllib.error
import urllib.request
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[4] / "scripts" / "devqa"))
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
    results.append((bool(ok), name, str(detail)[:200]))


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


def in_team(team, mid):
    return run_sql(f"SELECT COUNT(*) FROM org_unit_memberships WHERE org_unit_id={q(team)} AND membership_id={q(mid)} AND status='active'")


def report():
    print()
    for ok, name, detail in results:
        print(f"{'PASS' if ok else 'FAIL'}  {name}  {detail}")
    print(f"\n{sum(r[0] for r in results)}/{len(results)} passed")


def main():
    ent, _ = login("ENT", QA)
    _, m_mem = login("MEMBER", QA)
    stamp = int(time.time())
    depts, team = {}, None
    try:
        for k in ("X", "Y"):
            st, raw = call("POST", f"{A}/departments", ent, {"name": f"QA smoke ROLE-10 {k} {stamp}"})
            depts[k] = bodyj(raw).get("department_id") if st in (200, 201) else None
        record(all(depts.values()), "fixture: departments X and Y created", depts)
        st, raw = call("POST", f"{A}/departments/{depts['Y']}/teams", ent, {"name": f"QA smoke ROLE-10 team {stamp}"})
        team = bodyj(raw).get("team_id") if st in (200, 201) else None
        record(bool(team), "fixture: team in Y created", f"{st} {code_of(raw)}")
        st, raw = call("POST", f"{A}/departments/{depts['X']}/members", ent, {"membership_id": m_mem})
        record(st == 200, "fixture: MEMBER in X", f"{st} {code_of(raw)}")

        st, raw = call("POST", f"{A}/teams/{team}/members", ent, {"membership_id": m_mem, "department_id": depts["X"]})
        record(st == 400 and in_team(team, m_mem) == ["0"], "ROLE-10 body department X cannot add MEMBER to Y's team", f"{st} {code_of(raw)}")
        st, raw = call("POST", f"{A}/teams/{team}/members", ent, {"membership_id": m_mem})
        record(st == 409 and in_team(team, m_mem) == ["0"], "ROLE-10 MEMBER not in the team's department -> 409", f"{st} {code_of(raw)}")

        st, raw = call("POST", f"{A}/departments/{depts['Y']}/members", ent, {"membership_id": m_mem})
        st, raw = call("POST", f"{A}/teams/{team}/members", ent, {"membership_id": m_mem, "department_id": depts["Y"]})
        record(st == 200 and in_team(team, m_mem) == ["1"], "member of the team's department is added", f"{st} {code_of(raw)}")
    finally:
        if team:
            call("DELETE", f"{A}/teams/{team}/members/{m_mem}", ent)
            st, raw = call("DELETE", f"{A}/teams/{team}", ent)
            record(st in (200, 204), "cleanup: team deleted", f"{st} {code_of(raw)}")
        for k, d in depts.items():
            if d:
                call("DELETE", f"{A}/departments/{d}/members/{m_mem}", ent)
                st, raw = call("DELETE", f"{A}/departments/{d}", ent)
                record(st in (200, 204), f"cleanup: department {k} deleted", f"{st} {code_of(raw)}")
        left = run_sql(f"SELECT COUNT(*) FROM department_memberships dm JOIN departments d ON d.department_id=dm.department_id "
                       f"WHERE dm.membership_id={q(m_mem)} AND dm.status='active' AND d.status='active' AND d.company_id={q(QA)}")
        record(left == ["0"], "cleanup: MEMBER in no active QA department", left)


if __name__ == "__main__":
    try:
        main()
    finally:
        report()
    sys.exit(0 if results and all(r[0] for r in results) else 1)
