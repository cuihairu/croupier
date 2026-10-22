/**
 * 配置管理页单测（覆盖率补缺轮：Operations/Configs 五文件 800 行 0% → 收口，
 * 全仓零测试目录排行首位）。
 *
 * 锁定契约：
 * - 列表链：首拉空载荷 {}、六列渲染（ID/Format Tag/Game/Env/Latest/编辑）、
 *   load 失败 toast「加载失败」+ 空表、响应缺省（{} / undefined）右翼。
 * - 筛选链：搜索输入实时重拉（trim 入 idLike）、格式/游戏/环境三下拉
 *   （game/env 选项由 rows distinct filter(Boolean) 派生）、Enter 与
 *   「查询」按当前闭包参数重复拉取、「重置」清四态后回空载荷。
 * - 全局 scope 联动：setScope({gameId,env}) → 双值同步进载荷（store merge
 *   语义 + 页面只同步 truthy——scope 键清除不回拉，现状行为如实断言）。
 * - 编辑弹窗：openItem 兜底链（fmt → r.format → 'json'、content/version/
 *   gameId/env 各 ?. 翼 + r nullish 五翼）、标题 `${id} (${fmt}) v${version||''}`、
 *   getConfig 失败 toast、csv 预览（\r\n 归一 + 空行过滤 + 逗号切列）、
 *   编辑器受控（value/language/onChange）、弹窗内格式切换（langOf 矩阵 +
 *   csv 预览卸载）。
 * - 校验三态：valid=true「校验通过」、errors join('\n') 透出、无 errors
 *   「校验失败」。
 * - 保存版本：载荷（gameId/env 的 cur 空回退 toolbar 态、baseVersion
 *   version||0、message 无必填拦截——placeholder 标「必填」但 doSave 不校验，
 *   现状行为锁定）、成功 toast `已保存版本 N` + 弹窗关（destroyOnHidden）+
 *   reload + saveMsg 清空、失败 toast 弹窗保持、r?.version undefined 翼。
 * - 历史版本弹窗：listVersions 行渲染（createdAt formatDateTime / '' 双翼）、
 *   响应缺省右翼；查看（getVersion 回填 content/format、version 不动——
 *   baseVersion 语义、verOpen 关闭、value/format 空串右翼）。
 * - 版本对比：MonacoDiff 受 left/right、DiffView 真实算法（same/add-batch/
 *   del-batch/add-tail/del-tail/单点替换矩阵、del 红 #fff1f0 / add 绿
 *   #f6ffed、对侧空 div 占位）、left '' 双右翼（cur.content || '' 与
 *   (left || '')）。
 * - 回滚：confirm 标题/文案（模板串已内插版本号）、onOk 载荷（format 回退
 *   cur.format、content String(value||'')、message `rollback to v${ver}`、
 *   getVersion undefined 双右翼）、成功「已回滚」+ verOpen 关 + reload、
 *   失败「回滚失败」+ 版本弹窗保持。
 *
 * mock 口径：services/api/configs 六函数 jest.mock；@umijs/max 本地 mock
 * （含 getIntl——schema.tsx 模块顶层解析）；pro-components PageContainer /
 * MonacoDynamic 双编辑器桩替换（CodeEditor→textarea 暴露 value/language/
 * onChange，DiffEditor→div 暴露 left/right）；stores/scope 用真实单例 +
 * setScope 驱动（OperationLogs 批次先例）；antd/真实 formatDateTime。
 *
 * 边界（诚实）：
 * 1. 六处 `if (!cur) return` 守卫（validate/doSave/openVersions/viewVersion/
 *    diffWithVersion/rollbackTo）经 UI 不可达——动作按钮仅在 `{cur && ...}`
 *    分支内渲染，cur nullish 时按钮不存在，不造假用例。
 * 2. validate/openVersions/viewVersion/diffWithVersion/rollbackTo 外层
 *    async 无 try/catch——接口 reject 产生 unhandled rejection（组件现状
 *    缺陷，同 Store openDetail 巡检结论），不造假 reject 场景；有 catch 的
 *    load/openItem/doSave/回滚 onOk 失败翼均真实覆盖。
 * 3. 编辑弹窗 title 三元 `cur ? ... : ''` 的 false 翼不可达——Modal 带
 *    destroyOnHidden 且未开启时不渲染 title，cur 非空与弹窗 open 恒同态；
 *    diff 弹窗 `langOf(cur?.format || '')` 的 cur nullish 翼同理（diff 仅
 *    经 cur 态按钮打开）。
 * 4. hasMonaco() 硬编码返回 false——true 翼构造性不可达（diff.tsx 常量），
 *    因此 MonacoDiff 桩与 DiffView 恒同时渲染，按此断言。
 */
