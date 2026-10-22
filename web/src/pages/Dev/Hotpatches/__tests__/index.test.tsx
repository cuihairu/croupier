/**
 * 服务端热更新页单测（覆盖率巡检：Dev/Hotpatches/index.tsx 554 行 0% →
 * 收口，零测试页排行次席；骨架与 Dev/Releases 同族，坑档互通）。
 *
 * 锁定契约：
 * - 初始加载与表格矩阵：框架查表（skynet/KBEngine/JVM/Node.js/自定义）
 *   + 未知框架回退原文、关联缺陷 `#id`、状态 Tag（label/色查表 + 未知
 *   状态双回退）、灰度列三臂（rolling strong `${n}%` / 非 rolling n>0
 *   纯文本 / 其余 '-'）、补丁包列（packageKey 有值 → formatSize 三段位
 *   （size 缺省 '-'/KB/MB）/ 无值未上传）、首拉载荷（status/framework
 *   空串 + page/pageSize）；
 * - 状态机操作矩阵（canManage）：draft 无包仅传包（提交审批两条件臂
 *   false 侧）/ draft 有包传包+提交审批（Popconfirm 双人规则文案）、
 *   approved 开始灰度、rolling 放量+标记生效+回滚、failed 回滚、
 *   applied/rolled_back/未知态无操作；
 * - 灰度放量弹窗：标题 `节点灰度放量（当前 ${n}%）` + 提示文案 + Slider
 *   （step 5 键盘 ArrowRight → onChange；rolling 放量 min=当前值取
 *   max(rolloutPercent,10)）+ 确认放量 → transition(id,'roll',value) +
 *   关闭 + 重拉；footer 取消与右上 X 双关流不触达；
 * - 传包 Upload customRequest → uploadHotpatchPackage(id, file) →
 *   onSuccess + 「补丁包已上传（SHA-256 已登记）」+ 重拉；失败两翼；
 * - 创建热更单：ModalForm（title/bugId required 拦截（framework 默认
 *   skynet 过校验）、framework Select 切换自定义、InputNumber 数值）、
 *   载荷 `{...v, gameId: ''}`（gameId 走 X-Game-ID header 契约）、
 *   「热更单已创建（草稿），请上传补丁包」+ 重拉 + 关闭；失败两翼弹窗保持；
 * - 工具栏双筛选下拉（状态/框架）：值进 request + clear 复位 `v || ''`
 *   右翼 + 回第 1 页；load 失败两翼（Error.message / 非 Error「加载热更单
 *   失败」）、响应缺省（items/total || 右翼）空表、刷新重拉；
 * - canManage false：操作列全 '-'、无创建/状态机按钮。
 *
 * mock 口径：services/api/hotpatches 四函数 + 三张 label/color 表
 * jest.mock（真实模块顶层 getIntl）；@umijs/max 本地 mock（defaultMessage
 * 即文案 + useAccess 可控）；ProTable/Upload/antd/pro-components/
 * extractErrorMessage 走真实实现。
 *
 * 登记不可达（防御分支，不造假用例不删分支）：
 * - request 回调 `statusFilter ?? ''`/`fw ?? ''` 右翼：params 键由
 *   useState 恒为 string（'' 或选中值），永非 null/undefined；
 * - 确认放量 onClick 的 `if (!rollTarget) return` 守卫：按钮仅在
 *   rollTarget 态（弹窗 open）渲染，闭包捕获恒非空。
 *
 * 坑实证（antd6 沿用，同 Releases 坑档）：Upload 同一 input 二次 change
 * 被 rc-upload 吞（value 复位去重），失败两翼须分渲染各自触发；
 * Popconfirm 确认锚未隐藏 .ant-popover 内 .ant-btn-primary（类名两态
 * 通吃）；ModalForm 提交锚 .ant-modal-footer .ant-btn-primary（双字中文
 * 插空格「创 建」）；带图标按钮 accessible name 前缀拼 icon aria-label
 * （「cloud-upload 传包」），role 查询用正则。
 */
