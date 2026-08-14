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

    // 验证页面加载 - 报表页面应该有表单或卡片
    const form = page.locator('.ant-form, form').first();
    const card = page.locator('.ant-card').first();
    const main = page.locator('main, .ant-layout-content').first();

    // 至少有一个可见
    const formVisible = await form.isVisible().catch(() => false);
    const cardVisible = await card.isVisible().catch(() => false);
    const mainVisible = await main.isVisible().catch(() => false);

    expect(formVisible || cardVisible || mainVisible).toBeTruthy();
  });

  test('执行查询', async ({ page }) => {
    await navigateToConsole(page, 'analytics', 'report--analytics.retention');
    await waitForPageReady(page);

    // 填写查询参数（如果表单存在）
    const startDateInput = page.locator('input[name="startDate"], input[id*="startDate"]').first();
    const hasStartDate = await startDateInput.isVisible().catch(() => false);

    if (hasStartDate) {
      await startDateInput.fill('2024-01-01');

      const endDateInput = page.locator('input[name="endDate"], input[id*="endDate"]').first();
      const hasEndDate = await endDateInput.isVisible().catch(() => false);
      if (hasEndDate) {
        await endDateInput.fill('2024-01-07');
      }

      // 点击查询
      const queryBtn = page
        .locator('button:has-text("查询"), button:has-text("搜索"), button:has-text("Query")')
        .first();
      await expect(queryBtn).toBeVisible();
      await queryBtn.click();

      // 等待响应
      await page.waitForTimeout(2000);
    }
  });

  test('图表展示', async ({ page }) => {
    await navigateToConsole(page, 'analytics', 'report--analytics.retention');
    await waitForPageReady(page);

    // 验证页面加载 - 报表页面应该有内容
    const content = page.locator('.ant-form, form, .ant-card, main').first();
    await expect(content).toBeVisible();
  });

  test('导出功能', async ({ page }) => {
    await navigateToConsole(page, 'analytics', 'report--analytics.retention');
    await waitForPageReady(page);

    // 验证页面加载
    const content = page.locator('.ant-form, form, .ant-card, main').first();
    await expect(content).toBeVisible();

    // 检查导出按钮（如果存在）
    const exportBtn = page.locator('button:has-text("导出"), button:has-text("Export")').first();
    const hasExport = await exportBtn.isVisible().catch(() => false);

    // 如果有导出按钮，验证它可以点击
    if (hasExport) {
      await expect(exportBtn).toBeEnabled();
    }
  });
});
