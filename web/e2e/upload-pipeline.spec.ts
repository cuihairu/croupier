/**
 * 上传即成页（契约与绑定正交化）real-dashboard 场景（todo.md T11 E2E，
 * 设计文档 docs/architecture/ui-generation-upload-pipeline.md）。
 *
 * 覆盖：
 * 1. @upload-pipeline-summary      无 agent 上传 OpenAPI → unbound 契约 +
 *    组件模板 + 页面提案一次生成，响应摘要计数与实际产出一致。
 * 2. @upload-single-section-page   组合页单函数区块即可保存/发布/执行
 *    （≥2 区块限制已移除）。
 * 3. @upload-unbound-empty-state   unbound 契约 execute 409
 *    executor_unbound；发布页渲染「未绑定执行器」空态（无 mock 兜底）。
 * 4. @upload-auto-bind             运行时注册同名函数自动翻转 bound，页面
 *    恢复可执行。
 * 5. @upload-auto-publish          publishReview=auto（fixture 经
 *    CROUPIER_E2E_PUBLISH_REVIEW=auto 注入）下 composite 保存直接落
 *    published_page_specs，无需人工接受提案。
 *
 * 隔离：全部使用 uploadalpha/uploadbeta/uploadgamma 独立 functionId 前缀与
 * 独立 source 名，不触碰 players/mail 共享物料；生成的页面在测试内经
 * DELETE /api/v1/versioning/pages/:pageKey 回收（source 无删除路由，独立
 * 名称保证对共享 scope 无副作用）。SDK 函数集的替换在用后恢复 fixture
 * 默认集（mail.send + mail.wait）。
 */

import { test, expect } from '@playwright/test';
import type { APIRequestContext } from '@playwright/test';
import { readRealFixtureState } from './helpers/realFixture';
import { login, navigateToConsole, waitForPageReady } from './helpers';

type PipelineSummaryDTO = {
  operations: number;
  contractsCreated: number;
  templatesUpdated: number;
  proposalsCreated: number;
  diagnostics?: Array<{ code: string; severity: string }>;
};

type UploadSourceResponseDTO = {
  source: { sourceId: string; operationCount: number };
  summary?: PipelineSummaryDTO;
};

type FunctionDescriptorDTO = {
  id: string;
  executionState?: string;
  version?: string;
  inputSchema?: unknown;
  outputSchema?: unknown;
};

type ProposalDTO = {
  proposalKey: string;
  pageKey: string;
  pageType: string;
  status: string;
};

type CompositeSaveResponseDTO = {
  proposalKey: string;
  pageKey: string;
  pageType: string;
  quality: string;
  published: boolean;
  publishError?: string;
};

type ConsolePageDTO = {
  page?: {
    pageKey: string;
    version?: number;
    bindings?: Array<{ id: string; functionId: string }>;
    bindingFreshness?: Array<{ status: string; bindingId?: string }>;
  };
};

type FixtureSDKFunctionDTO = {
  id: string;
  version?: string;
  summary?: string;
  inputSchema?: string;
  outputSchema?: string;
  enabled: boolean;
};

type OpenAPIDoc = Record<string, unknown>;

