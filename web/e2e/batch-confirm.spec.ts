/**
 * UI-003: 批量确认与默认呈现实页验收（发布态）
 */

import { test, expect, type Page } from '@playwright/test';
import { login, waitForPageReady, waitForTable } from './helpers';

async function gotoPage(page: Page, path: string): Promise<void> {
  await page.goto(path, { waitUntil: 'domcontentloaded', timeout: 90000 });
  await waitForPageReady(page);
}

async function selectGame(page: Page, game: string): Promise<void> {
  const gameSel = page.getByTestId('game-selector');
  await expect(gameSel).toBeVisible();
  await gameSel.click();
  const opt = page.locator('.ant-select-item-option').filter({ hasText: game }).first();
  await expect(opt).toBeVisible();
  await opt.click();
  await page.getByTestId('env-selector').waitFor({ state: 'visible', timeout: 60000 });
}

test.describe('UI-003 批量确认实页', () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
    await gotoPage(page, '/console/player/template--player-manage');
    await selectGame(page, 'croupier-demo');
    await gotoPage(page, '/console/player/template--player-manage');
  });

  test('跨页选择总量确认与提交一致 @fixture-health', async ({ page }) => {
    await waitForTable(page);
    const table = page.locator('.ant-table-tbody');
    const rows = page.locator('.ant-table-tbody tr.ant-table-row');
    await expect(rows.first()).toBeVisible();
    await rows.nth(0).locator('input[type="checkbox"]').check();
    await rows.nth(1).locator('input[type="checkbox"]').check();

    await page.locator('.ant-pagination-item-2').first().click();
    await page.waitForTimeout(800);
    const rowsPage2 = page.locator('.ant-table-tbody tr.ant-table-row');
    await rowsPage2.nth(0).locator('input[type="checkbox"]').check();

    await expect(page.getByTestId('selection-count')).toHaveText(/已选择 3 项/);

    const batchBtn = page.getByRole('button', { name: /批量充值/ }).first();
    await expect(batchBtn).toBeVisible();
    await batchBtn.click();

    const amountInput = page.getByRole('spinbutton', { name: /每人数量/ }).first();
    await expect(amountInput).toBeVisible();
    await amountInput.fill('5');

    const formSubmit = page.getByRole('button', { name: /确\s*定|提\s*交/ }).last();
    await formSubmit.click();

    const confirmModal = page.locator('.ant-modal-confirm');
    await expect(confirmModal).toBeVisible();
    await expect(confirmModal.getByText(/5 金/).first()).toBeVisible();
    await expect(confirmModal.getByText(/10 金/).first()).toBeVisible();

    const executeResponse = page.waitForResponse(
      (response) =>
        response.url().includes('/bindings/action.batchRecharge/execute') &&
        response.request().method() === 'POST',
    );
    const confirmBtn = page
      .getByRole('dialog', { name: /核对批量充值/ })
      .getByRole('button', { name: /确\s*定/ })
      .last();
    await confirmBtn.click();
    const resp = await executeResponse;
    expect(resp.status()).toBe(200);
    const payload = resp.request().postDataJSON();
    const selection = payload?.context?.selection ?? [];
    expect(Array.isArray(selection)).toBeTruthy();
    expect(selection.length).toBe(3);
    expect(payload?.context?.form?.amount).toBe(5);
    const ids = selection.map((s: { id: string }) => s.id);
    expect(ids).toContain('20001');
    expect(ids).toContain('20002');
  });
});
