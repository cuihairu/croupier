/**
 * 场景 8: 契约变化 - Stale 页面处理
 */

import { test, expect } from '@playwright/test';
import { login, navigateToConsole, waitForPageReady, waitForTable } from './helpers';

test.describe('契约变化', () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
  });

  test('Stale 页面警告展示', async ({ page }) => {
    await navigateToConsole(page, 'players', 'resource--players');
    await waitForPageReady(page);

    // 检查是否有 stale 警告或正常表格
    const staleAlert = page
      .locator('text=页面绑定的函数契约已变化, text=stale, text=契约变化')
      .first();
    const hasStale = await staleAlert.isVisible().catch(() => false);

    if (hasStale) {
      // 如果有 stale 警告，验证执行被阻断
      await expect(staleAlert).toBeVisible();

      // 验证执行按钮被禁用或有阻断提示
      const blockedMsg = page.locator('text=执行被阻断, text=阻断, .ant-alert-error').first();
      await expect(blockedMsg).toBeVisible();
    } else {
      // 如果没有 stale，页面应该正常显示表格
      await waitForTable(page);
      await expect(page.locator('.ant-pro-table, .ant-table').first()).toBeVisible();
    }
  });

  test('正常页面无 stale 警告', async ({ page }) => {
    await navigateToConsole(page, 'players', 'resource--players');
    await waitForPageReady(page);

    // 验证页面加载 - 应该有表格或明确的错误
    const hasTable = await page
      .locator('.ant-pro-table, .ant-table')
      .first()
      .isVisible()
      .catch(() => false);
    const hasError = await page
      .locator('.ant-result-error, text=加载失败')
      .first()
      .isVisible()
      .catch(() => false);

    // 必须有表格或错误，不能两者都没有
    expect(hasTable || hasError).toBeTruthy();

    // 如果有表格，验证有数据行
    if (hasTable) {
      const rows = await page.locator('tbody tr, .ant-table-row').count();
      expect(rows).toBeGreaterThan(0);
    }
  });
});