import React from 'react';
import { act, configure, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { App } from 'antd';
import OperationsConfigsPage from '../index';
import { setScope } from '@/stores/scope';
import type { ConfigDetail, ConfigItem, ConfigVersion } from '@/services/api/configs';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

jest.mock('@/services/api/configs', () => ({
  listConfigs: jest.fn(),
  getConfig: jest.fn(),
  saveConfig: jest.fn(),
  validateConfig: jest.fn(),
  listVersions: jest.fn(),
  getVersion: jest.fn(),
}));

// 工厂自包含：schema.tsx 模块顶层就调 getIntl()（import 期执行），工厂外的
// const 变量此时仍在 TDZ——intl 对象必须在工厂体内构造
jest.mock('@umijs/max', () => {
  const intl = {
    formatMessage: (
      opts: { defaultMessage?: string },
      values?: Record<string, string | number>,
    ) => {
      let msg = opts.defaultMessage ?? '';
      if (values) {
        for (const [k, v] of Object.entries(values)) msg = msg.split(`{${k}}`).join(String(v));
      }
      return msg;
    },
  };
  return {
    FormattedMessage: ({ defaultMessage }: { defaultMessage?: string }) => (
      <>{defaultMessage ?? ''}</>
    ),
    useIntl: () => intl,
    getIntl: () => intl,
  };
});

jest.mock('@ant-design/pro-components', () => ({
  PageContainer: ({ children }: { children?: React.ReactNode }) => <div>{children}</div>,
}));

jest.mock('@/components/MonacoDynamic', () => ({
  CodeEditor: ({
    value,
    language,
    onChange,
  }: {
    value?: string;
    language?: string;
    onChange?: (v: string) => void;
  }) => (
    <textarea
      data-testid="code-editor"
      data-language={language ?? ''}
      value={value ?? ''}
      onChange={(e) => onChange?.(e.target.value)}
    />
  ),
  DiffEditor: ({ left, right }: { left?: string; right?: string }) => (
    <div data-testid="monaco-diff" data-left={left ?? ''} data-right={right ?? ''} />
  ),
}));

import {
  getConfig,
  getVersion,
  listConfigs,
  listVersions,
  saveConfig,
  validateConfig,
} from '@/services/api/configs';

const mList = listConfigs as jest.MockedFunction<typeof listConfigs>;
const mGetConfig = getConfig as jest.MockedFunction<typeof getConfig>;
const mSave = saveConfig as jest.MockedFunction<typeof saveConfig>;
const mValidate = validateConfig as jest.MockedFunction<typeof validateConfig>;
const mVersions = listVersions as jest.MockedFunction<typeof listVersions>;
const mGetVersion = getVersion as jest.MockedFunction<typeof getVersion>;

const mkRow = (
  id: string,
  format: string,
  gameId = 'demo',
  env = 'prod',
  latestVersion = 1,
): ConfigItem => ({
  id,
  format,
  gameId,
  env,
  latestVersion,
  updatedAt: '2026-09-01T00:00:00Z',
  lastMessage: 'init',
  lastModifiedBy: 'admin',
});

// 八行：csv 主行 + lua（gameId/env 空走 filter(Boolean) 右翼）+ 空格式行
// （openItem 兜底链）+ yml/py/xml/ini/toml（langOf 矩阵，toml 走默认 plaintext）
const ROWS: ConfigItem[] = [
  mkRow('cfg/shop', 'csv', 'demo', 'prod', 3),
  mkRow('cfg/edge', 'lua', '', '', 7),
  mkRow('cfg/blank', '', 'demo', 'prod', 0),
  mkRow('cfg/yml', 'yml'),
  mkRow('cfg/py', 'py'),
  mkRow('cfg/xml', 'xml'),
  mkRow('cfg/ini', 'ini'),
  mkRow('cfg/toml', 'toml'),
];

const DETAIL: ConfigDetail = {
  id: 'cfg/shop',
  format: 'csv',
  content: 'h1,h2\r\n\r\n1,2',
  version: 3,
  gameId: 'demo',
  env: 'prod',
};

const mkVer = (v: number, over: Partial<ConfigVersion> = {}): ConfigVersion => ({
  key: 'cfg/shop',
  version: v,
  createdBy: 'ops',
  createdAt: `2026-09-0${v}T00:00:00Z`,
  gameId: 'demo',
  env: 'prod',
  format: 'csv',
  message: `m${v}`,
  value: '',
  ...over,
});

const V2 = mkVer(2);
const V3 = mkVer(3);

const fmtDate = (s: string) =>
  new Date(s).toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  });

function renderPage() {
  return render(
    <App>
      <OperationsConfigsPage />
    </App>,
  );
}

async function waitRows() {
  expect(await screen.findByText('cfg/shop')).toBeInTheDocument();
  await waitFor(() => expect(mList).toHaveBeenCalledTimes(1));
}

function sleep(ms: number) {
  return new Promise((r) => setTimeout(r, ms));
}

/** 工具栏第 index 个下拉（Card extra 内 DOM 序：game/env/format） */
function toolbarSelect(index: number) {
  const extra = document.querySelector('.ant-card-extra') as HTMLElement;
  expect(extra).not.toBeNull();
  return extra.querySelectorAll('.ant-select')[index] as HTMLElement;
}

/** 打开下拉并点可见 option（rc-select 关闭动画 ~300ms，重开须留时间隙） */
async function pickOption(selectRoot: HTMLElement, label: string) {
  await sleep(60);
  fireEvent.mouseDown(selectRoot);
  const item = await waitFor(() => {
    const els = Array.from(
      document.querySelectorAll(
        '.ant-select-dropdown:not(.ant-dropdown-hidden) .ant-select-item-option-content',
      ),
    ).filter((el) => el.textContent === label);
    expect(els.length).toBeGreaterThan(0);
    return els[els.length - 1] as HTMLElement;
  });
  fireEvent.click(item);
}

