/**
 * 函数权限配置页单测（覆盖率补缺轮：Permissions/index.tsx 222 行 0% → 收口，
 * 零测试簇排行现席）。
 *
 * 锁定契约：
 * - 挂载：fetchSummary → getFunctionSummary()（无参）→ 数组行渲染
 *   （函数 id / displayName.zh / verbs Tag / scopes Tag / 编辑链接）；
 * - 列渲染兜底：行缺 permissions 字段（后端 JSON 省略形态）→
 *   `(r.permissions?.verbs || [])`/scopes 右翼 → 空格无 Tag；
 * - reload 失败：reject Error → e.message toast；reject 非 Error
 *   （字符串）→ intl 兜底（id 回显）；finally loading 收尾不白屏；
 * - 编辑：行内「编辑」→ getAdminFunctionPermissions(fid) → permDraft 落表
 *   （verbs/scopes tags 回显 + 中文文案动态输入 initialValues 回显 +
 *   仅配置过的 verb 出输入框）；缺 permissions 行 → 空表单 + 中文组无输入框
 *   （formI18nKeys 空 && permDraft.verbs || [] 右翼）；
 * - 保存：values.verbs/scopes 直取 → setAdminFunctionPermissions(fid,
 *   {verbs, scopes, defaults: draft.slice(), i18n_zh: 仅非空串键}) →
 *   success toast + 弹窗关闭 + 重拉；i18n_zh 只收「有值且为 string」的键
 *   （initialValue '' 的 verb 不进 payload）；
 * - 动态中文输入：verbs tags 追加 → onChange setFormI18nKeys → 中文组
 *   即时渲染新 verb 输入框（formI18nKeys 左翼）→ 填值进 payload；
 * - 保存失败：reject Error / 非 Error 双翼 → error toast + return false
 *   弹窗保持；
 * - 取消：onOpenChange(false) → setEditing(null) 关闭。
 *
 * mock 口径：services/api/{functions-enhanced,admin} 三函数 jest.mock；
 * @umijs/max 本地 formatMessage mock——本页 intl 调用全部 **不带
 * defaultMessage**（id-only），mock 取 `defaultMessage ?? id ?? ''` 使
 * 标题/按钮/toast 文案确定性可见（标题 `：${fid}`、编辑链接/成功文案
 * 按 id 断言）；pro-components spread requireActual（ProTable/ModalForm/
 * ProFormSelect 真实实现）+ PageContainer 桩；antd App 真实包裹
 * （App.useApp().message 真实例，toast 经 portal 断言文本）。
 *
 * 附带修定（回归锁定，已落生产代码）：编辑动作原为「先 setEditing 后
 * await fetchPermissions」——ModalForm 内容在 open 翻真时即以旧 permDraft
 * （首次 {}）挂载，而 antd initialValue 挂载后不随 prop 变化重放：
 * ① 编辑态 verbs/scopes tags 恒空显示（placeholder 常驻）；② 因 tags
 * onChange 整组覆盖 store 值，追加/删减任一 tag 后 values.verbs 即为
 * 手工小集合，提交把既有 verbs/scopes/i18n 静默丢弃（保存丢值）；
 * ③ form store 跨开合存活（无 destroyOnHidden），二次编辑沿用首开旧值。
 * 修定 = 先取权限再与 setEditing 同批落 state + formI18nKeys 开启重置 +
 * modalProps.destroyOnHidden；「编辑回显 read/write tags」「追加 exec 后
 * i18n_zh 仍含 read」两组断言即回归锁（修定前实跑取证：tags 恒空、
 * zh 输入框只随手工键渲染）。
 *
 * 现状锁定 / 边界（诚实清单，不造假用例不删防御分支）：
 * 1. fetchSummary 的 `Array.isArray(res)` 右翼——getFunctionSummary 归一
 *    层恒返 FunctionSummary[]（FunctionSummaryListResponse | Raw[] 统一），
 *    resolve 非数组违反返回类型即造假，登记。
 * 2. `perm || {}` 右翼——getAdminFunctionPermissions 恒返
 *    `res?.permissions || {}`，null/undefined 形态违反返回类型，登记；
 *    {} 形态经缺 permissions 行真实覆盖（表单空态 + 空表单直提链）。
 * 3. onFinish 的 `values.verbs || permDraft.verbs || []` 三段链——满配置
 *    行走左翼（表单值直取）；缺 permissions 行空表单直提走中+尾翼
 *    （values.verbs undefined → draft undefined → []，scopes/defaults
 *    同族），均真实覆盖。`values.verbs` 为「左值但 draft 有值」的混合态
 *    仅修定前的竞态存在（弹窗先挂载后落 draft），修定后不可达。
 * 4. `(verbs || [])`（forEach 收集处）——verbs 是链结果恒真值，
 *    右翼防御性登记。
 * 5. verbs Select onChange 的 `(vals as string[]) || []` 右翼——antd tags
 *    模式 onChange 恒传数组，登记。
 * 6. reload/onFinish 的 `e instanceof Error` 双翼均真实触达（Error/
 *    字符串 reject）；i18n 收集的 `typeof val === 'string'` 非 string 翼
 *    ——ProFormText 值恒 string|undefined，登记。
 * 7. 编辑 onClick 无 catch——fetchPermissions reject 成 unhandled
 *    rejection 且弹窗不开（修定后先取数后开窗），同族页面既有口径，
 *    不造假 reject 场景。
 */
