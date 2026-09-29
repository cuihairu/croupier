/**
 * 升级扩展弹窗单测（覆盖率巡检：UpgradeModal.tsx 151 行 0% → 收口，
 * Extensions 簇余量顺序第二位）。
 *
 * 锁定契约：打开拉目录版本列表（loading 收尾）、当前版本补齐翼（releases
 * 不含 row.releaseVersion 且非空时前置补一项；releaseVersion 空不补）、
 * 空版本提交拦截（warning、不触达 upgradeExtension）、成功链（upgrade →
 * 成功 message → onClose → onUpgraded）、失败翼四分支（missing_dependency
 * /version_mismatch/dependency_cycle 的结构化文案 + unknown 兜底 message，
 * 三分支均保持弹窗开启）、关闭/空行守卫（不发请求）。
 *
 * mock 口径：services/api/extensions 两函数 jest.mock；adapter /
 * mapExtensionError 走真实实现（与 index.test 同款）；@umijs/max 本地 mock。
 * App.useApp 的 message 在 <App> 包裹下真实渲染。
 *
 * 边界（诚实）：handleOk 的 `if (!row) return` 守卫经 UI 不可达（OK 按钮
 * 仅在弹窗 open 且 effect 已按 row 拉取后可点，row null 时同守卫已挡 effect），
 * 以 open=false 守卫用例的「不发请求」锁等价前提。
 */
