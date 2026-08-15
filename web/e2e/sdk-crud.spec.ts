/**
 * 场景 2: SDK CRUD - 资源页面功能
 */

import { test, expect } from '@playwright/test';
import type { APIRequestContext } from '@playwright/test';
import { readRealFixtureState } from './helpers/realFixture';
import {
  login,
  navigateToConsole,
  waitForPageReady,
  waitForTable,
  expectTableVisible,
  expectModalVisible,
} from './helpers';

type SDKFunction = {
  id: string;
  version: string;
  summary: string;
  input_schema: string;
  output_schema: string;
  resource?: string;
  operation?: string;
  capability?: string;
  execution?: string;
  risk?: string;
  enabled: boolean;
};

type ProposalDTO = {
  proposalKey: string;
  pageKey: string;
  pageType: string;
  resourceKey?: string;
  quality: string;
  pageSpec: {
    category?: { key?: string; labels?: Record<string, string> };
    resource?: {
      listView?: {
        identityKey?: string;
        pagination?: { enabled?: boolean };
      };
    };
    bindings?: Array<{ id: string; functionId: string }>;
  };
};

type ResourceCatalogItem = {
  resourceKey: string;
  status: string;
  functions: Array<{ functionId: string; capability: string; source: string }>;
  semantics?: {
    source?: string;
    hasIdentity?: boolean;
    identityField?: string;
    identityFieldType?: string;
    identityPath?: string;
    hasCollection?: boolean;
    hasCreate?: boolean;
    hasUpdate?: boolean;
    hasDelete?: boolean;
  };
  diagnostics?: Array<{ code: string; severity: string }>;
};

const mailFunction: SDKFunction = {
  id: 'mail.send',
  version: '1.0.0',
  summary: 'Send an in-game mail',
  input_schema:
    '{"type":"object","properties":{"player_id":{"type":"string"},"title":{"type":"string"},"content":{"type":"string"}},"required":["player_id","title"]}',
  output_schema:
    '{"type":"object","properties":{"success":{"type":"boolean"},"mail_id":{"type":"string"}}}',
  enabled: true,
};

const inventoryFunctions: SDKFunction[] = [
  {
    id: 'inventory.list',
    version: '1.0.0',
    summary: 'List inventory items',
    resource: 'inventory',
    operation: 'list',
    capability: 'collection_query',
    execution: 'sync',
    risk: 'safe',
    input_schema:
      '{"type":"object","properties":{"page":{"type":"integer"},"page_size":{"type":"integer"}}}',
    output_schema:
      '{"type":"object","properties":{"items":{"type":"array","items":{"type":"object","properties":{"id":{"type":"string"},"name":{"type":"string"},"quantity":{"type":"integer"}},"required":["id","name"]}},"total":{"type":"integer"}},"required":["items","total"]}',
    enabled: true,
  },
  {
    id: 'inventory.get',
    version: '1.0.0',
    summary: 'Get inventory item',
    resource: 'inventory',
    operation: 'get',
    capability: 'item_query',
    execution: 'sync',
    risk: 'safe',
    input_schema: '{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}',
    output_schema:
      '{"type":"object","properties":{"id":{"type":"string"},"name":{"type":"string"},"quantity":{"type":"integer"}},"required":["id","name"]}',
    enabled: true,
  },
  {
    id: 'inventory.create',
    version: '1.0.0',
    summary: 'Create inventory item',
    resource: 'inventory',
    operation: 'create',
    capability: 'create',
    execution: 'sync',
    risk: 'safe',
    input_schema:
      '{"type":"object","properties":{"name":{"type":"string"},"quantity":{"type":"integer"}},"required":["name"]}',
    output_schema:
      '{"type":"object","properties":{"id":{"type":"string"},"name":{"type":"string"},"quantity":{"type":"integer"}},"required":["id","name"]}',
    enabled: true,
  },
  {
    id: 'inventory.update',
    version: '1.0.0',
    summary: 'Update inventory item',
    resource: 'inventory',
    operation: 'update',
    capability: 'update',
    execution: 'sync',
    risk: 'warning',
    input_schema:
      '{"type":"object","properties":{"id":{"type":"string"},"name":{"type":"string"},"quantity":{"type":"integer"}},"required":["id"]}',
    output_schema:
      '{"type":"object","properties":{"id":{"type":"string"},"name":{"type":"string"},"quantity":{"type":"integer"}},"required":["id","name"]}',
    enabled: true,
  },
  {
    id: 'inventory.delete',
    version: '1.0.0',
    summary: 'Delete inventory item',
    resource: 'inventory',
    operation: 'delete',
    capability: 'delete',
    execution: 'sync',
    risk: 'high',
    input_schema: '{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}',
    output_schema: '{"type":"object","properties":{"success":{"type":"boolean"}}}',
    enabled: true,
  },
];

