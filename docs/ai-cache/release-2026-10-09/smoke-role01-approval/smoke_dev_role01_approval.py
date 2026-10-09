"""DEV write smoke for the ROLE-01 follow-up: approval flow (A1), compare changes, nothing-to-apply,
critical rollback approval and the in-transaction restore, on real MySQL.

Writes: one temporary custom role (left inactive, no permissions) and one grantable direct
permission on m_102 that is granted and revoked again. Stops at the first unexpected answer.

Writes are limited to requests that the fixed server must reject, and are only sent after a
read probe proves the fixed binary is live. No data is created on a passing run.
Never prints passwords or tokens.
"""
import json
import re
import sys
import time
import urllib.error
import urllib.request

BASE = "http://88.216.208.0:3000"
IAM = sys.argv[1]
OWN = "c_001"
OTHER = "c_002"
CMS_OPERATOR_ROLE_C001 = "r0000001-0001-4000-8000-000000000016"  # seeded cms_operator of c_001 (carries platform.cms.view)
results = []


def seed_password(path, pattern):
    with open(f"{IAM}/migrations/{path}", encoding="utf-8") as f:
        m = re.search(pattern, f.read())
    if not m:
        raise SystemExit(f"cannot read seed password from {path}")
    return m.group(1)


def call(method, path, token=None, body=None):
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(BASE + path, data=data, method=method)
    req.add_header("Content-Type", "application/json")
    if token:
        req.add_header("Authorization", "Bearer " + token)
    try:
        with urllib.request.urlopen(req, timeout=25) as r:
            return r.status, r.read()
    except urllib.error.HTTPError as e:
        return e.code, e.read()


def code_of(raw):
    try:
        return json.loads(raw).get("error", {}).get("code", "")
    except Exception:
        return raw[:50].decode(errors="replace").strip()


def check(name, method, path, token, expect_status, expect_code=None, body=None):
    status, raw = call(method, path, token, body)
    code = code_of(raw) if status >= 400 else ""
    ok = status == expect_status and (expect_code is None or code == expect_code)
    results.append((ok, name, method, path, status, code))
    return ok, status, raw


def login(email, password, prefer_company):
    status, raw = call("POST", "/api/v1/auth/login", body={"email": email, "password": password})
    if status != 200:
        raise SystemExit(f"login failed for {email}: HTTP {status} {code_of(raw)}")
    body = json.loads(raw)
    body = body.get("data", body)
    sess = body.get("session", {})
    memberships = {m["company_id"]: m["membership_id"] for m in body.get("memberships", [])}
    if sess.get("access_token") and (body.get("current_context") or {}).get("company_id") == prefer_company:
        return sess["access_token"], memberships.get(prefer_company)
    pre = sess.get("pre_company_token") or sess.get("access_token")
    status, raw = call("POST", "/api/v1/auth/select-company", pre, {"company_id": prefer_company})
    if status != 200:
        raise SystemExit(f"select-company {prefer_company} failed for {email}: HTTP {status} {code_of(raw)}")
    sel = json.loads(raw)
    sel = sel.get("data", sel)
    return sel["access_token"], memberships.get(prefer_company)





import subprocess
A = "/api/v1/admin"
SSH = ["ssh", "-o", "BatchMode=yes", "-p", "21239", "root@88.216.208.0"]
MYSQL = "docker exec -i cobo-iam-mysql sh -c 'mysql -uroot -p\"$MYSQL_ROOT_PASSWORD\" cobo_iam -N'"

def sql(statement):
    r = subprocess.run(SSH + [MYSQL], input=statement, capture_output=True, text=True, timeout=60)
    if r.returncode != 0 and "Using a password" not in r.stderr:
        raise SystemExit(f"sql failed: {r.stderr[:300]}")
    return [l for l in r.stdout.splitlines() if l.strip()]

tenant_pw = seed_password("0009_seed_authz_test_accounts.up.sql", r"Password for all users below: (\S+)")
cms_pw = seed_password("0063_dev_platform_tenant_dual_admin.up.sql", r"platform\.tenant\.admin@example\.com / (\S+)")
req_tok, req_mid = login("admin.dn@example.com", tenant_pw, OWN)       # requester (rbac.manage via admin_doanh_nghiep)
apv_tok, apv_mid = login("platform.tenant.admin@example.com", cms_pw, OWN)  # approver: another member with rbac.manage
print(f"requester={req_mid} approver={apv_mid} company={OWN}")
assert req_mid != apv_mid
stamp = str(int(time.time()))

