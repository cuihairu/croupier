/**
 * 角色管理页单测（覆盖率补缺轮：Permissions/RolesV2/index.tsx 312 行
 * 0% → 收口，零测试簇排行现席）。
 *
 * 锁定契约：
 * - 挂载：refresh(1, pageSize) → listRoles({page:1, pageSize:10})；
 *   卡片标题「角色管理」+ 新增按钮 + 列头（名称/描述/权限/操作）+
 *   showTotal `共 N 条`；
 * - 权限列：`(arr || []).slice(0, 6)` —— 夹具 8 权限行只上 6 个 Tag、
 *   行缺 permissions 字段（后端 JSON 省略形态）→ 空；
 * - 响应归一：`r.items || []`（缺 items → 空表）、`r.total ??
 *   (r.items || []).length`（缺 total → 回落 items 长度）；
 * - 分页：onChange(nextPage, nextSize) → setPage + setPageSize +
 *   refresh(nextPage, nextSize) 透传请求参数；
 * - 新增：openAdd（editing=null）→ 标题「新增角色」→ name 必填拦截
 *   （请输入名称，不触达服务）→ createRole({name, description,
 *   permissions: []}) → toast `已创建 #{id}` → 重拉 → 弹窗关闭；
 * - 编辑：openEdit → 标题「编辑角色」+ initialValues 回填 → 改值提交 →
 *   updateRole(id, {name, description}) → toast 已更新 → 重拉 → 关；
 *   失败（reject）→ onFinish false → 弹窗保持开启、无 success toast；
 * - 权限弹窗：openPerms → 标题 `编辑权限：{name}` + initialValues
 *   permissions 回显 → tags Select 追加 → updateRolePermissions(id, perms)
 *   → toast 权限已更新 → 重拉 → 关；失败 → 弹窗保持；
 * - 删除：Popconfirm 确认 → deleteRole(id) → toast 已删除 → 重拉。
 *
 * mock 口径：services/api/permissions 五函数 jest.mock；@/utils/antdApp
 * getMessage 桩（mockMessageApi.success 捕获 toast）；@umijs/max 本地
 * defaultMessage mock（{key} values replace，模板串已在调用点插值、
 * passthrough 即得原文）；pro-components spread requireActual 真实实现（ModalForm
 * 校验/destroyOnHidden 行为依赖本体）+ PageContainer 桩；antd 真实实现
 * （App 包裹）。
 *
 * 附带修定（回归锁定，已落生产代码）：两个 Form.Item（name/description）
 * 原为 `{' '}<Input />{' '}` 三元素数组子节点——antd Form.Item 源码
 * `Array.isArray(mergedChildren) && hasName` 分支只发 dev warning 并原样
 * 渲染，**不做 cloneElement 控制注入**（同 Round 40 RateLimits 缺陷）：
 * 输入脱管于 form store——新建时 name 恒 undefined 过不了 required
 * （新增角色不可用）、编辑时键入值不落 store（提交恒为初始值）、
 * label htmlFor 无 id 可指（getByLabelText 找不到 control）。
 * 修定 = 去掉 `{' '}` 恢复单子节点（套件曾对未修页面实跑取证：
 * 修前「回填显示」断言挂在 label 关联失败）；「键入值进载荷」
 * 「回填显示」两断言即回归锁。
 *
 * 坑实证（antd6/RTL 新档，四条）：
 * - 分页 `showSizeChanger: true` 的 size-changer Select 在主内容区、
 *   DOM 序先于 portal 弹窗——`document.querySelector('.ant-select')`
 *   打到 page-size 选择器（表象：tags 输入毫无反应、载荷恒 []），
 *   Select 锚必须限 `.ant-modal`；
 * - antd6 Pager 是 `<li title="2" onClick><a rel="nofollow">2</a></li>`——
 *   a 无 href 无 button/link role，getByRole('button',{name:'2'}) 恒空，
 *   翻页点击落 `.ant-pagination-item-N` 的 li 本体（onClick 挂 li）；
 * - total=0 时 antd Table 不渲染分页（showTotal `共 0 条` 不可见）——
 *   空态断言锚 `.ant-empty-description`，且 Empty 图标 svg 内
 *   `<title>No data</title>` 同文双命中须 selector 收窄；
 * - 弹窗标题与工具栏按钮同文本（新增角色）——标题断言锚
 *   `.ant-modal-title` textContent；同用例内多渲一次会多吃一个
 *   `mockResolvedValueOnce` 队列（unmount 取自首渲）。
 *
 * 现状锁定 / 边界（诚实清单，不造假用例不删防御分支）：
 * 1. submitPerms 的 `if (!editing) return false` 守卫——权限弹窗唯一
 *    打开路径 openPerms 先置 editing 再开窗，editing 恒非空，构造性
 *    不可达，登记。
 * 2. submitPerms 的 `v.permissions || []` 右翼——initialValues 恒设
 *    permissions 键、tags Select 清空回 []，undefined 形态违反表单
 *    值契约，登记。
 * 3. refresh 与 remove 无 catch——listRoles/deleteRole reject 成
 *    unhandled rejection（同族页面既有口径），不造假 reject 场景；
 *    create/update/updatePerms 三者有 catch→false，真实 reject 触达。
 * 4. description 经 payload 的 `description: v.description` 直传
 *    （无 nullish 算子、无分支差异）——已断言 undefined 态（新增失败：
 *    表单缺省键不存在）与显式值态（主链）；'' 显式清空态与前者走
 *    同一行代码，不另铺用例，登记。
 */
