import type { FullConfig, FullResult } from '@playwright/test';
import { chromium } from '@playwright/test';
import fs from 'node:fs';
import path from 'node:path';
import { shouldStartRealFixture, startRealFixture } from './realFixture';

const stateDir = path.resolve(__dirname, '..', '.auth');
const mockAuthState = path.join(stateDir, 'mock.json');

function shouldPrepareMockAuth(): boolean {
  // Any mock-dashboard test selected (no --project, or mock-dashboard chosen)
  for (let i = 0; i < process.argv.length; i += 1) {
    const arg = process.argv[i];
    if (arg.startsWith('--project=') && !arg.includes('mock-dashboard')) return false;
    if ((arg === '--project' || arg === '-p') && process.argv[i + 1] !== 'mock-dashboard') {
      return false;
    }
  }
  return true;
}

/**
 * Login once and persist localStorage token into a storageState file so every
 * mock-dashboard test starts authenticated. This removes the per-test login
 * round-trip (~6-10s each) which dominated suite wall time.
 */
async function prepareMockAuthState(config: FullConfig): Promise<void> {
  const mockProject = config.projects.find((p) => p.name === 'mock-dashboard');
  const baseURL = mockProject?.use?.baseURL as string | undefined;
  if (!baseURL) {
    console.log('[mock-auth] no mock-dashboard baseURL, skipping pre-login');
    return;
  }

  fs.mkdirSync(stateDir, { recursive: true });
  const browser = await chromium.launch();
  const context = await browser.newContext({ baseURL });
  const page = await context.newPage();

  try {
    await page.goto('/user/login');
    // Login form is rendered client-side; wait for the input instead of a
    // fixed sleep.
    const username = page
      .locator('input[id="username"], input[placeholder*="admin"], input[placeholder*="用户名"]')
      .first();
    await username.waitFor({ state: 'visible', timeout: 60000 });
    await username.fill('admin');
    await page.locator('input[type="password"]').fill('admin123');
    await page
      .locator('button[type="submit"], button:has-text("Login"), button:has-text("登录")')
      .first()
      .click();
    // Login completes when the app navigates away and stores the token.
    await page.waitForURL(
      (url) => /\/(console|dashboard)/.test(url.pathname) || url.pathname === '/',
      { timeout: 30000 },
    );
    // token + scope（game_id/env）都落盘后才保存：否则每个用例都要重新
    // 等 GameSelector 解析 games，Console 页面会卡 loading。
    await page.waitForFunction(
      () =>
        Boolean(window.localStorage.getItem('token')) &&
        Boolean(window.localStorage.getItem('game_id')) &&
        Boolean(window.localStorage.getItem('env')),
      undefined,
      { timeout: 30000, polling: 500 },
    );
    await context.storageState({ path: mockAuthState });
    console.log('[mock-auth] pre-login state saved (token + scope)');

    // 预热 Console 页面：触发 dev server 完成相关 chunk 编译，
    // 避免首个访问页面的用例在导航阶段超时。
    for (const warmPath of [
      '/console/players/resource--players',
      '/console/system/operation--system.dangerous-op',
      '/console/reward/task--reward.batchGrant',
      '/console/analytics/report--analytics.retention',
      '/console/mail/operation--mail.send',
    ]) {
      const warmed = await page
        .goto(warmPath, { waitUntil: 'domcontentloaded', timeout: 90000 })
        .then(() => true)
        .catch((e: unknown) => {
          console.warn(`[mock-auth] warm goto failed: ${warmPath}`, e);
          return false;
        });
      if (!warmed) continue;
      // 等“正在加载资源”占位消失且真实表格/表单出现（chunk 编译完成）
      await page
        .locator('.ant-pro-table, .ant-table, .ant-form, form')
        .first()
        .waitFor({ state: 'visible', timeout: 120000 })
        .catch(() => console.warn(`[mock-auth] warm content timeout: ${warmPath}`));
    }
    console.log('[mock-auth] console pages warmed');
  } finally {
    await browser.close();
  }
}

export default async function globalSetup(config: FullConfig): Promise<void> {
  if (shouldStartRealFixture()) {
    await startRealFixture();
  } else {
    console.log('[real-dashboard fixture] skipped (no real-dashboard project selected)');
  }

  if (shouldPrepareMockAuth()) {
    // Retry once: the freshly-started dev server may still be bundling.
    for (let attempt = 1; attempt <= 2; attempt += 1) {
      try {
        await prepareMockAuthState(config);
        return;
      } catch (error) {
        if (attempt === 2) {
          console.warn('[mock-auth] pre-login failed, tests will fall back to UI login', error);
          return;
        }
        await new Promise((r) => setTimeout(r, 5000));
      }
    }
  }
}

export const onTeardown: FullResult['onEnd'] = () => {
  // keep the state file for reuse across runs; it lives under e2e/.auth
};

export { mockAuthState };
