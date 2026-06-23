import { chromium } from 'playwright';
import fs from 'fs';

const BASE = 'http://88.216.208.0:3000';
const evidenceDir = 'C:/Users/tvttt/OneDrive/Desktop/cobo/cobo_web/cobo_iam_services/docs/ai-cache/workflow-override-foundation-batch5/evidence';
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
  await page.fill('input[type="email"]', 'tvttthptlvh@gmail.com');
  await page.fill('input[type="password"]', 'bichhanh0701');
  await page.click('button[type="submit"]');
  await page.waitForTimeout(2000);

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
  await page.waitForURL('**/app/**', { timeout: 15000 }).catch(() => {});

  await page.goto(`${BASE}/app/disclosure-types/bao-cao-tai-chinh-quy-1`);
  await page.waitForTimeout(2500);
  await page.screenshot({ path: `${screenshotsDir}/portal-before-apply.png`, fullPage: true });

  // Open the rebase preview modal (real, fresh staleness/conflict fixture is live on DEV now).
  const previewBtn = page.locator('button:has-text("Xem trước cập nhật")');
  await previewBtn.first().click();
  await page.waitForTimeout(2000);

  // Apply button should be disabled — the Rule 1 conflict is unresolved.
  const applyBtn = page.locator('[data-testid="workflow-override-apply-button"]');
  await applyBtn.scrollIntoViewIfNeeded();
  await page.waitForTimeout(300);
  await page.screenshot({ path: `${screenshotsDir}/apply-disabled-unresolved.png`, fullPage: true });
  const disabledBefore = await applyBtn.isDisabled().catch(() => null);

  // Resolve the conflict for real, in the browser, by clicking a resolution option.
  const resolveBtn = page.locator('button:has-text("Chấp nhận hệ thống")');
  if (await resolveBtn.count() > 0) {
    await resolveBtn.first().click();
    await page.waitForTimeout(2000);
  }

  await applyBtn.scrollIntoViewIfNeeded();
  await page.waitForTimeout(300);
  await page.screenshot({ path: `${screenshotsDir}/apply-enabled-resolved.png`, fullPage: true });
  const disabledAfterResolve = await applyBtn.isDisabled().catch(() => null);

  // Click apply for real.
  await applyBtn.click();
  await page.waitForTimeout(2500);
  await page.screenshot({ path: `${screenshotsDir}/apply-success.png`, fullPage: true });
  const successVisible = await page.locator('[data-testid="workflow-override-apply-success"]').count();

  // Close modal, screenshot Portal baseline view reflecting the new applied content.
  const closeBtn = page.locator('button[aria-label="Đóng"]');
  if (await closeBtn.count() > 0) {
    await closeBtn.first().click();
  }
  await page.waitForTimeout(1500);
  await page.screenshot({ path: `${screenshotsDir}/portal-after-apply.png`, fullPage: true });

  const bodyTextAfter = await page.locator('body').textContent();

  fs.writeFileSync(`${evidenceDir}/network-log.json`, JSON.stringify(networkLog, null, 2));
  fs.writeFileSync(`${evidenceDir}/ui-smoke-result.json`, JSON.stringify({
    disabledBeforeResolve: disabledBefore,
    disabledAfterResolve: disabledAfterResolve,
    applySuccessVisible: successVisible > 0,
    portalContainsAppliedStage: bodyTextAfter.includes('REBASE-TEST CMS V3 Stage') || bodyTextAfter.includes('SmokeQA C-Fix Stage Edit'),
  }, null, 2));

  console.log('disabledBeforeResolve:', disabledBefore);
  console.log('disabledAfterResolve:', disabledAfterResolve);
  console.log('applySuccessVisible:', successVisible > 0);

  await browser.close();
})();
