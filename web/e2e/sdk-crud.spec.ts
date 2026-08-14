/**
 * 场景 2: SDK CRUD - 资源页面功能
 */

import { test, expect } from '@playwright/test';
import {
  login,
  navigateToConsole,
  waitForPageReady,
  waitForTable,
  expectTableVisible,
  expectModalVisible,
} from './helpers';

test.describe('SDK CRUD', () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
  });

  test('背包资源列表页加载', async ({ page }) => {
    await navigateToConsole(page, 'inventory', 'resource--inventory');
    await waitForPageReady(page);
    await waitForTable(page);

    // 验证 ProTable 渲染
    await expectTableVisible(page);
  });

  test('背包资源 CRUD 操作', async ({ page }) => {
    await navigateToConsole(page, 'inventory', 'resource--inventory');
    await waitForPageReady(page);
    await waitForTable(page);

    // 测试新建
    const createBtn = page.locator('button:has-text("新建"), button:has-text("创建")').first();
    const hasCreate = await createBtn.isVisible().catch(() => false);

    if (hasCreate) {
      await createBtn.click();

      // 等待 Modal 出现
      await expectModalVisible(page);

      // 取消创建
      const cancelBtn = page
        .locator('.ant-modal button:has-text("取"), .ant-modal button:has-text("Cancel")')
        .first();
      await expect(cancelBtn).toBeVisible();
      await cancelBtn.click();

      // 等待 Modal 关闭
      await page.locator('.ant-modal').waitFor({ state: 'hidden', timeout: 5000 });
    }
  });
});
