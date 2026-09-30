/**
 * 版本发布页单测（覆盖率巡检：Dev/Releases/index.tsx 576 行 0% → 收口，
 * 零测试页排行首位）。
 *
 * 锁定契约：
 * - 初始加载与表格矩阵：渠道/平台 Tag（label 查表 + 未知平台回退原文）、
 *   类型查表 + 未知回退、状态 Tag（label 查表 + 未知回退原文）、灰度列
 *   三臂（gray 态 strong `${n}%` / 非 gray n>0 纯文本 / 其余 '-'）、
 *   资源包列（objectKey 有值 → formatSize 三段位 B 缺省 '-'/KB/MB +
 *   title=checksum / 无值 → 未上传）、首拉载荷（status/platform 空串 +
 *   page/pageSize）；
 * - 工具栏双筛选下拉（选项来自 label 表）：status/platform 精确值进
 *   request、clear → onChange(undefined) → `v || ''` 右翼复位 + 回第 1 页；
 * - 状态机操作矩阵（canManage）：draft 传包（Upload customRequest →
 *   uploadReleaseArtifact(id, file) → onSuccess + 「资源包已上传」+ 重拉）；
 *   uploading 内测（Popconfirm → transition(id,'testing')）；testing
 *   开始灰度（开灰度弹窗）；gray 放量（弹窗 min=当前灰度、值取
 *   max(grayPercent,10)）+ 全量（Popconfirm → 'full'）+ 废弃；full 回滚
 *   （Popconfirm danger）；draft/uploading/testing/gray 废弃、archived/
 *   rolled_back/未知态无操作；
 * - 灰度弹窗：标题 `灰度放量：{version}（当前 {n}%）` + 提示文案 + Slider
 *   （step 5，键盘 ArrowRight → onChange）+ 确认放量 → transition(id,
 *   'gray', value) + 关闭 + 重拉；取消 → 关闭不触达；确认后 `!grayTarget`
 *   守卫外的 doTransition 失败两翼（Error.message / 非 Error「操作失败」）；
 * - 创建版本：ModalForm（version/platform required 拦截、channel
 *   initialValue official、type initialValues full、platform/type Select）、
 *   载荷 `{...v, gameId: ''}`（gameId 走 X-Game-ID header 契约）、
 *   「版本已创建（草稿）」+ 重拉 + 关闭；失败两翼（Error.message /
 *   非 Error「创建失败」）弹窗保持；
 * - load 失败两翼（Error.message / 非 Error「加载版本列表失败」→ 空表）、
 *   响应缺省（items/total `||` 右翼 → 空表）、刷新按钮重拉；
 * - canManage false：操作列全 '-'、无创建/传包/状态机按钮。
 *
 * mock 口径：services/api/releases 四函数 + 三张 label/color 表 jest.mock
 * （真实模块顶层 getIntl，mock 面更小更稳）；@umijs/max 本地 mock
 * （defaultMessage 即文案 + useAccess 可控）；ProTable/Upload/antd/
 * pro-components/extractErrorMessage 走真实实现。
 *
 * 登记不可达（防御分支，不造假用例不删分支）：
 * - request 回调 `statusFilter ?? ''`/`platformFilter ?? ''` 右翼：params
 *   键由 useState 恒为 string（'' 或选中值），永非 null/undefined；
 * - 确认放量 onClick 的 `if (!grayTarget) return` 守卫：按钮仅在
 *   grayTarget 态（弹窗 open）渲染，闭包捕获恒非空。
 *
 * 坑实证（antd6 沿用）：Popconfirm 确认按钮锚未隐藏 .ant-popover 内
 * .ant-btn-primary（本页 zh 文案，但锚类名两态通吃）；ModalForm 提交锚
 * .ant-modal-footer .ant-btn-primary（submitText 双字中文插空格「创 建」）；
 * 灰度弹窗取消是显式文案按钮（name=/取/）；带图标工具按钮 accessible
 * name 前缀拼 icon aria-label（「cloud-upload 传包」），role 查询用正则；
 * 工具栏双 Select 按 DOM 序 [status, platform]，mouseDown 根 + 点可见
 * option content；上传走 Upload 隐藏 input[type=file] 直接触发 change。
 */
