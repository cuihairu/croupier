/**
 * 场景 4: 独立操作 - OperationPage 完整功能
 */

import { test, expect } from '@playwright/test';
import type { APIRequestContext, Page } from '@playwright/test';
import { readRealFixtureState } from './helpers/realFixture';
import { login, navigateToConsole, waitForPageReady, expectFormVisible } from './helpers';

type ProposalDTO = {
  proposalKey: string;
  pageKey: string;
  pageType: string;
  resourceKey?: string;
  quality: string;
  status: string;
  pageSpec?: {
    category?: { key?: string };
  };
};

type AuthenticatedAPI = {
  baseURL: string;
  headers: Record<string, string>;
};

type PublishedOperation = {
  page?: {
    pageKey: string;
    bindingContracts?: Array<{
      bindingId: string;
      functionId: string;
      functionVersion?: string;
      inputSchemaDigest?: string;
      outputSchemaDigest?: string;
      rendererSchemaVersion?: string;
    }>;
  };
};

const scopeHeaders = (): Record<string, string> => {
  const state = readRealFixtureState();
  return { 'X-Game-ID': state.gameId, 'X-Env': state.env };
};

async function browserAPIHeaders(page: Page): Promise<Record<string, string>> {
  const token = await page.evaluate(() => localStorage.getItem('token') || '');
  expect(token).toMatch(/^eyJ/);
  return {
    ...scopeHeaders(),
    Authorization: `Bearer ${token}`,
  };
}

async function authenticatedAPI(request: APIRequestContext): Promise<AuthenticatedAPI> {
  const state = readRealFixtureState();
  const response = await request.post(`${state.serverBaseURL}/api/v1/auth/login`, {
    data: { username: 'admin', password: 'admin123' },
  });
  expect(response.status()).toBe(200);
  const session = (await response.json()) as { token?: string };
  expect(session.token).toMatch(/^eyJ/);
  return {
    baseURL: state.serverBaseURL,
    headers: {
      ...scopeHeaders(),
      Authorization: `Bearer ${session.token}`,
    },
  };
}

async function ensureMailOperationPublished(request: APIRequestContext): Promise<void> {
  const api = await authenticatedAPI(request);
  const pageURL = `${api.baseURL}/api/v1/console/pages/operation--mail.send`;
  const current = await request.get(pageURL, { headers: api.headers });
  if (current.status() === 200) return;
  expect(current.status()).toBe(404);

  const publish = await request.post(
    `${api.baseURL}/api/v1/proposals/operation%3Amail.send/accept-and-publish`,
    { headers: api.headers },
  );
  expect(publish.status()).toBe(200);

  const published = await request.get(pageURL, { headers: api.headers });
  expect(published.status()).toBe(200);
}

async function expectMailOperationPublished(page: Page): Promise<void> {
  const headers = await browserAPIHeaders(page);
  const publishedResponse = await page.request.get('/api/v1/console/pages/operation--mail.send', {
    headers,
  });
  expect(publishedResponse.status()).toBe(200);
  const published = (await publishedResponse.json()) as PublishedOperation;
  expect(published.page?.pageKey).toBe('operation--mail.send');
  expect(published.page?.bindingContracts).toHaveLength(1);
  expect(published.page?.bindingContracts?.[0]).toMatchObject({
    bindingId: 'mail.send.main',
    functionId: 'mail.send',
    functionVersion: '1.0.0',
    rendererSchemaVersion: 'page-spec:1',
  });
  expect(published.page?.bindingContracts?.[0].inputSchemaDigest).toMatch(/^[a-f0-9]{64}$/);
  expect(published.page?.bindingContracts?.[0].outputSchemaDigest).toMatch(/^[a-f0-9]{64}$/);
}

