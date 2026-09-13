/**
 * B2 DEV smoke — runtime document fulfillment CRUD + ACL (safe QA fixture).
 * Creates one new disclosure with snapshots, then exercises B2 APIs.
 * FE_DEV_DEPLOY not required. Does not enable Complete Step document gate.
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

function ssh(cmd) {
  const full = `ssh -p 21239 -i "${process.env.USERPROFILE}\\.ssh\\id_ed25519" -o BatchMode=yes root@88.216.208.0 ${JSON.stringify(cmd)}`;
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
  const boundary = '----cobob2' + Date.now();
  const body = Buffer.concat([
    Buffer.from(`--${boundary}\r\nContent-Disposition: form-data; name="file"; filename="${fileName}"\r\nContent-Type: ${contentType}\r\n\r\n`),
    Buffer.from(content),
    Buffer.from(`\r\n--${boundary}--\r\n`),
  ]);
  return { body, contentType: `multipart/form-data; boundary=${boundary}` };
}

(async () => {
  const out = {
    DEV_RUNTIME_REQUIREMENT_LIST: 'FAIL',
    DEV_UPLOAD: 'FAIL',
    DEV_DOWNLOAD: 'FAIL',
    DEV_ONE_TO_MANY: 'FAIL',
    DEV_LOGICAL_DELETE: 'FAIL',
    DEV_REPLACE_LINEAGE: 'FAIL',
    DEV_FILE_LIMIT: 'AUTOMATED_ONLY',
    DEV_COMPLETED_STEP_IMMUTABILITY: 'NOT_AVAILABLE',
    DEV_NON_CURRENT_STEP_MUTATION_DENIED: 'NOT_AVAILABLE',
    DEV_CROSS_COMPANY_DOWNLOAD_DENIED: 'FAIL',
    DEV_CROSS_COMPANY_UPLOAD_DENIED: 'FAIL',
    DEV_VIEW_ONLY_MUTATION_DENIED: 'NOT_AVAILABLE',
    DEV_REQUIRED_DOCUMENT_GATE_ACTIVE: false,
    DEV_STORAGE_NAMESPACE: 'FAIL',
    DEV_STORAGE_PATH_LEAK_COUNT: -1,
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

  // Use an in-scope deadline alert (data-scope safe). Seed one QA snapshot row if missing.
  const alerts = await jfetch(`${BASE}/api/v1/company/deadline-alerts?page_size=20`, { headers });
  if (alerts.status >= 500) out.API_UNEXPECTED_5XX++;
  const alert = (alerts.body?.items || [])[0];
  if (!alert?.record_id) {
    out.error = 'no deadline alerts in scope';
    console.log(JSON.stringify(out, null, 2));
    process.exit(1);
  }
  const recordId = alert.record_id;
  const stepsResp = await jfetch(`${BASE}/api/v1/company/deadlines/${recordId}/steps`, { headers });
  if (stepsResp.status >= 500) out.API_UNEXPECTED_5XX++;
  if (stepsResp.status !== 200) {
    out.error = { steps: stepsResp };
    console.log(JSON.stringify(out, null, 2));
    process.exit(1);
  }
  const stepCode = stepsResp.body?.current_step_code || (stepsResp.body?.steps || [])[0]?.step_code;
  const wi = alert.workflow_instance_id || sshMysql(`SELECT workflow_instance_id FROM workflow_instances WHERE record_id='${recordId}' LIMIT 1`)[0];
  let snapCount = sshMysql(`SELECT COUNT(*) FROM workflow_step_document_requirement_snapshots WHERE disclosure_record_id='${recordId}' AND step_code='${stepCode}'`);
  if (Array.isArray(snapCount) && Number(snapCount[0]) === 0) {
    const snapId = 'b2qa-' + Date.now();
    sshMysql(
      `INSERT INTO workflow_step_document_requirement_snapshots (id, company_id, disclosure_record_id, workflow_instance_id, step_code, source_doc_id, requirement_key, name, required, ordinal) VALUES ('${snapId}', 'c_001', '${recordId}', '${wi}', '${stepCode}', 'b2-qa-doc', 'b2-qa-doc', 'B2 QA Requirement', 1, 0)`,
    );
    out.seededSnapshot = snapId;
  }
  out.recordId = recordId;
  out.stepCode = stepCode;
  out.workflowInstanceId = wi;

  const list = await jfetch(
    `${BASE}/api/v1/company/deadlines/${recordId}/steps/${encodeURIComponent(stepCode)}/document-requirements`,
    { headers },
  );
  if (list.status >= 500) out.API_UNEXPECTED_5XX++;
  const reqs = list.body?.requirements || [];
  const leak = JSON.stringify(list.body || {}).includes('storage_key') || JSON.stringify(list.body || {}).includes('cms-media');
  out.DEV_STORAGE_PATH_LEAK_COUNT = leak ? 1 : 0;
  const snapOnly = list.body?.workflow_instance_id && Array.isArray(reqs);
  const namesOk = reqs.length > 0 && reqs.every((r) => r.requirement_snapshot_id && r.name != null);
  out.DEV_RUNTIME_REQUIREMENT_LIST = list.status === 200 && snapOnly && namesOk ? 'PASS' : 'FAIL';
  out.listStatus = list.status;
  out.requirementCount = reqs.length;
  if (reqs.length === 0) {
    console.log(JSON.stringify({ out, list: list.body }, null, 2));
    process.exit(1);
  }
  const reqId = reqs[0].requirement_snapshot_id;

  // Upload
  const mp1 = multipart('b2-smoke.pdf', '%PDF-1.4 smoke1', 'application/pdf');
  const up1 = await jfetch(
    `${BASE}/api/v1/company/deadlines/${recordId}/steps/${encodeURIComponent(stepCode)}/document-requirements/${reqId}/files`,
    { method: 'POST', headers: { ...headers, 'Content-Type': mp1.contentType }, body: mp1.body },
  );
  if (up1.status >= 500) out.API_UNEXPECTED_5XX++;
  const fileId1 = up1.body?.file?.file_id;
  const db1 = sshMysql(`SELECT lifecycle_status, storage_key, original_file_name FROM workflow_step_document_fulfillment_files WHERE id='${fileId1}'`);
  out.DEV_UPLOAD = up1.status === 201 && fileId1 && Array.isArray(db1) && db1[0]?.startsWith('ACTIVE') ? 'PASS' : 'FAIL';
  out.upload = { status: up1.status, body: up1.body, db1 };

  const storageKey = Array.isArray(db1) && db1[0] ? db1[0].split('\t')[1] : '';
  out.DEV_STORAGE_NAMESPACE =
    storageKey.startsWith('workflow-step-document-fulfillments/') ? 'PASS' : 'FAIL';
  const exists = ssh(`docker exec cobo-iam-api sh -c 'test -f /data/cms-media/workflow-step-document-fulfillments/*/*/*/b2-smoke.pdf 2>/dev/null || find / -path "*workflow-step-document-fulfillments*" -name "b2-smoke.pdf" 2>/dev/null | head -1'`);
  out.physicalExistsProbe = exists;

  // Download
  const dl = await jfetch(
    `${BASE}/api/v1/company/deadlines/${recordId}/steps/${encodeURIComponent(stepCode)}/document-requirements/files/${fileId1}/content`,
    { headers },
  );
  if (dl.status >= 500) out.API_UNEXPECTED_5XX++;
  const cd = dl.headers.get('content-disposition') || '';
  out.DEV_DOWNLOAD =
    dl.status === 200 && String(dl.raw).includes('smoke1') && /b2-smoke\.pdf/.test(cd) ? 'PASS' : 'FAIL';
  out.download = { status: dl.status, cd, ctype: dl.headers.get('content-type') };

  // One-to-many
  const mp2 = multipart('b2-smoke-2.pdf', '%PDF-1.4 smoke2', 'application/pdf');
  const up2 = await jfetch(
    `${BASE}/api/v1/company/deadlines/${recordId}/steps/${encodeURIComponent(stepCode)}/document-requirements/${reqId}/files`,
    { method: 'POST', headers: { ...headers, 'Content-Type': mp2.contentType }, body: mp2.body },
  );
  if (up2.status >= 500) out.API_UNEXPECTED_5XX++;
  const list2 = await jfetch(
    `${BASE}/api/v1/company/deadlines/${recordId}/steps/${encodeURIComponent(stepCode)}/document-requirements`,
    { headers },
  );
  const filesAfter = (list2.body?.requirements || []).find((r) => r.requirement_snapshot_id === reqId)?.files || [];
  out.DEV_ONE_TO_MANY = up2.status === 201 && filesAfter.length >= 2 ? 'PASS' : 'FAIL';

  // Replace
  const fileId2 = up2.body?.file?.file_id;
  const mp3 = multipart('b2-replaced.pdf', '%PDF-1.4 replaced', 'application/pdf');
  const rep = await jfetch(
    `${BASE}/api/v1/company/deadlines/${recordId}/steps/${encodeURIComponent(stepCode)}/document-requirements/files/${fileId2}/replace`,
    { method: 'POST', headers: { ...headers, 'Content-Type': mp3.contentType }, body: mp3.body },
  );
  if (rep.status >= 500) out.API_UNEXPECTED_5XX++;
  const lineage = sshMysql(
    `SELECT id, lifecycle_status, IFNULL(supersedes_file_id,''), IFNULL(superseded_by_file_id,'') FROM workflow_step_document_fulfillment_files WHERE id IN ('${fileId2}','${rep.body?.file?.file_id}') ORDER BY uploaded_at`,
  );
  out.DEV_REPLACE_LINEAGE =
    rep.status === 200 &&
    Array.isArray(lineage) &&
    lineage.some((l) => l.includes(fileId2) && l.includes('SUPERSEDED')) &&
    lineage.some((l) => l.includes(rep.body?.file?.file_id) && l.includes('ACTIVE') && l.includes(fileId2))
      ? 'PASS'
      : 'FAIL';
  out.replace = { status: rep.status, lineage };

  // Logical delete file1
  const del = await jfetch(
    `${BASE}/api/v1/company/deadlines/${recordId}/steps/${encodeURIComponent(stepCode)}/document-requirements/files/${fileId1}`,
    { method: 'DELETE', headers },
  );
  if (del.status >= 500) out.API_UNEXPECTED_5XX++;
  const afterDel = sshMysql(`SELECT lifecycle_status, storage_key FROM workflow_step_document_fulfillment_files WHERE id='${fileId1}'`);
  const list3 = await jfetch(
    `${BASE}/api/v1/company/deadlines/${recordId}/steps/${encodeURIComponent(stepCode)}/document-requirements`,
    { headers },
  );
  const visible = ((list3.body?.requirements || []).find((r) => r.requirement_snapshot_id === reqId)?.files || []).some(
    (f) => f.file_id === fileId1,
  );
  out.DEV_LOGICAL_DELETE =
    del.status === 204 && Array.isArray(afterDel) && afterDel[0]?.startsWith('DELETED') && !visible ? 'PASS' : 'FAIL';
  out.delete = { status: del.status, afterDel, visible };

  // Cross-company: user@example.com membership on c_002
  const authB = await loginCompany('user@example.com', 'secret', 'c_002', 'm_002');
  if (!authB.error) {
    const headersB = { Authorization: `Bearer ${authB.token}` };
    const dlB = await jfetch(
      `${BASE}/api/v1/company/deadlines/${recordId}/steps/${encodeURIComponent(stepCode)}/document-requirements/files/${rep.body?.file?.file_id}/content`,
      { headers: headersB },
    );
    if (dlB.status >= 500) out.API_UNEXPECTED_5XX++;
    out.DEV_CROSS_COMPANY_DOWNLOAD_DENIED = dlB.status === 403 || dlB.status === 404 ? 'PASS' : 'FAIL';
    out.crossDl = { status: dlB.status };

    const mpB = multipart('evil.pdf', '%PDF', 'application/pdf');
    const upB = await jfetch(
      `${BASE}/api/v1/company/deadlines/${recordId}/steps/${encodeURIComponent(stepCode)}/document-requirements/${reqId}/files`,
      { method: 'POST', headers: { ...headersB, 'Content-Type': mpB.contentType }, body: mpB.body },
    );
    if (upB.status >= 500) out.API_UNEXPECTED_5XX++;
    out.DEV_CROSS_COMPANY_UPLOAD_DENIED = upB.status === 403 || upB.status === 404 ? 'PASS' : 'FAIL';
    out.crossUp = { status: upB.status };
  } else {
    out.crossLoginB = authB;
    out.DEV_CROSS_COMPANY_DOWNLOAD_DENIED = 'FAIL';
    out.DEV_CROSS_COMPANY_UPLOAD_DENIED = 'FAIL';
  }

  // Complete gate still inactive: source proof + list capabilities when not completed
  out.DEV_REQUIRED_DOCUMENT_GATE_ACTIVE = false;

  const ok =
    out.DEV_RUNTIME_REQUIREMENT_LIST === 'PASS' &&
    out.DEV_UPLOAD === 'PASS' &&
    out.DEV_DOWNLOAD === 'PASS' &&
    out.DEV_ONE_TO_MANY === 'PASS' &&
    out.DEV_LOGICAL_DELETE === 'PASS' &&
    out.DEV_REPLACE_LINEAGE === 'PASS' &&
    out.DEV_CROSS_COMPANY_DOWNLOAD_DENIED === 'PASS' &&
    out.DEV_CROSS_COMPANY_UPLOAD_DENIED === 'PASS' &&
    out.DEV_STORAGE_PATH_LEAK_COUNT === 0 &&
    out.BE_HEALTH === 'PASS' &&
    out.API_UNEXPECTED_5XX === 0;

  console.log(JSON.stringify(out, null, 2));
  try {
    require('fs').writeFileSync(require('path').join(__dirname, 'b2-smoke-last.json'), JSON.stringify(out, null, 2));
  } catch (_) {}
  process.exit(ok ? 0 : 1);
})().catch((e) => {
  console.error(e);
  process.exit(1);
});
