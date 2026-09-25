/**
 * 资源目录页面定位提示体系：PageContainer 标题/副标题、SummaryOverview
 * 概览卡（统计项随数据派生）、边界 Alert 与「进入 Page Studio」跳转。
 * 提示层是纯展示，列表/搜索行为回归只做存在性断言，交互细节由组件自身
 * 测试覆盖。
 */
import React from 'react';
import { App as AntdApp, message } from 'antd';
import { configure, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import ResourceCatalogPage from '../index';
import type {
  DiagnosticInfo,
  FunctionInfo,
  ResourceCatalogItem,
  SemanticsInfo,
} from '@/types/dashboard';

configure({ asyncUtilTimeout: 5000 });
jest.setTimeout(20000);

jest.mock('@umijs/max', () => {
  const formatMessage = jest.fn(
    ({ defaultMessage }: { defaultMessage: string }, values?: Record<string, unknown>) =>
      Object.entries(values || {}).reduce(
        (msg: string, [key, val]) => msg.split(`{${key}}`).join(String(val)),
        defaultMessage,
      ),
  );
  const intl = { formatMessage };
  return {
    __esModule: true,
    useIntl: () => intl,
    getIntl: () => intl,
    // 工厂内联创建：jest.mock 提升早于模块体 const 声明，外部引用会 TDZ 报错
    history: { push: jest.fn() },
    FormattedMessage: ({ defaultMessage }: { defaultMessage: string }) => defaultMessage,
  };
});

const umiMock = jest.requireMock('@umijs/max') as {
  history: { push: jest.Mock };
};
const mockHistoryPush = umiMock.history.push;

// jsdom 下 PageContainer 依赖 ProLayout 的 RouteContext，mock 成透传渲染，
// 同时把 title/subTitle 落到 DOM 便于断言。
jest.mock('@ant-design/pro-components', () => ({
  __esModule: true,
  PageContainer: ({
    title,
    subTitle,
    children,
  }: {
    title: React.ReactNode;
    subTitle?: React.ReactNode;
    children: React.ReactNode;
  }) => (
    <div>
      <h1>{title}</h1>
      <p data-testid="page-subtitle">{subTitle}</p>
      {children}
    </div>
  ),
}));

const mockListResourceCatalog = jest.fn();
const mockGetDetail = jest.fn();
const mockGetConflicts = jest.fn();
const mockGetVersions = jest.fn();
const mockResolveConflict = jest.fn();
const mockUpdateSemantics = jest.fn();

jest.mock('@/services/dashboard', () => ({
  __esModule: true,
  listResourceCatalog: (...args: unknown[]) => mockListResourceCatalog(...args),
  getResourceDetail: (...args: unknown[]) => mockGetDetail(...args),
  getResourceSemanticConflicts: (...args: unknown[]) => mockGetConflicts(...args),
  getResourceSemanticVersions: (...args: unknown[]) => mockGetVersions(...args),
  resolveResourceSemanticConflict: (...args: unknown[]) => mockResolveConflict(...args),
  updateResourceSemantics: (...args: unknown[]) => mockUpdateSemantics(...args),
}));

const fixtureItems = [
  {
    resourceKey: 'player',
    labels: { 'zh-CN': '玩家' },
    categoryKey: '玩家运营',
    status: 'ready',
    functions: [{ id: 'player.kick' }],
    semantics: { version: 3 },
    diagnostics: [],
  },
  {
    resourceKey: 'mail',
    labels: { 'zh-CN': '邮件' },
    categoryKey: '玩家运营',
    status: 'draft',
    functions: [{ id: 'mail.send' }],
    semantics: undefined,
    diagnostics: [{ severity: 'warning', message: 'collection missing' }],
  },
];

function renderPage() {
  return render(
    <AntdApp>
      <ResourceCatalogPage />
    </AntdApp>,
  );
}

describe('ResourceCatalogPage 定位提示体系', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockListResourceCatalog.mockResolvedValue({ items: fixtureItems, total: 2 });
  });

  it('渲染页面标题与定位副标题（页面、菜单和分类在 Page Studio 确定）', async () => {
    renderPage();

    expect(screen.getByRole('heading', { name: '资源目录' })).toBeInTheDocument();
    expect(screen.getByTestId('page-subtitle')).toHaveTextContent(
      '资源目录只管理函数聚合后的资源语义；页面、菜单和分类在 Page Studio 中确定',
    );
    await waitFor(() => expect(mockListResourceCatalog).toHaveBeenCalled());
  });

  it('概览卡按数据派生统计项：总数/分类/已声明语义/诊断异常', async () => {
    renderPage();

    await waitFor(() => expect(screen.getByText('资源概览')).toBeInTheDocument());
    expect(screen.getByText('总数 2')).toBeInTheDocument();
    expect(screen.getByText('分类 1')).toBeInTheDocument();
    // 只有 player 声明了 semantics.version
    expect(screen.getByText('已声明语义 1')).toBeInTheDocument();
    // 只有 mail 有 warning 诊断
    expect(screen.getByText('诊断异常 1')).toBeInTheDocument();
    expect(
      screen.getByText(
        '资源层负责语义供给，Page Studio 负责页面装配，运行端菜单只来自已发布 PageSpec。',
      ),
    ).toBeInTheDocument();
  });

  it('边界 Alert 展示定位说明，按钮跳转 Page Studio', async () => {
    renderPage();

    await waitFor(() =>
      expect(screen.getByText('资源目录只展示语义与候选，不承载页面 UI')).toBeInTheDocument(),
    );
    expect(
      screen.getByText(
        '页面标题、菜单、表格列与按钮位置在 Page Studio 的 Proposal 中确定；确认资源语义后，请进入 Page Studio 生成或调整页面。',
      ),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: '进入 Page Studio' }));
    expect(mockHistoryPush).toHaveBeenCalledWith('/functions/pages');
  });

  it('列表与搜索区仍正常渲染（提示层不破坏既有结构）', async () => {
    renderPage();

    await waitFor(() => expect(screen.getByText('资源能力目录')).toBeInTheDocument());
    expect(screen.getByPlaceholderText('搜索资源')).toBeInTheDocument();
    expect(screen.getByText('player')).toBeInTheDocument();
    expect(screen.getByText('mail')).toBeInTheDocument();
  });
});

