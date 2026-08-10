const fs = require("fs");
const crypto = require("crypto");
const BASE = "http://88.216.208.0:8080";
async function fetchJson(url, opts = {}) {
  const res = await fetch(url, opts);
  const text = await res.text();
  let body = null; try { body = text ? JSON.parse(text) : null; } catch { body = text; }
  return { status: res.status, body };
}
function enc(spki, pw) {
  const key = crypto.createPublicKey({ key: Buffer.from(spki, "base64"), format: "der", type: "spki" });
  return crypto.publicEncrypt({ key, padding: crypto.constants.RSA_PKCS1_OAEP_PADDING, oaepHash: "sha256" }, Buffer.from(pw)).toString("base64");
}
async function login(email, password) {
  const keyRes = await fetchJson(BASE + "/api/v1/auth/login-password-key");
  const cipher = enc(keyRes.body.public_key_spki_b64, password);
  const r = await fetchJson(BASE + "/api/v1/auth/login", {
    method: "POST", headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ email, remember_me: false, password_cipher: { alg: keyRes.body.alg, kid: keyRes.body.kid, ciphertext_b64: cipher } })
  });
  const data = r.body.data || r.body;
  return { status: r.status, token: data.session?.access_token, mid: data.current_context?.membership_id, cid: data.current_context?.company_id, next: data.next_action, err: r.body?.error };
}
async function api(token, path, opts = {}) {
  const headers = { Authorization: "Bearer " + token, ...(opts.headers||{}) };
  if (opts.body) headers["Content-Type"] = "application/json";
  return fetchJson(BASE + path, { method: opts.method || "GET", headers, body: opts.body ? JSON.stringify(opts.body) : undefined });
}
(async () => {
  const propose = await login("nhanvien@example.com", "secret");
  console.log("login", { status: propose.status, mid: propose.mid, cid: propose.cid, next: propose.next, hasToken: !!propose.token, err: propose.err });
  const me = await api(propose.token, "/api/v1/me");
  console.log("me", { status: me.status, keys: Object.keys(me.body||{}), ctx: me.body?.current_context || me.body?.data?.current_context, err: me.body?.error });
  const create = await api(propose.token, "/api/v1/company/ad-hoc-proposals", {
    method: "POST",
    body: {
      type_id: "dt-co-1783321973009265050",
      use_template_workflow: true,
      change_note: "[QA-TRACKING-T5-" + Date.now() + "] create",
      proposed_t0_date: "2026-08-20",
      proposed_deadline_day_type: "CALENDAR_DAYS",
      proposed_deadline_days: 5,
      reviewer_membership_ids: ["m_102"]
    }
  });
  console.log("create", JSON.stringify({ status: create.status, err: create.body?.error, id: create.body?.proposal_id, keys: Object.keys(create.body||{}) }, null, 2));
})().catch(e => { console.error(e); process.exit(1); });