test.describe('独立操作', () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
  });

  test('OperationPage 表单加载', async ({ page }) => {
    await navigateToConsole(page, 'mail', 'operation--mail.send');
    await waitForPageReady(page);

    // 验证表单渲染
    await expectFormVisible(page);
  });

  test('填写表单并执行', async ({ page }) => {
    await navigateToConsole(page, 'mail', 'operation--mail.send');
    await waitForPageReady(page);

    // 验证表单存在
    await expectFormVisible(page);

    // 填写表单
    const toInput = page.getByLabel(/^收件人|收件/i).first();
    await expect(toInput).toBeVisible();
    await toInput.fill('test@example.com');

    const contentInput = page.getByLabel(/内容|正文/i).first();
    await expect(contentInput).toBeVisible();
    await contentInput.fill('测试邮件内容');

    // 点击执行按钮
    const submitBtn = page.getByRole('button', { name: /发\s*送|执\s*行|提\s*交/ }).first();
    await expect(submitBtn).toBeVisible();
    await submitBtn.click();

    const confirmBtn = page.getByRole('dialog').getByRole('button', { name: /确\s*定|确\s*认/ });
    await expect(confirmBtn).toBeVisible();
    const executeResponse = page.waitForResponse(
      (response) =>
        response.url().includes('/bindings/main/execute') && response.request().method() === 'POST',
    );
    await confirmBtn.click();
    expect((await executeResponse).status()).toBe(200);

    await expect(page.getByText('操作成功', { exact: true }).first()).toBeVisible();
    await expect(page.getByText('执行结果')).toBeVisible();
  });

  test('高风险操作确认', async ({ page }) => {
    await navigateToConsole(page, 'system', 'operation--system.dangerous-op');
    await waitForPageReady(page);

    // 验证表单存在
    await expectFormVisible(page);

    // 填写表单
    const reasonInput = page.getByLabel(/原因|理由/i).first();
    await expect(reasonInput).toBeVisible();
    await reasonInput.fill('测试原因');

    // 点击执行
    const submitBtn = page.getByRole('button', { name: /执\s*行|提\s*交/ }).first();
    await expect(submitBtn).toBeVisible();
    await submitBtn.click();

    // 确认弹窗应该出现
    const confirmBtn = page
      .getByRole('dialog')
      .getByRole('button', { name: /确\s*定|确\s*认|OK/ })
      .first();
    await expect(confirmBtn).toBeVisible({ timeout: 5000 });
    const executeResponse = page.waitForResponse(
      (response) => response.url().includes('/bindings/') && response.request().method() === 'POST',
    );
    await confirmBtn.click();
    expect((await executeResponse).status()).toBe(200);
    await expect(page.getByText(/操作成功|等待审批|任务已提交/).first()).toBeVisible();
  });
});

