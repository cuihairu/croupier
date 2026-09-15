/** ComponentPanel（组件面板）覆盖：基础组件网格 + 点击 onAddBasic、
 * 函数树（资源分组排序/无 resource 归「其他」/搜索过滤/叶子点击
 * onAddFunction/无匹配空态）、listDescriptors 失败静默回退 ScopeGuide、
 * 空态 scope 引导（探测各 scope 函数分布：数组/信封/失败三形态、
 * 其他 scope 有函数 → 切换链接 setScope + 重载、无 → 提示文案、
 * getMyGames 失败回退）。 */
import React from 'react';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { App } from 'antd';
import { request } from '@umijs/max';
import ComponentPanel, { type AddFnEvent } from '../ComponentPanel';
import { setScope } from '@/stores/scope';
import type { FunctionDescriptor } from '@/services/api/functions';

jest.mock('@dnd-kit/core', () => ({
  useDraggable: () => ({
    attributes: { role: 'button', tabIndex: 0 },
    listeners: {},
    setNodeRef: jest.fn(),
    isDragging: false,
  }),
}));

jest.mock('@/services/api/functions', () => ({
  ...jest.requireActual('@/services/api/functions'),
  listDescriptors: jest.fn(),
}));

jest.mock('@/services/api/me', () => ({
  ...jest.requireActual('@/services/api/me'),
  getMyGames: jest.fn(),
}));

import { listDescriptors } from '@/services/api/functions';
import { getMyGames } from '@/services/api/me';
import { resetRegistryForTest } from '../registry';
import { registerBuiltinComponents } from '../components/builtin';

const mockedList = listDescriptors as jest.Mock;
const mockedGames = getMyGames as jest.Mock;
const mockedRequest = request as unknown as jest.Mock;

beforeAll(() => {
  resetRegistryForTest();
  registerBuiltinComponents();
});

const fn = (id: string, operation: string, resource?: string): FunctionDescriptor =>
  resource ? { id, operation, resource } : { id, operation };

const fns = [fn('player.list', 'list', 'player'), fn('player.ban', 'update', 'player')];

function renderPanel(onAddBasic = jest.fn(), onAddFunction = jest.fn()) {
  render(
    <App>
      <ComponentPanel onAddBasic={onAddBasic} onAddFunction={onAddFunction} />
    </App>,
  );
  return { onAddBasic, onAddFunction };
}

beforeEach(() => {
  setScope({ gameId: 'demo', env: 'prod' });
  mockedList.mockReset().mockResolvedValue(fns);
  mockedGames.mockReset().mockResolvedValue({ games: [] });
  mockedRequest.mockReset().mockImplementation(async () => ({}));
});

describe('ComponentPanel 基础组件与函数树', () => {
  it('基础组件网格：点击回调 onAddBasic（不含常量表单）', async () => {
    const { onAddBasic } = renderPanel();
    await screen.findByText('player (2)');
    // basics：button/modal/container/tabs/text（staticForm 被排除）；
    // 按钮的 icon Tag 与组件名同为「按钮」，取第一个（点击冒泡到外层 <a>）
    fireEvent.click(screen.getAllByText('按钮')[0]);
    expect(onAddBasic).toHaveBeenCalledWith('button');
    fireEvent.click(screen.getByText('分组容器'));
    expect(onAddBasic).toHaveBeenCalledWith('container');
  });

  it('函数树：资源分组排序 + 叶子点击 onAddFunction（视图映射）', async () => {
    mockedList.mockResolvedValue([
      ...fns,
      fn('player.kick', 'kick', 'player'),
      fn('mail.sendAll', 'update', 'mail'),
    ]);
    const onAddFunction = jest.fn();
    renderPanel(jest.fn(), onAddFunction);
    // 分组按资源名排序：mail < player；计数随行
    expect(await screen.findByText('mail (1)')).toBeInTheDocument();
    expect(screen.getByText('player (3)')).toBeInTheDocument();
    // 叶子 label = operation（kick）；点击 → onAddFunction
    fireEvent.click(await screen.findByText('kick'));
    const calls = onAddFunction.mock.calls as unknown as [AddFnEvent][];
    expect(calls.some(([e]) => e.fn.id === 'player.kick' && e.componentType === 'fnForm')).toBe(
      true,
    );
    expect(calls.some(([e]) => e.fn.id === 'player.list' && e.componentType === 'fnTable')).toBe(
      false, // 本用例只点了 kick
    );
  });

  it('list 视图映射：点击走 fnTable', async () => {
    mockedList.mockResolvedValue([fn('player.query', 'list', 'player')]);
    const onAddFunction = jest.fn();
    renderPanel(jest.fn(), onAddFunction);
    fireEvent.click(await screen.findByText('list'));
    expect(onAddFunction.mock.calls[0]?.[0].componentType).toBe('fnTable');
  });

  it('叶子二次点击：直达 onAddFunction（二击 deselect 走 keys 空分支）', async () => {
    mockedList.mockResolvedValue(fns);
    const onAddFunction = jest.fn();
    renderPanel(jest.fn(), onAddFunction);
    const leaf = await screen.findByText('list');
    fireEvent.click(leaf);
    // 一次点击可同时命中 PanelDraggable onClick 与 Tree onSelect（选中），
    // 只约束「至少触发」；二次点击触发 deselect（keys 空 → fnMap 未命中）
    await waitFor(() => expect(onAddFunction.mock.calls.length).toBeGreaterThanOrEqual(2));
    fireEvent.click(leaf);
    expect(onAddFunction.mock.calls.length).toBeGreaterThanOrEqual(3);
  });

  it('搜索过滤：按函数 id（组内叶子过滤）；无匹配出空态', async () => {
    renderPanel();
    await screen.findByText('player (2)');
    fireEvent.change(screen.getByPlaceholderText('搜索函数 / 资源'), {
      target: { value: 'ban' },
    });
    // 命中 player.ban：叶子只剩它（label=operation update）；player.list（list）消失
    // 注：分组标题计数为组内总数，不随过滤变化
    await waitFor(() => expect(screen.getByText('update')).toBeInTheDocument());
    expect(screen.queryByText('list')).not.toBeInTheDocument();
    // 过滤词无匹配 → 空态
    fireEvent.change(screen.getByPlaceholderText('搜索函数 / 资源'), {
      target: { value: 'nope' },
    });
    await waitFor(() => expect(screen.getByText('无匹配函数')).toBeInTheDocument());
  });

  it('resource 与 operation 均缺省：归入「其他」分组，叶子 label 回退 id 末段', async () => {
    mockedList.mockResolvedValue([...fns, { id: 'misc.tool' } as FunctionDescriptor]);
    renderPanel();
    expect(await screen.findByText('其他 (1)')).toBeInTheDocument();
    expect(screen.getByText('tool')).toBeInTheDocument();
  });

  it('T7 unbound 物料：叶子带「未绑定」Tag（不置灰）；bound 不带', async () => {
    mockedList.mockResolvedValue([
      ...fns,
      { id: 'player.query', operation: 'get', resource: 'player', executionState: 'unbound' },
      { id: 'player.kick', operation: 'kick', resource: 'player', executionState: 'bound' },
    ]);
    const onAddFunction = jest.fn();
    renderPanel(jest.fn(), onAddFunction);
    expect(await screen.findByText('player (4)')).toBeInTheDocument();
    // unbound 叶子标注「未绑定」（仅 1 个：bound/缺省不标注）
    expect(screen.getAllByText('未绑定')).toHaveLength(1);
    // 不置灰：unbound 叶子仍可点击加入画布（点击 → onAddFunction）
    fireEvent.click(screen.getByText('get'));
    const calls = onAddFunction.mock.calls as unknown as [AddFnEvent][];
    expect(calls.some(([e]) => e.fn.id === 'player.query')).toBe(true);
    // opacity 由 isDragging 控制，与执行状态无关——Tag 只标注不弱化
  });

  it('listDescriptors 失败：静默回退空态引导（无未捕获异常）', async () => {
    mockedList.mockRejectedValue(new Error('boom'));
    const { container } = render(
      <App>
        <ComponentPanel onAddBasic={jest.fn()} onAddFunction={jest.fn()} />
      </App>,
    );
    await waitFor(() =>
      expect(container.textContent).toContain('当前 scope（demo/prod）没有函数契约'),
    );
  });
});

