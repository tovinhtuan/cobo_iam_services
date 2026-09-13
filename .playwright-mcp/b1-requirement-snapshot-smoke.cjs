/**
 * B1 DEV smoke: create one new disclosure+workflow instance and prove requirement snapshots.
 * Read-only beyond creating one QA record. No upload/complete gate.
 */
const BASE = process.env.COBO_API_BASE || 'http://88.216.208.0:8080';
const { execSync } = require('child_process');
const path = require('path');
const fs = require('fs');

async function jfetch(url, opts = {}) {
  const res = await fetch(url, opts);
  const text = await res.text();
  let body;
  try {
    body = JSON.parse(text);
  } catch {
    body = text;
  }
  return { status: res.status, body };
}

function sshMysql(sql) {
  const cmd = `ssh -p 21239 -i "${process.env.USERPROFILE}\\.ssh\\id_ed25519" -o BatchMode=yes root@88.216.208.0 "docker exec cobo-iam-mysql mysql -uroot -proot cobo_iam -Nse \\"${sql.replace(/"/g, '\\"')}\\""`;
  try {
    return execSync(cmd, { encoding: 'utf8', stdio: ['pipe', 'pipe', 'pipe'] })
      .split(/\r?\n/)
      .map((l) => l.trim())
      .filter((l) => l && !/Warning/.test(l));
  } catch (e) {
    return { error: String(e.stderr || e.message || e) };
  }
}

