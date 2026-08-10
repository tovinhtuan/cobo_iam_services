const fs = require("fs");
const crypto = require("crypto");
const BASE = "http://88.216.208.0:8080";

async function fetchJson(url, opts = {}) {
  const res = await fetch(url, opts);
  const text = await res.text();
  let body = null;
  try { body = text ? JSON.parse(text) : null; } catch { body = text; }
  return { status: res.status, body, text };
}

function encryptPassword(spkiB64, password) {
  const spki = Buffer.from(spkiB64, "base64");
  const key = crypto.createPublicKey({ key: spki, format: "der", type: "spki" });
  return crypto.publicEncrypt(
    { key, padding: crypto.constants.RSA_PKCS1_OAEP_PADDING, oaepHash: "sha256" },
    Buffer.from(password, "utf8")
  ).toString("base64");
}

async function login(email, password) {
  const keyRes = await fetchJson(BASE + "/api/v1/auth/login-password-key");
  const cipher = encryptPassword(keyRes.body.public_key_spki_b64, password);
  const r = await fetchJson(BASE + "/api/v1/auth/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      email,
      remember_me: false,
      password_cipher: { alg: keyRes.body.alg, kid: keyRes.body.kid, ciphertext_b64: cipher }
    })
  });
  const data = r.body.data || r.body;
  return {
    status: r.status,
    token: data.session?.access_token,
    company_id: data.current_context?.company_id,
    membership_id: data.current_context?.membership_id,
    next_action: data.next_action
  };
}

async function api(token, path, opts = {}) {
  const headers = Object.assign({ Authorization: "Bearer " + token }, opts.headers || {});
  if (opts.body && !headers["Content-Type"]) headers["Content-Type"] = "application/json";
  const r = await fetchJson(BASE + path, {
    method: opts.method || "GET",
    headers,
    body: opts.body ? JSON.stringify(opts.body) : undefined
  });
  return r;
}

function summarizeList(body) {
  const items = body?.data?.items || body?.data?.proposals || body?.items || [];
  return {
    count: Array.isArray(items) ? items.length : null,
    ids: Array.isArray(items) ? items.slice(0, 5).map(i => i.id || i.proposal_id) : [],
    created_bys: Array.isArray(items) ? [...new Set(items.map(i => i.created_by || i.createdBy))] : [],
    error: body?.error || null
  };
}

