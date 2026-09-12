/** U11 模板更新提醒（登记三入口 + 保存并入 + 打开比对提示）。
 *
 * 1. 快照登记三入口：「从模板开始」onPick / 组件库 onInsert /
 *    拖拽与带参弹窗共用的 applyTemplateInsert（onTemplateUsed）；
 * 2. 保存把快照并入 POST body componentTemplates（同 key 重复插入去重）；
 * 3. 打开旧页面时与当前模板库比对：digest 不一致 → 顶部 Alert（只提示
 *    不自动同步）；一致 / 旧快照无 digest → 不提示。
 *
 * 本文件覆写 tests/setupTests.jsx 的 @umijs/max mock（其余导出同语义），
 * 差异：useSearchParams 读 globalThis.__TEST_PAGE_KEY__（支持 ?pageKey= 回读场景）。 */
import React from 'react';
import { act, fireEvent, render, renderHook, screen, waitFor } from '@testing-library/react';
import { App } from 'antd';
import { request } from '@umijs/max';
import CompositeEditorPage from '../index';
import { useCanvasDnd } from '../useCanvasDnd';
import type { ComponentTemplateDTO } from '../ComponentLibrary';
import type { PageNode } from '../model';

jest.mock('@/services/api/functions', () => ({
  invokeFunction: jest.fn(async () => ({ result: {} })),
  listDescriptors: jest.fn(async () => []),
}));

jest.mock('@umijs/max', () => {
  const ReactLib = require('react');
  const formatMessage = (
    descriptor: { defaultMessage: string },
    values?: Record<string, unknown>,
  ) =>
    Object.entries(values || {}).reduce(
      (msg: string, [key, val]) => msg.split(`{${key}}`).join(String(val)),
      descriptor.defaultMessage,
    );
  return {
    __esModule: true,
    history: { push: jest.fn(), location: { pathname: '/functions/pages' } },
    request: jest.fn(async () => ({})),
    useIntl: () => ({ formatMessage }),
    getIntl: () => ({ formatMessage }),
    FormattedMessage: ({ defaultMessage }: { defaultMessage: React.ReactNode }) =>
      ReactLib.createElement(ReactLib.Fragment, null, defaultMessage),
    useSearchParams: () => [
      new URLSearchParams(
        (globalThis as typeof globalThis & { __TEST_PAGE_KEY__?: string }).__TEST_PAGE_KEY__ ?? '',
      ),
      jest.fn(),
    ],
  };
});

const mockRequest = request as unknown as jest.Mock;

// 全页渲染（DndContext + PageContainer + 模板引导/组件库 fetch），放宽预算
jest.setTimeout(20000);
const FIND = { timeout: 5000 } as const;

interface RequestOptions {
  method?: string;
  data?: Record<string, unknown>;
}

function tpl(partial: { key: string; digest?: string; tree: PageNode[] }): ComponentTemplateDTO {
  return {
    name: { 'zh-CN': `名称-${partial.key}` },
    builtin: false,
    ...partial,
  } as ComponentTemplateDTO;
}

/** 组合模板（≥2 区块：quick-start 可列出、保存过 minSections 门禁）。 */
const comboTpl = tpl({
  key: 'combo--player',
  digest: 'digest-v1',
  tree: [
    { id: 'ct1', type: 'fnTable', props: { functionId: 'player.list', title: '玩家列表' } },
    { id: 'cf1', type: 'fnForm', props: { functionId: 'mail.send', title: '发邮件' } },
  ],
});

const libTpl = tpl({
  key: 'comp--mail',
  digest: 'digest-m1',
  tree: [
    { id: 'lt1', type: 'fnForm', props: { functionId: 'mail.send', title: '发邮件表单' } },
    { id: 'lt2', type: 'fnFields', props: { functionId: 'mail.send', title: '邮件详情' } },
  ],
});

/** 按「METHOD url / url」分流（未命中返回 undefined → 编辑器各 fetcher 自行跳过）。 */
function setRoutes(routes: Record<string, unknown>) {
  mockRequest.mockImplementation(async (url: string, opts?: RequestOptions) => {
    const method = (opts?.method ?? 'GET').toUpperCase();
    const hit = routes[`${method} ${url}`] ?? routes[url];
    return typeof hit === 'function' ? (hit as () => unknown)() : hit;
  });
}

function lastCompositePost(): Record<string, unknown> | undefined {
  const call = mockRequest.mock.calls.find(
    ([url, opts]) =>
      url === '/api/v1/versioning/pages/composite' &&
      (opts as RequestOptions | undefined)?.method === 'POST',
  );
  return (call?.[1] as RequestOptions | undefined)?.data;
}