(async () => {
  const out = {
    DEV_SNAPSHOT_TABLE_EXISTS: 'FAIL',
    DEV_LEGACY_INSTANCE_NO_BACKFILL: 'FAIL',
    DEV_NEW_INSTANCE_SNAPSHOT_CASE: 'FAIL',
    DEV_SNAPSHOT_FIELD_PARITY: 'FAIL',
    DEV_COMPANY_OVERRIDE_SNAPSHOT: 'NOT_AVAILABLE',
    DEV_TEMPLATE_CHANGE_IMMUTABILITY: 'NOT_AVAILABLE',
    DEV_REQUIRED_DOCUMENT_GATE_ACTIVE: false,
    BE_HEALTH: 'FAIL',
    WORKER_HEALTH: 'FAIL',
    API_UNEXPECTED_5XX: 0,
  };

  const health = await jfetch(`${BASE}/healthz`);
  const ready = await jfetch(`${BASE}/readyz`);
  out.BE_HEALTH = health.status === 200 ? 'PASS' : 'FAIL';
  out.WORKER_HEALTH = ready.status === 200 ? 'PASS' : 'FAIL'; // proxy of stack readiness

  const tables = sshMysql("SHOW TABLES LIKE 'workflow_step_document_requirement_snapshots'");
  out.DEV_SNAPSHOT_TABLE_EXISTS = Array.isArray(tables) && tables.includes('workflow_step_document_requirement_snapshots') ? 'PASS' : 'FAIL';

  const legacyCountBefore = sshMysql('SELECT COUNT(*) FROM workflow_step_document_requirement_snapshots');
  const legacyInstances = sshMysql('SELECT COUNT(*) FROM workflow_instances');
  // Pre-B1 instances should not have been backfilled: total snapshot rows for old instances = 0 before new create
  // After migration only new creates add rows. If count was 0 and we have instances, legacy OK.
  if (Array.isArray(legacyCountBefore) && Array.isArray(legacyInstances) && Number(legacyCountBefore[0]) === 0 && Number(legacyInstances[0]) > 0) {
    out.DEV_LEGACY_INSTANCE_NO_BACKFILL = 'PASS';
  } else if (Array.isArray(legacyCountBefore) && Number(legacyCountBefore[0]) === 0) {
    out.DEV_LEGACY_INSTANCE_NO_BACKFILL = 'PASS';
  }

  const login = await jfetch(`${BASE}/api/v1/auth/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ login_id: 'admin.dn@example.com', password: 'secret' }),
  });
  const pre = login.body?.session?.pre_company_token || login.body?.pre_company_token;
  if (!pre) {
    out.error = 'login failed';
    console.log(JSON.stringify({ out, login: login.status }, null, 2));
    process.exit(1);
  }
  const sel = await jfetch(`${BASE}/api/v1/auth/select-company`, {
    method: 'POST',
    headers: { Authorization: `Bearer ${pre}`, 'Content-Type': 'application/json' },
    body: JSON.stringify({ company_id: 'c_001', membership_id: 'm_102' }),
  });
  let token = sel.body?.session?.access_token || sel.body?.access_token;
  if (!token) {
    out.error = 'select-company failed';
    console.log(JSON.stringify({ out, sel }, null, 2));
    process.exit(1);
  }

  const auth = { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' };

  // Find a type with documents in effective workflow
  const types = await jfetch(`${BASE}/api/v1/disclosure-types?limit=50`, { headers: auth });
  const typeList = types.body?.data || types.body?.items || types.body || [];
  let chosen = null;
  let effDocs = [];
  for (const t of Array.isArray(typeList) ? typeList : []) {
    const typeId = t.type_id || t.id;
    if (!typeId) continue;
    const eff = await jfetch(`${BASE}/api/v1/disclosure-types/${encodeURIComponent(typeId)}/effective-workflow`, {
      headers: auth,
    });
    if (eff.status >= 500) out.API_UNEXPECTED_5XX++;
    const steps = eff.body?.data?.workflow || eff.body?.workflow || [];
    const docs = [];
    for (const s of steps) {
      for (const d of s.documents || []) {
        docs.push({ step_id: s.step_id, ...d });
      }
    }
    if (docs.some((d) => d.required) && docs.some((d) => !d.required)) {
      chosen = { typeId, steps, docs, source: eff.body?.data?.source };
      effDocs = docs;
      break;
    }
    if (!chosen && docs.length > 0) {
      chosen = { typeId, steps, docs, source: eff.body?.data?.source };
      effDocs = docs;
    }
  }

  if (!chosen) {
    out.DEV_NEW_INSTANCE_SNAPSHOT_CASE = 'NOT_AVAILABLE';
    out.DEV_SNAPSHOT_FIELD_PARITY = 'NOT_AVAILABLE';
    out.note = 'No type with documents found for safe create';
    console.log(JSON.stringify(out, null, 2));
    process.exit(out.DEV_SNAPSHOT_TABLE_EXISTS === 'PASS' && out.DEV_LEGACY_INSTANCE_NO_BACKFILL === 'PASS' ? 0 : 1);
  }

  const title = `B1 snapshot smoke ${new Date().toISOString()}`;
  const create = await jfetch(`${BASE}/api/v1/disclosures`, {
    method: 'POST',
    headers: auth,
    body: JSON.stringify({
      type_id: chosen.typeId,
      title,
      content: title,
    }),
  });
  if (create.status >= 500) out.API_UNEXPECTED_5XX++;
  const recordId = create.body?.data?.record_id || create.body?.record_id || create.body?.data?.id;
  if (!recordId) {
    out.create = { status: create.status, body: create.body };
    console.log(JSON.stringify(out, null, 2));
    process.exit(1);
  }

  const submit = await jfetch(`${BASE}/api/v1/disclosures/${recordId}/submit`, {
    method: 'POST',
    headers: auth,
    body: JSON.stringify({}),
  });
  if (submit.status >= 500) out.API_UNEXPECTED_5XX++;

  // Wait briefly for workflow bootstrap
  await new Promise((r) => setTimeout(r, 1500));

  const snapRows = sshMysql(
    `SELECT step_code, source_doc_id, name, required, IFNULL(template_file_id,''), ordinal FROM workflow_step_document_requirement_snapshots WHERE disclosure_record_id='${recordId}' ORDER BY step_code, ordinal`,
  );

  if (!Array.isArray(snapRows) || snapRows.length === 0) {
    out.DEV_NEW_INSTANCE_SNAPSHOT_CASE = 'FAIL';
    out.snapRows = snapRows;
    out.submit = { status: submit.status, body: submit.body };
    out.recordId = recordId;
    console.log(JSON.stringify(out, null, 2));
    process.exit(1);
  }

  out.DEV_NEW_INSTANCE_SNAPSHOT_CASE = 'PASS';
  out.recordId = recordId;
  out.snapshot_row_count = snapRows.length;
  out.effective_doc_count = effDocs.length;

  // Field parity: every effective doc should appear
  const parsed = snapRows.map((line) => {
    const [step_code, source_doc_id, name, required, template_file_id, ordinal] = line.split('\t');
    return { step_code, source_doc_id, name, required: required === '1', template_file_id, ordinal: Number(ordinal) };
  });
  let parity = true;
  for (const d of effDocs) {
    const hit = parsed.find((p) => p.source_doc_id === d.doc_id && p.step_code === d.step_id);
    if (!hit || hit.name !== (d.name || '').trim() || hit.required !== !!d.required) {
      parity = false;
      break;
    }
  }
  if (parsed.length !== effDocs.length) parity = false;
  out.DEV_SNAPSHOT_FIELD_PARITY = parity ? 'PASS' : 'FAIL';
  out.parsed = parsed;
  out.effDocsSample = effDocs.slice(0, 5);

  // Automated immutability already covered in unit tests
  out.DEV_TEMPLATE_CHANGE_IMMUTABILITY = 'NOT_AVAILABLE';
  out.DEV_COMPANY_OVERRIDE_SNAPSHOT = chosen.source === 'company_override' ? (parity ? 'PASS' : 'FAIL') : 'NOT_AVAILABLE';

  console.log(JSON.stringify(out, null, 2));
  const ok =
    out.DEV_SNAPSHOT_TABLE_EXISTS === 'PASS' &&
    out.DEV_LEGACY_INSTANCE_NO_BACKFILL === 'PASS' &&
    out.DEV_NEW_INSTANCE_SNAPSHOT_CASE === 'PASS' &&
    out.DEV_SNAPSHOT_FIELD_PARITY === 'PASS' &&
    out.BE_HEALTH === 'PASS' &&
    out.API_UNEXPECTED_5XX === 0;
  process.exit(ok ? 0 : 1);
})().catch((e) => {
  console.error(e);
  process.exit(1);
});
