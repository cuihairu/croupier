/**
 * 真实 OpenAPI /players source 导入与绑定 helper。
 *
 * 供 openapi-source.spec.ts（完整链路断言）与 openapi-crud.spec.ts
 * （消费侧就绪前置）复用。ensurePlayersSourceBound 幂等：scope 中已存在
 * 全部绑定的 players source 时直接复用，不重复导入。
 */

import type { APIRequestContext } from '@playwright/test';
import { expect } from '@playwright/test';
import { readRealFixtureState } from './realFixture';

export type OpenAPIPlayersOperation = {
  operationId: string;
  method: string;
  path: string;
  operation: string;
  capability: string;
};

export const playersOperations: OpenAPIPlayersOperation[] = [
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

export async function openapiAuthenticatedHeaders(
  request: APIRequestContext,
): Promise<Record<string, string>> {
  const state = readRealFixtureState();
  const response = await request.post(`${state.serverBaseURL}/api/v1/auth/login`, {
    data: { username: 'admin', password: 'admin123' },
  });
  expect(response.status()).toBe(200);
  const session = (await response.json()) as { token?: string };
  expect(session.token).toMatch(/^EyJ/i);
  return {
    Authorization: `Bearer ${session.token}`,
    'X-Game-ID': state.gameId,
    'X-Env': state.env,
  };
}

export async function importAndBindPlayersSource(
  request: APIRequestContext,
  headers: Record<string, string>,
): Promise<string> {
  const state = readRealFixtureState();

  const docResponse = await request.get(`${state.providerBaseURL}/openapi.json`);
  expect(docResponse.status()).toBe(200);
  const spec = await docResponse.json();

  const createResponse = await request.post(`${state.serverBaseURL}/api/v1/openapi/sources`, {
    headers,
    data: { name: 'players-provider', spec },
  });
  expect(createResponse.status()).toBe(201);
  const sourceId = ((await createResponse.json()) as { source: { sourceId: string } }).source
    .sourceId;
  expect(sourceId).toBeTruthy();

  const bindOrder = [
    ...playersOperations.filter((op) => op.operationId === 'player.list'),
    ...playersOperations.filter((op) => op.operationId !== 'player.list'),
  ];
  for (const expected of bindOrder) {
    const bindResponse = await request.post(
      `${state.serverBaseURL}/api/v1/openapi/sources/${sourceId}/bindings`,
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
  }
  return sourceId;
}

type SourceSummary = {
  sourceId: string;
  name: string;
};

type OpenAPISourceDetail = {
  source: {
    sourceId: string;
    bindings?: Array<{ bindingId: string }>;
  };
};

type ProposalListItem = {
  proposalKey: string;
  pageSpec?: {
    bindings?: Array<{ functionId: string }>;
  };
};

export async function ensurePlayersSourceBound(
  request: APIRequestContext,
  headers: Record<string, string>,
): Promise<string> {
  const state = readRealFixtureState();
  const listResponse = await request.get(`${state.serverBaseURL}/api/v1/openapi/sources`, {
    headers,
  });
  expect(listResponse.status()).toBe(200);
  const body = (await listResponse.json()) as { items?: SourceSummary[] } | SourceSummary[];
  const items = Array.isArray(body) ? body : (body.items ?? []);
  const candidates = items.filter((item) => item.name === 'players-provider');
  if (candidates.length > 0) {
    const proposalsResponse = await request.get(`${state.serverBaseURL}/api/v1/proposals`, {
      headers,
    });
    expect(proposalsResponse.status()).toBe(200);
    const proposals = (await proposalsResponse.json()) as ProposalListItem[];
    const resource = proposals.filter((item) => item.proposalKey === 'resource:players');
    const boundFunctionIds = new Set(
      resource.flatMap((item) =>
        (item.pageSpec?.bindings ?? []).map((binding) => binding.functionId),
      ),
    );
    const allBound = playersOperations.every((op) =>
      boundFunctionIds.has(`players.${op.operationId}`),
    );
    if (allBound && resource.length > 0) {
      return candidates[0].sourceId;
    }
  }
  return importAndBindPlayersSource(request, headers);
}

export async function ensurePlayersResourcePublished(
  request: APIRequestContext,
  headers: Record<string, string>,
): Promise<void> {
  const state = readRealFixtureState();
  const pageURL = `${state.serverBaseURL}/api/v1/console/pages/resource--players`;
  const current = await request.get(pageURL, { headers });
  if (current.status() === 200) {
    return;
  }
  expect(current.status()).toBe(404);
  const publish = await request.post(
    `${state.serverBaseURL}/api/v1/proposals/resource%3Aplayers/accept-and-publish`,
    { headers },
  );
  expect(publish.status()).toBe(200);
  const published = await request.get(pageURL, { headers });
  expect(published.status()).toBe(200);
}

/**
 * 删除当前 scope 内所有 players-provider source 的 provider binding，使
 * resource:players 派生对象回收。用于让“从零导入”场景可重复执行。
 */
export async function cleanupPlayersSourceBindings(
  request: APIRequestContext,
  headers: Record<string, string>,
): Promise<void> {
  const state = readRealFixtureState();
  const listResponse = await request.get(`${state.serverBaseURL}/api/v1/openapi/sources`, {
    headers,
  });
  expect(listResponse.status()).toBe(200);
  const body = (await listResponse.json()) as { items?: SourceSummary[] } | SourceSummary[];
  const items = Array.isArray(body) ? body : (body.items ?? []);
  for (const source of items.filter((item) => item.name === 'players-provider')) {
    const detailResponse = await request.get(
      `${state.serverBaseURL}/api/v1/openapi/sources/${source.sourceId}`,
      { headers },
    );
    if (detailResponse.status() !== 200) continue;
    const detail = ((await detailResponse.json()) as OpenAPISourceDetail).source;
    for (const binding of detail.bindings ?? []) {
      const deleteResponse = await request.delete(
        `${state.serverBaseURL}/api/v1/openapi/sources/${source.sourceId}/bindings/${binding.bindingId}`,
        { headers },
      );
      expect([200, 204]).toContain(deleteResponse.status());
    }
  }
  // 等待派生 proposal 回收完成。
  await expect
    .poll(
      async () => {
        const proposalsResponse = await request.get(`${state.serverBaseURL}/api/v1/proposals`, {
          headers,
        });
        if (proposalsResponse.status() !== 200) return -1;
        const proposals = (await proposalsResponse.json()) as ProposalListItem[];
        return proposals.filter((item) => item.proposalKey === 'resource:players').length;
      },
      { timeout: 30000 },
    )
    .toBe(0);
}