const missingIdentityFunction: SDKFunction = {
  id: 'profile.list',
  version: '1.0.0',
  summary: 'List profiles without an identity contract',
  resource: 'profile',
  operation: 'list',
  capability: 'collection_query',
  execution: 'sync',
  risk: 'safe',
  input_schema: '{"type":"object"}',
  output_schema:
    '{"type":"object","properties":{"items":{"type":"array","items":{"type":"object","properties":{"display_name":{"type":"string"},"level":{"type":"integer"}}}},"total":{"type":"integer"}}}',
  enabled: true,
};

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

async function waitForSDKProposal(
  request: APIRequestContext,
  headers: Record<string, string>,
): Promise<ProposalDTO[]> {
  const state = readRealFixtureState();
  await expect
    .poll(
      async () => {
        const response = await request.get(`${state.serverBaseURL}/api/v1/proposals`, {
          headers,
        });
        if (response.status() !== 200) return [];
        return (await response.json()) as ProposalDTO[];
      },
      { timeout: 30000 },
    )
    .toEqual(
      expect.arrayContaining([
        expect.objectContaining({ proposalKey: 'resource:inventory' }),
        expect.objectContaining({ proposalKey: 'operation:profile.list' }),
      ]),
    );

  const response = await request.get(`${state.serverBaseURL}/api/v1/proposals`, { headers });
  expect(response.status()).toBe(200);
  return (await response.json()) as ProposalDTO[];
}

test.describe('SDK CRUD', () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
  });

  test('背包资源列表页加载', async ({ page }) => {
    await navigateToConsole(page, 'inventory', 'resource--inventory');
    await waitForPageReady(page);
    await waitForTable(page);

    // 验证 ProTable 渲染
    await expectTableVisible(page);
  });

  test('背包资源 CRUD 操作', async ({ page }) => {
    await navigateToConsole(page, 'inventory', 'resource--inventory');
    await waitForPageReady(page);
    await waitForTable(page);

    // SDK CRUD 页面声明了 create binding 时，新建入口必须真实存在。
    const createBtn = page.locator('button:has-text("新建"), button:has-text("创建")').first();
    await expect(createBtn).toBeVisible();
    await createBtn.click();

    await expectModalVisible(page);
    await expect(page.getByRole('dialog').getByText('新建', { exact: true })).toBeVisible();

    const cancelBtn = page.getByRole('dialog').getByRole('button', { name: '取消' });
    await expect(cancelBtn).toBeVisible();
    await cancelBtn.click();
    await expect(page.getByRole('dialog')).toBeHidden();
  });
});

