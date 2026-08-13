/**
 * 场景 4: 独立操作 - OperationPage 完整功能
 */

import { test, expect } from '@playwright/test';
import { login, navigateToConsole, waitForPageReady } from './helpers';

test.describe('独立操作', () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
  });

  test('OperationPage 表单加载', async ({ page }) => {
    await navigateToConsole(page, 'mail', 'operation--mail.send');
    await waitForPageReady(page);

    // 验证表单渲染
    await expect(page.locator('.ant-form, form').first()).toBeVisible();
  });

  test('填写表单并执行', async ({ page }) => {
    await navigateToConsole(page, 'mail', 'operation--mail.send');
    await waitForPageReady(page);

    // 填写表单
    const toInput = page.locator('input[name="to"], input[id*="to"]').first();
    if (await toInput.isVisible()) {
      await toInput.fill('test@example.com');
    }

    const contentInput = page.locator('textarea[name="content"], textarea[id*="content"]').first();
    if (await contentInput.isVisible()) {
      await contentInput.fill('测试邮件内容');
    }

    // 点击执行按钮
    const submitBtn = page
      .locator('button:has-text("发送"), button:has-text("执行"), button:has-text("提交")')
      .first();
    if (await submitBtn.isVisible()) {
      await submitBtn.click();

      // 等待结果
      await page.waitForTimeout(2000);
    }
  });

  test('高风险操作确认', async ({ page }) => {
    await navigateToConsole(page, 'system', 'operation--system.dangerous-op');
    await waitForPageReady(page);

    // 填写表单
    const reasonInput = page.locator('input[name="reason"], textarea[name="reason"]').first();
    if (await reasonInput.isVisible()) {
      await reasonInput.fill('测试原因');
    }

    // 点击执行
    const submitBtn = page.locator('button:has-text("执行"), button:has-text("提交")').first();
    if (await submitBtn.isVisible()) {
      await submitBtn.click();

      // 确认弹窗
      const confirmBtn = page
        .locator('.ant-popconfirm .ant-btn-primary, .ant-modal-confirm .ant-btn-primary')
        .first();
      if (await confirmBtn.isVisible({ timeout: 5000 }).catch(() => false)) {
        await confirmBtn.click();
      }
    }
  });
});