import React from 'react';
import { configure, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { App } from 'antd';
import DevReleasesPage from '../index';
import type { Release } from '@/services/api/releases';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

jest.mock('@/services/api/releases', () => ({
  listReleases: jest.fn(),
  createRelease: jest.fn(),
  transitionRelease: jest.fn(),
  uploadReleaseArtifact: jest.fn(),
  releaseStatusLabels: {
    draft: '草稿',
    uploading: '待验证',
    testing: '内测',
    gray: '灰度',
    full: '全量',
    archived: '已归档',
    rolled_back: '已回滚',
  },
  releaseStatusColors: {
    draft: 'default',
    uploading: 'default',
    testing: 'purple',
    gray: 'orange',
    full: 'green',
    archived: 'default',
    rolled_back: 'red',
  },
  releaseTypeLabels: { hotfix: '热更', full: '整包', forced: '强更' },
  releasePlatformLabels: { ios: 'iOS', android: 'Android', pc: 'PC', webgl: 'WebGL' },
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
  listReleases,
  createRelease,
  transitionRelease,
  uploadReleaseArtifact,
} from '@/services/api/releases';

const mList = listReleases as jest.MockedFunction<typeof listReleases>;
const mCreate = createRelease as jest.MockedFunction<typeof createRelease>;
const mTransition = transitionRelease as jest.MockedFunction<typeof transitionRelease>;
const mUpload = uploadReleaseArtifact as jest.MockedFunction<typeof uploadReleaseArtifact>;

const mk = (over: Partial<Release> & Pick<Release, 'id' | 'version' | 'status'>): Release => ({
  gameId: 'demo',
  channel: 'official',
  platform: 'android',
  type: 'full',
  grayPercent: 0,
  createdAt: '2026-09-01T00:00:00Z',
  updatedAt: '2026-09-01T00:00:00Z',
  ...over,
});

// 覆盖翼：draft 无 objectKey → 未上传；grayPercent 0 → '-'
const rel1 = mk({ id: 1, version: '1.0.0', status: 'draft' });
// 覆盖翼：uploading + KB 段 + grayPercent 30 纯文本 + ios 查表 + 热更
const rel2 = mk({
  id: 2,
  version: '1.1.0',
  status: 'uploading',
  platform: 'ios',
  type: 'hotfix',
  objectKey: 'art/2',
  size: 2048,
  checksum: 'sha-2',
  grayPercent: 30,
});
// 覆盖翼：testing + MB 段 + 未知平台回退 + 未知类型回退
const rel3 = mk({
  id: 3,
  version: '1.2.0',
  status: 'testing',
  platform: 'switch' as never,
  type: 'weird' as never,
  objectKey: 'art/3',
  size: 2.5 * 1024 * 1024,
  checksum: 'sha-3',
});
// 覆盖翼：gray 态 strong + size 缺省 '-'（objectKey 在）+ 强更 + 放量/全量/废弃
const rel4 = mk({
  id: 4,
  version: '1.3.0',
  status: 'gray',
  platform: 'pc',
  type: 'forced',
  objectKey: 'art/4',
  size: undefined,
  checksum: 'sha-4',
  grayPercent: 40,
});
// 覆盖翼：full 态回滚；webgl 查表；无 objectKey
const rel5 = mk({ id: 5, version: '1.4.0', status: 'full', platform: 'webgl' });
// 覆盖翼：archived 无操作
const rel6 = mk({ id: 6, version: '0.9.0', status: 'archived' });
// 覆盖翼：rolled_back 红 Tag 无操作
const rel7 = mk({ id: 7, version: '0.8.0', status: 'rolled_back' });
// 覆盖翼：未知状态 label/色双回退 + grayPercent undefined → '-'
const rel8 = mk({ id: 8, version: '0.7.0', status: 'mystery', grayPercent: undefined as never });

const releases = [rel1, rel2, rel3, rel4, rel5, rel6, rel7, rel8];

beforeEach(() => {
  mockCanDevManage = true;
  jest.clearAllMocks();
  mList.mockResolvedValue({ items: releases, total: releases.length, page: 1, pageSize: 20 });
  mCreate.mockResolvedValue(releases[0] as never);
  mTransition.mockResolvedValue(releases[0] as never);
  mUpload.mockResolvedValue(releases[0] as never);
});

function renderPage() {
  return render(
    <App>
      <DevReleasesPage />
    </App>,
  );
}

/** 等首拉落定 */
async function waitLoad() {
  expect(await screen.findByText('1.0.0')).toBeInTheDocument();
  await waitFor(() =>
    expect(mList).toHaveBeenCalledWith({ status: '', platform: '', page: 1, pageSize: 20 }),
  );
}

/** 表格行（锚版本号文本） */
function rowOf(version: string) {
  return within(document.querySelector('.ant-table') as HTMLElement)
    .getByText(version)
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

describe('版本发布 初始渲染', () => {
  it('表格矩阵：平台/类型/状态查表回退 + 灰度三臂 + 资源包三段位 + 未上传', async () => {
    renderPage();
    await waitLoad();

    // 平台查表（iOS/WebGL/PC）+ 未知平台回退原文
    expect(within(rowOf('1.1.0')).getByText('iOS')).toBeInTheDocument();
    expect(within(rowOf('1.4.0')).getByText('WebGL')).toBeInTheDocument();
    expect(within(rowOf('1.3.0')).getByText('PC')).toBeInTheDocument();
    expect(within(rowOf('1.2.0')).getByText('switch')).toBeInTheDocument();

    // 类型查表（热更/整包/强更）+ 未知回退
    expect(within(rowOf('1.1.0')).getByText('热更')).toBeInTheDocument();
    expect(within(rowOf('1.0.0')).getByText('整包')).toBeInTheDocument();
    expect(within(rowOf('1.3.0')).getByText('强更')).toBeInTheDocument();
    expect(within(rowOf('1.2.0')).getByText('weird')).toBeInTheDocument();

    // 状态 Tag 色查表 + 未知状态 label/色双回退
    expect(within(rowOf('1.3.0')).getByText('灰度').closest('.ant-tag')).toHaveClass(
      'ant-tag-orange',
    );
    expect(within(rowOf('1.2.0')).getByText('内测').closest('.ant-tag')).toHaveClass(
      'ant-tag-purple',
    );
    expect(within(rowOf('1.4.0')).getByText('全量').closest('.ant-tag')).toHaveClass(
      'ant-tag-green',
    );
    expect(within(rowOf('0.8.0')).getByText('已回滚').closest('.ant-tag')).toHaveClass(
      'ant-tag-red',
    );
    expect(within(rowOf('0.7.0')).getByText('mystery')).toBeInTheDocument();

    // 灰度列三臂：gray strong / 非 gray n>0 纯文本 / 其余 '-'
    expect(within(rowOf('1.3.0')).getByText('40%').closest('strong')).not.toBeNull();
    expect(within(rowOf('1.1.0')).getByText('30%').closest('strong')).toBeNull();
    expect(within(rowOf('1.0.0')).getByText('-')).toBeInTheDocument();
    expect(within(rowOf('0.7.0')).getByText('-')).toBeInTheDocument(); // undefined > 0

    // 资源包列：KB/MB/'-' + title=checksum / 未上传
    expect(within(rowOf('1.1.0')).getByText('2.0 KB')).toHaveAttribute('title', 'sha-2');
    expect(within(rowOf('1.2.0')).getByText('2.5 MB')).toHaveAttribute('title', 'sha-3');
    expect(within(rowOf('1.3.0')).getByText('-')).toHaveAttribute('title', 'sha-4');
    expect(within(rowOf('1.0.0')).getByText('未上传')).toBeInTheDocument();

    // 更新时间列
    expect(within(rowOf('1.0.0')).getByText('2026-09-01T00:00:00Z')).toBeInTheDocument();

    // 工具栏
    expect(screen.getByRole('button', { name: /刷新/ })).toBeEnabled();
    expect(screen.getByRole('button', { name: /创建版本/ })).toBeEnabled();
  });

  it('状态机按钮矩阵：draft 传包+废弃 / uploading 内测 / testing 灰度 / gray 三键 / full 回滚 / 余无操作', async () => {
    renderPage();
    await waitLoad();

    expect(within(rowOf('1.0.0')).getByRole('button', { name: /传包/ })).toBeInTheDocument();
    expect(within(rowOf('1.0.0')).getAllByRole('button', { name: /废弃/ })).toHaveLength(1);

    expect(within(rowOf('1.1.0')).getByRole('button', { name: /内测/ })).toBeInTheDocument();
    expect(within(rowOf('1.2.0')).getByRole('button', { name: /开始灰度/ })).toBeInTheDocument();
    expect(within(rowOf('1.3.0')).getByRole('button', { name: /放量/ })).toBeInTheDocument();
    expect(within(rowOf('1.3.0')).getByRole('button', { name: '全量' })).toBeInTheDocument();
    expect(within(rowOf('1.4.0')).getByRole('button', { name: /回滚/ })).toBeInTheDocument();
    // archived/rolled_back/mystery 无任何操作
    expect(within(rowOf('0.9.0')).queryByRole('button')).toBeNull();
    expect(within(rowOf('0.8.0')).queryByRole('button')).toBeNull();
    expect(within(rowOf('0.7.0')).queryByRole('button')).toBeNull();
  });

  it('canManage false：操作列全 ' - '、无创建按钮', async () => {
    mockCanDevManage = false;
    renderPage();
    await waitLoad();

    expect(screen.queryByRole('button', { name: /创建版本/ })).not.toBeInTheDocument();
    for (const v of ['1.0.0', '1.1.0', '1.3.0', '1.4.0']) {
      expect(within(rowOf(v)).queryByRole('button')).toBeNull();
    }
    // 操作列 '-' 文案（灰度列 '-' 并存，合计 ≥ 行数）
    expect(screen.getAllByText('-').length).toBeGreaterThanOrEqual(8);
  });
});

describe('版本发布 筛选与刷新', () => {
  it('status/platform 双下拉精确进 request + clear 复位回第 1 页', async () => {
    renderPage();
    await waitLoad();

    await pickToolbarSelect(0, '内测');
    await waitFor(() =>
      expect(mList).toHaveBeenLastCalledWith({
        status: 'testing',
        platform: '',
        page: 1,
        pageSize: 20,
      }),
    );

    await pickToolbarSelect(1, 'PC');
    await waitFor(() =>
      expect(mList).toHaveBeenLastCalledWith({
        status: 'testing',
        platform: 'pc',
        page: 1,
        pageSize: 20,
      }),
    );

    // clear → onChange(undefined) → `v || ''` 右翼复位
    const selects = document.querySelectorAll('.ant-card-extra .ant-select');
    fireEvent.mouseEnter(selects[0] as HTMLElement);
    fireEvent.click((selects[0] as HTMLElement).querySelector('.ant-select-clear') as HTMLElement);
    await waitFor(() =>
      expect(mList).toHaveBeenLastCalledWith({ status: '', platform: 'pc', page: 1, pageSize: 20 }),
    );

    fireEvent.mouseEnter(selects[1] as HTMLElement);
    fireEvent.click((selects[1] as HTMLElement).querySelector('.ant-select-clear') as HTMLElement);
    await waitFor(() =>
      expect(mList).toHaveBeenLastCalledWith({ status: '', platform: '', page: 1, pageSize: 20 }),
    );
  });

  it('刷新按钮重拉；响应缺省（items/total || 右翼）空表；load 失败两翼', async () => {
    renderPage();
    await waitLoad();

    fireEvent.click(screen.getByRole('button', { name: /刷新/ }));
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(2));

    // 响应缺省 → 空表
    mList.mockResolvedValue({} as never);
    fireEvent.click(screen.getByRole('button', { name: /刷新/ }));
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(3));
    await waitFor(() =>
      expect(document.querySelector('.ant-table .ant-empty-description')).not.toBeNull(),
    );
  });

  it('load 失败两翼：Error.message 透传 / 非 Error 兜底「加载版本列表失败」', async () => {
    mList.mockRejectedValueOnce(new Error('rel-down'));
    renderPage();
    expect(await screen.findByText('rel-down')).toBeInTheDocument();
    await waitFor(() =>
      expect(document.querySelector('.ant-table .ant-empty-description')).not.toBeNull(),
    );

    mList.mockRejectedValueOnce('plain' as never);
    const second = renderPage();
    expect(await screen.findByText('加载版本列表失败')).toBeInTheDocument();
    second.unmount();
  });
});

describe('版本发布 创建', () => {
  it('required 拦截 → 默认值全量载荷（gameId 空串契约）→ 已创建 + 重拉 + 关闭', async () => {
    renderPage();
    await waitLoad();

    fireEvent.click(screen.getByRole('button', { name: /创建版本/ }));
    await waitFor(() =>
      expect(document.querySelector('.ant-modal-title')?.textContent).toBe('创建版本'),
    );

    // 空提交：version + platform 双 required 拦截
    const submit = document.querySelector('.ant-modal-footer .ant-btn-primary') as HTMLElement;
    fireEvent.click(submit);
    await waitFor(() =>
      expect(document.querySelector('.ant-form-item-explain-error')).not.toBeNull(),
    );
    expect(mCreate).not.toHaveBeenCalled();

    // 填值：version + platform 选 Android（channel/type 走 initialValue）
    fireEvent.change(screen.getByPlaceholderText('1.5.0'), { target: { value: '2.0.0' } });
    await new Promise((r) => setTimeout(r, 60));
    const modalSelects = document.querySelectorAll('.ant-modal .ant-select');
    fireEvent.mouseDown(modalSelects[0] as HTMLElement);
    const dropdown = await waitFor(() => {
      const visible = Array.from(document.querySelectorAll('.ant-select-dropdown')).find(
        (d) => !d.className.includes('ant-select-dropdown-hidden'),
      ) as HTMLElement;
      expect(visible).not.toBeUndefined();
      return visible;
    });
    fireEvent.click(
      within(dropdown).getByText('Android', { selector: '.ant-select-item-option-content' }),
    );

    fireEvent.click(submit);
    await waitFor(() => expect(mCreate).toHaveBeenCalledTimes(1));
    expect(mCreate).toHaveBeenCalledWith({
      version: '2.0.0',
      channel: 'official',
      platform: 'android',
      type: 'full',
      gameId: '',
    });
    expect(await screen.findByText('版本已创建（草稿）')).toBeInTheDocument();
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(screen.queryByPlaceholderText('1.5.0')).not.toBeInTheDocument());
  });

  it('创建失败两翼：Error.message 透传 / 非 Error 兜底「创建失败」，弹窗保持', async () => {
    renderPage();
    await waitLoad();

    fireEvent.click(screen.getByRole('button', { name: /创建版本/ }));
    await waitFor(() =>
      expect(document.querySelector('.ant-modal-title')?.textContent).toBe('创建版本'),
    );
    fireEvent.change(screen.getByPlaceholderText('1.5.0'), { target: { value: '3.0.0' } });
    await new Promise((r) => setTimeout(r, 60));
    const modalSelects = document.querySelectorAll('.ant-modal .ant-select');
    fireEvent.mouseDown(modalSelects[0] as HTMLElement);
    const dropdown = await waitFor(() => {
      const visible = Array.from(document.querySelectorAll('.ant-select-dropdown')).find(
        (d) => !d.className.includes('ant-select-dropdown-hidden'),
      ) as HTMLElement;
      expect(visible).not.toBeUndefined();
      return visible;
    });
    fireEvent.click(
      within(dropdown).getByText('iOS', { selector: '.ant-select-item-option-content' }),
    );

    const submit = document.querySelector('.ant-modal-footer .ant-btn-primary') as HTMLElement;
    mCreate.mockRejectedValueOnce(new Error('create-x'));
    fireEvent.click(submit);
    expect(await screen.findByText('create-x')).toBeInTheDocument();
    expect(screen.getByPlaceholderText('1.5.0')).toBeInTheDocument();

    mCreate.mockRejectedValueOnce('plain' as never);
    fireEvent.click(submit);
    expect(await screen.findByText('创建失败')).toBeInTheDocument();
    expect(screen.getByPlaceholderText('1.5.0')).toBeInTheDocument();
    expect(mList).toHaveBeenCalledTimes(1);
  });
});

describe('版本发布 传包', () => {
  it('Upload customRequest → uploadReleaseArtifact(id, file) → 已上传 + 重拉', async () => {
    renderPage();
    await waitLoad();

    const input = rowOf('1.0.0').querySelector('input[type="file"]') as HTMLInputElement;
    expect(input).not.toBeNull();
    const file = new File(['pkg-bytes'], '1.0.0.zip', { type: 'application/zip' });
    fireEvent.change(input, { target: { files: [file] } });

    await waitFor(() => expect(mUpload).toHaveBeenCalledWith(1, file));
    expect(await screen.findByText('资源包已上传')).toBeInTheDocument();
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(2));
  });

  it('上传失败两翼：Error.message 透传 / 非 Error 兜底「上传失败」', async () => {
    // 同一 input 二次 change 被 rc-upload 吞（value 复位去重），两翼分渲染
    mUpload.mockRejectedValueOnce(new Error('up-x'));
    const first = renderPage();
    await waitLoad();
    const input = rowOf('1.0.0').querySelector('input[type="file"]') as HTMLInputElement;
    fireEvent.change(input, {
      target: { files: [new File(['a'], 'a.zip')] },
    });
    expect(await screen.findByText('up-x')).toBeInTheDocument();
    expect(mList).toHaveBeenCalledTimes(1);
    first.unmount();

    mUpload.mockRejectedValueOnce('plain' as never);
    renderPage();
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(2));
    const input2 = rowOf('1.0.0').querySelector('input[type="file"]') as HTMLInputElement;
    fireEvent.change(input2, {
      target: { files: [new File(['b'], 'b.zip')] },
    });
    expect(await screen.findByText('上传失败')).toBeInTheDocument();
    expect(mList).toHaveBeenCalledTimes(2);
  });
});