import React from 'react';
import { configure, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { App } from 'antd';
import DevHotpatchesPage from '../index';
import type { HotpatchItem } from '@/services/api/hotpatches';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

jest.mock('@/services/api/hotpatches', () => ({
  listHotpatches: jest.fn(),
  createHotpatch: jest.fn(),
  transitionHotpatch: jest.fn(),
  uploadHotpatchPackage: jest.fn(),
  hotpatchStatusLabels: {
    draft: '草稿',
    approved: '已审批',
    rolling: '灰度中',
    applied: '已生效',
    failed: '失败',
    rolled_back: '已回滚',
  },
  hotpatchStatusColors: {
    draft: 'default',
    approved: 'purple',
    rolling: 'orange',
    applied: 'green',
    failed: 'red',
    rolled_back: 'red',
  },
  hotpatchFrameworkLabels: {
    skynet: 'skynet (Lua)',
    kbengine: 'KBEngine (Python)',
    jvm: 'JVM (Java)',
    nodejs: 'Node.js (JS/TS)',
    custom: '自定义',
  },
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

import {
  listHotpatches,
  createHotpatch,
  transitionHotpatch,
  uploadHotpatchPackage,
} from '@/services/api/hotpatches';

const mList = listHotpatches as jest.MockedFunction<typeof listHotpatches>;
const mCreate = createHotpatch as jest.MockedFunction<typeof createHotpatch>;
const mTransition = transitionHotpatch as jest.MockedFunction<typeof transitionHotpatch>;
const mUpload = uploadHotpatchPackage as jest.MockedFunction<typeof uploadHotpatchPackage>;

const mk = (
  over: Partial<HotpatchItem> & Pick<HotpatchItem, 'id' | 'bugId' | 'status'>,
): HotpatchItem => ({
  framework: 'skynet',
  rolloutPercent: 0,
  createdAt: '2026-09-01T00:00:00Z',
  updatedAt: '2026-09-01T00:00:00Z',
  ...over,
});

// 覆盖翼：draft 无 packageKey → 仅传包（提交审批两条件臂 false 侧）+ 未上传
const hp1 = mk({ id: 1, bugId: 101, status: 'draft' });
// 覆盖翼：draft 有包 → 传包+提交审批；KB 段；rollout 30 纯文本
const hp2 = mk({
  id: 2,
  bugId: 102,
  status: 'draft',
  framework: 'kbengine',
  packageKey: 'pkg/2',
  size: 2048,
  rolloutPercent: 30,
});
// 覆盖翼：approved 开始灰度；MB 段
const hp3 = mk({
  id: 3,
  bugId: 103,
  status: 'approved',
  framework: 'jvm',
  packageKey: 'pkg/3',
  size: 2.5 * 1024 * 1024,
});
// 覆盖翼：rolling strong + size 缺省 '-'（packageKey 在）+ 放量/标记生效/回滚
const hp4 = mk({
  id: 4,
  bugId: 104,
  status: 'rolling',
  framework: 'nodejs',
  packageKey: 'pkg/4',
  size: undefined,
  rolloutPercent: 40,
});
// 覆盖翼：failed 仅回滚
const hp5 = mk({
  id: 5,
  bugId: 105,
  status: 'failed',
  packageKey: 'pkg/5',
  size: 512,
});
// 覆盖翼：applied 无操作；custom 查表
const hp6 = mk({ id: 6, bugId: 106, status: 'applied', framework: 'custom' });
// 覆盖翼：rolled_back 无操作；未知框架回退原文
const hp7 = mk({ id: 7, bugId: 107, status: 'rolled_back', framework: 'weird' });
// 覆盖翼：未知状态 label/色双回退 + rolloutPercent undefined → '-'
const hp8 = mk({ id: 8, bugId: 108, status: 'mystery', rolloutPercent: undefined as never });

const items = [hp1, hp2, hp3, hp4, hp5, hp6, hp7, hp8];

beforeEach(() => {
  mockCanDevManage = true;
  jest.clearAllMocks();
  mList.mockResolvedValue({ items, total: items.length, page: 1, pageSize: 20 });
  mCreate.mockResolvedValue(items[0] as never);
  mTransition.mockResolvedValue(items[0] as never);
  mUpload.mockResolvedValue(items[0] as never);
});

function renderPage() {
  return render(
    <App>
      <DevHotpatchesPage />
    </App>,
  );
}

/** 等首拉落定（锚关联缺陷列 #id） */
async function waitLoad() {
  expect(await screen.findByText('#101')).toBeInTheDocument();
  await waitFor(() =>
    expect(mList).toHaveBeenCalledWith({ status: '', framework: '', page: 1, pageSize: 20 }),
  );
}

/** 表格行（锚关联缺陷 #id 文本） */
function rowOf(bug: string) {
  return within(document.querySelector('.ant-table') as HTMLElement)
    .getByText(bug)
    .closest('tr') as HTMLElement;
}

/** Popconfirm 确认（未隐藏 popover 内主按钮，锚类名两态通吃） */
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

/** 工具栏第 idx 个 Select 选 option（antd6：mouseDown 根 + 点可见 option content） */
async function pickToolbarSelect(idx: number, label: string) {
  await new Promise((r) => setTimeout(r, 60));
  const selects = document.querySelectorAll('.ant-card-extra .ant-select');
  fireEvent.mouseDown(selects[idx] as HTMLElement);
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

describe('服务端热更新 初始渲染', () => {
  it('表格矩阵：框架/状态查表回退 + 灰度三臂 + 补丁包三段位 + 未上传', async () => {
    renderPage();
    await waitLoad();

    // 框架查表 + 未知回退
    expect(within(rowOf('#101')).getByText('skynet (Lua)')).toBeInTheDocument();
    expect(within(rowOf('#102')).getByText('KBEngine (Python)')).toBeInTheDocument();
    expect(within(rowOf('#103')).getByText('JVM (Java)')).toBeInTheDocument();
    expect(within(rowOf('#104')).getByText('Node.js (JS/TS)')).toBeInTheDocument();
    expect(within(rowOf('#106')).getByText('自定义')).toBeInTheDocument();
    expect(within(rowOf('#107')).getByText('weird')).toBeInTheDocument();

    // 状态 Tag 色查表 + 未知状态 label/色双回退
    expect(within(rowOf('#103')).getByText('已审批').closest('.ant-tag')).toHaveClass(
      'ant-tag-purple',
    );
    expect(within(rowOf('#104')).getByText('灰度中').closest('.ant-tag')).toHaveClass(
      'ant-tag-orange',
    );
    expect(within(rowOf('#106')).getByText('已生效').closest('.ant-tag')).toHaveClass(
      'ant-tag-green',
    );
    expect(within(rowOf('#105')).getByText('失败').closest('.ant-tag')).toHaveClass('ant-tag-red');
    expect(within(rowOf('#107')).getByText('已回滚').closest('.ant-tag')).toHaveClass(
      'ant-tag-red',
    );
    expect(within(rowOf('#108')).getByText('mystery')).toBeInTheDocument();

    // 灰度列三臂
    expect(within(rowOf('#104')).getByText('40%').closest('strong')).not.toBeNull();
    expect(within(rowOf('#102')).getByText('30%').closest('strong')).toBeNull();
    expect(within(rowOf('#101')).getByText('-')).toBeInTheDocument();
    expect(within(rowOf('#108')).getByText('-')).toBeInTheDocument(); // undefined > 0

    // 补丁包列三段位 + 未上传
    expect(within(rowOf('#102')).getByText('2.0 KB')).toBeInTheDocument();
    expect(within(rowOf('#103')).getByText('2.5 MB')).toBeInTheDocument();
    expect(within(rowOf('#104')).getByText('-')).toBeInTheDocument(); // size 缺省
    expect(within(rowOf('#105')).getByText('0.5 KB')).toBeInTheDocument();
    expect(within(rowOf('#101')).getByText('未上传')).toBeInTheDocument();

    // 工具栏
    expect(screen.getByRole('button', { name: /刷新/ })).toBeEnabled();
    expect(screen.getByRole('button', { name: /创建热更单/ })).toBeEnabled();
  });

  it('状态机按钮矩阵：draft 双态 / approved 灰度 / rolling 三键 / failed 回滚 / 余无操作', async () => {
    renderPage();
    await waitLoad();

    // draft 无包：仅传包，无提交审批
    expect(within(rowOf('#101')).getByRole('button', { name: /传包/ })).toBeInTheDocument();
    expect(within(rowOf('#101')).queryByRole('button', { name: /提交审批/ })).toBeNull();
    // draft 有包：传包 + 提交审批
    expect(within(rowOf('#102')).getByRole('button', { name: /传包/ })).toBeInTheDocument();
    expect(within(rowOf('#102')).getByRole('button', { name: /提交审批/ })).toBeInTheDocument();

    expect(within(rowOf('#103')).getByRole('button', { name: /开始灰度/ })).toBeInTheDocument();
    expect(within(rowOf('#104')).getByRole('button', { name: /放量/ })).toBeInTheDocument();
    expect(within(rowOf('#104')).getByRole('button', { name: /标记生效/ })).toBeInTheDocument();
    expect(within(rowOf('#104')).getByRole('button', { name: /回滚/ })).toBeInTheDocument();
    expect(within(rowOf('#105')).getByRole('button', { name: /回滚/ })).toBeInTheDocument();
    // applied/rolled_back/mystery 无任何操作
    expect(within(rowOf('#106')).queryByRole('button')).toBeNull();
    expect(within(rowOf('#107')).queryByRole('button')).toBeNull();
    expect(within(rowOf('#108')).queryByRole('button')).toBeNull();
  });

  it('canManage false：操作列全 ' - '、无创建按钮', async () => {
    mockCanDevManage = false;
    renderPage();
    await waitLoad();

    expect(screen.queryByRole('button', { name: /创建热更单/ })).not.toBeInTheDocument();
    for (const bug of ['#101', '#102', '#104', '#105']) {
      expect(within(rowOf(bug)).queryByRole('button')).toBeNull();
    }
    expect(screen.getAllByText('-').length).toBeGreaterThanOrEqual(8);
  });
});

describe('服务端热更新 筛选与刷新', () => {
  it('状态/框架双下拉精确进 request + clear 复位回第 1 页', async () => {
    renderPage();
    await waitLoad();

    await pickToolbarSelect(0, '灰度中');
    await waitFor(() =>
      expect(mList).toHaveBeenLastCalledWith({
        status: 'rolling',
        framework: '',
        page: 1,
        pageSize: 20,
      }),
    );

    await pickToolbarSelect(1, 'JVM (Java)');
    await waitFor(() =>
      expect(mList).toHaveBeenLastCalledWith({
        status: 'rolling',
        framework: 'jvm',
        page: 1,
        pageSize: 20,
      }),
    );

    // clear → onChange(undefined) → `v || ''` 右翼复位
    const selects = document.querySelectorAll('.ant-card-extra .ant-select');
    fireEvent.mouseEnter(selects[0] as HTMLElement);
    fireEvent.click((selects[0] as HTMLElement).querySelector('.ant-select-clear') as HTMLElement);
    await waitFor(() =>
      expect(mList).toHaveBeenLastCalledWith({
        status: '',
        framework: 'jvm',
        page: 1,
        pageSize: 20,
      }),
    );

    fireEvent.mouseEnter(selects[1] as HTMLElement);
    fireEvent.click((selects[1] as HTMLElement).querySelector('.ant-select-clear') as HTMLElement);
    await waitFor(() =>
      expect(mList).toHaveBeenLastCalledWith({ status: '', framework: '', page: 1, pageSize: 20 }),
    );
  });

  it('刷新重拉；响应缺省（items/total || 右翼）空表；load 失败两翼', async () => {
    renderPage();
    await waitLoad();

    fireEvent.click(screen.getByRole('button', { name: /刷新/ }));
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(2));

    mList.mockResolvedValue({} as never);
    fireEvent.click(screen.getByRole('button', { name: /刷新/ }));
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(3));
    await waitFor(() =>
      expect(document.querySelector('.ant-table .ant-empty-description')).not.toBeNull(),
    );

    mList.mockRejectedValueOnce(new Error('hp-down'));
    const second = renderPage();
    expect(await screen.findByText('hp-down')).toBeInTheDocument();
    second.unmount();

    mList.mockRejectedValueOnce('plain' as never);
    renderPage();
    expect(await screen.findByText('加载热更单失败')).toBeInTheDocument();
  });
});

describe('服务端热更新 创建', () => {
  it('required 拦截 → 默认值全量载荷（gameId 空串契约）→ 已创建 + 重拉 + 关闭', async () => {
    renderPage();
    await waitLoad();

    fireEvent.click(screen.getByRole('button', { name: /创建热更单/ }));
    await waitFor(() =>
      expect(document.querySelector('.ant-modal-title')?.textContent).toBe('创建热更单'),
    );

    // 空提交：title + bugId 双 required 拦截（framework 默认 skynet 过校验）
    const submit = document.querySelector('.ant-modal-footer .ant-btn-primary') as HTMLElement;
    fireEvent.click(submit);
    await waitFor(() =>
      expect(document.querySelector('.ant-form-item-explain-error')).not.toBeNull(),
    );
    expect(mCreate).not.toHaveBeenCalled();

    // 填值：title + bugId 数值 + framework 切自定义
    fireEvent.change(screen.getByPlaceholderText('如：修复背包闪退'), {
      target: { value: '修复背包闪退' },
    });
    fireEvent.change(screen.getByPlaceholderText('缺陷追踪里的 Bug ID'), {
      target: { value: '5' },
    });
    await new Promise((r) => setTimeout(r, 60));
    fireEvent.mouseDown(document.querySelector('.ant-modal .ant-select') as HTMLElement);
    const dropdown = await waitFor(() => {
      const visible = Array.from(document.querySelectorAll('.ant-select-dropdown')).find(
        (d) => !d.className.includes('ant-select-dropdown-hidden'),
      ) as HTMLElement;
      expect(visible).not.toBeUndefined();
      return visible;
    });
    fireEvent.click(
      within(dropdown).getByText('自定义', { selector: '.ant-select-item-option-content' }),
    );

    fireEvent.click(submit);
    await waitFor(() => expect(mCreate).toHaveBeenCalledTimes(1));
    expect(mCreate).toHaveBeenCalledWith({
      title: '修复背包闪退',
      bugId: 5,
      framework: 'custom',
      gameId: '',
    });
    expect(await screen.findByText('热更单已创建（草稿），请上传补丁包')).toBeInTheDocument();
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(2));
    await waitFor(() =>
      expect(screen.queryByPlaceholderText('如：修复背包闪退')).not.toBeInTheDocument(),
    );
  });

  it('创建失败两翼：Error.message 透传 / 非 Error 兜底「创建失败」，弹窗保持', async () => {
    renderPage();
    await waitLoad();

    fireEvent.click(screen.getByRole('button', { name: /创建热更单/ }));
    await waitFor(() =>
      expect(document.querySelector('.ant-modal-title')?.textContent).toBe('创建热更单'),
    );
    fireEvent.change(screen.getByPlaceholderText('如：修复背包闪退'), {
      target: { value: 'x' },
    });
    fireEvent.change(screen.getByPlaceholderText('缺陷追踪里的 Bug ID'), {
      target: { value: '7' },
    });

    const submit = document.querySelector('.ant-modal-footer .ant-btn-primary') as HTMLElement;
    mCreate.mockRejectedValueOnce(new Error('hp-create-x'));
    fireEvent.click(submit);
    expect(await screen.findByText('hp-create-x')).toBeInTheDocument();
    expect(screen.getByPlaceholderText('如：修复背包闪退')).toBeInTheDocument();

    mCreate.mockRejectedValueOnce('plain' as never);
    fireEvent.click(submit);
    expect(await screen.findByText('创建失败')).toBeInTheDocument();
    expect(screen.getByPlaceholderText('如：修复背包闪退')).toBeInTheDocument();
    expect(mList).toHaveBeenCalledTimes(1);
  });
});

describe('服务端热更新 传包', () => {
  it('Upload customRequest → uploadHotpatchPackage(id, file) → 已上传 + 重拉', async () => {
    renderPage();
    await waitLoad();

    const input = rowOf('#101').querySelector('input[type="file"]') as HTMLInputElement;
    expect(input).not.toBeNull();
    const file = new File(['patch-bytes'], 'p1.zip', { type: 'application/zip' });
    fireEvent.change(input, { target: { files: [file] } });

    await waitFor(() => expect(mUpload).toHaveBeenCalledWith(1, file));
    expect(await screen.findByText('补丁包已上传（SHA-256 已登记）')).toBeInTheDocument();
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(2));
  });

  it('上传失败两翼：Error.message 透传 / 非 Error 兜底「上传失败」', async () => {
    // 同一 input 二次 change 被 rc-upload 吞（value 复位去重），两翼分渲染
    mUpload.mockRejectedValueOnce(new Error('hp-up-x'));
    const first = renderPage();
    await waitLoad();
    const input = rowOf('#101').querySelector('input[type="file"]') as HTMLInputElement;
    fireEvent.change(input, {
      target: { files: [new File(['a'], 'a.zip')] },
    });
    expect(await screen.findByText('hp-up-x')).toBeInTheDocument();
    expect(mList).toHaveBeenCalledTimes(1);
    first.unmount();

    mUpload.mockRejectedValueOnce('plain' as never);
    renderPage();
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(2));
    const input2 = rowOf('#101').querySelector('input[type="file"]') as HTMLInputElement;
    fireEvent.change(input2, {
      target: { files: [new File(['b'], 'b.zip')] },
    });
    expect(await screen.findByText('上传失败')).toBeInTheDocument();
    expect(mList).toHaveBeenCalledTimes(2);
  });
});

