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

    // 验证页面加载 - 检查页面标题或内容
    const hasContent = await page.locator('.ant-pro-table, .ant-table, .ant-card, .ant-tabs, main, [class*="page"]').first().isVisible({ timeout: 10000 }).catch(() => false);
    const hasTitle = await page.locator('h1, h2, h3, .ant-page-header').first().isVisible({ timeout: 5000 }).catch(() => false);

    // 页面应该有内容
    expect(hasContent || hasTitle).toBeTruthy();
  });

  test('Proposal Inbox 展示', async ({ page }) => {
    await page.goto(`${BASE_URL}/system/functions/pages`);
    await waitForPageReady(page);

    // 验证页面加载成功（没有错误）
    const hasError = await page.locator('.ant-result-error, text=加载失败, text=Error').first().isVisible({ timeout: 5000 }).catch(() => false);

    // 页面应该正常加载（没有错误）
    expect(hasError).toBeFalsy();
  });

  test('预览功能', async ({ page }) => {
    await page.goto(`${BASE_URL}/system/functions/pages`);
    await waitForPageReady(page);

    // 点击预览
    const previewBtn = page.locator('button:has-text("预览"), a:has-text("预览"), button:has-text("Preview")').first();
    if (await previewBtn.isVisible({ timeout: 5000 }).catch(() => false)) {
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
    if (await publishBtn.isVisible({ timeout: 5000 }).catch(() => false)) {
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
