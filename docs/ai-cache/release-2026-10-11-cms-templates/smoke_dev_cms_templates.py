"""DEV API smoke for /cms/templates fixes (2026-10-11).

Covers the High batch (create_only, archived guard, narrative preserve, body limits) and the Medium
batch (revision/expected_revision/STALE_ETAG, PUT /config 410, duration-type derivation, list search
by type_id + paging, audit). Uses QA personas from ~/.cobo/dev-qa.env (never printed).

Data: creates disposable global templates `qa-smoke-cms-<ts>-*` (DEV templates are example data per
user, 2026-10-11) and archives them at the end.
"""
import json
import sys
import time
import urllib.error
import urllib.request
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[3] / "scripts" / "devqa"))
from devqa_env import base_url, optional, persona, run_sql  # noqa: E402

BASE = base_url()
OWN = optional("E2E_COMPANY_ID", "c_001")
TS = time.strftime("%Y%m%d%H%M%S")
results = []


def call(method, path, token=None, body=None, raw_body=None):
    data = raw_body if raw_body is not None else (json.dumps(body).encode() if body is not None else None)
    req = urllib.request.Request(BASE + path, method=method, data=data, headers={"Content-Type": "application/json"})
    if token:
        req.add_header("Authorization", "Bearer " + token)
    try:
        with urllib.request.urlopen(req, timeout=40) as r:
            return r.status, r.read()
    except urllib.error.HTTPError as e:
        return e.code, e.read()


def j(raw):
    try:
        d = json.loads(raw)
    except Exception:
        return {}
    return d["data"] if isinstance(d, dict) and isinstance(d.get("data"), (dict, list)) else d


def err(raw):
    try:
        return (json.loads(raw).get("error") or {}).get("code", "")
    except Exception:
        return ""


def record(ok, name, detail=""):
    results.append((bool(ok), name, str(detail)[:220]))


def login(p, company):
    email, password = persona(p)
    st, raw = call("POST", "/api/v1/auth/login", body={"email": email, "password": password})
    if st != 200:
        raise SystemExit(f"login failed for persona {p}: HTTP {st}")
    body = j(raw)
    sess, ctx = body.get("session", {}), body.get("current_context") or {}
    if sess.get("access_token") and ctx.get("company_id") == company:
        return sess["access_token"]
    st, raw = call("POST", "/api/v1/auth/select-company", sess.get("pre_company_token") or sess.get("access_token"), {"company_id": company})
    if st != 200:
        raise SystemExit(f"select-company failed for persona {p}: HTTP {st}")
    return j(raw)["access_token"]


def versions(tok, type_id):
    st, raw = call("GET", f"/api/v1/admin/disclosure-types/{type_id}/versions", tok)
    return st, j(raw)


def version_detail(tok, type_id, n):
    st, raw = call("GET", f"/api/v1/admin/disclosure-types/{type_id}/versions/{n}", tok)
    return st, j(raw)


UPSERT_KEYS = [
    "group_id", "name", "category", "template_category", "deadline_strategy", "description", "deadline_rule",
    "periodicity", "legal_basis", "report_content", "required_docs", "legal_risks_text", "implementation_content",
    "general_info", "channels_text", "format", "checklist", "tags", "deadline_config", "blocks",
    "display_group_codes", "applicability_rules", "legal_bases",
]


def body_from_detail(detail, type_id, **extra):
    body = {k: detail[k] for k in UPSERT_KEYS if k in detail and detail[k] is not None}
    body.update({"type_id": type_id, "scope": "global"})
    body.update(extra)
    return body


