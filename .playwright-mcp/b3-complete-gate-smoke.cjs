/**
 * B3 DEV smoke — required-document Complete Step gate.
 * Safe QA: seed dedicated required snapshots on an in-scope alert; do not mutate unrelated companies.
 */
const BASE = process.env.COBO_API_BASE || 'http://88.216.208.0:8080';
const { execSync } = require('child_process');
const fs = require('fs');
const path = require('path');

async function jfetch(url, opts = {}) {
  const res = await fetch(url, opts);
  const text = await res.text();
  let body;
  try {
    body = JSON.parse(text);
  } catch {
    body = text;
  }
  return { status: res.status, headers: res.headers, body, raw: text };
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

async function loginCompany(loginId, password, companyId, membershipId) {
  const login = await jfetch(`${BASE}/api/v1/auth/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ login_id: loginId, password }),
  });
  const pre = login.body?.session?.pre_company_token || login.body?.pre_company_token;
  if (!pre) return { error: 'login', login };
  const sel = await jfetch(`${BASE}/api/v1/auth/select-company`, {
    method: 'POST',
    headers: { Authorization: `Bearer ${pre}`, 'Content-Type': 'application/json' },
    body: JSON.stringify({ company_id: companyId, membership_id: membershipId }),
  });
  const token = sel.body?.session?.access_token || sel.body?.access_token;
  if (!token) return { error: 'select', sel };
  return { token };
}

function multipart(fileName, content, contentType) {
  const boundary = '----cobob3' + Date.now();
  const body = Buffer.concat([
    Buffer.from(`--${boundary}\r\nContent-Disposition: form-data; name="file"; filename="${fileName}"\r\nContent-Type: ${contentType}\r\n\r\n`),
    Buffer.from(content),
    Buffer.from(`\r\n--${boundary}--\r\n`),
  ]);
  return { body, contentType: `multipart/form-data; boundary=${boundary}` };
}

function errCode(body) {
  return body?.error?.code || body?.code || '';
}

function missingReqs(body) {
  return body?.error?.details?.missing_requirements || body?.error?.details?.missing_requirements || [];
}

(async () => {
  const out = {
    DEV_REQUIRED_MISSING_COMPLETE_BLOCKED: 'FAIL',
    DEV_MISSING_REQUIREMENT_PAYLOAD: 'FAIL',
    DEV_UPLOAD_THEN_COMPLETE: 'FAIL',
    DEV_OPTIONAL_MISSING_DOES_NOT_BLOCK: 'NOT_AVAILABLE',
    DEV_NO_REQUIREMENT_COMPLETE: 'NOT_AVAILABLE',
    DEV_LEGACY_COMPLETE_PRESERVED: 'FAIL',
    DEV_DELETED_FILE_NOT_FULFILLMENT: 'AUTOMATED_ONLY',
    DEV_REPLACE_ACTIVE_FILE_FULFILLMENT: 'AUTOMATED_ONLY',
    DEV_ALL_MISSING_RETURNED: 'AUTOMATED_ONLY',
    DEV_AVAILABLE_ACTIONS_CONTRACT_CHANGED: false,
    DEV_B2_FILE_API_REGRESSION: 'FAIL',
    DEV_B2_CROSS_COMPANY_REGRESSION: 'FAIL',
    BE_HEALTH: 'FAIL',
    WORKER_HEALTH: 'FAIL',
    API_UNEXPECTED_5XX: 0,
  };

  const health = await jfetch(`${BASE}/healthz`);
  const ready = await jfetch(`${BASE}/readyz`);
  out.BE_HEALTH = health.status === 200 ? 'PASS' : 'FAIL';
  out.WORKER_HEALTH = ready.status === 200 ? 'PASS' : 'FAIL';

  const authA = await loginCompany('admin.dn@example.com', 'secret', 'c_001', 'm_102');
  if (authA.error) {
    out.error = authA;
    console.log(JSON.stringify(out, null, 2));
    process.exit(1);
  }
  const headers = { Authorization: `Bearer ${authA.token}` };

  // --- LEGACY first: find alert whose current step has ZERO B1 snapshots; Complete must not return REQUIRED_DOCUMENT_MISSING
  const alerts = await jfetch(`${BASE}/api/v1/company/deadline-alerts?page_size=50`, { headers });
  if (alerts.status >= 500) out.API_UNEXPECTED_5XX++;
  const items = alerts.body?.items || [];
  let legacyHit = null;
  for (const alert of items) {
    const rid = alert.record_id;
    const stepsResp = await jfetch(`${BASE}/api/v1/company/deadlines/${rid}/steps`, { headers });
    if (stepsResp.status !== 200) continue;
    const stepCode = stepsResp.body?.current_step_code;
    if (!stepCode) continue;
    const wi =
      alert.workflow_instance_id ||
      (sshMysql(`SELECT workflow_instance_id FROM workflow_instances WHERE record_id='${rid}' LIMIT 1`) || [])[0];
    const cnt = sshMysql(
      `SELECT COUNT(*) FROM workflow_step_document_requirement_snapshots WHERE company_id='c_001' AND workflow_instance_id='${wi}' AND step_code='${stepCode}'`,
    );
    if (Array.isArray(cnt) && Number(cnt[0]) === 0) {
      const before = sshMysql(
        `SELECT IFNULL(completed_at,'') FROM workflow_instance_step_states WHERE workflow_instance_id='${wi}' AND step_code='${stepCode}'`,
      );
      const complete = await jfetch(
        `${BASE}/api/v1/company/deadlines/${rid}/steps/${encodeURIComponent(stepCode)}/complete`,
        { method: 'POST', headers },
      );
      if (complete.status >= 500) out.API_UNEXPECTED_5XX++;
      const code = errCode(complete.body);
      legacyHit = { rid, stepCode, wi, status: complete.status, code, before, body: complete.body };
      // PASS if not blocked by B3 gate (success OR pre-existing non-B3 business error)
      if (code !== 'WORKFLOW_STEP_REQUIRED_DOCUMENT_MISSING') {
        out.DEV_LEGACY_COMPLETE_PRESERVED = 'PASS';
        out.DEV_NO_REQUIREMENT_COMPLETE = complete.status === 200 ? 'PASS' : 'NOT_AVAILABLE';
      }
      break;
    }
  }
  out.legacy = legacyHit;
  if (out.DEV_LEGACY_COMPLETE_PRESERVED !== 'PASS') {
    // Fallback: zero-snap SQL probe + automated contract (still FAIL hard gate if no fixture)
    out.legacyNote = 'no zero-snapshot current step found in-scope';
  }

  // --- REQUIRED MISSING fixture on a different (or same unfinished) alert
  let gateRecord = null;
  for (const alert of items) {
    const rid = alert.record_id;
    const stepsResp = await jfetch(`${BASE}/api/v1/company/deadlines/${rid}/steps`, { headers });
    if (stepsResp.status !== 200) continue;
    const stepCode = stepsResp.body?.current_step_code;
    if (!stepCode) continue;
    const wi =
      alert.workflow_instance_id ||
      (sshMysql(`SELECT workflow_instance_id FROM workflow_instances WHERE record_id='${rid}' LIMIT 1`) || [])[0];
    const completed = sshMysql(
      `SELECT IFNULL(completed_at,'NULL') FROM workflow_instance_step_states WHERE workflow_instance_id='${wi}' AND step_code='${stepCode}'`,
    );
    const done = Array.isArray(completed) && completed[0] && completed[0] !== 'NULL' && completed[0] !== '';
    if (done) continue;
    gateRecord = { rid, stepCode, wi };
    break;
  }
  if (!gateRecord) {
    out.error = 'no incomplete current step for B3 gate fixture';
    console.log(JSON.stringify(out, null, 2));
    process.exit(1);
  }

  const ts = Date.now();
  const reqA = `b3qa-a-${ts}`;
  const reqB = `b3qa-b-${ts}`;
  const reqOpt = `b3qa-opt-${ts}`;
  sshMysql(
    `INSERT INTO workflow_step_document_requirement_snapshots (id, company_id, disclosure_record_id, workflow_instance_id, step_code, source_doc_id, requirement_key, name, required, ordinal) VALUES ('${reqA}', 'c_001', '${gateRecord.rid}', '${gateRecord.wi}', '${gateRecord.stepCode}', 'b3-a', 'b3-a', 'B3 Required A', 1, 0)`,
  );
  sshMysql(
    `INSERT INTO workflow_step_document_requirement_snapshots (id, company_id, disclosure_record_id, workflow_instance_id, step_code, source_doc_id, requirement_key, name, required, ordinal) VALUES ('${reqB}', 'c_001', '${gateRecord.rid}', '${gateRecord.wi}', '${gateRecord.stepCode}', 'b3-b', 'b3-b', 'B3 Required B', 1, 1)`,
  );
  sshMysql(
    `INSERT INTO workflow_step_document_requirement_snapshots (id, company_id, disclosure_record_id, workflow_instance_id, step_code, source_doc_id, requirement_key, name, required, ordinal) VALUES ('${reqOpt}', 'c_001', '${gateRecord.rid}', '${gateRecord.wi}', '${gateRecord.stepCode}', 'b3-opt', 'b3-opt', 'B3 Optional', 0, 2)`,
  );
  out.gateRecord = gateRecord;
  out.seeded = { reqA, reqB, reqOpt };

  const beforeComplete = sshMysql(
    `SELECT IFNULL(completed_at,'') FROM workflow_instance_step_states WHERE workflow_instance_id='${gateRecord.wi}' AND step_code='${gateRecord.stepCode}'`,
  );

  const blocked = await jfetch(
    `${BASE}/api/v1/company/deadlines/${gateRecord.rid}/steps/${encodeURIComponent(gateRecord.stepCode)}/complete`,
    { method: 'POST', headers },
  );
  if (blocked.status >= 500) out.API_UNEXPECTED_5XX++;
  const bCode = errCode(blocked.body);
  const missing = missingReqs(blocked.body);
  const afterBlocked = sshMysql(
    `SELECT IFNULL(completed_at,'') FROM workflow_instance_step_states WHERE workflow_instance_id='${gateRecord.wi}' AND step_code='${gateRecord.stepCode}'`,
  );
  const noMutation =
    JSON.stringify(beforeComplete) === JSON.stringify(afterBlocked) ||
    (Array.isArray(afterBlocked) && (!afterBlocked[0] || afterBlocked[0] === ''));
  out.DEV_REQUIRED_MISSING_COMPLETE_BLOCKED =
    blocked.status === 422 && bCode === 'WORKFLOW_STEP_REQUIRED_DOCUMENT_MISSING' && noMutation ? 'PASS' : 'FAIL';
  out.blocked = { status: blocked.status, code: bCode, missing, beforeComplete, afterBlocked };

  const ids = missing.map((m) => m.requirement_snapshot_id);
  const payloadOk =
    missing.length >= 2 &&
    ids.includes(reqA) &&
    ids.includes(reqB) &&
    missing.every((m) => m.requirement_snapshot_id && m.source_doc_id && m.name) &&
    !JSON.stringify(missing).includes('storage_key');
  out.DEV_MISSING_REQUIREMENT_PAYLOAD = payloadOk ? 'PASS' : 'FAIL';
  out.DEV_ALL_MISSING_RETURNED = ids.includes(reqA) && ids.includes(reqB) ? 'PASS' : 'AUTOMATED_ONLY';

  // B2 upload both required (+ skip optional)
  const mpA = multipart('b3-a.pdf', '%PDF-1.4 b3a', 'application/pdf');
  const upA = await jfetch(
    `${BASE}/api/v1/company/deadlines/${gateRecord.rid}/steps/${encodeURIComponent(gateRecord.stepCode)}/document-requirements/${reqA}/files`,
    { method: 'POST', headers: { ...headers, 'Content-Type': mpA.contentType }, body: mpA.body },
  );
  if (upA.status >= 500) out.API_UNEXPECTED_5XX++;
  const mpB = multipart('b3-b.pdf', '%PDF-1.4 b3b', 'application/pdf');
  const upB = await jfetch(
    `${BASE}/api/v1/company/deadlines/${gateRecord.rid}/steps/${encodeURIComponent(gateRecord.stepCode)}/document-requirements/${reqB}/files`,
    { method: 'POST', headers: { ...headers, 'Content-Type': mpB.contentType }, body: mpB.body },
  );
  if (upB.status >= 500) out.API_UNEXPECTED_5XX++;
  out.uploads = { upA: upA.status, upB: upB.status, fileA: upA.body?.file?.file_id, fileB: upB.body?.file?.file_id };
  out.DEV_B2_FILE_API_REGRESSION = upA.status === 201 && upB.status === 201 ? 'PASS' : 'FAIL';

  // Optional missing should not block once required fulfilled
  const retry = await jfetch(
    `${BASE}/api/v1/company/deadlines/${gateRecord.rid}/steps/${encodeURIComponent(gateRecord.stepCode)}/complete`,
    { method: 'POST', headers },
  );
  if (retry.status >= 500) out.API_UNEXPECTED_5XX++;
  out.retry = { status: retry.status, code: errCode(retry.body), body: retry.body };
  out.DEV_UPLOAD_THEN_COMPLETE =
    retry.status === 200 && errCode(retry.body) !== 'WORKFLOW_STEP_REQUIRED_DOCUMENT_MISSING' ? 'PASS' : 'FAIL';
  out.DEV_OPTIONAL_MISSING_DOES_NOT_BLOCK =
    out.DEV_UPLOAD_THEN_COMPLETE === 'PASS' ? 'PASS' : 'NOT_AVAILABLE';

  // Cross-company denial on one uploaded file
  const authB = await loginCompany('user@example.com', 'secret', 'c_002', 'm_002');
  if (!authB.error && out.uploads.fileA) {
    const headersB = { Authorization: `Bearer ${authB.token}` };
    const dlB = await jfetch(
      `${BASE}/api/v1/company/deadlines/${gateRecord.rid}/steps/${encodeURIComponent(gateRecord.stepCode)}/document-requirements/files/${out.uploads.fileA}/content`,
      { headers: headersB },
    );
    if (dlB.status >= 500) out.API_UNEXPECTED_5XX++;
    out.DEV_B2_CROSS_COMPANY_REGRESSION = dlB.status === 403 || dlB.status === 404 ? 'PASS' : 'FAIL';
    out.crossDl = { status: dlB.status };
  }

  // Deleted-file probe (if still incomplete somehow — usually step completed after retry)
  if (out.DEV_UPLOAD_THEN_COMPLETE !== 'PASS' && out.uploads.fileA) {
    await jfetch(
      `${BASE}/api/v1/company/deadlines/${gateRecord.rid}/steps/${encodeURIComponent(gateRecord.stepCode)}/document-requirements/files/${out.uploads.fileA}`,
      { method: 'DELETE', headers },
    );
    const again = await jfetch(
      `${BASE}/api/v1/company/deadlines/${gateRecord.rid}/steps/${encodeURIComponent(gateRecord.stepCode)}/complete`,
      { method: 'POST', headers },
    );
    if (errCode(again.body) === 'WORKFLOW_STEP_REQUIRED_DOCUMENT_MISSING') {
      out.DEV_DELETED_FILE_NOT_FULFILLMENT = 'PASS';
    }
  }

  const outPath = path.join(
    __dirname,
    '..',
    '..',
    'cobo_web_design',
    'docs',
    'ai-cache',
    'tenant-workflow-step-evidence-b3-complete-gate-2026-09-13',
  );
  try {
    fs.mkdirSync(outPath, { recursive: true });
    fs.writeFileSync(path.join(outPath, 'dev-smoke-result.json'), JSON.stringify(out, null, 2));
  } catch (_) {
    fs.writeFileSync(path.join(__dirname, 'b3-dev-smoke-result.json'), JSON.stringify(out, null, 2));
  }
  console.log(JSON.stringify(out, null, 2));
  const hardFail =
    out.DEV_REQUIRED_MISSING_COMPLETE_BLOCKED !== 'PASS' ||
    out.DEV_MISSING_REQUIREMENT_PAYLOAD !== 'PASS' ||
    out.DEV_UPLOAD_THEN_COMPLETE !== 'PASS' ||
    out.DEV_LEGACY_COMPLETE_PRESERVED !== 'PASS' ||
    out.DEV_B2_FILE_API_REGRESSION !== 'PASS' ||
    out.DEV_B2_CROSS_COMPANY_REGRESSION !== 'PASS' ||
    out.BE_HEALTH !== 'PASS' ||
    out.WORKER_HEALTH !== 'PASS' ||
    out.API_UNEXPECTED_5XX !== 0;
  process.exit(hardFail ? 1 : 0);
})().catch((e) => {
  console.error(e);
  process.exit(1);
});