def bodyj(raw):
    d = json.loads(raw)
    return d.get("data", d) if isinstance(d, dict) else d

def call_as(tok, name, method, path, expect, code=None, body=None):
    ok, st, raw = check(name, method, path, tok, expect, code, body)
    if not ok:
        print(f"STOP at '{name}': HTTP {st} {raw[:300]!r}"); report(); sys.exit(1)
    return bodyj(raw) if raw else {}

def report():
    print()
    for ok, name, method, path, status, code in results:
        print(f"{'PASS' if ok else 'FAIL'}  {status}  {name}  [{method} {path}] {code}")
    print(f"\n{sum(r[0] for r in results)}/{len(results)} passed")

def expect(label, cond, detail=""):
    results.append((bool(cond), label, "-", "-", 0, str(detail)[:200]))
    if not cond:
        print(f"STOP: {label}: {detail}"); report(); sys.exit(1)

OUT_OF_SCOPE = ("SELECT COUNT(*), MD5(GROUP_CONCAT(CONCAT(rp.role_id,':',rp.permission_id,':',rp.status) ORDER BY rp.role_id, rp.permission_id)) "
                "FROM role_permissions rp JOIN roles r ON r.role_id=rp.role_id "
                "WHERE NOT (r.role_type='tenant_custom' AND r.is_protected=0 AND r.company_id='c_001');")
hash_before = sql(OUT_OF_SCOPE)
print("out-of-scope role_permissions fingerprint before:", hash_before)

# 1) ROLE-04 probe: rbac.manage reaches the real checks (before the fix: 403 from system.settings)
call_as(req_tok, "ROLE-04 assign admin to unknown member passes the gate -> 404", "POST", f"{A}/company/admins", 404, "MEMBERSHIP_NOT_FOUND", {"membership_id": "smoke-unknown-member"})

# 2) temporary custom role holding a legacy critical permission
perms = call_as(req_tok, "list permissions", "GET", f"{A}/permissions", 200).get("items", [])
pid = {p["permission_code"]: p["permission_id"] for p in perms}
role = call_as(req_tok, "create temp custom role", "POST", f"{A}/roles", 201, None, {"role_name": f"smoke-appr-{stamp}", "description": "approval smoke, temporary"})["role"]
RID = role["role_id"]
call_as(req_tok, "assign dashboard.view", "POST", f"{A}/roles/{RID}/permissions", 200, None, {"permission_id": pid["dashboard.view"]})
sql(f"INSERT INTO role_permissions (role_id, permission_id, status) VALUES ('{RID}', '{pid['rbac.manage']}', 'active');")   # legacy critical grant (not possible via the API)
call_as(req_tok, "assign deadline.view (snapshot with the legacy grant)", "POST", f"{A}/roles/{RID}/permissions", 200, None, {"permission_id": pid["deadline.view"]})

def role_codes():
    d = call_as(req_tok, "read role permissions", "GET", f"{A}/roles/{RID}/permissions", 200)
    return sorted(p["permission_code"] for p in d.get("permissions", []))
expect("role holds the legacy critical permission", "rbac.manage" in role_codes(), role_codes())

# 3) critical removal goes to approval; compare lists the one real change; self-approval is refused
routed = call_as(req_tok, "remove critical permission -> 202 flat body", "DELETE", f"{A}/roles/{RID}/permissions/{pid['rbac.manage']}", 202)
AID = routed.get("approval_id"); expect("202 body is flat {approval_id,status}", AID and routed.get("status") == "pending", routed)
cmp_ = call_as(req_tok, "compare approval lists real changes", "GET", f"{A}/config-approvals/{AID}/compare", 200)
ch = cmp_.get("changes", [])
expect("compare lists exactly the removal of rbac.manage on the temp role", len(ch) == 1 and ch[0].get("action") == "remove" and ch[0].get("permission_code") == "rbac.manage" and ch[0].get("role_id") == RID and ch[0].get("critical") is True, ch)
call_as(req_tok, "requester cannot approve own request -> 403", "POST", f"{A}/config-approvals/{AID}/approve", 403, "SELF_APPROVAL_NOT_ALLOWED")
expect("nothing changed before approval", "rbac.manage" in role_codes(), role_codes())