const makeFn = (functionId: string): FunctionInfo => ({
  id: 1,
  functionId,
  version: '1.0.0',
  capability: 'action',
  execution: 'sync',
  risk: 'safe',
  enabled: true,
  source: 'sdk',
});

const makeSemantics = (over: Partial<SemanticsInfo> = {}): SemanticsInfo => ({
  version: 3,
  hasIdentity: true,
  hasCollection: false,
  hasCreate: false,
  hasUpdate: false,
  hasDelete: false,
  hasActions: false,
  hasTasks: false,
  hasReports: false,
  source: 'platform_review',
  unresolvedConflicts: 0,
  identityField: 'id',
  identityFieldType: 'string',
  ...over,
});

const diagnostics: DiagnosticInfo[] = [
  { code: 'e1', severity: 'error', message: 'boom' },
  { code: 'w1', severity: 'warning', message: 'warn' },
];

const colItems: ResourceCatalogItem[] = [
  {
    resourceKey: 'player',
    labels: { 'zh-CN': '玩家' },
    categoryKey: '玩家运营',
    status: 'identified',
    functions: [makeFn('player.kick'), makeFn('player.mute')],
    semantics: makeSemantics(),
    diagnostics: [],
  },
  {
    resourceKey: 'mail',
    labels: {},
    categoryKey: '玩家运营',
    status: 'conflict',
    functions: [],
    diagnostics,
  },
  {
    resourceKey: 'task',
    labels: { 'zh-CN': '任务' },
    categoryKey: '任务',
    status: 'pending',
    functions: [makeFn('task.start')],
    diagnostics: [{ code: 'w2', severity: 'warning', message: 'w' }],
  },
];

const detailPlayer: ResourceCatalogItem = colItems[0];

function rowOf(key: string): HTMLElement {
  const tr = screen.getByText(key).closest('tr');
  if (!tr) throw new Error(`row not found: ${key}`);
  return tr;
}

function rowAction(key: string, iconClass: string): HTMLElement {
  const btn = rowOf(key).querySelector(`${iconClass}`)?.closest('button');
  if (!btn) throw new Error(`row action not found: ${key} ${iconClass}`);
  return btn as HTMLElement;
}

