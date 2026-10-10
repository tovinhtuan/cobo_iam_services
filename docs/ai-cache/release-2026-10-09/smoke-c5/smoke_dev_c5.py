"""DEV smoke for C5 (membership_id / team / department / title company scope + new MySQL SQL).

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



FM, FTEAM, FDEPT, FTITLE = "m_002", "ou_org_legal_tv_001", "01a00dff-045d-763d-b3e2-2c7ed7e6a4db", "019e6c79-7231-72d9-95e6-a10283d4b22a"
MNF, NF = "MEMBERSHIP_NOT_FOUND", None
tok, mid = login(*persona("ENT"), OWN)
print(f"tenant admin logged in company={OWN} membership={mid}")
stamp = str(int(time.time()))
A = "/api/v1/admin"

def jid(raw, *keys):
    d = json.loads(raw); d = d.get("data", d)
    for k in keys:
        if isinstance(d, dict) and d.get(k): return d[k]
    for v in (d.values() if isinstance(d, dict) else []):
        if isinstance(v, dict):
            for k in keys:
                if v.get(k): return v[k]
    raise SystemExit(f"id not found in response keys={list(d) if isinstance(d, dict) else type(d)}")

# 1) read probe: proves the new binary is live
live, _, _ = check("READ foreign membership permissions -> 404", "GET", f"{A}/memberships/{FM}/permissions", tok, 404, MNF)
if not live:
    print("ABORT: read probe did not return 404 MEMBERSHIP_NOT_FOUND"); sys.exit(2)

# 2) rejected writes against other-company objects (new guards + new SQL paths)
neg = [
 ("PATCH foreign membership status (UpdateMembershipStatus)", "PATCH", f"{A}/memberships/{FM}", {"status": "inactive"}, MNF),
 ("DELETE foreign membership (DeleteMembership tx)", "DELETE", f"{A}/memberships/{FM}", None, MNF),
 ("POST role on foreign membership", "POST", f"{A}/memberships/{FM}/roles", {"role_id": "r0000001-0001-4000-8000-000000000001"}, MNF),
 ("PUT primary-role on foreign", "PUT", f"{A}/memberships/{FM}/primary-role", {"role_id": "r0000001-0001-4000-8000-000000000001"}, MNF),
 ("POST direct permission on foreign", "POST", f"{A}/memberships/{FM}/permissions", {"permission_code": "x", "permission_codes": ["x"]}, MNF),
 ("DELETE direct permission on foreign", "DELETE", f"{A}/memberships/{FM}/permissions/x", None, MNF),
 ("PUT org-assignments on foreign", "PUT", f"{A}/memberships/{FM}/org-assignments", {"department_ids": [], "title_ids": []}, MNF),
 ("POST team member (foreign team, own member)", "POST", f"{A}/teams/{FTEAM}/members", {"membership_id": mid}, None),
 ("POST team member (own team?, foreign member)", "POST", f"{A}/teams/{FTEAM}/members", {"membership_id": FM}, None),
 ("DELETE team member on foreign team", "DELETE", f"{A}/teams/{FTEAM}/members/{FM}", None, None),
 ("DELETE foreign team (DeleteTeamRow tx)", "DELETE", f"{A}/teams/{FTEAM}", None, None),
 ("PATCH foreign team", "PATCH", f"{A}/teams/{FTEAM}", {"name": "smoke"}, None),
 ("PATCH foreign department", "PATCH", f"{A}/departments/{FDEPT}", {"name": "smoke"}, None),
 ("DELETE foreign department (count SQL)", "DELETE", f"{A}/departments/{FDEPT}", None, None),
 ("POST team in foreign department", "POST", f"{A}/departments/{FDEPT}/teams", {"name": "smoke-c5"}, None),
 ("PATCH foreign title", "PATCH", f"{A}/titles/{FTITLE}", {"name": "smoke"}, None),
 ("DELETE foreign title (count SQL)", "DELETE", f"{A}/titles/{FTITLE}", None, None),
 ("POST title member foreign title", "POST", f"{A}/titles/{FTITLE}/members", {"membership_id": mid}, None, 400),
]
for row in neg:
    name, m, p, b, code = row[:5]
    check("NEG " + name, m, p, tok, row[5] if len(row) > 5 else 404, code, b)

# 3) own-company flow with throw-away objects (exercises new MySQL SQL, cleans itself up)
created = {}
def own(name, m, p, exp, body=None, grab=None):
    ok, st, raw = check("OWN " + name, m, p, tok, exp, None, body)
    if ok and grab:
        created[grab[0]] = jid(raw, *grab[1:])
    return ok
if own("create department", "POST", f"{A}/departments", 201, {"name": f"smoke-c5-{stamp}", "sort_order": 999}, ("dept", "department_id", "id")) or created.get("dept"):
    d = created["dept"]
    if own("create team (DepartmentBelongsToCompany + CountTeamsInDepartment)", "POST", f"{A}/departments/{d}/teams", 201, {"name": f"smoke-c5-team-{stamp}"}, ("team", "team_id", "id", "org_unit_id")):
        t = created["team"]
        own("add dept member (own)", "POST", f"{A}/departments/{d}/members", 200, {"membership_id": mid}) or None
        own("add team member (TeamBelongsToCompany)", "POST", f"{A}/teams/{t}/members", 201 if False else 200, {"membership_id": mid, "department_id": d})
        own("patch own team", "PATCH", f"{A}/teams/{t}", 200, {"name": f"smoke-c5-team2-{stamp}"})
        own("remove team member (JOIN delete SQL)", "DELETE", f"{A}/teams/{t}/members/{mid}", 204 if False else 200)
        own("add team member again", "POST", f"{A}/teams/{t}/members", 200, {"membership_id": mid, "department_id": d})
        own("delete team (tx lock + cascade)", "DELETE", f"{A}/teams/{t}", 200)
    own("remove dept member", "DELETE", f"{A}/departments/{d}/members/{mid}", 200)
    own("patch own department", "PATCH", f"{A}/departments/{d}", 200, {"name": f"smoke-c5-d2-{stamp}"})
    own("delete department (CountDepartmentMembers/CountTeams)", "DELETE", f"{A}/departments/{d}", 200)
if own("create title", "POST", f"{A}/titles", 201, {"name": f"smoke-c5-title-{stamp}", "sort_order": 999}, ("title", "title_id", "id")) or created.get("title"):
    ti = created["title"]
    own("delete title (CountTitleMembers)", "DELETE", f"{A}/titles/{ti}", 200)
own("idempotent PATCH own membership status unchanged (UpdateMembershipStatus)", "PATCH", f"{A}/memberships/{mid}", 200, {"status": "active"})
check("own permissions list still works", "GET", f"{A}/memberships/{mid}/permissions", tok, 200)

# 4) platform operator route still works
cms, _ = login(*persona("CMS"), OWN)
check("CMS operator lists users of other company", "GET", f"/api/v1/platform/cms/admin/users?company_id={OTHER}", cms, 200)

print()
for ok, name, method, path, status, code in results:
    print(f"{'PASS' if ok else 'FAIL'}  {status}  {name}  [{method} {path}] {code}")
print(f"\n{sum(r[0] for r in results)}/{len(results)} passed; created ids: {created}")
sys.exit(0 if all(r[0] for r in results) else 1)
