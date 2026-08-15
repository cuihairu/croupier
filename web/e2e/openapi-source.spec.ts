/**
 * 场景 10: OpenAPI Source - 导入和绑定
 */

import { test, expect } from '@playwright/test';
import type { APIRequestContext } from '@playwright/test';
import { readRealFixtureState } from './helpers/realFixture';
import { cleanupPlayersSourceBindings } from './helpers/openapiPlayers';
import {
  login,
  waitForPageReady,
  expectTableVisible,
  expectModalVisible,
  expectDrawerVisible,
} from './helpers';

test.describe('OpenAPI Source', () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
  });

  test('OpenAPI Source 页面加载', async ({ page }) => {
    await page.goto('/system/functions/openapi-sources');
    await waitForPageReady(page);

    await expect(page.getByRole('heading', { name: 'OpenAPI Sources' })).toBeVisible();
    await expect(page.getByText('Source 不是 UI，也不是自动注册')).toBeVisible();
    await expectTableVisible(page);
  });

  test('上传 OpenAPI 文档', async ({ page }) => {
    await page.goto('/system/functions/openapi-sources');
    await waitForPageReady(page);

    const uploadBtn = page.getByRole('button', { name: '上传 Source' });
    await expect(uploadBtn).toBeVisible();
    await uploadBtn.click();

    await expectModalVisible(page);
    await expect(page.getByRole('dialog').getByText('不要在 OpenAPI 中写 UI')).toBeVisible();

    const cancelBtn = page.getByRole('dialog').getByRole('button', { name: '取消' });
    await expect(cancelBtn).toBeVisible();
    await cancelBtn.click();
    await expect(page.getByRole('dialog')).toBeHidden();
  });

  test('Provider 绑定', async ({ page }) => {
    await page.goto('/system/functions/openapi-sources');
    await waitForPageReady(page);

    const openBtn = page.getByRole('button', { name: '打开' }).first();
    await expect(openBtn).toBeVisible();
    await openBtn.click();
    await expectDrawerVisible(page);
    await expect(page.getByText('Operations', { exact: true })).toBeVisible();

    const bindBtn = page.getByRole('button', { name: '绑定', exact: true }).first();
    await expect(bindBtn).toBeVisible();
    await bindBtn.click();

    await expectModalVisible(page);
    await expect(page.getByRole('dialog').getByText('当前只启用 Provider binding')).toBeVisible();

    const cancelBtn = page.getByRole('dialog').getByRole('button', { name: '取消' });
    await expect(cancelBtn).toBeVisible();
    await cancelBtn.click();
    await expect(page.getByRole('dialog')).toBeHidden();
  });
});

type OpenAPISourceOperation = {
  operationId: string;
  method: string;
  path: string;
  resource?: string;
  operation?: string;
  capability?: string;
  bound: boolean;
};

type OpenAPISourceDetail = {
  source: {
    sourceId: string;
    name: string;
    revision: number;
    operationCount: number;
    operations?: OpenAPISourceOperation[];
    bindings?: Array<{ bindingId: string; operationId: string; functionId: string; kind: string }>;
    diagnostics?: Array<{ code: string; severity: string }>;
  };
};

type ProposalDTO = {
  proposalKey: string;
  pageKey: string;
  pageType: string;
  resourceKey?: string;
  quality: string;
  status: string;
};

type ResourceCatalogItem = {
  resourceKey: string;
  status: string;
  functions: Array<{ functionId: string; capability: string; source: string }>;
  semantics?: {
    source?: string;
    hasIdentity?: boolean;
    hasCollection?: boolean;
    hasCreate?: boolean;
    hasUpdate?: boolean;
    hasDelete?: boolean;
  };
};

const expectedPlayersOperations = [
  {
    operationId: 'player.create',
    method: 'POST',
    path: '/players',
    operation: 'create',
    capability: 'create',
  },
  {
    operationId: 'player.delete',
    method: 'DELETE',
    path: '/players/{id}',
    operation: 'delete',
    capability: 'delete',
  },
  {
    operationId: 'player.get',
    method: 'GET',
    path: '/players/{id}',
    operation: 'get',
    capability: 'item_query',
  },
  {
    operationId: 'player.kick',
    method: 'POST',
    path: '/players/{id}/kick',
    operation: 'kick',
    capability: 'action',
  },
  {
    operationId: 'player.list',
    method: 'GET',
    path: '/players',
    operation: 'list',
    capability: 'collection_query',
  },
  {
    operationId: 'player.update',
    method: 'PUT',
    path: '/players/{id}',
    operation: 'update',
    capability: 'update',
  },
];

async function authenticatedHeaders(request: APIRequestContext): Promise<Record<string, string>> {
  const state = readRealFixtureState();
  const response = await request.post(`${state.serverBaseURL}/api/v1/auth/login`, {
    data: { username: 'admin', password: 'admin123' },
  });
  expect(response.status()).toBe(200);
  const session = (await response.json()) as { token?: string };
  expect(session.token).toMatch(/^eyJ/);
  return {
    Authorization: `Bearer ${session.token}`,
    'X-Game-ID': state.gameId,
    'X-Env': state.env,
  };
}

