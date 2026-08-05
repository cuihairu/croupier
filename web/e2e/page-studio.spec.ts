/**
 * Page Studio 测试
 */

import { test, expect } from '@playwright/test';
import { login, waitForPageReady } from './helpers';

const BASE_URL = process.env.BASE_URL || 'http://localhost:8000';

test.describe('Page Studio', () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
  });

  test('页面列表加载', async ({ page }) => {
    await page.goto(`${BASE_URL}/system/functions/pages`);
    await waitForPageReady(page);

    // 验证页面列表
    await expect(page.locator('.ant-pro-table, .ant-table, .ant-card').first()).toBeVisible();
  });

  test('Proposal Inbox 展示', async ({ page }) => {
    await page.goto(`${BASE_URL}/system/functions/pages`);
    await waitForPageReady(page);

    // 验证有 Proposal 列表或页面列表
    const hasTable = await page.locator('.ant-pro-table, .ant-table').first().isVisible().catch(() => false);
    const hasCards = await page.locator('.ant-card').first().isVisible().catch(() => false);

    expect(hasTable || hasCards).toBeTruthy();
  });

  test('预览功能', async ({ page }) => {
    await page.goto(`${BASE_URL}/system/functions/pages`);
    await waitForPageReady(page);

    // 点击预览
    const previewBtn = page.locator('button:has-text("预览"), a:has-text("预览"), button:has-text("Preview")').first();
    if (await previewBtn.isVisible()) {
      await previewBtn.click();

      // 等待 Drawer 出现
      await page.locator('.ant-drawer').first().waitFor({ state: 'visible', timeout: 10000 });

      // 关闭预览
      await page.locator('.ant-drawer-close').first().click();
    }
  });

  test('发布功能', async ({ page }) => {
    await page.goto(`${BASE_URL}/system/functions/pages`);
    await waitForPageReady(page);

    // 点击发布
    const publishBtn = page.locator('button:has-text("发布"), a:has-text("发布"), button:has-text("Publish")').first();
    if (await publishBtn.isVisible()) {
      await publishBtn.click();

      // 确认发布
      const confirmBtn = page.locator('.ant-popconfirm .ant-btn-primary, .ant-modal-confirm .ant-btn-primary').first();
      if (await confirmBtn.isVisible({ timeout: 5000 }).catch(() => false)) {
        await confirmBtn.click();
        await page.waitForTimeout(2000);
      }
    }
  });
});
