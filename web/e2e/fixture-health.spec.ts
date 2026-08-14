/**
 * @fixture-health 真实链路 fixture 健康检查。
 *
 * 验证 real-dashboard fixture 的四个真实组件均可用：
 *   1. Server HTTP /healthz
 *   2. fixture 控制 API 报告 Agent 已连接且 SDK 函数已注册
 *   3. /players provider 返回两条确定性种子记录
 *   4. 管理端 UI 可以用 bootstrap 的 admin 账号登录
 */

import { expect, test } from '@playwright/test';
import { readRealFixtureState } from './helpers/realFixture';
import { login } from './helpers';

interface FixtureHealth {
  status: string;
  gameId: string;
  env: string;
  agentConnected: boolean;
  functions: string[];
}

interface PlayerListResponse {
  items: Array<{ id: string; name: string; level: number }>;
  total: number;
}

test.describe('real-dashboard fixture @fixture-health', () => {
  test('server health endpoint is reachable @fixture-health', async ({ request }) => {
    const state = readRealFixtureState();
    const resp = await request.get(`${state.serverBaseURL}/healthz`);
    expect(resp.status()).toBe(200);
  });

  test('agent connected and SDK function registered @fixture-health', async ({ request }) => {
    const state = readRealFixtureState();
    const resp = await request.get(`${state.fixtureBaseURL}/__fixture__/health`);
    expect(resp.status()).toBe(200);
    const health = (await resp.json()) as FixtureHealth;
    expect(health.status).toBe('ok');
    expect(health.agentConnected).toBe(true);
    expect(health.functions).toContain('mail.send');
  });

  test('players provider serves deterministic seed data @fixture-health', async ({ request }) => {
    const state = readRealFixtureState();
    const resp = await request.get(`${state.providerBaseURL}/players`);
    expect(resp.status()).toBe(200);
    const list = (await resp.json()) as PlayerListResponse;
    expect(list.total).toBe(2);
    expect(list.items.map((item) => item.id)).toEqual(['p-001', 'p-002']);

    const doc = await request.get(`${state.providerBaseURL}/openapi.json`);
    expect(doc.status()).toBe(200);
    const spec = (await doc.json()) as { paths?: Record<string, unknown> };
    expect(Object.keys(spec.paths ?? {})).toEqual(
      expect.arrayContaining(['/players', '/players/{id}', '/players/{id}/kick']),
    );
  });

  test('admin can log into the real server UI @fixture-health', async ({ page }) => {
    await login(page);
    await expect(page.locator('text=登录').first()).not.toBeVisible({ timeout: 10000 });
  });
});
