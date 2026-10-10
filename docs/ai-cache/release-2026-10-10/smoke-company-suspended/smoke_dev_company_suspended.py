"""DEV smoke: a suspended company ("Tạm ngưng", e.g. unpaid) keeps read and export access only.

QA data only, everything created here is purged at the end (root SQL guarded by company name and
login prefix). No email is sent (password account without an email address).
Fixture (with a temporary admin.membership.invite grant for the CMS persona, revoked after):
company S created through the platform API, user W its admin.
- the platform cannot suspend a company that hosts a platform operator (409);
- after suspending S: W's reads work, company writes are 403 COMPANY_SUSPENDED, the user's own
  account (profile) and export / validate POSTs are not blocked, refresh and sign-in still work,
  /me/companies reports company_status = suspended;
- after reactivating S: writes work again.
"""
import json
import secrets
import sys
import time
import urllib.error
import urllib.request
from pathlib import Path

DEVQA = Path(__file__).resolve().parents[4] / "scripts" / "devqa"
sys.path.insert(0, str(DEVQA))
from devqa_env import base_url, optional, persona, run_sql  # noqa: E402
from provision_qa_accounts import _run_root  # noqa: E402
from provision_qa_company import drop_access_cache  # noqa: E402

BASE = base_url()
OWN = optional("E2E_COMPANY_ID", "c_001")
P = "/api/v1/platform/cms/admin"
A = "/api/v1/admin"
TAG = "qa_smoke_company_suspended"
STAMP = time.strftime("%Y%m%d%H%M%S")
NAME = f"QA Company Suspended {STAMP}"
LOGIN_PREFIX = "qa.smoke.csusp."
SUSPENDED = "COMPANY_SUSPENDED"
results = []
created = {"companies": [], "users": []}


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


def q(v):
    return "'" + str(v).replace("'", "''") + "'"


def sign_in(email, password):
    st, raw = call("POST", "/api/v1/auth/login", body={"email": email, "password": password})
    if st != 200:
        return st, code_of(raw), None, None, None, None
    body = bodyj(raw)
    sess, ctx = body.get("session", {}), body.get("current_context") or {}
    return st, "", sess.get("access_token"), ctx.get("membership_id"), sess.get("refresh_token"), ctx.get("company_id")


def set_invite_grant(on):
    if on:
        run_sql("INSERT INTO membership_direct_permissions (membership_id, company_id, permission_code, granted_by) "
                f"VALUES ('m_qa_cms_c001', {q(OWN)}, 'admin.membership.invite', {q(TAG)}) "
                f"ON DUPLICATE KEY UPDATE revoked_at=NULL, revoked_by=NULL, granted_by={q(TAG)}", write=True)
    else:
        run_sql(f"UPDATE membership_direct_permissions SET revoked_at=CURRENT_TIMESTAMP, revoked_by={q(TAG)} "
                f"WHERE membership_id='m_qa_cms_c001' AND granted_by={q(TAG)} AND revoked_at IS NULL", write=True)
    drop_access_cache(OWN)


