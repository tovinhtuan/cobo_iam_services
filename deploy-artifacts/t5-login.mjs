const crypto = require("crypto");
const https = require("https");
const http = require("http");

async function fetchJson(url, opts = {}) {
  const res = await fetch(url, opts);
  const text = await res.text();
  let body = null;
  try { body = text ? JSON.parse(text) : null; } catch { body = text; }
  return { status: res.status, body, headers: Object.fromEntries(res.headers) };
}

function encryptPassword(spkiB64, password) {
  const spki = Buffer.from(spkiB64, "base64");
  const key = crypto.createPublicKey({ key: spki, format: "der", type: "spki" });
  const ciphertext = crypto.publicEncrypt(
    { key, padding: crypto.constants.RSA_PKCS1_OAEP_PADDING, oaepHash: "sha256" },
    Buffer.from(password, "utf8")
  );
  return ciphertext.toString("base64");
}

async function login(email, password) {
  const base = "http://88.216.208.0:8080";
  const keyRes = await fetchJson(base + "/api/v1/auth/login-password-key");
  if (keyRes.status !== 200 || !keyRes.body?.public_key_spki_b64) {
    throw new Error("key fetch failed " + keyRes.status + " " + JSON.stringify(keyRes.body));
  }
  const cipher = encryptPassword(keyRes.body.public_key_spki_b64, password);
  const loginRes = await fetchJson(base + "/api/v1/auth/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      email,
      remember_me: false,
      password_cipher: { alg: keyRes.body.alg, kid: keyRes.body.kid, ciphertext_b64: cipher }
    })
  });
  return loginRes;
}

(async () => {
  const email = process.argv[2];
  const password = process.argv[3];
  const r = await login(email, password);
  // print only non-secret summary
  const data = r.body?.data || r.body;
  const out = {
    status: r.status,
    next_action: data?.next_action,
    companies: (data?.companies || []).map(c => ({ id: c.company_id || c.id, name: c.company_name || c.name })),
    hasAccessToken: Boolean(data?.access_token || data?.tokens?.access_token),
    membershipHints: data?.membership_id || data?.active_membership_id || null,
    keys: data ? Object.keys(data) : [],
  };
  // Keep tokens in memory only for follow-up via env file in deploy-artifacts (gitignored)
  if (data?.access_token) {
    require("fs").writeFileSync("deploy-artifacts/t5-token-tmp.json", JSON.stringify({
      email,
      access_token: data.access_token,
      refresh_token: data.refresh_token,
      next_action: data.next_action,
      company_id: data.company_id,
      membership_id: data.membership_id,
      raw_keys: Object.keys(data)
    }), { encoding: "utf8" });
  } else if (data?.pre_company_token || data?.tokens) {
    require("fs").writeFileSync("deploy-artifacts/t5-token-tmp.json", JSON.stringify(data), { encoding: "utf8" });
  }
  console.log(JSON.stringify(out, null, 2));
  console.log("HTTP_STATUS=" + r.status);
})().catch(e => { console.error(e); process.exit(1); });
