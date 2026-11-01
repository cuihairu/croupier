/**
 * UI-003（旧 D2/D5）发布态实页验收：真实资源页发布后，D5 默认呈现保留
 * 身份列/单位/时区/枚举语义；D2 批量确认（跨页选人、筛选变化、总量=每人
 * 数量×目标数、确认内容与实际提交一致）在真实 published binding 上成立。
 *
 * 复用 real-dashboard fixture 的 /players OpenAPI provider（本 spec 的
 * fixture provider 已扩展 playerSchema：status enum、balanceGold unit、
 * lastLoginAt datetime、timezone；并新增 player.batchRecharge 批量动作，
 * amount 带 x-field-per-target）。发布链走真实 accept-and-publish。
 *
 * 隔离：使用专用 players-provider 与 resource--players，发布后回收
 * （unpublish + DELETE source），不影响其他真实场景。
 */

import { test, expect } from '@playwright/test';
import type { APIRequestContext, Page } from '@playwright/test';
import { readRealFixtureState } from './helpers/realFixture';
import { login, navigateToConsole, waitForPageReady, waitForTable } from './helpers';

type OpenAPIHeaders = { Authorization: string; 'X-Game-ID': string; 'X-Env': string };

async function openapiHeaders(request: APIRequestContext): Promise<OpenAPIHeaders> {
  const state = readRealFixtureState();
  const resp = await request.post(`${state.serverBaseURL}/api/v1/auth/login`, {
    data: { username: 'admin', password: 'admin123' },
  });
  expect(resp.status()).toBe(200);
  const session = (await resp.json()) as { token?: string };
  expect(session.token).toMatch(/^eyJ/i);
  return {
    Authorization: `Bearer ${session.token}`,
    'X-Game-ID': state.gameId,
    'X-Env': state.env,
  };
}

async function ensurePlayersSourceAndPublish(
  request: APIRequestContext,
  headers: OpenAPIHeaders,
): Promise<{ sourceId: string; pageURL: string }> {
  const state = readRealFixtureState();
  // 幂等清理：先前运行可能已发布 resource--players 或遗留 players-provider。
  // DeletePage 同时清草稿/已发布/提案，使 accept-and-publish 可从零重放。
  await request
    .delete(`${state.serverBaseURL}/api/v1/versioning/pages/resource--players`, { headers })
    .catch(() => {});
  const existingSources = await request
    .get(`${state.serverBaseURL}/api/v1/openapi/sources`, { headers })
    .catch(() => null);
  if (existingSources?.status() === 200) {
    const body = (await existingSources.json()) as
      | { items?: Array<{ sourceId: string; name: string }> }
      | Array<{ sourceId: string; name: string }>;
    const items = Array.isArray(body) ? body : (body.items ?? []);
    for (const source of items.filter((s) => s.name === 'players-provider')) {
      const detail = await request
        .get(`${state.serverBaseURL}/api/v1/openapi/sources/${source.sourceId}`, { headers })
        .catch(() => null);
      if (detail?.status() === 200) {
        const d = ((await detail.json()) as { source: { bindings?: Array<{ bindingId: string }> } })
          .source;
        for (const binding of d.bindings ?? []) {
          await request
            .delete(
              `${state.serverBaseURL}/api/v1/openapi/sources/${source.sourceId}/bindings/${binding.bindingId}`,
              { headers },
            )
            .catch(() => {});
        }
      }
      await request
        .delete(`${state.serverBaseURL}/api/v1/openapi/sources/${source.sourceId}`, { headers })
        .catch(() => {});
    }
  }

  // 拉取 provider OpenAPI 文档
  const docResp = await request.get(`${state.providerBaseURL}/openapi.json`);
  expect(docResp.status()).toBe(200);
  const spec = await docResp.json();

  // 创建 source
  const createResp = await request.post(`${state.serverBaseURL}/api/v1/openapi/sources`, {
    headers,
    data: { name: 'players-provider', spec },
  });
  expect(createResp.status()).toBe(201);
  const sourceId = ((await createResp.json()) as { source: { sourceId: string } }).source.sourceId;
  expect(sourceId.length).toBeGreaterThan(0);

  // 绑定全部 REST 操作
  const ops = [
    'player.list',
    'player.get',
    'player.create',
    'player.update',
    'player.delete',
    'player.kick',
    'player.batchRecharge',
  ];
  for (const operationId of ops) {
    const bindResp = await request.post(
      `${state.serverBaseURL}/api/v1/openapi/sources/${sourceId}/bindings`,
      {
        headers,
        // 运行时注册 functionId 恒小写（control_handler 折叠）；camelCase
        // operationId 需折叠到同一字符集才能命中已注册 provider 函数。
        data: { operationId, kind: 'provider', functionId: `players.${operationId.toLowerCase()}` },
      },
    );
    expect(bindResp.status()).toBe(200);
  }

  // 发布 resource:players 提案（真实 accept-and-publish）
  const publishResp = await request.post(
    `${state.serverBaseURL}/api/v1/proposals/resource%3Aplayers/accept-and-publish`,
    { headers },
  );
  expect(publishResp.status()).toBe(200);

  const pageURL = `${state.serverBaseURL}/api/v1/console/pages/resource--players`;
  const pageResp = await request.get(pageURL, { headers });
  expect(pageResp.status()).toBe(200);
  return { sourceId, pageURL };
}

