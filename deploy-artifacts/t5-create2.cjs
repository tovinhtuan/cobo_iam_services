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
  return { token: data.session.access_token, mid: data.current_context.membership_id, cid: data.current_context.company_id };
}
async function api(token, path, opts = {}) {
  const headers = { Authorization: "Bearer " + token };
  if (opts.body) headers["Content-Type"] = "application/json";
  return fetchJson(BASE + path, { method: opts.method || "GET", headers, body: opts.body ? JSON.stringify(opts.body) : undefined });
}
(async () => {
  const admin = await login("admin.dn@example.com", "secret");
  const propose = await login("nhanvien@example.com", "secret");
  const body = {
    type_id: "dt-co-1783321973009265050",
    use_template_workflow: true,
    change_note: "[QA-TRACKING-T5-admin] create",
    proposed_t0_date: "2026-08-20",
    proposed_deadline_day_type: "CALENDAR_DAYS",
    proposed_deadline_days: 5,
    reviewer_membership_ids: ["m_102"]
  };
  // admin cannot self-review maybe - try m_103 as reviewer? admin has focal_review so maybe can self
  const aCreate = await api(admin.token, "/api/v1/company/ad-hoc-proposals", { method: "POST", body });
  console.log("adminCreate", aCreate.status, aCreate.body?.error || aCreate.body?.proposal_id);

  const pBody = { ...body, change_note: "[QA-TRACKING-T5-propose] create", reviewer_membership_ids: ["m_102"] };
  const pCreate = await api(propose.token, "/api/v1/company/ad-hoc-proposals", { method: "POST", body: pBody });
  console.log("proposeCreate", pCreate.status, pCreate.body?.error || pCreate.body?.proposal_id);

  // compare list authorize
  const pList = await api(propose.token, "/api/v1/company/ad-hoc-proposals?scope=my");
  console.log("proposeList", pList.status, (pList.body.items||[]).length);

  // authorize endpoint?
  const authz = await api(propose.token, "/api/v1/authorization/authorize", { method: "POST", body: { action: "ad_hoc_alert.propose", resource: { type: "ad_hoc_proposal" } } });
  console.log("authz", authz.status, JSON.stringify(authz.body).slice(0,300));
})().catch(e=>{console.error(e); process.exit(1);});
