/**
 * 场景 10: OpenAPI Source - 导入和绑定
 */

import { test, expect } from '@playwright/test';
import {
  login,
  waitForPageReady,
  expectTableVisible,
  expectModalVisible,
  expectDrawerVisible,
} from './helpers';

test.describe('OpenAPI Source', () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
  });

  test('OpenAPI Source 页面加载', async ({ page }) => {
    await page.goto('/system/functions/openapi-sources');
    await waitForPageReady(page);

    // 验证页面加载
    const content = page.locator('.ant-pro-table, .ant-table, .ant-card').first();
    await expect(content).toBeVisible();
  });

  test('上传 OpenAPI 文档', async ({ page }) => {
    await page.goto('/system/functions/openapi-sources');
    await waitForPageReady(page);

    // 检查上传按钮
    const uploadBtn = page
      .locator('button:has-text("上传"), button:has-text("导入"), button:has-text("Upload")')
      .first();
    const hasUpload = await uploadBtn.isVisible().catch(() => false);

    if (hasUpload) {
      await uploadBtn.click();

      // 等待上传对话框
      const modal = page.locator('.ant-modal').first();
      const drawer = page.locator('.ant-drawer').first();

      const modalVisible = await modal.isVisible({ timeout: 10000 }).catch(() => false);
      const drawerVisible = await drawer.isVisible({ timeout: 10000 }).catch(() => false);

      expect(modalVisible || drawerVisible).toBeTruthy();

      // 取消上传
      const cancelBtn = page.locator('button:has-text("取"), button:has-text("Cancel")').first();
      await expect(cancelBtn).toBeVisible();
      await cancelBtn.click();
    }
  });

  test('Provider 绑定', async ({ page }) => {
    await page.goto('/system/functions/openapi-sources');
    await waitForPageReady(page);

    // 检查绑定按钮
    const bindBtn = page
      .locator('button:has-text("绑定"), a:has-text("绑定"), button:has-text("Bind")')
      .first();
    const hasBind = await bindBtn.isVisible().catch(() => false);

    if (hasBind) {
      await bindBtn.click();

      // 等待绑定对话框
      const modal = page.locator('.ant-modal').first();
      const drawer = page.locator('.ant-drawer').first();

      const modalVisible = await modal.isVisible({ timeout: 10000 }).catch(() => false);
      const drawerVisible = await drawer.isVisible({ timeout: 10000 }).catch(() => false);

      expect(modalVisible || drawerVisible).toBeTruthy();

      // 取消绑定
      const cancelBtn = page.locator('button:has-text("取"), button:has-text("Cancel")').first();
      await expect(cancelBtn).toBeVisible();
      await cancelBtn.click();
    }
  });
});
