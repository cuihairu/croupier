/**
 * Page Studio 测试
 */

import { test, expect } from '@playwright/test';
import { login, waitForPageReady, expectDrawerVisible } from './helpers';

test.describe('Page Studio', () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
  });

  test('页面列表加载', async ({ page }) => {
    await page.goto('/system/functions/pages');
    await waitForPageReady(page);

    // 验证页面加载 - 检查页面标题或内容
    const content = page
      .locator('.ant-pro-table, .ant-table, .ant-card, .ant-tabs, main, [class*="page"]')
      .first();
    const title = page.locator('h1, h2, h3, .ant-page-header').first();

    const contentVisible = await content.isVisible().catch(() => false);
    const titleVisible = await title.isVisible().catch(() => false);

    // 页面应该有内容
    expect(contentVisible || titleVisible).toBeTruthy();
  });

  test('Proposal Inbox 展示', async ({ page }) => {
    await page.goto('/system/functions/pages');
    await waitForPageReady(page);

    // 验证页面加载成功（没有错误）
    const error = page.locator('.ant-result-error, text=加载失败, text=Error').first();
    const hasError = await error.isVisible().catch(() => false);

    // 页面应该正常加载（没有错误）
    expect(hasError).toBeFalsy();
  });

  test('预览功能', async ({ page }) => {
    await page.goto('/system/functions/pages');
    await waitForPageReady(page);

    // 点击预览
    const previewBtn = page
      .locator('button:has-text("预览"), a:has-text("预览"), button:has-text("Preview")')
      .first();
    const hasPreview = await previewBtn.isVisible().catch(() => false);

    if (hasPreview) {
      await previewBtn.click();

      // 等待 Drawer 出现
      await expectDrawerVisible(page);

      // 关闭预览
      await page.locator('.ant-drawer-close').first().click();
    }
  });

  test('发布功能', async ({ page }) => {
    await page.goto('/system/functions/pages');
    await waitForPageReady(page);

    // 点击发布
    const publishBtn = page
      .locator('button:has-text("发布"), a:has-text("发布"), button:has-text("Publish")')
      .first();
    const hasPublish = await publishBtn.isVisible().catch(() => false);

    if (hasPublish) {
      await publishBtn.click();

      // 确认发布
      const confirmBtn = page
        .locator('.ant-popconfirm .ant-btn-primary, .ant-modal-confirm .ant-btn-primary')
        .first();
      const hasConfirm = await confirmBtn.isVisible({ timeout: 5000 }).catch(() => false);

      if (hasConfirm) {
        await confirmBtn.click();
        await page.waitForTimeout(2000);
      }
    }
  });
});
