/**
 * 场景 7: 报表 - ReportPage 完整功能
 */

import { test, expect } from '@playwright/test';
import {
  login,
  navigateToConsole,
  waitForPageReady,
  expectFormVisible,
  expectTableVisible,
} from './helpers';

test.describe('报表', () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
  });

  test('ReportPage 查询表单加载', async ({ page }) => {
    await navigateToConsole(page, 'analytics', 'report--analytics.retention');
    await waitForPageReady(page);

    await expect(page.getByRole('heading', { name: '留存分析' })).toBeVisible();
    await expectFormVisible(page);
    await expect(page.locator('input[name="startDate"]')).toBeVisible();
    await expect(page.locator('input[name="endDate"]')).toBeVisible();
  });

  test('执行查询', async ({ page }) => {
    await navigateToConsole(page, 'analytics', 'report--analytics.retention');
    await waitForPageReady(page);

    const startDateInput = page.locator('input[name="startDate"], input[id*="startDate"]').first();
    await expect(startDateInput).toBeVisible();
    await startDateInput.fill('2024-01-01');

    const endDateInput = page.locator('input[name="endDate"], input[id*="endDate"]').first();
    await expect(endDateInput).toBeVisible();
    await endDateInput.fill('2024-01-07');

    const queryBtn = page.getByRole('button', { name: '提交' });
    await expect(queryBtn).toBeVisible();
    const executeResponse = page.waitForResponse(
      (response) =>
        response.url().includes('/bindings/query/execute') &&
        response.request().method() === 'POST',
    );
    await queryBtn.click();
    expect((await executeResponse).status()).toBe(200);
    await expect(page.getByText('查询成功')).toBeVisible();
    await expect(page.getByText('数据展示')).toBeVisible();
    await expect(page.getByText('2024-01-01')).toBeVisible();
  });

  test('图表展示', async ({ page }) => {
    await navigateToConsole(page, 'analytics', 'report--analytics.retention');
    await waitForPageReady(page);

    await expect(page.getByText('留存趋势')).toBeVisible();
    await expect(page.getByText('请先查询数据')).toBeVisible();
  });

  test('导出功能', async ({ page }) => {
    await navigateToConsole(page, 'analytics', 'report--analytics.retention');
    await waitForPageReady(page);

    await page.locator('input[name="startDate"]').fill('2024-01-01');
    await page.locator('input[name="endDate"]').fill('2024-01-07');
    const executeResponse = page.waitForResponse((response) =>
      response.url().includes('/bindings/query/execute'),
    );
    await page.getByRole('button', { name: '提交' }).click();
    expect((await executeResponse).status()).toBe(200);

    const exportBtn = page.getByRole('button', { name: '导出 CSV' });
    await expect(exportBtn).toBeVisible();
    await expect(exportBtn).toBeEnabled();
  });
});