test.describe('真实 SDK Operation 链路', () => {
  test('@sdk-unannotated-proposal 仅生成 standalone Operation Proposal', async ({ request }) => {
    const api = await authenticatedAPI(request);
    const response = await request.get(`${api.baseURL}/api/v1/proposals`, {
      headers: api.headers,
    });
    expect(response.status()).toBe(200);
    const proposals = (await response.json()) as ProposalDTO[];

    const operationProposals = proposals.filter(
      (item) => item.proposalKey === 'operation:mail.send',
    );
    expect(operationProposals).toHaveLength(1);
    expect(operationProposals[0]).toMatchObject({
      pageKey: 'operation--mail.send',
      pageType: 'operation',
      quality: 'basic',
      status: 'pending',
    });
    expect(proposals.some((item) => item.proposalKey === 'resource:mail')).toBe(false);
  });

  test('@sdk-operation-publish 从 Inbox 预览并发布冻结快照', async ({ page }) => {
    // 套件中 contract-change 等先行 spec 可能已发布该页面；先经 unpublish 路由
    // 回到未发布状态，保证本测试验证完整的 Inbox 发布 UI 流程。
    const api = await authenticatedAPI(page.request);
    const existing = await page.request.get(
      `${api.baseURL}/api/v1/console/pages/operation--mail.send`,
      { headers: api.headers },
    );
    if (existing.status() === 200) {
      const unpublish = await page.request.post(
        `${api.baseURL}/api/v1/pages/operation--mail.send/unpublish`,
        { headers: api.headers, data: {} },
      );
      expect(unpublish.status()).toBe(200);
    }

    await login(page);
    await page.goto('/functions/pages');
    await waitForPageReady(page);

    const proposalRow = page.locator('tr').filter({ hasText: 'operation:mail.send' });
    await expect(proposalRow).toHaveCount(1);
    const previewResponsePromise = page.waitForResponse(
      (response) =>
        decodeURIComponent(new URL(response.url()).pathname).endsWith(
          '/proposals/operation:mail.send',
        ) && response.request().method() === 'GET',
    );
    await proposalRow.getByRole('button', { name: '预览' }).click();
    expect((await previewResponsePromise).status()).toBe(200);
    await expect(page.getByRole('dialog').getByText('默认页面预览')).toBeVisible();
    await expect(page.getByRole('dialog').getByText('Send an in-game mail')).toBeVisible();
    await page.getByRole('dialog').getByRole('button', { name: 'Close' }).click();

    await proposalRow.getByRole('button', { name: '发布' }).click();
    const confirm = page.locator('.ant-popconfirm .ant-btn-primary');
    await expect(confirm).toBeVisible();
    const publishResponsePromise = page.waitForResponse(
      (response) =>
        decodeURIComponent(new URL(response.url()).pathname).endsWith(
          '/proposals/operation:mail.send/accept-and-publish',
        ) && response.request().method() === 'POST',
    );
    await confirm.click();
    const publishResponse = await publishResponsePromise;
    expect(publishResponse.status()).toBe(200);
    const successDialog = page.getByRole('dialog').filter({ hasText: '已直接发布' });
    await expect(successDialog).toHaveCount(1);
    await expect(
      successDialog.getByText(/页面 operation--mail\.send 已发布，版本 \d+/),
    ).toBeVisible();
    await expectMailOperationPublished(page);
  });

  test('@sdk-operation-menu 菜单只来自已发布 PageSpec', async ({ page }) => {
    await ensureMailOperationPublished(page.request);
    await login(page);
    const headers = await browserAPIHeaders(page);
    const menuResponse = await page.request.get('/api/v1/console/menu', { headers });
    expect(menuResponse.status()).toBe(200);
    const menu = (await menuResponse.json()) as {
      items?: Array<{ key: string; children?: Array<{ key: string; path: string }> }>;
    };
    const menuItems = menu.items?.flatMap((category) => category.children || []) || [];
    expect(menuItems.filter((item) => item.key === 'operation--mail.send')).toHaveLength(1);
    const mailItem = menuItems.find((item) => item.key === 'operation--mail.send');
    expect(mailItem?.path).toBe('/console/mail/operation--mail.send');

    await page.goto(mailItem!.path);
    await waitForPageReady(page);
    await expect(page.getByText('Send an in-game mail', { exact: true }).first()).toBeVisible();
    await expect(page.getByText('operation--mail.send', { exact: true })).toBeVisible();
    await expect(page.getByRole('textbox', { name: /player[ _-]?id/i })).toBeVisible();
    await expectFormVisible(page);
  });

  test('@sdk-operation-execute 通过 published binding 执行并渲染结果', async ({ page }) => {
    const state = readRealFixtureState();
    await ensureMailOperationPublished(page.request);
    await page.request.delete(`${state.fixtureBaseURL}/__fixture__/sdk/calls`);
    await login(page);
    await navigateToConsole(page, 'mail', 'operation--mail.send');
    await waitForPageReady(page);

    await page.getByRole('textbox', { name: /player[ _-]?id/i }).fill('p-001');
    await page.getByRole('textbox', { name: /title/ }).fill('真实 SDK 邮件');
    await page.getByRole('textbox', { name: 'content' }).fill('fixture message');
    const executeResponsePromise = page.waitForResponse(
      (response) =>
        response.url().includes('/bindings/mail.send.main/execute') &&
        response.request().method() === 'POST',
    );
    await page.getByRole('button', { name: /提\s*交/ }).click();
    const executeResponse = await executeResponsePromise;
    expect(executeResponse.status()).toBe(200);

    await expect(page.getByText('操作成功', { exact: true }).first()).toBeVisible();
    await expect(page.getByText('mail-0001')).toBeVisible();

    const callsResponse = await page.request.get(`${state.fixtureBaseURL}/__fixture__/sdk/calls`);
    expect(callsResponse.status()).toBe(200);
    const calls = (await callsResponse.json()) as {
      calls: Array<{ functionId: string; payload: Record<string, unknown> }>;
    };
    expect(calls.calls).toHaveLength(1);
    expect(calls.calls[0]).toEqual({
      functionId: 'mail.send',
      payload: {
        player_id: 'p-001',
        title: '真实 SDK 邮件',
        content: 'fixture message',
      },
    });

    const auditResponse = await page.request.get(
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
        page_key: 'operation--mail.send',
        binding_id: 'mail.send.main',
        function_id: 'mail.send',
        result_kind: 'sync',
      },
    });
  });
});
