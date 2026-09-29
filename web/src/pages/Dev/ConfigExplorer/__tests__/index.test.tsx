/**
 * 配置中心浏览器页单测（覆盖率巡检：Dev/ConfigExplorer/index.tsx 599 行 0% →
 * 收口，ConfigExplorer 簇首发；SourceManageModal 本体另见
 * SourceManageModal.test.tsx，其中本页为桩替身）。
 *
 * 锁定契约：
 * - 挂载链：listGamesMeta → 全局 scope 同步（gameId/env 跟随）→
 *   loadSources(game, env)（缺游戏/环境 → setSources([]) 短路）→ 默认
 *   sourceId 取 items[0] → reloadTree(sourceId, '')；
 * - 工具栏三下拉：游戏（displayName label）、环境（当前游戏 envs 派生）、
 *   数据源（类型图标 + 可写/只读 Tag），currentSource 类型 meta 文案
 *   （Git 仓库 · 只读浏览分支目录）；切游戏/环境重拉源、切源重拉树；
 * - 文件打开矩阵：路径/格式 Tag/humanSize 三段位（B/KB/MB）+ langOf 全
 *   switch 臂（json/yaml/xml/ini/lua/python/yml→yaml/py→python/其余
 *   plaintext）、只读文件无应急按钮 + 编辑器 readOnly、可写文件应急入口；
 * - 目录树懒加载：expand → listConfigTree(sourceId, dir) → 递归挂子节点；
 * - xlsx 预览：base64 → atob → XLSX.read → sheet_to_json → 表格列
 *   「列 N」与单元格值（真实 xlsx 库造 fixture：满表/空 sheet/参差
 *   中洞+短行）；
 * - 应急写回流：编辑文本 → Popconfirm → Modal 通知文案（源名/类型/路径）→
 *   取消关流（不触达服务、编辑值保留可重开）→ 空 reason 拦截警告 →
 *   writeConfigFile(sourceId, path, content, reason) →
 *   「已写回」+ 弹窗关闭 + 重开文件；失败两翼（Error.message / 非 Error
 *   兜底「写回失败」）弹窗保持；
 * - 失败翼：listConfigSources/listConfigTree/readConfigFile 各自静默 catch
 *   文案；空源 → 树区 Empty；canDevManage false → 管理按钮禁用 + 应急
 *   按钮隐藏；管理弹窗挂载/OnChanged 重拉/OnClose 卸载（桩替身）。
 *
 * mock 口径：services/api/configExplorer 四函数 + services/api/games +
 * @/hooks/useScopeReload（useScope 返回可控 scope）+ @umijs/max 本地 mock
 * （defaultMessage 即文案）jest.mock；CodeEditor 桩为受控 textarea（真实
 * Monaco 在 jsdom 不可用）；SourceManageModal 桩替身；xlsx/antd/
 * pro-components 走真实实现。
 *
 * 坑实证（antd6 沿用）：工具栏 Select mouseDown 落 .ant-select 根 + 点可见
 * option content，两次连开留 ≥60ms 时间隙（rc-select 宏任务竞态）；Tree 展开
 * 点 .ant-tree-switcher、选文件点 title 文本；Modal okText 双字中文会插空格
 * （「写 回」），danger 主按钮锚 .ant-modal-footer .ant-btn-dangerous；
 * Popconfirm 确认按钮 name=/确/。
 *
 * 登记不可达（防御分支，不造假用例不删分支）：
 * - loadSources `!g || !e` 短路体（L172-175）：唯一调用方挂载 effect
 *   `if (game && env)` 先行判空，双参恒真值（scope 空态在 effect 处即短路，
 *   loadSources 不被调）；
 * - loadDir/openFile/doSave 三处 `!sourceId` 守卫：树/文件/应急按钮仅在
 *   sources 非空（sourceId 已随 items[0].id 落定）时渲染或被调用；doSave
 *   的 `!file` 同理——应急按钮仅在 file 态渲染；
 * - onSelect `Array.isArray(keys) ? keys[0] : keys` 非 array 臂：antd Tree
 *   单选模式 onSelect 恒传 Key[]；
 * - xlsx dataRows `(r || [])` 右翼：sheet_to_json(header:1) 恒产数组；
 * - xlsx `String(c ?? '')` 右翼：稀疏洞被 Array.map 跳过（回调不触达），
 *   sheet_to_json 亦不产显式 null，c 恒有值。
 */
