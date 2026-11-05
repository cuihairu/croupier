// Evidence capture: log in to the operator console and screenshot the sidebar.
//
// Part of the DOC-001 config-switch evidence pipeline (run via
// scripts/docs/config-switch-evidence.sh). Captures the authenticated sidebar
// so the off/on difference of a feature flag (a top-level menu appearing or
// disappearing) is visible.
//
// Usage: node config-evidence-capture.js <outPng> [--full]
// Env:   BASE (default http://localhost:8001), USER_NAME, PASS
//
// Requires @playwright/test resolvable (the caller sets NODE_PATH to
// web/node_modules). Browsers must be installed (npx playwright install).
const { chromium } = require('@playwright/test');

const out = process.argv[2];
const full = process.argv.includes('--full');
const BASE = process.env.BASE || 'http://localhost:8001';
const USER = process.env.USER_NAME || 'admin';
const PASS = process.env.PASS || 'admin123';

(async () => {
  const browser = await chromium.launch({ headless: true });
  const context = await browser.newContext({
    viewport: { width: 1440, height: 900 },
    locale: 'zh-CN',
    baseURL: BASE,
  });
  const page = await context.newPage();
  const errors = [];
  page.on('pageerror', (e) => errors.push(String(e).slice(0, 200)));

  try {
    await page.goto('/user/login', { waitUntil: 'domcontentloaded' });
    const username = page
      .locator('input[id="username"], input[placeholder*="admin"], input[placeholder*="用户名"]')
      .first();
    await username.waitFor({ state: 'visible', timeout: 90000 });
    await username.fill(USER);
    await page.locator('input[type="password"]').fill(PASS);
    await page
      .locator('button[type="submit"], button:has-text("Login")')
      .or(page.locator('button', { hasText: /登\s*录/ }))
      .first()
      .click();
    await page.waitForURL(
      (url) => /\/(console|dashboard)/.test(url.pathname) || url.pathname === '/',
      { timeout: 40000 },
    );
    await page.waitForFunction(
      () =>
        Boolean(window.localStorage.getItem('token')) &&
        Boolean(window.localStorage.getItem('game_id')),
      undefined,
      { timeout: 40000, polling: 500 },
    );
    await page.goto('/console/home', { waitUntil: 'networkidle' });
    await page
      .locator('.ant-layout-sider, aside')
      .first()
      .waitFor({ state: 'visible', timeout: 40000 });
    await page.waitForTimeout(1500);
    if (full) {
      await page.screenshot({ path: out, fullPage: false });
    } else {
      const sider = page.locator('.ant-layout-sider, aside').first();
      await sider.screenshot({ path: out });
    }
    const labels = await page
      .locator(
        '.ant-layout-sider li, .ant-layout-sider .ant-menu-item, .ant-layout-sider .ant-menu-submenu-title',
      )
      .allTextContents();
    console.log('CAPTURED', out);
    console.log('MENU_LABELS', JSON.stringify(labels.map((s) => s.trim()).filter(Boolean)));
    if (errors.length) console.log('PAGE_ERRORS', JSON.stringify(errors));
  } catch (e) {
    console.log('CAPTURE_FAIL', String(e).slice(0, 600));
    process.exit(1);
  } finally {
    await browser.close();
  }
})();
