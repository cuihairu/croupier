/**
 * Regenerate the interface-preview screenshots (docs/guide/interface-preview.md).
 *
 * Captures the ten views referenced by the guide against a live, running
 * quick-start stack: the production dashboard build (croupier-quickstart-
 * dashboard, default http://127.0.0.1:18000) in front of the real server
 * (http://127.0.0.1:18780). Output overwrites docs/screenshots/ui/*.png —
 * run scripts/optimize-screenshots.sh afterwards to convert PNGs to WebP
 * and keep the docs tree small.
 *
 * Usage:
 *   node --import tsx scripts/capture-interface-preview.mjs [baseURL] [outDir]
 *
 * View recipes mirror the guide: zh-CN interface, light theme (production
 * builds are pinned to light), 1600x900 @2x. Interactive views reuse the
 * demo data the guide documents: item grant form (04a/04c) and the
 * approval-gated player.delete flow (05), which seeds the approval center
 * shots (06/07).
 */
import fs from 'node:fs';
import path from 'node:path';
import { chromium } from '@playwright/test';

const BASE = process.argv[2] || 'http://127.0.0.1:18000';
const OUT = path.resolve(process.argv[3] || '../docs/screenshots/ui');

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

// Wait for the route's content shell to render (same approach as
// capture-audit-after.mjs: poll DOM size, one re-goto retry on a blank pass).
async function settle(page, route) {
  const nodeCount = () => page.evaluate(() => document.querySelectorAll('*').length);
  const deadline = Date.now() + 120000;
  let retried = false;
  while (Date.now() < deadline) {
    const nodes = await nodeCount();
    if (nodes > 150) {
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

async function shot(page, name) {
  const file = path.join(OUT, `${name}.png`);
  await page.screenshot({ path: file, fullPage: false });
  console.log(`[capture] ${name}.png`);
}

async function goto(page, route, name) {
  await page.goto(route, { waitUntil: 'domcontentloaded', timeout: 30000 });
  await settle(page, route);
  await shot(page, name);
}

// The invoke workbench renders one antd Select for the function (placeholder
// 选择已注册函数 until loaded) and a schema form whose labels are the
// descriptor's property names.
async function pickFunction(page, needle) {
  const combo = page.locator('.ant-select').filter({ hasText: /选择已注册函数|·/ }).first();
  // The descriptor list loads after the scope-ready signal; give it time
  // before forcing a manual refresh.
  try {
    await combo
      .locator('input')
      .first()
      .waitFor({ state: 'visible', timeout: 20000 });
  } catch {
    /* fall through to the refresh path */
  }
  for (let attempt = 0; attempt < 2; attempt += 1) {
    try {
      await combo.click({ timeout: 5000 });
      // antd 6 virtualizes the option list (~10 rendered nodes); type into
      // the searchable select so the target option enters the window.
      await combo.locator('input').first().fill(needle, { timeout: 5000 });
      const option = page.locator('.ant-select-item-option', { hasText: needle }).first();
      await option.waitFor({ state: 'visible', timeout: 8000 });
      await option.click();
      return;
    } catch (e) {
      if (attempt === 1) throw e;
      await page
        .getByRole('button', { name: /刷新函数|Refresh/ })
        .first()
        .click();
      await page.waitForTimeout(4000);
    }
  }
}

// The request body editor opens on the 原始 JSON tab; the schema form (and
// its labelled fields) mounts only after switching to the Schema 表单 tab.
async function ensureSchemaTab(page) {
  const tab = page.getByRole('tab', { name: /Schema 表单/ }).first();
  if (await tab.count()) {
    await tab.click();
    await page.waitForTimeout(600);
  }
}

async function fillField(page, label, value) {
  // antd form controls carry a deterministic id of root_<camelField>. The
  // schema form nests form-items (a parent item contains a child label), so
  // a :has(label:text-is(...)) query can resolve to the PARENT item whose
  // first input is a different field (observed: player_001 landed in Name).
  // Address by id to be unambiguous.
  const camel = label.replace(/\s+/g, '');
  const field = camel.charAt(0).toLowerCase() + camel.slice(1);
  const input = page.locator(`#root_${field}`).first();
  await input.waitFor({ state: 'visible', timeout: 15000 });
  await input.click();
  await input.fill(value);
  await input.press('Tab');
}

async function sendAndWait(page, waitText) {
  await page.getByRole('button', { name: /发\s*送|Send/ }).first().click();
  // The response card shows a placeholder until a request resolves; wait for
  // that placeholder to go away (a real result) plus the requested state.
  await page.locator('text=发送请求后，响应结果将显示在这里').first().waitFor({ state: 'detached', timeout: 30000 });
  await page.locator(`text=${waitText}`).first().waitFor({ state: 'visible', timeout: 30000 });
  await page.waitForTimeout(800);
}

// Approval-gated functions do NOT render a result card: the invoke returns
// approvalRequired and the page switches to the 审批中 alert banner instead
// (src/pages/Functions/Invoke/index.tsx renders pendingApproval as an Alert).
async function sendForApproval(page, waitText) {
  await page.getByRole('button', { name: /发\s*送|Send/ }).first().click();
  await page.locator(`text=${waitText}`).first().waitFor({ state: 'visible', timeout: 30000 });
  await page.waitForTimeout(800);
}

async function main() {
  fs.mkdirSync(OUT, { recursive: true });
  const browser = await chromium.launch(resolveChromium());
  const ctx = await browser.newContext({
    baseURL: BASE,
    locale: 'zh-CN',
    viewport: { width: 1600, height: 900 },
    deviceScaleFactor: 2,
  });
  const page = await ctx.newPage();
  page.on('pageerror', (e) => console.log('[pageerror]', String(e).slice(0, 160)));
  const failed = [];

  // 01: login page (pre-login, no token yet)
  try {
    await goto(page, '/user/login', '01-login');
  } catch (e) {
    failed.push(['01-login', String(e).slice(0, 160)]);
  }

  // login via the form
  const username = page.locator('input[id="username"], input[name="username"]').first();
  await username.waitFor({ state: 'visible', timeout: 60000 });
  await username.fill('admin');
  await page.locator('input[name="password"], input[type="password"]').first().fill('admin123');
  await page
    .locator('button[type="submit"]')
    .or(page.locator('button', { hasText: /登\s*录/ }))
    .or(page.locator('button', { hasText: /Login/ }))
    .first()
    .click();
  await page.waitForURL((u) => !u.pathname.startsWith('/user/login'), { timeout: 30000 });
  await page.waitForFunction(
    () =>
      Boolean(window.localStorage.getItem('token')) &&
      Boolean(window.localStorage.getItem('game_id')) &&
      Boolean(window.localStorage.getItem('env')),
    undefined,
    { timeout: 30000, polling: 500 },
  );
  // Pin the guide's documented scope (default / dev): the persisted last-used
  // scope varies with whoever used the stack last, and pages defer loading
  // until the GameSelector validates the scope (stores/scope.ts).
  await page.evaluate(() => {
    window.localStorage.setItem('game_id', 'default');
    window.localStorage.setItem('env', 'dev');
  });
  console.log('[capture] logged in (zh-CN, desktop, scope=default/dev)');

  // 02/03: static shell views
  try {
    await goto(page, '/console/home', '02-console-home');
  } catch (e) {
    failed.push(['02-console-home', String(e).slice(0, 160)]);
  }
  try {
    await page.goto('/functions/catalog', { waitUntil: 'domcontentloaded', timeout: 30000 });
    await settle(page, '/functions/catalog');
    // The summary counts (可装配函数 / 总数) load asynchronously after the
    // descriptor fetch; wait for the table's result count to reach the demo's
    // 19 (settle() only checks DOM node count and can shoot before data lands).
    await page.locator('text=当前结果 19 个函数').first().waitFor({ state: 'visible', timeout: 30000 });
    await page.waitForTimeout(800);
    await shot(page, '03-functions');
  } catch (e) {
    failed.push(['03-functions', String(e).slice(0, 160)]);
  }

  // 04a/04c: item grant schema form + success response (demo demo data:
  // player_001 grants sword_01 x2)
  try {
    await page.goto('/functions/invoke', { waitUntil: 'domcontentloaded', timeout: 30000 });
    await settle(page, '/functions/invoke');
    await pickFunction(page, 'inventory.grant');
    await ensureSchemaTab(page);
    await fillField(page, 'Player Id', 'player_001');
    await fillField(page, 'Quantity', '2');
    await fillField(page, 'Template Id', 'sword_01');
    await page.waitForTimeout(500);
    await shot(page, '04a-invoke-form');
    await sendAndWait(page, '响应');
    // The response card sits below the fold; scroll the last form field to
    // the top so the 成功 badge and the structured result are in frame
    // (matches the guide's framing: form bottom + response card).
    await page.locator('#root_templateId').last().scrollIntoViewIfNeeded();
    await page.waitForTimeout(400);
    await shot(page, '04c-invoke-result');
  } catch (e) {
    failed.push(['04a/04c invoke', String(e).slice(0, 160)]);
  }

  // 05: approval-gated flow (player.delete needs approval before executing;
  // the page switches to the 审批中 alert banner with a ticket number)
  try {
    await pickFunction(page, 'player.delete');
    await ensureSchemaTab(page);
    await fillField(page, 'Id', 'player_002');
    await page.waitForTimeout(500);
    await sendForApproval(page, '审批中：该操作需要审批通过后才会执行');
    await shot(page, '05-invoke-approval-required');
  } catch (e) {
    failed.push(['05-invoke-approval-required', String(e).slice(0, 160)]);
  }

  // 06/07: approval center + detail drawer (the submission above guarantees
  // at least one row)
  try {
    await page.goto('/approvals', { waitUntil: 'domcontentloaded', timeout: 30000 });
    await settle(page, '/approvals');
    await shot(page, '06-approvals');
    const detail = page
      .locator('.ant-table-row button, .ant-table-row a')
      .filter({ hasText: /查看|详情|Detail|View/ })
      .first();
    await detail.click();
    await page.locator('.ant-drawer-open').first().waitFor({ state: 'visible', timeout: 15000 });
    await page.waitForTimeout(800);
    await shot(page, '07-approval-detail');
    await page.keyboard.press('Escape');
  } catch (e) {
    failed.push(['06/07 approvals', String(e).slice(0, 160)]);
  }

  // 08/10: ops and execution logs
  try {
    await goto(page, '/ops/nodes', '08-ops-nodes');
  } catch (e) {
    failed.push(['08-ops-nodes', String(e).slice(0, 160)]);
  }
  try {
    await goto(page, '/functions/execution-logs', '10-execution-logs');
  } catch (e) {
    failed.push(['10-execution-logs', String(e).slice(0, 160)]);
  }

  await ctx.close();
  await browser.close();

  if (failed.length > 0) {
    console.error('[capture] FAILED views:');
    for (const [name, err] of failed) console.error(`  ${name}: ${err}`);
    process.exit(1);
  }
  console.log(`[capture] done -> ${OUT}`);
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});
