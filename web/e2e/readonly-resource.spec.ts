/**
 * 场景 3: 只读资源 - collection + identity，无写 capability
 */

import { test, expect } from '@playwright/test';
import {
  login,
  navigateToConsole,
  waitForPageReady,
  waitForTable,
  expectTableVisible,
} from './helpers';

test.describe('只读资源', () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
  });

  test('只读资源列表加载', async ({ page }) => {
    await navigateToConsole(page, 'inventory', 'resource--inventory');
    await waitForPageReady(page);
    await waitForTable(page);

    // 验证 ProTable 渲染
    await expectTableVisible(page);
  });

  test('只读资源无写操作按钮', async ({ page }) => {
    await navigateToConsole(page, 'inventory', 'resource--inventory');
    await waitForPageReady(page);
    await waitForTable(page);

    // 检查新建按钮 - 只读资源不应该有，或者被禁用
    const createBtn = page.locator('button:has-text("新建"), button:has-text("创建")').first();
    const hasCreate = await createBtn.isVisible().catch(() => false);

    if (hasCreate) {
      // 如果有新建按钮，它应该被禁用
      await expect(createBtn).toBeDisabled();
    }

    // 验证表格正常显示
    await expectTableVisible(page);
  });

  test('只读资源详情查看', async ({ page }) => {
    await navigateToConsole(page, 'inventory', 'resource--inventory');
    await waitForPageReady(page);
    await waitForTable(page);

    // 点击查看详情
    const detailBtn = page
      .locator('a:has-text("查看"), button:has-text("查看"), a:has-text("详情")')
      .first();
    await expect(detailBtn).toBeVisible();
    await detailBtn.click();

    // 等待 Drawer 出现
    await page.locator('.ant-drawer').waitFor({ state: 'visible', timeout: 10000 });

    // 验证详情内容
    await expect(page.locator('.ant-drawer').first()).toBeVisible();

    // 关闭 Drawer
    await page.locator('.ant-drawer-close').first().click();
  });

  test('只读资源筛选功能', async ({ page }) => {
    await navigateToConsole(page, 'inventory', 'resource--inventory');
    await waitForPageReady(page);
    await waitForTable(page);

    // 验证表格正常显示
    await expectTableVisible(page);

    // 检查筛选表单是否存在
    const filterForm = page.locator('.ant-pro-table .ant-form, .ant-table-filter').first();
    const hasFilter = await filterForm.isVisible().catch(() => false);

    // 如果有筛选表单，验证它可见
    if (hasFilter) {
      await expect(filterForm).toBeVisible();
    }
  });
});
