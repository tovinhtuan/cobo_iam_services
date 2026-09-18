/**
 * Evidence V1 recovery — REAL authoring → materialization → tenant upload E2E.
 * HARD: never INSERT into workflow_step_document_requirement_snapshots.
 * Uses dedicated QA type qa-evidence-recovery-e2e-20260918 (documents authored via CMS).
 */
const BASE = process.env.COBO_API_BASE || 'http://88.216.208.0:8080';
const TYPE_ID = process.env.COBO_EVIDENCE_QA_TYPE || 'qa-evidence-recovery-e2e-20260918';
const { execSync } = require('child_process');
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
  const escaped = sql.replace(/\\/g, '\\\\').replace(/"/g, '\\"');
  const cmd = `ssh -p 21239 -o BatchMode=yes root@88.216.208.0 "docker exec cobo-iam-mysql mysql -uroot -proot cobo_iam -Nse \\"${escaped}\\""`;
  try {
    return execSync(cmd, { encoding: 'utf8' })
      .split(/\r?\n/)
      .map((l) => l.trim())
      .filter((l) => l && !/Warning/.test(l));
  } catch (e) {
    return { error: String(e.stderr || e.message || e) };
  }
}

function multipart(filename, content, contentType) {
  const boundary = '----cobo' + Date.now();
  const body = Buffer.concat([
    Buffer.from(
      `--${boundary}\r\nContent-Disposition: form-data; name="file"; filename="${filename}"\r\nContent-Type: ${contentType}\r\n\r\n`,
    ),
    Buffer.from(content),
    Buffer.from(`\r\n--${boundary}--\r\n`),
  ]);
  return { body, contentType: `multipart/form-data; boundary=${boundary}` };
}

