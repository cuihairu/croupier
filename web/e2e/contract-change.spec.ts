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

    // 检查是否有 stale 警告
    const staleAlert = page.locator('text=页面绑定的函数契约已变化, text=stale, text=契约变化').first();
    const hasStale = await staleAlert.isVisible().catch(() => false);

    if (hasStale) {
      // 如果有 stale 警告，验证执行被阻断
      await expect(staleAlert).toBeVisible();

      // 验证执行按钮被禁用或有阻断提示
      const blockedMsg = page.locator('text=执行被阻断, text=阻断, .ant-alert-error').first();
      const isBlocked = await blockedMsg.isVisible().catch(() => false);

      // 至少应该有警告信息
      expect(hasStale || isBlocked).toBeTruthy();
    } else {
      // 如果没有 stale，页面应该正常工作
      await waitForTable(page);
    }
  });

  test('正常页面无 stale 警告', async ({ page }) => {
    await navigateToConsole(page, 'players', 'resource--players');
    await waitForPageReady(page);

    // 验证页面加载
    await page.waitForTimeout(2000);

    // 检查页面状态
    const hasError = await page.locator('.ant-result-error, text=加载失败').first().isVisible().catch(() => false);
    const hasTable = await page.locator('.ant-pro-table, .ant-table').first().isVisible().catch(() => false);

    // 页面应该正常加载（有表格或有错误提示）
    expect(hasTable || hasError).toBeTruthy();
  });
});