import React from 'react';
import { configure, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { App } from 'antd';
import * as XLSX from 'xlsx';
import ConfigExplorerPage from '../index';
import type { ConfigExplorerFile, ConfigSourceBinding } from '@/services/api/configExplorer';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

jest.mock('@/services/api/configExplorer', () => ({
  listConfigSources: jest.fn(),
  listConfigTree: jest.fn(),
  readConfigFile: jest.fn(),
  writeConfigFile: jest.fn(),
  upsertConfigSource: jest.fn(),
  deleteConfigSource: jest.fn(),
}));

jest.mock('@/services/api/games', () => ({
  listGamesMeta: jest.fn(),
}));

// scope 可控：default demo/prod；各用例改 mockScope 后 render
const mockScope = { gameId: 'demo', env: 'prod' };
jest.mock('@/hooks/useScopeReload', () => ({
  useScope: () => ({ scope: mockScope, scopeKey: `${mockScope.gameId}:${mockScope.env}` }),
}));

jest.mock('@/components/MonacoDynamic', () => ({
  CodeEditor: ({
    value,
    onChange,
    language,
    readOnly,
  }: {
    value: string;
    onChange?: (v: string) => void;
    language?: string;
    readOnly?: boolean;
  }) => (
    <textarea
      data-testid="code-editor"
      data-language={language}
      data-readonly={String(!!readOnly)}
      value={value}
      onChange={(e) => onChange?.(e.target.value)}
    />
  ),
}));

jest.mock('../SourceManageModal', () => ({
  __esModule: true,
  default: ({
    gameId,
    env,
    onClose,
    onChanged,
  }: {
    gameId: string;
    env: string;
    onClose: () => void;
    onChanged: () => void;
  }) => (
    <div data-testid="source-manage-stub">
      <span>
        {gameId}/{env}
      </span>
      <button onClick={onChanged}>stub-changed</button>
      <button onClick={onClose}>stub-close</button>
    </div>
  ),
}));

const mockIntl = {
  formatMessage: (opts: { defaultMessage?: string }, values?: Record<string, string | number>) => {
    let msg = opts.defaultMessage ?? '';
    if (values) {
      for (const [k, v] of Object.entries(values)) {
        msg = msg.split(`{${k}}`).join(String(v));
      }
    }
    return msg;
  },
};
let mockCanDevManage = true;

jest.mock('@umijs/max', () => ({
  FormattedMessage: ({ defaultMessage }: { defaultMessage?: string }) => (
    <>{defaultMessage ?? ''}</>
  ),
  useIntl: () => mockIntl,
  useAccess: () => ({ canDevManage: mockCanDevManage }),
}));

import {
  listConfigSources,
  listConfigTree,
  readConfigFile,
  writeConfigFile,
} from '@/services/api/configExplorer';
import { listGamesMeta } from '@/services/api/games';

const mSources = listConfigSources as jest.MockedFunction<typeof listConfigSources>;
const mTree = listConfigTree as jest.MockedFunction<typeof listConfigTree>;
const mRead = readConfigFile as jest.MockedFunction<typeof readConfigFile>;
const mWrite = writeConfigFile as jest.MockedFunction<typeof writeConfigFile>;
const mGames = listGamesMeta as jest.MockedFunction<typeof listGamesMeta>;

const s1: ConfigSourceBinding = {
  id: 1,
  gameId: 'demo',
  env: 'prod',
  name: 'git-main',
  type: 'git',
  config: '{}',
  writable: true,
  createdAt: '2026-09-01T00:00:00Z',
  updatedAt: '2026-09-01T00:00:00Z',
};
const s2: ConfigSourceBinding = {
  id: 2,
  gameId: 'demo',
  env: 'prod',
  name: 'redis-bus',
  type: 'redis',
  config: '{}',
  writable: false,
  createdAt: '2026-09-01T00:00:00Z',
  updatedAt: '2026-09-01T00:00:00Z',
};

const rootEntries = [
  { name: 'conf', path: 'conf', dir: true, size: 0 },
  { name: 'app.json', path: 'app.json', dir: false, size: 512 },
  { name: 'big.yml', path: 'big.yml', dir: false, size: 2048 },
  { name: 'huge.py', path: 'huge.py', dir: false, size: 3 * 1024 * 1024 },
  { name: 'plain.txt', path: 'plain.txt', dir: false, size: 100 },
  { name: 'map.xml', path: 'map.xml', dir: false, size: 40 },
  { name: 'cfg.ini', path: 'cfg.ini', dir: false, size: 30 },
  { name: 'doc.yaml', path: 'doc.yaml', dir: false, size: 20 },
  { name: 'script.python', path: 'script.python', dir: false, size: 15 },
  { name: 'data.xlsx', path: 'data.xlsx', dir: false, size: 600 },
  { name: 'empty.xlsx', path: 'empty.xlsx', dir: false, size: 60 },
  { name: 'ragged.xlsx', path: 'ragged.xlsx', dir: false, size: 61 },
];
const childEntries = [
  { name: 'sub', path: 'conf/sub', dir: true, size: 0 },
  { name: 'inner.lua', path: 'conf/inner.lua', dir: false, size: 10 },
];
const grandEntries = [{ name: 'deep.json', path: 'conf/sub/deep.json', dir: false, size: 8 }];

// 真实 xlsx 库构造 base64 fixture：满表（列头 h1/h2 + 数据行）、空 sheet、
// 参差行（第二行缺列 → undefined → '' 兜底臂）
const buildXlsxBase64 = (rows: unknown[][]) => {
  const wb = XLSX.utils.book_new();
  XLSX.utils.book_append_sheet(wb, XLSX.utils.aoa_to_sheet(rows), 'Sheet1');
  return XLSX.write(wb, { type: 'base64', bookType: 'xlsx' }) as string;
};
const xlsxFixture = buildXlsxBase64([
  ['h1', 'h2'],
  ['v1', 'v2'],
]);
const emptyXlsxFixture = buildXlsxBase64([]);
// 行 2 中洞（sheet_to_json 稀疏 → c undefined → String(c ?? '') 右翼）+
// 行 3 短行（r[i] undefined → r[i] ?? '' 右翼）
const raggedXlsxFixture = buildXlsxBase64([['h1', 'h2', 'h3'], ['v1', null, 'v3'], ['x']]);

const FILE_MAP: Record<string, ConfigExplorerFile> = {
  'app.json': { path: 'app.json', format: 'json', text: '{"k":1}', size: 512, writable: true },
  'big.yml': { path: 'big.yml', format: 'yml', text: 'a: 1', size: 2048, writable: false },
  'huge.py': { path: 'huge.py', format: 'py', text: 'x=1', size: 3 * 1024 * 1024, writable: false },
  'plain.txt': { path: 'plain.txt', format: 'txt', text: 'hello', size: 100, writable: false },
  'map.xml': { path: 'map.xml', format: 'xml', text: '<x/>', size: 40, writable: false },
  'cfg.ini': { path: 'cfg.ini', format: 'ini', text: 'k=1', size: 30, writable: false },
  'doc.yaml': { path: 'doc.yaml', format: 'yaml', text: 'b: 2', size: 20, writable: false },
  'script.python': {
    path: 'script.python',
    format: 'python',
    text: 'y=2',
    size: 15,
    writable: false,
  },
  'data.xlsx': {
    path: 'data.xlsx',
    format: 'xlsx',
    base64: xlsxFixture,
    size: 600,
    writable: false,
  },
  'empty.xlsx': {
    path: 'empty.xlsx',
    format: 'xlsx',
    base64: emptyXlsxFixture,
    size: 60,
    writable: false,
  },
  'ragged.xlsx': {
    path: 'ragged.xlsx',
    format: 'xlsx',
    base64: raggedXlsxFixture,
    size: 61,
    writable: false,
  },
  'conf/inner.lua': {
    path: 'conf/inner.lua',
    format: 'lua',
    text: 'local x=1',
    size: 10,
    writable: false,
  },
  'conf/sub/deep.json': {
    path: 'conf/sub/deep.json',
    format: 'json',
    text: '{"d":1}',
    size: 8,
    writable: false,
  },
};

beforeEach(() => {
  mockScope.gameId = 'demo';
  mockScope.env = 'prod';
  mockCanDevManage = true;
  jest.clearAllMocks();
  mGames.mockResolvedValue({
    games: [
      { name: 'demo', displayName: 'Demo Game', envs: ['prod', 'dev'] },
      { name: 'solo', displayName: 'Solo Game', envs: ['prod'] },
      // 覆盖翼：displayName 缺省 → label 回退 name；双缺省 → label/value 双 '' 臂
      { name: 'bare', envs: ['prod'] },
      { envs: ['prod'] },
    ],
  });
  mSources.mockResolvedValue({ items: [s1, s2] });
  mTree.mockImplementation(async (sourceId: number, dir: string) => ({
    items: dir === '' ? rootEntries : dir === 'conf' ? childEntries : grandEntries,
  }));
  mRead.mockImplementation(async (_sourceId: number, path: string) => FILE_MAP[path]);
  mWrite.mockResolvedValue(undefined);
});

function renderPage() {
  return render(
    <App>
      <ConfigExplorerPage />
    </App>,
  );
}

/** 等挂载链落定：sources 首拉 + 树首拉 */
async function waitLoad() {
  await waitFor(() => expect(mSources).toHaveBeenCalledWith({ gameId: 'demo', env: 'prod' }));
  await waitFor(() => expect(mTree).toHaveBeenCalledWith(1, ''));
  expect(await screen.findByText('app.json')).toBeInTheDocument();
}

/** 打开树上的文件（点 title 文本） */
async function openTreeFile(name: string) {
  fireEvent.click(screen.getByText(name));
  await waitFor(() => expect(mRead).toHaveBeenCalled());
  await waitFor(() => expect(screen.getByTestId('code-editor')).toBeInTheDocument());
}

/** 工具栏第 idx 个 Select 选 option（antd6：mouseDown 根 + 点可见 option content；
 * 连续两次开下拉留 ≥60ms 时间隙避 rc-select 宏任务竞态） */
async function pickToolbarSelect(idx: number, matcher: (t: string) => boolean) {
  await new Promise((r) => setTimeout(r, 60));
  const selects = document.querySelectorAll('.ant-select');
  fireEvent.mouseDown(selects[idx] as HTMLElement);
  const dropdown = await waitFor(() => {
    const visible = Array.from(document.querySelectorAll('.ant-select-dropdown')).find(
      (d) => !d.className.includes('ant-select-dropdown-hidden'),
    ) as HTMLElement;
    expect(visible).not.toBeUndefined();
    return visible;
  });
  const option = Array.from(dropdown.querySelectorAll('.ant-select-item-option-content')).find(
    (o) => matcher(o.textContent ?? ''),
  ) as HTMLElement;
  expect(option).not.toBeUndefined();
  fireEvent.click(option);
}

describe('配置浏览器 初始渲染', () => {
  it('挂载链 + 三下拉 + 源 meta + 根节点 + 空文件提示', async () => {
    renderPage();
    await waitLoad();

    // 三下拉选中态
    const selects = document.querySelectorAll('.ant-select');
    expect((selects[0] as HTMLElement).querySelector('.ant-select-content')?.textContent).toContain(
      'Demo Game',
    );
    expect((selects[1] as HTMLElement).querySelector('.ant-select-content')?.textContent).toBe(
      'prod',
    );
    expect((selects[2] as HTMLElement).querySelector('.ant-select-content')?.textContent).toContain(
      'git-main',
    );

    // currentSource meta：Git 仓库 · 只读浏览分支目录
    expect(screen.getByText('Git 仓库 · 只读浏览分支目录')).toBeInTheDocument();

    // 树根节点：目录 + 文件
    expect(screen.getByText('conf')).toBeInTheDocument();

    // 文件区空提示
    expect(screen.getByText('选择左侧文件查看在线配置')).toBeInTheDocument();

    // 工具按钮 + 刷新重拉树
    expect(screen.getByRole('button', { name: /管理数据源/ })).toBeEnabled();
    fireEvent.click(screen.getByRole('button', { name: /刷新/ }));
    await waitFor(() => expect(mTree).toHaveBeenCalledTimes(2));
    expect(mTree).toHaveBeenLastCalledWith(1, '');
  });

  it('全局 scope 缺省（gameId/env 空）：不拉源 → 空源 Empty', async () => {
    mockScope.gameId = '';
    mockScope.env = '';
    renderPage();
    await waitFor(() => expect(mGames).toHaveBeenCalledTimes(1));
    // env 空 → loadSources 短路 setSources([])：源区 Empty、树不渲染
    await waitFor(() => expect(mSources).not.toHaveBeenCalled());
    expect(screen.getByText('暂无数据源，请先管理数据源添加')).toBeInTheDocument();
    expect(screen.queryByText('conf')).not.toBeInTheDocument();
  });

  it('全局 scope 指向他游戏：首拉按 scope，games 回填后回切首游戏', async () => {
    mockScope.gameId = 'solo';
    renderPage();
    await waitFor(() => expect(mSources).toHaveBeenCalledTimes(2));
    expect(mSources.mock.calls[0][0]).toEqual({ gameId: 'solo', env: 'prod' });
    expect(mSources).toHaveBeenLastCalledWith({ gameId: 'demo', env: 'prod' });
  });
});

describe('配置浏览器 文件打开矩阵', () => {
  it('可写 json：路径/格式 Tag/humanSize B 段/应急按钮/编辑器值与语言', async () => {
    renderPage();
    await waitLoad();
    await openTreeFile('app.json');

    // 树节点 + 文件头双实例
    expect(screen.getAllByText('app.json')).toHaveLength(2);
    expect(screen.getByText('json').closest('.ant-tag')).toBeInTheDocument();
    expect(screen.getByText('512 B')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /应急编辑/ })).toBeInTheDocument();
    const editor = screen.getByTestId('code-editor');
    expect(editor).toHaveValue('{"k":1}');
    expect(editor).toHaveAttribute('data-language', 'json');
    expect(editor).toHaveAttribute('data-readonly', 'false');
  });

  it('humanSize 三段位 + langOf 全 switch 臂 + 只读无应急按钮', async () => {
    renderPage();
    await waitLoad();

    // 2.0 KB / yml→yaml / 只读
    await openTreeFile('big.yml');
    expect(screen.getByText('2.0 KB')).toBeInTheDocument();
    expect(screen.getByTestId('code-editor')).toHaveAttribute('data-language', 'yaml');
    expect(screen.queryByRole('button', { name: /应急编辑/ })).not.toBeInTheDocument();
    expect(screen.getByTestId('code-editor')).toHaveAttribute('data-readonly', 'true');

    // 3.0 MB / py→python
    await openTreeFile('huge.py');
    expect(screen.getByText('3.0 MB')).toBeInTheDocument();
    expect(screen.getByTestId('code-editor')).toHaveAttribute('data-language', 'python');

    // txt → 默认 plaintext
    await openTreeFile('plain.txt');
    expect(screen.getByText('100 B')).toBeInTheDocument();
    expect(screen.getByTestId('code-editor')).toHaveAttribute('data-language', 'plaintext');

    // xml / ini / yaml 原值 / python 原值
    await openTreeFile('map.xml');
    expect(screen.getByTestId('code-editor')).toHaveAttribute('data-language', 'xml');
    await openTreeFile('cfg.ini');
    expect(screen.getByTestId('code-editor')).toHaveAttribute('data-language', 'ini');
    await openTreeFile('doc.yaml');
    expect(screen.getByTestId('code-editor')).toHaveAttribute('data-language', 'yaml');
    await openTreeFile('script.python');
    expect(screen.getByTestId('code-editor')).toHaveAttribute('data-language', 'python');
  });

  it('目录懒加载：expand conf → 子节点 inner.lua（lua 臂）', async () => {
    renderPage();
    await waitLoad();

    const switcher = screen
      .getByText('conf')
      .closest('.ant-tree-treenode')
      ?.querySelector('.ant-tree-switcher') as HTMLElement;
    fireEvent.click(switcher);
    await waitFor(() => expect(mTree).toHaveBeenCalledWith(1, 'conf'));
    await openTreeFile('inner.lua');
    expect(screen.getByTestId('code-editor')).toHaveAttribute('data-language', 'lua');
    expect(screen.getByText('10 B')).toBeInTheDocument();

    // 嵌套目录：expand conf/sub → 孙节点可见（updateChildren 递归臂）
    const subSwitcher = screen
      .getByText('sub')
      .closest('.ant-tree-treenode')
      ?.querySelector('.ant-tree-switcher') as HTMLElement;
    fireEvent.click(subSwitcher);
    await waitFor(() => expect(mTree).toHaveBeenCalledWith(1, 'conf/sub'));
    await openTreeFile('deep.json');
    expect(screen.getByTestId('code-editor')).toHaveAttribute('data-language', 'json');
  });

  it('xlsx 预览：列 1/列 2 表头 + 单元格值，无编辑器', async () => {
    renderPage();
    await waitLoad();

    fireEvent.click(screen.getByText('data.xlsx'));
    expect(await screen.findAllByText('列 1')).not.toHaveLength(0);
    expect(screen.getAllByText('列 2')).not.toHaveLength(0);
    expect(screen.getByText('h1')).toBeInTheDocument();
    expect(screen.getByText('v2')).toBeInTheDocument();
    expect(screen.queryByTestId('code-editor')).not.toBeInTheDocument();

    // 空 sheet：rows [] → (rows[0] || []) 右翼 → 无列头空表
    fireEvent.click(screen.getByText('empty.xlsx'));
    await waitFor(() => expect(mRead).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(screen.queryByText('列 1')).not.toBeInTheDocument());
    expect(screen.queryByText('h1')).not.toBeInTheDocument();

    // 参差行：行 2 中洞渲染 ''、行 3 短行补 ''
    fireEvent.click(screen.getByText('ragged.xlsx'));
    await waitFor(() => expect(mRead).toHaveBeenCalledTimes(3));
    expect(await screen.findAllByText('列 3')).not.toHaveLength(0);
    expect(screen.getByText('v3')).toBeInTheDocument();
    expect(screen.getByText('x')).toBeInTheDocument();
    expect(screen.queryByText('v2')).not.toBeInTheDocument();
  });
});

