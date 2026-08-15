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

    await expect(page.getByRole('heading', { name: 'OpenAPI Sources' })).toBeVisible();
    await expect(page.getByText('Source 不是 UI，也不是自动注册')).toBeVisible();
    await expectTableVisible(page);
  });

  test('上传 OpenAPI 文档', async ({ page }) => {
    await page.goto('/system/functions/openapi-sources');
    await waitForPageReady(page);

    const uploadBtn = page.getByRole('button', { name: '上传 Source' });
    await expect(uploadBtn).toBeVisible();
    await uploadBtn.click();

    await expectModalVisible(page);
    await expect(page.getByRole('dialog').getByText('不要在 OpenAPI 中写 UI')).toBeVisible();

    const cancelBtn = page.getByRole('dialog').getByRole('button', { name: '取消' });
    await expect(cancelBtn).toBeVisible();
    await cancelBtn.click();
    await expect(page.getByRole('dialog')).toBeHidden();
  });

  test('Provider 绑定', async ({ page }) => {
    await page.goto('/system/functions/openapi-sources');
    await waitForPageReady(page);

    const openBtn = page.getByRole('button', { name: '打开' }).first();
    await expect(openBtn).toBeVisible();
    await openBtn.click();
    await expectDrawerVisible(page);
    await expect(page.getByText('Operations', { exact: true })).toBeVisible();

    const bindBtn = page.getByRole('button', { name: '绑定', exact: true }).first();
    await expect(bindBtn).toBeVisible();
    await bindBtn.click();

    await expectModalVisible(page);
    await expect(page.getByRole('dialog').getByText('当前只启用 Provider binding')).toBeVisible();

    const cancelBtn = page.getByRole('dialog').getByRole('button', { name: '取消' });
    await expect(cancelBtn).toBeVisible();
    await cancelBtn.click();
    await expect(page.getByRole('dialog')).toBeHidden();
  });
});
