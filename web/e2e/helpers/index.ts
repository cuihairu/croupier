/**
 * E2E 测试辅助函数
 */

import { expect } from '@playwright/test';
import type { Page } from '@playwright/test';

/**
 * 登录并保存认证状态。
 * storageState（globalSetup 预登录）存在时直接复用，跳过 UI 登录；
 * 否则走完整 UI 登录流程（真实环境/回退路径）。
 */
export async function login(page: Page): Promise<void> {
  // storageState 预登录时 token 已在 localStorage：先到应用页再检测，
  // 避免 about:blank 上访问 localStorage 抛 SecurityError。
  await page.goto('/');
  await page.waitForLoadState('domcontentloaded');
  const hasToken = await page.evaluate(() => Boolean(window.localStorage.getItem('token')));
  if (hasToken && !page.url().includes('/user/login')) {
    // 已认证。等待 scope 解析完成（env selector 出现 = games 校验通过），
    // 否则 Console 页面会在 scope 未定时卡 loading。
    await page
      .getByTestId('env-selector')
      .waitFor({ state: 'visible', timeout: 60000 })
      .catch(() => {});
    return;
  }

  await page.goto('/user/login');
  await page.waitForLoadState('domcontentloaded');

  // umi dev overlay iframe 偶发拦截点击：移除所有 overlay iframe
  await page.evaluate(() => {
    document.querySelectorAll('iframe').forEach((el) => {
      const style = window.getComputedStyle(el);
      if (style.position === 'fixed' || el.getAttribute('id')?.includes('overlay')) {
        el.remove();
      }
    });
  });

  // 等待登录表单（客户端渲染，替代固定 sleep）
  const usernameInput = page
    .locator('input[id="username"], input[placeholder*="admin"], input[placeholder*="用户名"]')
    .first();
  await usernameInput.waitFor({ state: 'visible', timeout: 60000 });

  await usernameInput.fill('admin');
  await page.locator('input[type="password"]').fill('admin123');

  // 点击登录按钮
  const loginBtn = page
    .locator('button[type="submit"], button:has-text("Login"), button:has-text("登录")')
    .first();
  await loginBtn.click();

  // 等待登录成功
  await page.waitForURL(
    (url) => /\/(console|dashboard)/.test(url.pathname) || url.pathname === '/',
    { timeout: 30000 },
  );
  await page.waitForLoadState('domcontentloaded');
}

/**
 * 导航到控制台页面
 */
export async function navigateToConsole(
  page: Page,
  categoryKey: string,
  pageKey: string,
): Promise<void> {
  await page.goto(`/console/${categoryKey}/${pageKey}`);
  await page.waitForLoadState('domcontentloaded');
}

/**
 * 导航到系统功能页面
 */
export async function navigateToSystem(page: Page, path: string): Promise<void> {
  await page.goto(`/system/functions/${path}`);
  await page.waitForLoadState('domcontentloaded');
}

/**
 * 等待页面加载完成
 */
export async function waitForPageReady(page: Page): Promise<void> {
  // 等待 ProLayout 主体渲染（侧栏或主内容），替代 networkidle + 固定 sleep
  await page
    .locator('aside, main, .ant-pro-layout-content, .ant-layout')
    .first()
    .waitFor({ state: 'visible', timeout: 30000 });
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
  await page.locator('.ant-drawer').waitFor({ state: 'hidden', timeout: 5000 });
}

/**
 * 确认 Popconfirm
 */
export async function confirmPopconfirm(page: Page): Promise<void> {
  await page.locator('.ant-popconfirm .ant-btn-primary').click();
}

/**
 * 等待表格加载完成
 */
export async function waitForTable(
  page: Page,
  timeout = process.env.CI ? 45000 : 15000,
): Promise<void> {
  // .ant-pro-table 根容器可能存在被折叠/隐藏的实例（first() 命中即挂）。
  // 等待真实表格体。
  await page.locator('.ant-table-tbody').first().waitFor({ state: 'visible', timeout });
}

/**
 * 选择环境
 */
export async function selectEnv(page: Page, env: string): Promise<void> {
  const envSelector = page.getByTestId('env-selector');
  // scope 切换会触发整页 reload（Console 页订阅 scope 变化）；
  // reload 后 env-selector 需重新出现（games 重新校验）。
  await expect(envSelector).toBeVisible();
  await envSelector.click();

  const envOption = page.locator('.ant-select-item-option').filter({ hasText: env }).first();
  await expect(envOption).toBeVisible();
  await envOption.click();
  // 选择后页面可能整页 reload；轮询等待 selector 重新出现且值为目标环境。
  await expect
    .poll(
      async () => {
        const el = page.getByTestId('env-selector');
        if (!(await el.isVisible().catch(() => false))) return '';
        return (await el.innerText().catch(() => '')) || '';
      },
      { timeout: 30000 },
    )
    .toMatch(new RegExp(env, 'i'));
}

/**
 * 断言表格有数据行
 */
export async function expectTableHasRows(page: Page, minRows = 1): Promise<void> {
  const rows = await page.locator('tbody tr, .ant-table-row').count();
  expect(rows).toBeGreaterThanOrEqual(minRows);
}

/**
 * 断言页面有可见的表格
 */
export async function expectTableVisible(page: Page): Promise<void> {
  await expect(page.locator('.ant-table-tbody').first()).toBeVisible();
}

/**
 * 断言页面有表单
 */
export async function expectFormVisible(page: Page): Promise<void> {
  await expect(page.locator('.ant-form, form').first()).toBeVisible();
}

/**
 * 断言 Toast/Message 消息出现
 */
export async function expectMessageVisible(
  page: Page,
  text: string,
  timeout = 10000,
): Promise<void> {
  await expect(page.locator(`.ant-message:has-text("${text}")`).first()).toBeVisible({ timeout });
}

/**
 * 断言 Modal 出现
 */
export async function expectModalVisible(page: Page, timeout = 10000): Promise<void> {
  await expect(page.locator('.ant-modal').first()).toBeVisible({ timeout });
}

/**
 * 断言 Drawer 出现
 */
export async function expectDrawerVisible(page: Page, timeout = 10000): Promise<void> {
  await expect(page.locator('.ant-drawer').first()).toBeVisible({ timeout });
}

/**
 * 断言没有错误页面
 */
export async function expectNoErrorPage(page: Page): Promise<void> {
  await expect(page.locator('.ant-result-error, text=加载失败')).toHaveCount(0);
}
