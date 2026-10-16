"""Smoke MR-4 (mr4-override-maker-checker-2026-10-16) on DEV, through nginx (E2E_BASE_URL).

Writes (approved at GATE A of MR-4, test-owned only):
- a temporary direct grant of template.workflow.override.read/.write to the MEMBER persona
  (membership_direct_permissions through the QA read-write DB user), revoked at the end;
- workflow override versions on ONE QA-owned company type of c_001 (--type, default the
  "QA smoke F3" type), reset through the API at the end (DELETE …/workflow-override/active).

Checks:
1. Schema: migration 0156 in the ledger and its six columns present.
2. MEMBER (write only): publish:true is 403 and saves nothing; saves a draft and submits it; approve,
   reject, the pending list and rebase-apply are 403; a body that is not JSON is 400.
3. ENT (approver): the pending list shows the submission with names and the submit note; reject
   needs a reason (422) and keeps the submit note; approving a rejected version is 409.
4. Four eyes: MEMBER submits a new draft, ENT approves it (self_approved=false, from pending);
   approving again is 409. ENT publish:true activates at once (self_approved=true).
5. A deleted draft stays as discarded and its number is not reused.
6. Audit rows for submitted / rejected / approved / applied with the expected meta.
7. FE: the deployed bundle carries the MR-4 copy.

    python3 -B scripts/devqa/smoke_mr4_override_maker_checker.py [--type <type_id>] [--keep-grant]
    python3 -B scripts/devqa/smoke_mr4_override_maker_checker.py --ui-fixture up|down

--ui-fixture up grants MEMBER the temporary write grant and leaves one MEMBER submission pending on
the QA type for the read-only browser smoke (cobo_web_design docs/ai-cache/
mr4-override-maker-checker-2026-10-16/smoke-mr4-ui.mjs); --ui-fixture down rejects it, discards a
follow-up draft so no open version is left, and revokes the grant.

Nothing secret is printed. Exit code 1 when a check fails.
"""
import argparse
import json
import re
import subprocess
import sys
import time
import urllib.request
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
import devqa_env as env  # noqa: E402
from provision_qa_company import drop_access_cache  # noqa: E402
from smoke_workflow_step_skip import call, check, data, login, results  # noqa: E402

SAFE_ID = re.compile(r"^[A-Za-z0-9_.:-]{1,128}$")
DEFAULT_TYPE = "dt-co-1791964269635927893"  # "QA smoke F3 bất thường 2026-10-14", company type of c_001
PERMS = ("template.workflow.override.read", "template.workflow.override.write")
REVIEW_COLUMNS = ("submitted_by", "submitted_at", "submit_note", "rejected_by", "rejected_at", "decision_note")
FE_COPY = "Gửi duyệt"


def safe(value):
    if not SAFE_ID.match(value or ""):
        raise SystemExit(f"unexpected id format: {value!r}")
    return value


def sql_rows(statement, write=False):
    return [row.split("\t") for row in env.run_sql(statement, write=write)]


def workflow(label):
    return [{"step_id": "qa-mr4-s1", "stage": "Soạn thảo", "description": f"QA MR-4 smoke {label}", "due_rule": "T+2"}]


def code(body):
    return ((body or {}).get("error") or {}).get("code") or (body or {}).get("code")


def details(body):
    return ((body or {}).get("error") or {}).get("details") or (body or {}).get("details") or {}


def raw_post(path, token, raw):
    req = urllib.request.Request(env.base_url() + path, method="POST", data=raw,
                                 headers={"Content-Type": "application/json", "Authorization": "Bearer " + token})
    try:
        with urllib.request.urlopen(req, timeout=60) as r:
            return r.status
    except urllib.error.HTTPError as e:
        return e.code


def grant(member_membership, company):
    granted = []
    for perm in PERMS:
        have = sql_rows(f"SELECT COUNT(*) FROM membership_direct_permissions WHERE membership_id = '{member_membership}' AND company_id = '{company}' AND permission_code = '{perm}' AND revoked_at IS NULL")[0][0]
        if have == "0":
            env.run_sql(f"INSERT INTO membership_direct_permissions (membership_id, company_id, permission_code, granted_by, granted_at) "
                        f"VALUES ('{member_membership}', '{company}', '{perm}', 'qa-smoke-mr4', UTC_TIMESTAMP())", write=True)
            granted.append(perm)
    drop_access_cache(company)
    return granted