describe('配置浏览器 应急写回', () => {
  it('主链：编辑 → Popconfirm → 取消可重开 → reason 必填拦截 → 写回载荷 → 已写回', async () => {
    renderPage();
    await waitLoad();
    await openTreeFile('app.json');

    fireEvent.change(screen.getByTestId('code-editor'), { target: { value: '{"k":2}' } });
    fireEvent.click(screen.getByRole('button', { name: /应急编辑/ }));
    expect(await screen.findByText('应急编辑并写回？')).toBeInTheDocument();
    let popover = Array.from(document.querySelectorAll('.ant-popover')).find(
      (p) => !p.className.includes('ant-popover-hidden'),
    ) as HTMLElement;
    fireEvent.click(within(popover).getByRole('button', { name: /确/ }));

    // 写回弹窗：标题 + 通知文案（源名/类型/路径）
    await waitFor(() =>
      expect(document.querySelector('.ant-modal-title')?.textContent).toBe('应急写回'),
    );
    let modal = document.querySelector('.ant-modal') as HTMLElement;
    expect(within(modal).getByText('git-main')).toBeInTheDocument();
    expect(within(modal).getByText('app.json')).toBeInTheDocument();

    // 取消：弹窗关闭、不触达服务；编辑值保留可重开
    fireEvent.click(within(modal).getByRole('button', { name: /取/ }));
    await waitFor(() =>
      expect(
        screen.queryByPlaceholderText('应急原因（必填，如：线上活动奖励配置错误紧急修正）'),
      ).not.toBeInTheDocument(),
    );
    expect(mWrite).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole('button', { name: /应急编辑/ }));
    expect(await screen.findByText('应急编辑并写回？')).toBeInTheDocument();
    popover = Array.from(document.querySelectorAll('.ant-popover')).find(
      (p) => !p.className.includes('ant-popover-hidden'),
    ) as HTMLElement;
    fireEvent.click(within(popover).getByRole('button', { name: /确/ }));
    await waitFor(() =>
      expect(document.querySelector('.ant-modal-title')?.textContent).toBe('应急写回'),
    );
    modal = document.querySelector('.ant-modal') as HTMLElement;

    // 空 reason：警告拦截，不触达服务
    const ok = modal.querySelector('.ant-modal-footer .ant-btn-dangerous') as HTMLElement;
    fireEvent.click(ok);
    expect(await screen.findByText('应急原因必填（将记入审计）')).toBeInTheDocument();
    expect(mWrite).not.toHaveBeenCalled();

    // 填 reason → 写回成功 + 弹窗关闭 + 文件重开
    fireEvent.change(
      screen.getByPlaceholderText('应急原因（必填，如：线上活动奖励配置错误紧急修正）'),
      { target: { value: '  线上紧急修正  ' } },
    );
    fireEvent.click(ok);
    await waitFor(() => expect(mWrite).toHaveBeenCalledTimes(1));
    expect(mWrite).toHaveBeenCalledWith({
      sourceId: 1,
      path: 'app.json',
      content: '{"k":2}',
      reason: '线上紧急修正',
    });
    expect(await screen.findByText('已写回')).toBeInTheDocument();
    await waitFor(() =>
      expect(
        screen.queryByPlaceholderText('应急原因（必填，如：线上活动奖励配置错误紧急修正）'),
      ).not.toBeInTheDocument(),
    );
    await waitFor(() => expect(mRead).toHaveBeenCalledTimes(2)); // 重开刷新
  });

  it('失败两翼：Error.message 透传 / 非 Error 兜底「写回失败」，弹窗保持', async () => {
    renderPage();
    await waitLoad();
    await openTreeFile('app.json');

    fireEvent.click(screen.getByRole('button', { name: /应急编辑/ }));
    expect(await screen.findByText('应急编辑并写回？')).toBeInTheDocument();
    const popover = Array.from(document.querySelectorAll('.ant-popover')).find(
      (p) => !p.className.includes('ant-popover-hidden'),
    ) as HTMLElement;
    fireEvent.click(within(popover).getByRole('button', { name: /确/ }));
    await waitFor(() =>
      expect(document.querySelector('.ant-modal-title')?.textContent).toBe('应急写回'),
    );
    fireEvent.change(
      screen.getByPlaceholderText('应急原因（必填，如：线上活动奖励配置错误紧急修正）'),
      { target: { value: 'r1' } },
    );

    mWrite.mockRejectedValueOnce(new Error('write-x'));
    const modal = document.querySelector('.ant-modal') as HTMLElement;
    fireEvent.click(modal.querySelector('.ant-modal-footer .ant-btn-dangerous') as HTMLElement);
    expect(await screen.findByText('write-x')).toBeInTheDocument();
    expect(
      screen.getByPlaceholderText('应急原因（必填，如：线上活动奖励配置错误紧急修正）'),
    ).toBeInTheDocument();

    mWrite.mockRejectedValueOnce('plain' as never);
    fireEvent.click(modal.querySelector('.ant-modal-footer .ant-btn-dangerous') as HTMLElement);
    expect(await screen.findByText('写回失败')).toBeInTheDocument();
    expect(
      screen.getByPlaceholderText('应急原因（必填，如：线上活动奖励配置错误紧急修正）'),
    ).toBeInTheDocument();
  });
});

