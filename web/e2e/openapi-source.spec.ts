/**
 * 场景 10: OpenAPI Source - 导入和绑定
 */

import { test, expect } from '@playwright/test';
import { login, waitForPageReady } from './helpers';

const BASE_URL = process.env.BASE_URL || 'http://localhost:8000';

test.describe('OpenAPI Source', () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
  });

  test('OpenAPI Source 页面加载', async ({ page }) => {
    await page.goto(`${BASE_URL}/system/functions/openapi-sources`);
    await waitForPageReady(page);

    // 验证页面加载
    await expect(page.locator('.ant-pro-table, .ant-table, .ant-card').first()).toBeVisible();
  });

  test('上传 OpenAPI 文档', async ({ page }) => {
    await page.goto(`${BASE_URL}/system/functions/openapi-sources`);
    await waitForPageReady(page);

    // 检查上传按钮
    const uploadBtn = page
      .locator('button:has-text("上传"), button:has-text("导入"), button:has-text("Upload")')
      .first();
    if (await uploadBtn.isVisible()) {
      await uploadBtn.click();

      // 等待上传对话框
      await page
        .locator('.ant-modal, .ant-drawer')
        .first()
        .waitFor({ state: 'visible', timeout: 10000 });

      // 取消上传
      await page.locator('button:has-text("取"), button:has-text("Cancel")').first().click();
    }
  });

  test('Provider 绑定', async ({ page }) => {
    await page.goto(`${BASE_URL}/system/functions/openapi-sources`);
    await waitForPageReady(page);

    // 检查绑定按钮
    const bindBtn = page
      .locator('button:has-text("绑定"), a:has-text("绑定"), button:has-text("Bind")')
      .first();
    if (await bindBtn.isVisible()) {
      await bindBtn.click();

      // 等待绑定对话框
      await page
        .locator('.ant-modal, .ant-drawer')
        .first()
        .waitFor({ state: 'visible', timeout: 10000 });

      // 取消绑定
      await page.locator('button:has-text("取"), button:has-text("Cancel")').first().click();
    }
  });
});
