/**
 * 场景 1: OpenAPI CRUD - 资源页面完整功能
 *
 * 验收标准：
 * - ProTable 分页、详情、create/update/delete
 * - row ban action
 * - 动态菜单
 * - 受控执行
 */

import { test, expect } from '@playwright/test';
import type { APIRequestContext } from '@playwright/test';
import { readRealFixtureState } from './helpers/realFixture';
import {
  ensurePlayersSourceBound,
  ensurePlayersResourcePublished,
  openapiAuthenticatedHeaders,
  playersOperations,
} from './helpers/openapiPlayers';
import {
  login,
  navigateToConsole,
  waitForPageReady,
  waitForTable,
  expectTableVisible,
  expectModalVisible,
  expectDrawerVisible,
} from './helpers';

test.describe('OpenAPI CRUD', () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
  });

  test('资源列表页加载', async ({ page }) => {
    await navigateToConsole(page, 'players', 'resource--players');
    await waitForPageReady(page);

    // 验证 ProTable 渲染
    await waitForTable(page);
    await expectTableVisible(page);

    // 验证列标题
    await expect(page.locator('th:has-text("ID"), th:has-text("id")').first()).toBeVisible();
    await expect(page.locator('th:has-text("名称"), th:has-text("name")').first()).toBeVisible();
  });

  test('资源列表数据展示', async ({ page }) => {
    await navigateToConsole(page, 'players', 'resource--players');
    await waitForPageReady(page);
    await waitForTable(page);

    // 验证表格有数据行
    const rows = await page.locator('tbody tr, .ant-table-row').count();
    expect(rows).toBeGreaterThan(0);
  });

  test('创建资源', async ({ page }) => {
    await navigateToConsole(page, 'players', 'resource--players');
    await waitForPageReady(page);
    await waitForTable(page);

    // 点击新建按钮
    const createBtn = page
      .locator('button:has-text("新建"), button:has-text("创建"), button:has-text("新增")')
      .first();
    await expect(createBtn).toBeVisible();
    await createBtn.click();

    // 等待 Modal 出现
    await expectModalVisible(page);

    // 生成表单必须包含 OpenAPI request schema 中的 name 字段。
    const nameInput = page
      .getByRole('dialog')
      .getByLabel(/名称|name/i)
      .first();
    await expect(nameInput).toBeVisible();
    await nameInput.fill('测试玩家');

    // 提交
    const submitBtn = page
      .locator('.ant-modal button:has-text("确"), .ant-modal button:has-text("OK")')
      .first();
    await expect(submitBtn).toBeVisible();
    const executeResponse = page.waitForResponse(
      (response) =>
        response.url().includes('/bindings/create/execute') &&
        response.request().method() === 'POST',
    );
    await submitBtn.click();
    expect((await executeResponse).status()).toBe(200);
    await expect(page.getByText('创建成功')).toBeVisible();
  });

  test('查看详情', async ({ page }) => {
    await navigateToConsole(page, 'players', 'resource--players');
    await waitForPageReady(page);
    await waitForTable(page);

    // 点击查看/详情按钮
    const detailBtn = page
      .locator('a:has-text("查看"), button:has-text("查看"), a:has-text("详情")')
      .first();
    await expect(detailBtn).toBeVisible();
    await detailBtn.click();

    // 等待 Drawer 出现
    await expectDrawerVisible(page);

    // 验证详情内容
    await expect(page.locator('.ant-drawer').first()).toBeVisible();

    // 关闭 Drawer
    await page.locator('.ant-drawer-close').first().click();
  });

  test('编辑资源', async ({ page }) => {
    await navigateToConsole(page, 'players', 'resource--players');
    await waitForPageReady(page);
    await waitForTable(page);

    // 点击编辑按钮
    const editBtn = page.locator('a:has-text("编辑"), button:has-text("编辑")').first();
    await expect(editBtn).toBeVisible();
    await editBtn.click();

    // 等待 Modal 出现
    await expectModalVisible(page);

    // 修改名称
    const nameInput = page.getByRole('dialog').getByLabel(/名称|name/i);
    await nameInput.clear();
    await nameInput.fill('更新后的玩家');

    // 提交
    const submitBtn = page
      .locator('.ant-modal button:has-text("确"), .ant-modal button:has-text("OK")')
      .first();
    await submitBtn.click();

    // 等待 Modal 关闭
    await page.locator('.ant-modal').waitFor({ state: 'hidden', timeout: 10000 });
  });

  test('删除资源', async ({ page }) => {
    await navigateToConsole(page, 'players', 'resource--players');
    await waitForPageReady(page);
    await waitForTable(page);

    // 点击删除按钮
    const deleteBtn = page.locator('button:has-text("删除"), a:has-text("删除")').first();
    await expect(deleteBtn).toBeVisible();
    await deleteBtn.click();

    // 确认删除
    const confirmBtn = page
      .locator('.ant-popconfirm .ant-btn-primary, .ant-modal-confirm .ant-btn-primary')
      .first();
    await expect(confirmBtn).toBeVisible();
    const executeResponse = page.waitForResponse(
      (response) =>
        response.url().includes('/bindings/delete/execute') &&
        response.request().method() === 'POST',
    );
    await confirmBtn.click();
    expect((await executeResponse).status()).toBe(200);
    await expect(page.locator('.ant-message')).toBeVisible();
  });

  test('行操作 - 封禁', async ({ page }) => {
    await navigateToConsole(page, 'players', 'resource--players');
    await waitForPageReady(page);
    await waitForTable(page);

    // 点击行操作按钮
    const banBtn = page.locator('button:has-text("封禁"), a:has-text("封禁")').first();
    await expect(banBtn).toBeVisible();
    await banBtn.click();

    // 确认操作
    const confirmBtn = page
      .locator('.ant-popconfirm .ant-btn-primary, .ant-modal-confirm .ant-btn-primary')
      .first();
    await expect(confirmBtn).toBeVisible();
    const executeResponse = page.waitForResponse(
      (response) => response.url().includes('/bindings/') && response.request().method() === 'POST',
    );
    await confirmBtn.click();
    expect((await executeResponse).status()).toBe(200);
    await expect(page.locator('.ant-message-success')).toBeVisible();
  });
});