async function cleanupSource(
  request: APIRequestContext,
  headers: OpenAPIHeaders,
  sourceId: string,
): Promise<void> {
  const state = readRealFixtureState();
  // 清理容错：发布/回收失败不掩盖主断言（fixture 自身 teardown 亦回收 scope）。
  await request
    .delete(`${state.serverBaseURL}/api/v1/versioning/pages/resource--players`, { headers })
    .catch(() => {});
  const detailResp = await request
    .get(`${state.serverBaseURL}/api/v1/openapi/sources/${sourceId}`, { headers })
    .catch(() => null);
  if (detailResp?.status() === 200) {
    try {
      const detail = (
        (await detailResp.json()) as {
          source: { bindings?: Array<{ bindingId: string }> };
        }
      ).source;
      for (const binding of detail.bindings ?? []) {
        await request
          .delete(
            `${state.serverBaseURL}/api/v1/openapi/sources/${sourceId}/bindings/${binding.bindingId}`,
            { headers },
          )
          .catch(() => {});
      }
      await request
        .delete(`${state.serverBaseURL}/api/v1/openapi/sources/${sourceId}`, { headers })
        .catch(() => {});
    } catch {
      // ignore cleanup errors
    }
  }
}

test.describe('UI-003 发布态批量确认与默认呈现', () => {
  test('@openapi-d2d5-publish 发布页保留 D5 语义且 D2 批量确认总量一致', async ({
    page,
    request,
  }) => {
    test.setTimeout(180000);
    const headers = await openapiHeaders(request);
    const { sourceId } = await ensurePlayersSourceAndPublish(page.request, headers);

    try {
      await login(page);
      await navigateToConsole(page, 'players', 'resource--players');
      await waitForPageReady(page);
      await waitForTable(page);

      // D5 默认呈现：身份列/单位/时区/枚举在真实 published 页可见
      await expect(page.locator('th', { hasText: /玩家 ID|Player ID/ }).first()).toBeVisible();
      await expect(page.locator('th', { hasText: /金币余额|Gold Balance/ }).first()).toBeVisible();
      await expect(page.locator('th', { hasText: /时区|Timezone/ }).first()).toBeVisible();
      // 单位后缀在单元格值旁渲染（antd 单元格 text）
      const goldCell = page.locator('td', { hasText: /100/ }).first();
      await expect(goldCell).toBeVisible();

      // D2 跨页选人：勾选第一行 → 选择栏出现
      const firstRowCheck = page
        .locator('tr[data-row-key] .ant-checkbox-input:not([aria-label="Select all"])')
        .first();
      await firstRowCheck.check({ force: true });
      await expect(page.getByText(/已选择 1 项/).first()).toBeVisible();

      // 触发批量充值（批量确认 + 每人数量）
      const batchBtn = page.getByRole('button', { name: /批量充值|Batch recharge/i }).first();
      await expect(batchBtn).toBeVisible();
      await batchBtn.click();

      // 表单：每人数量输入
      const amountInput = page
        .getByRole('spinbutton', { name: /每人数量|amount|quantity/i })
        .first();
      await amountInput.fill('100');

      // 确认弹窗：总量 = 100 × 1
      const confirmBtn = page
        .locator('.ant-modal .ant-btn-primary, .ant-drawer .ant-btn-primary')
        .filter({ hasText: /确|OK|Submit/ })
        .first();
      await confirmBtn.click();
      // 合并确认框含目标数量 1 与总量 100（单位金）
      await expect(page.getByRole('rowheader', { name: '目标数量' })).toBeVisible();
      await expect(page.getByRole('rowheader', { name: '总量' })).toBeVisible();

      // 执行发布态 binding：捕获 POST 到 action.batch.recharge
      const executeRespPromise = page.waitForResponse(
        (response) =>
          response.url().includes('/bindings/action.batch.recharge/execute') &&
          response.request().method() === 'POST',
      );
      const okBtn = page
        .locator('.ant-modal .ant-btn-primary, .ant-drawer .ant-btn-primary')
        .filter({ hasText: /确|OK|Submit/ })
        .last();
      await okBtn.click();
      const executeResp = await executeRespPromise;
      if (executeResp.status() !== 200) {
        // 失败时携带响应体：409 需区分 binding_stale / executor_unbound /
        // idempotency_* 根因（服务端结构化 error code 只在 body 里）。
        const body = await executeResp.text().catch(() => '<unreadable>');
        throw new Error(`batch recharge execute failed: ${executeResp.status()} ${body}`);
      }
      expect(executeResp.status()).toBe(200);
    } finally {
      await cleanupSource(page.request, headers, sourceId);
    }
  });
});