def revoke(member_membership, company):
    env.run_sql(f"UPDATE membership_direct_permissions SET revoked_at = UTC_TIMESTAMP(), revoked_by = 'qa-smoke-mr4' "
                f"WHERE membership_id = '{member_membership}' AND company_id = '{company}' AND granted_by = 'qa-smoke-mr4' AND revoked_at IS NULL", write=True)
    drop_access_cache(company)
    return sql_rows(f"SELECT COUNT(*) FROM membership_direct_permissions WHERE membership_id = '{member_membership}' AND granted_by = 'qa-smoke-mr4' AND revoked_at IS NULL")[0][0]


def ui_fixture(mode, company, type_id, base):
    ent, _ = login("ENT")
    _, member_membership = login("MEMBER")
    member_membership = safe(member_membership)
    if mode == "up":
        print(f"temporary MEMBER grant: {grant(member_membership, company) or 'already present'}")
        member, _ = login("MEMBER")
        s, b = call("PUT", base + "/draft", member, {"workflow": workflow("UI")})
        v = (b or {}).get("draft_version_no")
        s2, _ = call("POST", base + "/submit", member, {"version_no": v, "note": "QA MR-4 UI smoke"})
        print(f"ui fixture: draft HTTP {s} v{v}, submit HTTP {s2}")
        sys.exit(0 if s == 200 and s2 == 200 else 1)
    s, g = call("GET", base, ent)
    draft = (data(g) or {}).get("draft_version") or {}
    if draft.get("state") == "pending_approval":
        s, _ = call("POST", base + "/reject", ent, {"version_no": draft.get("version_no"), "reason": "QA MR-4 UI smoke: dọn dữ liệu thử"})
        print(f"ui fixture: reject v{draft.get('version_no')} HTTP {s}")
    member, _ = login("MEMBER")
    s, b = call("PUT", base + "/draft", member, {"workflow": workflow("cleanup")})
    v = (b or {}).get("draft_version_no")
    s2, _ = call("DELETE", f"{base}/draft/{v}", member)
    s3, g = call("GET", base, ent)
    print(f"ui fixture: discard follow-up draft v{v} HTTP {s}/{s2}; open version left: {bool((data(g) or {}).get('draft_version'))}")
    print(f"ui fixture: active temporary grants left={revoke(member_membership, company)}")


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--type", default=DEFAULT_TYPE)
    ap.add_argument("--keep-grant", action="store_true", help="do not revoke the MEMBER grant (debugging only)")
    ap.add_argument("--ui-fixture", choices=("up", "down"))
    args = ap.parse_args()
    company = safe(env.optional("E2E_COMPANY_ID", "c_001"))
    type_id = safe(args.type)
    base = f"/api/v1/company/disclosure-types/{type_id}/workflow-override"
    if args.ui_fixture:
        return ui_fixture(args.ui_fixture, company, type_id, base)

    # 1. Schema
    ledger = sql_rows("SELECT COUNT(*) FROM schema_migrations WHERE file_name = '0156_workflow_override_version_review.up.sql'")[0][0]
    cols = sql_rows("SELECT GROUP_CONCAT(column_name ORDER BY column_name) FROM information_schema.columns WHERE table_schema = DATABASE() "
                    "AND table_name = 'company_template_workflow_override_versions' AND column_name IN (" + ",".join(f"'{c}'" for c in REVIEW_COLUMNS) + ")")[0][0]
    check("[Schema] 0156 in the ledger with its six review columns", ledger == "1" and sorted(cols.split(",")) == sorted(REVIEW_COLUMNS), f"ledger={ledger} columns={cols}")
    owner = sql_rows(f"SELECT COALESCE(company_id, 'SYS'), (SELECT COUNT(*) FROM company_template_workflow_overrides WHERE company_id = '{company}' AND type_id = '{type_id}') FROM disclosure_types WHERE type_id = '{type_id}'")
    if not owner or owner[0][0] != company or owner[0][1] != "0":
        raise SystemExit(f"type {type_id} must be a company type of {company} without an override: {owner}")

    ent, ent_membership = login("ENT")
    member, member_membership = login("MEMBER")
    member_membership = safe(member_membership)
    start = sql_rows("SELECT DATE_FORMAT(UTC_TIMESTAMP() - INTERVAL 2 SECOND, '%Y-%m-%d %H:%i:%s')")[0][0]

    granted = []
    try:
        # Temporary grant: MEMBER becomes a writer without the approve permission.
        granted = grant(member_membership, company)
        print(f"temporary MEMBER grant: {granted or 'already present'}")
        member, _ = login("MEMBER")

        # 2. MEMBER, write only
        s, _ = call("GET", base, member)
        check("[Writer] GET override", s == 200, f"HTTP {s}")
        s, b = call("PUT", base + "/draft", member, {"workflow": workflow("publish"), "publish": True})
        rows = sql_rows(f"SELECT COUNT(*) FROM company_template_workflow_overrides WHERE company_id = '{company}' AND type_id = '{type_id}'")[0][0]
        check("[Writer] publish:true is 403 and saves nothing", s == 403 and rows == "0", f"HTTP {s} {code(b)} overrides={rows}")
        s, b = call("PUT", base + "/draft", member, {"workflow": workflow("v1")})
        v1 = (data(b) or {}).get("draft_version_no") or (b or {}).get("draft_version_no")
        check("[Writer] save draft", s == 200 and v1, f"HTTP {s} version={v1}")
        s, b = call("POST", base + "/submit", member, {"version_no": v1, "note": "QA MR-4 nhờ duyệt"})
        d = data(b) or {}
        check("[Writer] submit → pending_approval with submit_note", s == 200 and d.get("state") == "pending_approval" and d.get("submit_note") == "QA MR-4 nhờ duyệt" and not d.get("decision_note"),
              f"HTTP {s} state={d.get('state')} submit_note={d.get('submit_note')!r}")
        for name, method, path, body, headers in (
            ("approve", "POST", base + "/approve", {"version_no": v1}, None),
            ("reject", "POST", base + "/reject", {"version_no": v1, "reason": "x"}, None),
            ("pending list", "GET", "/api/v1/company/workflow-overrides/pending", None, None),
            ("rebase-apply", "POST", f"/api/v1/company/disclosure-types/{type_id}/workflow/override/rebase-apply", {"preview_id": "qa-mr4-none"}, {"Idempotency-Key": f"qa-mr4-{int(time.time())}"}),
        ):
            s, b = call(method, path, member, body, headers)
            check(f"[Writer] {name} is 403", s == 403, f"HTTP {s} {code(b)}")
        s = raw_post(base + "/submit", member, b"{not json")
        check("[Writer] submit with a body that is not JSON is 400", s == 400, f"HTTP {s}")

        # 3. ENT, approver
        s, b = call("GET", "/api/v1/company/workflow-overrides/pending", ent)
        items = (data(b) or {}).get("items") or []
        item = next((i for i in items if i.get("type_id") == type_id), None) or {}
        check("[Approver] pending list shows the submission with names and note",
              s == 200 and item.get("version_no") == v1 and item.get("type_name") and item.get("submitted_by_name") and item.get("submit_note") == "QA MR-4 nhờ duyệt",
              f"HTTP {s} items={len(items)} version={item.get('version_no')} names={bool(item.get('type_name'))}/{bool(item.get('submitted_by_name'))}")
        s, b = call("POST", base + "/reject", ent, {"version_no": v1, "reason": " "})
        check("[Approver] reject without a reason is 422 (field reason)", s == 422 and details(b).get("field") == "reason", f"HTTP {s} {details(b)}")
        s, b = call("POST", base + "/reject", ent, {"version_no": v1, "reason": "QA MR-4: thiếu bước rà soát"})
        d = data(b) or {}
        check("[Approver] reject keeps the submit note", s == 200 and d.get("state") == "rejected" and d.get("decision_note") == "QA MR-4: thiếu bước rà soát" and d.get("submit_note") == "QA MR-4 nhờ duyệt",
              f"HTTP {s} state={d.get('state')}")
        s, b = call("POST", base + "/approve", ent, {"version_no": v1})
        check("[Approver] approving a rejected version is 409 STATE_CONFLICT", s == 409 and code(b) == "STATE_CONFLICT", f"HTTP {s} {code(b)}")

        # 4. Four eyes, then self-approval
        s, b = call("PUT", base + "/draft", member, {"workflow": workflow("v2")})
        v2 = (data(b) or {}).get("draft_version_no") or (b or {}).get("draft_version_no")
        s2, _ = call("POST", base + "/submit", member, {"version_no": v2})
        s, b = call("POST", base + "/approve", ent, {"version_no": v2})
        check("[Four eyes] ENT approves MEMBER's submission: self_approved=false, from pending",
              s == 200 and b.get("self_approved") is False and b.get("from_state") == "pending_approval" and b.get("active_version_no") == v2,
              f"submit HTTP {s2}, approve HTTP {s} self_approved={b.get('self_approved')} from={b.get('from_state')}")
        s, b = call("POST", base + "/approve", ent, {"version_no": v2})
        check("[Four eyes] approving again is 409 STATE_CONFLICT", s == 409 and code(b) == "STATE_CONFLICT", f"HTTP {s} {code(b)}")
        s, b = call("PUT", base + "/draft", ent, {"workflow": workflow("v3"), "publish": True})
        v3 = (b or {}).get("draft_version_no")
        s_get, g = call("GET", base, ent)
        g = data(g) or {}
        active = ((g.get("override") or {}).get("active_version_no"))
        check("[Self] ENT publish:true activates at once (self_approved=true)",
              s == 200 and b.get("state") == "approved" and b.get("self_approved") is True and active == v3 and not g.get("draft_version"),
              f"HTTP {s} state={b.get('state')} self_approved={b.get('self_approved')} active={active}")

        # 5. Discarded draft
        s, b = call("PUT", base + "/draft", member, {"workflow": workflow("v4")})
        v4 = (data(b) or {}).get("draft_version_no") or (b or {}).get("draft_version_no")
        s_del, _ = call("DELETE", f"{base}/draft/{v4}", member)
        state = sql_rows(f"SELECT v.state FROM company_template_workflow_override_versions v JOIN company_template_workflow_overrides o ON o.override_id = v.override_id "
                         f"WHERE o.company_id = '{company}' AND o.type_id = '{type_id}' AND v.version_no = {int(v4)}")
        s, b = call("PUT", base + "/draft", member, {"workflow": workflow("v5")})
        v5 = (data(b) or {}).get("draft_version_no") or (b or {}).get("draft_version_no")
        check("[Discard] deleted draft kept as discarded, number not reused",
              s_del == 200 and state and state[0][0] == "discarded" and v5 == int(v4) + 1,
              f"delete HTTP {s_del} state={state} next={v5}")
        call("DELETE", f"{base}/draft/{v5}", member)

        # 6. Audit
        audit = sql_rows(f"""SELECT action, COALESCE(JSON_EXTRACT(metadata_json, '$.self_approved'), 'null'), COALESCE(JSON_UNQUOTE(JSON_EXTRACT(metadata_json, '$.from_state')), '')
            FROM audit_logs WHERE company_id = '{company}' AND resource_id = '{type_id}' AND occurred_at >= '{start}'
              AND action IN ('disclosure.workflow_override.submitted', 'disclosure.workflow_override.rejected', 'disclosure.workflow_override.approved', 'disclosure.workflow_override.applied')
            ORDER BY occurred_at""")
        actions = [a[0].rsplit(".", 1)[-1] for a in audit]
        approved = [a for a in audit if a[0].endswith(".approved")]
        applied = [a for a in audit if a[0].endswith(".applied")]
        check("[Audit] submitted x2, rejected, approved (self_approved=false), applied (self_approved=true)",
              actions.count("submitted") == 2 and actions.count("rejected") == 1 and len(approved) == 1 and approved[0][1] == "false"
              and approved[0][2] == "pending_approval" and len(applied) == 1 and applied[0][1] == "true",
              f"actions={actions}")
    finally:
        # Cleanup: reset the QA type's override through the API, revoke the temporary grant.
        s, _ = call("DELETE", base + "/active", ent)
        print(f"cleanup: reset override HTTP {s}")
        if not args.keep_grant:
            print(f"cleanup: active temporary grants left={revoke(member_membership, company)}")

    # 7. FE bundle
    with urllib.request.urlopen(env.base_url() + "/", timeout=60) as resp:
        html = resp.read().decode("utf-8", "replace")
    entry = re.findall(r'src="(/assets/index-[^"]+\.js)"', html)
    found, seen = False, set()
    queue = list(entry)
    while queue and not found and len(seen) < 400:
        path = queue.pop(0)
        if path in seen:
            continue
        seen.add(path)
        with urllib.request.urlopen(env.base_url() + path, timeout=120) as resp:
            text = resp.read().decode("utf-8", "replace")
        found = FE_COPY in text and "pending_approval" in text
        queue += [p if p.startswith("/") else "/assets/" + p.split("/")[-1] for p in re.findall(r'(?:/?assets/)?[A-Za-z0-9_.-]+-[A-Za-z0-9_-]{6,}\.js', text) if p.split("/")[-1] not in {s.split("/")[-1] for s in seen}]
    check("[FE] deployed bundle carries the MR-4 copy", found, f"chunks_scanned={len(seen)}")

    print(f"\n{sum(results)}/{len(results)} checks passed")
    sys.exit(0 if all(results) else 1)


if __name__ == "__main__":
    main()
