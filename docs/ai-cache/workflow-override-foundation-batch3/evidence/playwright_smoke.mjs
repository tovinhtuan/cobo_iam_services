import { chromium } from 'playwright';
import fs from 'fs';

const BASE = 'http://88.216.208.0:3000';
const evidenceDir = 'C:/Users/tvttt/OneDrive/Desktop/cobo/cobo_web/cobo_iam_services/docs/ai-cache/workflow-override-foundation-batch3/evidence';
const screenshotsDir = `${evidenceDir}/screenshots`;
fs.mkdirSync(screenshotsDir, { recursive: true });

const networkLog = [];

(async () => {
  const browser = await chromium.launch();
  const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } });
  page.on('console', (msg) => console.log('CONSOLE:', msg.type(), msg.text()));
  page.on('pageerror', (err) => console.log('PAGE ERROR:', err.message));

  page.on('response', async (resp) => {
    const url = resp.url();
    if (url.includes('/workflow/override/')) {
      let body = null;
      try { body = await resp.json(); } catch { /* ignore */ }
      networkLog.push({ url, status: resp.status(), method: resp.request().method(), body });
    }
  });

  await page.goto(`${BASE}/login`);
  await page.fill('input[type="email"]', 'tvttthptlvh@gmail.com');
  await page.fill('input[type="password"]', 'bichhanh0701');
  await page.click('button[type="submit"]');
  await page.waitForTimeout(1500);

  const companySelector = page.locator('text=CTCP Nhựa An Phát Xanh');
  if (await companySelector.count() > 0) {
    await companySelector.first().click();
    await page.waitForTimeout(500);
    const continueBtn = page.locator('button:has-text("Tiếp tục")');
    if (await continueBtn.count() > 0) {
      await continueBtn.first().click();
    }
    await page.waitForTimeout(1500);
  }

  await page.goto(`${BASE}/app/disclosure-types/bao-cao-tai-chinh-quy-1`);
  await page.waitForTimeout(2500);

  const badge = page.locator('[data-testid="workflow-override-staleness-badge"]');
  await badge.first().scrollIntoViewIfNeeded();
  await page.waitForTimeout(300);
  const badgeText = await badge.first().textContent().catch(() => null);

  const previewButton = page.locator('button:has-text("Xem trước cập nhật")');
  const previewButtonExists = await previewButton.count() > 0;
  await page.screenshot({ path: `${screenshotsDir}/preview-button.png` });

  let modalOpened = false;
  let bodyTextLower = '';
  if (previewButtonExists) {
    await previewButton.first().click();
    await page.waitForTimeout(2000);
    modalOpened = (await page.locator('text=Xem trước cập nhật workflow').count()) > 0;
    await page.screenshot({ path: `${screenshotsDir}/preview-modal.png` });
    bodyTextLower = (await page.locator('body').textContent()).toLowerCase();
    await page.screenshot({ path: `${screenshotsDir}/no-apply-button.png` });
  }

  const forbiddenTerms = ['áp dụng cập nhật', 'apply update', 'giữ tùy chỉnh', 'chấp nhận hệ thống'];
  const forbiddenFound = forbiddenTerms.filter((t) => bodyTextLower.includes(t.toLowerCase()));
  const applyButtonExists = (await page.locator('button:has-text("Áp dụng")').count()) > 0;
  const resolveButtonExists = (await page.locator('button:has-text("Resolve")').count()) > 0;

  fs.writeFileSync(`${evidenceDir}/network-log.json`, JSON.stringify(networkLog, null, 2));
  fs.writeFileSync(`${evidenceDir}/ui-smoke-result.json`, JSON.stringify({
    badgeText, previewButtonExists, modalOpened, forbiddenFound, applyButtonExists, resolveButtonExists,
  }, null, 2));

  console.log('badgeText:', badgeText);
  console.log('previewButtonExists:', previewButtonExists);
  console.log('modalOpened:', modalOpened);
  console.log('forbiddenFound:', forbiddenFound);
  console.log('applyButtonExists:', applyButtonExists);
  console.log('resolveButtonExists:', resolveButtonExists);
  console.log('networkLog entries:', networkLog.length);

  await browser.close();
})();
