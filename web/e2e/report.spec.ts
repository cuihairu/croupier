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

    await expect(page.getByText('留存分析').first()).toBeVisible();
    await expectFormVisible(page);
    await expect(page.getByRole('textbox', { name: /开始日期/ })).toBeVisible();
    await expect(page.getByRole('textbox', { name: /结束日期/ })).toBeVisible();
  });

  test('执行查询', async ({ page }) => {
    await navigateToConsole(page, 'analytics', 'report--analytics.retention');
    await waitForPageReady(page);

    const startDateInput = page.getByRole('textbox', { name: /开始日期/ }).first();
    await expect(startDateInput).toBeVisible();
    await startDateInput.fill('2024-01-01');

    const endDateInput = page.getByRole('textbox', { name: /结束日期/ }).first();
    await expect(endDateInput).toBeVisible();
    await endDateInput.fill('2024-01-07');

    const queryBtn = page.getByRole('button', { name: /提\s*交/ });
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
  });

  test('图表展示', async ({ page }) => {
    await navigateToConsole(page, 'analytics', 'report--analytics.retention');
    await waitForPageReady(page);

    // 查询前为空态；执行一次查询后图表/表格 Tab 出现
    await page
      .getByRole('textbox', { name: /开始日期/ })
      .first()
      .fill('2024-01-01');
    await page
      .getByRole('textbox', { name: /结束日期/ })
      .first()
      .fill('2024-01-07');
    await page.getByRole('button', { name: /提\s*交/ }).click();
    await expect(page.getByRole('tab', { name: /图表/ })).toBeVisible();
  });

  test('导出功能', async ({ page }) => {
    await navigateToConsole(page, 'analytics', 'report--analytics.retention');
    await waitForPageReady(page);

    await page
      .getByRole('textbox', { name: /开始日期/ })
      .first()
      .fill('2024-01-01');
    await page
      .getByRole('textbox', { name: /结束日期/ })
      .first()
      .fill('2024-01-07');
    const executeResponse = page.waitForResponse((response) =>
      response.url().includes('/bindings/query/execute'),
    );
    await page.getByRole('button', { name: /提\s*交/ }).click();
    expect((await executeResponse).status()).toBe(200);

    const exportBtn = page.getByRole('button', { name: '导出 CSV' });
    await expect(exportBtn).toBeVisible();
    await expect(exportBtn).toBeEnabled();
  });
});