describe('服务端热更新 状态流转', () => {
  it('提交审批/标记生效/回滚 主链：transition 载荷 + 状态已更新 + 重拉', async () => {
    renderPage();
    await waitLoad();

    fireEvent.click(within(rowOf('#102')).getByRole('button', { name: /提交审批/ }));
    expect(
      await screen.findByText('提交审批？（双人规则：需第二人复核后才能灰度）'),
    ).toBeInTheDocument();
    await confirmPopover();
    await waitFor(() => expect(mTransition).toHaveBeenCalledWith(2, 'approve', undefined));
    expect(await screen.findByText('状态已更新')).toBeInTheDocument();
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(2));

    // 标记生效：直连按钮（无 Popconfirm）
    fireEvent.click(within(rowOf('#104')).getByRole('button', { name: /标记生效/ }));
    await waitFor(() => expect(mTransition).toHaveBeenLastCalledWith(4, 'applied', undefined));
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(3));

    fireEvent.click(within(rowOf('#105')).getByRole('button', { name: /回滚/ }));
    expect(await screen.findByText('回滚所有已应用节点？')).toBeInTheDocument();
    await confirmPopover();
    await waitFor(() => expect(mTransition).toHaveBeenLastCalledWith(5, 'rollback', undefined));
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(4));
  });

  it('transition 失败两翼：Error.message 透传 / 非 Error 兜底「操作失败」', async () => {
    renderPage();
    await waitLoad();

    mTransition.mockRejectedValueOnce(new Error('hp-tr-x'));
    fireEvent.click(within(rowOf('#102')).getByRole('button', { name: /提交审批/ }));
    expect(
      await screen.findByText('提交审批？（双人规则：需第二人复核后才能灰度）'),
    ).toBeInTheDocument();
    await confirmPopover();
    expect(await screen.findByText('hp-tr-x')).toBeInTheDocument();

    mTransition.mockRejectedValueOnce('plain' as never);
    fireEvent.click(within(rowOf('#102')).getByRole('button', { name: /提交审批/ }));
    expect(
      await screen.findByText('提交审批？（双人规则：需第二人复核后才能灰度）'),
    ).toBeInTheDocument();
    await confirmPopover();
    expect(await screen.findByText('操作失败')).toBeInTheDocument();
    expect(mList).toHaveBeenCalledTimes(1);
  });
});

