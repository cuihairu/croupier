import { test, expect, type Page } from '@playwright/test';
import { login, waitForPageReady, waitForTable } from './helpers';

async function gotoPage(page: Page, path: string): Promise<void> {
  await page.goto(path, { waitUntil: 'domcontentloaded', timeout: 90000 });
  await waitForPageReady(page);
}

test('probe target list @fixture-health', async ({ page }) => {
  await login(page);
  await gotoPage(page, '/console/player/template--player-manage');
  const gameSel = page.getByTestId('game-selector');
  await gameSel.click();
  await page
    .locator('.ant-select-item-option')
    .filter({ hasText: 'croupier-demo' })
    .first()
    .click();
  await page.getByTestId('env-selector').waitFor({ state: 'visible', timeout: 60000 });
  await gotoPage(page, '/console/player/template--player-manage');
  await waitForTable(page);

  const rows = page.locator('.ant-table-tbody tr.ant-table-row');
  await rows.nth(0).locator('input[type="checkbox"]').check();
  await rows.nth(1).locator('input[type="checkbox"]').check();
  await page.getByTitle('下一页').first().click();
  await page.waitForTimeout(1500);
  const rows2 = page.locator('.ant-table-tbody tr.ant-table-row');
  console.log('page2 row count=', await rows2.count());
  await rows2.nth(0).locator('input[type="checkbox"]').check();
  await page.waitForTimeout(800);
  console.log('count label:', await page.getByTestId('selection-count').innerText());
  const targetBtn = page.locator('button:has-text("查看目标清单")');
  console.log('target disabled?', await targetBtn.isDisabled());
  await targetBtn.click();
  await page.waitForTimeout(1200);
  const modalRows = page.locator('[data-testid="target-list-table"] tbody tr');
  console.log('target list rows=', await modalRows.count());
  const body = await page.locator('[data-testid="target-list-table"]').innerText();
  console.log('target ids=', body.slice(0, 200).replace(/\n/g, '|'));
});
