/**
 * 场景 8: 契约变化 - Stale 页面处理
 */

import { test, expect } from '@playwright/test';
import type { APIRequestContext } from '@playwright/test';
import { readRealFixtureState } from './helpers/realFixture';
import {
  login,
  navigateToConsole,
  waitForPageReady,
  waitForTable,
  expectTableHasRows,
} from './helpers';

type FreshnessEntry = {
  bindingId: string;
  status: string;
  diagnostic: { code: string };
};

type ConsolePageDTO = {
  page?: {
    pageKey: string;
    version?: number;
    bindingContracts?: Array<{
      bindingId: string;
      inputSchemaDigest?: string;
      risk?: string;
    }>;
    bindingFreshness?: FreshnessEntry[];
  };
};

const defaultMailSend = {
  id: 'mail.send',
  version: '1.0.0',
  summary: 'Send an in-game mail',
  input_schema:
    '{"type":"object","properties":{"player_id":{"type":"string"},"title":{"type":"string"},"content":{"type":"string"}},"required":["player_id","title"]}',
  output_schema:
    '{"type":"object","properties":{"success":{"type":"boolean"},"mail_id":{"type":"string"}}}',
  enabled: true,
};

// 契约变化链路使用独立的 ops.restart 函数，避免污染 mail.send 的跨 spec 状态。
const defaultOpsRestart = {
  id: 'ops.restart',
  version: '1.0.0',
  summary: 'Restart game server',
  input_schema:
    '{"type":"object","properties":{"region":{"type":"string"},"reason":{"type":"string"}},"required":["region"]}',
  output_schema:
    '{"type":"object","properties":{"success":{"type":"boolean"},"job_id":{"type":"string"}}}',
  enabled: true,
};

async function authenticatedHeaders(request: APIRequestContext): Promise<Record<string, string>> {
  const state = readRealFixtureState();
  const response = await request.post(`${state.serverBaseURL}/api/v1/auth/login`, {
    data: { username: 'admin', password: 'admin123' },
  });
  expect(response.status()).toBe(200);
  const session = (await response.json()) as { token?: string };
  expect(session.token).toBeTruthy();
  return {
    Authorization: `Bearer ${session.token}`,
    'X-Game-ID': state.gameId,
    'X-Env': state.env,
  };
}

async function ensureOpsPublished(request: APIRequestContext, headers: Record<string, string>) {
  const state = readRealFixtureState();

  // 注册默认函数集（mail.send 保持原样 + 本链路专用的 ops.restart），并等待
  // un-annotated ops.restart 降级为 standalone Operation Proposal。
  await replaceSDKFunctions(request, [defaultMailSend, defaultOpsRestart]);
  await expect
    .poll(
      async () => {
        const response = await request.get(`${state.serverBaseURL}/api/v1/proposals`, {
          headers,
        });
        if (response.status() !== 200) return [];
        const proposals = (await response.json()) as Array<{ proposalKey?: string }>;
        return proposals.filter((item) => item.proposalKey === 'operation:ops.restart');
      },
      { timeout: 30000 },
    )
    .toHaveLength(1);

  const pageURL = `${state.serverBaseURL}/api/v1/console/pages/operation--ops.restart`;
  const current = await request.get(pageURL, { headers });
  if (current.status() === 200) return;
  expect(current.status()).toBe(404);
  const publish = await request.post(
    `${state.serverBaseURL}/api/v1/proposals/operation%3Aops.restart/accept-and-publish`,
    { headers },
  );
  expect(publish.status()).toBe(200);
}

async function replaceSDKFunctions(
  request: APIRequestContext,
  functions: Array<Record<string, unknown>>,
): Promise<void> {
  const state = readRealFixtureState();
  const replace = await request.post(`${state.fixtureBaseURL}/__fixture__/sdk/functions`, {
    data: { functions },
  });
  expect(replace.status()).toBe(200);
}

async function fetchOpsConsolePage(
  request: APIRequestContext,
  headers: Record<string, string>,
): Promise<ConsolePageDTO> {
  const state = readRealFixtureState();
  const response = await request.get(
    `${state.serverBaseURL}/api/v1/console/pages/operation--ops.restart`,
    { headers },
  );
  expect(response.status()).toBe(200);
  return (await response.json()) as ConsolePageDTO;
}

