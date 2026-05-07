import json
import urllib.request

BASE_URL = "http://localhost:8080"
LOGIN_ID = "cms.operator@example.com"
PASSWORD = "secret"
COMPANY_ID = "c_001"
SOURCE_ID = "dt-001"
TARGET_ID = "dt-qa-bao-cao-tai-chinh-quy-20260507-5"


def api_post(path: str, payload: dict, token: str | None = None) -> dict:
    headers = {"Content-Type": "application/json"}
    if token:
        headers["Authorization"] = f"Bearer {token}"
    req = urllib.request.Request(
        f"{BASE_URL}{path}",
        data=json.dumps(payload, ensure_ascii=False).encode("utf-8"),
        headers=headers,
        method="POST",
    )
    with urllib.request.urlopen(req) as resp:
        return json.loads(resp.read().decode("utf-8"))


def api_get(path: str, token: str) -> dict:
    req = urllib.request.Request(
        f"{BASE_URL}{path}",
        headers={"Authorization": f"Bearer {token}"},
        method="GET",
    )
    with urllib.request.urlopen(req) as resp:
        return json.loads(resp.read().decode("utf-8"))


def block_map(payload: dict) -> dict:
    blocks = payload.get("blocks", [])
    return {b.get("block_key", ""): b for b in blocks if b.get("block_key")}


def normalize(v):
    return json.dumps(v, ensure_ascii=False, sort_keys=True)


def main() -> None:
    login = api_post("/api/v1/auth/login", {"login_id": LOGIN_ID, "password": PASSWORD})
    token = login.get("session", {}).get("access_token", "")
    if not token:
        selected = api_post(
            "/api/v1/auth/select-company",
            {"company_id": COMPANY_ID},
            token=login["session"]["pre_company_token"],
        )
        token = selected["access_token"]

    src = api_get(f"/api/v1/disclosure-types/{SOURCE_ID}", token)
    tgt = api_get(f"/api/v1/disclosure-types/{TARGET_ID}", token)

    src_blocks = block_map(src)
    tgt_blocks = block_map(tgt)
    keys = [
        "legal_basis",
        "disclosure_content",
        "deadline",
        "channels_and_format",
        "legal_risks",
        "enterprise_workflow",
    ]

    rows = []
    for key in keys:
        s = src_blocks.get(key, {})
        t = tgt_blocks.get(key, {})
        same_title = s.get("title", "") == t.get("title", "")
        same_desc = s.get("description", "") == t.get("description", "")
        same_config = normalize(s.get("config", {})) == normalize(t.get("config", {}))
        rows.append(
            {
                "block_key": key,
                "title": "PASS" if same_title else "DIFF",
                "description": "PASS" if same_desc else "DIFF",
                "config": "PASS" if same_config else "DIFF",
            }
        )

    print(json.dumps({"source": SOURCE_ID, "target": TARGET_ID, "rows": rows}, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
