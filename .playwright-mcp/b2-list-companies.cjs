const BASE = 'http://88.216.208.0:8080';
async function text(url, opts) {
  const res = await fetch(url, opts);
  const t = await res.text();
  let body;
  try {
    body = JSON.parse(t);
  } catch {
    body = t;
  }
  return { status: res.status, body };
}
(async () => {
  const login = await text(`${BASE}/api/v1/auth/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ login_id: 'admin.dn@example.com', password: 'secret' }),
  });
  console.log('login', login.status);
  const pre = login.body?.session?.pre_company_token || login.body?.pre_company_token;
  const companies = await text(`${BASE}/api/v1/me/companies`, { headers: { Authorization: `Bearer ${pre}` } });
  console.log(JSON.stringify(companies, null, 2).slice(0, 3000));
})().catch((e) => console.error(e));