/** 打开某行编辑弹窗并等编辑器挂载 */
async function openEditAndWait(id: string) {
  const row = screen.getByText(id).closest('tr') as HTMLElement;
  fireEvent.click(within(row).getByRole('button', { name: /编\s*辑/ }));
  await waitFor(() => expect(screen.getByTestId('code-editor')).toBeInTheDocument());
  return screen.getByTestId('code-editor') as HTMLTextAreaElement;
}

/** 关闭最后一个弹窗（DOM 序后开者居后）并等其标题卸载（destroyOnHidden）。
 * 标题锚定 .ant-modal-title——「历史版本」与编辑弹窗内同名按钮碰撞，
 * 全文 queryByText 永不消失 */
async function closeLastModal(title: string) {
  const closes = document.querySelectorAll('.ant-modal-close');
  fireEvent.click(closes[closes.length - 1] as HTMLElement);
  await waitFor(() =>
    expect(
      Array.from(document.querySelectorAll('.ant-modal-title')).some(
        (t) => t.textContent === title,
      ),
    ).toBe(false),
  );
}

/** 版本弹窗内按版本号定位行（查 看 按钮仅存在于版本弹窗） */
function versionRow(v: number) {
  const rows = screen
    .getAllByRole('button', { name: /查\s*看/ })
    .map((b) => b.closest('tr') as HTMLElement);
  const row = rows.find((tr) => within(tr).queryByText(String(v)) !== null);
  expect(row).toBeTruthy();
  return row as HTMLElement;
}

/** diff 弹窗快照：两个 pre 面板 + 逐行文本 + 指定行文本的背景色（调用时取 DOM） */
function diffPanes() {
  const modal = (screen.getByText('版本对比') as HTMLElement).closest('.ant-modal') as HTMLElement;
  const pres = Array.from(modal.querySelectorAll('pre'));
  expect(pres.length).toBe(2);
  const lines = (pane: Element) => Array.from(pane.children).map((d) => d.textContent ?? '');
  const bgOf = (pane: Element, text: string) =>
    Array.from(pane.children)
      .filter((d) => (d.textContent ?? '') === text)
      .map((d) => (d as HTMLElement).style.background);
  return {
    left: pres[0],
    right: pres[1],
    leftLines: () => lines(pres[0]),
    rightLines: () => lines(pres[1]),
    bgOf,
  };
}

beforeEach(() => {
  jest.clearAllMocks();
  mList.mockResolvedValue({ items: ROWS });
  mGetConfig.mockResolvedValue(DETAIL);
  mSave.mockResolvedValue({ version: 4 });
  mValidate.mockResolvedValue({ valid: true, errors: [] });
  mVersions.mockResolvedValue({ key: 'cfg/shop', total: 2, versions: [V2, V3] });
  mGetVersion.mockResolvedValue(V2);
});

// scope store 是模块级单例，跨用例存活——merge 语义下须显式 undefined 复位
//（emit:false 避免触发已挂载组件的 listener）
afterEach(() => {
  setScope({ gameId: undefined, env: undefined }, { emit: false, persist: false });
});

