/**
 * ResourceCatalogPage 残余分支补齐（纯 hook 流程侧）：
 * 1. 语义版本拉取失败 → message.error（fetchSemanticVersions catch）；
 * 2. 编辑语义时详情接口返回空 → 早退不打开弹窗（handleEditSemantics 的 !detail）；
 * 3. 编辑/冲突弹窗已打开后详情接口返回空 → selectedResource 被置空，
 *    再点保存/确认分别走 handleSaveSemantics / handleResolveConflict 的
 *    「缺 selectedResource」早退分支。
 */
import React from 'react';
import { App as AntdApp, message } from 'antd';
import { configure, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import ResourceCatalogPage from '../index';
import type { ResourceCatalogItem } from '@/types/dashboard';

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
    history: { push: jest.fn() },
    FormattedMessage: ({ defaultMessage }: { defaultMessage: string }) => defaultMessage,
  };
});

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

const items: ResourceCatalogItem[] = [
  {
    resourceKey: 'player',
    labels: { 'zh-CN': '玩家' },
    categoryKey: '玩家运营',
    status: 'conflict',
    functions: [],
    semantics: { version: 3 } as ResourceCatalogItem['semantics'],
    diagnostics: [],
  },
  {
    resourceKey: 'mail',
    labels: { 'zh-CN': '邮件' },
    categoryKey: '玩家运营',
    status: 'pending',
    functions: [],
    diagnostics: [],
  },
  // 无 categoryKey / 无 diagnostics：分类列与统计的 || 空态分支
  {
    resourceKey: 'guild',
    labels: { 'zh-CN': '公会' },
    status: 'pending',
    functions: [],
  } as ResourceCatalogItem,
];

function renderPage() {
  return render(
    <AntdApp>
      <ResourceCatalogPage />
    </AntdApp>,
  );
}

function rowAction(key: string, iconClass: string): HTMLElement {
  const tr = screen.getByText(key).closest('tr');
  if (!tr) throw new Error(`row not found: ${key}`);
  const btn = tr.querySelector(`${iconClass}`)?.closest('button');
  if (!btn) throw new Error(`row action not found: ${key} ${iconClass}`);
  return btn as HTMLElement;
}

function tableRowAction(key: string, iconClass: string): HTMLElement {
  for (const table of screen.getAllByRole('table')) {
    const cell = within(table).queryByText(key);
    if (!cell) continue;
    const tr = cell.closest('tr');
    const btn = tr?.querySelector(`${iconClass}`)?.closest('button');
    if (btn) return btn as HTMLElement;
  }
  throw new Error(`table row action not found: ${key} ${iconClass}`);
}

const settle = async (ms = 60) => {
  await new Promise((resolve) => setTimeout(resolve, ms));
};

