const fs = require("fs");
const crypto = require("crypto");
const BASE = "http://88.216.208.0:8080";

async function fetchJson(url, opts = {}) {
  const res = await fetch(url, opts);
  const text = await res.text();
  let body = null;
  try { body = text ? JSON.parse(text) : null; } catch { body = text; }
  return { status: res.status, body };
}
function encryptPassword(spkiB64, password) {
  const spki = Buffer.from(spkiB64, "base64");
  const key = crypto.createPublicKey({ key: spki, format: "der", type: "spki" });
  return crypto.publicEncrypt({ key, padding: crypto.constants.RSA_PKCS1_OAEP_PADDING, oaepHash: "sha256" }, Buffer.from(password, "utf8")).toString("base64");
}
async function login(email, password) {
  const keyRes = await fetchJson(BASE + "/api/v1/auth/login-password-key");
  const cipher = encryptPassword(keyRes.body.public_key_spki_b64, password);
  const r = await fetchJson(BASE + "/api/v1/auth/login", {
    method: "POST", headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ email, remember_me: false, password_cipher: { alg: keyRes.body.alg, kid: keyRes.body.kid, ciphertext_b64: cipher } })
  });
  const data = r.body.data || r.body;
  return { token: data.session.access_token, membership_id: data.current_context.membership_id, company_id: data.current_context.company_id };
}
async function api(token, path, opts = {}) {
  const headers = Object.assign({ Authorization: "Bearer " + token }, opts.headers || {});
  if (opts.body && !headers["Content-Type"]) headers["Content-Type"] = "application/json";
  return fetchJson(BASE + path, { method: opts.method || "GET", headers, body: opts.body ? JSON.stringify(opts.body) : undefined });
}

(async () => {
  const admin = await login("admin.dn@example.com", "secret");
  const propose = await login("nhanvien@example.com", "secret");
  const out = {};

  // other creator detail
  const otherId = "019feb53-7ae2-7386-b1af-b675810b5dd0";
  const denied = await api(propose.token, `/api/v1/company/ad-hoc-proposals/${otherId}`);
  out.otherCreator = {
    status: denied.status,
    code: denied.body?.error?.code || null,
    hasData: Boolean(denied.body?.data),
    leakedTracking: Boolean(denied.body?.data?.tracking),
    leakedKeys: denied.body?.data ? Object.keys(denied.body.data) : []
  };

  // admin own detail + tracking
  const adminDetail = await api(admin.token, `/api/v1/company/ad-hoc-proposals/${otherId}`);
  const d = adminDetail.body?.data || {};
  out.adminDetail = {
    status: adminDetail.status,
    proposalStatus: d.status,
    created_by: d.created_by,
    hasTracking: Boolean(d.tracking),
    tracking: d.tracking ? {
      current_step_code: d.tracking.current_step_code || d.tracking.currentStepCode,
      progress: d.tracking.progress || d.tracking.workflow_progress,
      stepsCount: (d.tracking.steps || []).length,
      currentAssignees: (d.tracking.current_assignees || d.tracking.currentAssignees || []).length,
      keys: Object.keys(d.tracking)
    } : null,
    type_id: d.type_id,
    title: d.title
  };

  // discover type_id from detail or list eligible types
  const types = await api(admin.token, "/api/v1/company/ad-hoc-alert-types");
  out.typesStatus = types.status;
  out.typesSample = (types.body?.data?.items || types.body?.data || types.body?.items || []).slice?.(0,3) || types.body;

  // try create for propose-only with type from admin detail
  const typeId = d.type_id;
  if (typeId) {
    const ts = Date.now();
    const create = await api(propose.token, "/api/v1/company/ad-hoc-proposals", {
      method: "POST",
      body: {
        type_id: typeId,
        title: `[QA-TRACKING-T5-${ts}] propose-only create`,
        content: "T5 smoke create",
        event_date: "2026-08-20",
        t0_date: "2026-08-20"
      }
    });
    out.create = { status: create.status, error: create.body?.error || null, id: create.body?.data?.id, keys: create.body?.data ? Object.keys(create.body.data) : [] };
    if (create.body?.data?.id) {
      const own = await api(propose.token, `/api/v1/company/ad-hoc-proposals/${create.body.data.id}`);
      out.ownDetail = {
        status: own.status,
        created_by: own.body?.data?.created_by,
        hasTracking: Boolean(own.body?.data?.tracking),
        trackingKeys: own.body?.data?.tracking ? Object.keys(own.body.data.tracking) : []
      };
      const myList = await api(propose.token, "/api/v1/company/ad-hoc-proposals?scope=my&page=1&page_size=20");
      const items = myList.body?.data?.items || [];
      out.scopeMyAfterCreate = {
        status: myList.status,
        count: items.length,
        includesOwn: items.some(i => i.id === create.body.data.id),
        foreignRows: items.filter(i => i.created_by && i.created_by !== propose.membership_id).length
      };
    }
  }

  // cross tenant: pick proposal from another company via DB id probe — try nonsense UUID in company context still same company
  // Use known other company proposal if we can query one
  out.note = "cross-tenant via SQL next";

  fs.writeFileSync("deploy-artifacts/t5-auth-smoke2.json", JSON.stringify(out, null, 2));
  console.log(JSON.stringify(out, null, 2));
})().catch(e => { console.error(e); process.exit(1); });
