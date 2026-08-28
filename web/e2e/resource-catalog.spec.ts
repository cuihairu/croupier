/**
 * Resource Catalog 测试
 */

import { test, expect } from '@playwright/test';
import { login, waitForPageReady, expectTableVisible, expectModalVisible } from './helpers';

test.describe('Resource Catalog', () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
  });

  test('资源目录页面加载', async ({ page }) => {
    await page.goto('/functions/resource-catalog');
    await waitForPageReady(page);

    await expect(page.getByText('资源能力目录', { exact: true })).toBeVisible();
    await expectTableVisible(page);
  });

  test('资源列表展示', async ({ page }) => {
    await page.goto('/functions/resource-catalog');
    await waitForPageReady(page);

    await expectTableVisible(page);
    await expect(page.getByText('players', { exact: true })).toBeVisible();
    await expect(page.getByText('inventory', { exact: true })).toBeVisible();
  });

  test('资源详情查看', async ({ page }) => {
    await page.goto('/functions/resource-catalog');
    await waitForPageReady(page);

    // 操作列为紧凑图标按钮（Tooltip 提示），按 aria-label 定位查看
    const detailBtn = page
      .locator('button[title="查看详情"], .ant-table-tbody button:has(.anticon-eye)')
      .first();
    await expect(detailBtn).toBeVisible();
    await detailBtn.click();

    await expectModalVisible(page);
    await expect(page.getByRole('dialog').getByText('资源详情', { exact: true })).toBeVisible();
    await expect(
      page.getByRole('dialog').getByText('Resource Catalog 只维护资源能力语义'),
    ).toBeVisible();
    await page.getByRole('dialog').getByRole('button', { name: 'Close' }).click();
    await expect(page.getByRole('dialog')).toBeHidden();
  });

  test('语义信息展示', async ({ page }) => {
    await page.goto('/functions/resource-catalog');
    await waitForPageReady(page);

    // 验证状态标签
    const statusTag = page
      .locator(
        '.ant-tag:has-text("已识别"), .ant-tag:has-text("identified"), .ant-tag:has-text("待确认")',
      )
      .first();
    await expect(statusTag).toBeVisible();
    // fixed 操作列会让 antd 克隆表头单元格，同一列标题出现两次，取首个断言
    await expect(page.getByText('函数数量', { exact: true }).first()).toBeVisible();
    await expect(page.getByText('语义版本', { exact: true }).first()).toBeVisible();
  });
});