describe('ScopeGuide 空态 scope 引导', () => {
  beforeEach(() => {
    mockedList.mockResolvedValue([]);
    // 各 env 探测路由：prod2 数组 / stage 信封 / bad 失败 / junk functions 非数组 / 其余空
    mockedRequest.mockImplementation(
      async (url: string, opts?: { headers?: Record<string, string> }) => {
        if (typeof url === 'string' && url.includes('/api/v1/functions/descriptors')) {
          const env = opts?.headers?.['X-Env'];
          if (env === 'prod2') return [{ id: 'a' }, { id: 'b' }];
          if (env === 'stage') return { functions: [{ id: 's' }] };
          if (env === 'bad') throw new Error('boom');
          if (env === 'junk') return { functions: 'nope' };
          return {};
        }
        return {};
      },
    );
  });

  it('其他 scope 有函数：渲染切换链接，点击 setScope + 触发重载', async () => {
    mockedGames.mockResolvedValue({
      games: [
        { gameId: 'demo', envs: ['prod', 'prod2', 'stage', 'bad', 'junk'] },
        // gameId 缺省（envs 有值仍探测，结果因无 gameId 被滤除）；envs 缺省走空轮次
        { envs: ['ghost'] },
        {},
      ],
    });
    mockedList.mockResolvedValue([]).mockResolvedValueOnce([]).mockResolvedValueOnce(fns);
    renderPanel();
    // prod2（数组形态 count=2）与 stage（信封形态 count=1）在列；
    // bad（失败）与 junk（functions 非数组）count=0 被滤除；无 gameId 的游戏不产生候选
    const link2 = await screen.findByText('切换到 demo/prod2（2 函数）', undefined, {
      timeout: 5000,
    });
    expect(screen.getByText('切换到 demo/stage（1 函数）')).toBeInTheDocument();
    fireEvent.click(link2);
    // setScope 生效 + onReload → listDescriptors 重新拉取（第二次返回 fns → 树渲染）
    await waitFor(() => expect(screen.getByText('player (2)')).toBeInTheDocument(), {
      timeout: 5000,
    });
    expect(mockedList).toHaveBeenCalledTimes(2);
  });

  it('空 scope（无 gameId/env）：空态文案以 ? 占位', async () => {
    // setScope 为 merge 语义，显式传 undefined 才清键
    setScope({ gameId: undefined, env: undefined });
    renderPanel();
    await screen.findByText('当前 scope（?/?）没有函数契约', undefined, { timeout: 5000 });
  });

  it('其他 scope 也无函数：出提示文案（games 缺省兜底空表）', async () => {
    mockedGames.mockResolvedValue({});
    renderPanel();
    await screen.findByText('其他 scope 也没有函数——请先通过 SDK/OpenAPI 注册函数', undefined, {
      timeout: 5000,
    });
  });

  it('getMyGames 失败：回退空表 + 提示文案', async () => {
    mockedGames.mockRejectedValue(new Error('boom'));
    renderPanel();
    await screen.findByText('其他 scope 也没有函数——请先通过 SDK/OpenAPI 注册函数', undefined, {
      timeout: 5000,
    });
  });
});