describe('版本发布 状态流转', () => {
  it('内测/全量/回滚/废弃 四 Popconfirm 主链：transition 载荷 + 状态已更新 + 重拉', async () => {
    renderPage();
    await waitLoad();

    fireEvent.click(within(rowOf('1.1.0')).getByRole('button', { name: /内测/ }));
    expect(await screen.findByText('进入内测（仅白名单设备可获取）？')).toBeInTheDocument();
    await confirmPopover();
    await waitFor(() => expect(mTransition).toHaveBeenCalledWith(2, 'testing', undefined));
    expect(await screen.findByText('状态已更新')).toBeInTheDocument();
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(2));

    fireEvent.click(within(rowOf('1.3.0')).getByRole('button', { name: '全量' }));
    expect(await screen.findByText('直接全量发布？')).toBeInTheDocument();
    await confirmPopover();
    await waitFor(() => expect(mTransition).toHaveBeenLastCalledWith(4, 'full', undefined));
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(3));

    fireEvent.click(within(rowOf('1.4.0')).getByRole('button', { name: /回滚/ }));
    expect(await screen.findByText('回滚后客户端将取不到该版本，确认？')).toBeInTheDocument();
    await confirmPopover();
    await waitFor(() => expect(mTransition).toHaveBeenLastCalledWith(5, 'rollback', undefined));
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(4));

    fireEvent.click(within(rowOf('1.0.0')).getByRole('button', { name: /废弃/ }));
    expect(await screen.findByText('废弃该版本？')).toBeInTheDocument();
    await confirmPopover();
    await waitFor(() => expect(mTransition).toHaveBeenLastCalledWith(1, 'archive', undefined));
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(5));
  });

  it('transition 失败两翼：Error.message 透传 / 非 Error 兜底「操作失败」', async () => {
    renderPage();
    await waitLoad();

    mTransition.mockRejectedValueOnce(new Error('tr-x'));
    fireEvent.click(within(rowOf('1.1.0')).getByRole('button', { name: /内测/ }));
    expect(await screen.findByText('进入内测（仅白名单设备可获取）？')).toBeInTheDocument();
    await confirmPopover();
    expect(await screen.findByText('tr-x')).toBeInTheDocument();

    mTransition.mockRejectedValueOnce('plain' as never);
    fireEvent.click(within(rowOf('1.1.0')).getByRole('button', { name: /内测/ }));
    expect(await screen.findByText('进入内测（仅白名单设备可获取）？')).toBeInTheDocument();
    await confirmPopover();
    expect(await screen.findByText('操作失败')).toBeInTheDocument();
    expect(mList).toHaveBeenCalledTimes(1);
  });
});

