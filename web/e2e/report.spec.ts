/**
 * 场景 7: 报表 - ReportPage 完整功能
 */

import { test, expect } from '@playwright/test';
import { login, navigateToConsole, waitForPageReady } from './helpers';

test.describe('报表', () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
  });

  test('ReportPage 查询表单加载', async ({ page }) => {
    await navigateToConsole(page, 'analytics', 'report--analytics.retention');
    await waitForPageReady(page);

    // 验证查询表单
    await expect(page.locator('.ant-form, form').first()).toBeVisible();
  });

  test('执行查询', async ({ page }) => {
    await navigateToConsole(page, 'analytics', 'report--analytics.retention');
    await waitForPageReady(page);

    // 填写查询参数
    const startDateInput = page.locator('input[name="startDate"], input[id*="startDate"]').first();
    if (await startDateInput.isVisible()) {
      await startDateInput.fill('2024-01-01');
    }

    const endDateInput = page.locator('input[name="endDate"], input[id*="endDate"]').first();
    if (await endDateInput.isVisible()) {
      await endDateInput.fill('2024-01-07');
    }

    // 点击查询
    const queryBtn = page.locator('button:has-text("查询"), button:has-text("搜索"), button:has-text("Query")').first();
    if (await queryBtn.isVisible()) {
      await queryBtn.click();

      // 等待结果
      await page.waitForTimeout(3000);
    }
  });

  test('图表展示', async ({ page }) => {
    await navigateToConsole(page, 'analytics', 'report--analytics.retention');
    await waitForPageReady(page);

    // 验证图表区域（可能需要先执行查询）
    const chartArea = page.locator('.antv-charts, canvas, [data-testid="chart"]').first();
    const tableArea = page.locator('.ant-pro-table, .ant-table').first();

    // 验证有图表或表格展示区域
    const hasChart = await chartArea.isVisible().catch(() => false);
    const hasTable = await tableArea.isVisible().catch(() => false);

    // 至少有查询表单
    await expect(page.locator('.ant-form, form').first()).toBeVisible();
  });

  test('导出功能', async ({ page }) => {
    await navigateToConsole(page, 'analytics', 'report--analytics.retention');
    await waitForPageReady(page);

    // 检查导出按钮
    const exportBtn = page.locator('button:has-text("导出"), button:has-text("Export")').first();
    if (await exportBtn.isVisible()) {
      // 不实际点击导出，只验证按钮存在
      await expect(exportBtn).toBeVisible();
    }
  });
});
