/** T9 编辑器内绑定抽屉覆盖：
 * 1. unboundFunctionIdForOperation 与服务端确定性映射逐字符对齐（锁定防漂移：
 *    internal/function/openapi.DeriveFunctionID + internal/api/openapi.unboundFunctionID）；
 * 2. decideBindOutcome：同名 refresh（契约原地翻转）/ 不同名 swap（切换组件引用）；
 * 3. 抽屉打开：溯源单命中 → 来源/操作预填（含 method/path 标签），函数候选 =
 *    bound 描述符 + 运行时 provider 独有（unbound 物料与重复项排除）；
 * 4. 绑定保存：bindOpenAPISourceProvider 参数（bindingId 缺省 operationId）→
 *    onBound 回调 → onClose；
 * 5. 未选函数保存 → 警告拦截（不发请求）；
 * 6. 溯源失败：warning + 手动选择来源/操作后可保存。 */
import React from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App } from 'antd';
import BindingDrawer, { decideBindOutcome } from '../BindingDrawer';
import { unboundFunctionIdForOperation } from '../unboundTrace';
import {
  bindOpenAPISourceProvider,
  getOpenAPISource,
  listOpenAPISources,
  listRuntimeSources,
  type OpenAPISourceOperation,
  type OpenAPISourceSummary,
  type RuntimeProviderItem,
} from '@/services/api/openapi';
import type { FunctionDescriptor } from '@/services/api/functions';

jest.mock('@/services/api/openapi', () => ({
  __esModule: true,
  bindOpenAPISourceProvider: jest.fn(),
  getOpenAPISource: jest.fn(),
  listOpenAPISources: jest.fn(),
  listRuntimeSources: jest.fn(),
}));

const mockedBind = bindOpenAPISourceProvider as unknown as jest.Mock;
const mockedGetSource = getOpenAPISource as unknown as jest.Mock;
const mockedListSources = listOpenAPISources as unknown as jest.Mock;
const mockedListRuntime = listRuntimeSources as unknown as jest.Mock;

const op = (extra: Partial<OpenAPISourceOperation>): OpenAPISourceOperation => ({
  operationId: 'listPlayers',
  method: 'get',
  path: '/players',
  approval: { required: false },
  bound: false,
  ...extra,
});
const summary = (sourceId: string, name: string): OpenAPISourceSummary =>
  ({
    sourceId,
    name,
    revision: 1,
    format: 'json',
    openapiVersion: '3.0.3',
    contentHash: 'h',
    operationCount: 1,
    diagnosticCount: 0,
    createdAt: '2026-09-15T00:00:00Z',
    updatedAt: '2026-09-15T00:00:00Z',
  }) as OpenAPISourceSummary;
const runtimeProvider = (): RuntimeProviderItem => ({
  providerId: 'provider:ops',
  name: 'ops',
  agentId: 'agent-1',
  gameId: 'demo',
  env: 'dev',
  functionCount: 1,
  functions: ['runtime.only'],
  lastSeenUnix: 0,
});

const allFns: FunctionDescriptor[] = [
  { id: 'players.list', executionState: 'bound' },
  { id: 'listplayers', executionState: 'unbound' },
];

function renderDrawer(props?: { functionId?: string }) {
  const onClose = jest.fn();
  const onBound = jest.fn();
  render(
    <App>
      <BindingDrawer
        open
        functionId={props?.functionId ?? 'listplayers'}
        allFns={allFns}
        onClose={onClose}
        onBound={onBound}
      />
    </App>,
  );
  return { onClose, onBound };
}

/** 打开第 n 个 Select 并点击选项文本。选项 label 在 dropdown 内渲染两个
 *  元素（空 className + option-content），getAllByText 上溯 .ant-select-item-option
 *  后两者指向同一节点；不做 dropdown 容器限定——连续操作时上一个 dropdown
 *  退出动画未完仍是可见 DOM，容器定位会拿错。 */
function pickOption(comboboxIndex: number, optionText: string | RegExp): void {
  fireEvent.mouseDown(screen.getAllByRole('combobox')[comboboxIndex]);
  const option = screen
    .getAllByText(optionText)
    .map((el) => el.closest('.ant-select-item-option'))
    .find((el): el is HTMLElement => el !== null);
  if (!option) throw new Error(`option not found: ${String(optionText)}`);
  fireEvent.click(option);
}