import React from 'react';
import { configure, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { App } from 'antd';
import UpgradeModal from '../UpgradeModal';
import type { ExtensionInstallationItem } from '@/services/api/extensions';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

jest.mock('@/services/api/extensions', () => ({
  listExtensionCatalogReleases: jest.fn(),
  upgradeExtension: jest.fn(),
}));

jest.mock('@umijs/max', () => ({
  useIntl: () => ({
    formatMessage: (opts: { defaultMessage?: string }) => opts.defaultMessage ?? '',
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

function renderModal(props?: Partial<React.ComponentProps<typeof UpgradeModal>>) {
  const onClose = jest.fn();
  const onUpgraded = jest.fn().mockResolvedValue(undefined);
  const utils = render(
    <App>
      <UpgradeModal open row={row} onClose={onClose} onUpgraded={onUpgraded} {...props} />
    </App>,
  );
  return { ...utils, onClose, onUpgraded };
}

/** 在版本下拉里选一个 option（点可见 content 冒泡到 option onClick） */
async function pickVersion(label: string) {
  fireEvent.mouseDown(document.querySelector('.ant-select') as HTMLElement);
  const dropdown = document.querySelector('.ant-select-dropdown') as HTMLElement;
  expect(dropdown).not.toBeNull();
  fireEvent.click(
    within(dropdown).getByText(label, { selector: '.ant-select-item-option-content' }),
  );
}

/** 点弹窗 footer 主按钮（「确定」被 antd 双字插空，按选择器点） */
function clickOk() {
  const ok = document.querySelector('.ant-modal-footer .ant-btn-primary') as HTMLButtonElement;
  expect(ok).not.toBeNull();
  fireEvent.click(ok);
}

beforeEach(() => {
  jest.clearAllMocks();
  mReleases.mockResolvedValue({
    total: 2,
    releases: [
      { version: '1.5.0', releaseChannel: 'stable', minCoreVersion: '0.0.1', publishedAt: 1, changelog: '' },
      { version: '1.4.0', releaseChannel: 'stable', minCoreVersion: '0.0.1', publishedAt: 1, changelog: '' },
    ],
  });
  mUpgrade.mockResolvedValue({ status: 'upgrading' } as never);
});

describe('升级扩展弹窗 版本列表加载', () => {
  it('打开拉取目录 releases；当前版本已在列表 → 不重复前置', async () => {
    renderModal();
    await waitFor(() =>
      expect(mReleases).toHaveBeenCalledWith('chatops'),
    );

    await pickVersion('1.5.0');
    await pickVersion('1.4.0');
    // 下拉不出现补齐的重复项（补齐翼不触发：hasCurrent=true）
    fireEvent.mouseDown(document.querySelector('.ant-select') as HTMLElement);
    const dropdown = document.querySelector('.ant-select-dropdown') as HTMLElement;
    expect(
      within(dropdown).getAllByText('1.4.0', { selector: '.ant-select-item-option-content' }),
    ).toHaveLength(1);
  });

  it('当前版本补齐翼：releases 不含 row.releaseVersion → 前置补一项', async () => {
    mReleases.mockResolvedValue({
      total: 1,
      releases: [
        { version: '2.0.0', releaseChannel: 'stable', minCoreVersion: '0.0.1', publishedAt: 1, changelog: '' },
      ],
    });
    renderModal();
    await waitFor(() => expect(mReleases).toHaveBeenCalledTimes(1));

    await pickVersion('1.4.0'); // 补齐项可选即证在列
    await pickVersion('2.0.0');
  });

  it('releaseVersion 为空 → 不补齐，选项即 releases 原集', async () => {
    mReleases.mockResolvedValue({
      total: 1,
      releases: [
        { version: '2.0.0', releaseChannel: 'stable', minCoreVersion: '0.0.1', publishedAt: 1, changelog: '' },
      ],
    });
    renderModal({ row: { ...row, releaseVersion: '' } });
    await waitFor(() => expect(mReleases).toHaveBeenCalledTimes(1));

    fireEvent.mouseDown(document.querySelector('.ant-select') as HTMLElement);
    const dropdown = document.querySelector('.ant-select-dropdown') as HTMLElement;
    // 选项恰为 releases 原集：无补齐的当前版本（空 releaseVersion 不前置）
    expect(
      within(dropdown).getAllByText('2.0.0', { selector: '.ant-select-item-option-content' }),
    ).toHaveLength(1);
    expect(within(dropdown).queryByText('1.4.0')).not.toBeInTheDocument();
  });

  it('open=false 守卫：不发请求', async () => {
    renderModal({ open: false, row: null });
    await new Promise((r) => setTimeout(r, 20));
    expect(mReleases).not.toHaveBeenCalled();
    expect(mUpgrade).not.toHaveBeenCalled();
  });
});

describe('升级扩展弹窗 提交链', () => {
  it('空版本提交拦截：warning + 不触达 upgradeExtension', async () => {
    renderModal();
    await waitFor(() => expect(mReleases).toHaveBeenCalledTimes(1));
    await waitFor(() =>
      expect(document.querySelector('.ant-select')).not.toBeNull(),
    );

    clickOk();
    expect(await screen.findByText('请输入目标版本')).toBeInTheDocument();
    expect(mUpgrade).not.toHaveBeenCalled();
  });

  it('成功链：选版本 → upgradeExtension → 成功 message → onClose + onUpgraded', async () => {
    const { onClose, onUpgraded } = renderModal();
    await waitFor(() => expect(mReleases).toHaveBeenCalledTimes(1));

    await pickVersion('1.5.0');
    clickOk();

    await waitFor(() => expect(mUpgrade).toHaveBeenCalledWith(7, '1.5.0'));
    expect(await screen.findByText('升级请求已提交')).toBeInTheDocument();
    await waitFor(() => expect(onClose).toHaveBeenCalledTimes(1));
    await waitFor(() => expect(onUpgraded).toHaveBeenCalledTimes(1));
  });

  it('missing_dependency：结构化文案 + 弹窗保持开启', async () => {
    mUpgrade.mockRejectedValue({
      response: { data: { details: { code: 'missing_dependency', dependency: 'audit-log' } } },
    });
    const { onClose } = renderModal();
    await waitFor(() => expect(mReleases).toHaveBeenCalledTimes(1));

    await pickVersion('1.5.0');
    clickOk();

    await waitFor(() => expect(mUpgrade).toHaveBeenCalledWith(7, '1.5.0'));
    expect(await screen.findByText('升级失败，缺少依赖扩展：audit-log')).toBeInTheDocument();
    expect(onClose).not.toHaveBeenCalled();

    // details 不带 dependency → 'unknown' 兜底臂
    mUpgrade.mockRejectedValue({
      response: { data: { details: { code: 'missing_dependency' } } },
    });
    await pickVersion('1.5.0');
    clickOk();
    expect(await screen.findByText('升级失败，缺少依赖扩展：unknown')).toBeInTheDocument();
  });

  it('version_mismatch：依赖/要求/当前三段文案', async () => {
    mUpgrade.mockRejectedValue({
      response: {
        data: {
          details: {
            code: 'version_mismatch',
            dependency: 'chat-bridge',
            requiredVersion: '2.0.0',
            currentVersion: '1.0.0',
          },
        },
      },
    });
    renderModal();
    await waitFor(() => expect(mReleases).toHaveBeenCalledTimes(1));

    await pickVersion('1.5.0');
    clickOk();

    expect(
      await screen.findByText('升级失败，依赖版本不匹配：chat-bridge，要求 2.0.0，当前 1.0.0'),
    ).toBeInTheDocument();

    // details 三字段全缺 → unknown / - / - 兜底臂
    mUpgrade.mockRejectedValue({
      response: { data: { details: { code: 'version_mismatch' } } },
    });
    await pickVersion('1.5.0');
    clickOk();
    expect(
      await screen.findByText('升级失败，依赖版本不匹配：unknown，要求 -，当前 -'),
    ).toBeInTheDocument();
  });

  it('dependency_cycle：循环依赖文案', async () => {
    mUpgrade.mockRejectedValue({
      response: { data: { details: { code: 'dependency_cycle', dependency: 'loop-ext' } } },
    });
    renderModal();
    await waitFor(() => expect(mReleases).toHaveBeenCalledTimes(1));

    await pickVersion('1.5.0');
    clickOk();

    expect(
      await screen.findByText('升级失败，检测到循环依赖：loop-ext'),
    ).toBeInTheDocument();

    // details 不带 dependency → unknown 兜底臂
    mUpgrade.mockRejectedValue({
      response: { data: { details: { code: 'dependency_cycle' } } },
    });
    await pickVersion('1.5.0');
    clickOk();
    expect(await screen.findByText('升级失败，检测到循环依赖：unknown')).toBeInTheDocument();
  });

  it('unknown 兜底：非 HTTP 错误走 uiErr.message', async () => {
    mUpgrade.mockRejectedValue(new Error('plain'));
    renderModal();
    await waitFor(() => expect(mReleases).toHaveBeenCalledTimes(1));

    await pickVersion('1.5.0');
    clickOk();

    expect(
      await screen.findByText('Please retry or contact an administrator.'),
    ).toBeInTheDocument();
  });
});