def purge():
    companies = [c for c in created["companies"] if c]
    users = [u for u in created["users"] if u]
    for cid in companies:
        name = run_sql(f"SELECT company_name FROM companies WHERE company_id={q(cid)}")
        if not name or not name[0].startswith(NAME):
            record(False, "purge: company guard", f"refusing to delete {cid}: {name}")
            return
    if users:
        logins = run_sql("SELECT login_id FROM users WHERE user_id IN (" + ",".join(q(u) for u in users) + ")")
        if not all(l.startswith(LOGIN_PREFIX) for l in logins):
            record(False, "purge: user guard", f"refusing to delete users {logins}")
            return
    col_tables = lambda col, skip: _run_root(  # noqa: E731
        "SELECT c.table_name FROM information_schema.columns c JOIN information_schema.tables t "
        "ON t.table_schema=c.table_schema AND t.table_name=c.table_name AND t.table_type='BASE TABLE' "
        f"WHERE c.table_schema='cobo_iam' AND c.column_name='{col}' AND c.table_name<>'{skip}';").split()
    sql = ["SET FOREIGN_KEY_CHECKS=0;", "START TRANSACTION;"]
    for cid in companies:
        c = q(cid)
        sql += [
            f"DELETE mr FROM membership_roles mr JOIN memberships m ON m.membership_id=mr.membership_id WHERE m.company_id={c};",
            f"DELETE dm FROM department_memberships dm JOIN memberships m ON m.membership_id=dm.membership_id WHERE m.company_id={c};",
            f"DELETE mt FROM membership_titles mt JOIN memberships m ON m.membership_id=mt.membership_id WHERE m.company_id={c};",
            f"DELETE rp FROM role_permissions rp JOIN roles r ON r.role_id=rp.role_id WHERE r.company_id={c};",
        ]
        sql += [f"DELETE FROM `{t}` WHERE company_id={c};" for t in col_tables("company_id", "companies")]
        sql += [f"DELETE FROM companies WHERE company_id={c};"]
    if users:
        ids = ",".join(q(u) for u in users)
        sql += [f"DELETE FROM `{t}` WHERE user_id IN ({ids});" for t in col_tables("user_id", "users")]
        sql += [f"DELETE FROM users WHERE user_id IN ({ids});"]
    sql += ["COMMIT;", "SET FOREIGN_KEY_CHECKS=1;"]
    _run_root("\n".join(sql))
    left_c = run_sql("SELECT COUNT(*) FROM companies WHERE company_id IN (" + ",".join(q(c) for c in companies) + ")") if companies else ["0"]
    left_u = run_sql("SELECT COUNT(*) FROM users WHERE user_id IN (" + ",".join(q(u) for u in users) + ")") if users else ["0"]
    record(left_c == ["0"] and left_u == ["0"], "purge: companies and users removed", f"companies={left_c} users={left_u}")


def report():
    print()
    for ok, name, detail in results:
        print(f"{'PASS' if ok else 'FAIL'}  {name}  {detail}")
    print(f"\n{sum(r[0] for r in results)}/{len(results)} passed")


def create_department(token, label):
    st, raw = call("POST", f"{A}/departments", token, {"name": f"QA suspended {label} {STAMP}"})
    return st, code_of(raw), bodyj(raw).get("department_id") if st in (200, 201) else None


