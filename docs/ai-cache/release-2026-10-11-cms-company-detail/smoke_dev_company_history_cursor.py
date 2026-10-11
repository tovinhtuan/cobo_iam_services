"""DEV API smoke: company history keyset cursor (2026-10-11 follow-up F2).

Creates a disposable company `QA Smoke History <ts>` (test-owned, left inactive), fires concurrent PATCHes so several
real audit rows share one second (audit_logs is append-only: the QA DB user cannot insert), then pages GET …/history with limit=1 and checks: no gap /
no duplicate across same-second rows, opaque h1. cursors, no cursor on a full last page, malformed / fractional cursor
→ 400 field_errors.cursor, legacy RFC3339 cursor accepted, enterprise admin 403, and EXPLAIN of the keyset query.
Personas from ~/.cobo/dev-qa.env (never printed).
"""
import json
import random
import threading
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[3] / "scripts" / "devqa"))
from devqa_env import base_url, optional, persona, run_sql  # noqa: E402

BASE = base_url()
OWN = optional("E2E_COMPANY_ID", "c_001")
TS = time.strftime("%Y%m%d%H%M%S")
results = []


def call(method, path, token=None, body=None):
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(BASE + path, method=method, data=data, headers={"Content-Type": "application/json"})
    if token:
        req.add_header("Authorization", "Bearer " + token)
    try:
        with urllib.request.urlopen(req, timeout=40) as r:
            return r.status, json.loads(r.read() or b"{}")
    except urllib.error.HTTPError as e:
        try:
            return e.code, json.loads(e.read() or b"{}")
        except Exception:
            return e.code, {}


def record(ok, name, detail=""):
    results.append((bool(ok), name, str(detail)[:240]))


def field_errors(body):
    return (((body or {}).get("error") or {}).get("details") or {}).get("field_errors") or {}


def login(p, company):
    email, password = persona(p)
    st, body = call("POST", "/api/v1/auth/login", body={"email": email, "password": password})
    if st != 200:
        raise SystemExit(f"login failed for persona {p}: HTTP {st}")
    data = body.get("data") or body
    sess, ctx = data.get("session", {}), data.get("current_context") or {}
    if sess.get("access_token") and (ctx.get("company_id") == company or not sess.get("pre_company_token")):
        return sess["access_token"]
    st, body = call("POST", "/api/v1/auth/select-company", sess.get("pre_company_token") or sess.get("access_token"), {"company_id": company})
    if st != 200:
        raise SystemExit(f"select-company failed for persona {p}: HTTP {st}")
    return (body.get("data") or body)["access_token"]


def history(token, path, limit=None, cursor=None):
    q = {}
    if limit:
        q["limit"] = str(limit)
    if cursor is not None:
        q["cursor"] = cursor
    return call("GET", path + "/history" + ("?" + urllib.parse.urlencode(q) if q else ""), token)


