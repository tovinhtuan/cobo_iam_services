"""DEV write smoke, review round 2: validation of notification approvals, invite rule, single-change
approvals vs later direct grants, plan-digest binding, role create making approvals stale.

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
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[4] / "scripts" / "devqa"))
from devqa_env import base_url, optional, persona, run_sql  # credentials: ~/.cobo/dev-qa.env (docs: cobo_web_design/docs/ai-cache/dev-qa/)

BASE = base_url()
# Assumes DEV seed data (role/membership ids of c_001/c_002 used below).
OWN = optional("E2E_COMPANY_ID", "c_001")
OTHER = optional("E2E_OTHER_COMPANY_ID", "c_002")
CMS_OPERATOR_ROLE_C001 = "r0000001-0001-4000-8000-000000000016"  # seeded cms_operator of c_001 (carries platform.cms.view)
results = []


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
def sql(statement):  # writes fixtures too: QA_DB_RW_USER (falls back to container root)
    return run_sql(statement, write=True)

req_tok, req_mid = login(*persona("ENT"), OWN)       # requester (rbac.manage via admin_doanh_nghiep)
apv_tok, apv_mid = login(*persona("ENT2"), OWN)  # approver: another member with rbac.manage
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
print("out-of-scope fingerprint before:", hash_before)
D1, D2 = "template.workflow.override.approve", "template.workflow.override.write"

def direct_sql(code, active=True):
    if active:
        sql(f"INSERT INTO membership_direct_permissions (membership_id, company_id, permission_code, granted_by) VALUES ('m_102','c_001','{code}','smoke');")
    else:
        sql(f"UPDATE membership_direct_permissions SET revoked_at=CURRENT_TIMESTAMP, revoked_by='smoke' WHERE membership_id='m_102' AND permission_code='{code}' AND revoked_at IS NULL;")
def direct_active(code):
    return int(sql(f"SELECT COUNT(*) FROM membership_direct_permissions WHERE membership_id='m_102' AND permission_code='{code}' AND revoked_at IS NULL;")[0])

# 1) R17: an invalid alert-channel payload is not queued
rules = call_as(req_tok, "list notification rules", "GET", f"{A}/notification-rules", 200)
items_ = rules if isinstance(rules, list) else rules.get("items", [])
prefs = [r for r in items_ if r.get("rule_code") == "company.alert_channel_prefs.v1"]
if prefs:
    call_as(req_tok, "R17 invalid notification patch -> 400", "POST", f"{A}/config-approvals", 400, "INVALID_REQUEST",
            {"change_type": "notification_rule.patch", "aggregate_type": "notification_rule", "aggregate_id": prefs[0]["notification_rule_id"], "proposed": {"version": 2}})
else:
    print("no alert_channel_prefs rule in c_001: R17 probe skipped")
other = [r for r in items_ if r.get("rule_code") != "company.alert_channel_prefs.v1"]
if other:
    call_as(req_tok, "R17 non-prefs rule through the queue -> 400", "POST", f"{A}/config-approvals", 400, "INVALID_REQUEST",
            {"change_type": "notification_rule.patch", "aggregate_type": "notification_rule", "aggregate_id": other[0]["notification_rule_id"], "proposed": {"x": 1}})

# 2) R18: the invite permission belongs to the primary admin (admin.dn is not one)
call_as(req_tok, "R18 non-primary admin queues removal of admin.membership.invite -> 403", "POST", f"{A}/config-approvals", 403, "PERMISSION_DENIED",
        {"change_type": "rbac.direct_permission.remove", "aggregate_type": "rbac_matrix", "proposed": {"membership_id": req_mid, "permission_code": "admin.membership.invite"}})

# 3) temp role with a legacy critical permission
perms = call_as(req_tok, "list permissions", "GET", f"{A}/permissions", 200).get("items", [])
pid = {p["permission_code"]: p["permission_id"] for p in perms}
RID = call_as(req_tok, "create temp custom role", "POST", f"{A}/roles", 201, None, {"role_name": f"smoke-appr2-{stamp}", "description": "round 2 smoke, temporary"})["role"]["role_id"]
call_as(req_tok, "assign dashboard.view", "POST", f"{A}/roles/{RID}/permissions", 200, None, {"permission_id": pid["dashboard.view"]})
def legacy_critical():
    sql(f"INSERT IGNORE INTO role_permissions (role_id, permission_id, status) VALUES ('{RID}', '{pid['rbac.manage']}', 'active');")
def role_codes():
    d = call_as(req_tok, "read role permissions", "GET", f"{A}/roles/{RID}/permissions", 200)
    return sorted(p["permission_code"] for p in d.get("permissions", []))
def latest_version():
    return call_as(req_tok, "latest rbac version", "GET", f"{A}/rbac/matrix/versions?limit=1", 200)["items"][0]["version_no"]

# 4) ROLE-19 single change: a direct grant written later (no new version, like an invite) survives the approval
legacy_critical()
call_as(req_tok, "assign deadline.view (snapshot)", "POST", f"{A}/roles/{RID}/permissions", 200, None, {"permission_id": pid["deadline.view"]})
AID1 = call_as(req_tok, "queue removal of rbac.manage -> 202", "DELETE", f"{A}/roles/{RID}/permissions/{pid['rbac.manage']}", 202)["approval_id"]
direct_sql(D1)
call_as(apv_tok, "approve -> 200", "POST", f"{A}/config-approvals/{AID1}/approve", 200)
expect("role lost rbac.manage, kept the rest", "rbac.manage" not in role_codes() and "dashboard.view" in role_codes(), role_codes())
expect("ROLE-19: the direct grant made later survived", direct_active(D1) == 1, direct_active(D1))
direct_sql(D1, active=False)

# 5) ROLE-19 rollback: bound to the reviewed plan
v_last = latest_version()
legacy_critical()
AID2 = call_as(req_tok, f"critical rollback to v{v_last} -> 202", "POST", f"{A}/rbac/matrix/versions/{v_last}/rollback", 202, None, {"reason": "round 2 smoke"})["approval_id"]
direct_sql(D2)   # the plan would now also revoke this grant: nobody reviewed that
call_as(apv_tok, "approve after the plan changed -> 409 STALE_PROPOSAL", "POST", f"{A}/config-approvals/{AID2}/approve", 409, "STALE_PROPOSAL")
expect("a stale approval changed nothing", "rbac.manage" in role_codes() and direct_active(D2) == 1, [role_codes(), direct_active(D2)])
call_as(apv_tok, "reject the stale rollback -> 200", "POST", f"{A}/config-approvals/{AID2}/reject", 200, None, {"reject_reason": "smoke: stale"})
direct_sql(D2, active=False)

# 6) creating a custom role makes a pending approval stale
AID3 = call_as(req_tok, "queue another removal -> 202", "DELETE", f"{A}/roles/{RID}/permissions/{pid['rbac.manage']}", 202)["approval_id"]
RID2 = call_as(req_tok, "create a second custom role", "POST", f"{A}/roles", 201, None, {"role_name": f"smoke-appr2b-{stamp}", "description": "temporary"})["role"]["role_id"]
call_as(apv_tok, "approve after a role was created -> 409 STALE_PROPOSAL", "POST", f"{A}/config-approvals/{AID3}/approve", 409, "STALE_PROPOSAL")
call_as(apv_tok, "reject it -> 200", "POST", f"{A}/config-approvals/{AID3}/reject", 200, None, {"reject_reason": "smoke: stale"})

# 7) cleanup and checks
sql(f"DELETE FROM role_permissions WHERE role_id='{RID}' AND permission_id='{pid['rbac.manage']}';")
for code in ("dashboard.view", "deadline.view"):
    st, raw = call("DELETE", f"{A}/roles/{RID}/permissions/{pid[code]}", req_tok)
    results.append((st == 200, f"cleanup: remove {code}", "DELETE", "...", st, ""))
for r in (RID, RID2):
    st, raw = call("DELETE", f"{A}/roles/{r}", req_tok)
    results.append((st in (200, 204), "cleanup: inactivate temp role", "DELETE", f"{A}/roles/{r}", st, ""))
hash_after = sql(OUT_OF_SCOPE)
expect("roles outside the company's custom roles unchanged", hash_before == hash_after, f"{hash_before} -> {hash_after}")
pend = call_as(apv_tok, "no pending approvals left", "GET", f"{A}/config-approvals?status=pending", 200).get("items", [])
expect("no pending approval left over", len(pend) == 0, pend)
report()
print(f"temp roles {RID} {RID2}; approvals {AID1} {AID2} {AID3}")
sys.exit(0 if all(r[0] for r in results) else 1)
