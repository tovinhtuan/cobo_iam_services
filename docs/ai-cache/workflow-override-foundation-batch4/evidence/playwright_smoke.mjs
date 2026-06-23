import { chromium } from 'playwright';
import fs from 'fs';

const BASE = 'http://88.216.208.0:3000';
const evidenceDir = 'C:/Users/tvttt/OneDrive/Desktop/cobo/cobo_web/cobo_iam_services/docs/ai-cache/workflow-override-foundation-batch4/evidence';
const screenshotsDir = `${evidenceDir}/screenshots`;
fs.mkdirSync(screenshotsDir, { recursive: true });

const networkLog = [];

(async () => {
  const browser = await chromium.launch();
  const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } });
  page.on('console', (msg) => { if (msg.type() === 'error') console.log('CONSOLE ERROR:', msg.text()); });

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

  const previewButton = page.locator('button:has-text("Xem trước cập nhật")');
  await previewButton.first().click();
  await page.waitForTimeout(2000);

  const modalOpened = (await page.locator('text=Xem trước cập nhật workflow').count()) > 0;

  // Scenario 6: conflict list visible, severity visible, resolution options visible.
  await page.screenshot({ path: `${screenshotsDir}/conflict-list.png` });
  const conflictRowCount = await page.locator('[data-testid="workflow-override-conflict-row"]').count();
  const severityVisible = (await page.locator('text=Cảnh báo').count()) > 0;
  const resolutionButton = page.locator('button:has-text("Không áp dụng")');
  const resolutionOptionsVisible = (await resolutionButton.count()) > 0;
  await page.screenshot({ path: `${screenshotsDir}/conflict-resolution-options.png` });

  // Click resolve for real, via the browser.
  let resolvedAfterClick = false;
  if (resolutionOptionsVisible) {
    await resolutionButton.first().click();
    await page.waitForTimeout(2000);
    resolvedAfterClick = (await page.locator('text=/Đã xử lý/').count()) > 0;
  }
  await page.screenshot({ path: `${screenshotsDir}/conflict-resolved.png` });

  const bodyTextLower = (await page.locator('body').textContent()).toLowerCase();
  const applyButtonExists = (await page.locator('button:has-text("Áp dụng")').count()) > 0;
  const forbiddenTerms = ['áp dụng cập nhật', 'apply update'];
  const forbiddenFound = forbiddenTerms.filter((t) => bodyTextLower.includes(t.toLowerCase()));
  await page.screenshot({ path: `${screenshotsDir}/no-apply-button.png` });

  fs.writeFileSync(`${evidenceDir}/network-log.json`, JSON.stringify(networkLog, null, 2));
  fs.writeFileSync(`${evidenceDir}/ui-smoke-result.json`, JSON.stringify({
    modalOpened, conflictRowCount, severityVisible, resolutionOptionsVisible, resolvedAfterClick,
    applyButtonExists, forbiddenFound,
  }, null, 2));

  console.log('modalOpened:', modalOpened);
  console.log('conflictRowCount:', conflictRowCount);
  console.log('severityVisible:', severityVisible);
  console.log('resolutionOptionsVisible:', resolutionOptionsVisible);
  console.log('resolvedAfterClick:', resolvedAfterClick);
  console.log('applyButtonExists:', applyButtonExists);
  console.log('forbiddenFound:', forbiddenFound);
  console.log('networkLog entries:', networkLog.length);

  await browser.close();
})();
