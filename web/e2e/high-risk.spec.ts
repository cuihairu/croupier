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

    await expect(page.getByText('高风险操作').first()).toBeVisible();
    await expectFormVisible(page);
    await expect(page.getByLabel(/原因|理由/i).first()).toBeVisible();
  });

  test('高风险操作确认流程', async ({ page }) => {
    await navigateToConsole(page, 'system', 'operation--system.dangerous-op');
    await waitForPageReady(page);

    const reasonInput = page.getByLabel(/原因|理由/i).first();
    await expect(reasonInput).toBeVisible();
    await reasonInput.fill('测试高风险操作原因');

    const submitBtn = page.getByRole('button', { name: /提\s*交/ });
    await expect(submitBtn).toBeVisible();
    await submitBtn.click();

    const confirmModal = page.getByRole('dialog');
    await expect(confirmModal).toBeVisible();
    await expect(confirmModal.getByText('确认高风险操作')).toBeVisible();
    const executeResponse = page.waitForResponse(
      (response) =>
        response.url().includes('/bindings/main/execute') && response.request().method() === 'POST',
    );
    await confirmModal.getByRole('button', { name: /确\s*定|确\s*认/ }).click();
    expect((await executeResponse).status()).toBe(200);
    await expect(page.getByText('操作成功', { exact: true })).toBeVisible();
  });

  test('高风险操作审批状态', async ({ page }) => {
    await navigateToConsole(page, 'system', 'operation--system.dangerous-op');
    await waitForPageReady(page);

    await expect(page.getByText('确认高风险操作')).toHaveCount(0);
    await expect(page.getByRole('button', { name: /提\s*交/ })).toBeVisible();
  });
});
