/**
 * E2E 测试辅助函数
 */

import type { Page, BrowserContext } from '@playwright/test';

const BASE_URL = process.env.BASE_URL || 'http://localhost:8000';

/**
 * 登录并保存认证状态
 */
export async function login(page: Page): Promise<void> {
  await page.goto(`${BASE_URL}/user/login`);
  await page.waitForLoadState('domcontentloaded');

  // 等待页面完全加载（可能需要等待 bundling）
  await page.waitForTimeout(5000);

  // 等待登录表单
  const usernameInput = page
    .locator('input[id="username"], input[placeholder*="admin"], input[placeholder*="用户名"]')
    .first();
  await usernameInput.waitFor({ state: 'visible', timeout: 60000 });

  await usernameInput.fill('admin');
  await page.locator('input[type="password"]').fill('ant.design');

  // 点击登录按钮
  const loginBtn = page
    .locator('button[type="submit"], button:has-text("Login"), button:has-text("登录")')
    .first();
  await loginBtn.click();

  // 等待登录成功
  await page.waitForURL(/\/(console|dashboard|$)/, { timeout: 30000 });
  await page.waitForLoadState('networkidle');
}

/**
 * 导航到控制台页面
 */
export async function navigateToConsole(
  page: Page,
  categoryKey: string,
  pageKey: string,
): Promise<void> {
  await page.goto(`${BASE_URL}/console/${categoryKey}/${pageKey}`);
  await page.waitForLoadState('networkidle');
  await page.waitForTimeout(1000);
}

/**
 * 导航到系统功能页面
 */
export async function navigateToSystem(page: Page, path: string): Promise<void> {
  await page.goto(`${BASE_URL}/system/functions/${path}`);
  await page.waitForLoadState('networkidle');
  await page.waitForTimeout(1000);
}

/**
 * 等待页面加载完成
 */
export async function waitForPageReady(page: Page): Promise<void> {
  await page.waitForLoadState('networkidle');
  await page.waitForTimeout(500);
}

/**
 * 等待 API 响应
 */
export async function waitForApi(page: Page, urlPattern: string | RegExp): Promise<void> {
  await page.waitForResponse(urlPattern);
}

/**
 * 截图并保存
 */
export async function takeScreenshot(page: Page, name: string): Promise<void> {
  await page.screenshot({ path: `test-results/${name}.png`, fullPage: true });
}

/**
 * 等待 Toast/Message 消息
 */
export async function waitForMessage(page: Page, text: string, timeout = 10000): Promise<void> {
  await page.locator(`.ant-message:has-text("${text}")`).waitFor({ state: 'visible', timeout });
}

/**
 * 等待 Modal 出现
 */
export async function waitForModal(page: Page, timeout = 10000): Promise<void> {
  await page.locator('.ant-modal').waitFor({ state: 'visible', timeout });
}

/**
 * 等待 Drawer 出现
 */
export async function waitForDrawer(page: Page, timeout = 10000): Promise<void> {
  await page.locator('.ant-drawer').waitFor({ state: 'visible', timeout });
}

/**
 * 关闭 Drawer
 */
export async function closeDrawer(page: Page): Promise<void> {
  await page.locator('.ant-drawer-close').click();
  await page.waitForTimeout(500);
}

/**
 * 确认 Popconfirm
 */
export async function confirmPopconfirm(page: Page): Promise<void> {
  await page.locator('.ant-popconfirm .ant-btn-primary').click();
  await page.waitForTimeout(500);
}

/**
 * 等待表格加载完成
 */
export async function waitForTable(page: Page, timeout = 15000): Promise<void> {
  await page.locator('.ant-pro-table, .ant-table').first().waitFor({ state: 'visible', timeout });
  await page.waitForTimeout(500);
}

/**
 * 选择环境
 */
export async function selectEnv(page: Page, env: string): Promise<void> {
  const envSelector = page.locator('[data-testid="env-selector"], .ant-select').first();
  if (await envSelector.isVisible()) {
    await envSelector.click();
    await page.locator(`.ant-select-item:has-text("${env}")`).click();
    await page.waitForTimeout(1000);
  }
}