async function restoreFixtureFunctions(request: APIRequestContext): Promise<void> {
  await replaceSDKFunctions(request, [defaultMailSend, defaultOpsRestart]);
}

type OpsDraftDTO = {
  title?: unknown;
  bindings?: unknown[];
  operation?: unknown;
  draftRevision?: number;
  publishedVersion?: number;
};

async function fetchOpsDraft(
  request: APIRequestContext,
  headers: Record<string, string>,
): Promise<OpsDraftDTO> {
  const state = readRealFixtureState();
  const response = await request.get(`${state.serverBaseURL}/api/v1/pages/operation--ops.restart`, {
    headers,
  });
  expect(response.status()).toBe(200);
  return (await response.json()) as OpsDraftDTO;
}

type OpsDiffDTO = {
  changes?: Array<{ field: string; isSemantic?: boolean }>;
  autoMergeItems?: Array<{ field: string; mergedValue?: unknown }>;
  conflictItems?: Array<{ field: string }>;
};

async function fetchOpsDiff(
  request: APIRequestContext,
  headers: Record<string, string>,
): Promise<OpsDiffDTO> {
  const state = readRealFixtureState();
  const response = await request.get(
    `${state.serverBaseURL}/api/v1/versioning/pages/operation--ops.restart/diff`,
    { headers },
  );
  expect(response.status()).toBe(200);
  return (await response.json()) as OpsDiffDTO;
}

test.describe('契约变化', () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
  });

  test('Stale 页面警告展示', async ({ page }) => {
    await navigateToConsole(page, 'players', 'resource--players');
    await waitForPageReady(page);

    const staleAlert = page.getByText('页面绑定的函数契约已变化，执行已被阻断').first();
    // 当前 fixture 初始合同必须是 fresh；stale 场景由命名变更测试显式触发。
    await expect(staleAlert).toHaveCount(0);
    await waitForTable(page);
    await expectTableHasRows(page);
  });

  test('正常页面无 stale 警告', async ({ page }) => {
    await navigateToConsole(page, 'players', 'resource--players');
    await waitForPageReady(page);

    await waitForTable(page);
    await expect(page.locator('.ant-pro-table, .ant-table').first()).toBeVisible();
    await expectTableHasRows(page);
    await expect(page.getByText('玩家A')).toBeVisible();
    await expect(page.locator('.ant-result-error, text=加载失败')).toHaveCount(0);
  });
});

