/**
 * 扩展商店页单测（覆盖率巡检：index.tsx 512 行 0% → 行覆盖收口）。
 *
 * 锁定契约：7 列渲染矩阵（displayName 兜底 name、kind/版本空兜底 '-'、状态
 * active 绿 Tag、标签 defaultInstall 金标 + slice(0,3) + 全空 '-' 兜底、已安装
 * 是/否、安装按钮 canManage+installed 双门控与「已安装」文案）、草稿/提交双态
 * 筛选（草稿不触发拉取、查询提交三态 + params 未变 reload 兜底、重置清双态）、
 * 详情弹窗加载链（adapter fallbackItem 兜底 + capabilities/releases Tags）、
 * 安装弹窗链（latestVersion 兜底链 detail→releases[0]→item、manifest.configSchema
 * 双 typeof 提取、setFieldsValue 预填、SchemaFields 默认值回显、releases 空
 * 警告）、handleInstall（成功载荷 + configJson 非法拦截 + 合并覆盖 + 四错误码
 * 分支文案 + 兜底 message）、定位 Alert 的 history.push。
 *
 * mock 口径：services/api/extensions 四个函数 jest.mock；InstallModal/
 * CatalogDetailModal/SchemaFields/shared/adapters/mapper 走真实实现——安装表单
 * Form 实例由页面持有，mock 掉 InstallModal 会使 validateFields 永远空值，
 * 必须真实渲染才能走通提交链。@umijs/max 本地 mock（含 history.push）。
 *
 * 边界（诚实）：
 * 1. openDetail 的 try/finally 无 catch——detail 接口 reject 产生 unhandled
 *    rejection（组件现状缺陷，同 InstallationDetailDrawer 巡检结论），不造假
 *    reject 用例；adapter 的 fallbackItem 兜底翼改用 resolve `{}` 覆盖。
 * 2. handleInstall 的 `if (!installItem) return` 守卫经 UI 不可达（OK 按钮仅在
 *    弹窗打开且 installItem 已设时可见），不造假；latestVersion 兜底链的
 *    `|| ''` 尾翼同样不可达——item.latestVersion 为空时表单预填空串，提交被
 *    releaseVersion 的 required 规则拦截（validateFields 早于 installExtension），
 *    实测「请选择版本」。
 */
