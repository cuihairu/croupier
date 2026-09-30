/**
 * 工具箱页单测（覆盖率巡检：Dev/Tools/index.tsx 539 行 0% → 收口，
 * 零测试页排行第三）。
 *
 * 锁定契约：
 * - 初始加载（getScope 透传 gameId/env 进 listTools；scope 空 → undefined
 *   双臂）与分组矩阵：六类分组头（icon + label + 计数 Tag）、未知分类
 *   不在 toolCategoryOrder → 静默过滤不渲染、卡片标题外链（href + 新窗口）、
 *   description 有无两臂、作用域 Tag（scoped 蓝 `gameId/env` / 全局）、
 *   Switch 启停（canManage）、卡片三操作（外链开窗 / 编辑 / 删除）；
 * - 空态（items 缺省 `|| []` 右翼同形态）：Empty 文案 + 工具按钮仍在；
 * - load 失败两翼（Error.message 透传 / 非 Error「加载工具列表失败」）；
 * - 登记主链：标题、name/url 双 required 拦截 + url pattern 翼
 *   （`必须以 http:// 或 https:// 开头`）、默认值载荷（category ci +
 *   scopeMode global）、「工具已登记」+ 重拉 + 关闭；scopeMode 联动——
 *   切「当前游戏环境（demo/prod)」→ scopeMode 落库 + setFieldsValue 回填
 *   gameId/env + ScopedEnvFields 渲染双输入、切回全局清空，最终以 scoped
 *   提交（回归锁定：自定义包装组件必须透传 Form.Item 注入的 value/
 *   onChange——不透传则内层 Select 脱管，scopeMode 永远进不了 store，
 *   表单始终按 global 提交，scoped 工具无法从 UI 创建，本轮修定的真缺陷）；
 * - scope 空：listTools 双 undefined 载荷 + scoped 选项 label「-/-」回退；
 * - 编辑主链：标题 `编辑工具：{name}`、回填（scoped 工具 scopeMode
 *   scoped + gameId/env 输入 + enabled Switch）、updateTool(id, v) 载荷、
 *   「工具已更新」+ 重拉；编辑全局工具（gameId 缺省）scopeMode 回退
 *   'global'、载荷无 gameId/env 键；保存失败两翼（Error.message / 非
 *   Error「保存失败」）弹窗保持；
 * - 启停：Switch → updateTool(id, {enabled}) → 重拉；失败「操作失败」；
 * - 删除：Popconfirm `删除工具「{name}」？` → deleteTool(id) → 「已删除」
 *   + 重拉；失败「删除失败」；
 * - canManage false：无登记按钮、卡片无操作无 Switch。
 *
 * mock 口径：services/api/tools 四函数 + 两张表 jest.mock（真实模块顶层
 * getIntl）；stores/scope 的 getScope 可控；@umijs/max 本地 mock
 * （defaultMessage 即文案 + {name} 内插 + useAccess 可控）；antd/
 * pro-components/extractErrorMessage 走真实实现；window.open spy。
 *
 * 登记不可达（防御分支，不造假用例不删分支）：无——全部可达路径均有用例。
 *
 * 坑实证（antd6 沿用）：卡片 actions 里的图标操作是 span[role=img]
 * （aria-label export/setting），按 aria-label 锚点点击；ModalForm 提交锚
 * .ant-modal-footer .ant-btn-primary（双字中文插空格「保 存」）；作用域
 * Select 是弹窗内第 2 个 .ant-select（第 1 个是分类），mouseDown 根 + 点
 * 可见 option content；作用域 option 文案括号全/半角混排
 * （`当前游戏环境（demo/prod)`），matcher 勿带闭合括号；卡面 description
 * 区恒含作用域 Tag，无描述时 textContent 即 Tag 文案非空串；Form.Item 的
 * 子若是自定义组件，注入的 value/onChange 必须显式透传（rest 展开 + 先调
 * rest.onChange 再做联动）——脱管时 rc-select 内部态照常变化（选中显示
 * 正常），但 store 永不更新，仅提交载荷暴露，肉眼 UI 联动看起来只坏一半。
 */
