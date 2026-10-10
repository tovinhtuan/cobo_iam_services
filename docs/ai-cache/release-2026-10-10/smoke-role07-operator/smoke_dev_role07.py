"""DEV smoke for ROLE-07 / RP-05 (+ invite-scope SQL fix): platform.cms.view alone does not make a
platform operator.

Runs in the QA company (E2E_QA_COMPANY_ID) with QA data only, all reverted:
- ENT creates a department headed by MEMBER (so MEMBER has a department invite scope);
- MEMBER gets direct grants platform.cms.view + admin.membership.invite (qa_rw) -> the old
  "weak operator" shape.
Expectations on the fixed binary:
- the invite scope of a department head loads (before: 500, "Unknown column 'name'");
- the weak operator is a tenant inviter: role_code dept_lead is refused by the enterprise deny
  list (400 "... not dept_lead system role"), and the picker shows no deny-listed role.
Cleanup revokes the grants, deletes the department and checks MEMBER's permissions. No user is created.
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
GRANTS = ("platform.cms.view", "admin.membership.invite")
DENYLIST = {"dept_lead", "admin_web", "cms_operator", "full_access", "truong_phong_ban", "truong_nhom", "self_reg_company_owner"}
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


def err_of(raw):
    try:
        e = json.loads(raw).get("error") or {}
        return e.get("code", ""), e.get("message", "")
    except Exception:
        return "", ""


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


def perms(token):
    st, raw = call("GET", "/api/v1/me/effective-access", token)
    return (bodyj(raw).get("permissions") or []) if st == 200 else []


def report():
    print()
    for ok, name, detail in results:
        print(f"{'PASS' if ok else 'FAIL'}  {name}  {detail}")
    print(f"\n{sum(r[0] for r in results)}/{len(results)} passed")


def main():
    ent, _ = login("ENT", QA)
    mem, m_mem = login("MEMBER", QA)
    if not (m_mem or "").startswith("m_qa_"):
        raise SystemExit(f"refusing to run: MEMBER is not a QA membership: {m_mem}")
    normal = sorted(perms(mem))
    dept_id = None
    try:
        st, raw = call("POST", f"{A}/departments", ent, {"name": f"QA smoke ROLE-07 {int(time.time())}", "head_membership_id": m_mem})
        dept_id = bodyj(raw).get("department_id") if st in (200, 201) else None
        record(bool(dept_id), "fixture: department headed by MEMBER created", f"{st} {err_of(raw)}")
        for code in GRANTS:
            run_sql("INSERT INTO membership_direct_permissions (membership_id, company_id, permission_code, granted_by) "
                    f"VALUES ({q(m_mem)}, {q(QA)}, {q(code)}, 'qa_smoke_role07') "
                    "ON DUPLICATE KEY UPDATE revoked_at=NULL, revoked_by=NULL, granted_by='qa_smoke_role07'", write=True)
        call("PATCH", f"{A}/memberships/{m_mem}", ent, {"status": "active"})  # drops the cached access (H17)
        now = perms(mem)
        record(all(c in now for c in GRANTS), "fixture: MEMBER holds platform.cms.view + invite", [c for c in GRANTS if c in now])

        st, raw = call("GET", f"{A}/invite-roles", mem)
        items = bodyj(raw)
        items = items.get("items", items) if isinstance(items, dict) else items
        codes = sorted({i.get("role_code") for i in (items or [])})
        record(st == 200 and not (set(codes) & DENYLIST), "ROLE-07 weak operator's picker shows no deny-listed role", f"{st} {codes}")

        email = f"qa.smoke.role07.{int(time.time())}@cobo.test"
        st, raw = call("POST", f"{A}/users/invite", mem, {"email": email, "full_name": "QA smoke ROLE-07", "role_code": "dept_lead"})
        code, msg = err_of(raw)
        record(st != 500, "invite scope of a department head loads (no 500)", f"{st} {code}")
        record(st == 400 and code == "INVALID_REQUEST" and "dept_lead" in msg,
               "ROLE-07 weak operator goes through the tenant deny list", f"{st} {code} {msg[:90]}")
        created = run_sql(f"SELECT COUNT(*) FROM users WHERE login_id={q(email)}")
        record(created == ["0"], "no user was created by the refused invite", created)
    finally:
        run_sql("UPDATE membership_direct_permissions SET revoked_at=CURRENT_TIMESTAMP, revoked_by='qa_smoke_role07' "
                f"WHERE membership_id={q(m_mem)} AND revoked_at IS NULL AND permission_code IN ({','.join(q(c) for c in GRANTS)})", write=True)
        if dept_id:
            st, _ = call("DELETE", f"{A}/departments/{dept_id}", ent)
            record(st in (200, 204), "cleanup: department deleted", st)
        call("PATCH", f"{A}/memberships/{m_mem}", ent, {"status": "active"})
        left = run_sql(f"SELECT COUNT(*) FROM membership_direct_permissions WHERE membership_id={q(m_mem)} AND revoked_at IS NULL "
                       f"AND permission_code IN ({','.join(q(c) for c in GRANTS)})")
        record(left == ["0"], "cleanup: fixture grants revoked", left)
        heads = run_sql(f"SELECT COUNT(*) FROM departments WHERE company_id={q(QA)} AND head_membership_id={q(m_mem)} AND status='active'")
        record(heads == ["0"], "cleanup: MEMBER heads no active department", heads)
        mem2, _ = login("MEMBER", QA)
        record(sorted(perms(mem2)) == normal, "cleanup: MEMBER permissions back to normal", len(perms(mem2)))


if __name__ == "__main__":
    try:
        main()
    finally:
        report()
    sys.exit(0 if results and all(r[0] for r in results) else 1)