test.describe('真实 OpenAPI Source 导入与绑定链路', () => {
  test('@openapi-import-bind 经 source/binding 路由导入 /players 并物化合同', async ({
    request,
  }) => {
    const state = readRealFixtureState();
    const headers = await authenticatedHeaders(request);

    // 套件中其他 spec 可能已导入过 players source；先经删除路由回收，保证本测试
    // 验证的是完整“从零导入”链路（删除本身也是被测产品路径 I-009~I-012）。
    await cleanupPlayersSourceBindings(request, headers);

    // 导入前不允许存在预置的 resource:players proposal。
    const beforeProposals = await request.get(`${state.serverBaseURL}/api/v1/proposals`, {
      headers,
    });
    expect(beforeProposals.status()).toBe(200);
    const beforeItems = (await beforeProposals.json()) as ProposalDTO[];
    expect(beforeItems.filter((item) => item.proposalKey === 'resource:players')).toEqual([]);

    // 测试侧拉取 provider 文档（Server 不支持 URL 导入，spec 必须内联提交）。
    const docResponse = await request.get(`${state.providerBaseURL}/openapi.json`);
    expect(docResponse.status()).toBe(200);
    const spec = await docResponse.json();
    expect(spec).toMatchObject({ openapi: '3.0.3', info: { title: 'Players API' } });

    // 经 CreateSource 路由导入文档。
    const createResponse = await request.post(`${state.serverBaseURL}/api/v1/openapi/sources`, {
      headers,
      data: { name: 'players-provider', spec },
    });
    expect(createResponse.status()).toBe(201);
    const created = ((await createResponse.json()) as OpenAPISourceDetail).source;
    expect(created.sourceId).toBeTruthy();
    expect(created.revision).toBe(1);
    expect(created.operationCount).toBe(6);
    expect((created.operations ?? []).map((op) => op.operationId).sort()).toEqual(
      expectedPlayersOperations.map((op) => op.operationId).sort(),
    );
    for (const expected of expectedPlayersOperations) {
      const op = created.operations?.find((item) => item.operationId === expected.operationId);
      expect(op).toMatchObject({
        method: expected.method,
        path: expected.path,
        resource: 'players',
        operation: expected.operation,
        capability: expected.capability,
        bound: false,
      });
    }
    expect(created.diagnostics).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ code: 'rest_capability_inferred', severity: 'info' }),
      ]),
    );

    // 未注册函数不能绑定 provider。
    const rejectedBind = await request.post(
      `${state.serverBaseURL}/api/v1/openapi/sources/${created.sourceId}/bindings`,
      {
        headers,
        data: {
          operationId: 'player.list',
          kind: 'provider',
          functionId: 'players.not-registered',
        },
      },
    );
    expect(rejectedBind.status()).toBe(400);
    const rejectedBody = (await rejectedBind.json()) as { error?: string; message?: string };
    expect(rejectedBody.error).toBeTruthy();
    expect(rejectedBody.message).toBeTruthy();

    // 绑定全部 6 个 operation；collection_query 首个绑定的响应同步返回物化的 proposal。
    const bindOrder = [
      ...expectedPlayersOperations.filter((op) => op.operationId === 'player.list'),
      ...expectedPlayersOperations.filter((op) => op.operationId !== 'player.list'),
    ];
    for (const expected of bindOrder) {
      const bindResponse = await request.post(
        `${state.serverBaseURL}/api/v1/openapi/sources/${created.sourceId}/bindings`,
        {
          headers,
          data: {
            operationId: expected.operationId,
            kind: 'provider',
            functionId: `players.${expected.operationId}`,
          },
        },
      );
      expect(bindResponse.status()).toBe(200);
      const body = (await bindResponse.json()) as {
        binding: { bindingId: string; operationId: string; kind: string; functionId: string };
        proposal?: { proposalKey: string; pageKey: string; pageType: string };
      };
      expect(body.binding).toMatchObject({
        bindingId: expected.operationId,
        operationId: expected.operationId,
        kind: 'provider',
        functionId: `players.${expected.operationId}`,
      });
      if (expected.operationId === 'player.list') {
        expect(body.proposal).toMatchObject({
          proposalKey: 'resource:players',
          pageKey: 'resource--players',
          pageType: 'resource',
        });
      }
    }

    // source 详情反映全部绑定。
    const detailResponse = await request.get(
      `${state.serverBaseURL}/api/v1/openapi/sources/${created.sourceId}`,
      { headers },
    );
    expect(detailResponse.status()).toBe(200);
    const detail = ((await detailResponse.json()) as OpenAPISourceDetail).source;
    expect(detail.operations?.every((op) => op.bound)).toBe(true);
    expect(detail.bindings).toHaveLength(6);
    expect(detail.bindings).toEqual(
      expect.arrayContaining(
        expectedPlayersOperations.map((expected) =>
          expect.objectContaining({
            operationId: expected.operationId,
            kind: 'provider',
            functionId: `players.${expected.operationId}`,
          }),
        ),
      ),
    );

    // 绑定同步物化了 resource:players proposal 与 openapi 来源的合同语义。
    const proposalsResponse = await request.get(`${state.serverBaseURL}/api/v1/proposals`, {
      headers,
    });
    expect(proposalsResponse.status()).toBe(200);
    const proposals = (await proposalsResponse.json()) as ProposalDTO[];
    expect(proposals.filter((item) => item.proposalKey === 'resource:players')).toEqual([
      expect.objectContaining({
        pageKey: 'resource--players',
        pageType: 'resource',
        resourceKey: 'players',
      }),
    ]);

    const catalogResponse = await request.get(
      `${state.serverBaseURL}/api/v1/resource-catalog/players`,
      { headers },
    );
    expect(catalogResponse.status()).toBe(200);
    const catalog = (await catalogResponse.json()) as ResourceCatalogItem;
    expect(catalog).toMatchObject({
      resourceKey: 'players',
      semantics: expect.objectContaining({
        hasIdentity: true,
        hasCollection: true,
        hasCreate: true,
        hasUpdate: true,
        hasDelete: true,
      }),
    });
    expect(catalog.functions).toEqual(
      expect.arrayContaining(
        expectedPlayersOperations.map((expected) =>
          expect.objectContaining({
            functionId: `players.${expected.operationId}`,
            capability: expected.capability,
            source: 'openapi',
          }),
        ),
      ),
    );
  });
});
