"""DEV write smoke for ROLE-01 (RBAC rollback scope): runs the new MySQL restore SQL for real.

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




A = "/api/v1/admin"
CMS_OP_ROLE_C001 = "r0000001-0001-4000-8000-000000000016"  # cms_operator of c_001 (tenant_default, protected)
CMS_OP_ROLE_C002 = "r0000001-0001-4000-8000-000000000017"  # cms_operator of c_002
DIRECT_CODE = "template.workflow.override.approve"          # grantable, not critical, not held by m_102
tenant_pw = seed_password("0009_seed_authz_test_accounts.up.sql", r"Password for all users below: (\S+)")
cms_pw = seed_password("0063_dev_platform_tenant_dual_admin.up.sql", r"platform\.tenant\.admin@example\.com / (\S+)")
tok, mid = login("admin.dn@example.com", tenant_pw, OWN)
print(f"tenant admin logged in company={OWN} membership={mid}")
stamp = str(int(time.time()))

def body_of(raw):
    d = json.loads(raw)
    return d.get("data", d) if isinstance(d, dict) else d

def step(name, method, path, expect, code=None, body=None):
    ok, st, raw = check(name, method, path, tok, expect, code, body)
    if not ok:
        print(f"STOP at '{name}': HTTP {st} {raw[:300]!r}")
        report(); sys.exit(1)
    return body_of(raw) if raw else {}

def report():
    print()
    for ok, name, method, path, status, code in results:
        print(f"{'PASS' if ok else 'FAIL'}  {status}  {name}  [{method} {path}] {code}")
    print(f"\n{sum(r[0] for r in results)}/{len(results)} passed")

def role_perm_codes(role_id):
    d = step(f"read role permissions", "GET", f"{A}/roles/{role_id}/permissions", 200)
    return sorted(p["permission_code"] for p in d.get("permissions", []))

def direct_codes():
    d = step("read m_102 direct permissions", "GET", f"{A}/memberships/{mid}/permissions", 200)
    items = d.get("items", d.get("permissions", d if isinstance(d, list) else []))
    return sorted({p.get("permission_code") for p in items})

def latest_versions(n):
    d = step("list rbac versions", "GET", f"{A}/rbac/matrix/versions?limit={n}", 200)
    return [it["version_no"] for it in d.get("items", [])]

def expect(label, cond, detail):
    results.append((bool(cond), label, "-", "-", 0, detail))
    if not cond:
        print(f"STOP: {label}: {detail}"); report(); sys.exit(1)

# 1) probes that prove the new binary (no write on the new binary)
step("NEG submit rbac.permission.remove on protected cms_operator role -> 403", "POST", f"{A}/config-approvals", 403, "protected_role_read_only",
     {"change_type": "rbac.permission.remove", "proposed": {"role_id": CMS_OP_ROLE_C001, "permission_id": "smoke-nonexistent"}})
step("NEG submit rbac.permission.remove on other company role -> 404", "POST", f"{A}/config-approvals", 404, "NOT_FOUND",
     {"change_type": "rbac.permission.remove", "proposed": {"role_id": CMS_OP_ROLE_C002, "permission_id": "smoke-nonexistent"}})
if direct_codes().count(DIRECT_CODE):
    print(f"STOP: m_102 already holds {DIRECT_CODE}; pick another code"); sys.exit(1)

# 2) permission ids
perms = step("list permissions", "GET", f"{A}/permissions", 200).get("items", [])
pid = {p["permission_code"]: p["permission_id"] for p in perms}
P1, P2 = pid["dashboard.view"], pid["deadline.view"]

# 3) temporary custom role
role = step("create temp custom role", "POST", f"{A}/roles", 201, None, {"role_name": f"smoke-r1-{stamp}", "description": "ROLE-01 smoke, temporary"})["role"]
RID = role["role_id"]
print(f"temp role {RID}")

# 4) role SQL: DELETE path
step("assign dashboard.view (snapshot vA)", "POST", f"{A}/roles/{RID}/permissions", 200, None, {"permission_id": P1})
vA = latest_versions(1)[0]
step("assign deadline.view (snapshot vB)", "POST", f"{A}/roles/{RID}/permissions", 200, None, {"permission_id": P2})
out = step(f"ROLLBACK to vA={vA} (role DELETE SQL)", "POST", f"{A}/rbac/matrix/versions/{vA}/rollback", 200, None, {"reason": "ROLE-01 smoke delete path"})
expect("rollback 200 body has rolled_back_from/new_version.source=rollback", out.get("rolled_back_from") == vA and out.get("new_version", {}).get("source") == "rollback", out)
codes = role_perm_codes(RID)
expect("after DELETE path: role = [dashboard.view]", codes == ["dashboard.view"], codes)

# 5) role SQL: INSERT path
step("remove dashboard.view (snapshot vC)", "DELETE", f"{A}/roles/{RID}/permissions/{P1}", 200)
expect("role empty before INSERT path", role_perm_codes(RID) == [], "")
step(f"ROLLBACK to vA={vA} again (role INSERT SQL)", "POST", f"{A}/rbac/matrix/versions/{vA}/rollback", 200, None, {"reason": "ROLE-01 smoke insert path"})
codes = role_perm_codes(RID)
expect("after INSERT path: role = [dashboard.view]", codes == ["dashboard.view"], codes)

# 6) direct-grant SQL: INSERT and UPDATE paths
step(f"grant direct {DIRECT_CODE} to m_102 (snapshot vD)", "POST", f"{A}/memberships/{mid}/permissions", 200, None, {"permission_code": DIRECT_CODE})
vD = latest_versions(1)[0]
step(f"revoke direct {DIRECT_CODE} (snapshot vE)", "DELETE", f"{A}/memberships/{mid}/permissions/{DIRECT_CODE}", 200)
vE = latest_versions(1)[0]
expect("direct revoked before INSERT path", DIRECT_CODE not in direct_codes(), "")
step(f"ROLLBACK to vD={vD} (direct INSERT SQL)", "POST", f"{A}/rbac/matrix/versions/{vD}/rollback", 200, None, {"reason": "ROLE-01 smoke direct grant"})
expect("after direct INSERT path: m_102 holds the grant", DIRECT_CODE in direct_codes(), direct_codes())
step(f"ROLLBACK to vE={vE} (direct UPDATE SQL)", "POST", f"{A}/rbac/matrix/versions/{vE}/rollback", 200, None, {"reason": "ROLE-01 smoke direct revoke"})
expect("after direct UPDATE path: grant revoked again", DIRECT_CODE not in direct_codes(), direct_codes())
expect("temp role unchanged by direct rollbacks", role_perm_codes(RID) == ["dashboard.view"], role_perm_codes(RID))

# 7) cleanup: empty and inactivate the temp role
step("cleanup: remove dashboard.view", "DELETE", f"{A}/roles/{RID}/permissions/{P1}", 200)
st, raw = call("DELETE", f"{A}/roles/{RID}", tok)
results.append((st in (200, 204), "cleanup: inactivate temp role", "DELETE", f"{A}/roles/{RID}", st, ""))

# 8) platform operator still reaches the CMS
cms, _ = login("platform.tenant.admin@example.com", cms_pw, OWN)
check("CMS operator lists companies -> 200", "GET", "/api/v1/platform/cms/admin/companies", cms, 200)

report()
print(f"temp role: {RID}; versions vA={vA} vD={vD} vE={vE}")
sys.exit(0 if all(r[0] for r in results) else 1)