describe('配置管理 列表与筛选', () => {
  it('首拉空载荷 + 六列渲染矩阵（Format Tag / game-env 空行 / 8 行编辑按钮）', async () => {
    renderPage();
    await waitRows();

    expect(mList).toHaveBeenCalledWith({});
    expect(screen.getByText('配置管理')).toBeInTheDocument();

    const shopRow = screen.getByText('cfg/shop').closest('tr') as HTMLElement;
    expect(within(shopRow).getByText('csv')).toBeInTheDocument();
    expect(within(shopRow).getByText('demo')).toBeInTheDocument();
    expect(within(shopRow).getByText('prod')).toBeInTheDocument();
    expect(within(shopRow).getByText('3')).toBeInTheDocument();

    // game/env 空行：filter(Boolean) 右翼（不进下拉选项）
    const edgeRow = screen.getByText('cfg/edge').closest('tr') as HTMLElement;
    expect(within(edgeRow).getByText('lua')).toBeInTheDocument();

    expect(screen.getAllByRole('button', { name: /编\s*辑/ })).toHaveLength(8);
  });

  it('筛选链：搜索实时 trim 重拉、格式/游戏/环境下拉、Enter/查询重复拉取、重置回空载荷', async () => {
    renderPage();
    await waitRows();

    // 搜索：输入即重拉（load 依赖 q），两侧空白被 trim
    fireEvent.change(screen.getByPlaceholderText('按 id 搜索'), { target: { value: '  shop  ' } });
    await waitFor(() => expect(mList).toHaveBeenLastCalledWith({ idLike: 'shop' }));

    // 格式下拉（固定 options）+ 游戏下拉（rows 派生）+ 环境下拉
    await pickOption(toolbarSelect(2), 'csv');
    await waitFor(() => expect(mList).toHaveBeenLastCalledWith({ idLike: 'shop', format: 'csv' }));
    await pickOption(toolbarSelect(0), 'demo');
    await waitFor(() =>
      expect(mList).toHaveBeenLastCalledWith({ idLike: 'shop', format: 'csv', gameId: 'demo' }),
    );
    await pickOption(toolbarSelect(1), 'prod');
    await waitFor(() =>
      expect(mList).toHaveBeenLastCalledWith({
        idLike: 'shop',
        format: 'csv',
        gameId: 'demo',
        env: 'prod',
      }),
    );

    // Enter 与查询按钮：按当前闭包参数再拉一次
    let before = mList.mock.calls.length;
    fireEvent.keyDown(screen.getByPlaceholderText('按 id 搜索'), { key: 'Enter' });
    await waitFor(() => expect(mList.mock.calls.length).toBeGreaterThan(before));
    before = mList.mock.calls.length;
    fireEvent.click(screen.getByRole('button', { name: /查\s*询/ }));
    await waitFor(() => expect(mList.mock.calls.length).toBeGreaterThan(before));

    // 重置：四态清空 + 回空载荷（旧闭包 load 先行、新空载荷 load 兜底，锚最后一次）
    fireEvent.click(screen.getByRole('button', { name: /重\s*置/ }));
    await waitFor(() => expect(mList).toHaveBeenLastCalledWith({}));
    expect((screen.getByPlaceholderText('按 id 搜索') as HTMLInputElement).value).toBe('');
  });

  it('全局 scope 联动：setScope 双值同步进载荷（merge 语义只增不减）', async () => {
    renderPage();
    await waitRows();

    act(() => setScope({ gameId: 'demo', env: 'prod' }));
    await waitFor(() => expect(mList).toHaveBeenLastCalledWith({ gameId: 'demo', env: 'prod' }));

    // 换游戏：env 保持（store merge），页面只同步 truthy 值
    act(() => setScope({ gameId: 'gameX' }));
    await waitFor(() => expect(mList).toHaveBeenLastCalledWith({ gameId: 'gameX', env: 'prod' }));
  });

  it('load 失败 toast + 响应缺省双右翼（{} / undefined）', async () => {
    mList.mockRejectedValueOnce(new Error('list down'));
    const { unmount } = renderPage();
    expect(await screen.findByText('加载失败')).toBeInTheDocument();
    expect(screen.queryByText('cfg/shop')).not.toBeInTheDocument();
    unmount();
    // clearAllMocks 不清 Once 队列也不清计数：二次渲染前归零，断言只锚第二棵树
    mList.mockClear();

    mList.mockResolvedValueOnce({} as never);
    renderPage();
    expect(await screen.findByText('配置管理')).toBeInTheDocument();
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(1));
    expect(screen.queryByText('cfg/shop')).not.toBeInTheDocument();
  });
});