/** 断言 dropdown 中存在选项文本（限定 .ant-select-item-option 内）。 */
const hasOption = (optionText: string | RegExp) =>
  screen.getAllByText(optionText).some((el) => el.closest('.ant-select-item-option') !== null);

beforeEach(() => {
  jest.clearAllMocks();
  mockedGetSource.mockResolvedValue({
    source: { operations: [op({})] },
  });
  mockedListSources.mockResolvedValue({ items: [summary('src-1', '玩家服务')] });
  mockedListRuntime.mockResolvedValue({ items: [runtimeProvider()], total: 1 });
  mockedBind.mockResolvedValue({ binding: { bindingId: 'listPlayers' } });
});

describe('unboundFunctionIdForOperation（服务端映射锁定）', () => {
  it('operationId 优先：camelCase 折叠、非法字符替换、首尾修剪、fn- 前缀', () => {
    expect(unboundFunctionIdForOperation('listPlayers', '/x')).toBe('listplayers');
    expect(unboundFunctionIdForOperation('Player.List_9', undefined)).toBe('player.list_9');
    expect(unboundFunctionIdForOperation('list Players', undefined)).toBe('list-players');
    expect(unboundFunctionIdForOperation('-9lives', undefined)).toBe('9lives');
    expect(unboundFunctionIdForOperation('_list', undefined)).toBe('list');
    expect(unboundFunctionIdForOperation('.list', undefined)).toBe('list');
    expect(unboundFunctionIdForOperation('.-List', undefined)).toBe('list');
    // 全非法字符 → 空串（服务端跳过建契约）
    expect(unboundFunctionIdForOperation('玩家查询', undefined)).toBe('');
  });

  it('无 operationId：path 段点接 + 归一；兜底 unknown.function', () => {
    expect(unboundFunctionIdForOperation(undefined, '/api/players/{id}')).toBe('api.players.-id');
    expect(unboundFunctionIdForOperation(undefined, '/api/players/{id}/')).toBe('api.players.-id');
    expect(unboundFunctionIdForOperation(undefined, '/Players/')).toBe('players');
    expect(unboundFunctionIdForOperation(' ', '/x')).toBe('x');
    expect(unboundFunctionIdForOperation(undefined, undefined)).toBe('unknown.function');
    expect(unboundFunctionIdForOperation(undefined, '')).toBe('unknown.function');
    // path 仅 "/"：Go Split 得 [""] → join 空串 → 归一空
    expect(unboundFunctionIdForOperation(undefined, '/')).toBe('');
  });
});

describe('decideBindOutcome（同名翻转 / 不同名切换）', () => {
  it('同名 → refresh；不同名/缺省 → swap', () => {
    expect(decideBindOutcome('listplayers', 'listplayers')).toBe('refresh');
    expect(decideBindOutcome('listplayers', 'players.list')).toBe('swap');
    expect(decideBindOutcome(undefined, 'players.list')).toBe('swap');
  });
});