describe('ResourceCatalogPage 残余分支', () => {
  let msgError: jest.SpyInstance;

  beforeEach(() => {
    jest.clearAllMocks();
    mockListResourceCatalog.mockResolvedValue({ items, total: items.length });
    mockGetDetail.mockResolvedValue(items[0]);
    mockGetConflicts.mockResolvedValue({
      conflicts: [
        {
          field: 'collectionPath',
          values: { platform_review: '/data/items', openapi_rest: '"items"' },
        },
      ],
      provenance: [],
    });
    mockGetVersions.mockResolvedValue({ items: [], total: 0 });
    mockResolveConflict.mockResolvedValue(undefined);
    mockUpdateSemantics.mockResolvedValue(undefined);
    msgError = jest.spyOn(message, 'error').mockImplementation(() => undefined as never);
  });

  afterEach(() => {
    jest.restoreAllMocks();
  });

  it('语义版本拉取失败：提示「获取语义版本失败」且详情弹窗照常打开', async () => {
    mockGetVersions.mockRejectedValueOnce(new Error('versions boom'));
    renderPage();
    await waitFor(() => expect(screen.getByText('player')).toBeInTheDocument());

    fireEvent.click(rowAction('player', '.anticon-eye'));

    await waitFor(() => expect(msgError).toHaveBeenCalledWith('获取语义版本失败: versions boom'));
    expect(await screen.findByText('资源详情')).toBeInTheDocument();
  });

  it('编辑语义时详情接口返回空 → 早退不打开编辑弹窗', async () => {
    mockGetDetail.mockResolvedValue(null);
    renderPage();
    await waitFor(() => expect(screen.getByText('player')).toBeInTheDocument());

    fireEvent.click(rowAction('player', '.anticon-edit'));

    await waitFor(() => expect(mockGetDetail).toHaveBeenCalledWith('player'));
    await settle();
    expect(screen.queryByText('编辑语义')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /保\s*存/ })).not.toBeInTheDocument();
  });

  it('编辑弹窗已打开后详情返回空 → 保存走缺 selectedResource 早退', async () => {
    renderPage();
    await waitFor(() => expect(screen.getByText('player')).toBeInTheDocument());

    const editButton = rowAction('player', '.anticon-edit');
    fireEvent.click(editButton);
    expect(await screen.findByText('编辑语义')).toBeInTheDocument();
    await waitFor(() => expect(mockGetDetail).toHaveBeenCalledTimes(1));

    // 详情接口改为返回空：selectedResource 被置空，但编辑弹窗保持打开
    mockGetDetail.mockResolvedValue(null);
    fireEvent.click(editButton);
    await waitFor(() => expect(mockGetDetail).toHaveBeenCalledTimes(2));
    await settle();

    fireEvent.click(screen.getByRole('button', { name: /保\s*存/ }));
    await settle();
    expect(mockUpdateSemantics).not.toHaveBeenCalled();
    expect(screen.getByText('编辑语义')).toBeInTheDocument();
    expect(msgError).not.toHaveBeenCalled();
  });

  it('无 categoryKey 的资源：分类列回退 -；分类筛选可清除', async () => {
    renderPage();
    await waitFor(() => expect(screen.getByText('guild')).toBeInTheDocument());
    const row = screen.getByText('guild').closest('tr');
    expect(row).not.toBeNull();
    expect(within(row as HTMLElement).getAllByText('-').length).toBeGreaterThanOrEqual(1);

    // 选中分类（value 为真分支）→ 清除按钮出现（value || '' 的空串分支）
    fireEvent.mouseDown(document.querySelector('.ant-select-placeholder') as Element);
    const option = await screen.findAllByText('玩家运营');
    fireEvent.click(option[option.length - 1]);
    await waitFor(() => expect(document.querySelector('.ant-select-clear')).not.toBeNull());

    fireEvent.click(document.querySelector('.ant-select-clear') as Element);
    await waitFor(() => expect(document.querySelector('.ant-select-clear')).toBeNull());
    expect(document.querySelector('.ant-select-placeholder')).not.toBeNull();
  }, 60000);

  it('详情/编辑/冲突三个弹窗的关闭与取消回调均接线', async () => {
    renderPage();
    await waitFor(() => expect(screen.getByText('player')).toBeInTheDocument());
    const dialogCount = () => screen.queryAllByRole('dialog').length;

    // 详情弹窗 X → onClose → 关闭
    fireEvent.click(tableRowAction('player', '.anticon-eye'));
    await screen.findByText('资源详情');
    expect(dialogCount()).toBe(1);
    const detailModal = screen.getByText('资源详情').closest('.ant-modal');
    const closeBtn = detailModal?.querySelector('.ant-modal-close');
    if (!closeBtn) throw new Error('未找到详情弹窗关闭按钮');
    fireEvent.click(closeBtn as Element);
    await waitFor(() => expect(dialogCount()).toBe(0));

    // 编辑弹窗 取消 → onCancel → 关闭
    fireEvent.click(tableRowAction('player', '.anticon-edit'));
    await screen.findByText('编辑语义');
    fireEvent.click(screen.getByRole('button', { name: /取\s*消/ }));
    await waitFor(() => expect(dialogCount()).toBe(0));

    // 冲突弹窗 取消 → onCancel → 关闭
    fireEvent.click(tableRowAction('player', '.anticon-eye'));
    const conflictRow = (await screen.findByText('collectionPath')).closest('tr');
    if (!conflictRow) throw new Error('conflict row not found');
    fireEvent.click(within(conflictRow).getByRole('button', { name: '选择来源' }));
    await screen.findByText('解决语义冲突');
    // 详情弹窗仍开着，冲突弹窗叠在其上 → 两层
    expect(dialogCount()).toBe(2);
    fireEvent.click(screen.getByRole('button', { name: /取\s*消/ }));
    // 冲突弹窗关闭 → 只剩详情弹窗
    await waitFor(() => expect(dialogCount()).toBe(1));
    expect(screen.queryByRole('button', { name: '确认选择' })).not.toBeInTheDocument();
  }, 90000);

  it('冲突弹窗已打开后详情返回空 → 确认走缺 selectedResource 早退', async () => {
    renderPage();
    await waitFor(() => expect(screen.getByText('player')).toBeInTheDocument());

    const eyeButton = rowAction('player', '.anticon-eye');
    fireEvent.click(eyeButton);
    const conflictRow = (await screen.findByText('collectionPath')).closest('tr');
    if (!conflictRow) throw new Error('conflict row not found');
    fireEvent.click(within(conflictRow).getByRole('button', { name: '选择来源' }));
    expect(await screen.findByText('解决语义冲突')).toBeInTheDocument();

    // 详情接口改为返回空：selectedResource 被置空，冲突弹窗保持打开
    mockGetDetail.mockResolvedValue(null);
    fireEvent.click(eyeButton);
    await waitFor(() => expect(mockGetDetail).toHaveBeenCalledTimes(2));
    await settle();

    fireEvent.click(screen.getByRole('button', { name: '确认选择' }));
    await settle();
    expect(mockResolveConflict).not.toHaveBeenCalled();
    expect(screen.getByText('解决语义冲突')).toBeInTheDocument();
    expect(msgError).not.toHaveBeenCalled();
  });
});
