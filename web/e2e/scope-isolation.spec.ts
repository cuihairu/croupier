/**
 * 场景 9: Scope 隔离 - 不同环境数据隔离
 */

import { test, expect } from '@playwright/test';
import {
  login,
  navigateToConsole,
  selectEnv,
  waitForPageReady,
  waitForTable,
  expectTableVisible,
} from './helpers';

test.describe('Scope 隔离', () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
  });

  test('默认环境页面加载', async ({ page }) => {
    await navigateToConsole(page, 'players', 'resource--players');
    await waitForPageReady(page);

    // 验证页面正常加载
    await waitForTable(page);
    await expectTableVisible(page);
  });

  test('切换环境后页面重新加载', async ({ page }) => {
    // 先在默认环境加载
    await navigateToConsole(page, 'players', 'resource--players');
    await waitForPageReady(page);
    await waitForTable(page);

    const listResponse = page.waitForResponse(
      (response) =>
        response.url().includes('/bindings/list/execute') &&
        response.request().headers()['x-env'] === 'staging',
      { timeout: 45000 },
    );
    await selectEnv(page, 'staging');
    expect((await listResponse).status()).toBe(200);
    await expectTableVisible(page);
  });

  test('不同环境数据不串', async ({ page }) => {
    // 在第一个环境查看数据
    await navigateToConsole(page, 'players', 'resource--players');
    await waitForPageReady(page);
    await waitForTable(page);

    await expect(page.getByText('玩家A')).toBeVisible();

    const stagingResponse = page.waitForResponse(
      (response) =>
        response.url().includes('/bindings/list/execute') &&
        response.request().headers()['x-env'] === 'staging',
      { timeout: 45000 },
    );
    await selectEnv(page, 'staging');
    expect((await stagingResponse).status()).toBe(200);
    await expect(page.getByText('玩家A')).toBeVisible();

    // 先注册等待再切环境（reload 后的 list 请求可能极快发出）
    const defaultResponse = page.waitForResponse(
      (response) =>
        response.url().includes('/bindings/list/execute') &&
        response.request().headers()['x-env'] !== 'staging',
      { timeout: 45000 },
    );
    await selectEnv(page, 'development');
    expect((await defaultResponse).status()).toBe(200);
    await expect(page.getByText('玩家A')).toBeVisible();
  });
});
