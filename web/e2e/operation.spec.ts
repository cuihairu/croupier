/**
 * 场景 4: 独立操作 - OperationPage 完整功能
 */

import { test, expect } from '@playwright/test';
import {
  login,
  navigateToConsole,
  waitForPageReady,
  expectFormVisible,
  expectMessageVisible,
} from './helpers';

test.describe('独立操作', () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
  });

  test('OperationPage 表单加载', async ({ page }) => {
    await navigateToConsole(page, 'mail', 'operation--mail.send');
    await waitForPageReady(page);

    // 验证表单渲染
    await expectFormVisible(page);
  });

  test('填写表单并执行', async ({ page }) => {
    await navigateToConsole(page, 'mail', 'operation--mail.send');
    await waitForPageReady(page);

    // 验证表单存在
    await expectFormVisible(page);

    // 填写表单
    const toInput = page.locator('input[name="to"], input[id*="to"]').first();
    await expect(toInput).toBeVisible();
    await toInput.fill('test@example.com');

    const contentInput = page.locator('textarea[name="content"], textarea[id*="content"]').first();
    await expect(contentInput).toBeVisible();
    await contentInput.fill('测试邮件内容');

    // 点击执行按钮
    const submitBtn = page
      .locator('button:has-text("发送"), button:has-text("执行"), button:has-text("提交")')
      .first();
    await expect(submitBtn).toBeVisible();
    await submitBtn.click();

    // 等待结果 - 验证出现成功或错误消息
    await page.waitForTimeout(2000);
    const hasSuccess = await page
      .locator('.ant-message-success, text=成功')
      .first()
      .isVisible()
      .catch(() => false);
    const hasError = await page
      .locator('.ant-message-error, text=失败')
      .first()
      .isVisible()
      .catch(() => false);
    const hasResult = await page
      .locator('.ant-result, .ant-card')
      .first()
      .isVisible()
      .catch(() => false);
    expect(hasSuccess || hasError || hasResult).toBeTruthy();
  });

  test('高风险操作确认', async ({ page }) => {
    await navigateToConsole(page, 'system', 'operation--system.dangerous-op');
    await waitForPageReady(page);

    // 验证表单存在
    await expectFormVisible(page);

    // 填写表单
    const reasonInput = page.locator('input[name="reason"], textarea[name="reason"]').first();
    await expect(reasonInput).toBeVisible();
    await reasonInput.fill('测试原因');

    // 点击执行
    const submitBtn = page.locator('button:has-text("执行"), button:has-text("提交")').first();
    await expect(submitBtn).toBeVisible();
    await submitBtn.click();

    // 确认弹窗应该出现
    const confirmBtn = page
      .locator('.ant-popconfirm .ant-btn-primary, .ant-modal-confirm .ant-btn-primary')
      .first();
    await expect(confirmBtn).toBeVisible({ timeout: 5000 });
    await confirmBtn.click();

    // 等待结果
    await page.waitForTimeout(2000);
    const hasResult = await page
      .locator('.ant-message, .ant-result')
      .first()
      .isVisible()
      .catch(() => false);
    expect(hasResult).toBeTruthy();
  });
});
