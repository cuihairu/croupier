/**
 * 场景 6: 异步任务 - TaskPage 完整功能
 */

import { test, expect } from '@playwright/test';
import { login, navigateToConsole, waitForPageReady, expectFormVisible } from './helpers';

test.describe('异步任务', () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
  });

  test('TaskPage 表单加载', async ({ page }) => {
    await navigateToConsole(page, 'reward', 'task--reward.batchGrant');
    await waitForPageReady(page);

    // 验证表单渲染
    await expectFormVisible(page);
  });

  test('提交任务', async ({ page }) => {
    await navigateToConsole(page, 'reward', 'task--reward.batchGrant');
    await waitForPageReady(page);

    // 任务参数来自发布 PageSpec 的 schema，字段缺失必须使测试失败。
    const playerIdsInput = page
      .locator('input[name="playerIds"], textarea[name="playerIds"]')
      .first();
    await expect(playerIdsInput).toBeVisible();
    await playerIdsInput.fill('1001,1002,1003');

    const rewardIdInput = page.locator('input[name="rewardId"]').first();
    await expect(rewardIdInput).toBeVisible();
    await rewardIdInput.fill('100');

    // 提交任务
    const submitBtn = page
      .locator('button:has-text("提交"), button:has-text("开始"), button:has-text("执行")')
      .first();
    await expect(submitBtn).toBeVisible();
    const executeResponse = page.waitForResponse(
      (response) =>
        response.url().includes('/bindings/start/execute') &&
        response.request().method() === 'POST',
    );
    await submitBtn.click();
    expect((await executeResponse).status()).toBe(200);
    await expect(page.getByText('任务已提交', { exact: true }).first()).toBeVisible();
    await expect(page.getByText(/^task-/)).toBeVisible();
  });

  test('任务进度展示', async ({ page }) => {
    await navigateToConsole(page, 'reward', 'task--reward.batchGrant');
    await waitForPageReady(page);

    await expect(page.getByRole('heading', { name: '批量发奖' })).toBeVisible();
    await expectFormVisible(page);
    await expect(page.getByRole('button', { name: '提交' })).toBeVisible();
  });
});