import React from 'react';
import { configure, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { App } from 'antd';
import RolesV2 from '../index';
import {
  createRole,
  deleteRole,
  listRoles,
  updateRole,
  updateRolePermissions,
  type RoleRecord,
} from '@/services/api/permissions';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

jest.mock('@/services/api/permissions', () => ({
  listRoles: jest.fn(),
  createRole: jest.fn(),
  updateRole: jest.fn(),
  deleteRole: jest.fn(),
  updateRolePermissions: jest.fn(),
}));

// factory 只存引用、getMessage 调用时才读（TDZ 安全，UsersV2 同款）
const mockMessageApi = { error: jest.fn(), success: jest.fn(), warning: jest.fn() };
jest.mock('@/utils/antdApp', () => ({ getMessage: () => mockMessageApi }));

jest.mock('@umijs/max', () => ({
  FormattedMessage: ({ defaultMessage }: { defaultMessage?: string }) => (
    <>{defaultMessage ?? ''}</>
  ),
  useIntl: () => ({
    formatMessage: (opts: { defaultMessage?: string }, values?: Record<string, string>) => {
      let text = opts.defaultMessage ?? '';
      if (values) {
        for (const [k, v] of Object.entries(values)) text = text.replace(`{${k}}`, v);
      }
      return text;
    },
  }),
}));

// ModalForm 的 footer 校验/destroyOnHidden 依赖真实实现，仅 PageContainer 桩
jest.mock('@ant-design/pro-components', () => ({
  ...jest.requireActual('@ant-design/pro-components'),
  PageContainer: ({ children }: { children?: React.ReactNode }) => <div>{children}</div>,
}));

const mList = listRoles as jest.MockedFunction<typeof listRoles>;
const mCreate = createRole as jest.MockedFunction<typeof createRole>;
const mUpdate = updateRole as jest.MockedFunction<typeof updateRole>;
const mDel = deleteRole as jest.MockedFunction<typeof deleteRole>;
const mPerms = updateRolePermissions as jest.MockedFunction<typeof updateRolePermissions>;

// 行 1：8 权限 → slice(0,6) 只上 6 个；行 2：空描述 + 空权限；
// 行 3：缺 permissions 字段（后端 JSON 省略形态 → (arr || []) 右翼）
const ROLES: RoleRecord[] = [
  {
    id: 1,
    name: 'admin',
    description: '超级管理员',
    category: 'system',
    permissions: ['p1', 'p2', 'p3', 'p4', 'p5', 'p6', 'p7', 'p8'],
    createdAt: '2026-09-01T00:00:00Z',
    updatedAt: '2026-09-02T00:00:00Z',
  },
  {
    id: 2,
    name: 'ops',
    description: '',
    category: 'custom',
    permissions: [],
    createdAt: '2026-09-01T00:00:00Z',
    updatedAt: '2026-09-02T00:00:00Z',
  },
  {
    id: 3,
    name: 'ghost',
    description: '缺权限字段',
    category: 'custom',
    createdAt: '2026-09-01T00:00:00Z',
    updatedAt: '2026-09-02T00:00:00Z',
  } as RoleRecord,
];

function renderPage() {
  return render(
    <App>
      <RolesV2 />
    </App>,
  );
}

/** 未隐藏 Popconfirm 的确认按钮（querySelector 落元素本体） */
async function popconfirmOk(): Promise<HTMLElement> {
  return waitFor(() => {
    const pop = Array.from(document.querySelectorAll('.ant-popover')).find(
      (p) => !p.className.includes('ant-popover-hidden'),
    ) as HTMLElement;
    expect(pop).not.toBeUndefined();
    const btn = pop.querySelector('.ant-btn-primary') as HTMLElement;
    expect(btn).not.toBeNull();
    return btn;
  });
}

/** ModalForm 主提交按钮（双字中文自动插空格 → 正则收窄） */
function footerOk() {
  const footer = document.querySelector('.ant-modal-footer') as HTMLElement;
  return within(footer).getByRole('button', { name: /确\s*定/ });
}

async function waitList() {
  await waitFor(() => expect(mList).toHaveBeenCalledTimes(1));
}

beforeEach(() => {
  jest.clearAllMocks();
  mList.mockResolvedValue({ items: ROLES, total: 3, page: 1, pageSize: 10 });
  mCreate.mockResolvedValue({ ...ROLES[0], id: 7, name: 'new-role' });
  mUpdate.mockResolvedValue(ROLES[0]);
  mDel.mockResolvedValue(undefined);
  mPerms.mockResolvedValue(undefined);
});

describe('角色管理 挂载与列表', () => {
  it('首拉 {page:1,pageSize:10} + 卡片/列头/showTotal 共 3 条 + 操作列三按钮', async () => {
    renderPage();
    await waitFor(() => expect(mList).toHaveBeenCalledWith({ page: 1, pageSize: 10 }));

    expect(screen.getByText('角色管理')).toBeInTheDocument();
    expect(await screen.findByText('共 3 条')).toBeInTheDocument();
    // 列头断言收窄到 thead——「权限」与三行行内按钮同文本
    const thead = document.querySelector('.ant-table-thead') as HTMLElement;
    expect(within(thead).getByText('名称')).toBeInTheDocument();
    expect(within(thead).getByText('描述')).toBeInTheDocument();
    expect(within(thead).getByText('权限')).toBeInTheDocument();
    expect(within(thead).getByText('操作')).toBeInTheDocument();

    // 三行 × (编辑/权限/删除)
    expect(screen.getAllByRole('button', { name: /编\s*辑/ })).toHaveLength(3);
    expect(screen.getAllByRole('button', { name: /权\s*限/ })).toHaveLength(3);
    expect(screen.getAllByRole('button', { name: /删\s*除/ })).toHaveLength(3);
  });

  it('权限列 slice(0,6)：8 权限只上 6 Tag；缺字段行空（(arr||[]) 右翼）', async () => {
    renderPage();
    await waitList();
    await waitFor(() => expect(screen.getByText('p1')).toBeInTheDocument());

    const table = document.querySelector('.ant-table') as HTMLElement;
    expect(within(table).getByText('p6')).toBeInTheDocument();
    expect(within(table).queryByText('p7')).not.toBeInTheDocument();
    expect(within(table).queryByText('p8')).not.toBeInTheDocument();
    // 行 2 空权限、行 3 缺字段——两行都无 Tag
    expect(within(table).queryByText('ghost')).toBeInTheDocument();
  });

  it('缺 total → 回落 items 长度（共 2 条）；{} → 空表（total=0 不渲染分页）', async () => {
    mList.mockResolvedValueOnce({
      items: ROLES.slice(0, 2),
      page: 1,
      pageSize: 10,
    } as never);
    // unmount 取自首渲——中途再渲一次会多吃一个 mockResolvedValueOnce 队列
    const { unmount } = renderPage();
    await waitList();
    expect(await screen.findByText('共 2 条')).toBeInTheDocument();

    // 卸载重渲染（React 18 createRoot + RTL 自动 cleanup 在 afterEach，
    // 同用例内显式 unmount 后再渲一次走 {} 右翼）
    unmount();
    mList.mockResolvedValueOnce({} as never);
    renderPage();
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(2));
    // total=0 时 antd Table 不渲染分页（showTotal 随之不可见）——空态锚
    // .ant-empty-description（与 Empty 图标 svg 内 <title>No data</title> 收窄）
    expect(
      await screen.findByText('No data', { selector: '.ant-empty-description' }),
    ).toBeInTheDocument();
    expect(screen.queryByText('admin')).not.toBeInTheDocument();
  });

  it('分页 onChange → setPage + refresh(nextPage, nextSize) 参数透传', async () => {
    mList.mockResolvedValue({ items: ROLES, total: 25, page: 1, pageSize: 10 });
    renderPage();
    await waitList();

    // antd6 Pager：<li title="2" onClick><a rel="nofollow">2</a></li>——
    // a 无 href 无 button/link role，点击落 li 本体（onClick 挂 li）
    const page2 = document.querySelector('.ant-pagination-item-2') as HTMLElement;
    expect(page2).not.toBeNull();
    fireEvent.click(page2);
    await waitFor(() => expect(mList).toHaveBeenLastCalledWith({ page: 2, pageSize: 10 }));
  });
});