describe('配置管理 编辑弹窗', () => {
  it('打开链：标题、csv 预览（\\r\\n 归一 + 空行过滤）、编辑器 value/language', async () => {
    renderPage();
    await waitRows();
    const editor = await openEditAndWait('cfg/shop');

    expect(screen.getByText('cfg/shop (csv) v3')).toBeInTheDocument();
    // csv 预览：'h1,h2\r\n\r\n1,2' → 2×2 单元格（外层列表有同名数字格，按弹窗圈定）
    const modal = (screen.getByText('cfg/shop (csv) v3') as HTMLElement).closest(
      '.ant-modal',
    ) as HTMLElement;
    expect(within(modal).getByText('h1')).toBeInTheDocument();
    expect(within(modal).getByText('h2')).toBeInTheDocument();
    expect(within(modal).getByText('1')).toBeInTheDocument();
    expect(within(modal).getByText('2')).toBeInTheDocument();
    // jsdom textarea 读值把 \r\n 归一为 \n（state 侧仍是原始 CRLF，见保存载荷断言）
    expect(editor.value).toBe('h1,h2\n\n1,2');
    expect(editor.getAttribute('data-language')).toBe('plaintext');
  });

  it('openItem 兜底链：fmt 空 → r.format → json、version 0/undefined 尾翼、r nullish 五翼', async () => {
    renderPage();
    await waitRows();

    // fmt '' + r.format 'yaml' → yaml；version undefined → 标题尾 'v'
    mGetConfig.mockResolvedValueOnce({
      id: 'cfg/blank',
      format: 'yaml',
      content: 'x: 1',
      version: undefined as unknown as number,
      gameId: '',
      env: '',
    });
    let editor = await openEditAndWait('cfg/blank');
    expect(screen.getByText('cfg/blank (yaml) v')).toBeInTheDocument();
    expect(editor.value).toBe('x: 1');
    expect(editor.getAttribute('data-language')).toBe('yaml');
    await closeLastModal('cfg/blank (yaml) v');

    // fmt '' + r.format '' → 'json'；version 0（falsy）→ 标题尾同为 'v'
    mGetConfig.mockResolvedValueOnce({ ...DETAIL, format: '', content: '', version: 0 });
    editor = await openEditAndWait('cfg/blank');
    expect(screen.getByText('cfg/blank (json) v')).toBeInTheDocument();
    expect(editor.value).toBe('');
    expect(editor.getAttribute('data-language')).toBe('json');
    await closeLastModal('cfg/blank (json) v');

    // getConfig 失败 → toast，弹窗不开
    mGetConfig.mockRejectedValueOnce(new Error('get down'));
    const row = screen.getByText('cfg/blank').closest('tr') as HTMLElement;
    fireEvent.click(within(row).getByRole('button', { name: /编\s*辑/ }));
    expect(await screen.findByText('获取配置失败')).toBeInTheDocument();
    expect(screen.queryByTestId('code-editor')).not.toBeInTheDocument();

    // r nullish：五个 ?. 全走右翼 → format 'json'、content ''
    mGetConfig.mockResolvedValueOnce(undefined as never);
    editor = await openEditAndWait('cfg/blank');
    expect(screen.getByText('cfg/blank (json) v')).toBeInTheDocument();
    expect(editor.value).toBe('');
  });

  it('编辑器受控 + 弹窗格式切换：language 切 json、csv 预览卸载', async () => {
    renderPage();
    await waitRows();
    const editor = await openEditAndWait('cfg/shop');

    fireEvent.change(editor, { target: { value: 'a1,b1' } });
    expect(editor.value).toBe('a1,b1');

    const modal = (screen.getByText('cfg/shop (csv) v3') as HTMLElement).closest(
      '.ant-modal',
    ) as HTMLElement;
    await pickOption(modal.querySelector('.ant-select') as HTMLElement, 'json');
    expect(editor.getAttribute('data-language')).toBe('json');
    expect(within(modal).queryByText('h1')).not.toBeInTheDocument();
    expect(editor.value).toBe('a1,b1');
  });

  it('langOf 矩阵：yml/py/xml/ini/lua/toml 行经真实页面渲染', async () => {
    renderPage();
    await waitRows();

    // 行 fmt 非空优先于 detail.format；标题 version 取 detail.version=3
    const expectLang = async (id: string, fmt: string, lang: string) => {
      const editor = await openEditAndWait(id);
      expect(editor.getAttribute('data-language')).toBe(lang);
      await closeLastModal(`${id} (${fmt}) v3`);
    };
    await expectLang('cfg/yml', 'yml', 'yaml');
    await expectLang('cfg/py', 'py', 'python');
    await expectLang('cfg/xml', 'xml', 'xml');
    await expectLang('cfg/ini', 'ini', 'ini');
    await expectLang('cfg/edge', 'lua', 'lua');
    await expectLang('cfg/toml', 'toml', 'plaintext');
  });

  it('校验三态：通过 / errors join 透出 / 无 errors 兜底', async () => {
    renderPage();
    await waitRows();
    await openEditAndWait('cfg/shop');

    fireEvent.click(screen.getByRole('button', { name: /校\s*验/ }));
    await waitFor(() =>
      expect(mValidate).toHaveBeenCalledWith('cfg/shop', {
        format: 'csv',
        content: 'h1,h2\r\n\r\n1,2',
      }),
    );
    expect(await screen.findByText('校验通过')).toBeInTheDocument();

    mValidate.mockResolvedValueOnce({ valid: false, errors: ['bad json', 'missing key'] });
    fireEvent.click(screen.getByRole('button', { name: /校\s*验/ }));
    // toast 文本以 \n 连接，getByText 归一折叠空白后按单空格形态匹配
    expect(await screen.findByText('bad json missing key')).toBeInTheDocument();

    mValidate.mockResolvedValueOnce({ valid: false, errors: [] });
    fireEvent.click(screen.getByRole('button', { name: /校\s*验/ }));
    expect(await screen.findByText('校验失败')).toBeInTheDocument();
  });
});

describe('配置管理 保存版本', () => {
  it('成功链：载荷 + toast + 弹窗关 + reload + saveMsg 清空', async () => {
    renderPage();
    await waitRows();
    await openEditAndWait('cfg/shop');

    fireEvent.click(screen.getByRole('button', { name: /保存新版本/ }));
    expect(await screen.findByText('保存版本')).toBeInTheDocument();
    fireEvent.change(screen.getByPlaceholderText('本次修改原因（必填）'), {
      target: { value: 'fix shop' },
    });
    fireEvent.click(document.querySelector('.ant-modal-footer .ant-btn-primary') as HTMLElement);

    await waitFor(() =>
      expect(mSave).toHaveBeenCalledWith('cfg/shop', {
        gameId: 'demo',
        env: 'prod',
        format: 'csv',
        content: 'h1,h2\r\n\r\n1,2',
        message: 'fix shop',
        baseVersion: 3,
      }),
    );
    expect(await screen.findByText('已保存版本 4')).toBeInTheDocument();
    await waitFor(() => expect(screen.queryByText('保存版本')).not.toBeInTheDocument());
    await waitFor(() => expect(mList.mock.calls.length).toBeGreaterThan(1));

    // saveMsg 已清空：重开保存弹窗输入为空
    fireEvent.click(screen.getByRole('button', { name: /保存新版本/ }));
    expect(await screen.findByText('保存版本')).toBeInTheDocument();
    expect((screen.getByPlaceholderText('本次修改原因（必填）') as HTMLInputElement).value).toBe(
      '',
    );
    // onCancel 关闭翼（X 触发 setSaveOpen(false)，destroyOnHidden 卸载）
    await closeLastModal('保存版本');
  });

  it('回退翼与失败翼：gameId/env 回退 toolbar、baseVersion 0、message 无校验、失败保持', async () => {
    renderPage();
    await waitRows();

    // toolbar 选 game（cur.gameId '' 时回退）
    await pickOption(toolbarSelect(0), 'demo');
    await waitFor(() => expect(mList).toHaveBeenLastCalledWith({ gameId: 'demo' }));

    mGetConfig.mockResolvedValueOnce({
      ...DETAIL,
      format: 'lua',
      content: '-- z',
      version: 0,
      gameId: '',
      env: '',
    });
    await openEditAndWait('cfg/edge');
    fireEvent.click(screen.getByRole('button', { name: /保存新版本/ }));
    expect(await screen.findByText('保存版本')).toBeInTheDocument();
    // placeholder 标「必填」但 doSave 无校验——message 空串现状行为
    fireEvent.click(document.querySelector('.ant-modal-footer .ant-btn-primary') as HTMLElement);

    await waitFor(() =>
      expect(mSave).toHaveBeenCalledWith('cfg/edge', {
        gameId: 'demo',
        env: '',
        format: 'lua',
        content: '-- z',
        message: '',
        baseVersion: 0,
      }),
    );
    expect(await screen.findByText('已保存版本 4')).toBeInTheDocument();

    // 失败翼：toast + 弹窗保持
    mSave.mockRejectedValueOnce(new Error('save down'));
    fireEvent.click(screen.getByRole('button', { name: /保存新版本/ }));
    expect(await screen.findByText('保存版本')).toBeInTheDocument();
    fireEvent.click(document.querySelector('.ant-modal-footer .ant-btn-primary') as HTMLElement);
    expect(await screen.findByText('保存失败')).toBeInTheDocument();
    expect(screen.getByText('保存版本')).toBeInTheDocument();

    // r?.version undefined 翼：toast 透出 '已保存版本 undefined'（现状行为）
    mSave.mockResolvedValueOnce({} as never);
    fireEvent.click(document.querySelector('.ant-modal-footer .ant-btn-primary') as HTMLElement);
    expect(await screen.findByText('已保存版本 undefined')).toBeInTheDocument();
  });
});

