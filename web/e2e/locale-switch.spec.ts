/**
 * i18n 语言切换回归
 *
 * 2026-09 i18n 迁移后页面文案来自 locales 词典：默认（zh-CN）渲染中文，
 * en-US 渲染英文。本用例锁定 umi locale 选择链（localStorage umi_locale >
 * navigator.language）与词典装载不回退失效——防止后续批次迁移把已迁移
 * 页面的 en 译文或 zh 默认渲染弄丢。
 *
 * 其余 spec 在 playwright.config 两个 project 显式 pin zh-CN 下断言中文
 * （devices['Desktop Chrome'] 自带 en-US 会经 baseNavigator 盖过默认值）。
 */

import { test, expect } from '@playwright/test';
import { login, waitForPageReady, expectModalVisible } from './helpers';

test.describe('locale 切换', () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
  });

  test('en-US 下资源详情弹窗渲染英文词典文案', async ({ page }) => {
    // umi locale 优先级：localStorage umi_locale > navigator.language；
    // init script 在应用脚本引导前写入，页面启动即选中 en-US
    await page.addInitScript(() => localStorage.setItem('umi_locale', 'en-US'));

    await page.goto('/functions/resource-catalog');
    await waitForPageReady(page);

    // title 属性来自未迁移的列表页（中文），icon 兜底定位不受语言影响
    const detailBtn = page
      .locator('button[title="查看详情"], .ant-table-tbody button:has(.anticon-eye)')
      .first();
    await expect(detailBtn).toBeVisible();
    await detailBtn.click();

    await expectModalVisible(page);
    // pages.resourceCatalog.detail.title 的 en-US 词典值
    await expect(
      page.getByRole('dialog').getByText('Resource Details', { exact: true }),
    ).toBeVisible();
  });

  test('默认 zh-CN 渲染中文词典文案', async ({ page }) => {
    await page.goto('/functions/resource-catalog');
    await waitForPageReady(page);

    const detailBtn = page
      .locator('button[title="查看详情"], .ant-table-tbody button:has(.anticon-eye)')
      .first();
    await expect(detailBtn).toBeVisible();
    await detailBtn.click();

    await expectModalVisible(page);
    // pages.resourceCatalog.detail.title 的 zh-CN 词典值（非 defaultMessage 回退）
    await expect(page.getByRole('dialog').getByText('资源详情', { exact: true })).toBeVisible();
  });
});
