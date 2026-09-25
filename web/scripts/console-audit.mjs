/**
 * Frontend console audit against the live local stack.
 *
 * Reproduces the interaction paths that the antd-6 migration touched and
 * records every console warning/error/pageerror per route, so deprecation
 * regressions are observable instead of silent.
 *
 * Usage:
 *   node --import tsx scripts/console-audit.mjs [baseURL]
 */
import fs from 'node:fs';
import path from 'node:path';
import { chromium } from '@playwright/test';

const BASE = process.argv[2] || 'http://127.0.0.1:8000';

// Routes to audit. The first block is derived from the components that carried
// antd 6 deprecated props; the rest widens coverage to the route groups that
// were not represented at all (`/admin/*`, `/dev/*`, `/support/*`, `/system/*`,
// `/ops/*` remainder), so the "console is clean" claim is not scoped to a
// hand-picked subset. Paths come from web/config/routes.ts.
const ROUTES = [
  // ---- components with known antd 6 deprecated props ----
  ['/functions/instances', 'Drawer width / Alert message'],
  ['/functions/execution-logs', 'Drawer width / Alert message'],
  ['/functions/catalog', 'Drawer width / Alert message / Space direction'],
  ['/functions/openapi-sources', 'Alert message / Input addonBefore / Space direction'],
  ['/functions/pages/composite-editor', 'Space direction / Drawer width / Alert message'],
  ['/functions/component-templates', 'Alert message / Space direction'],
  ['/ops/cluster', 'Drawer width'],
  ['/ops/schedules', 'Drawer width'],
  ['/ops/alerts', 'Drawer width'],
  ['/ops/lb', 'Gauge/Progress + Statistic styles'],
  ['/account/center', 'Alert message'],
  ['/account/settings', 'Alert message / Modal'],
  ['/analytics/behavior', 'InputNumber addonBefore'],
  ['/analytics/invocations', 'Statistic valueStyle'],
  ['/approvals', 'Drawer width'],
  ['/operations/extensions/installations', 'Drawer width / Alert message'],
  ['/console/home', 'Alert message / Space direction'],
  ['/functions/invoke', 'Alert message'],
  ['/functions/menus', 'MenuTree ICU placeholder (排序 {order})'],
  ['/functions/resource-catalog', 'Alert message'],

  // ---- route groups not represented above ----
  ['/admin/account/center', 'Profile hero Avatar + InfoTab form'],
  ['/admin/permissions/users', 'admin tables'],
  ['/admin/permissions/roles', 'admin tables'],
  ['/admin/operation-logs', 'audit log page'],
  ['/admin/login-logs', 'login log page'],
  ['/dev/bugs', 'Modal width / dev tools'],
  ['/dev/config-explorer', 'Modal width / dev tools'],
  ['/dev/configs', 'Modal width / dev tools'],
  ['/dev/tools', 'Modal width / dev tools'],
  ['/support/tickets', 'Modal width / support'],
  ['/support/faq', 'Modal width / support'],
  ['/support/feedback', 'Modal width / support'],
  ['/system/foundation/site', 'system settings'],
  ['/system/foundation/environments', 'system settings'],
  ['/system/foundation/terms', 'system settings'],
  ['/system/foundation/analytics-filters', 'system settings'],
  ['/system/component-management', 'system component mgmt'],
  ['/system/extensions/store', 'extension store'],
  ['/ops/backups', 'Modal width / ops'],
  ['/ops/certificates', 'Modal width / ops'],
  ['/ops/notifications', 'Modal width / ops'],
  ['/ops/mq', 'ops'],
  ['/ops/registry', 'ops'],
  ['/ops/rate-limits', 'ops'],
  ['/ops/services', 'ops'],
  ['/ops/servers', 'ops'],
  ['/ops/maintenance', 'ops'],
  ['/ops/health', 'ops'],
  ['/functions/assignments', 'Divider type / ListTab'],
  ['/functions/proposals', 'proposal inbox'],
  ['/functions/sdk-distribution', 'sdk distribution'],
  ['/functions/warnings', 'Alert message'],
  ['/analytics/overview', 'analytics'],
  ['/analytics/levels', 'analytics'],
  ['/analytics/payments', 'analytics'],
  ['/analytics/realtime', 'analytics'],
  ['/analytics/retention', 'analytics'],
  ['/analytics/segments', 'analytics'],
  ['/analytics/warehouse', 'Alert message'],
  ['/operations/extensions/store', 'Alert message'],
  ['/operations/extensions/agent-sync', 'extensions'],
];

const IGNORE = [
  /Download the React DevTools/i,
  /favicon/i,
  /\[vite\]/i,
  /webpack/i,
  /Failed to load resource.*favicon/i,
];

function bucket(msgType, text) {
  if (IGNORE.some((r) => r.test(text))) return null;
  return { msgType, text };
}

