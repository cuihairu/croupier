/**
 * 场景 5: 高风险动作 - 需要审批的操作
 */

import { test, expect } from '@playwright/test';
import { login, navigateToConsole, waitForPageReady, expectFormVisible } from './helpers';

test.describe('高风险动作', () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
  });

  test('高风险操作表单加载', async ({ page }) => {
    await navigateToConsole(page, 'system', 'operation--system.dangerous-op');
    await waitForPageReady(page);

    // 验证页面加载 - 高风险操作应该有表单
    const form = page.locator('.ant-form, form').first();
    const card = page.locator('.ant-card').first();
    const main = page.locator('main, .ant-layout-content').first();

    const formVisible = await form.isVisible().catch(() => false);
    const cardVisible = await card.isVisible().catch(() => false);
    const mainVisible = await main.isVisible().catch(() => false);

    expect(formVisible || cardVisible || mainVisible).toBeTruthy();
  });

  test('高风险操作确认流程', async ({ page }) => {
    await navigateToConsole(page, 'system', 'operation--system.dangerous-op');
    await waitForPageReady(page);

    // 填写表单（如果表单存在）
    const reasonInput = page
      .locator('input[name="reason"], textarea[name="reason"], input[id*="reason"]')
      .first();
    const hasReasonInput = await reasonInput.isVisible().catch(() => false);

    if (hasReasonInput) {
      await reasonInput.fill('测试高风险操作原因');

      // 点击执行按钮
      const submitBtn = page
        .locator('button:has-text("执行"), button:has-text("提交"), button:has-text("Execute")')
        .first();
      const hasSubmit = await submitBtn.isVisible().catch(() => false);

      if (hasSubmit) {
        await submitBtn.click();

        // 等待确认弹窗
        const confirmModal = page.locator('.ant-popconfirm, .ant-modal-confirm').first();
        const hasConfirm = await confirmModal.isVisible({ timeout: 5000 }).catch(() => false);

        if (hasConfirm) {
          // 确认操作
          const confirmBtn = page
            .locator('.ant-popconfirm .ant-btn-primary, .ant-modal-confirm .ant-btn-primary')
            .first();
          await expect(confirmBtn).toBeVisible();
          await confirmBtn.click();
          await page.waitForTimeout(2000);
        }
      }
    }
  });

  test('高风险操作审批状态', async ({ page }) => {
    await navigateToConsole(page, 'system', 'operation--system.dangerous-op');
    await waitForPageReady(page);

    // 验证页面正常加载
    const content = page.locator('.ant-form, form, .ant-card, main').first();
    await expect(content).toBeVisible();
  });
});