import React from 'react';
import { configure, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import PermissionsPage from '../index';
import { getAdminFunctionPermissions, setAdminFunctionPermissions } from '@/services/api/admin';
import { getFunctionSummary } from '@/services/api/functions-enhanced';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

jest.mock('@/services/api/functions-enhanced', () => ({
  getFunctionSummary: jest.fn(),
}));

jest.mock('@/services/api/admin', () => ({
  getAdminFunctionPermissions: jest.fn(),
  setAdminFunctionPermissions: jest.fn(),
}));

// 本页 intl 全部 id-only（无 defaultMessage）——mock 回 id 使文案可见
jest.mock('@umijs/max', () => {
  const fmt = (opts: { id?: string; defaultMessage?: string }, values?: Record<string, string>) => {
    let msg = opts.defaultMessage ?? opts.id ?? '';
    if (values) {
      for (const [k, v] of Object.entries(values)) msg = msg.split(`{${k}}`).join(v);
    }
    return msg;
  };
  const intl = { formatMessage: fmt, locale: 'zh-CN' };
  return {
    useIntl: () => intl,
    FormattedMessage: ({ defaultMessage }: { defaultMessage?: string }) => (
      <>{defaultMessage ?? ''}</>
    ),
  };
});

jest.mock('@ant-design/pro-components', () => ({
  ...jest.requireActual('@ant-design/pro-components'),
  PageContainer: ({ children }: { children?: React.ReactNode }) => <div>{children}</div>,
}));

const mSummary = getFunctionSummary as jest.MockedFunction<typeof getFunctionSummary>;
const mGetPerms = getAdminFunctionPermissions as jest.MockedFunction<
  typeof getAdminFunctionPermissions
>;
const mSetPerms = setAdminFunctionPermissions as jest.MockedFunction<
  typeof setAdminFunctionPermissions
>;

// 行 1：满配置（verbs/scopes/defaults/i18n_zh）；行 2：缺 permissions 字段
// （列渲染右翼 + 空表单）；行 3：permissions 空对象（空数组左翼）
const ROWS = [
  {
    id: 'fn.edit.full',
    displayName: { zh: '全量函数' },
    permissions: {
      verbs: ['read', 'write'],
      scopes: ['game', 'env'],
      defaults: [{ role: 'admin', verbs: ['read'] }],
      i18n_zh: { read: '读', write: '' },
    },
  },
  { id: 'fn.bare', displayName: { zh: '裸函数' } },
  { id: 'fn.empty', displayName: { zh: '空配置' }, permissions: {} },
];

function renderPage() {
  return render(<PermissionsPage />);
}

/** ModalForm 主提交按钮（footer 内主按钮） */
function footerOk() {
  const footer = document.querySelector('.ant-modal-footer') as HTMLElement;
  return within(footer).getByRole('button', { name: /确\s*认/ });
}

async function waitLoaded() {
  await waitFor(() => expect(mSummary).toHaveBeenCalledTimes(1));
  await waitFor(() => expect(screen.getByText('fn.edit.full')).toBeInTheDocument());
}

beforeEach(() => {
  jest.clearAllMocks();
  mSummary.mockResolvedValue(ROWS as never);
  mGetPerms.mockResolvedValue({} as never);
  mSetPerms.mockResolvedValue(undefined as never);
});

describe('函数权限配置 挂载与列表', () => {
  it('首拉无参 + 列渲染（id/名称/verbs/scopes Tag/编辑链接）', async () => {
    renderPage();
    await waitLoaded();

    expect(mSummary).toHaveBeenCalledWith();

    const table = document.querySelector('.ant-table') as HTMLElement;
    expect(within(table).getByText('fn.edit.full')).toBeInTheDocument();
    expect(within(table).getByText('全量函数')).toBeInTheDocument();
    // verbs/scopes Tag（列 title 均为 id 串，Tag 文本才是业务值）
    expect(within(table).getAllByText('read').length).toBeGreaterThan(0);
    expect(within(table).getByText('write')).toBeInTheDocument();
    expect(within(table).getByText('game')).toBeInTheDocument();
    expect(within(table).getByText('env')).toBeInTheDocument();
    // 编辑链接按 id 串渲染 ×3（intl id-only）
    expect(screen.getAllByText('pages.permissions.edit.button')).toHaveLength(3);
  });

  it('行缺 permissions / 空配置 → verbs/scopes 格无 Tag（|| 右翼）', async () => {
    renderPage();
    await waitLoaded();

    const table = document.querySelector('.ant-table') as HTMLElement;
    const bareRow = (await within(table).findByText('fn.bare')).closest('tr') as HTMLElement;
    expect(within(bareRow).queryByText('read')).not.toBeInTheDocument();
    expect(within(bareRow).queryByText('game')).not.toBeInTheDocument();
    const emptyRow = within(table).getByText('fn.empty').closest('tr') as HTMLElement;
    expect(within(emptyRow).queryByText('read')).not.toBeInTheDocument();
  });

  it('加载失败：reject Error → e.message toast；非 Error → intl id 兜底', async () => {
    mSummary.mockRejectedValueOnce(new Error('summary down'));
    renderPage();
    expect(await screen.findByText('summary down')).toBeInTheDocument();

    // 非 Error（字符串）wing：e instanceof Error false → formatMessage(id)
    // （首个 render 未卸载、其 toast 为 'summary down'，id 文案仅第二渲染 1 份）
    mSummary.mockRejectedValueOnce('raw string' as never);
    renderPage();
    expect(await screen.findAllByText('pages.permissions.load.error')).toHaveLength(1);
  });
});

describe('函数权限配置 编辑弹窗', () => {
  it('行编辑：拉权限 + 标题内插 + tags/中文输入回显（仅配置过的 verb 有输入框）', async () => {
    mGetPerms.mockResolvedValueOnce(ROWS[0].permissions as never);
    renderPage();
    await waitLoaded();

    const table = document.querySelector('.ant-table') as HTMLElement;
    const row = within(table).getByText('fn.edit.full').closest('tr') as HTMLElement;
    fireEvent.click(within(row).getByText('pages.permissions.edit.button'));

    expect(mGetPerms).toHaveBeenCalledWith('fn.edit.full');
    await waitFor(() => {
      expect(document.querySelector('.ant-modal-title')).toHaveTextContent(
        'pages.permissions.configure：fn.edit.full',
      );
    });
    // verbs/scopes tags 回显（弹窗内两个 Select 根 textContent）
    const selects = () =>
      Array.from(document.querySelectorAll('.ant-modal .ant-select')) as HTMLElement[];
    await waitFor(() => expect(selects()[0]?.textContent).toContain('read'));
    expect(selects()[0].textContent).toContain('write');
    expect(selects()[1].textContent).toContain('game');
    // 中文输入回显：read → '读'（write 的 '' 不回显非空值）
    expect(screen.getByDisplayValue('读')).toBeInTheDocument();
    // 仅 read/write 两键出输入框（formI18nKeys 空 → permDraft.verbs 右支）
    expect(screen.getAllByDisplayValue('读')).toHaveLength(1);
    const modal = document.querySelector('.ant-modal') as HTMLElement;
    expect(
      within(modal).getAllByPlaceholderText('pages.permissions.verb.chinese.placeholder').length,
    ).toBe(2);
  });

  it('缺 permissions 行 → 空表单 + 中文组无输入框（permDraft.verbs || [] 右翼）', async () => {
    mGetPerms.mockResolvedValueOnce({} as never);
    renderPage();
    await waitLoaded();

    const table = document.querySelector('.ant-table') as HTMLElement;
    const row = within(table).getByText('fn.bare').closest('tr') as HTMLElement;
    fireEvent.click(within(row).getByText('pages.permissions.edit.button'));

    await waitFor(() => {
      expect(document.querySelector('.ant-modal-title')).toHaveTextContent(
        'pages.permissions.configure：fn.bare',
      );
    });
    expect(
      document.querySelectorAll('.ant-modal .ant-select input, .ant-modal input').length,
    ).toBeGreaterThanOrEqual(2); // verbs + scopes 两个 tags 输入
    // 中文组无任何 ProFormText 输入（无 placeholder 动态框）
    expect(
      document.querySelectorAll(
        '.ant-modal input[placeholder="pages.permissions.verb.chinese.placeholder"]',
      ).length,
    ).toBe(0);

    // 空表单直提：values.verbs/scopes undefined → `|| permDraft.* || []` 中尾
    // 翼全链落 []（defaults 同 (draft || []).slice() 右翼）+ i18n_zh 空对象
    fireEvent.click(footerOk());
    await waitFor(() => expect(mSetPerms).toHaveBeenCalledTimes(1));
    expect(mSetPerms).toHaveBeenCalledWith('fn.bare', {
      verbs: [],
      scopes: [],
      defaults: [],
      i18n_zh: {},
    });
    expect(await screen.findByText('pages.permissions.save.success')).toBeInTheDocument();
  });

  it('保存主链：载荷（i18n_zh 只收非空串键）+ success toast + 关闭 + 重拉', async () => {
    mGetPerms.mockResolvedValueOnce(ROWS[0].permissions as never);
    renderPage();
    await waitLoaded();

    const table = document.querySelector('.ant-table') as HTMLElement;
    const row = within(table).getByText('fn.edit.full').closest('tr') as HTMLElement;
    fireEvent.click(within(row).getByText('pages.permissions.edit.button'));
    await waitFor(() => expect(mGetPerms).toHaveBeenCalledTimes(1));

    fireEvent.click(footerOk());

    await waitFor(() => expect(mSetPerms).toHaveBeenCalledTimes(1));
    expect(mSetPerms).toHaveBeenCalledWith('fn.edit.full', {
      verbs: ['read', 'write'],
      scopes: ['game', 'env'],
      defaults: [{ role: 'admin', verbs: ['read'] }],
      i18n_zh: { read: '读' }, // write 的 initialValue '' 为 falsy → 不进 payload
    });
    expect(await screen.findByText('pages.permissions.save.success')).toBeInTheDocument();
    await waitFor(() => expect(mSummary).toHaveBeenCalledTimes(2));
    // 关闭（ModalForm 无 destroyOnHidden → display:none 残留或卸载二择）
    await waitFor(() => {
      const modal = document.querySelector('.ant-modal') as HTMLElement | null;
      if (modal) expect(modal.style.display).toBe('none');
    });
  });

  it('追加 verb → 动态中文输入渲染（formI18nKeys 左翼）→ 填值进 payload', async () => {
    mGetPerms.mockResolvedValueOnce(ROWS[0].permissions as never);
    renderPage();
    await waitLoaded();

    const table = document.querySelector('.ant-table') as HTMLElement;
    const row = within(table).getByText('fn.edit.full').closest('tr') as HTMLElement;
    fireEvent.click(within(row).getByText('pages.permissions.edit.button'));
    await waitFor(() => expect(screen.getByDisplayValue('读')).toBeInTheDocument());

    // verbs tags 追加 exec：Enter 提交首 token
    const verbsSelect = document.querySelectorAll('.ant-modal .ant-select')[0] as HTMLElement;
    const input = verbsSelect.querySelector('input') as HTMLInputElement;
    fireEvent.mouseDown(verbsSelect);
    fireEvent.change(input, { target: { value: 'exec' } });
    fireEvent.keyDown(input, { key: 'Enter', code: 'Enter', keyCode: 13 });
    await waitFor(() => expect(verbsSelect.textContent).toContain('exec'));

    // formI18nKeys → [read, write, exec]，中文组即时渲染第三个输入框
    const zhInputs = () =>
      Array.from(
        document.querySelectorAll(
          '.ant-modal input[placeholder="pages.permissions.verb.chinese.placeholder"]',
        ),
      ) as HTMLInputElement[];
    await waitFor(() => expect(zhInputs().length).toBe(3));
    fireEvent.change(zhInputs()[2], { target: { value: '执' } });

    fireEvent.click(footerOk());
    await waitFor(() =>
      expect(mSetPerms).toHaveBeenCalledWith(
        'fn.edit.full',
        expect.objectContaining({ i18n_zh: { read: '读', exec: '执' } }),
      ),
    );
  });

  it('保存失败：Error / 非 Error 双翼 → error toast + 弹窗保持', async () => {
    mGetPerms.mockResolvedValueOnce(ROWS[0].permissions as never);
    renderPage();
    await waitLoaded();

    const table = document.querySelector('.ant-table') as HTMLElement;
    const row = within(table).getByText('fn.edit.full').closest('tr') as HTMLElement;
    fireEvent.click(within(row).getByText('pages.permissions.edit.button'));
    await waitFor(() => expect(mGetPerms).toHaveBeenCalledTimes(1));

    mSetPerms.mockRejectedValueOnce(new Error('denied'));
    fireEvent.click(footerOk());
    expect(await screen.findByText('denied')).toBeInTheDocument();
    expect(document.querySelector('.ant-modal')).not.toBeNull();

    mSetPerms.mockRejectedValueOnce('raw fail' as never);
    fireEvent.click(footerOk());
    expect(await screen.findAllByText('pages.permissions.save.error')).toHaveLength(1);
    // return false → 弹窗保持
    expect((document.querySelector('.ant-modal') as HTMLElement).style.display).not.toBe('none');
  });

  it('取消关闭：onOpenChange(false) → setEditing(null)', async () => {
    mGetPerms.mockResolvedValueOnce(ROWS[0].permissions as never);
    renderPage();
    await waitLoaded();

    const table = document.querySelector('.ant-table') as HTMLElement;
    const row = within(table).getByText('fn.edit.full').closest('tr') as HTMLElement;
    fireEvent.click(within(row).getByText('pages.permissions.edit.button'));
    await waitFor(() => expect(mGetPerms).toHaveBeenCalledTimes(1));

    const footer = document.querySelector('.ant-modal-footer') as HTMLElement;
    fireEvent.click(within(footer).getByRole('button', { name: /取\s*消/ }));
    await waitFor(() => {
      const modal = document.querySelector('.ant-modal') as HTMLElement | null;
      if (modal) expect(modal.style.display).toBe('none');
    });
    expect(mSetPerms).not.toHaveBeenCalled();
  });
});