test.describe('真实契约变化链路', () => {
  test.afterEach(async ({ request }) => {
    // 恢复 fixture 默认函数集后，通过产品治理路径把 ops.restart 页面带回
    // fresh：draft 同步到最新 proposal（manual merge 接受全部冲突），若已发布
    // 快照 stale 则重发布。保证后续测试/spec 在干净状态上运行。
    const state = readRealFixtureState();
    const headers = await authenticatedHeaders(request);
    await restoreFixtureFunctions(request);

    // 等待默认 proposal 重建完成。
    await expect
      .poll(
        async () => {
          const response = await request.get(`${state.serverBaseURL}/api/v1/proposals`, {
            headers,
          });
          if (response.status() !== 200) return '';
          const proposals = (await response.json()) as Array<{ proposalKey?: string }>;
          const ops = proposals.find((item) => item.proposalKey === 'operation:ops.restart');
          return ops ? JSON.stringify(ops) : '';
        },
        { timeout: 30000 },
      )
      .not.toContain('priority');

    // 同步 draft 到最新 proposal。
    const diff = await fetchOpsDiff(request, headers);
    if ((diff.conflictItems ?? []).length > 0) {
      const draft = await fetchOpsDraft(request, headers);
      const merge = await request.post(
        `${state.serverBaseURL}/api/v1/versioning/pages/operation--ops.restart/merge`,
        {
          headers,
          data: {
            expectedDraftRevision: draft.draftRevision ?? 0,
            strategy: 'manual',
            reason: 'reset to default contract after contract-change spec',
            conflicts: (diff.conflictItems ?? []).map((item) => ({
              path: item.field,
              acceptNew: true,
            })),
          },
        },
      );
      expect(merge.status()).toBe(200);
    }

    // 已发布快照若仍 stale，重发布恢复 fresh。
    const consolePage = await fetchOpsConsolePage(request, headers);
    if ((consolePage.page?.bindingFreshness ?? []).length > 0) {
      const draft = await fetchOpsDraft(request, headers);
      const publish = await request.post(
        `${state.serverBaseURL}/api/v1/pages/operation--ops.restart/publish`,
        { headers, data: { draftRevision: draft.draftRevision ?? 0 } },
      );
      expect(publish.status()).toBe(200);
    }

    await expect
      .poll(
        async () => {
          const current = await fetchOpsConsolePage(request, headers);
          return (current.page?.bindingFreshness ?? []).length;
        },
        { timeout: 30000 },
      )
      .toBe(0);
  });

  test('@schema-change-stale 重注册 schema 变化使页面进入 stale', async ({ request }) => {
    const headers = await authenticatedHeaders(request);
    await ensureOpsPublished(request, headers);

    const before = await fetchOpsConsolePage(request, headers);
    const snapshotBefore = before.page?.bindingContracts?.[0];
    expect(snapshotBefore?.bindingId).toBe('ops.restart.main');
    const frozenDigest = snapshotBefore?.inputSchemaDigest;
    expect(frozenDigest).toMatch(/^[a-f0-9]{64}$/);
    expect(before.page?.bindingFreshness ?? []).toEqual([]);

    // 真实 Agent 重注册：input schema 新增 required 字段。
    await replaceSDKFunctions(request, [
      defaultMailSend,
      {
        ...defaultOpsRestart,
        input_schema:
          '{"type":"object","properties":{"player_id":{"type":"string"},"title":{"type":"string"},"content":{"type":"string"},"priority":{"type":"integer"}},"required":["player_id","title","priority"]}',
      },
    ]);

    // Server 检测到 schema 变化并产出 input_schema_stale 诊断。
    let stale: FreshnessEntry | undefined;
    await expect
      .poll(
        async () => {
          const current = await fetchOpsConsolePage(request, headers);
          stale = current.page?.bindingFreshness?.find(
            (item) => item.status === 'input_schema_stale',
          );
          return stale !== undefined;
        },
        { timeout: 30000 },
      )
      .toBe(true);
    expect(stale).toMatchObject({
      bindingId: 'ops.restart.main',
      status: 'input_schema_stale',
      diagnostic: { code: 'binding_input_schema_stale' },
    });

    // 重注册生成新 proposal（含新字段），但不得覆盖 PublishedPageSpec 冻结快照。
    const state = readRealFixtureState();
    const proposalsResponse = await request.get(`${state.serverBaseURL}/api/v1/proposals`, {
      headers,
    });
    expect(proposalsResponse.status()).toBe(200);
    const proposals = (await proposalsResponse.json()) as unknown[];
    const mailProposal = proposals.find(
      (item) => (item as { proposalKey?: string }).proposalKey === 'operation:ops.restart',
    );
    expect(mailProposal).toBeTruthy();
    expect(JSON.stringify(mailProposal)).toContain('priority');

    const after = await fetchOpsConsolePage(request, headers);
    expect(after.page?.bindingContracts?.[0]?.inputSchemaDigest).toBe(frozenDigest);

    // 恢复默认 schema 后页面回到 fresh。
    await restoreFixtureFunctions(request);
    await expect
      .poll(
        async () => {
          const current = await fetchOpsConsolePage(request, headers);
          return (current.page?.bindingFreshness ?? []).length;
        },
        { timeout: 30000 },
      )
      .toBe(0);
  });

  test('@governance-change-stale 真实 risk 变化使页面进入 governance stale', async ({
    request,
  }) => {
    const headers = await authenticatedHeaders(request);
    await ensureOpsPublished(request, headers);

    const before = await fetchOpsConsolePage(request, headers);
    expect(before.page?.bindingFreshness ?? []).toEqual([]);

    // 真实 Agent 重注册：risk 提升为 high。
    await replaceSDKFunctions(request, [defaultMailSend, { ...defaultOpsRestart, risk: 'high' }]);

    await expect
      .poll(
        async () => {
          const current = await fetchOpsConsolePage(request, headers);
          return (
            current.page?.bindingFreshness?.find((item) => item.status === 'governance_stale') !==
            undefined
          );
        },
        { timeout: 30000 },
      )
      .toBe(true);

    const stalePage = await fetchOpsConsolePage(request, headers);
    const governanceStale = stalePage.page?.bindingFreshness?.find(
      (item) => item.status === 'governance_stale',
    );
    expect(governanceStale).toMatchObject({
      bindingId: 'ops.restart.main',
      status: 'governance_stale',
      diagnostic: { code: 'binding_governance_stale' },
    });

    // 治理快照不静默升级：发布快照仍保留旧 risk。
    expect(stalePage.page?.bindingContracts?.[0]?.risk ?? '').not.toBe('high');

    // 恢复默认 risk 后页面回到 fresh。
    await restoreFixtureFunctions(request);
    await expect
      .poll(
        async () => {
          const current = await fetchOpsConsolePage(request, headers);
          return (current.page?.bindingFreshness ?? []).length;
        },
        { timeout: 30000 },
      )
      .toBe(0);
  });

  test('@stale-execute-rejected stale 页执行被拒绝且保留处理入口', async ({ request }) => {
    const state = readRealFixtureState();
    const headers = await authenticatedHeaders(request);
    await ensureOpsPublished(request, headers);

    await replaceSDKFunctions(request, [
      defaultMailSend,
      {
        ...defaultOpsRestart,
        input_schema:
          '{"type":"object","properties":{"player_id":{"type":"string"},"title":{"type":"string"},"content":{"type":"string"},"priority":{"type":"integer"}},"required":["player_id","title","priority"]}',
      },
    ]);
    await expect
      .poll(
        async () => {
          const current = await fetchOpsConsolePage(request, headers);
          return (current.page?.bindingFreshness ?? []).length > 0;
        },
        { timeout: 30000 },
      )
      .toBe(true);

    // Agent 调用计数在执行尝试前后都必须保持不变。
    const clear = await request.delete(`${state.fixtureBaseURL}/__fixture__/sdk/calls`);
    expect(clear.status()).toBe(200);

    const executeResponse = await request.post(
      `${state.serverBaseURL}/api/v1/console/pages/operation--ops.restart/bindings/ops.restart.main/execute`,
      {
        headers,
        data: { context: { form: { player_id: 'p-001', title: 'stale', priority: 1 } } },
      },
    );
    // 明确 stale 拒绝（409 + 结构化错误），不是泛化 500，也不是静默执行最新合同。
    expect(executeResponse.status()).toBe(409);
    const errorBody = (await executeResponse.json()) as {
      error?: string;
      details?: { bindingId?: string; functionId?: string; statuses?: string[] };
    };
    expect(errorBody.error).toBe('binding_stale');
    expect(errorBody.details).toMatchObject({
      bindingId: 'ops.restart.main',
      functionId: 'ops.restart',
    });
    expect(errorBody.details?.statuses).toContain('input_schema_stale');

    // Agent 从未被调用。
    const callsResponse = await request.get(`${state.fixtureBaseURL}/__fixture__/sdk/calls`);
    expect(callsResponse.status()).toBe(200);
    const calls = (await callsResponse.json()) as { calls?: unknown[] };
    expect(calls.calls ?? []).toEqual([]);

    // Inbox 提供契约变更处理入口。
    const inboxResponse = await request.get(`${state.serverBaseURL}/api/v1/proposals/inbox`, {
      headers,
    });
    expect(inboxResponse.status()).toBe(200);
    const inbox = (await inboxResponse.json()) as {
      contractChanges?: Array<{
        pageKey: string;
        kind: string;
        bindingFreshness?: FreshnessEntry[];
      }>;
    };
    const mailChanges = inbox.contractChanges?.filter(
      (item) => item.pageKey === 'operation--ops.restart',
    );
    expect(mailChanges ?? []).not.toEqual([]);
    expect(
      mailChanges?.some((item) =>
        (item.bindingFreshness ?? []).some(
          (entry) =>
            entry.bindingId === 'ops.restart.main' && entry.status === 'input_schema_stale',
        ),
      ),
    ).toBe(true);

    // 恢复默认 schema 后页面回到 fresh。
    await restoreFixtureFunctions(request);
    await expect
      .poll(
        async () => {
          const current = await fetchOpsConsolePage(request, headers);
          return (current.page?.bindingFreshness ?? []).length;
        },
        { timeout: 30000 },
      )
      .toBe(0);
  });

  test('@safe-auto-merge 展示类变化自动合并且不改执行字段', async ({ request }) => {
    const state = readRealFixtureState();
    const headers = await authenticatedHeaders(request);
    await ensureOpsPublished(request, headers);

    const draftBefore = await fetchOpsDraft(request, headers);
    const bindingsBefore = JSON.stringify(draftBefore.bindings ?? []);
    const revisionBefore = draftBefore.draftRevision ?? 0;
    expect(revisionBefore).toBeGreaterThan(0);

    // 真实 Agent 重注册：仅改 summary（展示层输入），schema/risk/permission 不变。
    await replaceSDKFunctions(request, [
      defaultMailSend,
      { ...defaultOpsRestart, summary: 'Restart a premium game server' },
    ]);

    // 等待新 proposal 反映新文案。
    await expect
      .poll(
        async () => {
          const response = await request.get(`${state.serverBaseURL}/api/v1/proposals`, {
            headers,
          });
          if (response.status() !== 200) return '';
          const proposals = (await response.json()) as Array<{ proposalKey?: string }>;
          const mail = proposals.find((item) => item.proposalKey === 'operation:ops.restart');
          return mail ? JSON.stringify(mail) : '';
        },
        { timeout: 30000 },
      )
      .toContain('premium');

    // diff 显示展示字段进入自动合并集，且没有执行类冲突。
    const diff = await fetchOpsDiff(request, headers);
    expect(diff.conflictItems ?? []).toEqual([]);
    expect(diff.autoMergeItems ?? []).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          field: 'title',
          mergedValue: expect.objectContaining({
            'zh-CN': expect.stringContaining('premium'),
          }),
        }),
      ]),
    );

    // Page Studio 调用的 auto 合并 API：安全字段落地，无冲突。
    const mergeResponse = await request.post(
      `${state.serverBaseURL}/api/v1/versioning/pages/operation--ops.restart/merge`,
      { headers, data: { expectedDraftRevision: revisionBefore, strategy: 'auto' } },
    );
    if (mergeResponse.status() !== 200) {
      console.log('merge error body:', await mergeResponse.text());
    }
    expect(mergeResponse.status()).toBe(200);
    const mergeBody = (await mergeResponse.json()) as {
      merged: number;
      conflicts: number;
      draftRevision: number;
    };
    expect(mergeBody.merged).toBeGreaterThanOrEqual(1);
    expect(mergeBody.conflicts).toBe(0);
    expect(mergeBody.draftRevision).toBe(revisionBefore + 1);

    // 展示字段更新，执行字段（bindings/selectors）未被改写。
    const draftAfter = await fetchOpsDraft(request, headers);
    expect(JSON.stringify(draftAfter.title ?? {})).toContain('premium');
    expect(JSON.stringify(draftAfter.bindings ?? [])).toBe(bindingsBefore);

    // 合同 digest 未变：已发布执行快照边界不受影响。
    const consolePage = await fetchOpsConsolePage(request, headers);
    expect(consolePage.page?.bindingFreshness ?? []).toEqual([]);
  });

  test('@identity-conflict-manual schema 变化要求人工冲突决策', async ({ request }) => {
    const state = readRealFixtureState();
    const headers = await authenticatedHeaders(request);
    await ensureOpsPublished(request, headers);

    const draftBefore = await fetchOpsDraft(request, headers);
    const revisionBefore = draftBefore.draftRevision ?? 0;
    expect(revisionBefore).toBeGreaterThan(0);

    await replaceSDKFunctions(request, [
      defaultMailSend,
      {
        ...defaultOpsRestart,
        input_schema:
          '{"type":"object","properties":{"player_id":{"type":"string"},"title":{"type":"string"},"content":{"type":"string"},"priority":{"type":"integer"}},"required":["player_id","title","priority"]}',
      },
    ]);
    await expect
      .poll(
        async () => {
          const current = await fetchOpsConsolePage(request, headers);
          return (current.page?.bindingFreshness ?? []).length > 0;
        },
        { timeout: 30000 },
      )
      .toBe(true);

    // identity/binding 执行字段变化出现在人工冲突列表。
    const diff = await fetchOpsDiff(request, headers);
    const conflictFields = (diff.conflictItems ?? []).map((item) => item.field);
    expect(conflictFields.length).toBeGreaterThan(0);
    expect(conflictFields.some((field) => /operation\.form\.jsonSchema|bindings/.test(field))).toBe(
      true,
    );

    // auto 合并只能落安全字段，冲突必须人工决策。
    const autoMerge = await request.post(
      `${state.serverBaseURL}/api/v1/versioning/pages/operation--ops.restart/merge`,
      { headers, data: { expectedDraftRevision: revisionBefore, strategy: 'auto' } },
    );
    expect(autoMerge.status()).toBe(200);
    const autoBody = (await autoMerge.json()) as {
      merged: number;
      conflicts: number;
      draftRevision: number;
    };
    expect(autoBody.conflicts).toBeGreaterThanOrEqual(1);
    const manualRevision = Math.max(autoBody.draftRevision ?? 0, revisionBefore);

    // 未解决冲突前发布被拒绝（selector/合同校验失败），而非静默发布。
    const publishRejected = await request.post(
      `${state.serverBaseURL}/api/v1/pages/operation--ops.restart/publish`,
      { headers, data: { draftRevision: manualRevision } },
    );
    if (![409, 422].includes(publishRejected.status())) {
      console.log(
        'publish reject:',
        await publishRejected.text(),
        'revisionBefore=',
        revisionBefore,
        'autoBody=',
        JSON.stringify(autoBody),
        'manualRevision=',
        manualRevision,
      );
    }
    expect([409, 422]).toContain(publishRejected.status());
    expect(publishRejected.status()).not.toBe(200);

    // 人工决策：逐冲突接受最新合同。
    const manualMerge = await request.post(
      `${state.serverBaseURL}/api/v1/versioning/pages/operation--ops.restart/merge`,
      {
        headers,
        data: {
          expectedDraftRevision: manualRevision,
          strategy: 'manual',
          reason: 'accept new required field',
          conflicts: (diff.conflictItems ?? []).map((item) => ({
            path: item.field,
            acceptNew: true,
          })),
        },
      },
    );
    expect(manualMerge.status()).toBe(200);
    const manualBody = (await manualMerge.json()) as {
      merged: number;
      conflicts: number;
      draftRevision: number;
    };
    expect(manualBody.conflicts).toBe(0);
    expect(manualBody.draftRevision).toBe(manualRevision + 1);

    // 决策后的草稿吸收新 schema。
    const draftAfter = await fetchOpsDraft(request, headers);
    expect(JSON.stringify(draftAfter.operation ?? {})).toContain('priority');

    // 恢复默认 schema。
    await restoreFixtureFunctions(request);
  });

  test('@republish-restores-execution 重发布后恢复执行且使用新快照', async ({ request }) => {
    const state = readRealFixtureState();
    const headers = await authenticatedHeaders(request);
    await ensureOpsPublished(request, headers);

    const staleBefore = await fetchOpsConsolePage(request, headers);
    const versionBefore = staleBefore.page?.version ?? 0;
    expect(versionBefore).toBeGreaterThan(0);
    expect(staleBefore.page?.bindingFreshness ?? []).toEqual([]);

    // 合同变化进入 stale。
    await replaceSDKFunctions(request, [
      defaultMailSend,
      {
        ...defaultOpsRestart,
        input_schema:
          '{"type":"object","properties":{"player_id":{"type":"string"},"title":{"type":"string"},"content":{"type":"string"},"priority":{"type":"integer"}},"required":["player_id","title","priority"]}',
      },
    ]);
    await expect
      .poll(
        async () => {
          const current = await fetchOpsConsolePage(request, headers);
          return (current.page?.bindingFreshness ?? []).length > 0;
        },
        { timeout: 30000 },
      )
      .toBe(true);

    // 旧 stale 快照执行被拒绝。
    const staleExecute = await request.post(
      `${state.serverBaseURL}/api/v1/console/pages/operation--ops.restart/bindings/ops.restart.main/execute`,
      { headers, data: { context: { form: { player_id: 'p-001', title: 'stale' } } } },
    );
    expect(staleExecute.status()).toBe(409);

    // 自动合并 + 人工决策吸收新合同。
    let draft = await fetchOpsDraft(request, headers);
    const revisionBefore = draft.draftRevision ?? 0;
    const diff = await fetchOpsDiff(request, headers);
    const autoMerge = await request.post(
      `${state.serverBaseURL}/api/v1/versioning/pages/operation--ops.restart/merge`,
      { headers, data: { expectedDraftRevision: revisionBefore, strategy: 'auto' } },
    );
    expect(autoMerge.status()).toBe(200);
    const autoBody = (await autoMerge.json()) as { draftRevision?: number; conflicts: number };
    expect(autoBody.conflicts).toBeGreaterThanOrEqual(1);
    const manualRevision = Math.max(autoBody.draftRevision ?? 0, revisionBefore);

    const manualMerge = await request.post(
      `${state.serverBaseURL}/api/v1/versioning/pages/operation--ops.restart/merge`,
      {
        headers,
        data: {
          expectedDraftRevision: manualRevision,
          strategy: 'manual',
          reason: 'accept new schema',
          conflicts: (diff.conflictItems ?? []).map((item) => ({
            path: item.field,
            acceptNew: true,
          })),
        },
      },
    );
    expect(manualMerge.status()).toBe(200);
    const manualBody = (await manualMerge.json()) as {
      conflicts: number;
      draftRevision: number;
    };
    expect(manualBody.conflicts).toBe(0);

    // 显式重发布新版本。
    const publish = await request.post(
      `${state.serverBaseURL}/api/v1/pages/operation--ops.restart/publish`,
      { headers, data: { draftRevision: manualBody.draftRevision } },
    );
    expect(publish.status()).toBe(200);
    const publishBody = (await publish.json()) as { publishedVersion?: number };
    expect(publishBody.publishedVersion ?? 0).toBeGreaterThan(versionBefore);

    // 新快照生效：页面 fresh。
    const republished = await fetchOpsConsolePage(request, headers);
    expect(republished.page?.bindingFreshness ?? []).toEqual([]);
    expect(republished.page?.version).toBe(publishBody.publishedVersion);

    // 用新 schema 完成一次真实执行。
    const clear = await request.delete(`${state.fixtureBaseURL}/__fixture__/sdk/calls`);
    expect(clear.status()).toBe(200);
    const execute = await request.post(
      `${state.serverBaseURL}/api/v1/console/pages/operation--ops.restart/bindings/ops.restart.main/execute`,
      {
        headers,
        data: { context: { form: { player_id: 'p-001', title: 'new snapshot', priority: 2 } } },
      },
    );
    expect(execute.status()).toBe(200);

    const callsResponse = await request.get(`${state.fixtureBaseURL}/__fixture__/sdk/calls`);
    const calls = (await callsResponse.json()) as {
      calls?: Array<{ functionId: string; payload?: Record<string, unknown> }>;
    };
    expect(calls.calls ?? []).toHaveLength(1);
    expect(calls.calls?.[0].payload).toMatchObject({
      player_id: 'p-001',
      title: 'new snapshot',
      priority: 2,
    });

    // 执行 audit 已使用新 proposal/page 版本。
    const auditResponse = await request.get(
      `${state.fixtureBaseURL}/__fixture__/audit/page-execute`,
    );
    expect(auditResponse.status()).toBe(200);
    const audit = (await auditResponse.json()) as {
      eventType: string;
      outcome: string;
      details: Record<string, unknown>;
    };
    expect(audit).toMatchObject({
      eventType: 'page.execute',
      outcome: 'success',
      details: {
        page_key: 'operation--ops.restart',
        binding_id: 'ops.restart.main',
      },
    });
    expect(audit.details.publish_version).toBe(publishBody.publishedVersion);
    expect(Number(audit.details.base_proposal_version)).toBeGreaterThan(0);

    // 恢复默认 schema 并回到 fresh。
    await restoreFixtureFunctions(request);
    await expect
      .poll(
        async () => {
          const current = await fetchOpsConsolePage(request, headers);
          return (current.page?.bindingFreshness ?? []).length;
        },
        { timeout: 30000 },
      )
      .toBe(0);
  });
});