type ProposalDTO = {
  proposalKey: string;
  pageKey: string;
  pageType: string;
  resourceKey?: string;
  quality: string;
  status: string;
  pageSpec: {
    resource?: {
      listView?: {
        identityKey?: string;
        rowActions?: Array<{ bindingId?: string }>;
      };
    };
    bindings?: Array<{ id: string; functionId: string; selectors?: unknown }>;
  };
};

test.describe('真实 OpenAPI players Proposal 链路', () => {
  test('@openapi-ready-proposal /players 生成单一 ready Resource Proposal', async ({ request }) => {
    const headers = await openapiAuthenticatedHeaders(request);
    const sourceId = await ensurePlayersSourceBound(request, headers);
    expect(sourceId).toMatch(/\S/);

    const state = readRealFixtureState();
    const proposalsResponse = await request.get(`${state.serverBaseURL}/api/v1/proposals`, {
      headers,
    });
    expect(proposalsResponse.status()).toBe(200);
    const proposals = (await proposalsResponse.json()) as ProposalDTO[];

    const playersProposals = proposals.filter((item) => item.proposalKey === 'resource:players');
    expect(playersProposals).toHaveLength(1);
    const proposal = playersProposals[0];
    expect(proposal).toMatchObject({
      pageKey: 'resource--players',
      pageType: 'resource',
      resourceKey: 'players',
      quality: 'ready',
    });
    expect(proposal.pageSpec.resource?.listView).toMatchObject({ identityKey: 'id' });

    // list/detail/create/update/delete 与 row action 都进入同一资源页面。
    expect(proposal.pageSpec.bindings).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ id: 'list', functionId: 'players.player.list' }),
        expect.objectContaining({ id: 'detail', functionId: 'players.player.get' }),
        expect.objectContaining({ id: 'create', functionId: 'players.player.create' }),
        expect.objectContaining({ id: 'update', functionId: 'players.player.update' }),
        expect.objectContaining({ id: 'delete', functionId: 'players.player.delete' }),
        expect.objectContaining({ id: 'action.kick', functionId: 'players.player.kick' }),
      ]),
    );
    expect(proposal.pageSpec.resource?.listView?.rowActions ?? []).toEqual(
      expect.arrayContaining([expect.objectContaining({ bindingId: 'action.kick' })]),
    );

    // ready proposal 的每个 binding 都必须有 selector，缺 selector 不得标记 ready。
    for (const binding of proposal.pageSpec.bindings ?? []) {
      expect(binding.selectors).toBeDefined();
    }

    // 每个 CRUD operation 不得生成为独立资源/操作页面。
    const standaloneKeys = playersOperations.map((op) => `operation:players.${op.operationId}`);
    expect(proposals.filter((item) => standaloneKeys.includes(item.proposalKey))).toEqual([]);
    expect(proposals.filter((item) => item.pageKey === 'resource--players')).toHaveLength(1);
  });

  test('@openapi-resource-publish Inbox 发布后菜单只包含此 published page', async ({ request }) => {
    const headers = await openapiAuthenticatedHeaders(request);
    await ensurePlayersSourceBound(request, headers);
    const state = readRealFixtureState();

    // 幂等发布：未发布时经 Inbox accept-and-publish 路由发布。
    const pageURL = `${state.serverBaseURL}/api/v1/console/pages/resource--players`;
    let current = await request.get(pageURL, { headers });
    if (current.status() !== 200) {
      expect(current.status()).toBe(404);
      const publish = await request.post(
        `${state.serverBaseURL}/api/v1/proposals/resource%3Aplayers/accept-and-publish`,
        { headers },
      );
      expect(publish.status()).toBe(200);
      current = await request.get(pageURL, { headers });
    }
    expect(current.status()).toBe(200);
    const published = (await current.json()) as {
      page?: {
        pageKey: string;
        bindingContracts?: Array<{
          bindingId: string;
          functionId: string;
          rendererSchemaVersion?: string;
          inputSchemaDigest?: string;
          outputSchemaDigest?: string;
        }>;
      };
    };
    expect(published.page?.pageKey).toBe('resource--players');
    const contracts = published.page?.bindingContracts ?? [];
    expect(contracts.map((item) => item.functionId).sort()).toEqual(
      playersOperations.map((op) => `players.${op.operationId}`).sort(),
    );
    for (const contract of contracts) {
      expect(contract.rendererSchemaVersion).toBe('page-spec:1');
      expect(contract.inputSchemaDigest).toMatch(/^[a-f0-9]{64}$/);
      // 无响应 body 的操作（如 204 delete）允许没有 output digest。
      if (contract.outputSchemaDigest !== undefined && contract.outputSchemaDigest !== '') {
        expect(contract.outputSchemaDigest).toMatch(/^[a-f0-9]{64}$/);
      }
    }

    // 菜单仅来自 PublishedPageSpec，不来自 OpenAPI tag/source/静态 locale。
    const menuResponse = await request.get(`${state.serverBaseURL}/api/v1/console/menu`, {
      headers,
    });
    expect(menuResponse.status()).toBe(200);
    const menu = (await menuResponse.json()) as {
      items?: Array<{ key: string; children?: Array<{ key: string; path: string }> }>;
    };
    const menuItems = menu.items?.flatMap((category) => category.children || []) || [];
    expect(menuItems.filter((item) => item.key === 'resource--players')).toHaveLength(1);
    const playersItem = menuItems.find((item) => item.key === 'resource--players');
    expect(playersItem?.path).toBe('/console/players/resource--players');

    // 不存在由 source/contract/OpenAPI tag 直接生成的额外 players 菜单项。
    const playerRelated = menuItems
      .filter((item) => item.key.toLowerCase().includes('player'))
      .map((item) => item.key)
      .sort();
    expect(playerRelated).toEqual(['resource--players']);
  });

  test('@openapi-list-pagination 列表渲染真实数据、total 与刷新', async ({ page, request }) => {
    const headers = await openapiAuthenticatedHeaders(request);
    await ensurePlayersSourceBound(request, headers);
    await ensurePlayersResourcePublished(request, headers);
    const state = readRealFixtureState();

    const reset = await request.post(`${state.fixtureBaseURL}/__fixture__/provider/reset`);
    expect(reset.status()).toBe(200);

    const providerListCalls = async (): Promise<number> => {
      const callsResponse = await request.get(`${state.fixtureBaseURL}/__fixture__/provider/calls`);
      expect(callsResponse.status()).toBe(200);
      const body = (await callsResponse.json()) as {
        calls?: Array<{ method: string; path: string }>;
      };
      return (body.calls ?? []).filter((call) => call.method === 'GET' && call.path === '/players')
        .length;
    };

    await login(page);
    const firstExecute = page.waitForResponse(
      (response) =>
        response.url().includes('/bindings/list/execute') && response.request().method() === 'POST',
    );
    await navigateToConsole(page, 'players', 'resource--players');
    await waitForPageReady(page);
    const initialExecute = await firstExecute;
    expect(initialExecute.status()).toBe(200);
    const executeBody = await initialExecute.json();
    const serialized = JSON.stringify(executeBody);
    expect(serialized).toContain('p-001');
    expect(serialized).toContain('Ada');
    expect(serialized).toContain('"total":2');

    // Console 表格显示 fixture 的两条真实 player 数据。
    await waitForTable(page);
    const row1 = page.locator('tbody tr').filter({ hasText: 'p-001' });
    await expect(row1).toHaveCount(1);
    await expect(row1).toContainText('Ada');
    const row2 = page.locator('tbody tr').filter({ hasText: 'p-002' });
    await expect(row2).toHaveCount(1);
    await expect(row2).toContainText('Ben');

    // 分页渲染：单一数据页处于激活状态。
    await expect(page.locator('.ant-pagination')).toBeVisible();
    await expect(page.locator('.ant-pagination-item-1')).toHaveClass(/ant-pagination-item-active/);

    // 刷新按钮触发新的 published list binding execute 与 provider 调用。
    const callsBefore = await providerListCalls();
    expect(callsBefore).toBeGreaterThanOrEqual(1);
    const refreshExecute = page.waitForResponse(
      (response) =>
        response.url().includes('/bindings/list/execute') && response.request().method() === 'POST',
    );
    await page.getByRole('button', { name: '刷新' }).click();
    expect((await refreshExecute).status()).toBe(200);
    await expect.poll(providerListCalls, { timeout: 10000 }).toBeGreaterThan(callsBefore);
  });

  test('@openapi-detail-identity 详情按 row identity 获取并渲染', async ({ page, request }) => {
    const headers = await openapiAuthenticatedHeaders(request);
    await ensurePlayersSourceBound(request, headers);
    await ensurePlayersResourcePublished(request, headers);
    const state = readRealFixtureState();

    const reset = await request.post(`${state.fixtureBaseURL}/__fixture__/provider/reset`);
    expect(reset.status()).toBe(200);

    await login(page);
    await navigateToConsole(page, 'players', 'resource--players');
    await waitForPageReady(page);
    await waitForTable(page);

    const row2 = page.locator('tbody tr').filter({ hasText: 'p-002' });
    await expect(row2).toHaveCount(1);

    const detailExecute = page.waitForResponse(
      (response) =>
        response.url().includes('/bindings/detail/execute') &&
        response.request().method() === 'POST',
    );
    await row2.getByRole('button', { name: '查看' }).click();
    const detailResponse = await detailExecute;
    expect(detailResponse.status()).toBe(200);
    const detailBody = await detailResponse.json();
    const detailSerialized = JSON.stringify(detailBody);
    expect(detailSerialized).toContain('p-002');
    expect(detailSerialized).toContain('Ben');

    // 详情抽屉展示 provider 返回的详情字段。
    const drawer = page.locator('.ant-drawer').filter({ hasText: '详情' });
    await expect(drawer).toBeVisible();
    await expect(drawer).toContainText('Ben');
    await expect(drawer).toContainText('20');

    // detail binding 只对该行 identity 发起 provider 请求，不透传整行或请求其他行。
    const callsResponse = await request.get(`${state.fixtureBaseURL}/__fixture__/provider/calls`);
    expect(callsResponse.status()).toBe(200);
    const callsBody = (await callsResponse.json()) as {
      calls?: Array<{ method: string; path: string }>;
    };
    const detailCalls = (callsBody.calls ?? []).filter(
      (call) => call.method === 'GET' && call.path.startsWith('/players/'),
    );
    expect(detailCalls).toEqual([{ method: 'GET', path: '/players/p-002' }]);
  });

  test('@openapi-create-refresh 生成表单创建 player 后列表刷新', async ({ page, request }) => {
    const headers = await openapiAuthenticatedHeaders(request);
    await ensurePlayersSourceBound(request, headers);
    await ensurePlayersResourcePublished(request, headers);
    const state = readRealFixtureState();

    const reset = await request.post(`${state.fixtureBaseURL}/__fixture__/provider/reset`);
    expect(reset.status()).toBe(200);

    await login(page);
    await navigateToConsole(page, 'players', 'resource--players');
    await waitForPageReady(page);
    await waitForTable(page);

    // 使用生成表单（SchemaFormRenderer）创建，不允许第二套表单。
    const createBtn = page
      .locator('button:has-text("新建"), button:has-text("创建"), button:has-text("新增")')
      .first();
    await expect(createBtn).toBeVisible();
    await createBtn.click();
    await expectModalVisible(page);

    const dialog = page.getByRole('dialog');
    const nameInput = dialog.getByRole('textbox', { name: /name/i }).first();
    await expect(nameInput).toBeVisible();
    await nameInput.fill('Cara');
    const levelInput = dialog.getByRole('spinbutton', { name: /level/i }).first();
    await levelInput.fill('30');

    const createExecute = page.waitForResponse(
      (response) =>
        response.url().includes('/bindings/create/execute') &&
        response.request().method() === 'POST',
    );
    await dialog.getByRole('button', { name: /确|OK/ }).first().click();
    const createResponse = await createExecute;
    expect(createResponse.status()).toBe(200);

    // provider 真实持久化了创建请求。
    const callsResponse = await request.get(`${state.fixtureBaseURL}/__fixture__/provider/calls`);
    const callsBody = (await callsResponse.json()) as {
      calls?: Array<{ method: string; path: string; body?: unknown }>;
    };
    const createCalls = (callsBody.calls ?? []).filter(
      (call) => call.method === 'POST' && call.path === '/players',
    );
    expect(createCalls).toHaveLength(1);
    expect(JSON.stringify(createCalls[0].body)).toContain('Cara');

    // 创建后列表刷新，新 identity 与名称可见（非前端乐观插行）。
    const newRow = page.locator('tbody tr').filter({ hasText: 'p-003' });
    await expect(newRow).toHaveCount(1);
    await expect(newRow).toContainText('Cara');
    const row1 = page.locator('tbody tr').filter({ hasText: 'p-001' });
    await expect(row1).toHaveCount(1);
  });

  test('@openapi-update-selector 更新只提交 selector identity 与可编辑字段', async ({
    page,
    request,
  }) => {
    const headers = await openapiAuthenticatedHeaders(request);
    await ensurePlayersSourceBound(request, headers);
    await ensurePlayersResourcePublished(request, headers);
    const state = readRealFixtureState();

    const reset = await request.post(`${state.fixtureBaseURL}/__fixture__/provider/reset`);
    expect(reset.status()).toBe(200);

    await login(page);
    await navigateToConsole(page, 'players', 'resource--players');
    await waitForPageReady(page);
    await waitForTable(page);

    const row1 = page.locator('tbody tr').filter({ hasText: 'p-001' });
    await expect(row1).toHaveCount(1);
    await row1.getByRole('button', { name: '编辑' }).click();

    const dialog = page.getByRole('dialog');
    await expect(dialog).toBeVisible();

    // 更新表单不得暴露 identity 输入。
    await expect(dialog.getByRole('textbox', { name: /^id$/i })).toHaveCount(0);
    await expect(dialog.getByRole('spinbutton', { name: /^id$/i })).toHaveCount(0);

    const nameInput = dialog.getByRole('textbox', { name: /name/i }).first();
    await expect(nameInput).toBeVisible();
    await nameInput.fill('Ada Updated');

    const updateExecute = page.waitForResponse(
      (response) =>
        response.url().includes('/bindings/update/execute') &&
        response.request().method() === 'POST',
    );
    await dialog.getByRole('button', { name: /确|OK/ }).first().click();
    const updateResponse = await updateExecute;
    expect(updateResponse.status()).toBe(200);

    // provider 只收到对选中 identity 的更新，payload 仅含可编辑字段。
    const callsResponse = await request.get(`${state.fixtureBaseURL}/__fixture__/provider/calls`);
    const callsBody = (await callsResponse.json()) as {
      calls?: Array<{ method: string; path: string; body?: unknown }>;
    };
    const updateCalls = (callsBody.calls ?? []).filter(
      (call) => call.method === 'PUT' && call.path === '/players/p-001',
    );
    expect(updateCalls).toHaveLength(1);
    const putBody = updateCalls[0].body as Record<string, unknown>;
    const putKeys = Object.keys(putBody).sort();
    expect(putKeys).toEqual(['level', 'name']);
    expect(putBody.name).toBe('Ada Updated');

    // 刷新后页面显示 provider 的更新值。
    const updatedRow = page.locator('tbody tr').filter({ hasText: 'p-001' });
    await expect(updatedRow).toHaveCount(1);
    await expect(updatedRow).toContainText('Ada Updated');
  });

  test('@openapi-delete-confirm 删除经确认后移除记录', async ({ page, request }) => {
    const headers = await openapiAuthenticatedHeaders(request);
    await ensurePlayersSourceBound(request, headers);
    await ensurePlayersResourcePublished(request, headers);
    const state = readRealFixtureState();

    const reset = await request.post(`${state.fixtureBaseURL}/__fixture__/provider/reset`);
    expect(reset.status()).toBe(200);

    await login(page);
    await navigateToConsole(page, 'players', 'resource--players');
    await waitForPageReady(page);
    await waitForTable(page);

    const row2 = page.locator('tbody tr').filter({ hasText: 'p-002' });
    await expect(row2).toHaveCount(1);

    // 删除必须先展示生成的确认信息。
    await row2.getByRole('button', { name: '删除' }).click();
    const confirm = page.locator('.ant-popconfirm').first();
    await expect(confirm).toBeVisible();
    await expect(confirm).toContainText(/确认|删除/);

    const deleteExecute = page.waitForResponse(
      (response) =>
        response.url().includes('/bindings/delete/execute') &&
        response.request().method() === 'POST',
    );
    await confirm.getByRole('button', { name: /确|OK|是/ }).click();
    const deleteResponse = await deleteExecute;
    expect(deleteResponse.status()).toBe(200);

    // provider 只删除选中 identity。
    const callsResponse = await request.get(`${state.fixtureBaseURL}/__fixture__/provider/calls`);
    const callsBody = (await callsResponse.json()) as {
      calls?: Array<{ method: string; path: string }>;
    };
    const deleteCalls = (callsBody.calls ?? []).filter((call) => call.method === 'DELETE');
    expect(deleteCalls).toEqual([{ method: 'DELETE', path: '/players/p-002' }]);

    // 列表刷新后该记录消失，其他记录仍在。
    await expect(page.locator('tbody tr').filter({ hasText: 'p-002' })).toHaveCount(0);
    await expect(page.locator('tbody tr').filter({ hasText: 'p-001' })).toHaveCount(1);
  });

  test('@openapi-row-action row action 只接收受控上下文', async ({ page, request }) => {
    const headers = await openapiAuthenticatedHeaders(request);
    await ensurePlayersSourceBound(request, headers);
    await ensurePlayersResourcePublished(request, headers);
    const state = readRealFixtureState();

    const reset = await request.post(`${state.fixtureBaseURL}/__fixture__/provider/reset`);
    expect(reset.status()).toBe(200);

    await login(page);
    await navigateToConsole(page, 'players', 'resource--players');
    await waitForPageReady(page);
    await waitForTable(page);

    // kick 是 row action：按钮位于行内而非工具栏。
    const row1 = page.locator('tbody tr').filter({ hasText: 'p-001' });
    await expect(row1).toHaveCount(1);
    const kickBtn = row1.getByRole('button', { name: /kick/i });
    await expect(kickBtn).toBeVisible();

    const kickExecute = page.waitForResponse(
      (response) =>
        response.url().includes('/bindings/action.kick/execute') &&
        response.request().method() === 'POST',
    );
    await kickBtn.click();
    const kickResponse = await kickExecute;

    // 浏览器请求不得提交 functionId/target/game/env。
    const requestBody = kickResponse.request().postData() ?? '';
    for (const forbidden of ['functionId', '"target"', 'gameId', '"game"', '"env"']) {
      expect(requestBody).not.toContain(forbidden);
    }
    expect(kickResponse.status()).toBe(200);

    // audit 记录 published binding 与执行上下文（须在随后的列表刷新覆盖前读取）。
    const auditResponse = await request.get(
      `${state.fixtureBaseURL}/__fixture__/audit/page-execute`,
    );
    expect(auditResponse.status()).toBe(200);
    const audit = (await auditResponse.json()) as {
      eventType: string;
      outcome: string;
      gameId: string;
      env: string;
      details: Record<string, unknown>;
    };
    expect(audit).toMatchObject({
      eventType: 'page.execute',
      outcome: 'success',
      gameId: state.gameId,
      env: state.env,
      details: {
        page_key: 'resource--players',
        binding_id: 'action.kick',
        function_id: 'players.player.kick',
      },
    });

    // provider 收到 published binding 派生的 identity 调用。
    const callsResponse = await request.get(`${state.fixtureBaseURL}/__fixture__/provider/calls`);
    const callsBody = (await callsResponse.json()) as {
      calls?: Array<{ method: string; path: string }>;
    };
    const kickCalls = (callsBody.calls ?? []).filter(
      (call) => call.method === 'POST' && call.path.endsWith('/kick'),
    );
    expect(kickCalls).toEqual([
      expect.objectContaining({ method: 'POST', path: '/players/p-001/kick' }),
    ]);

    // 结果反馈到页面。
    await expect(page.locator('.ant-message-success')).toBeVisible();
  });
});