describe('ResourceCatalogPage 列表列渲染', () => {
  let msgError: jest.SpyInstance;
  let msgSuccess: jest.SpyInstance;

  beforeEach(() => {
    jest.clearAllMocks();
    mockListResourceCatalog.mockResolvedValue({ items: colItems, total: colItems.length });
    mockGetDetail.mockResolvedValue(detailPlayer);
    mockGetConflicts.mockResolvedValue({ conflicts: [], provenance: [] });
    mockGetVersions.mockResolvedValue({ items: [], total: 0 });
    msgError = jest.spyOn(message, 'error').mockImplementation(() => undefined as never);
    msgSuccess = jest.spyOn(message, 'success').mockImplementation(() => undefined as never);
  });

  afterEach(() => {
    jest.restoreAllMocks();
  });

  it('状态/名称/语义版本/函数数/诊断按行渲染（缺省回退 - 与「无」）', async () => {
    renderPage();
    await waitFor(() => expect(screen.getByText('player')).toBeInTheDocument());

    const player = within(rowOf('player'));
    expect(player.getByText('玩家')).toBeInTheDocument();
    expect(player.getByText('已识别')).toBeInTheDocument();
    expect(player.getByText('3')).toBeInTheDocument();
    expect(player.getByText('2')).toBeInTheDocument();
    expect(player.getByText('无')).toBeInTheDocument();

    const mail = within(rowOf('mail'));
    // 空 labels → 名称占位；无 semantics → 语义版本占位（行内 2 个 -）
    expect(mail.getAllByText('-')).toHaveLength(2);
    expect(mail.getByText('冲突')).toBeInTheDocument();
    expect(mail.getByText('0')).toBeInTheDocument();
    expect(mail.getByText('1 错误')).toBeInTheDocument();
    expect(mail.getByText('1 警告')).toBeInTheDocument();

    const task = within(rowOf('task'));
    expect(task.getByText('待确认')).toBeInTheDocument();
    expect(task.getByText('1 警告')).toBeInTheDocument();
  });

  it('分类去重：同分类多项归一，概览「分类 2」', async () => {
    renderPage();
    await waitFor(() => expect(screen.getByText('分类 2')).toBeInTheDocument());
    expect(screen.getByText('总数 3')).toBeInTheDocument();
    expect(screen.getByText('诊断异常 2')).toBeInTheDocument();
  });

  it('输入搜索触发带 query 重查；搜索/刷新按钮继续重查', async () => {
    renderPage();
    await waitFor(() => expect(mockListResourceCatalog).toHaveBeenCalledTimes(1));

    fireEvent.change(screen.getByPlaceholderText('搜索资源'), { target: { value: 'player' } });
    await waitFor(() =>
      expect(mockListResourceCatalog).toHaveBeenLastCalledWith({
        category: undefined,
        query: 'player',
      }),
    );

    fireEvent.click(screen.getByRole('button', { name: /搜索/ }));
    await waitFor(() => expect(mockListResourceCatalog).toHaveBeenCalledTimes(3));

    fireEvent.click(screen.getByRole('button', { name: /刷新/ }));
    await waitFor(() => expect(mockListResourceCatalog).toHaveBeenCalledTimes(4));
    expect(screen.getByText('player')).toBeInTheDocument();
  });

  it('选择分类 → 按分类重查', async () => {
    renderPage();
    await waitFor(() => expect(mockListResourceCatalog).toHaveBeenCalledTimes(1));

    // antd Select 占位渲染为 div（无 placeholder 属性），按文本定位
    const select = screen.getByText('选择分类').closest('.ant-select');
    expect(select).not.toBeNull();
    fireEvent.mouseDown(select as HTMLElement);

    // 选项渲染在 body 下的下拉容器（表内也有「任务」同名单元格，必须限定下拉范围）
    // categoryOptions 按 Unicode 排序：任务(U+4EFB) < 玩家运营(U+73A9)，且同分类去重
    await waitFor(() =>
      expect(
        Array.from(document.querySelectorAll('.ant-select-dropdown .ant-select-item-option')).map(
          (el) => el.textContent,
        ),
      ).toEqual(['任务', '玩家运营']),
    );
    const option = document.querySelector(
      '.ant-select-dropdown .ant-select-item-option[title="任务"]',
    );
    expect(option).not.toBeNull();
    fireEvent.click(option as Element);
    await waitFor(() =>
      expect(mockListResourceCatalog).toHaveBeenLastCalledWith({
        category: '任务',
        query: undefined,
      }),
    );
  });

  it('列表拉取失败：message.error 提示且页面结构不丢', async () => {
    mockListResourceCatalog.mockRejectedValueOnce(new Error('load boom'));

    renderPage();

    await waitFor(() => expect(msgError).toHaveBeenCalledWith('load boom'));
    expect(screen.getByText('资源能力目录')).toBeInTheDocument();
    expect(msgSuccess).not.toHaveBeenCalled();
  });

  it('提案按钮 → 跳转 Page Studio 并携带 resourceKey', async () => {
    renderPage();
    await waitFor(() => expect(screen.getByText('player')).toBeInTheDocument());

    const buttons = rowOf('player').querySelectorAll('button');
    fireEvent.click(buttons[buttons.length - 1]);
    expect(mockHistoryPush).toHaveBeenCalledWith('/functions/pages?resourceKey=player');
  });
});