describe('配置管理 历史版本与对比', () => {
  it('历史版本列表：行渲染（createdAt 双翼）、响应缺省右翼', async () => {
    renderPage();
    await waitRows();
    await openEditAndWait('cfg/shop');

    fireEvent.click(screen.getByRole('button', { name: /历史版本/ }));
    await waitFor(() => expect(mVersions).toHaveBeenCalledWith('cfg/shop'));
    expect(await screen.findByText('m2')).toBeInTheDocument();

    const row2 = versionRow(2);
    expect(within(row2).getByText('ops')).toBeInTheDocument();
    expect(within(row2).getByText(fmtDate('2026-09-02T00:00:00Z'))).toBeInTheDocument();
    const row3 = versionRow(3);
    expect(within(row3).getByText('m3')).toBeInTheDocument();

    // 响应缺省右翼：{} → 空表
    mVersions.mockResolvedValueOnce({} as never);
    await closeLastModal('历史版本');
    fireEvent.click(screen.getByRole('button', { name: /历史版本/ }));
    await waitFor(() => expect(mVersions).toHaveBeenCalledTimes(2));
    // 标题多处命中（编辑弹窗同名按钮），锚 .ant-modal-title
    await waitFor(() =>
      expect(
        Array.from(document.querySelectorAll('.ant-modal-title')).some(
          (t) => t.textContent === '历史版本',
        ),
      ).toBe(true),
    );
    expect(screen.queryByRole('button', { name: /查\s*看/ })).not.toBeInTheDocument();

    // createdAt 空串翼：单元格 ''
    mVersions.mockResolvedValueOnce({
      key: 'cfg/shop',
      total: 1,
      versions: [mkVer(2, { createdAt: '' })],
    });
    await closeLastModal('历史版本');
    fireEvent.click(screen.getByRole('button', { name: /历史版本/ }));
    expect(await screen.findByText('m2')).toBeInTheDocument();
    expect(
      within(versionRow(2)).queryByText(fmtDate('2026-09-02T00:00:00Z')),
    ).not.toBeInTheDocument();
  });

  it('查看版本：content/format 回填、version 不动（baseVersion 语义）、verOpen 关闭、空串右翼', async () => {
    renderPage();
    await waitRows();
    await openEditAndWait('cfg/shop');

    fireEvent.click(screen.getByRole('button', { name: /历史版本/ }));
    expect(await screen.findByText('m2')).toBeInTheDocument();

    mGetVersion.mockResolvedValueOnce(mkVer(2, { value: 'old-v2', format: 'json' }));
    fireEvent.click(within(versionRow(2)).getByRole('button', { name: /查\s*看/ }));
    await waitFor(() => expect(mGetVersion).toHaveBeenCalledWith('cfg/shop', 2));
    const editor = screen.getByTestId('code-editor') as HTMLTextAreaElement;
    await waitFor(() => expect(editor.value).toBe('old-v2'));
    expect(editor.getAttribute('data-language')).toBe('json');
    // version 保持 3（保存的 baseVersion 语义，不随预览漂移）
    expect(screen.getByText('cfg/shop (json) v3')).toBeInTheDocument();
    await waitFor(() =>
      expect(screen.queryByRole('button', { name: /查\s*看/ })).not.toBeInTheDocument(),
    );

    // value/format 空串右翼：content ''、format 保持
    fireEvent.click(screen.getByRole('button', { name: /历史版本/ }));
    expect(await screen.findByText('m3')).toBeInTheDocument();
    mGetVersion.mockResolvedValueOnce(mkVer(3, { value: '', format: '' }));
    fireEvent.click(within(versionRow(3)).getByRole('button', { name: /查\s*看/ }));
    await waitFor(() => expect(editor.value).toBe(''));
    expect(screen.getByText('cfg/shop (json) v3')).toBeInTheDocument();
  });

  it('版本对比 add 族：桩 left/right + DiffView 加行绿底/对侧空占位/尾差/left 空双右翼', async () => {
    renderPage();
    await waitRows();
    const editor = await openEditAndWait('cfg/shop');

    // 先置空内容：覆盖 cur.content||'' 与 DiffView (left||'') 双右翼
    fireEvent.change(editor, { target: { value: '' } });
    mGetVersion.mockImplementation(async (_id: string, ver: number) =>
      mkVer(ver, { value: ver === 2 ? '' : 'a\nx\ny\nb\nc' }),
    );
    fireEvent.click(screen.getByRole('button', { name: /历史版本/ }));
    expect(await screen.findByText('m2')).toBeInTheDocument();
    fireEvent.click(within(versionRow(2)).getByRole('button', { name: 'Diff' }));
    expect(await screen.findByText('版本对比')).toBeInTheDocument();
    let stub = screen.getByTestId('monaco-diff');
    expect(stub.getAttribute('data-left')).toBe('');
    expect(stub.getAttribute('data-right')).toBe('');
    await closeLastModal('版本对比');

    // add-batch：a b c → a x y b c（x/y 绿底、左面板空 div 占位在对应位次）
    fireEvent.change(editor, { target: { value: 'a\nb\nc' } });
    fireEvent.click(within(versionRow(3)).getByRole('button', { name: 'Diff' }));
    expect(await screen.findByText('版本对比')).toBeInTheDocument();
    stub = screen.getByTestId('monaco-diff');
    expect(stub.getAttribute('data-left')).toBe('a\nb\nc');
    expect(stub.getAttribute('data-right')).toBe('a\nx\ny\nb\nc');
    let panes = diffPanes();
    expect(panes.leftLines()).toEqual(['a', '', '', 'b', 'c']);
    expect(panes.rightLines()).toEqual(['a', 'x', 'y', 'b', 'c']);
    expect(panes.bgOf(panes.right, 'x')).toContain('rgb(246, 255, 237)');
    expect(panes.bgOf(panes.right, 'y')).toContain('rgb(246, 255, 237)');
    await closeLastModal('版本对比');

    // add-tail：尾部新增 z
    mGetVersion.mockResolvedValueOnce(mkVer(3, { value: 'a\nb\nc\nz' }));
    fireEvent.click(within(versionRow(3)).getByRole('button', { name: 'Diff' }));
    expect(await screen.findByText('版本对比')).toBeInTheDocument();
    panes = diffPanes();
    expect(panes.leftLines()).toEqual(['a', 'b', 'c', '']);
    expect(panes.rightLines()).toEqual(['a', 'b', 'c', 'z']);
    expect(panes.bgOf(panes.right, 'z')).toContain('rgb(246, 255, 237)');
  });

  it('版本对比 del 族：删行红底、批量删 + 尾删 + 单点替换（del/add 同步）', async () => {
    renderPage();
    await waitRows();
    const editor = await openEditAndWait('cfg/shop');
    fireEvent.change(editor, { target: { value: 'a\np\nq\nb\nz' } });

    mGetVersion.mockImplementation(async (_id: string, ver: number) =>
      mkVer(ver, { value: ver === 2 ? 'a\nb' : 'a\nx\nc\nb\nz' }),
    );
    fireEvent.click(screen.getByRole('button', { name: /历史版本/ }));
    expect(await screen.findByText('m2')).toBeInTheDocument();

    // del-batch（p,q 红底）+ del-tail（z 红底）：右侧空 div 占位
    fireEvent.click(within(versionRow(2)).getByRole('button', { name: 'Diff' }));
    expect(await screen.findByText('版本对比')).toBeInTheDocument();
    let panes = diffPanes();
    expect(panes.leftLines()).toEqual(['a', 'p', 'q', 'b', 'z']);
    expect(panes.rightLines()).toEqual(['a', '', '', 'b', '']);
    expect(panes.bgOf(panes.left, 'p')).toContain('rgb(255, 241, 240)');
    expect(panes.bgOf(panes.left, 'q')).toContain('rgb(255, 241, 240)');
    expect(panes.bgOf(panes.left, 'z')).toContain('rgb(255, 241, 240)');
    await closeLastModal('版本对比');

    // 单点替换：p→x、q→c（红/绿各两行，双侧空占位交错）
    fireEvent.click(within(versionRow(3)).getByRole('button', { name: 'Diff' }));
    expect(await screen.findByText('版本对比')).toBeInTheDocument();
    panes = diffPanes();
    expect(panes.leftLines()).toEqual(['a', 'p', '', 'q', '', 'b', 'z']);
    expect(panes.rightLines()).toEqual(['a', '', 'x', '', 'c', 'b', 'z']);
    expect(panes.bgOf(panes.left, 'p')).toContain('rgb(255, 241, 240)');
    expect(panes.bgOf(panes.right, 'x')).toContain('rgb(246, 255, 237)');
    expect(panes.bgOf(panes.right, 'c')).toContain('rgb(246, 255, 237)');
  });

  it('回滚：confirm 文案 + onOk 载荷 + 成功链；getVersion undefined 双右翼；失败翼弹窗保持', async () => {
    renderPage();
    await waitRows();
    await openEditAndWait('cfg/shop');

    fireEvent.click(screen.getByRole('button', { name: /历史版本/ }));
    expect(await screen.findByText('m2')).toBeInTheDocument();

    mGetVersion.mockResolvedValueOnce(mkVer(2, { value: 'rb-content', format: 'json' }));
    fireEvent.click(within(versionRow(2)).getByRole('button', { name: /回\s*滚/ }));
    const confirm1 = await waitFor(() => {
      const el = Array.from(document.querySelectorAll('.ant-modal-confirm')).find((m) =>
        m.textContent?.includes('确认回滚到版本 2'),
      );
      expect(el).toBeTruthy();
      return el as HTMLElement;
    });
    expect(
      within(confirm1).getByText('确认回滚到版本 2 吗？此操作将创建一个新版本。'),
    ).toBeInTheDocument();
    fireEvent.click(
      confirm1.querySelector('.ant-modal-confirm-btns .ant-btn-primary') as HTMLElement,
    );

    await waitFor(() =>
      expect(mSave).toHaveBeenCalledWith('cfg/shop', {
        gameId: 'demo',
        env: 'prod',
        format: 'json',
        content: 'rb-content',
        message: 'rollback to v2',
        baseVersion: 3,
      }),
    );
    expect(await screen.findByText('已回滚')).toBeInTheDocument();
    await waitFor(() =>
      expect(screen.queryByRole('button', { name: /查\s*看/ })).not.toBeInTheDocument(),
    );
    await waitFor(() => expect(mList.mock.calls.length).toBeGreaterThan(1));

    // getVersion undefined：format 回退 cur.format、content ''
    fireEvent.click(screen.getByRole('button', { name: /历史版本/ }));
    expect(await screen.findByText('m3')).toBeInTheDocument();
    mGetVersion.mockResolvedValueOnce(undefined as never);
    fireEvent.click(within(versionRow(3)).getByRole('button', { name: /回\s*滚/ }));
    const confirm2 = await waitFor(() => {
      const el = Array.from(document.querySelectorAll('.ant-modal-confirm')).find((m) =>
        m.textContent?.includes('确认回滚到版本 3'),
      );
      expect(el).toBeTruthy();
      return el as HTMLElement;
    });
    fireEvent.click(
      confirm2.querySelector('.ant-modal-confirm-btns .ant-btn-primary') as HTMLElement,
    );
    await waitFor(() =>
      expect(mSave).toHaveBeenLastCalledWith('cfg/shop', {
        gameId: 'demo',
        env: 'prod',
        format: 'csv',
        content: '',
        message: 'rollback to v3',
        baseVersion: 3,
      }),
    );

    // 失败翼：toast + 版本弹窗保持（上一次成功已关版本弹窗，须重开）
    mSave.mockRejectedValueOnce(new Error('rb down'));
    mGetVersion.mockResolvedValueOnce(mkVer(2, { value: 'x' }));
    fireEvent.click(screen.getByRole('button', { name: /历史版本/ }));
    expect(await screen.findByText('m3')).toBeInTheDocument();
    fireEvent.click(within(versionRow(2)).getByRole('button', { name: /回\s*滚/ }));
    const confirm3 = await waitFor(() => {
      const el = Array.from(document.querySelectorAll('.ant-modal-confirm')).find((m) =>
        m.textContent?.includes('确认回滚到版本 2'),
      );
      expect(el).toBeTruthy();
      return el as HTMLElement;
    });
    fireEvent.click(
      confirm3.querySelector('.ant-modal-confirm-btns .ant-btn-primary') as HTMLElement,
    );
    expect(await screen.findByText('回滚失败')).toBeInTheDocument();
    expect(screen.getAllByRole('button', { name: /查\s*看/ }).length).toBeGreaterThan(0);
  });

  it('回滚载荷回退翼：cur.gameId/env/version 空 → toolbar 态与 0（回滚侧三处 || 右翼）', async () => {
    renderPage();
    await waitRows();

    mGetConfig.mockResolvedValueOnce({
      ...DETAIL,
      format: 'lua',
      content: '-- z',
      version: 0,
      gameId: '',
      env: '',
    });
    await openEditAndWait('cfg/edge');
    fireEvent.click(screen.getByRole('button', { name: /历史版本/ }));
    expect(await screen.findByText('m2')).toBeInTheDocument();

    mGetVersion.mockResolvedValueOnce(mkVer(2, { value: 'x', format: 'json' }));
    fireEvent.click(within(versionRow(2)).getByRole('button', { name: /回\s*滚/ }));
    const confirm = await waitFor(() => {
      const el = Array.from(document.querySelectorAll('.ant-modal-confirm')).find((m) =>
        m.textContent?.includes('确认回滚到版本 2'),
      );
      expect(el).toBeTruthy();
      return el as HTMLElement;
    });
    fireEvent.click(
      confirm.querySelector('.ant-modal-confirm-btns .ant-btn-primary') as HTMLElement,
    );

    // toolbar 未选 game（''）：gameId/env 回退双 ''，baseVersion 回退 0
    await waitFor(() =>
      expect(mSave).toHaveBeenLastCalledWith('cfg/edge', {
        gameId: '',
        env: '',
        format: 'json',
        content: 'x',
        message: 'rollback to v2',
        baseVersion: 0,
      }),
    );
  });
});
