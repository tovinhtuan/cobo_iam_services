import { chromium } from 'playwright';
import fs from 'fs';

const BASE = 'http://88.216.208.0:3000';
const evidenceDir = 'C:/Users/tvttt/OneDrive/Desktop/cobo/cobo_web/cobo_iam_services/docs/ai-cache/workflow-override-foundation-batch2/evidence';
const screenshotsDir = `${evidenceDir}/screenshots`;
fs.mkdirSync(screenshotsDir, { recursive: true });

const networkLog = [];

(async () => {
  const browser = await chromium.launch();
  const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } });

  page.on('response', async (resp) => {
    const url = resp.url();
    if (url.includes('/workflow/override/')) {
      let body = null;
      try { body = await resp.json(); } catch { /* ignore */ }
      networkLog.push({ url, status: resp.status(), method: resp.request().method(), body });
    }
  });

  await page.goto(`${BASE}/login`);
  await page.waitForTimeout(1000);
  await page.screenshot({ path: `${screenshotsDir}/debug-01-login-page.png`, fullPage: true });
  await page.fill('input[type="email"]', 'tvttthptlvh@gmail.com');
  await page.fill('input[type="password"]', 'bichhanh0701');
  await page.click('button[type="submit"]');

  await page.waitForTimeout(2000);
  await page.screenshot({ path: `${screenshotsDir}/debug-02-after-submit.png`, fullPage: true });
  console.log('URL after submit:', page.url());

  // Company selection step: click the company row, then confirm with "Tiếp tục"
  const companySelector = page.locator('text=CTCP Nhựa An Phát Xanh');
  if (await companySelector.count() > 0) {
    await companySelector.first().click();
    await page.waitForTimeout(500);
    const continueBtn = page.locator('button:has-text("Tiếp tục")');
    if (await continueBtn.count() > 0) {
      await continueBtn.first().click();
    }
    await page.waitForTimeout(1500);
    await page.screenshot({ path: `${screenshotsDir}/debug-03-after-company-select.png`, fullPage: true });
    console.log('URL after company select:', page.url());
  } else {
    console.log('Company selector not found. Page text snapshot:');
    console.log((await page.locator('body').textContent()).slice(0, 1000));
  }

  await page.waitForURL('**/app/**', { timeout: 15000 }).catch(() => {});

  await page.goto(`${BASE}/app/disclosure-types/bao-cao-tai-chinh-quy-1`);
  await page.waitForTimeout(2500);
  await page.screenshot({ path: `${screenshotsDir}/debug-04-type-detail.png`, fullPage: true });
  console.log('URL at type detail:', page.url());

  const badge = page.locator('[data-testid="workflow-override-staleness-badge"]');
  await badge.first().scrollIntoViewIfNeeded();
  await page.waitForTimeout(300);
  await page.screenshot({ path: `${screenshotsDir}/status-badge-before.png` });
  const badgeText = await badge.first().textContent().catch(() => null);

  const ctaButton = page.locator('button:has-text("Kiểm tra cập nhật")');
  const ctaExists = await ctaButton.count() > 0;
  if (ctaExists) {
    await ctaButton.first().click();
    await page.waitForTimeout(2000);
  }

  await badge.first().scrollIntoViewIfNeeded();
  await page.waitForTimeout(300);
  await page.screenshot({ path: `${screenshotsDir}/status-badge-after.png` });
  const badgeTextAfter = await badge.first().textContent().catch(() => null);

  const bodyText = await page.locator('body').textContent();
  const forbiddenTerms = ['xem trước', 'preview', 'xung đột', 'conflict', 'áp dụng cập nhật', 'rebase apply'];
  const forbiddenFound = forbiddenTerms.filter((t) => bodyText.toLowerCase().includes(t.toLowerCase()));

  fs.writeFileSync(`${evidenceDir}/network-log.json`, JSON.stringify(networkLog, null, 2));
  fs.writeFileSync(`${evidenceDir}/ui-smoke-result.json`, JSON.stringify({
    badgeTextBefore: badgeText,
    badgeTextAfter: badgeTextAfter,
    ctaExists,
    forbiddenTermsFound: forbiddenFound,
  }, null, 2));

  console.log('badgeTextBefore:', badgeText);
  console.log('badgeTextAfter:', badgeTextAfter);
  console.log('ctaExists:', ctaExists);
  console.log('forbiddenTermsFound:', forbiddenFound);
  console.log('networkLog entries:', networkLog.length);

  await browser.close();
})();
