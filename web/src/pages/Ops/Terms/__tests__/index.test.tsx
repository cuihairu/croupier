/**
 * 术语字典页单测（覆盖率巡检：Ops/Terms/index.tsx 231 行 0% → 收口）。
 *
 * 锁定契约：
 * - 挂载：useEffect → load() → listTerms('resource')（默认域）；域切换 →
 *   setDomain 重建 load → effect 重跑 → listTerms('operation')；
 * - 渲染矩阵：Domain Tag resource 蓝 / 其余紫；显示文本走
 *   localizedText(display, intl.locale, termKey)（zh 命中 / zh 空串→en
 *   回退 / display 缺省→termKey 兜底）；语言列 Object.keys(display||{})
 *   .filter(真值).join(' / ')（双真值 / 空串过滤 / display 缺省 '-'）；
 * - 新增：initialValues {domain: 当前筛选域, order: 100}（destroyOnHidden
 *   保证重开按最新 initialValues 重挂载），required 三键拦截（不触达
 *   服务、弹窗保持）；提交 upsertTerm(全字段) →「保存成功」+ load() 重拉
 *   + 弹窗关闭；
 * - 编辑：initialValues=editing 整行回填；提交载荷只含表单注册键——
 *   id 不参与提交（TermFormValues = Omit<TermItem,'id'>）；
 * - 保存失败：onFinish catch → return false（弹窗保持开启、不重拉、
 *   无本地弹错——全局拦截器语义）；
 * - 删除：Popconfirm 确认 → deleteTerm(row.domain, row.alias) →「已删除」
 *   + load() 重拉；
 * - load 失败：message.error(extractErrorMessage(error, getIntl() 解析的
 *   「加载术语失败」))——Error.message 透传。
 *
 * mock 口径：services/api/terms 三函数 jest.mock（service 边界）；
 * @umijs/max 本地 mock——getIntl 必须与 useIntl 同一稳定实例（getErrorMessage
 * 经 getIntl() 在调用时解析，页内注释言明不稳定实例会与 useEffect 成无限
 * 请求循环）；LocalizedTextEditor 桩为受控 value/onChange 组件（按 zh-CN
 * 键读写，组件本体另有测试）；antd/pro-components 真实实现。
 *
 * 登记不可达/边界（防御分支，不造假用例不删分支）：
 * - delete 的 onConfirm 无 catch：deleteTerm reject 会产生 unhandled
 *   rejection（页面原语义如此），不造假 reject 场景；
 * - line 82 `(row.display || {})[k]` 的 `|| {}` 右臂：display undefined 时
 *   上游 Object.keys(row.display || {}) 已得 []，filter 回调不执行——
 *   回调内二次归一右臂结构不可达（分支 96.55% 余量即此，3.45% 为 1/29）；
 * - 其余 ternary/?? 双臂（editing ?? 新增默认、locales 空数组、domain
 *   双色、display || {} 首处）均经 fixture 真实覆盖。
 *
 * 坑实证（antd6 沿用）：rc-select 选项点击须 sleep ≥60ms 再 mouseDown、
 * 点 .ant-select-item-option-content；ModalForm 提交锚
 * .ant-modal-footer .ant-btn-primary（submitText「确定」双字中文渲染插
 * 空格）；Popconfirm 确认锚未隐藏 .ant-popover 内 .ant-btn-primary；
 * ModalForm 关闭后整体卸载——.ant-modal 从 DOM 消失（探针实证
 * modalCount=0，区别于普通 Modal 的 display:none 残留）；InputNumber 取
 * .ant-input-number input。
 */