describe('ResourceCatalogPage 详情与编辑语义', () => {
  let msgError: jest.SpyInstance;
  let msgSuccess: jest.SpyInstance;

  beforeEach(() => {
    jest.clearAllMocks();
    mockListResourceCatalog.mockResolvedValue({ items: colItems, total: colItems.length });
    mockGetDetail.mockResolvedValue(detailPlayer);
    mockGetConflicts.mockResolvedValue({ conflicts: [], provenance: [] });
    mockGetVersions.mockResolvedValue({ items: [], total: 0 });
    msgError = jest.spyOn(message, 'error').mockImplementation(() => undefined as never);
    msgSuccess = jest.spyOn(message, 'success').mockImplementation(() => undefined as never);
  });

  afterEach(() => {
    jest.restoreAllMocks();
  });

  it('查看详情：并行拉详情+冲突、拉版本首屏并打开弹窗；翻页按页重拉', async () => {
    mockGetVersions.mockResolvedValue({ items: [], total: 12 });
    renderPage();
    await waitFor(() => expect(screen.getByText('player')).toBeInTheDocument());

    fireEvent.click(rowAction('player', '.anticon-eye'));

    await waitFor(() => expect(mockGetDetail).toHaveBeenCalledWith('player'));
    expect(mockGetConflicts).toHaveBeenCalledWith('player');
    await waitFor(() =>
      expect(mockGetVersions).toHaveBeenCalledWith('player', { limit: 5, offset: 0 }),
    );
    expect(await screen.findByText('资源详情')).toBeInTheDocument();

    mockGetVersions.mockClear();
    // 版本分页项渲染为 li[title="2"]（rc-pagination），点击触发 onChange 翻页
    fireEvent.click(screen.getByTitle('2'));
    await waitFor(() =>
      expect(mockGetVersions).toHaveBeenCalledWith('player', { limit: 5, offset: 5 }),
    );
  });

  it('详情拉取失败：提示错误且详情弹窗不打开', async () => {
    mockGetDetail.mockRejectedValueOnce(new Error('detail boom'));
    renderPage();
    await waitFor(() => expect(screen.getByText('player')).toBeInTheDocument());

    fireEvent.click(rowAction('player', '.anticon-eye'));

    await waitFor(() => expect(msgError).toHaveBeenCalledWith('获取详情失败: detail boom'));
    expect(screen.queryByText('资源详情')).not.toBeInTheDocument();
  });

  it('编辑语义：打开回显 identityField，保存成功并刷新列表', async () => {
    mockUpdateSemantics.mockResolvedValue(undefined);
    renderPage();
    await waitFor(() => expect(screen.getByText('player')).toBeInTheDocument());

    fireEvent.click(rowAction('player', '.anticon-edit'));

    expect(await screen.findByText('编辑语义')).toBeInTheDocument();
    await waitFor(() => expect(screen.getByDisplayValue('id')).toBeInTheDocument());

    // antd 对两字按钮插入空格（accessible name 为「保 存」）
    fireEvent.click(screen.getByRole('button', { name: /保\s*存/ }));

    await waitFor(() =>
      expect(mockUpdateSemantics).toHaveBeenCalledWith(
        'player',
        expect.objectContaining({ identityField: 'id', identityFieldType: 'string' }),
      ),
    );
    expect(msgSuccess).toHaveBeenCalledWith('语义更新成功');
    // antd Modal 关闭后仍挂载 DOM（display:none），用角色查询验证已隐藏
    await waitFor(() =>
      expect(screen.queryByRole('button', { name: /保\s*存/ })).not.toBeInTheDocument(),
    );
    // 保存后重拉详情与列表
    await waitFor(() => expect(mockGetDetail).toHaveBeenCalledTimes(2));
    expect(mockListResourceCatalog).toHaveBeenCalledTimes(2);
  });

  it('编辑语义保存失败：提示原因且弹窗保持打开', async () => {
    mockUpdateSemantics.mockRejectedValue(new Error('save boom'));
    renderPage();
    await waitFor(() => expect(screen.getByText('player')).toBeInTheDocument());

    fireEvent.click(rowAction('player', '.anticon-edit'));
    await screen.findByText('编辑语义');
    // antd 对两字按钮插入空格（accessible name 为「保 存」）
    fireEvent.click(screen.getByRole('button', { name: /保\s*存/ }));

    await waitFor(() => expect(msgError).toHaveBeenCalledWith('更新失败: save boom'));
    expect(screen.getByText('编辑语义')).toBeInTheDocument();
    expect(msgSuccess).not.toHaveBeenCalled();
  });
});