test.describe('真实 SDK Resource Proposal 链路', () => {
  test('@sdk-explicit-resource-proposal 显式 capability 仅在 identity 可验证时生成资源页', async ({
    request,
  }) => {
    const state = readRealFixtureState();
    const headers = await authenticatedHeaders(request);
    const replace = await request.post(`${state.fixtureBaseURL}/__fixture__/sdk/functions`, {
      data: {
        functions: [mailFunction, ...inventoryFunctions, missingIdentityFunction],
      },
    });
    expect(replace.status()).toBe(200);

    const proposals = await waitForSDKProposal(request, headers);
    const inventory = proposals.filter((item) => item.proposalKey === 'resource:inventory');
    expect(inventory).toHaveLength(1);
    expect(inventory[0]).toMatchObject({
      pageKey: 'resource--inventory',
      pageType: 'resource',
      resourceKey: 'inventory',
      quality: 'ready',
      pageSpec: {
        category: {
          key: 'inventory',
          // 术语字典（term_dictionary）把 inventory 本地化为「道具」。
          labels: expect.objectContaining({ 'zh-CN': '道具', 'en-US': 'Item' }),
        },
        resource: {
          listView: {
            identityKey: 'id',
            pagination: { enabled: true },
          },
        },
      },
    });
    expect(inventory[0].pageSpec.bindings).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ id: 'list', functionId: 'inventory.list' }),
        expect.objectContaining({ id: 'detail', functionId: 'inventory.get' }),
        expect.objectContaining({ id: 'create', functionId: 'inventory.create' }),
        expect.objectContaining({ id: 'update', functionId: 'inventory.update' }),
        expect.objectContaining({ id: 'delete', functionId: 'inventory.delete' }),
      ]),
    );
    expect(
      proposals.some((item) =>
        [
          'operation:inventory.list',
          'operation:inventory.get',
          'operation:inventory.create',
          'operation:inventory.update',
          'operation:inventory.delete',
        ].includes(item.proposalKey),
      ),
    ).toBe(false);

    expect(proposals.some((item) => item.proposalKey === 'resource:profile')).toBe(false);
    expect(proposals.filter((item) => item.proposalKey === 'operation:profile.list')).toEqual([
      expect.objectContaining({
        pageKey: 'operation--profile.list',
        pageType: 'operation',
        resourceKey: 'profile',
        quality: 'basic',
      }),
    ]);

    const inventoryCatalogResponse = await request.get(
      `${state.serverBaseURL}/api/v1/resource-catalog/inventory`,
      { headers },
    );
    expect(inventoryCatalogResponse.status()).toBe(200);
    const inventoryCatalog = (await inventoryCatalogResponse.json()) as ResourceCatalogItem;
    expect(inventoryCatalog).toMatchObject({
      resourceKey: 'inventory',
      status: 'identified',
      semantics: {
        source: 'sdk_explicit',
        hasIdentity: true,
        identityField: 'id',
        identityFieldType: 'string',
        identityPath: '/id',
        hasCollection: true,
        hasCreate: true,
        hasUpdate: true,
        hasDelete: true,
      },
    });
    expect(inventoryCatalog.functions).toHaveLength(5);
    expect(inventoryCatalog.functions).toEqual(
      expect.arrayContaining(
        inventoryFunctions.map((fn) =>
          expect.objectContaining({
            functionId: fn.id,
            capability: fn.capability,
            source: 'sdk',
          }),
        ),
      ),
    );

    const profileCatalogResponse = await request.get(
      `${state.serverBaseURL}/api/v1/resource-catalog/profile`,
      { headers },
    );
    expect(profileCatalogResponse.status()).toBe(200);
    const profileCatalog = (await profileCatalogResponse.json()) as ResourceCatalogItem;
    expect(profileCatalog).toMatchObject({
      resourceKey: 'profile',
      status: 'pending',
      semantics: {
        source: 'sdk_explicit',
        hasIdentity: false,
        hasCollection: true,
      },
    });
    expect(profileCatalog.functions).toEqual([
      expect.objectContaining({
        functionId: 'profile.list',
        capability: 'collection_query',
        source: 'sdk',
      }),
    ]);
    expect(profileCatalog.diagnostics).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          code: 'resource_identity_not_verifiable',
          severity: 'warning',
        }),
      ]),
    );
  });
});
