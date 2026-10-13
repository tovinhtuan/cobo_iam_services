"""Smoke GET /api/v1/disclosure-types pagination and sort on DEV (Portal and CMS list).

Regression for 4a6fbce: a huge ?page= used to overflow the page offset and panic, so nginx
returned no response. Checks, through nginx (E2E_BASE_URL) as personas ENT (Portal) and CMS
(list_mode=management):

- page / page_size beyond int64, non-numeric, 0, page_size 101 -> 400 INVALID_REQUEST
- page=500000000000000000 and page=MaxInt64 -> 200, items [], real total
- invalid sort_by / sort_dir -> 400; no token -> 401
- sort_by=name asc/desc: name initials follow Vietnamese order (a < ă < â < b ... d < đ ... u < ư)

    python3 -B scripts/devqa/smoke_disclosure_type_list.py

Read-only (GET + login). Nothing secret is printed. Exit code 1 when a check fails.
"""
import json
import sys
import unicodedata
import urllib.error
import urllib.request
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
import devqa_env as env  # noqa: E402

PATH = "/api/v1/disclosure-types"
MAX_INT64 = 9223372036854775807
HUGE_PAGE = 500000000000000000  # (page-1)*20 overflows int64
NAME_PAGES_LIMIT = 20  # page_size=100 -> up to 2000 names per order check

TONE_MARKS = {"̀", "́", "̃", "̉", "̣"}
ALPHABET = "a ă â b c d đ e ê f g h i j k l m n o ô ơ p q r s t u ư v w x y z".split()

results = []


def call(method, path, token=None, body=None):
    """Returns (status, json body); status None when the connection dropped without a response."""
    req = urllib.request.Request(env.base_url() + path, method=method,
                                 data=json.dumps(body).encode() if body is not None else None,
                                 headers={"Content-Type": "application/json"})
    if token:
        req.add_header("Authorization", "Bearer " + token)
    try:
        with urllib.request.urlopen(req, timeout=60) as r:
            return r.status, json.loads(r.read() or b"{}")
    except urllib.error.HTTPError as e:
        raw = e.read()
        try:
            return e.code, json.loads(raw or b"{}")
        except ValueError:
            return e.code, {}
    except Exception as e:  # noqa: BLE001 - connection reset / remote closed
        return None, {"_exc": type(e).__name__}


def persona_token(p):
    email, password = env.persona(p)
    status, body = call("POST", "/api/v1/auth/login", body={"email": email, "password": password})
    if status != 200:
        raise SystemExit(f"{p} persona login failed: HTTP {status}")
    sess = body.get("data", body).get("session", {})
    if sess.get("access_token"):
        return sess["access_token"]
    status, sel = call("POST", "/api/v1/auth/select-company", sess.get("pre_company_token"),
                       {"company_id": env.optional("E2E_COMPANY_ID", "c_001")})
    if status != 200:
        raise SystemExit(f"{p} persona select-company failed: HTTP {status}")
    return sel.get("data", sel)["access_token"]


def check(name, ok, detail):
    results.append(ok)
    print(f"{'PASS' if ok else 'FAIL'}  {name}  -> {detail}")


def status_text(status, body):
    return f"HTTP {status}" if status is not None else f"NO RESPONSE ({body.get('_exc')})"


def expect_400(name, token, query, message):
    status, body = call("GET", f"{PATH}?{query}", token)
    err = body.get("error", {})
    ok = status == 400 and err.get("code") == "INVALID_REQUEST" and err.get("message") == message
    check(name, ok, f"{status_text(status, body)} {err.get('code')} {err.get('message')!r}")


def expect_empty_page(name, token, query, total, page, page_size):
    status, body = call("GET", f"{PATH}?{query}", token)
    ok = (status == 200 and body.get("items") == [] and body.get("total") == total
          and body.get("page") == page and body.get("page_size") == page_size)
    check(name, ok, f"{status_text(status, body)} items={len(body.get('items') or [])} "
                    f"total={body.get('total')} page={body.get('page')} page_size={body.get('page_size')}")


