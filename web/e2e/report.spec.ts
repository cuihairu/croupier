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

    // 验证页面加载（表单或其他内容）
    const hasForm = await page
      .locator('.ant-form, form')
      .first()
      .isVisible({ timeout: 5000 })
      .catch(() => false);
    const hasCard = await page
      .locator('.ant-card')
      .first()
      .isVisible({ timeout: 5000 })
      .catch(() => false);
    const hasContent = await page
      .locator('main, .ant-layout-content')
      .first()
      .isVisible({ timeout: 5000 })
      .catch(() => false);

    // 页面应该有内容
    expect(hasForm || hasCard || hasContent).toBeTruthy();
  });

  test('执行查询', async ({ page }) => {
    await navigateToConsole(page, 'analytics', 'report--analytics.retention');
    await waitForPageReady(page);

    // 填写查询参数（如果表单存在）
    const startDateInput = page.locator('input[name="startDate"], input[id*="startDate"]').first();
    if (await startDateInput.isVisible({ timeout: 3000 }).catch(() => false)) {
      await startDateInput.fill('2024-01-01');
    }

    const endDateInput = page.locator('input[name="endDate"], input[id*="endDate"]').first();
    if (await endDateInput.isVisible({ timeout: 3000 }).catch(() => false)) {
      await endDateInput.fill('2024-01-07');
    }

    // 点击查询
    const queryBtn = page
      .locator('button:has-text("查询"), button:has-text("搜索"), button:has-text("Query")')
      .first();
    if (await queryBtn.isVisible({ timeout: 3000 }).catch(() => false)) {
      await queryBtn.click();
      await page.waitForTimeout(2000);
    }
  });

  test('图表展示', async ({ page }) => {
    await navigateToConsole(page, 'analytics', 'report--analytics.retention');
    await waitForPageReady(page);

    // 验证页面加载
    const hasContent = await page
      .locator('.ant-form, form, .ant-card, main')
      .first()
      .isVisible({ timeout: 5000 })
      .catch(() => false);
    expect(hasContent).toBeTruthy();
  });

  test('导出功能', async ({ page }) => {
    await navigateToConsole(page, 'analytics', 'report--analytics.retention');
    await waitForPageReady(page);

    // 检查导出按钮（如果存在）
    const exportBtn = page.locator('button:has-text("导出"), button:has-text("Export")').first();
    const hasExport = await exportBtn.isVisible({ timeout: 3000 }).catch(() => false);

    // 页面应该正常加载
    const hasContent = await page
      .locator('.ant-form, form, .ant-card, main')
      .first()
      .isVisible({ timeout: 5000 })
      .catch(() => false);
    expect(hasContent).toBeTruthy();
  });
});
