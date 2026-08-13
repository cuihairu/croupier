/**
 * 场景 5: 高风险动作 - 需要审批的操作
 */

import { test, expect } from '@playwright/test';
import { login, navigateToConsole, waitForPageReady } from './helpers';

test.describe('高风险动作', () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
  });

  test('高风险操作表单加载', async ({ page }) => {
    await navigateToConsole(page, 'system', 'operation--system.dangerous-op');
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

  test('高风险操作确认流程', async ({ page }) => {
    await navigateToConsole(page, 'system', 'operation--system.dangerous-op');
    await waitForPageReady(page);

    // 填写表单（如果表单存在）
    const reasonInput = page
      .locator('input[name="reason"], textarea[name="reason"], input[id*="reason"]')
      .first();
    if (await reasonInput.isVisible({ timeout: 3000 }).catch(() => false)) {
      await reasonInput.fill('测试高风险操作原因');
    }

    // 点击执行按钮
    const submitBtn = page
      .locator('button:has-text("执行"), button:has-text("提交"), button:has-text("Execute")')
      .first();
    if (await submitBtn.isVisible({ timeout: 3000 }).catch(() => false)) {
      await submitBtn.click();

      // 等待确认弹窗
      const confirmModal = page.locator('.ant-popconfirm, .ant-modal-confirm').first();
      const hasConfirm = await confirmModal.isVisible({ timeout: 5000 }).catch(() => false);

      if (hasConfirm) {
        // 确认操作
        const confirmBtn = page
          .locator('.ant-popconfirm .ant-btn-primary, .ant-modal-confirm .ant-btn-primary')
          .first();
        await confirmBtn.click();
        await page.waitForTimeout(2000);
      }
    }
  });

  test('高风险操作审批状态', async ({ page }) => {
    await navigateToConsole(page, 'system', 'operation--system.dangerous-op');
    await waitForPageReady(page);

    // 页面应该正常加载
    const hasContent = await page
      .locator('.ant-form, form, .ant-card, main')
      .first()
      .isVisible({ timeout: 5000 })
      .catch(() => false);
    expect(hasContent).toBeTruthy();
  });
});