import React from 'react';
import { configure, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { App } from 'antd';
import DevToolsPage from '../index';
import type { ToolItem } from '@/services/api/tools';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

jest.mock('@/services/api/tools', () => ({
  listTools: jest.fn(),
  createTool: jest.fn(),
  updateTool: jest.fn(),
  deleteTool: jest.fn(),
  toolCategoryLabels: {
    ci: 'CI/CD',
    repo: '代码仓库',
    monitor: '监控',
    docs: '文档',
    artifact: '制品库',
    other: '其他',
  },
  toolCategoryOrder: ['ci', 'repo', 'monitor', 'docs', 'artifact', 'other'],
}));

// scope 可控：default demo/prod；各用例改 mockScope 后 render
const mockScope: { gameId?: string; env?: string } = { gameId: 'demo', env: 'prod' };
jest.mock('@/stores/scope', () => ({
  getScope: () => mockScope,
}));

// mock* 前缀变量：babel-jest hoist 白名单；useAccess 每用例可控 canDevManage
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

import { listTools, createTool, updateTool, deleteTool } from '@/services/api/tools';

const mList = listTools as jest.MockedFunction<typeof listTools>;
const mCreate = createTool as jest.MockedFunction<typeof createTool>;
const mUpdate = updateTool as jest.MockedFunction<typeof updateTool>;
const mDelete = deleteTool as jest.MockedFunction<typeof deleteTool>;

const mk = (over: Partial<ToolItem> & Pick<ToolItem, 'id' | 'name' | 'category'>): ToolItem => ({
  url: 'https://t.example.com',
  enabled: true,
  sort: 0,
  updatedAt: '2026-09-01T00:00:00Z',
  ...over,
});

// 覆盖翼：全局（gameId 缺省 → 全局 Tag）+ description 缺省臂
const t1 = mk({ id: 1, name: 'Jenkins', category: 'ci' });
// 覆盖翼：scoped demo/prod 蓝 Tag + description
const t2 = mk({
  id: 2,
  name: 'GitLab',
  category: 'repo',
  description: '代码托管',
  gameId: 'demo',
  env: 'prod',
});
// 覆盖翼：停用 Switch + 他游戏 scoped
const t3 = mk({
  id: 3,
  name: 'Grafana',
  category: 'monitor',
  enabled: false,
  gameId: 'solo',
  env: 'dev',
});
const t4 = mk({ id: 4, name: 'Confluence', category: 'docs', gameId: 'demo', env: 'prod' });
const t5 = mk({ id: 5, name: 'Nexus', category: 'artifact', gameId: 'demo', env: 'prod' });
const t6 = mk({ id: 6, name: 'Misc', category: 'other', gameId: 'demo', env: 'prod' });
// 覆盖翼：未知分类不在 toolCategoryOrder → 静默过滤
const t7 = mk({ id: 7, name: 'Ghost', category: 'weird' as never });

const tools = [t1, t2, t3, t4, t5, t6, t7];

beforeEach(() => {
  mockCanDevManage = true;
  mockScope.gameId = 'demo';
  mockScope.env = 'prod';
  jest.clearAllMocks();
  mList.mockResolvedValue({ items: tools });
  mCreate.mockResolvedValue(t1 as never);
  mUpdate.mockResolvedValue(t1 as never);
  mDelete.mockResolvedValue(undefined);
  jest.spyOn(window, 'open').mockImplementation(() => null);
});

afterEach(() => {
  jest.restoreAllMocks();
});

function renderPage() {
  return render(
    <App>
      <DevToolsPage />
    </App>,
  );
}

/** 等首拉落定（锚首个工具名） */
async function waitLoad() {
  expect(await screen.findByText('Jenkins')).toBeInTheDocument();
  await waitFor(() => expect(mList).toHaveBeenCalledTimes(1));
}

/** 工具小卡（锚工具名，最近 .ant-card 祖先） */
function cardOf(name: string) {
  return screen.getByText(name).closest('.ant-card') as HTMLElement;
}

/** 弹窗 footer 保存按钮 */
function submitForm() {
  const ok = document.querySelector('.ant-modal-footer .ant-btn-primary') as HTMLElement;
  expect(ok).not.toBeNull();
  fireEvent.click(ok);
}

/** 弹窗内第 idx 个 Select 选 option（antd6：mouseDown 根 + 点可见 option content） */
async function pickModalSelect(idx: number, matcher: (t: string) => boolean) {
  await new Promise((r) => setTimeout(r, 60));
  const selects = document.querySelectorAll('.ant-modal .ant-select');
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

/** 分组头 Space（label 文本锚定：icon + strong + 计数 Tag 同 Space） */
function groupHead(label: string) {
  return screen.getByText(label).closest('.ant-space') as HTMLElement;
}

describe('工具箱 初始渲染', () => {
  it('六类分组矩阵 + 未知分类过滤 + 卡片字段两臂 + 首拉 scope 透传', async () => {
    renderPage();
    await waitLoad();

    expect(mList).toHaveBeenCalledWith({ gameId: 'demo', env: 'prod' });

    // 六类分组头 + 计数 Tag
    for (const label of ['CI/CD', '代码仓库', '监控', '文档', '制品库', '其他']) {
      expect(screen.getByText(label)).toBeInTheDocument();
    }
    expect(within(groupHead('CI/CD')).getByText('1')).toBeInTheDocument();
    // 未知分类静默过滤
    expect(screen.queryByText('Ghost')).not.toBeInTheDocument();

    // 标题外链（href + 新窗口）
    expect(screen.getByText('Jenkins').closest('a')).toHaveAttribute(
      'href',
      'https://t.example.com',
    );

    // description 有无两臂：description 区恒含作用域 Tag，无描述时仅 Tag 无文案
    expect(screen.getByText('代码托管')).toBeInTheDocument();
    expect(cardOf('Jenkins').querySelector('.ant-card-meta-description')?.textContent).toBe('全局');

    // 作用域 Tag 两臂
    expect(within(cardOf('GitLab')).getByText('demo/prod').closest('.ant-tag')).toHaveClass(
      'ant-tag-blue',
    );
    expect(within(cardOf('Jenkins')).getByText('全局').closest('.ant-tag')).not.toHaveClass(
      'ant-tag-blue',
    );

    // Switch 启停态（Grafana 停用）
    expect(within(cardOf('Grafana')).getByRole('switch')).not.toBeChecked();
    expect(within(cardOf('Jenkins')).getByRole('switch')).toBeChecked();

    // 卡片三操作
    expect(within(cardOf('Jenkins')).getByText('删除')).toBeInTheDocument();
    expect(within(cardOf('Jenkins')).getByLabelText('setting')).toBeInTheDocument();

    // 工具栏
    expect(screen.getByRole('button', { name: /刷新/ })).toBeEnabled();
    expect(screen.getByRole('button', { name: /登记工具/ })).toBeEnabled();
  });

  it('scope 空：listTools 双 undefined 载荷 + scoped 选项 label「-/-」回退', async () => {
    mockScope.gameId = undefined;
    mockScope.env = undefined;
    renderPage();
    await waitFor(() => expect(mList).toHaveBeenCalledWith({ gameId: undefined, env: undefined }));
    expect(await screen.findByText('Jenkins')).toBeInTheDocument();

    // 开弹窗：scopeMode 选项 label 的 gameId/env 双 '-' 回退臂（83-85 行）
    fireEvent.click(screen.getByRole('button', { name: /登记工具/ }));
    await waitFor(() =>
      expect(document.querySelector('.ant-modal-title')?.textContent).toBe('登记内部工具'),
    );
    await new Promise((r) => setTimeout(r, 60));
    const selects = document.querySelectorAll('.ant-modal .ant-select');
    fireEvent.mouseDown(selects[1] as HTMLElement);
    const dropdown = await waitFor(() => {
      const visible = Array.from(document.querySelectorAll('.ant-select-dropdown')).find(
        (d) => !d.className.includes('ant-select-dropdown-hidden'),
      ) as HTMLElement;
      expect(visible).not.toBeUndefined();
      return visible;
    });
    expect(
      Array.from(dropdown.querySelectorAll('.ant-select-item-option-content')).some(
        (o) => o.textContent === '当前游戏环境（-/-)',
      ),
    ).toBe(true);
  });

  it('空态：items 缺省 || 右翼 → Empty 文案 + 工具按钮仍在', async () => {
    mList.mockResolvedValue({} as never);
    renderPage();
    expect(
      await screen.findByText(
        '暂无工具。让管理员登记 Jenkins / GitLab / Grafana 等内部工具链接，即可在此集中访问。',
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /登记工具/ })).toBeInTheDocument();
  });

  it('load 失败两翼：Error.message 透传 / 非 Error 兜底「加载工具列表失败」', async () => {
    mList.mockRejectedValueOnce(new Error('tools-down'));
    renderPage();
    expect(await screen.findByText('tools-down')).toBeInTheDocument();

    mList.mockRejectedValueOnce('plain' as never);
    const second = renderPage();
    expect(await screen.findByText('加载工具列表失败')).toBeInTheDocument();
    second.unmount();
  });

  it('外链开窗：卡片 export 操作 → window.open(url, _blank, noreferrer)', async () => {
    renderPage();
    await waitLoad();

    // 标题链接里也有 export 图标：锚 .ant-card-actions 内的操作图标
    const exportAction = cardOf('GitLab').querySelector(
      '.ant-card-actions span[aria-label="export"]',
    ) as HTMLElement;
    fireEvent.click(exportAction);
    expect(window.open).toHaveBeenCalledWith('https://t.example.com', '_blank', 'noreferrer');
  });

  it('刷新重拉', async () => {
    renderPage();
    await waitLoad();
    fireEvent.click(screen.getByRole('button', { name: /刷新/ }));
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(2));
  });
});