(async () => {
  const evidence = { markers: [], cases: {} };

  const admin = await login("admin.dn@example.com", "secret");
  const propose = await login("nhanvien@example.com", "secret");
  evidence.fixtures = {
    admin: { email: "admin.dn@example.com", membership_id: admin.membership_id, company_id: admin.company_id, login: admin.status },
    proposeOnly: { email: "nhanvien@example.com", membership_id: propose.membership_id, company_id: propose.company_id, login: propose.status }
  };

  // effective access permissions
  const adminEA = await api(admin.token, "/api/v1/me/effective-access");
  const proposeEA = await api(propose.token, "/api/v1/me/effective-access");
  const pickPerms = (body) => {
    const perms = body?.data?.permissions || body?.permissions || [];
    return (Array.isArray(perms) ? perms : []).filter(p => String(p).includes("ad_hoc"));
  };
  evidence.fixtures.admin.adHocPerms = pickPerms(adminEA.body);
  evidence.fixtures.proposeOnly.adHocPerms = pickPerms(proposeEA.body);

  // Propose-only: company list without scope
  const pNoScope = await api(propose.token, "/api/v1/company/ad-hoc-proposals?page=1&page_size=5");
  evidence.cases.proposeOnlyCompanyList = { status: pNoScope.status, error: pNoScope.body?.error?.code || pNoScope.body?.error || null, summary: summarizeList(pNoScope.body) };

  // Propose-only: scope=my
  const pMy = await api(propose.token, "/api/v1/company/ad-hoc-proposals?scope=my&page=1&page_size=20");
  evidence.cases.proposeOnlyScopeMy = { status: pMy.status, summary: summarizeList(pMy.body), error: pMy.body?.error || null };

  // Admin create a proposal as other creator for security test (if create API known)
  // First list admin company proposals
  const aList = await api(admin.token, "/api/v1/company/ad-hoc-proposals?page=1&page_size=5");
  evidence.cases.readCapableCompanyList = { status: aList.status, summary: summarizeList(aList.body) };

  // Find an existing admin-owned proposal OR use first from list
  let otherProposalId = (aList.body?.data?.items || aList.body?.data?.proposals || aList.body?.items || [])[0]?.id;

  // Propose-only detail own: create draft+submit if needed
  // Probe create endpoint
  const ts = Date.now();
  const createBodyCandidates = [
    { title: `[QA-TRACKING-T5-${ts}] propose-only draft`, content: "T5 auth smoke", event_date: "2026-08-15", deadline_day_type: "calendar" },
    { title: `[QA-TRACKING-T5-${ts}] propose-only draft`, description: "T5 auth smoke" }
  ];

  let created = null;
  for (const body of createBodyCandidates) {
    const r = await api(propose.token, "/api/v1/company/ad-hoc-proposals", { method: "POST", body });
    evidence.cases.createProbe = evidence.cases.createProbe || [];
    evidence.cases.createProbe.push({ status: r.status, error: r.body?.error || null, keys: r.body?.data ? Object.keys(r.body.data) : Object.keys(r.body||{}) });
    if (r.status === 200 || r.status === 201) { created = r.body?.data || r.body; break; }
  }

  let ownId = created?.id || created?.proposal?.id;
  if (ownId) {
    const ownDetail = await api(propose.token, `/api/v1/company/ad-hoc-proposals/${ownId}`);
    evidence.cases.proposeOnlyOwnDetail = {
      status: ownDetail.status,
      hasTracking: Boolean(ownDetail.body?.data?.tracking || ownDetail.body?.tracking),
      trackingKeys: Object.keys(ownDetail.body?.data?.tracking || ownDetail.body?.tracking || {}),
      statusField: ownDetail.body?.data?.status || ownDetail.body?.status,
      error: ownDetail.body?.error || null
    };
  }

  if (otherProposalId && otherProposalId !== ownId) {
    const otherDetail = await api(propose.token, `/api/v1/company/ad-hoc-proposals/${otherProposalId}`);
    evidence.cases.otherCreatorDenied = {
      status: otherDetail.status,
      errorCode: otherDetail.body?.error?.code || null,
      leakedTracking: Boolean(otherDetail.body?.data?.tracking || otherDetail.body?.tracking),
      leakedStatus: otherDetail.body?.data?.status || otherDetail.body?.status || null
    };
  } else {
    evidence.cases.otherCreatorDenied = { status: null, note: "no other proposal id available yet" };
  }

  // Cross-company: try random other company proposal if we can find one via SQL-backed id unknown — mark later
  evidence.cases.crossTenant = { status: "PENDING_FIXTURE" };

  // Markers
  if (evidence.cases.proposeOnlyCompanyList.status === 403 || evidence.cases.proposeOnlyCompanyList.status === 401) {
    evidence.markers.push("PROPOSE_ONLY_COMPANY_LIST_DENIED");
  }
  if (evidence.cases.proposeOnlyScopeMy.status === 200) evidence.markers.push("SCOPE_MY_OK");
  if (evidence.cases.proposeOnlyOwnDetail?.status === 200) evidence.markers.push("SELF_DETAIL_OK");
  if (evidence.cases.otherCreatorDenied?.status === 403) evidence.markers.push("OTHER_CREATOR_DENIED");
  if (evidence.cases.readCapableCompanyList.status === 200) evidence.markers.push("READ_CAPABLE_LIST_OK");
  if ((evidence.fixtures.proposeOnly.adHocPerms || []).includes("ad_hoc_alert.propose") && !(evidence.fixtures.proposeOnly.adHocPerms || []).includes("ad_hoc_alert.read")) {
    evidence.markers.push("PROPOSE_ONLY_FIXTURE_OK");
  }

  fs.writeFileSync("deploy-artifacts/t5-auth-smoke-raw.json", JSON.stringify(evidence, null, 2));
  console.log(JSON.stringify(evidence, null, 2));
})().catch(e => { console.error(e); process.exit(1); });