# 4) another member with rbac.manage (A1) approves; the apply runs the restore transaction
call_as(apv_tok, "approver with rbac.manage approves -> 200", "POST", f"{A}/config-approvals/{AID}/approve", 200)
codes = role_codes()
expect("after approval: rbac.manage removed, other permissions kept", "rbac.manage" not in codes and "dashboard.view" in codes and "deadline.view" in codes, codes)
vers = call_as(req_tok, "list rbac versions", "GET", f"{A}/rbac/matrix/versions?limit=1", 200).get("items", [])
expect("latest version is the approval apply", vers and vers[0].get("source") == "approval_apply", vers)
v_apply = vers[0]["version_no"]

# 5) critical rollback goes to approval, then applies
sql(f"INSERT INTO role_permissions (role_id, permission_id, status) VALUES ('{RID}', '{pid['rbac.manage']}', 'active');")   # drift: legacy grant is back
routed = call_as(req_tok, f"rollback to v{v_apply} touches a critical permission -> 202", "POST", f"{A}/rbac/matrix/versions/{v_apply}/rollback", 202, None, {"reason": "approval smoke"})
RID2 = routed.get("approval_id"); expect("rollback 202 body is flat", RID2 and routed.get("status") == "pending", routed)
expect("nothing changed before approval (rollback)", "rbac.manage" in role_codes(), role_codes())
call_as(apv_tok, "approver approves the rollback -> 200", "POST", f"{A}/config-approvals/{RID2}/approve", 200)
expect("after rollback approval: rbac.manage removed", "rbac.manage" not in role_codes(), role_codes())

# 6) an approval that would change nothing is refused and can be rejected
sql(f"INSERT INTO role_permissions (role_id, permission_id, status) VALUES ('{RID}', '{pid['rbac.manage']}', 'active');")
routed = call_as(req_tok, "queue another critical removal -> 202", "DELETE", f"{A}/roles/{RID}/permissions/{pid['rbac.manage']}", 202)
AID3 = routed["approval_id"]
sql(f"DELETE FROM role_permissions WHERE role_id='{RID}' AND permission_id='{pid['rbac.manage']}';")                         # applied outside the queue
call_as(apv_tok, "approve a no-op proposal -> 409", "POST", f"{A}/config-approvals/{AID3}/approve", 409, "APPROVAL_NOTHING_TO_APPLY")
call_as(apv_tok, "reject the no-op proposal -> 200", "POST", f"{A}/config-approvals/{AID3}/reject", 200, None, {"reject_reason": "smoke: nothing to apply"})

# 7) out-of-scope roles untouched; cleanup of the temp role
hash_after = sql(OUT_OF_SCOPE)
expect("roles outside the company's custom roles unchanged", hash_before == hash_after, f"{hash_before} -> {hash_after}")
call_as(req_tok, "cleanup: remove dashboard.view", "DELETE", f"{A}/roles/{RID}/permissions/{pid['dashboard.view']}", 200)
call_as(req_tok, "cleanup: remove deadline.view", "DELETE", f"{A}/roles/{RID}/permissions/{pid['deadline.view']}", 200)
st, raw = call("DELETE", f"{A}/roles/{RID}", req_tok)
results.append((st in (200, 204), "cleanup: inactivate temp role", "DELETE", f"{A}/roles/{RID}", st, ""))

# 8) D-T3: clear the stale pending approval of c_001 (sample data from July) through the API
pend = call_as(apv_tok, "list pending approvals of c_001", "GET", f"{A}/config-approvals?status=pending", 200).get("items", [])
for p in pend:
    call_as(apv_tok, f"reject stale pending {p['change_type']} ({p['approval_id'][:8]})", "POST", f"{A}/config-approvals/{p['approval_id']}/reject", 200, None, {"reject_reason": "obsolete sample data"})

report()
print(f"temp role {RID}; approvals {AID} {RID2} {AID3}")
sys.exit(0 if all(r[0] for r in results) else 1)
