import { test, expect, type Page } from '@playwright/test';

async function login(page: Page) {
  await page.goto('/user/login');
  const username = page.locator('input[id="username"], input[placeholder*="用户名"]').first();
  await username.waitFor({ state: 'visible', timeout: 30000 });
  await username.fill('admin');
  await page.locator('input[type="password"]').fill('admin123');
  await page
    .locator('button', { hasText: /登\s*录/ })
    .first()
    .click();
  await page.waitForFunction(() => Boolean(window.localStorage.getItem('token')), undefined, {
    timeout: 30000,
    polling: 500,
  });
}

test('运行时导入展示 openapi provider', async ({ page }) => {
  await login(page);
  await page.evaluate(() => {
    window.localStorage.setItem('game_id', 'default');
    window.localStorage.setItem('env', 'dev');
  });
  await page.goto('/functions/openapi-sources');
  const card = page.locator('.ant-card', { hasText: '运行时导入' }).first();
  await expect(card).toBeVisible({ timeout: 30000 });
  const cell = card.getByText('players', { exact: true }).first();
  await expect(cell, '运行时导入表应出现 provider players').toBeVisible({
    timeout: 30000,
  });
  await expect(card.getByText('openapi-demo-agent').first()).toBeVisible();
  await expect(card.locator('.ant-table-row').first()).toBeVisible();
});