describe('快照登记三入口 + 保存并入（U11）', () => {
  beforeEach(() => {
    mockRequest.mockReset();
    mockRequest.mockImplementation(async () => ({}));
  });

  it('「从模板开始」点击组合模板登记 key+digest，保存并入 body.componentTemplates', async () => {
    setRoutes({
      '/api/v1/component-templates': { items: [comboTpl] },
      'POST /api/v1/versioning/pages/composite': { proposalKey: 'composite--ok' },
    });
    render(
      <App>
        <CompositeEditorPage />
      </App>,
    );
    // 引导面板（中栏）与左栏组件库同名（组件库 pane 懒挂载，实际通常仅引导面板一处）
    const cards = await screen.findAllByText('名称-combo--player', undefined, FIND);
    fireEvent.click(cards[0]);
    // 画布实例化出模板节点（fnTable 卡片头）后再保存
    await screen.findByText('玩家列表', undefined, FIND);
    // antd 图标 aria-label（save）拼进 accessible name，用正则尾匹配
    fireEvent.click(screen.getByRole('button', { name: /保存为提案$/ }));
    await waitFor(() =>
      expect(lastCompositePost()?.componentTemplates).toEqual([
        { key: 'combo--player', digest: 'digest-v1' },
      ]),
    );
  });

  it('组件库点击插入同样登记；同模板重复插入不重复登记（按 key 去重）', async () => {
    setRoutes({
      '/api/v1/component-templates': { items: [libTpl] },
      'POST /api/v1/versioning/pages/composite': { proposalKey: 'composite--ok' },
    });
    render(
      <App>
        <CompositeEditorPage />
      </App>,
    );
    // 空白起步：引导面板卸载，组件库模板名唯一
    fireEvent.click(await screen.findByRole('button', { name: '从空白开始' }, FIND));
    // rc-tabs 懒挂载：显式点击「组件库」tab 确保面板挂载并拉取
    fireEvent.click(screen.getByRole('tab', { name: '组件库' }));
    const card = await screen.findByText('名称-comp--mail', undefined, FIND);
    fireEvent.click(card);
    fireEvent.click(card);
    // 两次插入 → 画布出现同名表单卡片多处（卡片头/属性面板），存在即视为已插入
    await screen.findAllByText('发邮件表单', undefined, FIND);
    fireEvent.click(screen.getByRole('button', { name: /保存为提案$/ }));
    await waitFor(() =>
      expect(lastCompositePost()?.componentTemplates).toEqual([
        { key: 'comp--mail', digest: 'digest-m1' },
      ]),
    );
  });
});

describe('拖拽/带参弹窗共用入口（useCanvasDnd.onTemplateUsed）', () => {
  function setupHook(tplDto: ComponentTemplateDTO) {
    const onTemplateUsed = jest.fn();
    const setTree = jest.fn();
    const utils = {
      addChild: jest.fn(),
      registerFn: jest.fn(),
      setTree,
      setSelectedId: jest.fn(),
      setInsertTpl: jest.fn(),
    };
    const hook = renderHook(
      () =>
        useCanvasDnd({
          treeRef: { current: [] as PageNode[] },
          editingModalRef: { current: null },
          allFns: [],
          ...utils,
          onTemplateUsed,
        }),
      { wrapper: ({ children }: { children: React.ReactNode }) => <App>{children}</App> },
    );
    return { hook, onTemplateUsed, setTree };
  }

  it('插入成功后回调原模板（登记快照）', () => {
    const { hook, onTemplateUsed, setTree } = setupHook(libTpl);
    act(() => hook.result.current.applyTemplateInsert(libTpl, {}, 'canvas-root'));
    expect(onTemplateUsed).toHaveBeenCalledTimes(1);
    expect(onTemplateUsed).toHaveBeenCalledWith(libTpl);
    expect(setTree).toHaveBeenCalled();
  });

  it('模板无区块（空树）不插入 → 不回调（不虚登记）', () => {
    const emptyTpl = tpl({ key: 'combo--empty', digest: 'digest-e', tree: [] });
    const { hook, onTemplateUsed } = setupHook(emptyTpl);
    act(() => hook.result.current.applyTemplateInsert(emptyTpl, {}, 'canvas-root'));
    expect(onTemplateUsed).not.toHaveBeenCalled();
  });
});

describe('打开页面比对当前模板库（U11 提醒条）', () => {
  const sections = [
    { key: 'player.list', functionId: 'player.list', view: 'table', autoRun: true },
    { key: 'mail.send', functionId: 'mail.send', view: 'fields' },
  ];

  function renderLoadedPage(componentTemplates: Array<{ key: string; digest: string }>) {
    setRoutes({
      '/api/v1/versioning/pages/stale-page': {
        pageSpec: { composite: { sections }, componentTemplates },
      },
      '/api/v1/component-templates': { items: [comboTpl] },
    });
    (globalThis as typeof globalThis & { __TEST_PAGE_KEY__?: string }).__TEST_PAGE_KEY__ =
      'pageKey=stale-page';
    return render(
      <App>
        <CompositeEditorPage />
      </App>,
    );
  }

  afterEach(() => {
    delete (globalThis as typeof globalThis & { __TEST_PAGE_KEY__?: string }).__TEST_PAGE_KEY__;
    mockRequest.mockReset();
    mockRequest.mockImplementation(async () => ({}));
  });

  it('快照 digest ≠ 当前 digest → 顶部 Alert 列出模板名（只提示不自动同步）', async () => {
    renderLoadedPage([{ key: 'combo--player', digest: 'digest-v0' }]);
    expect(await screen.findByText('所用模板有新版本', undefined, FIND)).toBeInTheDocument();
    expect(await screen.findByText(/名称-combo--player/, undefined, FIND)).toBeInTheDocument();
  });

  it('digest 一致 → 不提示', async () => {
    renderLoadedPage([{ key: 'combo--player', digest: 'digest-v1' }]);
    // 回读完成标志（成功 message）后再断言 Alert 不存在
    await screen.findByText(/已载入页面 stale-page/, undefined, FIND);
    expect(screen.queryByText('所用模板有新版本')).not.toBeInTheDocument();
  });

  it('旧页面快照无 digest → 无法判定，不提示', async () => {
    renderLoadedPage([{ key: 'combo--player', digest: '' }]);
    await screen.findByText(/已载入页面 stale-page/, undefined, FIND);
    expect(screen.queryByText('所用模板有新版本')).not.toBeInTheDocument();
  });
});