def main():
    cms = login("CMS", OWN)
    ent = login("ENT", OWN)
    tax = "09" + "".join(random.choice("0123456789") for _ in range(8))
    st, body = call("POST", "/api/v1/platform/cms/admin/companies", cms, {"company_name": f"QA Smoke History {TS}", "tax_code": tax})
    record(st == 201, "create disposable company", st)
    data = (body or {}).get("data") or {}
    cid, code = data.get("company_id"), data.get("company_code")
    if not cid:
        raise SystemExit("company not created")
    path = f"/api/v1/platform/cms/admin/companies/{cid}"

    # Same-second events through the real API: concurrent PATCHes on different fields (rows serialize on the company
    # lock but commit within the same second). Retry a few rounds until >= 2 history rows share a second.
    fields = [("address", "Hà Nội"), ("phone", "0912345678"), ("representative_name", "Nguyễn Văn A"), ("registration_number", "0101")]
    synth, same_second = [], ""
    for round_no in range(4):
        threads = [
            threading.Thread(target=call, args=("PATCH", path, cms, {name: f"{value} {round_no}"}))
            for name, value in fields
        ]
        for t in threads:
            t.start()
        for t in threads:
            t.join()
        rows = run_sql(
            f"SELECT DATE_FORMAT(CONVERT_TZ(occurred_at, @@session.time_zone, '+00:00'), '%Y-%m-%d %H:%i:%s'), event_id FROM audit_logs "
            f"WHERE company_id = '{cid}' AND action = 'cms.company.update'"
        )
        by_second = {}
        for r in rows:
            sec, eid = r.split("	", 1)
            by_second.setdefault(sec, []).append(eid)
        sec, ids = max(by_second.items(), key=lambda kv: len(kv[1]))
        if len(ids) >= 2:
            same_second, synth = sec, ids
            break
    record(len(synth) >= 2, "real audit rows sharing one second", f"{len(synth)} at {same_second}")
    first_synth = sorted(synth)[0] if synth else ""

    try:
        st, body = history(cms, path)
        all_ids = [i.get("event_id") for i in ((body.get("data") or {}).get("items") or [])]
        record(st == 200 and synth and all(e in all_ids for e in synth), "first page (default limit) has the same-second events", len(all_ids))
        record(((body.get("meta") or {}).get("next_cursor")) is None, "full single page: next_cursor null (limit+1)", (body.get("meta") or {}).get("next_cursor"))

        paged, cursors, cursor = [], [], None
        for _ in range(len(all_ids) + 3):
            st, body = history(cms, path, limit=1, cursor=cursor)
            if st != 200:
                break
            paged += [i.get("event_id") for i in ((body.get("data") or {}).get("items") or [])]
            cursor = (body.get("meta") or {}).get("next_cursor")
            if not cursor:
                break
            cursors.append(cursor)
        record(paged == all_ids, "limit=1 paging = same order, no gap, no duplicate", f"{len(paged)} vs {len(all_ids)}")
        record(cursors and all(c.startswith("h1.") for c in cursors), "cursors are opaque h1.", cursors[:1])

        for bad in ["not-a-cursor", "2026-10-11T02:00:00.5Z", "h1.!!!"]:
            st, body = history(cms, path, cursor=bad)
            record(st == 400 and "cursor" in field_errors(body), f"malformed cursor {bad!r} -> 400 field_errors.cursor", st)
        # Legacy cursor = whole second, strictly older: at T it drops every event of second T (the old lossy paging),
        # at T+1s it returns them all.
        st, body = history(cms, path, cursor=same_second.replace(" ", "T") + "Z")
        at_t = [i.get("event_id") for i in ((body.get("data") or {}).get("items") or [])]
        next_second = time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime(time.mktime(time.strptime(same_second, "%Y-%m-%d %H:%M:%S")) - time.timezone + 1))
        st2, body2 = history(cms, path, cursor=next_second)
        after_t = [i.get("event_id") for i in ((body2.get("data") or {}).get("items") or [])]
        record(st == 200 and st2 == 200 and not any(e in at_t for e in synth) and all(e in after_t for e in synth),
               "legacy RFC3339 cursor accepted (strictly older second)", f"at T: {len(at_t)}, at T+1s: {len(after_t)}")

        st, _ = history(ent, path)
        record(st == 403, "enterprise admin history -> 403", st)

        plan = run_sql(
            "EXPLAIN SELECT event_id FROM audit_logs WHERE company_id = '" + cid + "' "
            f"AND (occurred_at < '{same_second}' OR (occurred_at = '{same_second}' AND event_id < '{first_synth}')) "
            "AND action LIKE CONCAT('cms.company', '%') ORDER BY occurred_at DESC, event_id DESC LIMIT 21"
        )
        joined = " | ".join(plan)
        record("idx_audit_logs_company_occurred" in joined and "filesort" not in joined.lower(), "EXPLAIN keyset: company index, no filesort", joined[:220])
        plan2 = run_sql(
            "EXPLAIN SELECT event_id FROM audit_logs WHERE ('" + cid + "' = '' OR company_id = '" + cid + "') "
            "AND action LIKE CONCAT('cms.company', '%') ORDER BY occurred_at DESC, event_id DESC LIMIT 21"
        )
        record(True, "info: EXPLAIN with repository's (? = '' OR company_id = ?) form", " | ".join(plan2)[:220])
    finally:
        st, _ = call("POST", path + "/deactivate", cms, {"reason_code": "OTHER", "reason_note": "qa smoke cleanup", "confirm_company_code": code})
        record(st == 200, "cleanup: company deactivated", st)

    out = Path(__file__).with_suffix(".out")
    lines = [f"{'PASS' if ok else 'FAIL'}  {n}  {d}" for ok, n, d in results]
    lines.append(f"\n{sum(ok for ok, _, _ in results)}/{len(results)} PASS  (company {cid})")
    out.write_text("\n".join(lines) + "\n", encoding="utf-8")
    print("\n".join(lines))
    return 0 if all(ok for ok, _, _ in results) else 1


if __name__ == "__main__":
    sys.exit(main())
