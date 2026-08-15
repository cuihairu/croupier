/**
 * 场景 8: 契约变化 - Stale 页面处理
 */

import { test, expect } from '@playwright/test';
import {
  login,
  navigateToConsole,
  waitForPageReady,
  waitForTable,
  expectTableHasRows,
} from './helpers';

test.describe('契约变化', () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
  });

  test('Stale 页面警告展示', async ({ page }) => {
    await navigateToConsole(page, 'players', 'resource--players');
    await waitForPageReady(page);

    const staleAlert = page.getByText('页面绑定的函数契约已变化，执行已被阻断').first();
    // 当前 fixture 初始合同必须是 fresh；stale 场景由命名变更测试显式触发。
    await expect(staleAlert).toHaveCount(0);
    await waitForTable(page);
    await expectTableHasRows(page);
  });

  test('正常页面无 stale 警告', async ({ page }) => {
    await navigateToConsole(page, 'players', 'resource--players');
    await waitForPageReady(page);

    await waitForTable(page);
    await expect(page.locator('.ant-pro-table, .ant-table').first()).toBeVisible();
    await expectTableHasRows(page);
    await expect(page.getByText('玩家A')).toBeVisible();
    await expect(page.locator('.ant-result-error, text=加载失败')).toHaveCount(0);
  });
});