describe('抽屉打开（溯源命中）', () => {
  it('预填来源操作（GET /players 标签）；函数候选排除 unbound 物料、含运行时独有', async () => {
    renderDrawer();
    expect(await screen.findByText('绑定运行时执行器')).toBeInTheDocument();
    expect(await screen.findByText('listplayers')).toBeInTheDocument(); // 标题 code
    // 操作预填：来源匹配标注 + method/path 标签
    await waitFor(() => expect(screen.getByText(/listPlayers（来源匹配）/)).toBeInTheDocument());
    expect(screen.getByText('GET /players')).toBeInTheDocument();
    // 候选：bound 描述符 + 运行时独有；unbound 物料（listplayers）不在候选中
    //（标题 code 也有 listplayers，断言限定在 option 节点内）
    fireEvent.mouseDown(screen.getAllByRole('combobox')[2]);
    expect(hasOption('players.list')).toBe(true);
    expect(hasOption(/runtime\.only/)).toBe(true);
    expect(hasOption(/agent-1/)).toBe(true);
    expect(
      screen.getAllByText(/^listplayers$/).every((el) => !el.closest('.ant-select-item-option')),
    ).toBe(true);
  });

  it('多来源时允许改选来源：切换后操作清空、选项跟随新来源', async () => {
    mockedListSources.mockResolvedValue({
      items: [summary('src-a', '服务A'), summary('src-b', '服务B')],
    });
    // src-a 命中（listPlayers → listplayers）；src-b 不命中
    mockedGetSource.mockImplementation((sourceId: string) =>
      Promise.resolve({
        source: {
          operations:
            sourceId === 'src-a' ? [op({})] : [op({ operationId: 'otherOp', path: '/other' })],
        },
      }),
    );
    renderDrawer({ functionId: 'listplayers' });
    await waitFor(() => expect(screen.getByText(/listPlayers（来源匹配）/)).toBeInTheDocument());
    // 改选 src-b → 操作清空，选项展示 otherOp
    pickOption(0, /服务B/);
    fireEvent.mouseDown(screen.getAllByRole('combobox')[1]);
    await waitFor(() => expect(hasOption('otherOp')).toBe(true));
  });
});

describe('绑定保存', () => {
  it('选函数保存 → bindOpenAPISourceProvider 参数 + onBound + onClose', async () => {
    const { onClose, onBound } = renderDrawer();
    await waitFor(() => expect(screen.getByText(/listPlayers（来源匹配）/)).toBeInTheDocument());
    pickOption(2, 'players.list');
    fireEvent.click(screen.getByRole('button', { name: '保存绑定' }));
    await waitFor(() =>
      expect(mockedBind).toHaveBeenCalledWith('src-1', {
        operationId: 'listPlayers',
        functionId: 'players.list',
        providerId: undefined,
        bindingId: 'listPlayers',
      }),
    );
    await waitFor(() => expect(onBound).toHaveBeenCalledWith('players.list'));
    await waitFor(() => expect(onClose).toHaveBeenCalled());
  });

  it('未选函数 → 警告拦截，不发请求', async () => {
    const { onClose } = renderDrawer();
    await waitFor(() => expect(screen.getByText(/listPlayers（来源匹配）/)).toBeInTheDocument());
    fireEvent.click(screen.getByRole('button', { name: '保存绑定' }));
    expect(await screen.findByText('请选择来源操作与运行时函数')).toBeInTheDocument();
    expect(mockedBind).not.toHaveBeenCalled();
    expect(onClose).not.toHaveBeenCalled();
  });

  it('保存失败 → 错误 toast（服务端 message），不回调/不关闭', async () => {
    mockedBind.mockRejectedValue(
      Object.assign(new Error('Request failed with status 400'), {
        data: {
          error: 'bad_request',
          message: 'functionId is not registered in current game/env runtime',
        },
      }),
    );
    const { onClose, onBound } = renderDrawer();
    await waitFor(() => expect(screen.getByText(/listPlayers（来源匹配）/)).toBeInTheDocument());
    pickOption(2, 'players.list');
    fireEvent.click(screen.getByRole('button', { name: '保存绑定' }));
    expect(
      await screen.findByText(/functionId is not registered in current game\/env runtime/),
    ).toBeInTheDocument();
    expect(onBound).not.toHaveBeenCalled();
    expect(onClose).not.toHaveBeenCalled();
  });
});

describe('溯源失败（手动兜底）', () => {
  it('无命中 → warning + 手动选择来源/操作后可保存', async () => {
    renderDrawer({ functionId: 'ghost.fn' });
    expect(
      await screen.findByText('未在 OpenAPI Sources 中找到该函数的来源操作'),
    ).toBeInTheDocument();
    // 手动选来源与操作（未命中无「来源匹配」标注，label 即 operationId）
    pickOption(0, /玩家服务/);
    pickOption(1, 'listPlayers');
    pickOption(2, 'players.list');
    fireEvent.click(screen.getByRole('button', { name: '保存绑定' }));
    await waitFor(() =>
      expect(mockedBind).toHaveBeenCalledWith('src-1', {
        operationId: 'listPlayers',
        functionId: 'players.list',
        providerId: undefined,
        bindingId: 'listPlayers',
      }),
    );
  });
});
