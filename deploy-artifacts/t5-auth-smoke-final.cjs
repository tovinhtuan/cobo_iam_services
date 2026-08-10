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
  return { token: data.session.access_token, mid: data.current_context.membership_id };
}
async function api(token, path, opts = {}) {
  const headers = Object.assign({ Authorization: "Bearer " + token }, opts.headers || {});
  if (opts.body) headers["Content-Type"] = "application/json";
  return fetchJson(BASE + path, { method: opts.method || "GET", headers, body: opts.body ? JSON.stringify(opts.body) : undefined });
}
(async () => {
  const admin = await login("admin.dn@example.com", "secret");
  const propose = await login("nhanvien@example.com", "secret");
  const out = {};
  const typeId = "dt-co-1783321973009265050";
  const ts = Date.now();
  const createBodies = [
    { type_id: typeId, use_template_workflow: true, change_note: `[QA-TRACKING-T5-${ts}]`, proposed_t0_date: "2026-08-20", proposed_deadline_day_type: "CALENDAR_DAYS", proposed_deadline_days: 5 },
    { type_id: typeId, use_template_workflow: true, change_note: `[QA-TRACKING-T5-${ts}]` },
  ];
  for (const body of createBodies) {
    const r = await api(propose.token, "/api/v1/company/ad-hoc-proposals", { method: "POST", body });
    out.createAttempts = out.createAttempts || [];
    out.createAttempts.push({ status: r.status, error: r.body?.error || null, id: r.body?.proposal_id || r.body?.data?.id || r.body?.id || null, keys: Object.keys(r.body || {}) });
    if (r.status === 200 || r.status === 201) { out.created = r.body; break; }
  }
  const ownId = out.created?.proposal_id || out.created?.id;
  if (ownId) {
    out.ownDetail = await (async () => {
      const r = await api(propose.token, `/api/v1/company/ad-hoc-proposals/${ownId}`);
      return { status: r.status, created_by: r.body.created_by, hasTracking: !!r.body.tracking, statusField: r.body.status };
    })();
    const my = await api(propose.token, "/api/v1/company/ad-hoc-proposals?scope=my&page=1&page_size=50");
    const items = my.body?.items || my.body?.data?.items || [];
    out.scopeMy = { status: my.status, count: items.length, includesOwn: items.some(i => i.proposal_id === ownId || i.id === ownId), foreign: items.filter(i => (i.created_by||i.createdBy) && (i.created_by||i.createdBy) !== propose.mid).length };
  }
  const otherId = "019feb53-7ae2-7386-b1af-b675810b5dd0";
  const denied = await api(propose.token, `/api/v1/company/ad-hoc-proposals/${otherId}`);
  out.otherCreator = { status: denied.status, code: denied.body?.error?.code, leaked: !!denied.body?.tracking || !!denied.body?.status && denied.status===200 };
  const crossId = "019fdc27-07e0-7ed4-a622-abb3e8f454b2";
  const cross = await api(propose.token, `/api/v1/company/ad-hoc-proposals/${crossId}`);
  out.crossTenant = { status: cross.status, code: cross.body?.error?.code, leakedTracking: !!cross.body?.tracking, leakedStatus: cross.body?.status && cross.status < 400 ? cross.body.status : null, hasBodyKeys: Object.keys(cross.body||{}) };
  // admin detail tracking pending
  const pending = await api(admin.token, `/api/v1/company/ad-hoc-proposals/${otherId}`);
  out.pendingTracking = { status: pending.status, has_runtime: pending.body?.tracking?.has_runtime, futureOnly: (pending.body?.tracking?.steps||[]).every(s => s.status === 'FUTURE'), approval: pending.body?.approval_progress };

  fs.writeFileSync("deploy-artifacts/t5-auth-smoke-final.json", JSON.stringify(out, null, 2));
  console.log(JSON.stringify(out, null, 2));
})().catch(e => { console.error(e); process.exit(1); });