describe('服务端热更新 灰度放量弹窗', () => {
  it('approved 开始灰度：标题 + 提示 + Slider 键盘步进 → 确认放量载荷 + 关闭 + 重拉', async () => {
    renderPage();
    await waitLoad();

    fireEvent.click(within(rowOf('#103')).getByRole('button', { name: /开始灰度/ }));
    await waitFor(() =>
      expect(document.querySelector('.ant-modal-title')?.textContent).toBe(
        '节点灰度放量（当前 0%）',
      ),
    );
    expect(screen.getByText(/按节点 hash 分桶/)).toBeInTheDocument();

    // Slider 键盘 ArrowRight：10 → 15（step 5）
    const handle = document.querySelector('.ant-slider-handle') as HTMLElement;
    fireEvent.keyDown(handle, { key: 'ArrowRight', keyCode: 39, which: 39 });

    fireEvent.click(document.querySelector('.ant-modal-footer .ant-btn-primary') as HTMLElement);
    await waitFor(() => expect(mTransition).toHaveBeenCalledWith(3, 'roll', 15));
    expect(await screen.findByText('状态已更新')).toBeInTheDocument();
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(document.querySelector('.ant-modal')).toBeNull());
  });

  it('rolling 放量：min=当前灰度、值取 max(rolloutPercent,10) → 载荷 40%', async () => {
    renderPage();
    await waitLoad();

    fireEvent.click(within(rowOf('#104')).getByRole('button', { name: /放量/ }));
    await waitFor(() =>
      expect(document.querySelector('.ant-modal-title')?.textContent).toBe(
        '节点灰度放量（当前 40%）',
      ),
    );
    fireEvent.click(document.querySelector('.ant-modal-footer .ant-btn-primary') as HTMLElement);
    await waitFor(() => expect(mTransition).toHaveBeenCalledWith(4, 'roll', 40));
    await waitFor(() => expect(document.querySelector('.ant-modal')).toBeNull());
  });

  it('取消：footer 取消与右上 X 双关流，不触达 transition', async () => {
    renderPage();
    await waitLoad();

    fireEvent.click(within(rowOf('#103')).getByRole('button', { name: /开始灰度/ }));
    await waitFor(() =>
      expect(document.querySelector('.ant-modal-title')?.textContent).toBe(
        '节点灰度放量（当前 0%）',
      ),
    );
    fireEvent.click(screen.getByRole('button', { name: /取/ }));
    await waitFor(() => expect(document.querySelector('.ant-modal')).toBeNull());
    expect(mTransition).not.toHaveBeenCalled();

    // 重开 → 右上 X：onCancel 关流
    fireEvent.click(within(rowOf('#103')).getByRole('button', { name: /开始灰度/ }));
    await waitFor(() =>
      expect(document.querySelector('.ant-modal-title')?.textContent).toBe(
        '节点灰度放量（当前 0%）',
      ),
    );
    fireEvent.click(document.querySelector('.ant-modal-close') as HTMLElement);
    await waitFor(() => expect(document.querySelector('.ant-modal')).toBeNull());
    expect(mTransition).not.toHaveBeenCalled();
  });
});