def main():
    cms = login("CMS", OWN)
    ent = login("ENT", OWN)

    # Source: any global template with an active version (example data).
    st, raw = call("GET", "/api/v1/disclosure-types?list_mode=management&scope=global&portal_state=active&page=1&page_size=50", cms)
    items = (j(raw) or {}).get("items") or []
    record(st == 200 and "total" in (j(raw) or {}), "list management returns total (paging)", f"{st} total={(j(raw) or {}).get('total')}")
    src = next((i for i in items if i.get("active_version_no")), None)
    if not src:
        record(False, "found an active global template to copy", "none")
        return
    st, vl = versions(cms, src["type_id"])
    active_no = next((v["version_no"] for v in vl.get("items", []) if v.get("is_active")), src["active_version_no"])
    st, detail = version_detail(cms, src["type_id"], active_no)
    record(st == 200 and str(detail.get("revision", "")).startswith("pc1:"), "version detail exposes revision", f"{st} rev={str(detail.get('revision'))[:12]}")

    new_id = f"qa-smoke-cms-{TS}"
    created = []

    # H1: create_only
    st, raw = call("PUT", f"/api/v1/admin/disclosure-types/{new_id}", cms,
                   body_from_detail(detail, new_id, name=f"QA smoke {TS}", create_only=True, change_note="qa smoke create"))
    rev1 = (j(raw) or {}).get("revision", "")
    record(st == 200 and rev1.startswith("pc1:"), "H1 create_only creates + returns revision", f"{st} {err(raw)}")
    if st == 200:
        created.append(new_id)
    st, raw = call("PUT", f"/api/v1/admin/disclosure-types/{new_id}", cms, body_from_detail(detail, new_id, create_only=True))
    record(st == 409 and err(raw) == "STATE_CONFLICT", "H1 create_only on existing type_id -> 409", f"{st} {err(raw)}")
    st, raw = call("PUT", f"/api/v1/admin/disclosure-types/{new_id}", cms, body_from_detail(detail, new_id, create_only="true"))
    record(st == 400, "create_only non-boolean -> 400 (fail closed)", f"{st} {err(raw)}")

    # M1: head_revision + compare-and-set
    st, vl = versions(cms, new_id)
    head = vl.get("head_revision", "")
    record(st == 200 and head == rev1, "M1 head_revision == create response revision", f"{st} head={head[:12]} rev={rev1[:12]}")
    st, raw = call("PUT", f"/api/v1/admin/disclosure-types/{new_id}", cms,
                   body_from_detail(detail, new_id, name=f"QA smoke {TS} v2", applicability="QA-APPLICABILITY", expected_revision=head))
    rev2 = (j(raw) or {}).get("revision", "")
    record(st == 200 and rev2 and rev2 != head, "M1 save with fresh expected_revision -> 200 + new revision", f"{st} {err(raw)}")
    st, raw = call("PUT", f"/api/v1/admin/disclosure-types/{new_id}", cms,
                   body_from_detail(detail, new_id, name="stale writer", expected_revision=head))
    d = json.loads(raw).get("error", {}).get("details", {}) if raw else {}
    record(st == 409 and err(raw) == "STALE_ETAG" and d.get("current_revision") == rev2,
           "M1 stale expected_revision -> 409 STALE_ETAG (+current_revision)", f"{st} {err(raw)}")

    # H3: omitted narrative key preserved; explicit value kept
    body = body_from_detail(detail, new_id, name=f"QA smoke {TS} v3", expected_revision=rev2)
    body.pop("applicability", None)
    st, raw = call("PUT", f"/api/v1/admin/disclosure-types/{new_id}", cms, body)
    rev3 = (j(raw) or {}).get("revision", "")
    st_v, vl = versions(cms, new_id)
    draft_no = next((v["version_no"] for v in vl.get("items", []) if not v.get("is_active") and not v.get("is_released")), None)
    st_d, dd = version_detail(cms, new_id, draft_no) if draft_no else (0, {})
    record(st == 200 and dd.get("applicability") == "QA-APPLICABILITY", "H3 omitted 'applicability' preserved on save", f"{st} got={dd.get('applicability')!r}")

    # H3-rest: deadline_duration_type derived from applicability_rules.deadline_day_type
    rules = dd.get("applicability_rules") or {}
    cfg = dd.get("deadline_config") or {}
    if rules and cfg:
        want = "WORKING_DAYS" if str(rules.get("deadline_day_type", "")).lower() == "working" else "CALENDAR_DAYS"
        record(cfg.get("deadline_duration_type") == want, "H3-rest deadline_duration_type derived from deadline_day_type",
               f"day_type={rules.get('deadline_day_type')!r} duration={cfg.get('deadline_duration_type')!r}")
    else:
        record(True, "H3-rest derivation (skipped: template has no rules/config)", "")

    # M2: PUT /config retired, GET still works for global
    st, raw = call("PUT", f"/api/v1/admin/disclosure-types/{new_id}/config", cms, {"deadline_config": {}})
    record(st == 410 and err(raw) == "ENDPOINT_REMOVED", "M2 PUT /config -> 410 ENDPOINT_REMOVED", f"{st} {err(raw)}")
    st, raw = call("GET", f"/api/v1/admin/disclosure-types/{src['type_id']}/config", cms)
    record(st == 200, "M2 GET /config (global) still 200", f"{st}")

    # M5: search matches type_id fragment
    st, raw = call("GET", f"/api/v1/disclosure-types?list_mode=management&scope=global&portal_state=all&q=cms-{TS}&page=1&page_size=50", cms)
    found = any(i.get("type_id") == new_id for i in (j(raw) or {}).get("items") or [])
    record(st == 200 and found, "M5 list q matches type_id", f"{st} found={found}")

    # M1 workflow facade: GET revision + stale PUT rejected
    st, raw = call("GET", f"/api/v1/platform/cms/templates/{new_id}/workflow", cms)
    wf = j(raw) or {}
    wf_rev = wf.get("revision", "")
    record(st == 200 and wf_rev == rev3, "M1 workflow GET revision == head", f"{st} wf={wf_rev[:12]} head={rev3[:12]}")
    steps = wf.get("steps") or []
    if not steps:
        st_dep, raw_dep = call("GET", "/api/v1/platform/cms/workflow/template-departments", cms)
        deps = j(raw_dep)
        deps = deps if isinstance(deps, list) else (deps or {}).get("items") or []
        dep_id = ((deps[0] or {}).get("department_id") or (deps[0] or {}).get("id")) if deps else "dept-compliance"
        steps = [{"stage": "QA review", "department_id": dep_id, "assignee_role_ids": ["role-reviewer"],
                  "due_rule": "T+3", "processing_days": 3, "display_order": 1, "description": "QA smoke step"}]
    st, raw = call("PUT", f"/api/v1/platform/cms/templates/{new_id}/workflow", cms,
                   {"change_note": "qa smoke stale", "steps": steps, "expected_revision": rev1})
    record(st == 409 and err(raw) == "STALE_ETAG", "M1 workflow PUT with stale revision -> 409", f"{st} {err(raw)} {raw[:120]!r}")
    st, raw = call("PUT", f"/api/v1/platform/cms/templates/{new_id}/workflow", cms,
                   {"change_note": "qa smoke fresh", "steps": steps, "expected_revision": wf_rev})
    wf_saved = j(raw) or {}
    record(st == 200 and wf_saved.get("revision") and wf_saved.get("revision") != wf_rev,
           "M1 workflow PUT with fresh revision -> 200 + new revision", f"{st} {err(raw)} {raw[:120]!r}")
    st_v, vl = versions(cms, new_id)
    record(vl.get("head_revision") == wf_saved.get("revision"), "M1 head_revision follows workflow save",
           f"head={str(vl.get('head_revision'))[:12]} saved={str(wf_saved.get('revision'))[:12]}")

    # M1 activate: stale client revision rejected before validation
    st, raw = call("POST", f"/api/v1/admin/disclosure-types/{new_id}/activate", cms,
                   {"version_no": draft_no or 1, "reason": "qa smoke", "expected_revision": rev1})
    record(st == 409 and err(raw) == "STALE_ETAG", "M1 activate with stale revision -> 409 STALE_ETAG", f"{st} {err(raw)}")

    # Body limits
    big = json.dumps({"name": "x" * (4 * 1024 * 1024 + 10)}).encode()
    st, raw = call("PUT", f"/api/v1/admin/disclosure-types/{new_id}", cms, raw_body=big)
    record(st == 413, "body > 4 MiB -> 413", f"{st} {err(raw)}")
    st, raw = call("PUT", f"/api/v1/admin/disclosure-types/{new_id}", cms, raw_body=b'{"name":')
    record(st == 400, "malformed JSON -> 400 (was 500)", f"{st} {err(raw)}")

    # Role boundary: enterprise admin cannot write global templates
    st, raw = call("PUT", f"/api/v1/admin/disclosure-types/{new_id}", ent, body_from_detail(detail, new_id, name="ent attempt"))
    record(st == 403, "Enterprise Admin PUT global template -> 403", f"{st} {err(raw)}")

    # H2: archived template rejects draft writes; restore works afterwards
    st, raw = call("POST", f"/api/v1/platform/cms/templates/{new_id}/archive", cms, {"reason": "qa smoke"})
    record(st == 200, "archive qa template", f"{st} {err(raw)}")
    st, raw = call("PUT", f"/api/v1/admin/disclosure-types/{new_id}", cms, body_from_detail(detail, new_id, name="after archive"))
    record(st == 409 and err(raw) == "TEMPLATE_ARCHIVED", "H2 save on archived global template -> 409 TEMPLATE_ARCHIVED", f"{st} {err(raw)}")
    st, raw = call("POST", f"/api/v1/platform/cms/templates/{new_id}/restore", cms, {"reason": "qa smoke"})
    record(st == 200, "H2 restore still works after rejected save", f"{st} {err(raw)}")

    # M8: audit rows for this run (read-only SQL)
    out = run_sql(
        "SELECT action, COUNT(*) FROM audit_logs WHERE resource_id = '" + new_id + "' GROUP BY action ORDER BY action"
    )
    text = "; ".join(out)
    record("disclosure.type.version.upsert" in text and "cms_template.archive" in text and "cms_workflow.upsert" in text,
           "M8 audit rows: upsert / workflow upsert / archive", text[:200])
    meta = run_sql("SELECT JSON_EXTRACT(metadata_json, '$.step_count') FROM audit_logs WHERE resource_id = '"
                   + new_id + "' AND action = 'cms_workflow.upsert' ORDER BY occurred_at DESC LIMIT 1")
    record(bool(meta) and meta[0].strip() not in ("", "NULL"), "M8 cms_workflow.upsert metadata has step_count", "; ".join(meta))

    # Cleanup: archive disposable templates
    for tid in created:
        call("POST", f"/api/v1/platform/cms/templates/{tid}/archive", cms, {"reason": "qa smoke cleanup"})


def report():
    print()
    for ok, name, detail in results:
        print(f"{'PASS' if ok else 'FAIL'}  {name}  {detail}")
    print(f"\n{sum(r[0] for r in results)}/{len(results)} passed")


if __name__ == "__main__":
    try:
        main()
    finally:
        report()