describe('ResourceCatalogPage 冲突解决流', () => {
  let msgError: jest.SpyInstance;
  let msgSuccess: jest.SpyInstance;

  beforeEach(() => {
    jest.clearAllMocks();
    mockListResourceCatalog.mockResolvedValue({ items: colItems, total: colItems.length });
    mockGetDetail.mockResolvedValue(detailPlayer);
    mockGetVersions.mockResolvedValue({ items: [], total: 0 });
    mockGetConflicts.mockResolvedValue({
      conflicts: [
        {
          field: 'collectionPath',
          values: { platform_review: '/data/items', openapi_rest: '"items"' },
        },
      ],
      provenance: [],
    });
    msgError = jest.spyOn(message, 'error').mockImplementation(() => undefined as never);
    msgSuccess = jest.spyOn(message, 'success').mockImplementation(() => undefined as never);
  });

  afterEach(() => {
    jest.restoreAllMocks();
  });

  async function openResolveModal(): Promise<HTMLElement> {
    renderPage();
    await waitFor(() => expect(screen.getByText('player')).toBeInTheDocument());
    fireEvent.click(rowAction('player', '.anticon-eye'));
    const conflictRow = (await screen.findByText('collectionPath')).closest('tr');
    if (!conflictRow) throw new Error('conflict row not found');
    expect(within(conflictRow).getByText('未解决')).toBeInTheDocument();
    fireEvent.click(within(conflictRow).getByRole('button', { name: '选择来源' }));
    const title = await screen.findByText('解决语义冲突');
    const modal = title.closest('.ant-modal');
    if (!modal) throw new Error('resolve modal not found');
    return modal as HTMLElement;
  }

  it('详情内选择来源 → 改选候选并填原因 → 确认提交决议', async () => {
    mockResolveConflict.mockResolvedValue(undefined);
    const modal = await openResolveModal();

    // 默认预选平台确认（platform_review 优先级最高）
    const selector = within(modal).getByRole('combobox');
    fireEvent.mouseDown(selector);
    fireEvent.click(await screen.findByText('OpenAPI REST', {}, { timeout: 3000 }));
    fireEvent.change(within(modal).getByPlaceholderText('说明为什么采用该来源'), {
      target: { value: '按 OpenAPI 文档为准' },
    });
    fireEvent.click(within(modal).getByRole('button', { name: '确认选择' }));

    await waitFor(() =>
      expect(mockResolveConflict).toHaveBeenCalledWith('player', 'collectionPath', {
        chosenSource: 'openapi_rest',
        reason: '按 OpenAPI 文档为准',
      }),
    );
    expect(msgSuccess).toHaveBeenCalledWith('冲突已解决，相关 Proposal 已触发重算');
    // antd Modal 关闭后仍挂载 DOM（display:none），用角色查询验证已隐藏
    await waitFor(() =>
      expect(screen.queryByRole('button', { name: '确认选择' })).not.toBeInTheDocument(),
    );
  });

  it('提交决议失败：提示原因且弹窗保持打开', async () => {
    mockResolveConflict.mockRejectedValue(new Error('resolve boom'));
    const modal = await openResolveModal();

    fireEvent.click(within(modal).getByRole('button', { name: '确认选择' }));

    await waitFor(() => expect(msgError).toHaveBeenCalledWith('解决冲突失败: resolve boom'));
    expect(screen.getByText('解决语义冲突')).toBeInTheDocument();
    expect(msgSuccess).not.toHaveBeenCalled();
  });
});
