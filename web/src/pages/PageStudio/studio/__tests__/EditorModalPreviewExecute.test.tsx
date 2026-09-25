/** EditorModal 直播预览侧 onExecute 拒绝路径 + 告警列表 id 双空回退占位。
 * EditorModal.test.tsx 覆盖挂载/告警/开关主干；本文件只补两处缺口：
 * 1) livePreview 开时传给 PageRenderer 的 onExecute 必须抛「预览不执行函数」；
 * 2) bindingFreshness 条目 functionId/bindingId 皆空时列表项回退 '-'。 */
import React from 'react';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { App } from 'antd';
import EditorModal from '../EditorModal';
import type { MenuItem } from '@/services/api/menu';
import type { BindingFreshnessDiagnostic, PageSpecDraft } from '@/types/dashboard';

jest.mock('@umijs/max', () => ({
  useIntl: () => ({
    locale: 'zh-CN',
    formatMessage: ({ defaultMessage }, values) =>
      Object.entries(values || {}).reduce(
        (msg: string, [key, val]) => msg.split(`{${key}}`).join(String(val)),
        defaultMessage,
      ),
  }),
  FormattedMessage: ({ defaultMessage }: { defaultMessage?: string }) => <>{defaultMessage}</>,
}));

jest.mock('@/components/PageEditor', () => {
  const MockPageEditor: React.FC<{
    value: unknown;
    onChange: (v: unknown) => void;
    mountMenuSlot?: React.ReactNode;
  }> = ({ mountMenuSlot }) => <div data-testid="page-editor">{mountMenuSlot}</div>;
  return { __esModule: true, default: MockPageEditor };
});

// 替身捕获真实回调：onExecute 由本测试显式调用以覆盖拒绝分支
interface RendererProps {
  pageSpec: unknown;
  preview?: boolean;
  onExecute?: () => Promise<void>;
}
let mockRendererProps: RendererProps | null = null;
jest.mock('@/components/PageRenderer', () => ({
  __esModule: true,
  default: (props: RendererProps) => {
    mockRendererProps = props;
    return <div data-testid="page-renderer" />;
  },
}));

const menu = (id: number, menuKey: string, zh: string): MenuItem => ({
  id,
  parentId: null,
  menuKey,
  labels: { 'zh-CN': zh },
  sortOrder: 0,
  isVisible: true,
  children: [],
});

const MENUS: MenuItem[] = [menu(1, 'player', '玩家')];

const draft: PageSpecDraft = {
  pageKey: 'operation--inventory.consume',
  type: 'operation',
  title: { 'zh-CN': '消耗道具' },
  category: { key: 'inventory' },
  status: 'draft',
  draftRevision: 3,
  updatedAt: '2026-09-19T00:00:00Z',
};

function renderModal(props: Partial<Parameters<typeof EditorModal>[0]> = {}) {
  const onSave = jest.fn();
  const onClose = jest.fn();
  const onLivePreviewChange = jest.fn();
  const onSyncSelectors = jest.fn();
  const utils = render(
    <App>
      <EditorModal
        open
        pageKey="operation--inventory.consume"
        draft={draft}
        livePreview={false}
        saving={false}
        menus={MENUS}
        onClose={onClose}
        onLivePreviewChange={onLivePreviewChange}
        onSave={onSave}
        onSpecChange={jest.fn()}
        onSyncSelectors={onSyncSelectors}
        {...props}
      />
    </App>,
  );
  return { onSave, onClose, onLivePreviewChange, onSyncSelectors, ...utils };
}

describe('EditorModal 直播预览 onExecute', () => {
  beforeEach(() => {
    mockRendererProps = null;
  });

  it('livePreview 开：PageRenderer 收到 onExecute，调用即拒绝且不执行函数', async () => {
    renderModal({ livePreview: true });
    expect(screen.getByTestId('page-renderer')).toBeInTheDocument();
    expect(mockRendererProps?.preview).toBe(true);
    expect(typeof mockRendererProps?.onExecute).toBe('function');

    const attempt = mockRendererProps?.onExecute?.();
    expect(attempt).toBeInstanceOf(Promise);
    await expect(attempt).rejects.toThrow('Page Studio 预览不执行函数；发布后请在运行控制台执行。');
  });

  it('onExecute 为受控回调：重复调用持续拒绝（不缓存结果）', async () => {
    renderModal({ livePreview: true });
    await expect(mockRendererProps?.onExecute?.()).rejects.toThrow(/预览不执行函数/);
    await expect(mockRendererProps?.onExecute?.()).rejects.toThrow(/运行控制台执行/);
  });
});

describe('EditorModal 告警条目 id 双空回退', () => {
  const noIds = (message: string): BindingFreshnessDiagnostic => ({
    bindingId: '',
    status: 'function_version_stale',
    diagnostic: { code: 'function_version_stale', severity: 'warning', message },
  });

  it('functionId/bindingId 皆空 → 列表项渲染占位 -', () => {
    renderModal({ draft: { ...draft, bindingFreshness: [noIds('契约已变更')] } });

    expect(screen.getByText('页面绑定与函数契约不一致（发布会校验失败）')).toBeInTheDocument();
    expect(screen.getByText('-')).toBeInTheDocument();
    expect(screen.getByText('：契约已变更')).toBeInTheDocument();
  });

  it('无诊断消息时占位 - 不追加冒号', () => {
    renderModal({ draft: { ...draft, bindingFreshness: [noIds('')] } });

    expect(screen.getByText('-')).toBeInTheDocument();
    expect(screen.queryByText('：')).not.toBeInTheDocument();
  });
});

describe('EditorModal 未改动保存回退（menuDirty=false）', () => {
  it('已挂载打开且未改动：保存并发布不带 menuId', async () => {
    const { onSave } = renderModal({ currentMenuId: 1 });
    await waitFor(() =>
      expect(document.querySelector('.ant-select-content')?.textContent).toContain('玩家'),
    );

    fireEvent.click(screen.getByRole('button', { name: /仅保存草稿/ }));
    expect(onSave).toHaveBeenLastCalledWith(undefined);

    fireEvent.click(screen.getByRole('button', { name: /保存并发布/ }));
    expect(onSave).toHaveBeenLastCalledWith({ publishAfterSave: true });
  });
});