import React from 'react';
import { configure, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { App } from 'antd';
import TermsPage from '../index';
import type { TermItem } from '@/services/api/terms';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

jest.mock('@/services/api/terms', () => ({
  listTerms: jest.fn(),
  upsertTerm: jest.fn(),
  deleteTerm: jest.fn(),
}));

const mockIntl = {
  locale: 'zh-CN',
  formatMessage: (opts: { defaultMessage?: string }) => opts.defaultMessage ?? '',
};
jest.mock('@umijs/max', () => ({
  FormattedMessage: ({ defaultMessage }: { defaultMessage?: string }) => (
    <>{defaultMessage ?? ''}</>
  ),
  useIntl: () => mockIntl,
  getIntl: () => mockIntl,
}));

// 桩：受控 value/onChange，按 BCP47 zh-CN 键读写
jest.mock('@/components/LocalizedTextEditor', () => ({
  __esModule: true,
  default: ({
    value,
    onChange,
  }: {
    value?: Record<string, string>;
    onChange?: (v: Record<string, string>) => void;
  }) => (
    <input
      aria-label="display-editor"
      value={value?.['zh-CN'] ?? ''}
      onChange={(e) => onChange?.({ ...(value || {}), 'zh-CN': e.target.value })}
    />
  ),
}));

import { deleteTerm, listTerms, upsertTerm } from '@/services/api/terms';

const mList = listTerms as jest.MockedFunction<typeof listTerms>;
const mUpsert = upsertTerm as jest.MockedFunction<typeof upsertTerm>;
const mDel = deleteTerm as jest.MockedFunction<typeof deleteTerm>;

const ITEMS: TermItem[] = [
  {
    id: 1,
    domain: 'resource',
    termKey: 'player',
    alias: 'players',
    display: { 'zh-CN': '玩家', 'en-US': 'Player' },
    order: 10,
  },
  // 覆盖翼：domain 紫 + zh-CN 空串 → 显示走 en 回退、语言列过滤空值
  {
    id: 2,
    domain: 'operation',
    termKey: 'read',
    alias: 'list',
    display: { 'zh-CN': '', 'en-US': 'List' },
    order: 20,
  },
  // 覆盖翼：display/order/id 全缺省 → 显示回退 termKey、语言 '-'、
  // row.display || {} 右臂
  { domain: 'resource', termKey: 'plain', alias: 'plain-alias' },
];

beforeEach(() => {
  jest.clearAllMocks();
  mList.mockResolvedValue(ITEMS);
  mUpsert.mockResolvedValue({ ok: true });
  mDel.mockResolvedValue({ ok: true });
});

function renderPage() {
  return render(
    <App>
      <TermsPage />
    </App>,
  );
}

/** 等首拉落定（首行别名出现） */
async function waitLoad() {
  await waitFor(() => expect(mList).toHaveBeenCalledTimes(1));
  expect(await screen.findByText('players')).toBeInTheDocument();
}

function rowOf(text: string) {
  return screen.getByText(text).closest('tr') as HTMLElement;
}

/** 弹窗标题（「新增术语」与页头按钮同文本，按 modal-title 锚） */
function modalTitle() {
  return document.querySelector('.ant-modal-title')?.textContent ?? '';
}

/** 打开下拉并点选可见选项文本（idx 为页内 .ant-select DOM 序） */
async function pickOption(idx: number, label: string) {
  await new Promise((r) => setTimeout(r, 60));
  fireEvent.mouseDown(document.querySelectorAll('.ant-select')[idx] as HTMLElement);
  const dropdown = await waitFor(() => {
    const visible = Array.from(document.querySelectorAll('.ant-select-dropdown')).find(
      (d) => !d.className.includes('ant-select-dropdown-hidden'),
    ) as HTMLElement;
    expect(visible).not.toBeUndefined();
    return visible;
  });
  fireEvent.click(
    within(dropdown).getByText(label, { selector: '.ant-select-item-option-content' }),
  );
}

/** Popconfirm 确认（未隐藏实例内的主按钮——zh/en 文案两态通吃） */
async function confirmPopover() {
  const popover = await waitFor(() => {
    const visible = Array.from(document.querySelectorAll('.ant-popover')).find(
      (p) => !p.className.includes('ant-popover-hidden'),
    ) as HTMLElement;
    expect(visible).not.toBeUndefined();
    return visible;
  });
  fireEvent.click(popover.querySelector('.ant-btn-primary') as HTMLElement);
}

/** ModalForm 提交（footer 主按钮，「确 定」双字中文插空格，按类名锚） */
function submitModal() {
  fireEvent.click(document.querySelector('.ant-modal-footer .ant-btn-primary') as HTMLElement);
}

describe('术语字典 列表渲染与筛选', () => {
  it('挂载加载 resource + 渲染矩阵（domain 双色 / display 三臂 / locales 过滤）', async () => {
    renderPage();
    await waitLoad();
    expect(mList.mock.calls[0][0]).toBe('resource');

    // row1：蓝 Tag + zh 命中 + 双语言 + order
    const r1 = rowOf('players');
    expect(within(r1).getByText('resource').closest('.ant-tag')).toHaveClass('ant-tag-blue');
    expect(within(r1).getByText('玩家')).toBeInTheDocument();
    expect(within(r1).getByText('zh-CN / en-US')).toBeInTheDocument();
    expect(within(r1).getByText('10')).toBeInTheDocument();

    // row2：紫 Tag + zh 空串 → en 回退 + 空值 locale 被过滤
    const r2 = rowOf('list');
    expect(within(r2).getByText('operation').closest('.ant-tag')).toHaveClass('ant-tag-purple');
    expect(within(r2).getByText('List')).toBeInTheDocument();
    expect(within(r2).getByText('en-US')).toBeInTheDocument();

    // row3：display 缺省 → termKey 兜底（Key 列与显示列同文本双命中）；
    // locales '-' 与 Order 空值（ProTable 空单元格兜底）同文本双命中
    const r3 = rowOf('plain-alias');
    expect(within(r3).getAllByText('plain')).toHaveLength(2);
    expect(within(r3).getAllByText('-')).toHaveLength(2);
  });

  it('切域 Operation → listTerms(operation) 重拉', async () => {
    renderPage();
    await waitLoad();

    await pickOption(0, 'Operation');
    await waitFor(() => expect(mList).toHaveBeenLastCalledWith('operation'));
    expect(mList).toHaveBeenCalledTimes(2);
  });

  it('load 失败 → toast 透传 Error.message', async () => {
    mList.mockRejectedValueOnce(new Error('terms-down'));
    renderPage();
    expect(await screen.findByText('terms-down')).toBeInTheDocument();
  });
});

describe('术语字典 新增与编辑', () => {
  it('新增：默认值（当前域 + order 100）+ 全字段落库 + 重拉 + 关闭', async () => {
    renderPage();
    await waitLoad();

    fireEvent.click(screen.getByRole('button', { name: '新增术语' }));
    await waitFor(() => expect(modalTitle()).toBe('新增术语'));
    // 默认值：domain=当前筛选 resource、order=100、display 空
    expect(document.querySelector('.ant-modal .ant-select')?.textContent).toContain('Resource');
    expect(
      (document.querySelector('.ant-modal .ant-input-number input') as HTMLInputElement).value,
    ).toBe('100');
    const editor = screen.getByLabelText('display-editor') as HTMLInputElement;
    expect(editor.value).toBe('');

    fireEvent.change(screen.getByPlaceholderText('player / read'), {
      target: { value: 'ticket' },
    });
    fireEvent.change(screen.getByPlaceholderText('players / list'), {
      target: { value: 'tickets' },
    });
    fireEvent.change(editor, { target: { value: '工单' } });

    mList.mockResolvedValueOnce([
      ...ITEMS,
      {
        domain: 'resource',
        termKey: 'ticket',
        alias: 'tickets',
        display: { 'zh-CN': '工单' },
        order: 100,
      },
    ]);
    submitModal();
    await waitFor(() =>
      expect(mUpsert).toHaveBeenCalledWith({
        domain: 'resource',
        termKey: 'ticket',
        alias: 'tickets',
        display: { 'zh-CN': '工单' },
        order: 100,
      }),
    );
    expect(await screen.findByText('保存成功')).toBeInTheDocument();
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(2));
    expect(await screen.findByText('tickets')).toBeInTheDocument();
    // ModalForm 关闭后整体卸载（destroyOnHidden 下 .ant-modal 从 DOM 消失
    // ——探针实证 modalCount=0，区别于普通 Modal 的 display:none 残留）
    await waitFor(() => expect(document.querySelector('.ant-modal')).toBeNull());
  });

  it('新增 required 拦截：不触达服务、弹窗保持', async () => {
    renderPage();
    await waitLoad();

    fireEvent.click(screen.getByRole('button', { name: '新增术语' }));
    await waitFor(() => expect(modalTitle()).toBe('新增术语'));
    submitModal();
    await new Promise((r) => setTimeout(r, 200));

    expect(mUpsert).not.toHaveBeenCalled();
    expect(mList).toHaveBeenCalledTimes(1);
    expect(modalTitle()).toBe('新增术语');
  });

  it('编辑：整行回填 + 载荷不含 id + 别名更新落表', async () => {
    renderPage();
    await waitLoad();

    fireEvent.click(within(rowOf('players')).getByText('编辑'));
    await waitFor(() => expect(modalTitle()).toBe('编辑术语'));
    // 回填：termKey / alias / display / order
    expect(screen.getByPlaceholderText('player / read')).toHaveValue('player');
    expect(screen.getByPlaceholderText('players / list')).toHaveValue('players');
    expect(screen.getByLabelText('display-editor')).toHaveValue('玩家');
    expect(
      (document.querySelector('.ant-modal .ant-input-number input') as HTMLInputElement).value,
    ).toBe('10');

    fireEvent.change(screen.getByPlaceholderText('players / list'), {
      target: { value: 'gamers' },
    });
    mList.mockResolvedValueOnce([
      {
        id: 1,
        domain: 'resource',
        termKey: 'player',
        alias: 'gamers',
        display: { 'zh-CN': '玩家', 'en-US': 'Player' },
        order: 10,
      },
      ...ITEMS.slice(1),
    ]);
    submitModal();
    await waitFor(() =>
      expect(mUpsert).toHaveBeenCalledWith({
        domain: 'resource',
        termKey: 'player',
        alias: 'gamers',
        display: { 'zh-CN': '玩家', 'en-US': 'Player' },
        order: 10,
      }),
    );
    // id 不参与提交（TermFormValues = Omit<TermItem,'id'>，id 非 Form.Item）
    expect('id' in mUpsert.mock.calls[0][0]).toBe(false);
    expect(await screen.findByText('gamers')).toBeInTheDocument();
  });

  it('保存失败：return false 弹窗保持、不重拉、无本地弹错', async () => {
    renderPage();
    await waitLoad();

    fireEvent.click(within(rowOf('players')).getByText('编辑'));
    await waitFor(() => expect(modalTitle()).toBe('编辑术语'));
    mUpsert.mockRejectedValueOnce(new Error('save-down'));
    submitModal();
    await waitFor(() => expect(mUpsert).toHaveBeenCalledTimes(1));
    await new Promise((r) => setTimeout(r, 150));

    expect(modalTitle()).toBe('编辑术语');
    expect(mList).toHaveBeenCalledTimes(1);
  });
});

describe('术语字典 删除', () => {
  it('Popconfirm 确认 → deleteTerm(domain, alias) + 已删除 + 重拉', async () => {
    renderPage();
    await waitLoad();

    mList.mockResolvedValueOnce([ITEMS[0], ITEMS[2]]);
    fireEvent.click(within(rowOf('list')).getByText('删除'));
    await confirmPopover();

    await waitFor(() => expect(mDel).toHaveBeenCalledWith('operation', 'list'));
    expect(await screen.findByText('已删除')).toBeInTheDocument();
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(screen.queryByText('read')).toBeNull());
    expect(screen.getByText('players')).toBeInTheDocument();
  });
});
