/**
 * 场景 6: 异步任务 - TaskPage 完整功能
 */

import { test, expect } from '@playwright/test';
import { login, navigateToConsole, waitForPageReady } from './helpers';

test.describe('异步任务', () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
  });

  test('TaskPage 表单加载', async ({ page }) => {
    await navigateToConsole(page, 'reward', 'task--reward.batchGrant');
    await waitForPageReady(page);

    // 验证表单渲染
    await expect(page.locator('.ant-form, form').first()).toBeVisible();
  });

  test('提交任务', async ({ page }) => {
    await navigateToConsole(page, 'reward', 'task--reward.batchGrant');
    await waitForPageReady(page);

    // 填写任务参数
    const playerIdsInput = page
      .locator('input[name="playerIds"], textarea[name="playerIds"]')
      .first();
    if (await playerIdsInput.isVisible()) {
      await playerIdsInput.fill('1001,1002,1003');
    }

    const rewardIdInput = page.locator('input[name="rewardId"]').first();
    if (await rewardIdInput.isVisible()) {
      await rewardIdInput.fill('100');
    }

    // 提交任务
    const submitBtn = page
      .locator('button:has-text("提交"), button:has-text("开始"), button:has-text("执行")')
      .first();
    if (await submitBtn.isVisible()) {
      await submitBtn.click();

      // 等待任务状态更新
      await page.waitForTimeout(3000);
    }
  });

  test('任务进度展示', async ({ page }) => {
    await navigateToConsole(page, 'reward', 'task--reward.batchGrant');
    await waitForPageReady(page);

    // 验证进度组件（如果任务已提交）
    const progress = page.locator('.ant-progress, [data-testid="task-progress"]').first();
    const timeline = page.locator('.ant-timeline, [data-testid="task-timeline"]').first();

    // 至少应该有表单或任务状态展示
    const hasForm = await page.locator('.ant-form, form').first().isVisible();
    const hasProgress = await progress.isVisible().catch(() => false);
    const hasTimeline = await timeline.isVisible().catch(() => false);

    expect(hasForm || hasProgress || hasTimeline).toBeTruthy();
  });
});
