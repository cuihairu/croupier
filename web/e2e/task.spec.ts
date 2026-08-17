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
    const playerIdsInput = page.getByLabel(/玩家ID|playerIds/i).first();
    await expect(playerIdsInput).toBeVisible();
    await playerIdsInput.fill('1001,1002,1003');

    const rewardIdInput = page.getByLabel(/奖励ID|rewardId/i).first();
    await expect(rewardIdInput).toBeVisible();
    await rewardIdInput.fill('100');

    // 提交任务
    const submitBtn = page.getByRole('button', { name: /提\s*交|开始|执\s*行/ }).first();
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

    await expect(page.getByText('批量发奖').first()).toBeVisible();
    await expectFormVisible(page);
    await expect(page.getByRole('button', { name: /提\s*交/ })).toBeVisible();
  });
});
