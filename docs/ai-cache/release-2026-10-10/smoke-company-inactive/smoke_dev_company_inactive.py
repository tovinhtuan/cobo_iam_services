"""DEV smoke: a deactivated company ("Ngừng hoạt động") blocks all access at once; reactivating
restores it.

QA data only, everything created here is purged at the end (root SQL guarded by company name and
login prefix). No email is sent: accounts are password accounts without an email address.
Fixture (with a temporary admin.membership.invite grant for the CMS persona, revoked after):
- companies A and B created through the platform API;
- user U: admin of A and of B; user V: admin of B only.
Checks after deactivating B:
- V's and U's sessions in B -> 403 COMPANY_INACTIVE; U's session in A still works;
- V's refresh -> 403; V's new sign-in -> 403 (its only company); U's sign-in lands in A;
- U's /me/companies omits B; U switching to B -> 403;
then reactivating B: V's existing session and a new sign-in work again.
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
TAG = "qa_smoke_company_status"
STAMP = time.strftime("%Y%m%d%H%M%S")
NAME = f"QA Company Status {STAMP}"
LOGIN_PREFIX = "qa.smoke.cstatus."
INACTIVE = "COMPANY_INACTIVE"
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


def strong_password():
    return secrets.token_urlsafe(18) + "Aa1!"


def sign_in(email, password, company=None):
    """Returns (status, error code, access token, membership id, refresh token, current company)."""
    st, raw = call("POST", "/api/v1/auth/login", body={"email": email, "password": password})
    if st != 200:
        return st, code_of(raw), None, None, None, None
    body = bodyj(raw)
    sess, ctx = body.get("session", {}), body.get("current_context") or {}
    if company and ctx.get("company_id") != company:
        st, raw = call("POST", "/api/v1/auth/select-company", sess.get("pre_company_token"), {"company_id": company})
        if st != 200:
            return st, code_of(raw), None, None, None, None
        data = bodyj(raw)
        ctx = data.get("current_context") or {}
        return st, "", data.get("access_token"), ctx.get("membership_id"), sess.get("refresh_token"), ctx.get("company_id")
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


def admin_api(token, mid):
    st, raw = call("GET", f"{A}/memberships/{mid}/permissions", token)
    return st, code_of(raw)


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


def make_user(cms, label, companies):
    login_id, pw = f"{LOGIN_PREFIX}{label}.{STAMP}@cobo.test", strong_password()
    st, raw = call("POST", f"{P}/users", cms, {"login_id": login_id, "password": pw, "full_name": f"QA Company Status {label}"})
    uid = bodyj(raw).get("user_id") if st in (200, 201) else None
    uid = uid or (run_sql(f"SELECT user_id FROM users WHERE login_id={q(login_id)}") or [None])[0]
    if uid:
        created["users"].append(uid)
    for cid in companies:
        call("POST", f"{P}/users/{uid}/assign-company", cms, {"company_id": cid, "role_code": "admin_doanh_nghiep"})
    return login_id, pw


def main():
    st, raw = call("POST", "/api/v1/auth/login", body=dict(zip(("email", "password"), persona("CMS"))))
    cms = bodyj(raw).get("session", {}).get("access_token")
    if not cms:
        raise SystemExit(f"CMS login failed: {st}")
    try:
        set_invite_grant(True)
        for label in ("A", "B"):
            st, raw = call("POST", f"{P}/companies", cms, {"company_name": f"{NAME} {label} (smoke only)"})
            created["companies"].append(bodyj(raw).get("company_id") if st in (200, 201) else None)
        ca, cb = created["companies"]
        record(ca and cb, "fixture: companies A and B created", created["companies"])
        u_login, u_pw = make_user(cms, "u", [ca, cb])
        v_login, v_pw = make_user(cms, "v", [cb])

        u_a = sign_in(u_login, u_pw, ca)
        u_b = sign_in(u_login, u_pw, cb)
        v_b = sign_in(v_login, v_pw)
        record(u_a[2] and u_b[2] and v_b[2] and v_b[5] == cb, "fixture: U signed in to A and to B, V to B",
               f"U/A={u_a[:2]} U/B={u_b[:2]} V={v_b[:2]} V company={v_b[5] == cb}")
        record(admin_api(u_b[2], u_b[3])[0] == 200 and admin_api(v_b[2], v_b[3])[0] == 200, "before: admin API works in B", "")

        # a company where an active member holds platform.cms.view cannot be deactivated (lock-out guard)
        run_sql("INSERT INTO membership_direct_permissions (membership_id, company_id, permission_code, granted_by) "
                f"VALUES ({q(u_a[3])}, {q(ca)}, 'platform.cms.view', {q(TAG)}) "
                f"ON DUPLICATE KEY UPDATE revoked_at=NULL, revoked_by=NULL, granted_by={q(TAG)}", write=True)
        st, raw = call("POST", f"{P}/companies/{ca}/deactivate", cms, {"reason": "qa smoke"})
        status_a = run_sql(f"SELECT status FROM companies WHERE company_id={q(ca)}")
        record(st == 409 and status_a == ["active"], "a company hosting a platform operator cannot be deactivated",
               f"{st} {code_of(raw)} status={status_a}")
        run_sql(f"UPDATE membership_direct_permissions SET revoked_at=CURRENT_TIMESTAMP, revoked_by={q(TAG)} "
                f"WHERE membership_id={q(u_a[3])} AND granted_by={q(TAG)} AND revoked_at IS NULL", write=True)
        if status_a != ["active"]:
            call("POST", f"{P}/companies/{ca}/activate", cms, {"reason": "qa smoke"})

        st, raw = call("POST", f"{P}/companies/{cb}/deactivate", cms, {"reason": "qa smoke"})
        record(st == 200, "deactivate B", f"{st} {code_of(raw)}")

        st, err = admin_api(v_b[2], v_b[3])
        record(st == 403 and err.startswith(INACTIVE), "V's existing session in B is cut off", f"{st} {err}")
        st, err = admin_api(u_b[2], u_b[3])
        record(st == 403 and err.startswith(INACTIVE), "U's existing session in B is cut off", f"{st} {err}")
        st, err = admin_api(u_a[2], u_a[3])
        record(st == 200, "U's session in A is not affected", f"{st} {err}")
        st, raw = call("POST", "/api/v1/auth/refresh", body={"refresh_token": v_b[4]})
        record(st == 403 and code_of(raw).startswith(INACTIVE), "V's refresh in B is refused", f"{st} {code_of(raw)}")
        r = sign_in(v_login, v_pw)
        record(r[0] == 403 and r[1].startswith(INACTIVE), "V cannot sign in (its only company is deactivated)", r[:2])
        r = sign_in(u_login, u_pw)
        record(r[2] and r[5] == ca, "U signs in and lands in A (B is skipped)", f"{r[:2]} company_is_A={r[5] == ca}")
        if r[2]:
            st, raw = call("GET", "/api/v1/me/companies", r[2])
            items = {i.get("company_id"): i.get("company_status") for i in (bodyj(raw).get("items") or [])}
            record(st == 200 and cb not in items and items.get(ca) == "active", "U's /me/companies omits B and shows company_status",
                   f"{st} B_listed={cb in items} A_status={items.get(ca)}")
            st, raw = call("POST", "/api/v1/auth/switch-company", r[2], {"company_id": cb})
            record(st == 403 and code_of(raw).startswith(INACTIVE), "U cannot switch to B", f"{st} {code_of(raw)}")

        st, raw = call("POST", f"{P}/companies/{cb}/activate", cms, {"reason": "qa smoke"})
        record(st == 200, "reactivate B", f"{st} {code_of(raw)}")
        st, err = admin_api(v_b[2], v_b[3])
        record(st == 200, "after reactivation V's existing session works again", f"{st} {err}")
        r = sign_in(v_login, v_pw)
        record(r[2] and r[5] == cb, "after reactivation V signs in to B", r[:2])
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
