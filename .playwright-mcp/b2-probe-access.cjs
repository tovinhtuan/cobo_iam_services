const BASE = 'http://88.216.208.0:8080';
(async () => {
  const login = await fetch(`${BASE}/api/v1/auth/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ login_id: 'admin.dn@example.com', password: 'secret' }),
  }).then((r) => r.json());
  const pre = login.session?.pre_company_token || login.pre_company_token;
  const companies = await fetch(`${BASE}/api/v1/auth/companies`, {
    headers: { Authorization: `Bearer ${pre}` },
  }).then((r) => r.json());
  console.log(JSON.stringify(companies, null, 2).slice(0, 2000));

  for (const c of companies.data || companies.items || companies.companies || []) {
    const cid = c.company_id || c.id;
    const mid = c.membership_id || c.membershipId;
    if (!cid || !mid) continue;
    const sel = await fetch(`${BASE}/api/v1/auth/select-company`, {
      method: 'POST',
      headers: { Authorization: `Bearer ${pre}`, 'Content-Type': 'application/json' },
      body: JSON.stringify({ company_id: cid, membership_id: mid }),
    }).then((r) => r.json());
    const token = sel.session?.access_token || sel.access_token;
    const me = await fetch(`${BASE}/api/v1/me/effective-access`, {
      headers: { Authorization: `Bearer ${token}` },
    }).then((r) => r.json());
    const perms = me.data?.permissions || me.permissions || [];
    const scopes = me.data?.data_scopes || me.data_scopes || me.data?.scopes;
    console.log(JSON.stringify({ cid, mid, hasView: perms.includes('deadline.view'), hasConfirm: perms.includes('deadline.confirm'), scopes: scopes || me.data?.data_scope || null, permSample: perms.filter((p) => /deadline|disclosure|rbac/.test(p)).slice(0, 20) }));
  }
})().catch((e) => console.error(e));