def main():
    st, raw = call("POST", "/api/v1/auth/login", body=dict(zip(("email", "password"), persona("CMS"))))
    cms = bodyj(raw).get("session", {}).get("access_token")
    if not cms:
        raise SystemExit(f"CMS login failed: {st}")
    try:
        set_invite_grant(True)
        st, raw = call("POST", f"{P}/companies", cms, {"company_name": f"{NAME} (smoke only)"})
        cs = bodyj(raw).get("company_id") if st in (200, 201) else None
        created["companies"].append(cs)
        login_id, pw = f"{LOGIN_PREFIX}w.{STAMP}@cobo.test", secrets.token_urlsafe(18) + "Aa1!"
        st, raw = call("POST", f"{P}/users", cms, {"login_id": login_id, "password": pw, "full_name": "QA Company Suspended W"})
        uid = bodyj(raw).get("user_id") if st in (200, 201) else None
        created["users"].append(uid)
        st, raw = call("POST", f"{P}/users/{uid}/assign-company", cms, {"company_id": cs, "role_code": "admin_doanh_nghiep"})
        w = sign_in(login_id, pw)
        record(cs and uid and w[2] and w[5] == cs, "fixture: company S and its admin W signed in", f"{w[:2]}")
        tok, mid, refresh = w[2], w[3], w[4]

        # lock-out guard also covers suspension
        run_sql("INSERT INTO membership_direct_permissions (membership_id, company_id, permission_code, granted_by) "
                f"VALUES ({q(mid)}, {q(cs)}, 'platform.cms.view', {q(TAG)}) "
                f"ON DUPLICATE KEY UPDATE revoked_at=NULL, revoked_by=NULL, granted_by={q(TAG)}", write=True)
        st, raw = call("POST", f"{P}/companies/{cs}/suspend", cms, {"reason": "qa smoke"})
        record(st == 409 and run_sql(f"SELECT status FROM companies WHERE company_id={q(cs)}") == ["active"],
               "a company hosting a platform operator cannot be suspended", f"{st} {code_of(raw)}")
        run_sql(f"UPDATE membership_direct_permissions SET revoked_at=CURRENT_TIMESTAMP, revoked_by={q(TAG)} "
                f"WHERE membership_id={q(mid)} AND granted_by={q(TAG)} AND revoked_at IS NULL", write=True)
        drop_access_cache(cs)
        if st not in (409, 404, 405):  # an old binary may have suspended it: put it back
            call("POST", f"{P}/companies/{cs}/activate", cms, {"reason": "qa smoke"})

        st, err, dept = create_department(tok, "before")
        record(st in (200, 201), "before: W can create a department", f"{st} {err}")
        if dept:
            call("DELETE", f"{A}/departments/{dept}", tok)

        st, raw = call("POST", f"{P}/companies/{cs}/suspend", cms, {"reason": "qa smoke"})
        status = run_sql(f"SELECT status FROM companies WHERE company_id={q(cs)}")
        record(st == 200 and status == ["suspended"], "suspend S", f"{st} {code_of(raw)} status={status}")

        st, raw = call("GET", f"{A}/memberships/{mid}/permissions", tok)
        record(st == 200, "suspended: W can still read", f"{st} {code_of(raw)}")
        st, err, dept = create_department(tok, "during")
        record(st == 403 and err.startswith(SUSPENDED), "suspended: a company write is refused", f"{st} {err}")
        if dept:
            call("DELETE", f"{A}/departments/{dept}", tok)
        st, raw = call("PATCH", f"{A}/memberships/{mid}", tok, {"status": "active"})
        record(st == 403 and code_of(raw).startswith(SUSPENDED), "suspended: a membership update is refused", f"{st} {code_of(raw)}")
        st, raw = call("POST", f"{A}/config-export", tok, {"modules": ["rbac"]})
        record(not code_of(raw).startswith(SUSPENDED), "suspended: export is not blocked", f"{st} {code_of(raw)}")
        st, raw = call("PATCH", "/api/v1/me/profile", tok, {})
        record(not code_of(raw).startswith(SUSPENDED), "suspended: the user's own profile is not blocked", f"{st} {code_of(raw)}")
        st, raw = call("POST", "/api/v1/auth/refresh", body={"refresh_token": refresh})
        record(st == 200, "suspended: refresh works", f"{st} {code_of(raw)}")
        r = sign_in(login_id, pw)
        record(r[2] and r[5] == cs, "suspended: W can sign in to S", r[:2])
        if r[2]:
            st, raw = call("GET", "/api/v1/me/companies", r[2])
            items = {i.get("company_id"): i.get("company_status") for i in (bodyj(raw).get("items") or [])}
            record(items.get(cs) == "suspended", "suspended: /me/companies reports company_status", items.get(cs))

        st, raw = call("POST", f"{P}/companies/{cs}/activate", cms, {"reason": "qa smoke"})
        record(st == 200, "reactivate S", f"{st} {code_of(raw)}")
        st, err, dept = create_department(tok, "after")
        record(st in (200, 201), "after reactivation: W can write again", f"{st} {err}")
        if dept:
            call("DELETE", f"{A}/departments/{dept}", tok)
    finally:
        set_invite_grant(False)
        left = run_sql(f"SELECT COUNT(*) FROM membership_direct_permissions WHERE membership_id='m_qa_cms_c001' "
                       f"AND granted_by={q(TAG)} AND revoked_at IS NULL")
        record(left == ["0"], "cleanup: temporary grant revoked", left)
        purge()


if __name__ == "__main__":
    try:
        main()
    finally:
        report()
    sys.exit(0 if results and all(r[0] for r in results) else 1)