def baseline(name, token, extra):
    status, body = call("GET", f"{PATH}?page=1&page_size=20{extra}", token)
    total = body.get("total")
    ok = (status == 200 and isinstance(total, int) and body.get("page") == 1
          and body.get("page_size") == 20 and len(body.get("items") or []) == min(20, total))
    check(name, ok, f"{status_text(status, body)} total={total} items={len(body.get('items') or [])}")
    if not isinstance(total, int):
        raise SystemExit("cannot continue without a baseline total")
    return total


def initial_rank(name):
    """Primary rank of the first letter in the Vietnamese alphabet (tone marks ignored)."""
    s = name.strip().lower()
    if not s:
        return None
    base = unicodedata.normalize("NFC", "".join(
        c for c in unicodedata.normalize("NFD", s[0]) if c not in TONE_MARKS))
    return ALPHABET.index(base) if base in ALPHABET else None


def check_name_order(label, token, extra, total):
    for direction in ("asc", "desc"):
        names = []
        for page in range(1, NAME_PAGES_LIMIT + 1):
            status, body = call("GET", f"{PATH}?page={page}&page_size=100&sort_by=name&sort_dir={direction}{extra}", token)
            if status != 200:
                check(f"{label} sort_by=name {direction}", False, status_text(status, body))
                return
            names += [item.get("name") or "" for item in body.get("items", [])]
            if len(names) >= total or not body.get("items"):
                break
        ranks = [r for r in map(initial_rank, names) if r is not None]
        violations = sum(1 for a, b in zip(ranks, ranks[1:]) if (a > b if direction == "asc" else a < b))
        initials = " ".join(dict.fromkeys(n.strip()[:1] for n in names if n.strip()))
        check(f"{label} sort_by=name {direction}: initials in Vietnamese order",
              len(names) == total and violations == 0,
              f"names={len(names)}/{total} violations={violations} initials: {initials}")


def run_pagination_checks(label, token, extra, total):
    expect_400(f"{label} page beyond int64", token, f"page=100000000000000000000{extra}", "page must be a positive integer")
    expect_400(f"{label} page below int64", token, f"page=-100000000000000000000{extra}", "page must be a positive integer")
    expect_400(f"{label} page non-numeric", token, f"page=abc{extra}", "page must be a positive integer")
    expect_400(f"{label} page zero", token, f"page=0{extra}", "page must be a positive integer")
    expect_400(f"{label} page_size beyond int64", token, f"page_size=100000000000000000000{extra}",
               "page_size must be between 1 and 100")
    expect_400(f"{label} page_size 101", token, f"page_size=101{extra}", "page_size must be between 1 and 100")
    expect_empty_page(f"{label} page={HUGE_PAGE} (former panic)", token,
                      f"page={HUGE_PAGE}&page_size=20{extra}", total, HUGE_PAGE, 20)
    expect_empty_page(f"{label} page=MaxInt64", token, f"page={MAX_INT64}&page_size=100{extra}", total, MAX_INT64, 100)


def main():
    print(f"target: {env.base_url()}{PATH}")
    ent = persona_token("ENT")
    total = baseline("portal baseline", ent, "")
    status, body = call("GET", PATH, ent)
    check("portal defaults (no params)",
          status == 200 and body.get("page") == 1 and body.get("page_size") == 20 and body.get("total") == total,
          f"{status_text(status, body)} page={body.get('page')} page_size={body.get('page_size')} total={body.get('total')}")
    run_pagination_checks("portal", ent, "", total)
    expect_400("portal sort_by invalid", ent, "sort_by=foo", "sort_by must be one of: name, created_at")
    expect_400("portal sort_dir invalid", ent, "sort_by=name&sort_dir=up", "sort_dir must be one of: asc, desc")
    check_name_order("portal", ent, "", total)

    cms = persona_token("CMS")
    cms_extra = "&list_mode=management"
    cms_total = baseline("cms baseline (list_mode=management)", cms, cms_extra)
    run_pagination_checks("cms", cms, cms_extra, cms_total)
    check_name_order("cms", cms, cms_extra, cms_total)

    status, body = call("GET", f"{PATH}?page=100000000000000000000", None)
    check("no token -> 401", status == 401, status_text(status, body))

    passed = sum(results)
    print(f"\n{passed}/{len(results)} passed")
    sys.exit(0 if passed == len(results) else 1)


if __name__ == "__main__":
    main()
