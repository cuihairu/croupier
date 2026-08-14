/**
 * 场景 1: OpenAPI CRUD - 资源页面完整功能
 *
 * 验收标准：
 * - ProTable 分页、详情、create/update/delete
 * - row ban action
 * - 动态菜单
 * - 受控执行
 */

import { test, expect } from '@playwright/test';
import {
  login,
  navigateToConsole,
  waitForPageReady,
  waitForTable,
  expectTableVisible,
  expectModalVisible,
  expectDrawerVisible,
} from './helpers';

test.describe('OpenAPI CRUD', () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
  });

  test('资源列表页加载', async ({ page }) => {
    await navigateToConsole(page, 'players', 'resource--players');
    await waitForPageReady(page);

    // 验证 ProTable 渲染
    await waitForTable(page);
    await expectTableVisible(page);

    // 验证列标题
    await expect(page.locator('th:has-text("ID"), th:has-text("id")').first()).toBeVisible();
    await expect(page.locator('th:has-text("名称"), th:has-text("name")').first()).toBeVisible();
  });

  test('资源列表数据展示', async ({ page }) => {
    await navigateToConsole(page, 'players', 'resource--players');
    await waitForPageReady(page);
    await waitForTable(page);

    // 验证表格有数据行
    const rows = await page.locator('tbody tr, .ant-table-row').count();
    expect(rows).toBeGreaterThan(0);
  });

  test('创建资源', async ({ page }) => {
    await navigateToConsole(page, 'players', 'resource--players');
    await waitForPageReady(page);
    await waitForTable(page);

    // 点击新建按钮
    const createBtn = page
      .locator('button:has-text("新建"), button:has-text("创建"), button:has-text("新增")')
      .first();
    await expect(createBtn).toBeVisible();
    await createBtn.click();

    // 等待 Modal 出现
    await expectModalVisible(page);

    // 填写表单（如果表单字段存在）
    const nameInput = page.locator('.ant-modal input[name="name"], .ant-modal #name').first();
    const hasNameInput = await nameInput.isVisible().catch(() => false);

    if (hasNameInput) {
      await nameInput.fill('测试玩家');
    }

    // 提交
    const submitBtn = page
      .locator('.ant-modal button:has-text("确"), .ant-modal button:has-text("OK")')
      .first();
    await expect(submitBtn).toBeVisible();
    await submitBtn.click();

    // 等待 Modal 关闭或操作完成
    await page.waitForTimeout(2000);
  });

  test('查看详情', async ({ page }) => {
    await navigateToConsole(page, 'players', 'resource--players');
    await waitForPageReady(page);
    await waitForTable(page);

    // 点击查看/详情按钮
    const detailBtn = page
      .locator('a:has-text("查看"), button:has-text("查看"), a:has-text("详情")')
      .first();
    await expect(detailBtn).toBeVisible();
    await detailBtn.click();

    // 等待 Drawer 出现
    await expectDrawerVisible(page);

    // 验证详情内容
    await expect(page.locator('.ant-drawer').first()).toBeVisible();

    // 关闭 Drawer
    await page.locator('.ant-drawer-close').first().click();
  });

  test('编辑资源', async ({ page }) => {
    await navigateToConsole(page, 'players', 'resource--players');
    await waitForPageReady(page);
    await waitForTable(page);

    // 点击编辑按钮
    const editBtn = page.locator('a:has-text("编辑"), button:has-text("编辑")').first();
    await expect(editBtn).toBeVisible();
    await editBtn.click();

    // 等待 Modal 出现
    await expectModalVisible(page);

    // 修改名称
    const nameInput = page.locator('.ant-modal input[name="name"], .ant-modal #name');
    await nameInput.clear();
    await nameInput.fill('更新后的玩家');

    // 提交
    const submitBtn = page
      .locator('.ant-modal button:has-text("确"), .ant-modal button:has-text("OK")')
      .first();
    await submitBtn.click();

    // 等待 Modal 关闭
    await page.locator('.ant-modal').waitFor({ state: 'hidden', timeout: 10000 });
  });

  test('删除资源', async ({ page }) => {
    await navigateToConsole(page, 'players', 'resource--players');
    await waitForPageReady(page);
    await waitForTable(page);

    // 点击删除按钮
    const deleteBtn = page.locator('button:has-text("删除"), a:has-text("删除")').first();
    await expect(deleteBtn).toBeVisible();
    await deleteBtn.click();

    // 确认删除
    const confirmBtn = page
      .locator('.ant-popconfirm .ant-btn-primary, .ant-modal-confirm .ant-btn-primary')
      .first();
    await expect(confirmBtn).toBeVisible();
    await confirmBtn.click();

    await page.waitForTimeout(1000);
  });

  test('行操作 - 封禁', async ({ page }) => {
    await navigateToConsole(page, 'players', 'resource--players');
    await waitForPageReady(page);
    await waitForTable(page);

    // 点击行操作按钮
    const banBtn = page.locator('button:has-text("封禁"), a:has-text("封禁")').first();
    await expect(banBtn).toBeVisible();
    await banBtn.click();

    // 确认操作
    const confirmBtn = page
      .locator('.ant-popconfirm .ant-btn-primary, .ant-modal-confirm .ant-btn-primary')
      .first();
    await expect(confirmBtn).toBeVisible();
    await confirmBtn.click();

    await page.waitForTimeout(1000);
  });
});
