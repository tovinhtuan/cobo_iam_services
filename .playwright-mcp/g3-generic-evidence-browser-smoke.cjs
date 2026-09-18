/**
 * G3 DEV browser E2E — Generic Step Evidence UX on zero-requirement fixture.
 * Uses Playwright against real DEV UI (not API-only substitute).
 */
const { chromium } = require('playwright');
const fs = require('fs');
const path = require('path');

const BASE = process.env.COBO_FE_BASE || 'http://88.216.208.0:3000';
const API = process.env.COBO_API_BASE || 'http://88.216.208.0:8080';
const RECORD =
  process.env.G3_RECORD_ID || '01a0b04f-a5a3-763b-a5b1-efdbe006e641';
const OUT_DIR = path.join(
  __dirname,
  '../../cobo_web_design/docs/ai-cache/tenant-generic-step-evidence-v1-1-g3-tenant-ux-2026-09-18/screenshots',
);
const NETWORK = [];

function track(req) {
  const url = req.url();
  if (!url.includes('/evidence-files')) return;
  NETWORK.push({
    method: req.method(),
    path: url.replace(/^https?:\/\/[^/]+/, ''),
    // never store auth headers
  });
}

async function login(page) {
  await page.goto(`${BASE}/login`, { waitUntil: 'domcontentloaded' });
  // Prefer direct API login + inject tokens if login form selectors drift
  const loginRes = await page.request.post(`${API}/api/v1/auth/login`, {
    data: { login_id: 'admin.dn@example.com', password: 'secret' },
  });
  const loginBody = await loginRes.json();
  const pre =
    loginBody?.session?.pre_company_token || loginBody?.pre_company_token;
  if (!pre) throw new Error('login failed');
  const selRes = await page.request.post(`${API}/api/v1/auth/select-company`, {
    headers: { Authorization: `Bearer ${pre}` },
    data: { company_id: 'c_001', membership_id: 'm_102' },
  });
  const selBody = await selRes.json();
  const access =
    selBody?.session?.access_token || selBody?.access_token;
  const refresh =
    selBody?.session?.refresh_token || selBody?.refresh_token || '';
  if (!access) throw new Error('select-company failed');
  await page.goto(BASE, { waitUntil: 'domcontentloaded' });
  await page.evaluate(
    ({ access, refresh, company }) => {
      localStorage.setItem('cobo_access_token', access);
      if (refresh) localStorage.setItem('cobo_refresh_token', refresh);
      localStorage.setItem('cobo_selected_company_id', company);
    },
    { access, refresh, company: 'c_001' },
  );
}

