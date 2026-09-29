/**
 * 升级扩展弹窗单测（覆盖率巡检：Extensions 簇余量第二批，UpgradeModal.tsx
 * 151 行 0% → 行覆盖收口）。
 *
 * 锁定契约：打开加载链（listExtensionCatalogReleases + adapter 真实归一 +
 * options 组装 + 当前版本不在列表时前插 / 在列表时不重复 / 当前版本为空不前插
 * 的三态）、loading 收尾、空版本提交 warning 拦截、选择版本提交链
 * （upgradeExtension 载荷 + 「升级请求已提交」+ onClose + onUpgraded + 按钮
 * 退出 loading）、失败四分支文案（missing_dependency / version_mismatch /
 * dependency_cycle 含 details 缺省 unknown-兜底 / 普通错误透传 mapper message）
 * 与弹窗不关闭、open/row 守卫不发请求。
 *
 * mock 口径：services/api/extensions 仅 listExtensionCatalogReleases/
 * upgradeExtension；mapper 走真实实现；@umijs/max 本地 mock。App 包裹
 * （App.useApp message）。
 *
 * 边界（诚实）：
 * 1. 加载链 .then().finally() 无 catch——releases 接口 reject 产生 unhandled
 *    rejection（组件现状缺陷，同簇巡检既定结论），不造假 reject 用例。
 * 2. handleOk 的 `if (!row) return` 守卫经 UI 不可达（OK 按钮仅在 open 且 row
 *    已设时可点）；`version.trim()` 空白翼——Select 只产出选项值无空白，不可达。
 */
import React from 'react';
import { configure, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { App } from 'antd';
import UpgradeModal from '../UpgradeModal';
import type { ExtensionInstallationItem, ExtensionReleaseItem } from '@/services/api/extensions';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

jest.mock('@/services/api/extensions', () => ({
  listExtensionCatalogReleases: jest.fn(),
  upgradeExtension: jest.fn(),
}));

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
}));

import {
  listExtensionCatalogReleases,
  upgradeExtension,
} from '@/services/api/extensions';

const mReleases =
  listExtensionCatalogReleases as jest.MockedFunction<typeof listExtensionCatalogReleases>;
const mUpgrade = upgradeExtension as jest.MockedFunction<typeof upgradeExtension>;

const row: ExtensionInstallationItem = {
  id: 7,
  installationKey: 'chatops-demo',
  extensionId: 'chatops',
  displayName: 'ChatOps',
  releaseVersion: '1.4.0',
  scopeType: 'game',
  scopeId: 'demo',
  targetType: 'agent',
  targetId: 'agent-1',
  status: 'running',
  desiredState: 'active',
  enabled: true,
  healthStatus: 'healthy',
  lastError: '',
  updatedAt: 1727500000,
};

const releaseOf = (version: string): ExtensionReleaseItem => ({
  version,
  releaseChannel: 'stable',
  minCoreVersion: '0.0.1',
  publishedAt: 1,
  changelog: '',
});

function renderModal(over?: { open?: boolean; row?: ExtensionInstallationItem | null }) {
  const onClose = jest.fn();
  const onUpgraded = jest.fn().mockResolvedValue(undefined);
  const utils = render(
    <App>
      <UpgradeModal
        open={over?.open ?? true}
        row={over?.row !== undefined ? over.row : row}
        onClose={onClose}
        onUpgraded={onUpgraded}
      />
    </App>,
  );
  return { ...utils, onClose, onUpgraded };
}

/** 等打开加载链落定（releases 拉取完成） */
async function waitOpen() {
  await waitFor(() => expect(mReleases).toHaveBeenCalledTimes(1));
}

/** 弹窗 footer 主按钮（OK） */
function clickOk() {
  const ok = document.querySelector('.ant-modal-footer .ant-btn-primary') as HTMLButtonElement;
  expect(ok).not.toBeNull();
  fireEvent.click(ok);
}

/** 打开版本下拉并点选指定版本 */
function selectVersion(version: string) {
  fireEvent.mouseDown(document.querySelector('.ant-select') as HTMLElement);
  const dropdown = document.querySelector('.ant-select-dropdown') as HTMLElement;
  expect(dropdown).not.toBeNull();
  fireEvent.click(
    within(dropdown).getByText(version, { selector: '.ant-select-item-option-content' }),
  );
}

/** 安装接口错误 fixture（mapper 从 response.data.details 读 code/字段） */
const errWith = (details: Record<string, unknown>) => ({
  response: { data: { details } },
});

beforeEach(() => {
  jest.clearAllMocks();
  mReleases.mockResolvedValue({
    total: 2,
    releases: [releaseOf('1.4.0'), releaseOf('1.5.0')],
  });
  mUpgrade.mockResolvedValue({ installationId: 7, status: 'upgrading' } as never);
});

