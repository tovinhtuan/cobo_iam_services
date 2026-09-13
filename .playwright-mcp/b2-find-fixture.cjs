const BASE = 'http://88.216.208.0:8080';
const { execSync } = require('child_process');
function sshMysql(sql) {
  const cmd = `ssh -p 21239 -i "${process.env.USERPROFILE}\\.ssh\\id_ed25519" -o BatchMode=yes root@88.216.208.0 "docker exec cobo-iam-mysql mysql -uroot -proot cobo_iam -Nse \\"${sql.replace(/"/g, '\\"')}\\""`;
  try {
    return execSync(cmd, { encoding: 'utf8', stdio: ['pipe', 'pipe', 'pipe'] })
      .split(/\r?\n/)
      .map((l) => l.trim())
      .filter((l) => l && !/Warning/.test(l));
  } catch (e) {
    return [String(e.stderr || e.message || e)];
  }
}
(async () => {
  const login = await fetch(`${BASE}/api/v1/auth/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ login_id: 'admin.dn@example.com', password: 'secret' }),
  }).then((r) => r.json());
  const pre = login.session?.pre_company_token || login.pre_company_token;
  const sel = await fetch(`${BASE}/api/v1/auth/select-company`, {
    method: 'POST',
    headers: { Authorization: `Bearer ${pre}`, 'Content-Type': 'application/json' },
    body: JSON.stringify({ company_id: 'c_001', membership_id: 'm_102' }),
  }).then((r) => r.json());
  const token = sel.session?.access_token || sel.access_token;
  const alerts = await fetch(`${BASE}/api/v1/company/deadline-alerts?page_size=40`, {
    headers: { Authorization: `Bearer ${token}` },
  }).then((r) => r.json());
  for (const a of alerts.items || []) {
    const snaps = sshMysql(
      `SELECT COUNT(*), MIN(step_code) FROM workflow_step_document_requirement_snapshots WHERE disclosure_record_id='${a.record_id}'`,
    );
    const n = Number((snaps[0] || '0\t').split('\t')[0]);
    if (n > 0) console.log(JSON.stringify({ record_id: a.record_id, title: a.title, snaps: snaps[0] }));
  }
})().catch((e) => console.error(e));