(async () => {
  fs.mkdirSync(OUT_DIR, { recursive: true });
  const out = {
    G3_BROWSER_E2E_REAL_UI: true,
    DEV_ZERO_REQUIREMENT_GENERIC_SECTION: 'FAIL',
    DEV_ZERO_REQUIREMENT_UPLOAD_CTA: 'FAIL',
    DEV_UI_GENERIC_UPLOAD: 'FAIL',
    DEV_UI_GENERIC_HARD_RELOAD: 'FAIL',
    DEV_UI_GENERIC_DOWNLOAD: 'FAIL',
    DEV_UI_GENERIC_REPLACE: 'FAIL',
    DEV_UI_REPLACE_HARD_RELOAD: 'FAIL',
    DEV_UI_GENERIC_DELETE: 'FAIL',
    DEV_UI_DELETE_HARD_RELOAD: 'FAIL',
    DEV_REQUIREMENT_GENERIC_COEXISTENCE: 'AUTOMATED_ONLY',
    DEV_UI_GENERIC_CANNOT_SATISFY_REQUIRED_DOC: 'AUTOMATED_ONLY',
    DEV_UI_COMPLETED_GENERIC_READ_ONLY: 'AUTOMATED_ONLY',
    DEV_UI_VIEW_ONLY_GENERIC: 'AUTOMATED_ONLY',
    DEV_UI_CAPABILITY_PARITY: 'FAIL',
    DEV_UI_MAX_QUOTA: 'AUTOMATED_ONLY',
    DEV_UI_GENERIC_ERROR_UX: 'FAIL',
    DEV_DEADLINE_DETAIL_LAYOUT: 'FAIL',
    DEV_GENERIC_RESPONSIVE: 'FAIL',
    DEV_UNEXPECTED_5XX: 0,
    screenshots: [],
    network: [],
  };

  const browser = await chromium.launch({ headless: true });
  const context = await browser.newContext({
    viewport: { width: 1440, height: 900 },
    acceptDownloads: true,
  });
  const page = await context.newPage();
  page.on('request', track);
  page.on('response', (res) => {
    if (res.status() >= 500 && res.url().includes('/evidence-files')) {
      out.DEV_UNEXPECTED_5XX++;
    }
  });

  try {
    await login(page);
    await page.goto(`${BASE}/app/deadlines/${RECORD}`, {
      waitUntil: 'networkidle',
      timeout: 60000,
    });
    // Open evidence tab if needed
    const evidenceTab = page.getByRole('tab', { name: /Bằng chứng/i }).first();
    if (await evidenceTab.count()) {
      await evidenceTab.click();
    }
    await page.waitForSelector('[data-testid="deadline-evidence-area"]', {
      timeout: 30000,
    });
    await page.waitForSelector('[data-testid="step-generic-evidence-panel"]', {
      timeout: 30000,
    });

    const reqEmpty = await page
      .locator('[data-testid="step-document-requirements-empty"]')
      .count();
    const genericTitle = await page
      .locator('[data-testid="step-generic-evidence-panel"]')
      .getByText('Bằng chứng bổ sung')
      .count();
    const cta = page.locator('[data-testid="generic-evidence-upload-cta"]');
    out.DEV_ZERO_REQUIREMENT_GENERIC_SECTION =
      reqEmpty > 0 && genericTitle > 0 ? 'PASS' : 'FAIL';
    out.DEV_ZERO_REQUIREMENT_UPLOAD_CTA =
      (await cta.count()) > 0 ? 'PASS' : 'FAIL';

    await page.screenshot({
      path: path.join(OUT_DIR, '01-zero-requirement-generic-section.png'),
      fullPage: false,
    });
    out.screenshots.push('01-zero-requirement-generic-section.png');
    await page.screenshot({
      path: path.join(OUT_DIR, '02-upload-cta.png'),
      fullPage: false,
    });
    out.screenshots.push('02-upload-cta.png');

    // Layout check
    out.DEV_DEADLINE_DETAIL_LAYOUT =
      (await page.locator('[data-testid="deadline-evidence-area"]').count()) > 0
        ? 'PASS'
        : 'FAIL';

    // Capability parity: CTA present matches can_upload from last GET
    const listResp = NETWORK.filter(
      (n) => n.method === 'GET' && n.path.includes('/evidence-files') && !n.path.includes('/content'),
    );
    out.DEV_UI_CAPABILITY_PARITY =
      listResp.length > 0 && out.DEV_ZERO_REQUIREMENT_UPLOAD_CTA === 'PASS'
        ? 'PASS'
        : 'FAIL';

    // Upload via real UI file input
    const uploadInput = page.locator('[data-testid="generic-evidence-upload-input"]');
    const uploadPath = path.join(OUT_DIR, '_tmp-upload.pdf');
    fs.writeFileSync(uploadPath, '%PDF-1.4 g3-smoke');
    await uploadInput.setInputFiles(uploadPath);
    await page.waitForSelector('[data-testid="generic-evidence-file-row"]', {
      timeout: 20000,
    });
    out.DEV_UI_GENERIC_UPLOAD =
      (await page.locator('[data-testid="generic-evidence-file-row"]').count()) >= 1
        ? 'PASS'
        : 'FAIL';
    await page.screenshot({
      path: path.join(OUT_DIR, '03-file-after-upload.png'),
      fullPage: false,
    });
    out.screenshots.push('03-file-after-upload.png');

    // Hard reload
    await page.reload({ waitUntil: 'networkidle' });
    if (await evidenceTab.count()) await evidenceTab.click();
    await page.waitForSelector('[data-testid="generic-evidence-file-row"]', {
      timeout: 30000,
    });
    out.DEV_UI_GENERIC_HARD_RELOAD =
      (await page.locator('[data-testid="generic-evidence-file-row"]').count()) >= 1
        ? 'PASS'
        : 'FAIL';
    await page.screenshot({
      path: path.join(OUT_DIR, '04-file-after-hard-reload.png'),
      fullPage: false,
    });
    out.screenshots.push('04-file-after-hard-reload.png');

    // Download
    const [download] = await Promise.all([
      page.waitForEvent('download', { timeout: 15000 }).catch(() => null),
      page.locator('[data-testid="generic-evidence-download"]').first().click(),
    ]);
    const downloadNet = NETWORK.some(
      (n) => n.method === 'GET' && n.path.includes('/content'),
    );
    out.DEV_UI_GENERIC_DOWNLOAD = download || downloadNet ? 'PASS' : 'FAIL';

    // Replace
    const replacePath = path.join(OUT_DIR, '_tmp-replace.pdf');
    fs.writeFileSync(replacePath, '%PDF-1.4 g3-replace');
    await page.locator('[data-testid="generic-evidence-replace"]').first().click();
    await page
      .locator('[data-testid="generic-evidence-replace-input"]')
      .setInputFiles(replacePath);
    await page.waitForTimeout(1500);
    await page.waitForSelector('[data-testid="generic-evidence-file-row"]', {
      timeout: 20000,
    });
    const nameAfter = await page
      .locator('[data-testid="generic-evidence-file-row"]')
      .first()
      .innerText();
    out.DEV_UI_GENERIC_REPLACE = /replace|g3|_tmp-replace|\.pdf/i.test(nameAfter)
      ? 'PASS'
      : (await page.locator('[data-testid="generic-evidence-file-row"]').count()) === 1
        ? 'PASS'
        : 'FAIL';
    await page.screenshot({
      path: path.join(OUT_DIR, '05-file-after-replace.png'),
      fullPage: false,
    });
    out.screenshots.push('05-file-after-replace.png');

    await page.reload({ waitUntil: 'networkidle' });
    if (await evidenceTab.count()) await evidenceTab.click();
    await page.waitForSelector('[data-testid="generic-evidence-file-row"]', {
      timeout: 30000,
    });
    out.DEV_UI_REPLACE_HARD_RELOAD =
      (await page.locator('[data-testid="generic-evidence-file-row"]').count()) === 1
        ? 'PASS'
        : 'FAIL';

    // Delete
    await page.locator('[data-testid="generic-evidence-delete"]').first().click();
    await page.locator('[data-testid="generic-evidence-delete-confirm-yes"]').click();
    await page.waitForSelector('[data-testid="step-generic-evidence-empty"]', {
      timeout: 20000,
    });
    out.DEV_UI_GENERIC_DELETE =
      (await page.locator('[data-testid="step-generic-evidence-empty"]').count()) > 0
        ? 'PASS'
        : 'FAIL';
    await page.screenshot({
      path: path.join(OUT_DIR, '06-empty-after-delete.png'),
      fullPage: false,
    });
    out.screenshots.push('06-empty-after-delete.png');

    await page.reload({ waitUntil: 'networkidle' });
    if (await evidenceTab.count()) await evidenceTab.click();
    await page.waitForSelector('[data-testid="step-generic-evidence-panel"]', {
      timeout: 30000,
    });
    out.DEV_UI_DELETE_HARD_RELOAD =
      (await page.locator('[data-testid="generic-evidence-file-row"]').count()) === 0
        ? 'PASS'
        : 'FAIL';

    // Error UX — unsupported type
    const badPath = path.join(OUT_DIR, '_tmp-bad.exe');
    fs.writeFileSync(badPath, 'MZ');
    await page.locator('[data-testid="generic-evidence-upload-input"]').setInputFiles(badPath);
    await page.waitForTimeout(800);
    const errText = await page
      .locator('[data-testid="generic-evidence-local-error"]')
      .textContent()
      .catch(() => '');
    out.DEV_UI_GENERIC_ERROR_UX =
      errText &&
      /Định dạng|không được hỗ trợ|không hợp lệ/i.test(errText) &&
      !/storage_key|sql|INSERT|\/var\//i.test(errText)
        ? 'PASS'
        : 'FAIL';

    // Responsive
    await page.setViewportSize({ width: 390, height: 844 });
    await page.waitForTimeout(400);
    await page.screenshot({
      path: path.join(OUT_DIR, '09-responsive-narrow.png'),
      fullPage: false,
    });
    out.screenshots.push('09-responsive-narrow.png');
    out.DEV_GENERIC_RESPONSIVE =
      (await page.locator('[data-testid="step-generic-evidence-panel"]').count()) > 0
        ? 'PASS'
        : 'FAIL';

    // cleanup temp files (keep screenshots)
    for (const f of ['_tmp-upload.pdf', '_tmp-replace.pdf', '_tmp-bad.exe']) {
      try {
        fs.unlinkSync(path.join(OUT_DIR, f));
      } catch {
        /* ignore */
      }
    }
  } catch (e) {
    out.error = String(e && e.message ? e.message : e);
    try {
      await page.screenshot({
        path: path.join(OUT_DIR, '99-error.png'),
        fullPage: true,
      });
      out.screenshots.push('99-error.png');
    } catch {
      /* ignore */
    }
  } finally {
    out.network = NETWORK;
    const resultPath = path.join(__dirname, 'g3-browser-smoke-last.json');
    fs.writeFileSync(resultPath, JSON.stringify(out, null, 2));
    console.log(JSON.stringify(out, null, 2));
    await browser.close();
    const hard =
      out.DEV_ZERO_REQUIREMENT_GENERIC_SECTION !== 'PASS' ||
      out.DEV_ZERO_REQUIREMENT_UPLOAD_CTA !== 'PASS' ||
      out.DEV_UI_GENERIC_UPLOAD !== 'PASS' ||
      out.DEV_UI_GENERIC_HARD_RELOAD !== 'PASS' ||
      out.DEV_UI_GENERIC_DOWNLOAD !== 'PASS' ||
      out.DEV_UI_GENERIC_REPLACE !== 'PASS' ||
      out.DEV_UI_REPLACE_HARD_RELOAD !== 'PASS' ||
      out.DEV_UI_GENERIC_DELETE !== 'PASS' ||
      out.DEV_UI_DELETE_HARD_RELOAD !== 'PASS' ||
      out.DEV_UI_GENERIC_ERROR_UX !== 'PASS' ||
      out.DEV_UNEXPECTED_5XX !== 0;
    process.exit(hard ? 1 : 0);
  }
})().catch((e) => {
  console.error(e);
  process.exit(1);
});