describe('UpgradeModal 打开加载链', () => {
  it('拉取版本列表 + 当前版本在列表中不重复前插；选择 1.5.0 提交 → 载荷 + 成功文案 + onClose + onUpgraded', async () => {
    const inst = renderModal();
    await waitOpen();
    expect(await screen.findByText('升级扩展')).toBeInTheDocument();

    // 当前版本 1.4.0 在 releases 中 → options 恰为两个版本（无重复项）
    selectVersion('1.5.0');
    clickOk();

    await waitFor(() => expect(mUpgrade).toHaveBeenCalledWith(7, '1.5.0'));
    expect(await screen.findByText('升级请求已提交')).toBeInTheDocument();
    await waitFor(() => expect(inst.onClose).toHaveBeenCalledTimes(1));
    await waitFor(() => expect(inst.onUpgraded).toHaveBeenCalledTimes(1));
    // finally 翼：OK 按钮退出 loading
    const ok = document.querySelector('.ant-modal-footer .ant-btn-primary') as HTMLElement;
    await waitFor(() => expect(ok).not.toHaveClass('ant-btn-loading'));
  });

  it('当前版本不在列表 → 前插补齐（hasCurrent=false 左翼）', async () => {
    mReleases.mockResolvedValue({ total: 1, releases: [releaseOf('2.0.0')] });
    renderModal({ row: { ...row, releaseVersion: '0.8.0' } });
    await waitOpen();

    // options = [0.8.0(前插), 2.0.0]（antd6 双 DOM：只数可见 option content）
    fireEvent.mouseDown(document.querySelector('.ant-select') as HTMLElement);
    const dropdown = document.querySelector('.ant-select-dropdown') as HTMLElement;
    expect(
      within(dropdown).getAllByText('0.8.0', { selector: '.ant-select-item-option-content' }),
    ).toHaveLength(1);
    expect(
      within(dropdown).getAllByText('2.0.0', { selector: '.ant-select-item-option-content' }),
    ).toHaveLength(1);

    // 选前插的当前版本也能提交
    fireEvent.click(
      within(dropdown).getByText('0.8.0', { selector: '.ant-select-item-option-content' }),
    );
    clickOk();
    await waitFor(() => expect(mUpgrade).toHaveBeenCalledWith(7, '0.8.0'));
  });

  it('当前版本为空 → 不前插（`&& row.releaseVersion` 右翼），options 仅 releases', async () => {
    mReleases.mockResolvedValue({ total: 1, releases: [releaseOf('2.0.0')] });
    renderModal({ row: { ...row, releaseVersion: '' } });
    await waitOpen();

    fireEvent.mouseDown(document.querySelector('.ant-select') as HTMLElement);
    const dropdown = document.querySelector('.ant-select-dropdown') as HTMLElement;
    expect(
      within(dropdown).getAllByText('2.0.0', { selector: '.ant-select-item-option-content' }),
    ).toHaveLength(1);
    expect(within(dropdown).queryByText('0.0.0')).not.toBeInTheDocument();
  });

  it('空版本直接提交：warning 拦截，不调 upgradeExtension', async () => {
    renderModal();
    await waitOpen();
    expect(await screen.findByText('升级扩展')).toBeInTheDocument();

    clickOk();
    expect(await screen.findByText('请输入目标版本')).toBeInTheDocument();
    expect(mUpgrade).not.toHaveBeenCalled();
  });
});

describe('UpgradeModal 失败分支', () => {
  async function openAndSelect() {
    renderModal();
    await waitOpen();
    expect(await screen.findByText('升级扩展')).toBeInTheDocument();
    selectVersion('1.5.0');
  }

  it('missing_dependency：details 透出依赖名；details 缺省 → unknown 兜底', async () => {
    await openAndSelect();
    mUpgrade.mockRejectedValue(errWith({ code: 'missing_dependency', dependency: 'audit' }));
    clickOk();
    expect(await screen.findByText('升级失败，缺少依赖扩展：audit')).toBeInTheDocument();

    mUpgrade.mockRejectedValue(errWith({ code: 'dependency_missing' }));
    clickOk();
    expect(await screen.findByText('升级失败，缺少依赖扩展：unknown')).toBeInTheDocument();
  });

  it('version_mismatch：三元组文案；缺省全落 unknown/-', async () => {
    await openAndSelect();
    mUpgrade.mockRejectedValue(
      errWith({
        code: 'dependency_version_mismatch',
        dependency: 'audit',
        requiredVersion: '^1.0',
        currentVersion: '0.9',
      }),
    );
    clickOk();
    expect(
      await screen.findByText('升级失败，依赖版本不匹配：audit，要求 ^1.0，当前 0.9'),
    ).toBeInTheDocument();

    mUpgrade.mockRejectedValue(errWith({ code: 'dependency_version_mismatch' }));
    clickOk();
    expect(
      await screen.findByText('升级失败，依赖版本不匹配：unknown，要求 -，当前 -'),
    ).toBeInTheDocument();
  });

  it('dependency_cycle + 普通错误透传 mapper message；失败均不关弹窗', async () => {
    await openAndSelect();
    mUpgrade.mockRejectedValue(errWith({ code: 'dependency_cycle', dependency: 'chat' }));
    clickOk();
    expect(await screen.findByText('升级失败，检测到循环依赖：chat')).toBeInTheDocument();

    // cycle 无 dependency → unknown 兜底（`?? 'unknown'` 右翼）
    mUpgrade.mockRejectedValue(errWith({ code: 'dependency_cycle' }));
    clickOk();
    expect(await screen.findByText('升级失败，检测到循环依赖：unknown')).toBeInTheDocument();

    mUpgrade.mockRejectedValue(errWith({ code: 'forbidden' }));
    clickOk();
    expect(
      await screen.findByText('You do not have permission for this operation.'),
    ).toBeInTheDocument();

    // 非 HTTP 错误：mapper details 落 undefined → `|| {}` 右翼 + unknown 兜底 message
    mUpgrade.mockRejectedValue(new Error('net'));
    clickOk();
    expect(
      await screen.findByText('Please retry or contact an administrator.'),
    ).toBeInTheDocument();

    // 失败链不走 onClose
    expect(screen.getByText('升级扩展')).toBeInTheDocument();
  });
});

describe('UpgradeModal 守卫', () => {
  it('open=false 或 row=null：useEffect 早退不发请求', async () => {
    renderModal({ open: false });
    renderModal({ row: null });
    await new Promise((r) => setTimeout(r, 80));
    expect(mReleases).not.toHaveBeenCalled();
  });
});