/**
 * Resolve a usable Chromium binary.
 *
 * `chromium.launch()` pins an exact build revision. This machine has a newer
 * `chromium-*` build than the one @playwright/test expects (and no
 * headless-shell), so fall back to whatever full build is on disk. That keeps
 * the audit runnable offline instead of requiring `playwright install`.
 */
function resolveChromium() {
  const cacheRoot = path.join(
    process.env.HOME || '/root',
    '.cache',
    'ms-playwright',
  );
  let candidates = [];
  try {
    candidates = fs
      .readdirSync(cacheRoot)
      .filter((d) => d.startsWith('chromium-'))
      .sort()
      .reverse();
  } catch {
    /* cache root missing -> let Playwright use its own resolution */
  }
  for (const dir of candidates) {
    for (const rel of [
      'chrome-linux64/chrome',
      'chrome-linux/chrome',
    ]) {
      const exe = path.join(cacheRoot, dir, rel);
      if (fs.existsSync(exe)) return { executablePath: exe };
    }
  }
  return {};
}

async function main() {
  const browser = await chromium.launch(resolveChromium());
  const context = await browser.newContext({ baseURL: BASE, locale: 'zh-CN' });
  const page = await context.newPage();

  const buckets = new Map();
  const routeRecords = [];
  // Current route label. Console events fire asynchronously and `page.url()`
  // mid-navigation is unreliable (it still reports the previous route), so
  // track the label explicitly and stamp each message as it arrives.
  let current = '<login>';

  page.on('console', (msg) => {
    const t = msg.type();
    if (t !== 'error' && t !== 'warning') return;
    const b = bucket(t, msg.text());
    if (!b) return;
    const list = buckets.get(current) || [];
    list.push(b);
    buckets.set(current, list);
  });
  page.on('pageerror', (err) => {
    const list = buckets.get(current) || [];
    list.push({ msgType: 'pageerror', text: String(err) });
    buckets.set(current, list);
  });

  // ---- login ----
  await page.goto('/user/login', { waitUntil: 'domcontentloaded' });
  const username = page
    .locator('input[id="username"], input[placeholder*="admin"], input[placeholder*="用户名"]')
    .first();
  await username.waitFor({ state: 'visible', timeout: 60000 });
  await username.fill('admin');
  await page.locator('input[type="password"]').fill('admin123');
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
  console.log('[audit] logged in');

  // ---- visit each route ----
  for (const [route, why] of ROUTES) {
    current = route;
    buckets.set(route, []);
    try {
      await page.goto(route, { waitUntil: 'domcontentloaded', timeout: 30000 });
      await page.waitForTimeout(3000);
      const msgs = buckets.get(route) || [];
      routeRecords.push({
        route,
        why,
        status: 'ok',
        count: msgs.length,
        // Sanity signal: a blank shell means the route never mounted, so a
        // "clean console" would be a false negative.
        domNodes: await page.evaluate(() => document.querySelectorAll('*').length),
      });
    } catch (e) {
      routeRecords.push({
        route,
        why,
        status: 'nav-failed',
        count: 0,
        error: String(e).slice(0, 200),
      });
    }
  }

  // ---- report ----
  const deprecations = new Map();
  for (const [route, list] of buckets) {
    for (const m of list) {
      if (/deprecated/i.test(m.text)) {
        const key = m.text.replace(/\s+/g, ' ').slice(0, 220);
        deprecations.set(key, (deprecations.get(key) || 0) + 1);
      }
    }
  }

  const allMsgs = [...buckets.entries()].flatMap(([route, list]) =>
    list.map((m) => ({ route, ...m })),
  );

  const out = {
    base: BASE,
    at: new Date().toISOString(),
    routes: routeRecords,
    deprecations: [...deprecations.entries()]
      .map(([text, count]) => ({ text, count }))
      .sort((a, b) => b.count - a.count),
    other: allMsgs
      .filter((m) => !/deprecated/i.test(m.text))
      .map((m) => ({ route: m.route, type: m.msgType, text: m.text.replace(/\s+/g, ' ').slice(0, 240) })),
  };

  const destDir = path.resolve(process.cwd(), 'test-results');
  fs.mkdirSync(destDir, { recursive: true });
  const dest = path.join(destDir, 'console-audit.json');
  fs.writeFileSync(dest, JSON.stringify(out, null, 2));

  console.log(`\n=== DEPRECATION WARNINGS (${out.deprecations.length} distinct) ===`);
  for (const d of out.deprecations) console.log(`  x${d.count}  ${d.text}`);
  console.log(`\n=== OTHER console noise (${out.other.length}) ===`);
  for (const o of out.other.slice(0, 60)) console.log(`  [${o.type}] ${o.route}  ${o.text}`);
  console.log(`\nreport: ${dest}`);

  await browser.close();
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});