// 与 cmd/server DefaultFixtureSDKFunctions 对齐：SDK 函数集替换是全量语义，
// 场景 4 注册 uploadgamma.ping 后必须恢复这套默认值，避免污染共享 fixture。
const fixtureDefaultFunctions: FixtureSDKFunctionDTO[] = [
  {
    id: 'mail.send',
    version: '1.0.0',
    summary: 'Send an in-game mail',
    inputSchema:
      '{"type":"object","properties":{"player_id":{"type":"string"},"title":{"type":"string"},"content":{"type":"string"}},"required":["player_id","title"]}',
    outputSchema:
      '{"type":"object","properties":{"success":{"type":"boolean"},"mail_id":{"type":"string"}}}',
    enabled: true,
  },
  {
    id: 'mail.wait',
    version: '1.0.0',
    summary: 'Wait until completion or cancellation',
    inputSchema:
      '{"type":"object","properties":{"wait_ms":{"type":"integer","minimum":1}},"required":["wait_ms"]}',
    outputSchema:
      '{"type":"object","properties":{"success":{"type":"boolean"},"waited_ms":{"type":"integer"}}}',
    enabled: true,
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

async function listDescriptors(
  request: APIRequestContext,
  headers: Record<string, string>,
): Promise<FunctionDescriptorDTO[]> {
  const state = readRealFixtureState();
  const response = await request.get(`${state.serverBaseURL}/api/v1/functions/descriptors`, {
    headers,
  });
  expect(response.status()).toBe(200);
  return ((await response.json()) as { functions?: FunctionDescriptorDTO[] }).functions ?? [];
}

async function listProposalKeys(
  request: APIRequestContext,
  headers: Record<string, string>,
): Promise<string[]> {
  const state = readRealFixtureState();
  const response = await request.get(`${state.serverBaseURL}/api/v1/proposals`, { headers });
  expect(response.status()).toBe(200);
  return ((await response.json()) as ProposalDTO[]).map((item) => item.proposalKey);
}

async function uploadSource(
  request: APIRequestContext,
  headers: Record<string, string>,
  name: string,
  spec: OpenAPIDoc,
): Promise<UploadSourceResponseDTO> {
  const state = readRealFixtureState();
  const response = await request.post(`${state.serverBaseURL}/api/v1/openapi/sources`, {
    headers,
    data: { name, spec },
  });
  if (response.status() !== 201) {
    console.log('upload source error body:', await response.text());
  }
  expect(response.status()).toBe(201);
  return (await response.json()) as UploadSourceResponseDTO;
}

async function acceptAndPublish(
  request: APIRequestContext,
  headers: Record<string, string>,
  proposalKey: string,
): Promise<void> {
  const state = readRealFixtureState();
  const response = await request.post(
    `${state.serverBaseURL}/api/v1/proposals/${encodeURIComponent(proposalKey)}/accept-and-publish`,
    { headers },
  );
  if (response.status() !== 200) {
    console.log('accept-and-publish error body:', await response.text());
  }
  expect(response.status()).toBe(200);
}

async function fetchConsolePage(
  request: APIRequestContext,
  headers: Record<string, string>,
  pageKey: string,
): Promise<ConsolePageDTO> {
  const state = readRealFixtureState();
  const response = await request.get(
    `${state.serverBaseURL}/api/v1/console/pages/${encodeURIComponent(pageKey)}`,
    { headers },
  );
  expect(response.status()).toBe(200);
  return (await response.json()) as ConsolePageDTO;
}

async function deletePage(
  request: APIRequestContext,
  headers: Record<string, string>,
  pageKey: string,
): Promise<void> {
  const state = readRealFixtureState();
  const response = await request.delete(
    `${state.serverBaseURL}/api/v1/versioning/pages/${encodeURIComponent(pageKey)}`,
    { headers },
  );
  expect([200, 404]).toContain(response.status());
}

function jsonContent(schema: Record<string, unknown>): Record<string, unknown> {
  return { 'application/json': { schema } };
}

const alphaEntitySchema: Record<string, unknown> = {
  type: 'object',
  properties: { id: { type: 'string' }, name: { type: 'string' } },
};

/** 场景 1 文档：resource=uploadalphas（list/create/item get），无运行时函数。 */
function alphaDoc(): OpenAPIDoc {
  return {
    openapi: '3.0.3',
    info: { title: 'Upload Alpha API', version: '1.0.0' },
    paths: {
      '/uploadalphas': {
        get: {
          operationId: 'uploadalpha.list',
          summary: 'List upload alphas',
          responses: {
            '200': {
              description: 'OK',
              content: jsonContent({
                type: 'object',
                properties: {
                  items: { type: 'array', items: alphaEntitySchema },
                  total: { type: 'integer' },
                },
              }),
            },
          },
        },
        post: {
          operationId: 'uploadalpha.create',
          summary: 'Create upload alpha',
          requestBody: {
            required: true,
            content: jsonContent({
              type: 'object',
              properties: { name: { type: 'string' } },
              required: ['name'],
            }),
          },
          responses: {
            '201': { description: 'Created', content: jsonContent(alphaEntitySchema) },
          },
        },
      },
      '/uploadalphas/{id}': {
        get: {
          operationId: 'uploadalpha.get',
          summary: 'Get upload alpha',
          parameters: [{ name: 'id', in: 'path', required: true, schema: { type: 'string' } }],
          responses: {
            '200': { description: 'OK', content: jsonContent(alphaEntitySchema) },
          },
        },
      },
    },
  };
}

/** 场景 3 文档：resource=uploadbetas（list + item get），全部 unbound。 */
function betaDoc(): OpenAPIDoc {
  return {
    openapi: '3.0.3',
    info: { title: 'Upload Beta API', version: '1.0.0' },
    paths: {
      '/uploadbetas': {
        get: {
          operationId: 'uploadbeta.list',
          summary: 'List upload betas',
          responses: {
            '200': {
              description: 'OK',
              content: jsonContent({
                type: 'object',
                properties: {
                  items: { type: 'array', items: alphaEntitySchema },
                  total: { type: 'integer' },
                },
              }),
            },
          },
        },
      },
      '/uploadbetas/{id}': {
        get: {
          operationId: 'uploadbeta.get',
          summary: 'Get upload beta',
          parameters: [{ name: 'id', in: 'path', required: true, schema: { type: 'string' } }],
          responses: {
            '200': { description: 'OK', content: jsonContent(alphaEntitySchema) },
          },
        },
      },
    },
  };
}

/**
 * 场景 4 文档：显式 x-capability=action（规避 REST 分类推断出 resource），
 * 产出 standalone operation 提案 operation:uploadgamma.ping。
 */
function gammaDoc(): OpenAPIDoc {
  return {
    openapi: '3.0.3',
    info: { title: 'Upload Gamma API', version: '1.0.0' },
    paths: {
      '/gamma/ping': {
        post: {
          operationId: 'uploadgamma.ping',
          summary: 'Upload gamma ping',
          'x-capability': 'action',
          requestBody: {
            required: true,
            content: jsonContent({
              type: 'object',
              properties: { message: { type: 'string' } },
              required: ['message'],
            }),
          },
          responses: {
            '200': {
              description: 'OK',
              content: jsonContent({
                type: 'object',
                properties: { ok: { type: 'boolean' } },
              }),
            },
          },
        },
      },
    },
  };
}

test.describe('上传即成页管线', () => {
  test('@upload-pipeline-summary 无运行时函数上传即出 unbound 契约/组件模板/页面提案', async ({
    request,
  }) => {
    const headers = await authenticatedHeaders(request);
    const beforeKeys = await listProposalKeys(request, headers);

    const uploaded = await uploadSource(request, headers, 'upload-alpha-provider', alphaDoc());
    expect(uploaded.source.sourceId).toMatch(/\S/);
    expect(uploaded.source.operationCount).toBe(3);

    // 摘要计数与实际操作数一致：3 个 operation → 3 个新 unbound 契约，
    // 提案 diff 出 1 个新资源提案，模板数不少于契约数（每契约至少一个模板）。
    const summary = uploaded.summary;
    expect(summary).toBeDefined();
    expect(summary?.operations).toBe(3);
    expect(summary?.contractsCreated).toBe(3);
    const afterKeys = await listProposalKeys(request, headers);
    const newProposalKeys = afterKeys.filter((key) => !beforeKeys.includes(key));
    expect(newProposalKeys).toEqual(['resource:uploadalphas']);
    expect(summary?.proposalsCreated).toBe(newProposalKeys.length);
    expect(summary?.templatesUpdated ?? 0).toBeGreaterThanOrEqual(summary?.contractsCreated ?? 0);

    // 契约落库且 executionState=unbound（无对应运行时函数）。
    const descriptors = await listDescriptors(request, headers);
    for (const functionId of ['uploadalpha.list', 'uploadalpha.create', 'uploadalpha.get']) {
      const fn = descriptors.find((item) => item.id === functionId);
      expect(fn, `descriptor ${functionId} 应存在`).toBeDefined();
      expect(fn?.executionState).toBe('unbound');
    }

    // 组件模板按契约生成（fn--<functionId>）。
    const state = readRealFixtureState();
    for (const functionId of ['uploadalpha.list', 'uploadalpha.create', 'uploadalpha.get']) {
      const template = await request.get(
        `${state.serverBaseURL}/api/v1/component-templates/fn--${functionId}`,
        { headers },
      );
      expect(template.status(), `template fn--${functionId} 应存在`).toBe(200);
    }
  });

  test('@upload-single-section-page 单区块组合页保存/发布/执行全链', async ({ page, request }) => {
    const headers = await authenticatedHeaders(request);
    const state = readRealFixtureState();
    const pageKey = 'composite--upload-single';

    // 单函数区块（≥2 区块限制已移除）：保存即生成 composite 提案；
    // publishReview=auto（fixture 注入）时保存同时直接发布。
    const save = await request.post(`${state.serverBaseURL}/api/v1/versioning/pages/composite`, {
      headers,
      data: {
        pageKey,
        sections: [{ functionId: 'mail.send', view: 'form', title: '单区块发送邮件' }],
      },
    });
    if (save.status() !== 200) {
      console.log('composite save error body:', await save.text());
    }
    expect(save.status()).toBe(200);
    const saved = (await save.json()) as CompositeSaveResponseDTO;
    expect(saved.pageType).toBe('composite');
    expect(saved.pageKey).toBe(pageKey);
    if (!saved.published) {
      // required 策略兜底：走人工接受发布链（行为与 publishReview 配置无关）。
      await acceptAndPublish(request, headers, saved.proposalKey);
    }

    // 发布链走通：console 页可读，且只有 1 个 binding（单区块）。
    const consolePage = await fetchConsolePage(request, headers, pageKey);
    expect(consolePage.page?.pageKey).toBe(pageKey);
    expect(consolePage.page?.bindings).toHaveLength(1);
    expect(consolePage.page?.bindings?.[0]).toMatchObject({
      id: 'mail.send',
      functionId: 'mail.send',
    });

    // 页面可执行：缺省 selector 为 form 同名路径（渲染层区块执行只发 context.form）。
    const clear = await request.delete(`${state.fixtureBaseURL}/__fixture__/sdk/calls`);
    expect(clear.status()).toBe(200);
    const form = { player_id: 'p-001', title: 'upload-single', content: 'hello' };
    const execute = await request.post(
      `${state.serverBaseURL}/api/v1/console/pages/${encodeURIComponent(pageKey)}/bindings/mail.send/execute`,
      { headers, data: { context: { form } } },
    );
    if (execute.status() !== 200) {
      console.log('composite execute error body:', await execute.text());
    }
    expect(execute.status()).toBe(200);
    const callsResponse = await request.get(`${state.fixtureBaseURL}/__fixture__/sdk/calls`);
    const calls = (await callsResponse.json()) as {
      calls?: Array<{ functionId: string; payload?: Record<string, unknown> }>;
    };
    expect(calls.calls ?? []).toHaveLength(1);
    expect(calls.calls?.[0].functionId).toBe('mail.send');
    expect(calls.calls?.[0].payload).toMatchObject({
      player_id: 'p-001',
      title: 'upload-single',
    });

    // UI 冒烟：发布页按 composite 渲染出唯一区块表单。
    await login(page);
    await navigateToConsole(page, 'composite', pageKey);
    await waitForPageReady(page);
    await expect(page.getByText('单区块发送邮件').first()).toBeVisible();
    await expect(page.getByRole('textbox', { name: /player[ _-]?id/i })).toBeVisible();

    await deletePage(request, headers, pageKey);
  });

  test('@upload-unbound-empty-state unbound 契约执行 409 且发布页渲染「未绑定执行器」空态', async ({
    page,
    request,
  }) => {
    const headers = await authenticatedHeaders(request);
    const state = readRealFixtureState();

    await uploadSource(request, headers, 'upload-beta-provider', betaDoc());
    await acceptAndPublish(request, headers, 'resource:uploadbetas');

    const consolePage = await fetchConsolePage(request, headers, 'resource--uploadbetas');
    const listBinding = consolePage.page?.bindings?.find((binding) => binding.id === 'list');
    expect(listBinding).toMatchObject({ id: 'list', functionId: 'uploadbeta.list' });

    // 服务端边界：409 + 稳定错误码 executor_unbound（非 500、非静默执行）。
    const execute = await request.post(
      `${state.serverBaseURL}/api/v1/console/pages/resource--uploadbetas/bindings/list/execute`,
      { headers, data: { context: { form: {} } } },
    );
    expect(execute.status()).toBe(409);
    const errorBody = (await execute.json()) as {
      error?: string;
      details?: { bindingId?: string; functionId?: string };
    };
    expect(errorBody.error).toBe('executor_unbound');
    expect(errorBody.details).toMatchObject({
      bindingId: 'list',
      functionId: 'uploadbeta.list',
    });

    // 前端：发布页加载触发 list 执行被 409 阻断 → 渲染结构化空态（无 mock 兜底）。
    await login(page);
    // DEBUG（临时）：全量 API 网络日志，定位 list execute 是否发出。
    page.on('request', (r) => {
      if (r.url().includes('/api/v1/console')) {
        console.log(`[DBG REQ] ${r.method()} ${r.url()}`);
      }
    });
    page.on('response', (r) => {
      if (r.url().includes('/api/v1/console')) {
        console.log(`[DBG RES] ${r.status()} ${r.url()}`);
      }
    });
    page.on('requestfailed', (r) => {
      if (r.url().includes('/api/v1/console')) {
        console.log(`[DBG FAIL] ${r.failure()?.errorText} ${r.url()}`);
      }
    });
    page.on('console', (msg) => {
      if (msg.type() === 'error' || msg.type() === 'warning' || msg.text().includes('DBG')) {
        console.log(`[DBG CONSOLE ${msg.type()}] ${msg.text().slice(0, 600)}`);
      }
    });
    page.on('pageerror', (err) => {
      console.log(`[DBG PAGEERROR] ${String(err).slice(0, 600)}`);
    });
    await page.addInitScript(() => {
      const origOpen = XMLHttpRequest.prototype.open;
      XMLHttpRequest.prototype.open = function patchedOpen(
        this: XMLHttpRequest,
        ...args: Parameters<typeof origOpen>
      ) {
        console.log(`[DBG XHR-OPEN] ${String(args[0])} ${String(args[1])}`);
        return origOpen.apply(this, args);
      };
      const origSend = XMLHttpRequest.prototype.send;
      XMLHttpRequest.prototype.send = function patchedSend(
        this: XMLHttpRequest,
        ...args: Parameters<typeof origSend>
      ) {
        console.log(`[DBG XHR-SEND] ${String(JSON.stringify(args[0])?.slice(0, 200))}`);
        return origSend.apply(this, args);
      };
      window.addEventListener('unhandledrejection', (event) => {
        const reason = event.reason as { stack?: string } | undefined;
        console.log(`[DBG UNHANDLED-REJECTION] ${String(reason?.stack ?? reason).slice(0, 800)}`);
      });
    });
    page.on('response', async (response) => {
      if (response.url().includes('/console/pages/resource--uploadbetas')) {
        try {
          console.log(`[DBG PAGE-SPEC] ${(await response.text()).slice(0, 4000)}`);
        } catch {
          /* body unavailable */
        }
      }
    });
    const listExecute = page.waitForResponse(
      (response) =>
        response.url().includes('/bindings/list/execute') && response.request().method() === 'POST',
    );
    await navigateToConsole(page, 'uploadbetas', 'resource--uploadbetas');
    await waitForPageReady(page);
    expect((await listExecute).status()).toBe(409);
    await expect(page.getByText('未绑定执行器')).toBeVisible();
    await expect(page.getByRole('button', { name: '去绑定' })).toBeVisible();
    // 空态即空数据：不允许出现伪造数据行。
    await expect(page.locator('.ant-table-row')).toHaveCount(0);

    await deletePage(request, headers, 'resource--uploadbetas');
  });

  test.describe('自动绑定', () => {
    test.afterEach(async ({ request }) => {
      // 恢复 fixture 默认 SDK 函数集，避免 uploadgamma.ping 泄漏到其他 spec。
      const state = readRealFixtureState();
      const restore = await request.post(`${state.fixtureBaseURL}/__fixture__/sdk/functions`, {
        data: { functions: fixtureDefaultFunctions },
      });
      expect(restore.status()).toBe(200);
    });

    test('@upload-auto-bind 运行时注册同名函数自动翻转 bound 并恢复可执行', async ({ request }) => {
      const headers = await authenticatedHeaders(request);
      const state = readRealFixtureState();
      const pageKey = 'operation--uploadgamma.ping';

      await uploadSource(request, headers, 'upload-gamma-provider', gammaDoc());
      const before = await listDescriptors(request, headers);
      const unbound = before.find((item) => item.id === 'uploadgamma.ping');
      expect(unbound?.executionState).toBe('unbound');

      // unbound 物料照常走提案 → 发布（契约与绑定正交，发布不被阻断）。
      await acceptAndPublish(request, headers, 'operation:uploadgamma.ping');
      const published = await fetchConsolePage(request, headers, pageKey);
      const bindingId = published.page?.bindings?.[0]?.id ?? '';
      expect(bindingId).toMatch(/\S/);

      // 注册前执行被 409 executor_unbound 阻断。
      const blocked = await request.post(
        `${state.serverBaseURL}/api/v1/console/pages/${encodeURIComponent(pageKey)}/bindings/${encodeURIComponent(bindingId)}/execute`,
        { headers, data: { context: { form: { message: 'hello-gamma' } } } },
      );
      expect(blocked.status()).toBe(409);
      expect(((await blocked.json()) as { error?: string }).error).toBe('executor_unbound');

      // 运行时注册同名函数（schema 取自契约描述符 round-trip，保持 digest
      // 一致避免误报 stale；version 与上传文档 info.version 对齐）。
      const gammaFunction: FixtureSDKFunctionDTO = {
        id: 'uploadgamma.ping',
        version: unbound?.version ?? '1.0.0',
        summary: 'Upload gamma ping',
        inputSchema: JSON.stringify(unbound?.inputSchema ?? {}),
        outputSchema: JSON.stringify(unbound?.outputSchema ?? {}),
        enabled: true,
      };
      const replace = await request.post(`${state.fixtureBaseURL}/__fixture__/sdk/functions`, {
        data: { functions: [...fixtureDefaultFunctions, gammaFunction] },
      });
      expect(replace.status()).toBe(200);

      // D3/T6：注册命中同 scope 同 functionId 的 unbound 契约 → 自动翻转 bound。
      await expect
        .poll(
          async () => {
            const descriptors = await listDescriptors(request, headers);
            return descriptors.find((item) => item.id === 'uploadgamma.ping')?.executionState;
          },
          { timeout: 30000 },
        )
        .toBe('bound');

      // 发布快照保持 fresh（schema/governance 未漂移），页面恢复可执行。
      const afterBind = await fetchConsolePage(request, headers, pageKey);
      expect(afterBind.page?.bindingFreshness ?? []).toEqual([]);

      const clear = await request.delete(`${state.fixtureBaseURL}/__fixture__/sdk/calls`);
      expect(clear.status()).toBe(200);
      const execute = await request.post(
        `${state.serverBaseURL}/api/v1/console/pages/${encodeURIComponent(pageKey)}/bindings/${encodeURIComponent(bindingId)}/execute`,
        { headers, data: { context: { form: { message: 'hello-gamma' } } } },
      );
      if (execute.status() !== 200) {
        console.log('auto-bind execute error body:', await execute.text());
      }
      expect(execute.status()).toBe(200);
      const callsResponse = await request.get(`${state.fixtureBaseURL}/__fixture__/sdk/calls`);
      const calls = (await callsResponse.json()) as {
        calls?: Array<{ functionId: string; payload?: Record<string, unknown> }>;
      };
      expect(calls.calls ?? []).toEqual([
        expect.objectContaining({ functionId: 'uploadgamma.ping' }),
      ]);

      await deletePage(request, headers, pageKey);
    });
  });

  test('@upload-auto-publish publishReview=auto 下 composite 保存即落 published_page_specs', async ({
    request,
  }) => {
    const headers = await authenticatedHeaders(request);
    const state = readRealFixtureState();
    const pageKey = 'composite--upload-autopub';

    const save = await request.post(`${state.serverBaseURL}/api/v1/versioning/pages/composite`, {
      headers,
      data: {
        pageKey,
        sections: [{ functionId: 'mail.send', view: 'form', title: '自动发布区块' }],
      },
    });
    if (save.status() !== 200) {
      console.log('composite save error body:', await save.text());
    }
    expect(save.status()).toBe(200);
    const saved = (await save.json()) as CompositeSaveResponseDTO;
    // 保存即发布：无需人工接受提案；发布失败会以 publishError 降级回人工链。
    expect(saved.publishError ?? '').toBe('');
    expect(saved.published).toBe(true);

    // published_page_specs 已落库：不做任何 accept-and-publish，console 页直接可读。
    const consolePage = await fetchConsolePage(request, headers, pageKey);
    expect(consolePage.page?.pageKey).toBe(pageKey);
    expect(consolePage.page?.version ?? 0).toBeGreaterThanOrEqual(1);
    expect(consolePage.page?.bindings).toHaveLength(1);

    // 提案已被自动接受（不再处于 pending 待审）。
    const proposalsResponse = await request.get(`${state.serverBaseURL}/api/v1/proposals`, {
      headers,
    });
    expect(proposalsResponse.status()).toBe(200);
    const proposals = (await proposalsResponse.json()) as ProposalDTO[];
    const proposal = proposals.find((item) => item.pageKey === pageKey);
    expect(proposal).toBeDefined();
    expect(proposal?.status).not.toBe('pending');

    await deletePage(request, headers, pageKey);
  });
});
