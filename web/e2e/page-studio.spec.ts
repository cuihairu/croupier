/**
 * Page Studio 测试
 */

import { test, expect } from '@playwright/test';
import { login, waitForPageReady } from './helpers';

test.describe('Page Studio', () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
  });

  test('页面列表加载', async ({ page }) => {
    await page.goto('/functions/pages');
    await waitForPageReady(page);

    await expect(page.getByText('页面工作台').first()).toBeVisible();
    await expect(page.getByText('默认页面先生成 Proposal')).toBeVisible();
    await expect(page.getByRole('tab', { name: /可直接发布/ })).toBeVisible();
    await expect(page.getByRole('tab', { name: /需要处理/ })).toBeVisible();
    await expect(page.getByRole('tab', { name: /契约变更/ })).toBeVisible();
  });

  test('Proposal Inbox 展示', async ({ page }) => {
    await page.goto('/functions/pages');
    await waitForPageReady(page);

    await expect(page.getByPlaceholder('搜索提案、页面或资源')).toBeVisible();
    await expect(page.getByRole('button', { name: '刷新' }).first()).toBeVisible();
    await expect(page.locator('.ant-result-error')).toHaveCount(0);
    await expect(page.getByText('加载失败')).toHaveCount(0);
  });

  test('预览功能', async ({ page }) => {
    await page.goto('/functions/pages');
    await waitForPageReady(page);

    const previewBtn = page
      .locator('button:has-text("预览"), a:has-text("预览"), button:has-text("Preview")')
      .first();
    await expect(previewBtn).toBeVisible();
    await previewBtn.click();

    await expect(page.getByRole('dialog')).toBeVisible();
    await expect(page.getByRole('dialog').getByText('默认页面预览')).toBeVisible();
    await page.getByRole('dialog').getByRole('button', { name: 'Close' }).click();
    await expect(page.getByRole('dialog')).toBeHidden();
  });

  test('发布功能', async ({ page }) => {
    await page.goto('/functions/pages');
    await waitForPageReady(page);

    // 排除主视图「一键发布全部」（modal.confirm 流程），命中提案行内发布
    // （发布确认弹窗：含挂载菜单选择，不选菜单则仅发布）
    const publishBtn = page
      .locator(
        'button:has-text("发布"):not(:has-text("一键")), a:has-text("发布"), button:has-text("Publish")',
      )
      .first();
    await expect(publishBtn).toBeVisible();
    await publishBtn.click();

    const confirmBtn = page.locator('.ant-modal .ant-btn-primary').first();
    await expect(confirmBtn).toBeVisible();
    const publishResponse = page.waitForResponse(
      (response) =>
        response.url().includes('/accept-and-publish') && response.request().method() === 'POST',
    );
    await confirmBtn.click();
    expect((await publishResponse).status()).toBe(200);
    await expect(page.getByText('已直接发布').first()).toBeAttached();
  });

  test('一键发布/下架按钮在主视图直接可见', async ({ page }) => {
    await page.goto('/functions/pages');
    await waitForPageReady(page);

    // 不展开「高级页面管理」折叠面板，主视图（提案收件箱上方）直接可见
    await expect(page.getByRole('button', { name: /一键发布全部/ })).toBeVisible();
    await expect(page.getByRole('button', { name: /一键下架全部/ })).toBeVisible();

    // 点击一键发布出现 confirm 弹窗（modal.confirm），取消不产生请求
    await page.getByRole('button', { name: /一键发布全部/ }).click();
    await expect(page.getByText('将重算提案并把所有 ready/basic 提案按真实链路发布')).toBeVisible();
    await page.getByRole('button', { name: /取 消/ }).click();
    await expect(page.getByText('将重算提案并把所有 ready/basic 提案按真实链路发布')).toBeHidden();
  });

  test('编辑器内挂载菜单并保存发布', async ({ page }) => {
    await page.goto('/functions/pages');
    await waitForPageReady(page);

    // 高级页面管理（默认展开）：草稿行「编辑」（icon-only + Tooltip）打开 EditorModal。
    // 主视图提案行（resource--players 同名）操作列无 edit 图标，用图标过滤天然区分
    const row = page
      .getByRole('row', { name: /resource--players/ })
      .filter({ has: page.locator('button:has(.anticon-edit)') });
    await expect(row).toHaveCount(1);
    await row.locator('button:has(.anticon-edit)').click();

    const dialog = page.getByRole('dialog');
    await expect(dialog).toBeVisible();
    await expect(dialog.getByText('页面编辑')).toBeVisible();
    await expect(dialog.getByText('resource--players').first()).toBeVisible();

    // footer 挂载菜单选择（T-M8 后控制台导航只由 menu_items 驱动）。
    // 弹窗内还有语言选择 combobox，按挂载 placeholder 过滤避免命中
    const mountSelect = dialog.locator('.ant-select').filter({ hasText: '挂载到菜单' }).first();
    await expect(mountSelect).toBeVisible();
    await mountSelect.click();
    const option = page.getByRole('treeitem', { name: /邮件/ }).first();
    await expect(option).toBeVisible();
    const menuResponse = page.waitForResponse(
      (response) =>
        response.url().includes('/pages/resource--players/menu') &&
        response.request().method() === 'PUT',
    );
    await option.click();

    // 保存并发布：保存 → 发布 → 挂载（挂载失败不回滚发布）
    const publishResponse = page.waitForResponse(
      (response) =>
        response.url().includes('/pages/resource--players/publish') &&
        response.request().method() === 'POST',
    );
    await dialog.getByRole('button', { name: /保存并发布/ }).click();
    expect((await publishResponse).status()).toBe(200);
    const mounted = await menuResponse;
    expect(mounted.status()).toBe(200);
    expect(mounted.request().postDataJSON()).toEqual({ menuId: 1 });
    await expect(page.getByText('已保存并发布，已挂载到所选菜单').first()).toBeVisible();
  });
});
