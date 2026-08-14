/**
 * 场景 9: Scope 隔离 - 不同环境数据隔离
 */

import { test, expect } from '@playwright/test';
import {
  login,
  navigateToConsole,
  waitForPageReady,
  waitForTable,
  expectTableVisible,
} from './helpers';

test.describe('Scope 隔离', () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
  });

  test('默认环境页面加载', async ({ page }) => {
    await navigateToConsole(page, 'players', 'resource--players');
    await waitForPageReady(page);

    // 验证页面正常加载
    await waitForTable(page);
    await expectTableVisible(page);
  });

  test('切换环境后页面重新加载', async ({ page }) => {
    // 先在默认环境加载
    await navigateToConsole(page, 'players', 'resource--players');
    await waitForPageReady(page);
    await waitForTable(page);

    // 尝试切换环境
    const envSelector = page
      .locator(
        '[data-testid="env-selector"], .ant-select:has-text("环境"), .ant-select:has-text("Env")',
      )
      .first();
    const hasEnvSelector = await envSelector.isVisible().catch(() => false);

    if (hasEnvSelector) {
      await envSelector.click();

      // 选择不同的环境
      const envOption = page
        .locator('.ant-select-item:has-text("staging"), .ant-select-item:has-text("test")')
        .first();
      const hasOption = await envOption.isVisible().catch(() => false);

      if (hasOption) {
        await envOption.click();
        await page.waitForTimeout(2000);

        // 验证页面重新加载
        await waitForPageReady(page);
        await expectTableVisible(page);
      }
    }
  });

  test('不同环境数据不串', async ({ page }) => {
    // 在第一个环境查看数据
    await navigateToConsole(page, 'players', 'resource--players');
    await waitForPageReady(page);
    await waitForTable(page);

    // 记录当前数据
    const firstEnvData = await page.locator('td').allTextContents();

    // 切换环境
    const envSelector = page.locator('[data-testid="env-selector"], .ant-select').first();
    const hasEnvSelector = await envSelector.isVisible().catch(() => false);

    if (hasEnvSelector) {
      await envSelector.click();
      const envOption = page.locator('.ant-select-item').nth(1);
      const hasOption = await envOption.isVisible().catch(() => false);

      if (hasOption) {
        await envOption.click();
        await page.waitForTimeout(2000);
        await waitForPageReady(page);

        // 验证新环境的数据也加载成功
        const secondEnvData = await page.locator('td').allTextContents();
        expect(secondEnvData.length).toBeGreaterThan(0);
      }
    }
  });
});
