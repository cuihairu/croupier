import { test, expect, type Page } from '@playwright/test';

// 线上验证清单（部署后一次性 smoke）：
//   1. approvals 种子三态（default/dev，空则依次试 prod/stage/test）
//   2. component-templates「创建组合页」主按钮
//   3. assignments「函数开放范围」新页面（无灰度残留）
//   4. catalog「版本」列
//   5. openapi-sources「运行时导入」区块
//   6. 菜单图标 + 中文菜单渲染（svg 存在）

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

async function useScope(page: Page, gameId: string, env: string) {
  await page.evaluate(
    ([g, e]) => {
      window.localStorage.setItem('game_id', g as string);
      window.localStorage.setItem('env', e as string);
    },
    [gameId, env],
  );
}

test('线上部署验证清单', async ({ page }) => {
  await login(page);
  await useScope(page, 'default', 'dev');

  // ── 1. approvals 种子三态 ──────────────────────────────────────────
  // 「待我审批」默认 tab：应出现种子 pending（operator/gm01 发起）
  await page.goto('/approvals');
  await expect(
    page.getByText('player.unban').first(),
    '待我审批应出现种子 pending player.unban',
  ).toBeVisible({ timeout: 30000 });
  // 「全部」tab：三态 Tag 与申请人可见
  await page.locator('.ant-tabs-tab', { hasText: '全部' }).first().click();
  await expect(page.getByText('mail.send').first(), '全部视图应有已通过 mail.send').toBeVisible({
    timeout: 30000,
  });
  await expect(page.getByText('已通过').first()).toBeVisible();
  await expect(page.getByText('已拒绝').first()).toBeVisible();
  await expect(page.getByText('待审批').first()).toBeVisible();

  // ── 2. component-templates 创建组合页按钮 ─────────────────────────
  await page.goto('/functions/component-templates');
  await expect(
    page.getByRole('button', { name: /创\s*建\s*组\s*合\s*页/ }).first(),
    'component-templates 页头应有「创建组合页」按钮',
  ).toBeVisible({ timeout: 30000 });

  // ── 3. assignments 函数开放范围 ───────────────────────────────────
  await page.goto('/functions/assignments');
  await expect(page.getByText('函数开放范围').first(), 'assignments 应显示新标题').toBeVisible({
    timeout: 30000,
  });
  const bodyText = await page.locator('body').innerText();
  expect(bodyText, 'assignments 不应残留灰度文案').not.toContain('灰度');

  // ── 4. catalog 版本列 ─────────────────────────────────────────────
  await page.goto('/functions/catalog');
  await expect(
    page.locator('th', { hasText: '版本' }).first(),
    '函数目录表头应有版本列',
  ).toBeVisible({ timeout: 30000 });

  // ── 5. openapi-sources 运行时导入区块 ─────────────────────────────
  await page.goto('/functions/openapi-sources');
  await expect(
    page.getByText('运行时导入').first(),
    'openapi-sources 应有运行时导入区块',
  ).toBeVisible({ timeout: 30000 });

  // ── 6. 菜单图标 + 中文菜单 ─────────────────────────────────────
  // 顶层组「函数与页面」带 function 图标（span[role=img] > svg）；子项「函数目录」为中文文本。
  const groupItem = page
    .locator('.ant-menu-submenu-title, li[role="menuitem"]', { hasText: '函数与页面' })
    .first();
  await expect(groupItem, '菜单组「函数与页面」应可见').toBeVisible({ timeout: 15000 });
  await expect(
    groupItem.locator('span[role="img"] svg, .anticon svg').first(),
    '菜单组应有 svg 图标',
  ).toBeVisible();
  const menuItem = page
    .locator('li[role="menuitem"], .ant-menu-item', { hasText: '函数目录' })
    .first();
  await expect(menuItem).toBeVisible({ timeout: 15000 });
});