describe('角色管理 新增与编辑', () => {
  it('新增主链：键入值落 store → createRole 载荷 + toast 已创建 #7 + 重拉 + 关', async () => {
    renderPage();
    await waitList();

    fireEvent.click(screen.getByRole('button', { name: /新增角色/ }));
    // 弹窗标题与工具栏「新增角色」按钮同文本——锚 .ant-modal-title 收窄
    await waitFor(() => {
      expect(document.querySelector('.ant-modal-title')).toHaveTextContent('新增角色');
    });

    fireEvent.change(screen.getByLabelText('名称'), { target: { value: 'new-role' } });
    fireEvent.change(screen.getByLabelText('描述'), { target: { value: 'a brand new role' } });
    fireEvent.click(footerOk());

    await waitFor(() =>
      expect(mCreate).toHaveBeenCalledWith({
        name: 'new-role',
        description: 'a brand new role',
        permissions: [],
      }),
    );
    expect(mockMessageApi.success).toHaveBeenCalledWith('已创建 #7');
    // 成功 → destroyOnHidden 卸载（.ant-modal 整体消失）+ 重拉
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(document.querySelector('.ant-modal')).toBeNull());
  });

  it('required 拦截：空名提交 → 请输入名称，不触达 createRole、弹窗保持', async () => {
    renderPage();
    await waitList();

    fireEvent.click(screen.getByRole('button', { name: /新增角色/ }));
    // 弹窗标题与工具栏「新增角色」按钮同文本——锚 .ant-modal-title 收窄
    await waitFor(() => {
      expect(document.querySelector('.ant-modal-title')).toHaveTextContent('新增角色');
    });
    fireEvent.click(footerOk());

    expect(await screen.findByText('请输入名称')).toBeInTheDocument();
    expect(mCreate).not.toHaveBeenCalled();
    expect(document.querySelector('.ant-modal')).not.toBeNull();
  });

  it('编辑主链：回填显示 + 改值落 store → updateRole 载荷 + toast 已更新 + 重拉 + 关', async () => {
    renderPage();
    await waitList();

    // admin 行的编辑按钮（三行同名按钮，按行锚定）
    const row = (await screen.findByText('admin')).closest('tr') as HTMLElement;
    fireEvent.click(within(row).getByRole('button', { name: /编\s*辑/ }));

    expect(await screen.findByText('编辑角色')).toBeInTheDocument();
    // 回填（控制态输入显示初始值——脱管时恒为空串，此断言即回归锁）
    const nameInput = screen.getByLabelText('名称') as HTMLInputElement;
    expect(nameInput.value).toBe('admin');
    expect((screen.getByLabelText('描述') as HTMLInputElement).value).toBe('超级管理员');

    fireEvent.change(nameInput, { target: { value: 'root' } });
    fireEvent.click(footerOk());

    await waitFor(() =>
      expect(mUpdate).toHaveBeenCalledWith(1, { name: 'root', description: '超级管理员' }),
    );
    expect(mockMessageApi.success).toHaveBeenCalledWith('已更新');
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(document.querySelector('.ant-modal')).toBeNull());
  });

  it('编辑失败：updateRole reject → return false 弹窗保持 + 无 success toast', async () => {
    renderPage();
    await waitList();

    const row = (await screen.findByText('admin')).closest('tr') as HTMLElement;
    fireEvent.click(within(row).getByRole('button', { name: /编\s*辑/ }));
    expect(await screen.findByText('编辑角色')).toBeInTheDocument();

    mUpdate.mockRejectedValueOnce(new Error('denied'));
    fireEvent.click(footerOk());

    await waitFor(() => expect(mUpdate).toHaveBeenCalledTimes(1));
    expect(document.querySelector('.ant-modal')).not.toBeNull();
    expect(mockMessageApi.success).not.toHaveBeenCalled();
  });

  it('新增失败：createRole reject → return false 弹窗保持 + 无 success toast', async () => {
    renderPage();
    await waitList();

    fireEvent.click(screen.getByRole('button', { name: /新增角色/ }));
    // 弹窗标题与工具栏「新增角色」按钮同文本——锚 .ant-modal-title 收窄
    await waitFor(() => {
      expect(document.querySelector('.ant-modal-title')).toHaveTextContent('新增角色');
    });
    fireEvent.change(screen.getByLabelText('名称'), { target: { value: 'x' } });

    mCreate.mockRejectedValueOnce(new Error('boom'));
    fireEvent.click(footerOk());

    await waitFor(() => expect(mCreate).toHaveBeenCalledTimes(1));
    // description 未填：表单缺省键不存在 → payload 直传 undefined（边界 4 左态）
    expect(mCreate).toHaveBeenCalledWith({
      name: 'x',
      description: undefined,
      permissions: [],
    });
    expect(document.querySelector('.ant-modal')).not.toBeNull();
    expect(mockMessageApi.success).not.toHaveBeenCalled();
  });
});

