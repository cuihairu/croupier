/**
 * Capture "after" screenshots for the 2026-10-31 UI audit report.
 *
 * Reproduces the audit's en-US interface (baseNavigator uses navigator.language)
 * against the live real backend, and captures the same views as the "before"
 * evidence so each fixed item has a before/after pair.
 *
 * Usage:
 *   node --import tsx scripts/capture-audit-after.mjs [baseURL] [outDir]
 *
 * The script logs in as admin/admin123, sets a default game/env scope in
 * localStorage (the header selector reads these), and visits each target view
 * at desktop (1440x900) and narrow (390x844) viewports.
 */
import fs from 'node:fs';
import path from 'node:path';
import { chromium } from '@playwright/test';

const BASE = process.argv[2] || 'http://127.0.0.1:8000';
const OUT = path.resolve(process.argv[3] || '../../docs/review/shots-after');

// Same chromium fallback logic as console-audit.mjs.
function resolveChromium() {
  const cacheRoot = path.join(process.env.HOME || '/root', '.cache', 'ms-playwright');
  let candidates = [];
  try {
    candidates = fs
      .readdirSync(cacheRoot)
      .filter((d) => d.startsWith('chromium-'))
      .sort()
      .reverse();
  } catch {
    /* fall through */
  }
  for (const dir of candidates) {
    for (const rel of ['chrome-linux64/chrome', 'chrome-linux/chrome']) {
      const exe = path.join(cacheRoot, dir, rel);
      if (fs.existsSync(exe)) return { executablePath: exe };
    }
  }
  return {};
}

async function login(page) {
  await page.goto('/user/login', { waitUntil: 'domcontentloaded' });
  const username = page
    .locator('input[id="username"], input[name="username"], input[placeholder*="admin"]')
    .first();
  await username.waitFor({ state: 'visible', timeout: 60000 });
  await username.fill('admin');
  await page.locator('input[name="password"], input[type="password"]').first().fill('admin123');
  await page
    .locator('button[type="submit"], button:has-text("Login")')
    .or(page.locator('button', { hasText: /登\s*录/ }))
    .first()
    .click();
  await page.waitForURL((u) => /\/(console|dashboard)/.test(u.pathname) || u.pathname === '/', {
    timeout: 30000,
  });
  await page.waitForFunction(
    () =>
      Boolean(window.localStorage.getItem('token')) &&
      Boolean(window.localStorage.getItem('game_id')) &&
      Boolean(window.localStorage.getItem('env')),
    undefined,
    { timeout: 30000, polling: 500 },
  );
}

// Views to capture, mapped to the audit issue they document.
const DESKTOP = [
  { name: 'games-manage-after', route: '/system/games', issue: 'F3' },
  { name: 'site-settings-after', route: '/system/site', issue: 'F4' },
  { name: 'games-envs-managegames-after', route: '/system/environments', issue: 'F6' },
  { name: 'functions-catalog-after', route: '/functions/catalog', issue: 'F7' },
  { name: 'page-studio-after', route: '/functions/pages', issue: 'F7' },
];
const NARROW = [
  { name: 'narrow-title-after', route: '/functions/catalog', issue: 'F9', width: 390, height: 844 },
];

// Wait for the route's content shell to render (not just HTML shell). Dev
// first-compile can hang route chunk requests for tens of seconds, leaving a
// blank content area (~20 DOM nodes). Poll until the app has rendered a real
// layout; re-goto once if the first pass stays blank (stuck lazy chunk).
async function settle(page, route) {
  const nodeCount = () => page.evaluate(() => document.querySelectorAll('*').length);
  const deadline = Date.now() + 120000;
  let retried = false;
  while (Date.now() < deadline) {
    const nodes = await nodeCount();
    if (nodes > 150) {
      // Layout mounted; let async data/tables finish one beat.
      await page.waitForTimeout(2000);
      return nodes;
    }
    if (!retried && Date.now() > deadline - 60000) {
      retried = true;
      await page.goto(route, { waitUntil: 'domcontentloaded', timeout: 30000 }).catch(() => {});
    }
    await page.waitForTimeout(1500);
  }
  return nodeCount();
}

async function main() {
  fs.mkdirSync(OUT, { recursive: true });
  const browser = await chromium.launch(resolveChromium());

  // ---- desktop: en-US interface, same shell as the audit "before" shots ----
  const ctx = await browser.newContext({
    baseURL: BASE,
    locale: 'en-US',
    viewport: { width: 1440, height: 900 },
    deviceScaleFactor: 1,
  });
  const page = await ctx.newPage();
  page.on('pageerror', (e) => console.log('[pageerror]', String(e).slice(0, 160)));
  await login(page);
  console.log('[capture] logged in (en-US, desktop)');

  for (const { name, route, issue } of DESKTOP) {
    try {
      await page.goto(route, { waitUntil: 'domcontentloaded', timeout: 30000 });
      const nodes = await settle(page, route);
      const file = path.join(OUT, `${name}.png`);
      await page.screenshot({ path: file, fullPage: false });
      console.log(`[capture] ${issue} ${route} -> ${name}.png (nodes=${nodes})`);
    } catch (e) {
      console.error(`[capture] FAIL ${issue} ${route}: ${String(e).slice(0, 200)}`);
    }
  }

  // ---- F8: breadcrumb contrast pair ----
  // Current code renders the accessible #0958d9 ("after"). The "before" is the
  // pre-fix #1677ff; recreate it by injecting the exact old color so the pair
  // is visually comparable (report documents the computed ratios 4.12:1/5.57:1).
  try {
    await page.goto('/functions/catalog', { waitUntil: 'domcontentloaded', timeout: 30000 });
    await settle(page, '/functions/catalog');
    const clip = { x: 0, y: 0, width: 1440, height: 110 };
    await page.screenshot({ path: path.join(OUT, 'breadcrumb-after.png'), clip });
    await page.addStyleTag({
      content: '.ant-breadcrumb a,.ant-breadcrumb-link a{color:#1677ff !important;text-decoration:none !important;}',
    });
    await page.waitForTimeout(300);
    await page.screenshot({ path: path.join(OUT, 'breadcrumb-before.png'), clip });
    console.log('[capture] F8 breadcrumb pair captured');
  } catch (e) {
    console.error(`[capture] FAIL F8 breadcrumb: ${String(e).slice(0, 200)}`);
  }

  await ctx.close();

  // ---- narrow: en-US, 390x844, reproduces the F9 truncation view ----
  for (const { name, route, issue, width, height } of NARROW) {
    const nctx = await browser.newContext({
      baseURL: BASE,
      locale: 'en-US',
      viewport: { width, height },
      deviceScaleFactor: 1,
    });
    const np = await nctx.newPage();
    await login(np);
    try {
      await np.goto(route, { waitUntil: 'domcontentloaded', timeout: 30000 });
      await settle(np, route);
      const file = path.join(OUT, `${name}.png`);
      await np.screenshot({ path: file, fullPage: false });
      console.log(`[capture] ${issue} ${route} (narrow) -> ${name}.png`);
    } catch (e) {
      console.error(`[capture] FAIL ${issue} ${route} (narrow): ${String(e).slice(0, 200)}`);
    }
    await nctx.close();
  }

  await browser.close();
  console.log(`[capture] done -> ${OUT}`);
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});