describe('版本发布 灰度放量弹窗', () => {
  it('testing 开始灰度：标题 + 提示 + Slider 键盘步进 → 确认放量载荷 + 关闭 + 重拉', async () => {
    renderPage();
    await waitLoad();

    fireEvent.click(within(rowOf('1.2.0')).getByRole('button', { name: /开始灰度/ }));
    await waitFor(() =>
      expect(document.querySelector('.ant-modal-title')?.textContent).toBe(
        '灰度放量：1.2.0（当前 0%）',
      ),
    );
    expect(screen.getByText(/放量只增不减/)).toBeInTheDocument();

    // Slider 键盘 ArrowRight：10 → 15（step 5）
    const handle = document.querySelector('.ant-slider-handle') as HTMLElement;
    fireEvent.keyDown(handle, { key: 'ArrowRight', keyCode: 39, which: 39 });

    fireEvent.click(document.querySelector('.ant-modal-footer .ant-btn-primary') as HTMLElement);
    await waitFor(() => expect(mTransition).toHaveBeenCalledWith(3, 'gray', 15));
    expect(await screen.findByText('状态已更新')).toBeInTheDocument();
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(document.querySelector('.ant-modal')).toBeNull());
  });

  it('gray 放量：min=当前灰度、值取 max(grayPercent,10) → 载荷 40%', async () => {
    renderPage();
    await waitLoad();

    fireEvent.click(within(rowOf('1.3.0')).getByRole('button', { name: /放量/ }));
    await waitFor(() =>
      expect(document.querySelector('.ant-modal-title')?.textContent).toBe(
        '灰度放量：1.3.0（当前 40%）',
      ),
    );
    fireEvent.click(document.querySelector('.ant-modal-footer .ant-btn-primary') as HTMLElement);
    await waitFor(() => expect(mTransition).toHaveBeenCalledWith(4, 'gray', 40));
    await waitFor(() => expect(document.querySelector('.ant-modal')).toBeNull());
  });

  it('取消：footer 取消与右上 X 双关流，不触达 transition', async () => {
    renderPage();
    await waitLoad();

    fireEvent.click(within(rowOf('1.2.0')).getByRole('button', { name: /开始灰度/ }));
    await waitFor(() =>
      expect(document.querySelector('.ant-modal-title')?.textContent).toBe(
        '灰度放量：1.2.0（当前 0%）',
      ),
    );
    fireEvent.click(screen.getByRole('button', { name: /取/ }));
    await waitFor(() => expect(document.querySelector('.ant-modal')).toBeNull());
    expect(mTransition).not.toHaveBeenCalled();

    // 重开 → 右上 X：onCancel 关流
    fireEvent.click(within(rowOf('1.2.0')).getByRole('button', { name: /开始灰度/ }));
    await waitFor(() =>
      expect(document.querySelector('.ant-modal-title')?.textContent).toBe(
        '灰度放量：1.2.0（当前 0%）',
      ),
    );
    fireEvent.click(document.querySelector('.ant-modal-close') as HTMLElement);
    await waitFor(() => expect(document.querySelector('.ant-modal')).toBeNull());
    expect(mTransition).not.toHaveBeenCalled();
  });
});