import React from 'react';
import { configure, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { App } from 'antd';
import ExtensionsStorePage from '../index';
import type { ExtensionCatalogItem, ExtensionReleaseItem } from '@/services/api/extensions';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

jest.mock('@/services/api/extensions', () => ({
  listExtensionCatalog: jest.fn(),
  getExtensionCatalogDetail: jest.fn(),
  listExtensionCatalogReleases: jest.fn(),
  installExtension: jest.fn(),
  createExtensionCatalog: jest.fn(),
  updateExtensionCatalog: jest.fn(),
  deleteExtensionCatalog: jest.fn(),
  publishExtensionRelease: jest.fn(),
  importExtensionPack: jest.fn(),
}));

// mock* 前缀变量：babel-jest hoist 白名单，允许 mock 工厂延迟绑定
const mockCanManage = jest.fn(() => true);
const mockHistoryPush = jest.fn();

jest.mock('@umijs/max', () => ({
  FormattedMessage: ({
    defaultMessage,
    values,
  }: {
    defaultMessage?: string;
    values?: Record<string, unknown>;
  }) => {
    let text = defaultMessage ?? '';
    if (values) {
      for (const [k, v] of Object.entries(values)) text = text.split(`{${k}}`).join(String(v));
    }
    return <>{text}</>;
  },
  useIntl: () => ({
    formatMessage: (opts: { defaultMessage?: string }, values?: Record<string, unknown>) => {
      let text = opts.defaultMessage ?? '';
      if (values) {
        for (const [k, v] of Object.entries(values)) text = text.split(`{${k}}`).join(String(v));
      }
      return text;
    },
  }),
  useAccess: () => ({ canExtensionsManage: mockCanManage() }),
  history: { push: (...args: unknown[]) => mockHistoryPush(...(args as [])) },
  PageContainer: ({ children, title }: { children?: React.ReactNode; title?: React.ReactNode }) => (
    <div>
      <h1>{title}</h1>
      {children}
    </div>
  ),
}));

import {
  createExtensionCatalog,
  deleteExtensionCatalog,
  getExtensionCatalogDetail,
  importExtensionPack,
  installExtension,
  listExtensionCatalog,
  listExtensionCatalogReleases,
  publishExtensionRelease,
  updateExtensionCatalog,
} from '@/services/api/extensions';

const mList = listExtensionCatalog as jest.MockedFunction<typeof listExtensionCatalog>;
const mDetail = getExtensionCatalogDetail as jest.MockedFunction<typeof getExtensionCatalogDetail>;
const mReleases = listExtensionCatalogReleases as jest.MockedFunction<
  typeof listExtensionCatalogReleases
>;
const mInstall = installExtension as jest.MockedFunction<typeof installExtension>;
const mCreate = createExtensionCatalog as jest.MockedFunction<typeof createExtensionCatalog>;
const mUpdate = updateExtensionCatalog as jest.MockedFunction<typeof updateExtensionCatalog>;
const mDelete = deleteExtensionCatalog as jest.MockedFunction<typeof deleteExtensionCatalog>;
const mPublish = publishExtensionRelease as jest.MockedFunction<typeof publishExtensionRelease>;
const mImport = importExtensionPack as jest.MockedFunction<typeof importExtensionPack>;

const item1: ExtensionCatalogItem = {
  id: 'chatops',
  name: 'chatops',
  displayName: 'ChatOps',
  vendor: 'croupier',
  kind: 'integration',
  summary: 'Chat ops pack',
  iconUrl: '',
  status: 'active',
  latestVersion: '1.4.0',
  installed: false,
  defaultInstall: true,
  tags: ['ai', 'chat', 'ops', 'extra1', 'extra2'],
};

// displayName 空 → 兜底 name；installed=true → 「已安装」禁用按钮
const item2: ExtensionCatalogItem = {
  id: 'wiki',
  name: 'wiki',
  displayName: '',
  vendor: '',
  kind: 'ui',
  summary: '',
  iconUrl: '',
  status: 'active',
  latestVersion: '0.9.1',
  installed: true,
  defaultInstall: false,
  tags: [],
};

// kind/版本空 → '-' 兜底；非 active 状态；无标签无默认 → 标签列 '-'
const item3: ExtensionCatalogItem = {
  id: 'legacy',
  name: 'legacy',
  displayName: 'Legacy',
  vendor: '',
  kind: '',
  summary: '',
  iconUrl: '',
  status: 'inactive',
  latestVersion: '',
  installed: false,
  defaultInstall: false,
  tags: [],
};

// latestVersion 空 → openInstall 的 latestVersion 兜底链走 releases[0]
const item4: ExtensionCatalogItem = {
  ...item1,
  id: 'nopad',
  name: 'nopad',
  displayName: 'NoPad',
  latestVersion: '',
};

const releaseOf = (version: string): ExtensionReleaseItem => ({
  version,
  releaseChannel: 'stable',
  minCoreVersion: '0.0.1',
  publishedAt: 1,
  changelog: '',
});

const configSchema = {
  properties: {
    endpoint: { type: 'string', title: '端点', default: 'http://localhost:9' },
    retries: { type: 'number', title: '重试', default: 3 },
    mode: { type: 'string', title: '模式', enum: ['fast', 'safe'] },
  },
  required: ['endpoint'],
};

function renderPage() {
  return render(
    <App>
      <ExtensionsStorePage />
    </App>,
  );
}

async function waitFirstLoad() {
  expect(await screen.findByText('ChatOps')).toBeInTheDocument();
  await waitFor(() => expect(mList).toHaveBeenCalledTimes(1));
}

/** 打开某行的安装弹窗并等待预填完成（scopeType 回显即 Promise.all 链已落） */
async function openInstallAndWait(rowTitle: string) {
  const row = screen.getByText(rowTitle).closest('tr') as HTMLElement;
  fireEvent.click(within(row).getByRole('button', { name: '安装' }));
  expect(await screen.findByText('安装扩展: ChatOps')).toBeInTheDocument();
  expect(await screen.findByDisplayValue('system')).toBeInTheDocument();
}

/** 点安装弹窗主按钮（footer primary = OK） */
function clickInstallOk() {
  const ok = document.querySelector('.ant-modal-footer .ant-btn-primary') as HTMLButtonElement;
  expect(ok).not.toBeNull();
  fireEvent.click(ok);
}

/** 安装接口错误 fixture（mapper 从 response.data.details 读 code/字段） */
const errWith = (details: Record<string, unknown>) => ({
  response: { data: { details } },
});

beforeEach(() => {
  jest.clearAllMocks();
  mockCanManage.mockReturnValue(true);
  mList.mockResolvedValue({ total: 4, items: [item1, item2, item3, item4] });
  mDetail.mockResolvedValue({
    item: item1,
    releases: [releaseOf('1.4.0'), releaseOf('1.5.0')],
    manifest: { configSchema },
    capabilities: ['cap.echo'],
  });
  mReleases.mockResolvedValue({ total: 2, releases: [releaseOf('1.4.0'), releaseOf('1.5.0')] });
  mInstall.mockResolvedValue({ installationId: 9, status: 'installing' });
  mCreate.mockResolvedValue({ item: item1 });
  mUpdate.mockResolvedValue({ item: item1 });
  mDelete.mockResolvedValue({ deleted: true });
  mPublish.mockResolvedValue({ release: releaseOf('1.5.0') });
  mImport.mockResolvedValue({
    catalog: item1,
    release: releaseOf('2.0.0'),
    catalogCreated: true,
    packageRef: 'extension-packs/chatops/2.0.0.tgz',
    checksum: 'sha256:' + 'a'.repeat(64),
    size: 1024,
  });
});

describe('扩展商店 列表与筛选', () => {
  it('7 列渲染矩阵：displayName 兜底、kind/版本空兜底、标签三态、已安装两翼、安装按钮门控', async () => {
    renderPage();
    await waitFirstLoad();

    // 扩展列：displayName 直出 / 空 → name 兜底
    expect(screen.getByText('ChatOps')).toBeInTheDocument();
    expect(screen.getByText('chatops', { selector: 'td span' })).toBeInTheDocument();
    expect(screen.getByText('wiki', { selector: 'strong' })).toBeInTheDocument();
    expect(screen.getByText('Legacy')).toBeInTheDocument();

    // 类型列（item1 与 item4 同 kind）
    expect(screen.getAllByText('integration')).toHaveLength(2);
    expect(screen.getByText('ui')).toBeInTheDocument();

    // 标签列：defaultInstall 金标 + slice(0,3)（item1/item4 同 fixture，extra1/extra2 截断）
    expect(screen.getAllByText('默认安装')).toHaveLength(2);
    expect(screen.getAllByText('ai')).toHaveLength(2);
    expect(screen.getAllByText('chat')).toHaveLength(2);
    expect(screen.getAllByText('ops')).toHaveLength(2);
    expect(screen.queryByText('extra1')).not.toBeInTheDocument();

    // 已安装列：是（blue）/否
    expect(screen.getByText('是')).toBeInTheDocument();
    expect(screen.getAllByText('否')).toHaveLength(3);

    // 操作列：详情 ×4；安装（未安装 ×3）与「已安装」禁用（row2）
    expect(screen.getAllByRole('button', { name: '详情' })).toHaveLength(4);
    const rowChat = screen.getByText('ChatOps').closest('tr') as HTMLElement;
    expect(within(rowChat).getByRole('button', { name: '安装' })).toBeEnabled();
    const rowWiki = screen.getByText('wiki', { selector: 'strong' }).closest('tr') as HTMLElement;
    expect(within(rowWiki).getByRole('button', { name: '已安装' })).toBeDisabled();

    // 空兜底翼在行内：row3 kind '-' + 版本 '-' + 标签 '-'
    const rowLegacy = screen.getByText('Legacy').closest('tr') as HTMLElement;
    expect(within(rowLegacy).getAllByText('-').length).toBeGreaterThanOrEqual(2);
    expect(within(rowLegacy).getByRole('button', { name: '安装' })).toBeEnabled();

    // 首拉载荷：keyword 空串、其余 undefined
    expect(mList).toHaveBeenCalledWith({
      keyword: '',
      kind: undefined,
      status: undefined,
      page: 1,
      pageSize: 10,
    });

    // 定位 Alert + 页头
    expect(screen.getByText('商店只负责发现与安装扩展物料')).toBeInTheDocument();
    expect(screen.getByText('扩展商店')).toBeInTheDocument();
  });

  it('request 失败静默翼：catch 返回 success:false、空表无 crash', async () => {
    mList.mockRejectedValue(new Error('list down'));
    renderPage();

    await waitFor(() => expect(mList).toHaveBeenCalledTimes(1));
    expect(screen.queryByText('ChatOps')).not.toBeInTheDocument();
    expect(screen.queryByText('商店只负责发现与安装扩展物料')).not.toBeNull();
  });

  it('草稿/提交双态：草稿变更不触发拉取，查询提交三态，params 未变再查询仍 reload 兜底', async () => {
    renderPage();
    await waitFirstLoad();

    // 草稿输入 + 类型选择：params（提交态）未变 → 不触发第二次拉取
    // （antd6 可见 option 行无 role：点 .ant-select-item-option-content）
    fireEvent.change(screen.getByPlaceholderText('关键字'), { target: { value: 'chat' } });
    fireEvent.mouseDown(document.querySelectorAll('.ant-select')[0]);
    fireEvent.click(
      within(document.querySelector('.ant-select-dropdown') as HTMLElement).getByText('ops', {
        selector: '.ant-select-item-option-content',
      }),
    );
    await new Promise((r) => setTimeout(r, 60));
    expect(mList).toHaveBeenCalledTimes(1);

    // 查询：提交草稿 + 回第 1 页
    fireEvent.click(screen.getByRole('button', { name: '查询' }));
    await waitFor(() =>
      expect(mList).toHaveBeenLastCalledWith(
        expect.objectContaining({ keyword: 'chat', kind: 'ops', status: undefined }),
      ),
    );

    // params 未变再点查询：reload 兜底仍触发重查
    const before = mList.mock.calls.length;
    fireEvent.click(screen.getByRole('button', { name: '查询' }));
    await waitFor(() => expect(mList.mock.calls.length).toBeGreaterThan(before));
  });

  it('重置：草稿与提交双态同时清空 + 触发回空载荷的重查', async () => {
    renderPage();
    await waitFirstLoad();

    fireEvent.change(screen.getByPlaceholderText('关键字'), { target: { value: 'chat' } });
    fireEvent.click(screen.getByRole('button', { name: '查询' }));
    await waitFor(() =>
      expect(mList).toHaveBeenLastCalledWith(expect.objectContaining({ keyword: 'chat' })),
    );

    fireEvent.click(screen.getByRole('button', { name: '重置' }));
    await waitFor(() =>
      expect(mList).toHaveBeenLastCalledWith(
        expect.objectContaining({ keyword: '', kind: undefined, status: undefined }),
      ),
    );
    expect((screen.getByPlaceholderText('关键字') as HTMLInputElement).value).toBe('');
  });
});

describe('扩展商店 详情弹窗（openDetail）', () => {
  it('加载链：标题、ID、描述、能力 Tag、版本 Tags；adapter fallbackItem 兜底（resolve `{}`）', async () => {
    renderPage();
    await waitFirstLoad();

    // 正常链
    const rowChat = screen.getByText('ChatOps').closest('tr') as HTMLElement;
    fireEvent.click(within(rowChat).getByRole('button', { name: '详情' }));
    const modal = (await screen.findByText('ChatOps', { selector: '.ant-modal-title' })).closest(
      '.ant-modal',
    ) as HTMLElement;
    expect(within(modal).getByText('chatops')).toBeInTheDocument();
    expect(within(modal).getByText('Chat ops pack')).toBeInTheDocument();
    expect(within(modal).getByText('cap.echo')).toBeInTheDocument();
    expect(within(modal).getByText('1.4.0')).toBeInTheDocument();
    expect(within(modal).getByText('1.5.0')).toBeInTheDocument();
    expect(within(modal).queryByText('加载中...')).not.toBeInTheDocument();

    // 关闭后重开 row3（无同名文本干扰）并让 detail 返回 `{}`：item 落 fallbackItem
    fireEvent.click(modal.querySelector('.ant-modal-close') as HTMLElement);
    await waitFor(() => expect(screen.queryByText('描述: ')).not.toBeInTheDocument());
    mDetail.mockResolvedValue({
      item: undefined,
      releases: [],
      manifest: undefined,
      capabilities: [],
    });
    fireEvent.click(within(rowChat).getByRole('button', { name: '详情' }));
    const modal2 = (await screen.findByText('ChatOps', { selector: '.ant-modal-title' })).closest(
      '.ant-modal',
    ) as HTMLElement;
    expect(within(modal2).getByText('Chat ops pack')).toBeInTheDocument();
    expect(within(modal2).getAllByText('无')).toHaveLength(2);
    expect(within(modal2).queryByText('加载中...')).not.toBeInTheDocument();
  });
});

describe('扩展商店 安装弹窗（openInstall / handleInstall）', () => {
  it('打开链：latestVersion 预填、五字段预填回显、configSchema 提取 + SchemaFields 默认值回显', async () => {
    renderPage();
    await waitFirstLoad();
    await openInstallAndWait('ChatOps');

    // 预填回显：scope/scopeId/targetType/targetId Input + schema 默认值
    expect(screen.getByDisplayValue('global')).toBeInTheDocument();
    expect(screen.getByDisplayValue('agent_group')).toBeInTheDocument();
    expect(screen.getByDisplayValue('default')).toBeInTheDocument();
    expect(screen.getByDisplayValue('http://localhost:9')).toBeInTheDocument();
    // SchemaFields 卡片标题（configSchema 双 typeof 检查通过）
    expect(screen.getByText('配置字段（来自 manifest.configSchema）')).toBeInTheDocument();
    // releaseVersion 经 Select 预填（值在 selection 文本中）
    expect(document.querySelector('.ant-modal')?.textContent).toContain('1.4.0');
  });

  it('latestVersion 兜底链：detail item 缺失 + item.latestVersion 空 → 落 releases[0].version', async () => {
    mDetail.mockResolvedValue({
      item: undefined,
      releases: [],
      manifest: undefined,
      capabilities: [],
    });
    mReleases.mockResolvedValue({ total: 1, releases: [releaseOf('2.0.0')] });
    renderPage();
    await waitFirstLoad();

    const row = screen.getByText('NoPad').closest('tr') as HTMLElement;
    fireEvent.click(within(row).getByRole('button', { name: '安装' }));
    expect(await screen.findByText('安装扩展: NoPad')).toBeInTheDocument();
    expect(await screen.findByDisplayValue('system')).toBeInTheDocument();

    const ok = document.querySelector('.ant-modal-footer .ant-btn-primary') as HTMLButtonElement;
    fireEvent.click(ok);

    await waitFor(() =>
      expect(mInstall).toHaveBeenCalledWith(expect.objectContaining({ releaseVersion: '2.0.0' })),
    );
    // schema 未提取 → 无 SchemaFields 卡片
    expect(screen.queryByText('配置字段（来自 manifest.configSchema）')).not.toBeInTheDocument();
  });

  it('成功链：预填直接提交 → 载荷（含 schema 默认 config）+ 「已提交安装：ChatOps」+ reload + 关闭', async () => {
    renderPage();
    await waitFirstLoad();
    await openInstallAndWait('ChatOps');

    clickInstallOk();

    await waitFor(() =>
      expect(mInstall).toHaveBeenCalledWith({
        extensionId: 'chatops',
        releaseVersion: '1.4.0',
        scopeType: 'system',
        scopeId: 'global',
        targetType: 'agent_group',
        targetId: 'default',
        config: { endpoint: 'http://localhost:9', retries: 3 },
      }),
    );
    expect(await screen.findByText('已提交安装：ChatOps')).toBeInTheDocument();
    // 关闭语义：antd6 Modal 壳残留（无 destroyOnHidden），以 reload 触达为准
    await waitFor(() => expect(mList.mock.calls.length).toBeGreaterThan(1));
  });

  it('configJson 非法翼：「配置 JSON 格式不正确」拦截、不调安装接口、弹窗不关', async () => {
    renderPage();
    await waitFirstLoad();
    await openInstallAndWait('ChatOps');

    fireEvent.change(screen.getByPlaceholderText('{"enabled": true}'), {
      target: { value: '{oops' },
    });
    clickInstallOk();

    expect(await screen.findByText('配置 JSON 格式不正确')).toBeInTheDocument();
    expect(mInstall).not.toHaveBeenCalled();
    expect(screen.queryByText('已提交安装：ChatOps')).not.toBeInTheDocument();
  });

  it('configJson 合并翼：合法 JSON 覆盖 schema 默认值后进入载荷', async () => {
    renderPage();
    await waitFirstLoad();
    await openInstallAndWait('ChatOps');

    fireEvent.change(screen.getByPlaceholderText('{"enabled": true}'), {
      target: { value: '{"endpoint":"http://override","tls":true}' },
    });
    clickInstallOk();

    await waitFor(() =>
      expect(mInstall).toHaveBeenCalledWith(
        expect.objectContaining({
          config: { endpoint: 'http://override', retries: 3, tls: true },
        }),
      ),
    );
  });

  it('错误码 already_installed：details 六字段透出详情串', async () => {
    mInstall.mockRejectedValue(
      errWith({
        code: 'extension_already_installed',
        installationId: 3,
        scopeType: 'system',
        scopeId: 'global',
        targetType: 'agent_group',
        targetId: 'default',
        releaseVersion: '1.4.0',
      }),
    );
    renderPage();
    await waitFirstLoad();
    await openInstallAndWait('ChatOps');

    clickInstallOk();
    expect(
      await screen.findByText(
        '该扩展已安装（实例 3）。范围 system:global，目标 agent_group:default，版本 1.4.0',
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText('已提交安装：ChatOps')).not.toBeInTheDocument();
  });

  it('错误码三连：missing_dependency / dependency_version_mismatch / dependency_cycle 文案', async () => {
    renderPage();
    await waitFirstLoad();
    await openInstallAndWait('ChatOps');

    // 1) missing_dependency（dependency 缺省 → unknown 兜底）
    mInstall.mockRejectedValue(errWith({ code: 'missing_dependency' }));
    clickInstallOk();
    expect(await screen.findByText('缺少依赖扩展：unknown')).toBeInTheDocument();

    // 2) dependency_version_mismatch
    mInstall.mockRejectedValue(
      errWith({
        code: 'dependency_version_mismatch',
        dependency: 'audit',
        requiredVersion: '^1.0',
        currentVersion: '0.9',
      }),
    );
    clickInstallOk();
    expect(
      await screen.findByText('依赖版本不匹配：audit，要求 ^1.0，当前 0.9'),
    ).toBeInTheDocument();

    // 3) dependency_cycle
    mInstall.mockRejectedValue(errWith({ code: 'dependency_cycle', dependency: 'audit' }));
    clickInstallOk();
    expect(await screen.findByText('检测到循环依赖：audit')).toBeInTheDocument();

    expect(mInstall).toHaveBeenCalledTimes(3);
  });

  it('错误兜底翼：未知 code → message.error 透传 mapExtensionError 的英文 message', async () => {
    mInstall.mockRejectedValue(errWith({ code: 'forbidden' }));
    renderPage();
    await waitFirstLoad();
    await openInstallAndWait('ChatOps');

    clickInstallOk();
    expect(
      await screen.findByText('You do not have permission for this operation.'),
    ).toBeInTheDocument();
  });

  it('错误 details 缺省兜底：already_installed 六字段全缺落 -，mismatch/cycle dependency 缺落 unknown/-', async () => {
    renderPage();
    await waitFirstLoad();
    await openInstallAndWait('ChatOps');

    mInstall.mockRejectedValue(errWith({ code: 'extension_already_installed' }));
    clickInstallOk();
    expect(
      await screen.findByText('该扩展已安装（实例 -）。范围 -:-，目标 -:-，版本 -'),
    ).toBeInTheDocument();

    mInstall.mockRejectedValue(errWith({ code: 'dependency_version_mismatch' }));
    clickInstallOk();
    expect(await screen.findByText('依赖版本不匹配：unknown，要求 -，当前 -')).toBeInTheDocument();

    mInstall.mockRejectedValue(errWith({ code: 'dependency_cycle' }));
    clickInstallOk();
    expect(await screen.findByText('检测到循环依赖：unknown')).toBeInTheDocument();

    // 非 HTTP 错误（无 response.data.details）→ unknown 兜底 message
    mInstall.mockRejectedValue(new Error('net'));
    clickInstallOk();
    expect(
      await screen.findByText('Please retry or contact an administrator.'),
    ).toBeInTheDocument();
  });

  it('latestVersion 尾链：releases 空落 item.latestVersion（`|| item.latestVersion` 右翼）', async () => {
    mDetail.mockResolvedValue({
      item: undefined,
      releases: [],
      manifest: undefined,
      capabilities: [],
    });
    mReleases.mockResolvedValue({ total: 0, releases: [] });
    renderPage();
    await waitFirstLoad();

    // item1.latestVersion='1.4.0'：detail 与 releases 均空 → 落 item.latestVersion
    await openInstallAndWait('ChatOps');
    clickInstallOk();
    await waitFor(() =>
      expect(mInstall).toHaveBeenCalledWith(expect.objectContaining({ releaseVersion: '1.4.0' })),
    );
    expect(await screen.findByText('已提交安装：ChatOps')).toBeInTheDocument();
  });

  it('成功 message 的 displayName 空兜底 name：安装 wiki → 「已提交安装：wiki」', async () => {
    // 同 fixture 顺带覆盖标签列组合翼：tags 空 + defaultInstall true → 只显金标、无 '-' 兜底
    mList.mockResolvedValue({
      total: 1,
      items: [{ ...item2, installed: false, defaultInstall: true }],
    });
    mDetail.mockResolvedValue({
      item: undefined,
      releases: [],
      manifest: undefined,
      capabilities: [],
    });
    mReleases.mockResolvedValue({ total: 0, releases: [] });
    renderPage();

    expect(await screen.findByText('wiki', { selector: 'strong' })).toBeInTheDocument();
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(1));

    // wiki 与 id 列 secondary 同文：行定位用 strong selector（openInstallAndWait 不适用）
    expect(screen.getByText('默认安装')).toBeInTheDocument();
    expect(screen.queryByText('-', { selector: 'td span' })).not.toBeInTheDocument();
    const row = screen.getByText('wiki', { selector: 'strong' }).closest('tr') as HTMLElement;
    fireEvent.click(within(row).getByRole('button', { name: '安装' }));
    expect(await screen.findByText('安装扩展: wiki')).toBeInTheDocument();
    expect(await screen.findByDisplayValue('system')).toBeInTheDocument();
    clickInstallOk();

    expect(await screen.findByText('已提交安装：wiki')).toBeInTheDocument();
  });
});

describe('扩展商店 权限与跳转', () => {
  it('canExtensionsManage=false：未安装行的安装按钮也禁用，详情仍可用', async () => {
    mockCanManage.mockReturnValue(false);
    renderPage();
    await waitFirstLoad();

    const rowChat = screen.getByText('ChatOps').closest('tr') as HTMLElement;
    expect(within(rowChat).getByRole('button', { name: '安装' })).toBeDisabled();
    expect(within(rowChat).getByRole('button', { name: '详情' })).toBeEnabled();

    fireEvent.click(within(rowChat).getByRole('button', { name: '安装' }));
    await new Promise((r) => setTimeout(r, 30));
    expect(mDetail).not.toHaveBeenCalled();
    expect(mReleases).not.toHaveBeenCalled();
  });

  it('定位 Alert 的「查看安装列表」→ history.push 安装页', async () => {
    renderPage();
    await waitFirstLoad();

    fireEvent.click(screen.getByRole('button', { name: '查看安装列表' }));
    expect(mockHistoryPush).toHaveBeenCalledWith('/system/extensions/installations');
  });
});

// ---- OPEN-ISSUES #46 批次 3 写路径 UI 接线：登记 / 上下架 / 发布版本 / 删除 ----

/** 打开某行「更多」菜单并点指定项（前一个菜单可能残留在 DOM，取最后一个可见菜单里的目标项） */
async function openMoreAndClick(rowTitle: string, menuItem: string) {
  const row = screen.getByText(rowTitle).closest('tr') as HTMLElement;
  fireEvent.click(within(row).getByRole('button', { name: 'more-actions' }));
  const item = await waitFor(() => {
    const items = Array.from(
      document.querySelectorAll(
        '.ant-dropdown:not(.ant-dropdown-hidden) .ant-dropdown-menu-title-content',
      ),
    ).filter((el) => el.textContent === menuItem);
    expect(items.length).toBeGreaterThan(0);
    return items[items.length - 1] as HTMLElement;
  });
  fireEvent.click(item);
}

/** 点最近弹窗 footer 的主按钮（「确定」被 antd 双中文字符插空，按选择器点） */
function clickModalOk() {
  const ok = document.querySelector('.ant-modal-footer .ant-btn-primary') as HTMLButtonElement;
  expect(ok).not.toBeNull();
  fireEvent.click(ok);
}

/**
 * 定位删除确认弹窗：antd 6 的 modal.confirm 会把标题同时渲染进
 * .ant-modal-title 与 .ant-modal-confirm-title（全局 findByText 命中双份），
 * 须按 .ant-modal-confirm 容器 + content 内条目名圈定目标弹窗。
 */
async function findDeleteConfirm(itemName: string) {
  return await waitFor(() => {
    const confirms = Array.from(document.querySelectorAll('.ant-modal-confirm'));
    const target = confirms.find((c) => c.textContent?.includes(itemName));
    expect(target).toBeTruthy();
    return target as HTMLElement;
  });
}

describe('扩展商店 管理动作（登记/上下架/发布/删除）', () => {
  it('登记：表单提交载荷（extensionId trim + 默认 kind/status）+ 成功提示 + reload', async () => {
    renderPage();
    await waitFirstLoad();

    fireEvent.click(screen.getByRole('button', { name: /登记扩展/ }));
    expect(await screen.findByText('登记扩展到目录')).toBeInTheDocument();
    fireEvent.change(screen.getByPlaceholderText('com.example.myextension'), {
      target: { value: '  com.example.new  ' },
    });
    clickModalOk();

    await waitFor(() =>
      expect(mCreate).toHaveBeenCalledWith(
        expect.objectContaining({
          extensionId: 'com.example.new',
          kind: 'community',
          status: 'active',
        }),
      ),
    );
    expect(await screen.findByText('已登记到目录')).toBeInTheDocument();
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(2));
  });

  it('登记重复 409：冲突文案、弹窗不关、不 reload', async () => {
    mCreate.mockRejectedValueOnce({ response: { status: 409 } });
    renderPage();
    await waitFirstLoad();

    fireEvent.click(screen.getByRole('button', { name: /登记扩展/ }));
    fireEvent.change(await screen.findByPlaceholderText('com.example.myextension'), {
      target: { value: 'com.example.dup' },
    });
    clickModalOk();

    expect(await screen.findByText('登记失败：该扩展 ID 已存在于目录')).toBeInTheDocument();
    expect(screen.getByText('登记扩展到目录')).toBeInTheDocument();
    expect(mList).toHaveBeenCalledTimes(1);
  });

  it('下架：行更多菜单提交 status=delisted 并 reload；上架对称（inactive 行）', async () => {
    renderPage();
    await waitFirstLoad();

    await openMoreAndClick('ChatOps', '下架');
    await waitFor(() => expect(mUpdate).toHaveBeenCalledWith('chatops', { status: 'delisted' }));
    expect(await screen.findByText('已下架：ChatOps')).toBeInTheDocument();
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(2));

    await openMoreAndClick('Legacy', '上架');
    await waitFor(() => expect(mUpdate).toHaveBeenCalledWith('legacy', { status: 'active' }));
    expect(await screen.findByText('已上架：Legacy')).toBeInTheDocument();
  });

  it('发布版本：manifest 非法被前端拦截不调接口；合法 JSON 解析进载荷', async () => {
    renderPage();
    await waitFirstLoad();

    await openMoreAndClick('ChatOps', '发布版本');
    expect(await screen.findByText('发布版本: ChatOps')).toBeInTheDocument();

    // 非法 manifest：validator 拦截、不调接口
    fireEvent.change(screen.getByPlaceholderText('1.2.3'), { target: { value: '1.5.0' } });
    fireEvent.change(screen.getByPlaceholderText('{"capabilities": []}'), {
      target: { value: 'not-json' },
    });
    clickModalOk();
    await waitFor(() => expect(screen.getByText('Manifest JSON 格式不正确')).toBeInTheDocument());
    expect(mPublish).not.toHaveBeenCalled();

    // 合法 manifest：解析为对象进载荷
    fireEvent.change(screen.getByPlaceholderText('{"capabilities": []}'), {
      target: { value: '{"capabilities":["cap.echo"]}' },
    });
    clickModalOk();
    await waitFor(() =>
      expect(mPublish).toHaveBeenCalledWith(
        'chatops',
        expect.objectContaining({
          version: '1.5.0',
          releaseChannel: 'stable',
          manifest: { capabilities: ['cap.echo'] },
        }),
      ),
    );
    expect(await screen.findByText('版本已发布')).toBeInTheDocument();
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(2));
  });

  it('删除：确认弹窗后调用删除；409 冲突透出「先卸载」文案', async () => {
    renderPage();
    await waitFirstLoad();

    await openMoreAndClick('ChatOps', '删除');
    const confirm1 = await findDeleteConfirm('ChatOps');
    fireEvent.click(
      confirm1.querySelector('.ant-modal-confirm-btns .ant-btn-dangerous') as HTMLButtonElement,
    );
    await waitFor(() => expect(mDelete).toHaveBeenCalledWith('chatops'));
    expect(await screen.findByText('已删除（含历史版本）')).toBeInTheDocument();

    // 活跃安装阻止：409
    mDelete.mockRejectedValueOnce({ response: { status: 409 } });
    await openMoreAndClick('Legacy', '删除');
    const confirm2 = await findDeleteConfirm('Legacy');
    fireEvent.click(
      confirm2.querySelector('.ant-modal-confirm-btns .ant-btn-dangerous') as HTMLButtonElement,
    );
    expect(await screen.findByText('删除失败：存在活跃安装实例，请先卸载')).toBeInTheDocument();
  });

  it('导入扩展包：上传 .tgz 调导入接口，成功提示带版本号并 reload', async () => {
    renderPage();
    await waitFirstLoad();

    expect(screen.getByText('导入扩展包')).toBeInTheDocument();
    const input = document.querySelector('input[type="file"]') as HTMLInputElement;
    expect(input).not.toBeNull();
    const pack = new File([new Uint8Array([0x1f, 0x8b])], 'chatops-2.0.0.tgz', {
      type: 'application/gzip',
    });
    fireEvent.change(input, { target: { files: [pack] } });

    await waitFor(() => expect(mImport).toHaveBeenCalledWith(pack));
    expect(await screen.findByText('已导入并发布版本 2.0.0')).toBeInTheDocument();
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(2));
  });

  it('导入扩展包 409：版本已存在透出冲突文案，不 reload', async () => {
    mImport.mockRejectedValueOnce({ response: { status: 409 } });
    renderPage();
    await waitFirstLoad();

    const input = document.querySelector('input[type="file"]') as HTMLInputElement;
    fireEvent.change(input, {
      target: {
        files: [new File([new Uint8Array([0x1f, 0x8b])], 'chatops-2.0.0.tgz')],
      },
    });
    expect(await screen.findByText('导入失败：该版本已存在')).toBeInTheDocument();
    expect(mList).toHaveBeenCalledTimes(1);
  });
});
