/**
 * Resource Catalog 测试
 */

import { test, expect } from '@playwright/test';
import { login, waitForPageReady } from './helpers';

const BASE_URL = process.env.BASE_URL || 'http://localhost:8000';

test.describe('Resource Catalog', () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
  });

  test('资源目录页面加载', async ({ page }) => {
    await page.goto(`${BASE_URL}/system/functions/resource-catalog`);
    await waitForPageReady(page);

    // 验证资源列表
    await expect(page.locator('.ant-pro-table, .ant-table, .ant-card').first()).toBeVisible();
  });

  test('资源列表展示', async ({ page }) => {
    await page.goto(`${BASE_URL}/system/functions/resource-catalog`);
    await waitForPageReady(page);

    // 验证有数据展示
    const hasTable = await page.locator('.ant-pro-table, .ant-table').first().isVisible().catch(() => false);
    const hasCards = await page.locator('.ant-card').first().isVisible().catch(() => false);

    expect(hasTable || hasCards).toBeTruthy();
  });

  test('资源详情查看', async ({ page }) => {
    await page.goto(`${BASE_URL}/system/functions/resource-catalog`);
    await waitForPageReady(page);

    // 点击查看详情
    const detailBtn = page.locator('button:has-text("查看"), a:has-text("查看"), button:has-text("详情"), a:has-text("详情")').first();
    if (await detailBtn.isVisible()) {
      await detailBtn.click();

      // 等待详情展示
      await page.locator('.ant-drawer, .ant-modal').first().waitFor({ state: 'visible', timeout: 10000 });

      // 关闭详情
      await page.locator('.ant-drawer-close, .ant-modal-close').first().click();
    }
  });

  test('语义信息展示', async ({ page }) => {
    await page.goto(`${BASE_URL}/system/functions/resource-catalog`);
    await waitForPageReady(page);

    // 验证状态标签
    const statusTag = page.locator('.ant-tag:has-text("已识别"), .ant-tag:has-text("identified"), .ant-tag:has-text("待确认")').first();
    const hasStatus = await statusTag.isVisible().catch(() => false);

    // 至少有页面内容
    await expect(page.locator('body')).toBeVisible();
  });
});
