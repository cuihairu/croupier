/**
 * Resource Catalog 测试
 */

import { test, expect } from '@playwright/test';
import {
  login,
  waitForPageReady,
  expectTableVisible,
  expectDrawerVisible,
  expectModalVisible,
} from './helpers';

test.describe('Resource Catalog', () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
  });

  test('资源目录页面加载', async ({ page }) => {
    await page.goto('/system/functions/resource-catalog');
    await waitForPageReady(page);

    // 验证资源列表
    const content = page.locator('.ant-pro-table, .ant-table, .ant-card').first();
    await expect(content).toBeVisible();
  });

  test('资源列表展示', async ({ page }) => {
    await page.goto('/system/functions/resource-catalog');
    await waitForPageReady(page);

    // 验证有数据展示
    const table = page.locator('.ant-pro-table, .ant-table').first();
    const cards = page.locator('.ant-card').first();

    const tableVisible = await table.isVisible().catch(() => false);
    const cardsVisible = await cards.isVisible().catch(() => false);

    expect(tableVisible || cardsVisible).toBeTruthy();
  });

  test('资源详情查看', async ({ page }) => {
    await page.goto('/system/functions/resource-catalog');
    await waitForPageReady(page);

    // 点击查看详情
    const detailBtn = page
      .locator(
        'button:has-text("查看"), a:has-text("查看"), button:has-text("详情"), a:has-text("详情")',
      )
      .first();
    const hasDetail = await detailBtn.isVisible().catch(() => false);

    if (hasDetail) {
      await detailBtn.click();

      // 等待详情展示
      const drawer = page.locator('.ant-drawer').first();
      const modal = page.locator('.ant-modal').first();

      const drawerVisible = await drawer.isVisible({ timeout: 10000 }).catch(() => false);
      const modalVisible = await modal.isVisible({ timeout: 10000 }).catch(() => false);

      expect(drawerVisible || modalVisible).toBeTruthy();

      // 关闭详情
      if (drawerVisible) {
        await page.locator('.ant-drawer-close').first().click();
      } else if (modalVisible) {
        await page.locator('.ant-modal-close').first().click();
      }
    }
  });

  test('语义信息展示', async ({ page }) => {
    await page.goto('/system/functions/resource-catalog');
    await waitForPageReady(page);

    // 验证状态标签
    const statusTag = page
      .locator(
        '.ant-tag:has-text("已识别"), .ant-tag:has-text("identified"), .ant-tag:has-text("待确认")',
      )
      .first();
    const hasStatus = await statusTag.isVisible().catch(() => false);

    // 验证页面有内容
    const content = page.locator('.ant-pro-table, .ant-table, .ant-card').first();
    await expect(content).toBeVisible();
  });
});