describe('配置浏览器 失败翼与切换', () => {
  it('sources/tree/read 三失败静默文案', async () => {
    // 源失败：sources 留空 → 源区 Empty
    mSources.mockRejectedValueOnce(new Error('src-down'));
    const r1 = renderPage();
    expect(await screen.findByText('加载数据源失败')).toBeInTheDocument();
    r1.unmount();

    // 树失败：挂载 reloadTree 即 reject
    mTree.mockRejectedValueOnce(new Error('tree-down'));
    const r2 = renderPage();
    expect(await screen.findByText('加载目录失败')).toBeInTheDocument();
    r2.unmount();

    // 文件失败
    mRead.mockRejectedValueOnce(new Error('file-down'));
    renderPage();
    await waitFor(() => expect(mTree).toHaveBeenCalled());
    fireEvent.click(screen.getByText('app.json'));
    expect(await screen.findByText('读取文件失败')).toBeInTheDocument();
  });

  it('空源列表 → 树区 Empty；切游戏/环境重拉源、切源重拉树', async () => {
    renderPage();
    await waitLoad();

    // 切数据源 → 树按新 sourceId 重拉
    await pickToolbarSelect(2, (t) => t.includes('redis-bus'));
    await waitFor(() => expect(mTree).toHaveBeenLastCalledWith(2, ''));

    // 切环境 → 源重拉（demo/dev）
    await pickToolbarSelect(1, (t) => t === 'dev');
    await waitFor(() => expect(mSources).toHaveBeenLastCalledWith({ gameId: 'demo', env: 'dev' }));

    // 切游戏 → 源重拉（env 状态保留 dev——页面不重置环境）
    await pickToolbarSelect(0, (t) => t.includes('Solo Game'));
    await waitFor(() => expect(mSources).toHaveBeenLastCalledWith({ gameId: 'solo', env: 'dev' }));

    // 空源：solo 洉 prod 拉空
    mSources.mockResolvedValue({ items: [] });
    await pickToolbarSelect(1, (t) => t === 'prod');
    await waitFor(() =>
      expect(screen.getByText('暂无数据源，请先管理数据源添加')).toBeInTheDocument(),
    );
  });

  it('canDevManage false：管理禁用 + 应急按钮隐藏（可写文件也不显示）', async () => {
    mockCanDevManage = false;
    renderPage();
    await waitLoad();

    expect(screen.getByRole('button', { name: /管理数据源/ })).toBeDisabled();
    await openTreeFile('app.json');
    expect(screen.queryByRole('button', { name: /应急编辑/ })).not.toBeInTheDocument();
    expect(screen.getByTestId('code-editor')).toHaveAttribute('data-readonly', 'true');
  });

  it('管理数据源弹窗：挂载（scope 透传）/OnChanged 重拉/OnClose 卸载', async () => {
    renderPage();
    await waitLoad();

    fireEvent.click(screen.getByRole('button', { name: /管理数据源/ }));
    const stub = await screen.findByTestId('source-manage-stub');
    expect(stub.textContent).toContain('demo/prod');

    fireEvent.click(within(stub).getByText('stub-changed'));
    await waitFor(() => expect(mSources).toHaveBeenCalledTimes(2));

    fireEvent.click(within(stub).getByText('stub-close'));
    await waitFor(() => expect(screen.queryByTestId('source-manage-stub')).not.toBeInTheDocument());
  });
});