async function session(loginId) {
  const login = await jfetch(`${BASE}/api/v1/auth/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ login_id: loginId, password: 'secret' }),
  });
  const pre = login.body?.session?.pre_company_token;
  const memb = (login.body?.memberships || []).find((m) => m.company_id === 'c_001') || {};
  const sel = await jfetch(`${BASE}/api/v1/auth/select-company`, {
    method: 'POST',
    headers: { Authorization: `Bearer ${pre}`, 'Content-Type': 'application/json' },
    body: JSON.stringify({ company_id: 'c_001', membership_id: memb.membership_id }),
  });
  return sel.body?.session?.access_token || sel.body?.access_token;
}

function passFail(ok) {
  return ok ? 'PASS' : 'FAIL';
}

(async () => {
  const out = {
    SQL_SEEDED_REQUIREMENT_SNAPSHOTS: false,
    RELEASE_E2E_DIRECT_SNAPSHOT_SEED_COUNT: 0,
    SMOKE_ASSERT_EFFECTIVE_DOCUMENTS_GT_ZERO: false,
    SMOKE_ASSERT_REAL_SNAPSHOT_COUNT_GT_ZERO: false,
    SMOKE_ASSERT_TENANT_UPLOAD_CTA: false,
    SMOKE_ASSERT_REAL_TENANT_UPLOAD: false,
    MATERIALIZATION_PATH: 'periodic_worker_CreateAndSubmitRecordWithPlannedDate',
    TYPE_ID,
  };

  // Effective docs must be > 0 (authoring proof)
  let token = await session('admin.dn@example.com');
  const eff = await jfetch(`${BASE}/api/v1/disclosure-types/${encodeURIComponent(TYPE_ID)}/effective-workflow`, {
    headers: { Authorization: `Bearer ${token}` },
  });
  const steps = eff.body?.data?.workflow || eff.body?.workflow || [];
  let effDocs = 0;
  for (const s of steps) effDocs += (s.documents || []).length;
  out.EFFECTIVE_DOCUMENT_COUNT = effDocs;
  out.SMOKE_ASSERT_EFFECTIVE_DOCUMENTS_GT_ZERO = effDocs > 0;
  if (effDocs === 0) {
    out.error = 'effective documents empty — authoring correction required before smoke';
    fs.writeFileSync(__dirname + '/recovery-authoring-real-e2e-last.json', JSON.stringify(out, null, 2));
    console.log(JSON.stringify(out, null, 2));
    process.exit(1);
  }

  // Prefer an unfinished alert for this type with real snaps (no SQL seed)
  token = await session('admin.dn@example.com');
  const alerts = await jfetch(`${BASE}/api/v1/company/deadline-alerts?tab=incomplete&page_size=50`, {
    headers: { Authorization: `Bearer ${token}` },
  });
  const items = alerts.body?.items || [];
  let chosen = null;
  for (const a of items) {
    if (a.type_id !== TYPE_ID) continue;
    const rid = a.record_id;
    const step = a.current_step_code || 'step-001';
    const n = Number(
      sshMysql(
        `SELECT COUNT(*) FROM workflow_step_document_requirement_snapshots WHERE disclosure_record_id='${rid}'`,
      )[0] || 0,
    );
    if (n <= 0) continue;
    // Skip historically completed steps (still listed as incomplete parent alert).
    const probe = await jfetch(
      `${BASE}/api/v1/company/deadlines/${rid}/steps/${encodeURIComponent(step)}/document-requirements`,
      { headers: { Authorization: `Bearer ${token}` } },
    );
    const list = probe.body?.requirements || [];
    const canUpload = list.some((r) => r.capabilities?.can_upload);
    if (probe.body?.completed || !canUpload) continue;
    chosen = { rid, step, wi: a.workflow_instance_id, snapN: n };
    break;
  }
  if (!chosen) {
    out.error = 'no incomplete alert for recovery type with real snapshots';
    fs.writeFileSync(__dirname + '/recovery-authoring-real-e2e-last.json', JSON.stringify(out, null, 2));
    console.log(JSON.stringify(out, null, 2));
    process.exit(1);
  }
  out.chosen = chosen;
  out.SMOKE_ASSERT_REAL_SNAPSHOT_COUNT_GT_ZERO = chosen.snapN > 0;
  out.REAL_MATERIALIZED_SNAPSHOT_COUNT = chosen.snapN;

  token = await session('admin.dn@example.com');
  const reqs = await jfetch(
    `${BASE}/api/v1/company/deadlines/${chosen.rid}/steps/${encodeURIComponent(chosen.step)}/document-requirements`,
    { headers: { Authorization: `Bearer ${token}` } },
  );
  const list = reqs.body?.requirements || [];
  out.TENANT_RUNTIME_REQUIREMENT_COUNT = list.length;
  const canUpload = list.some((r) => r.capabilities?.can_upload);
  out.TENANT_CAN_UPLOAD_CAPABILITY = canUpload;
  out.SMOKE_ASSERT_TENANT_UPLOAD_CTA = canUpload && list.length > 0;

  const r1 = list.find((r) => r.required) || list[0];
  token = await session('admin.dn@example.com');
  const mp = multipart('recovery-smoke.pdf', '%PDF-1.4 recovery-smoke', 'application/pdf');
  const up = await jfetch(
    `${BASE}/api/v1/company/deadlines/${chosen.rid}/steps/${encodeURIComponent(chosen.step)}/document-requirements/${r1.requirement_snapshot_id}/files`,
    {
      method: 'POST',
      headers: { Authorization: `Bearer ${token}`, 'Content-Type': mp.contentType },
      body: mp.body,
    },
  );
  out.REAL_UPLOAD_HTTP_STATUS = up.status;
  out.SMOKE_ASSERT_REAL_TENANT_UPLOAD = up.status >= 200 && up.status < 300;

  // Complete missing gate (if still missing required)
  token = await session('admin.dn@example.com');
  const complete = await jfetch(
    `${BASE}/api/v1/company/deadlines/${chosen.rid}/steps/${encodeURIComponent(chosen.step)}/complete`,
    { method: 'POST', headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' }, body: '{}' },
  );
  out.complete_probe = { status: complete.status, code: complete.body?.error?.code };

  out.PHASE_SMOKE_RESULT = passFail(
    out.SMOKE_ASSERT_EFFECTIVE_DOCUMENTS_GT_ZERO &&
      out.SMOKE_ASSERT_REAL_SNAPSHOT_COUNT_GT_ZERO &&
      out.SMOKE_ASSERT_TENANT_UPLOAD_CTA &&
      out.SMOKE_ASSERT_REAL_TENANT_UPLOAD &&
      out.RELEASE_E2E_DIRECT_SNAPSHOT_SEED_COUNT === 0,
  );

  fs.writeFileSync(__dirname + '/recovery-authoring-real-e2e-last.json', JSON.stringify(out, null, 2));
  console.log(JSON.stringify(out, null, 2));
  process.exit(out.PHASE_SMOKE_RESULT === 'PASS' ? 0 : 1);
})().catch((e) => {
  console.error(e);
  process.exit(1);
});