describe('工具箱 登记与编辑', () => {
  it('required/pattern 三拦截 → 默认值载荷 → 工具已登记 + 重拉 + 关闭', async () => {
    renderPage();
    await waitLoad();

    fireEvent.click(screen.getByRole('button', { name: /登记工具/ }));
    await waitFor(() =>
      expect(document.querySelector('.ant-modal-title')?.textContent).toBe('登记内部工具'),
    );

    // 空提交：name + url 双 required
    submitForm();
    await waitFor(() =>
      expect(document.querySelector('.ant-form-item-explain-error')).not.toBeNull(),
    );
    expect(mCreate).not.toHaveBeenCalled();

    // url pattern 翼
    fireEvent.change(screen.getByPlaceholderText('https://ci.example.com'), {
      target: { value: 'ftp://x' },
    });
    submitForm();
    expect(await screen.findByText('必须以 http:// 或 https:// 开头')).toBeInTheDocument();
    expect(mCreate).not.toHaveBeenCalled();

    // 填值提交：默认 category ci + scopeMode global（gameId/env 不出现）
    fireEvent.change(screen.getByPlaceholderText('如 Jenkins / GitLab / Grafana'), {
      target: { value: 'New Tool' },
    });
    fireEvent.change(screen.getByPlaceholderText('https://ci.example.com'), {
      target: { value: 'https://new.example.com' },
    });
    submitForm();
    await waitFor(() => expect(mCreate).toHaveBeenCalledTimes(1));
    expect(mCreate).toHaveBeenCalledWith({
      name: 'New Tool',
      url: 'https://new.example.com',
      category: 'ci',
      scopeMode: 'global',
    });
    expect(await screen.findByText('工具已登记')).toBeInTheDocument();
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(2));
    await waitFor(() =>
      expect(
        screen.queryByPlaceholderText('如 Jenkins / GitLab / Grafana'),
      ).not.toBeInTheDocument(),
    );
  });

  it('scopeMode 联动：切当前环境回填 gameId/env + 双输入渲染，切回全局清空', async () => {
    renderPage();
    await waitLoad();

    fireEvent.click(screen.getByRole('button', { name: /登记工具/ }));
    await waitFor(() =>
      expect(document.querySelector('.ant-modal-title')?.textContent).toBe('登记内部工具'),
    );
    // 初始 global：无 gameId/env 输入
    expect(screen.queryByDisplayValue('demo')).not.toBeInTheDocument();

    // 切当前游戏环境（demo/prod）：联动回填 + ScopedEnvFields 渲染
    // （源文案括号全/半角混排，matcher 取无括号后缀前缀）
    await pickModalSelect(1, (t) => t.includes('当前游戏环境（demo/prod'));
    await waitFor(() => expect(screen.getByDisplayValue('demo')).toBeInTheDocument());
    expect(screen.getByDisplayValue('prod')).toBeInTheDocument();

    // 切回全局：清空 + 双输入卸载
    await pickModalSelect(1, (t) => t.includes('全局（所有游戏可见）'));
    await waitFor(() => expect(screen.queryByDisplayValue('demo')).not.toBeInTheDocument());

    // 再切 scoped（最终以 scoped 提交）
    await pickModalSelect(1, (t) => t.includes('当前游戏环境（demo/prod'));
    await waitFor(() => expect(screen.getByDisplayValue('demo')).toBeInTheDocument());

    fireEvent.change(screen.getByPlaceholderText('如 Jenkins / GitLab / Grafana'), {
      target: { value: 'Scoped Tool' },
    });
    fireEvent.change(screen.getByPlaceholderText('https://ci.example.com'), {
      target: { value: 'https://s.example.com' },
    });
    submitForm();
    await waitFor(() => expect(mCreate).toHaveBeenCalledTimes(1));
    expect(mCreate).toHaveBeenCalledWith(
      expect.objectContaining({
        scopeMode: 'scoped',
        gameId: 'demo',
        env: 'prod',
        name: 'Scoped Tool',
        url: 'https://s.example.com',
      }),
    );
  });

  it('编辑主链：标题/回填/scoped 输入/enabled 开关 → updateTool 载荷 → 工具已更新', async () => {
    renderPage();
    await waitLoad();

    fireEvent.click(within(cardOf('GitLab')).getByLabelText('setting'));
    await waitFor(() =>
      expect(document.querySelector('.ant-modal-title')?.textContent).toBe('编辑工具：GitLab'),
    );

    // 回填：name/url/description + scoped 双输入 + enabled Switch（卡面
    // Switch 仍在 DOM，switch 查询须限定弹窗）
    expect(screen.getByDisplayValue('GitLab')).toBeInTheDocument();
    expect(screen.getByDisplayValue('代码托管')).toBeInTheDocument();
    expect(screen.getByDisplayValue('demo')).toBeInTheDocument();
    expect(screen.getByDisplayValue('prod')).toBeInTheDocument();
    expect(
      within(document.querySelector('.ant-modal') as HTMLElement).getByRole('switch'),
    ).toBeChecked();

    // 改名提交
    fireEvent.change(screen.getByDisplayValue('GitLab'), { target: { value: 'GitLab2' } });
    submitForm();
    await waitFor(() => expect(mUpdate).toHaveBeenCalledTimes(1));
    expect(mUpdate).toHaveBeenCalledWith(
      2,
      expect.objectContaining({
        name: 'GitLab2',
        url: 'https://t.example.com',
        description: '代码托管',
        category: 'repo',
        scopeMode: 'scoped',
        gameId: 'demo',
        env: 'prod',
        enabled: true,
      }),
    );
    expect(await screen.findByText('工具已更新')).toBeInTheDocument();
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(2));

    // 编辑全局工具（gameId 缺省）：scopeMode 回退 'global' 臂（439 行），
    // 载荷无 gameId/env 键
    fireEvent.click(within(cardOf('Jenkins')).getByLabelText('setting'));
    await waitFor(() =>
      expect(document.querySelector('.ant-modal-title')?.textContent).toBe('编辑工具：Jenkins'),
    );
    expect(screen.queryByDisplayValue('demo')).not.toBeInTheDocument();
    submitForm();
    await waitFor(() => expect(mUpdate).toHaveBeenCalledTimes(2));
    expect(mUpdate).toHaveBeenLastCalledWith(
      1,
      expect.objectContaining({ name: 'Jenkins', scopeMode: 'global' }),
    );
    expect(mUpdate.mock.calls[1][1]).not.toHaveProperty('gameId');
    expect(mUpdate.mock.calls[1][1]).not.toHaveProperty('env');
  });

  it('保存失败两翼：Error.message 透传 / 非 Error 兜底「保存失败」，弹窗保持', async () => {
    renderPage();
    await waitLoad();

    fireEvent.click(screen.getByRole('button', { name: /登记工具/ }));
    await waitFor(() =>
      expect(document.querySelector('.ant-modal-title')?.textContent).toBe('登记内部工具'),
    );
    fireEvent.change(screen.getByPlaceholderText('如 Jenkins / GitLab / Grafana'), {
      target: { value: 'x' },
    });
    fireEvent.change(screen.getByPlaceholderText('https://ci.example.com'), {
      target: { value: 'https://x.example.com' },
    });

    mCreate.mockRejectedValueOnce(new Error('tool-save-x'));
    submitForm();
    expect(await screen.findByText('tool-save-x')).toBeInTheDocument();
    expect(screen.getByPlaceholderText('如 Jenkins / GitLab / Grafana')).toBeInTheDocument();

    mCreate.mockRejectedValueOnce('plain' as never);
    submitForm();
    expect(await screen.findByText('保存失败')).toBeInTheDocument();
    expect(screen.getByPlaceholderText('如 Jenkins / GitLab / Grafana')).toBeInTheDocument();
    expect(mList).toHaveBeenCalledTimes(1);
  });
});

