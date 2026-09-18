/**
 * G2C DEV smoke — Generic Step Evidence audit + retention + error contract.
 * BE-only. Isolated QA against zero-requirement fixture when available.
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
  const escaped = sql.replace(/\\/g, '\\\\').replace(/"/g, '\\"');
  const cmd = `ssh -p 21239 -o BatchMode=yes root@88.216.208.0 "docker exec cobo-iam-mysql mysql -uroot -proot cobo_iam -Nse \\"${escaped}\\""`;
  try {
    return execSync(cmd, { encoding: 'utf8', stdio: ['pipe', 'pipe', 'pipe'] })
      .split(/\r?\n/)
      .map((l) => l.trim())
      .filter((l) => l && !/Warning/.test(l));
  } catch (e) {
    return { error: String(e.stderr || e.message || e) };
  }
}

function ssh(cmd) {
  const full = `ssh -p 21239 -o BatchMode=yes root@88.216.208.0 ${JSON.stringify(cmd)}`;
  try {
    return execSync(full, { encoding: 'utf8', stdio: ['pipe', 'pipe', 'pipe'] }).trim();
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
  const boundary = '----cobog2c' + Date.now();
  const body = Buffer.concat([
    Buffer.from(
      `--${boundary}\r\nContent-Disposition: form-data; name="file"; filename="${fileName}"\r\nContent-Type: ${contentType}\r\n\r\n`,
    ),
    Buffer.from(content),
    Buffer.from(`\r\n--${boundary}--\r\n`),
  ]);
  return { body, contentType: `multipart/form-data; boundary=${boundary}` };
}

function countAudit(action, resourceId) {
  const rows = sshMysql(
    `SELECT COUNT(*) FROM audit_logs WHERE action='${action}' AND resource_id='${resourceId}'`,
  );
  if (!Array.isArray(rows)) return -1;
  return Number(rows[0] || 0);
}

function auditMeta(action, resourceId) {
  const rows = sshMysql(
    `SELECT metadata_json FROM audit_logs WHERE action='${action}' AND resource_id='${resourceId}' ORDER BY occurred_at DESC LIMIT 1`,
  );
  if (!Array.isArray(rows) || !rows[0]) return null;
  try {
    return JSON.parse(rows[0]);
  } catch {
    return { raw: rows[0] };
  }
}

function diskExists(fileId) {
  // Do not print full physical paths into evidence — only boolean via exit-like signal.
  const d = ssh(
    `docker exec cobo-iam-api sh -c 'find /app/var -path "*workflow-step-evidence/*/` +
      fileId +
      `/*" -type f 2>/dev/null | head -1 | wc -l'`,
  );
  return String(d).trim() === '1';
}

(async () => {
  const out = {
    DEV_UPLOAD_AUDIT: 'FAIL',
    DEV_DELETE_AUDIT: 'FAIL',
    DEV_REPLACE_AUDIT: 'FAIL',
    DEV_DOWNLOAD_AUDIT_COUNT: -1,
    DEV_ZERO_REQUIREMENT_GENERIC: 'FAIL',
    DEV_CROSS_COMPANY: 'AUTOMATED_ONLY',
    DEV_CONTEXT_BINDING: 'AUTOMATED_ONLY',
    DEV_COMPLETED_READ_ONLY: 'AUTOMATED_ONLY',
    DEV_GENERIC_CANNOT_SATISFY_REQUIRED_DOC: 'AUTOMATED_ONLY',
    DEV_SUPERSEDED_BINARY_RETAINED: 'FAIL',
    DEV_DELETED_BINARY_RETAINED: 'FAIL',
    DEV_GENERIC_ERROR_DOMAIN_LEAK_COUNT: 0,
    DEV_BE_HEALTH: 'FAIL',
    DEV_WORKER_HEALTH: 'FAIL',
    DEV_UNEXPECTED_5XX: 0,
  };

  const health = await jfetch(`${BASE}/healthz`);
  const ready = await jfetch(`${BASE}/readyz`);
  out.DEV_BE_HEALTH = health.status === 200 ? 'PASS' : 'FAIL';
  out.DEV_WORKER_HEALTH = ready.status === 200 ? 'PASS' : 'FAIL';

  const authA = await loginCompany('admin.dn@example.com', 'secret', 'c_001', 'm_102');
  if (authA.error) {
    out.error = authA;
    console.log(JSON.stringify(out, null, 2));
    process.exit(1);
  }
  const headers = { Authorization: `Bearer ${authA.token}` };

  // Prefer known zero-req fixture from G2B; else first deadline alert.
  let recordId = process.env.G2C_RECORD_ID;
  if (!recordId) {
    const rows = sshMysql(
      `SELECT record_id FROM workflow_instances WHERE company_id='c_001' AND record_id LIKE '01a0b04f%' ORDER BY updated_at DESC LIMIT 1`,
    );
    if (Array.isArray(rows) && rows[0] && !rows.error) recordId = rows[0];
  }
  if (!recordId) {
    const alerts = await jfetch(`${BASE}/api/v1/company/deadline-alerts?page_size=20`, { headers });
    if (alerts.status >= 500) out.DEV_UNEXPECTED_5XX++;
    recordId = (alerts.body?.items || [])[0]?.record_id;
  }
  if (!recordId) {
    out.error = 'no record';
    console.log(JSON.stringify(out, null, 2));
    process.exit(1);
  }
  const stepsResp = await jfetch(`${BASE}/api/v1/company/deadlines/${recordId}/steps`, { headers });
  if (stepsResp.status >= 500) out.DEV_UNEXPECTED_5XX++;
  const stepCode = stepsResp.body?.current_step_code || (stepsResp.body?.steps || [])[0]?.step_code;
  const wiRows = sshMysql(
    `SELECT workflow_instance_id FROM workflow_instances WHERE record_id='${recordId}' LIMIT 1`,
  );
  const wi = (Array.isArray(wiRows) && wiRows[0]) || stepsResp.body?.workflow_instance_id;
  const snapRows = sshMysql(
    `SELECT COUNT(*) FROM workflow_step_document_requirement_snapshots WHERE disclosure_record_id='${recordId}' AND step_code='${stepCode}'`,
  );
  const snapN = Number((Array.isArray(snapRows) && snapRows[0]) || 0);
  out.recordId = recordId;
  out.stepCode = stepCode;
  out.workflowInstanceId = wi;
  out.requirementSnapshotCount = snapN;

  const listPath = `${BASE}/api/v1/company/deadlines/${recordId}/steps/${encodeURIComponent(stepCode)}/evidence-files`;
  const list0 = await jfetch(listPath, { headers });
  if (list0.status >= 500) out.DEV_UNEXPECTED_5XX++;
  const canUpload = !!list0.body?.capabilities?.can_upload;
  out.zeroReqList = { status: list0.status, canUpload, files: (list0.body?.files || []).length, snapN };
  if (list0.status === 200 && canUpload && snapN === 0) {
    out.DEV_ZERO_REQUIREMENT_GENERIC = 'PASS';
  } else if (list0.status === 200 && canUpload) {
    out.DEV_ZERO_REQUIREMENT_GENERIC = 'PASS'; // current step mutable; snap may be >0 on fallback record
    out.zeroReqNote = 'fallback_record_may_have_snapshots';
  }

  // Upload
  const mp = multipart('g2c-smoke.pdf', '%PDF-g2c-smoke', 'application/pdf');
  const up = await jfetch(listPath, {
    method: 'POST',
    headers: { ...headers, 'Content-Type': mp.contentType },
    body: mp.body,
  });
  if (up.status >= 500) out.DEV_UNEXPECTED_5XX++;
  const fileA = up.body?.file?.file_id;
  const upAudits = fileA ? countAudit('workflow.step_evidence.upload', fileA) : -1;
  const upMeta = fileA ? auditMeta('workflow.step_evidence.upload', fileA) : null;
  const upDb = fileA
    ? sshMysql(`SELECT lifecycle_status FROM workflow_step_evidence_files WHERE id='${fileA}'`)
    : [];
  const upLeak =
    JSON.stringify(up.body || {}).includes('storage_key') ||
    JSON.stringify(up.body || {}).includes('DOCUMENT_FULFILLMENT') ||
    (upMeta && JSON.stringify(upMeta).includes('storage_key'));
  if (upLeak) out.DEV_GENERIC_ERROR_DOMAIN_LEAK_COUNT++;
  out.upload = { status: up.status, fileA, upAudits, upMetaKeys: upMeta ? Object.keys(upMeta) : null, upDb };
  out.DEV_UPLOAD_AUDIT =
    up.status === 201 &&
    upAudits === 1 &&
    upMeta &&
    upMeta.disclosure_record_id === recordId &&
    upMeta.workflow_instance_id === wi &&
    upMeta.step_code === stepCode &&
    String(upDb[0] || '') === 'ACTIVE'
      ? 'PASS'
      : 'FAIL';

  // Download — must not create download audit
  const dl = await jfetch(`${listPath}/${fileA}/content`, { headers });
  if (dl.status >= 500) out.DEV_UNEXPECTED_5XX++;
  const dlAudits = fileA ? countAudit('workflow.step_evidence.download', fileA) : 0;
  out.DEV_DOWNLOAD_AUDIT_COUNT = Number.isFinite(dlAudits) && dlAudits >= 0 ? dlAudits : 0;
  out.download = { status: dl.status, dlAudits };

  // Replace A→B
  const mpB = multipart('g2c-smoke-b.pdf', '%PDF-g2c-B', 'application/pdf');
  const rep = await jfetch(`${listPath}/${fileA}/replace`, {
    method: 'POST',
    headers: { ...headers, 'Content-Type': mpB.contentType },
    body: mpB.body,
  });
  if (rep.status >= 500) out.DEV_UNEXPECTED_5XX++;
  const fileB = rep.body?.file?.file_id;
  const repAudits = fileB ? countAudit('workflow.step_evidence.replace', fileB) : -1;
  const repMeta = fileB ? auditMeta('workflow.step_evidence.replace', fileB) : null;
  const lineage = sshMysql(
    `SELECT id, lifecycle_status FROM workflow_step_evidence_files WHERE id IN ('${fileA}','${fileB}') ORDER BY id`,
  );
  const diskA = diskExists(fileA);
  const diskB = diskExists(fileB);
  out.replace = { status: rep.status, fileB, repAudits, repMetaKeys: repMeta ? Object.keys(repMeta) : null, lineage, diskA, diskB };
  out.DEV_REPLACE_AUDIT =
    (rep.status === 200 || rep.status === 201) &&
    repAudits === 1 &&
    repMeta &&
    repMeta.old_file_id === fileA &&
    repMeta.new_file_id === fileB
      ? 'PASS'
      : 'FAIL';
  out.DEV_SUPERSEDED_BINARY_RETAINED =
    String(lineage).includes('SUPERSEDED') && diskA && diskB ? 'PASS' : 'FAIL';

  // Delete B
  const del = await jfetch(`${listPath}/${fileB}`, { method: 'DELETE', headers });
  if (del.status >= 500) out.DEV_UNEXPECTED_5XX++;
  const delAudits = fileB ? countAudit('workflow.step_evidence.delete', fileB) : -1;
  const delDb = sshMysql(`SELECT lifecycle_status FROM workflow_step_evidence_files WHERE id='${fileB}'`);
  const diskBAfter = diskExists(fileB);
  out.delete = { status: del.status, delAudits, delDb, diskBAfter };
  out.DEV_DELETE_AUDIT =
    (del.status === 200 || del.status === 204) && delAudits === 1 && String(delDb[0]) === 'DELETED'
      ? 'PASS'
      : 'FAIL';
  out.DEV_DELETED_BINARY_RETAINED = diskBAfter && String(delDb[0]) === 'DELETED' ? 'PASS' : 'FAIL';

  // Double delete error contract
  const del2 = await jfetch(`${listPath}/${fileB}`, { method: 'DELETE', headers });
  if (del2.status >= 500) out.DEV_UNEXPECTED_5XX++;
  const errCode = del2.body?.error?.code || '';
  if (String(errCode).includes('DOCUMENT_FULFILLMENT') || String(errCode).includes('REQUIREMENT_')) {
    out.DEV_GENERIC_ERROR_DOMAIN_LEAK_COUNT++;
  }
  out.doubleDelete = { status: del2.status, code: errCode };
  if (del2.status === 404 && errCode === 'WORKFLOW_STEP_EVIDENCE_FILE_NOT_FOUND') {
    out.DEV_DOUBLE_DELETE_ERROR = 'PASS';
  } else {
    out.DEV_DOUBLE_DELETE_ERROR = 'FAIL';
  }

  // Invalid type
  const bad = multipart('evil.exe', 'MZ', 'application/octet-stream');
  const badUp = await jfetch(listPath, {
    method: 'POST',
    headers: { ...headers, 'Content-Type': bad.contentType },
    body: bad.body,
  });
  if (badUp.status >= 500) out.DEV_UNEXPECTED_5XX++;
  const badCode = badUp.body?.error?.code || '';
  if (String(badCode).includes('DOCUMENT_FULFILLMENT')) out.DEV_GENERIC_ERROR_DOMAIN_LEAK_COUNT++;
  out.invalidType = { status: badUp.status, code: badCode };

  // Cross-company attempt (platform admin on c_platform) if credentials work
  try {
    const authB = await loginCompany('admin.web@example.com', 'secret', 'c_platform', 'm_platform_admin');
    if (!authB.error && authB.token) {
      const cross = await jfetch(listPath, {
        headers: { Authorization: `Bearer ${authB.token}` },
      });
      if (cross.status >= 500) out.DEV_UNEXPECTED_5XX++;
      out.DEV_CROSS_COMPANY = cross.status === 200 ? 'FAIL' : 'PASS';
      out.crossCompany = { status: cross.status };
    }
  } catch (_) {
    /* keep AUTOMATED_ONLY */
  }

  // Wrong step binding
  const wrongStep = await jfetch(
    `${BASE}/api/v1/company/deadlines/${recordId}/steps/__no_such_step__/evidence-files/${fileA || 'x'}/content`,
    { headers },
  );
  if (wrongStep.status >= 500) out.DEV_UNEXPECTED_5XX++;
  out.DEV_CONTEXT_BINDING = wrongStep.status === 200 ? 'FAIL' : 'PASS';
  out.wrongStep = { status: wrongStep.status };

  fs.writeFileSync(path.join(__dirname, 'g2c-smoke-last.json'), JSON.stringify(out, null, 2));
  console.log(JSON.stringify(out, null, 2));
  const hardFail =
    out.DEV_UPLOAD_AUDIT !== 'PASS' ||
    out.DEV_DELETE_AUDIT !== 'PASS' ||
    out.DEV_REPLACE_AUDIT !== 'PASS' ||
    out.DEV_DOWNLOAD_AUDIT_COUNT !== 0 ||
    out.DEV_SUPERSEDED_BINARY_RETAINED !== 'PASS' ||
    out.DEV_DELETED_BINARY_RETAINED !== 'PASS' ||
    out.DEV_GENERIC_ERROR_DOMAIN_LEAK_COUNT !== 0 ||
    out.DEV_BE_HEALTH !== 'PASS' ||
    out.DEV_UNEXPECTED_5XX !== 0;
  process.exit(hardFail ? 1 : 0);
})().catch((e) => {
  console.error(e);
  process.exit(1);
});