describe('角色管理 权限编辑', () => {
  it('权限主链：标题内插 + 回显 + 追加 tag → updateRolePermissions 载荷 + toast + 重拉', async () => {
    renderPage();
    await waitList();

    const row = (await screen.findByText('admin')).closest('tr') as HTMLElement;
    fireEvent.click(within(row).getByRole('button', { name: /权\s*限/ }));

    expect(await screen.findByText('编辑权限：admin')).toBeInTheDocument();
    // initialValues 回显（tags Select 选中项按根 textContent 聚合断言）。
    // Select 锚须限 .ant-modal——分页 showSizeChanger 的 Select 在主内容区、
    // DOM 序先于 portal 弹窗（首轮实证打错到 page-size 选择器）
    await waitFor(() => {
      const select = document.querySelector('.ant-modal .ant-select') as HTMLElement;
      expect(select).not.toBeNull();
      expect(select.textContent).toContain('p1');
    });

    // 追加一个 tag：Enter 提交后须等 token 落 store（rc-select 分词宏任务）
    const select = document.querySelector('.ant-modal .ant-select') as HTMLElement;
    const input = select.querySelector('input') as HTMLInputElement;
    fireEvent.mouseDown(select);
    fireEvent.change(input, { target: { value: 'p-new' } });
    fireEvent.keyDown(input, { key: 'Enter', code: 'Enter', keyCode: 13 });
    await waitFor(() => expect(select.textContent).toContain('p-new'));

    fireEvent.click(footerOk());
    await waitFor(() =>
      expect(mPerms).toHaveBeenCalledWith(1, expect.arrayContaining(['p1', 'p-new'])),
    );
    expect(mockMessageApi.success).toHaveBeenCalledWith('权限已更新');
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(document.querySelector('.ant-modal')).toBeNull());
  });

  it('权限失败：updateRolePermissions reject → return false 弹窗保持 + 无 success', async () => {
    renderPage();
    await waitList();

    const row = (await screen.findByText('admin')).closest('tr') as HTMLElement;
    fireEvent.click(within(row).getByRole('button', { name: /权\s*限/ }));
    expect(await screen.findByText('编辑权限：admin')).toBeInTheDocument();

    mPerms.mockRejectedValueOnce(new Error('denied'));
    fireEvent.click(footerOk());

    await waitFor(() => expect(mPerms).toHaveBeenCalledTimes(1));
    expect(document.querySelector('.ant-modal')).not.toBeNull();
    expect(mockMessageApi.success).not.toHaveBeenCalled();
  });
});

describe('角色管理 删除', () => {
  it('Popconfirm 确认 → deleteRole(id) + toast 已删除 + 重拉', async () => {
    renderPage();
    await waitList();

    const row = (await screen.findByText('ops')).closest('tr') as HTMLElement;
    fireEvent.click(within(row).getByRole('button', { name: /删\s*除/ }));
    const ok = await popconfirmOk();
    fireEvent.click(ok);

    await waitFor(() => expect(mDel).toHaveBeenCalledWith(2));
    expect(mockMessageApi.success).toHaveBeenCalledWith('已删除');
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(2));
  });
});