describe('工具箱 启停与删除', () => {
  it('Switch 启停：updateTool(id, {enabled}) → 重拉；失败「操作失败」', async () => {
    renderPage();
    await waitLoad();

    // Grafana 停用 → 开
    fireEvent.click(within(cardOf('Grafana')).getByRole('switch'));
    await waitFor(() => expect(mUpdate).toHaveBeenCalledWith(3, { enabled: true }));
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(2));

    // Jenkins 启用 → 停
    fireEvent.click(within(cardOf('Jenkins')).getByRole('switch'));
    await waitFor(() => expect(mUpdate).toHaveBeenLastCalledWith(1, { enabled: false }));
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(3));

    mUpdate.mockRejectedValueOnce(new Error('toggle-x'));
    fireEvent.click(within(cardOf('Grafana')).getByRole('switch'));
    expect(await screen.findByText('toggle-x')).toBeInTheDocument();
  });

  it('删除主链：Popconfirm 文案 → deleteTool → 已删除 + 重拉；失败「删除失败」', async () => {
    renderPage();
    await waitLoad();

    fireEvent.click(within(cardOf('GitLab')).getByText('删除'));
    expect(await screen.findByText('删除工具「GitLab」？')).toBeInTheDocument();
    const popover = Array.from(document.querySelectorAll('.ant-popover')).find(
      (p) => !p.className.includes('ant-popover-hidden'),
    ) as HTMLElement;
    fireEvent.click(popover.querySelector('.ant-btn-primary') as HTMLElement);

    await waitFor(() => expect(mDelete).toHaveBeenCalledWith(2));
    expect(await screen.findByText('已删除')).toBeInTheDocument();
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(2));

    // 非 Error → 「删除失败」兜底臂（Error.message 透传臂已由 del 载荷锁定）
    mDelete.mockRejectedValueOnce('plain' as never);
    fireEvent.click(within(cardOf('GitLab')).getByText('删除'));
    expect(await screen.findByText('删除工具「GitLab」？')).toBeInTheDocument();
    const popover2 = Array.from(document.querySelectorAll('.ant-popover')).find(
      (p) => !p.className.includes('ant-popover-hidden'),
    ) as HTMLElement;
    fireEvent.click(popover2.querySelector('.ant-btn-primary') as HTMLElement);
    expect(await screen.findByText('删除失败')).toBeInTheDocument();
    expect(mList).toHaveBeenCalledTimes(2);
  });
});

describe('工具箱 canManage false', () => {
  it('无登记按钮、卡片无操作无 Switch、链接仍可访问', async () => {
    mockCanDevManage = false;
    renderPage();
    await waitLoad();

    expect(screen.queryByRole('button', { name: /登记工具/ })).not.toBeInTheDocument();
    expect(within(cardOf('Jenkins')).queryByText('删除')).not.toBeInTheDocument();
    expect(within(cardOf('Jenkins')).queryByLabelText('setting')).not.toBeInTheDocument();
    expect(within(cardOf('Jenkins')).queryByRole('switch')).not.toBeInTheDocument();
    expect(screen.getByText('Jenkins').closest('a')).toHaveAttribute(
      'href',
      'https://t.example.com',
    );
  });
});
