/*
 * SourceDetailDrawer 详情抽屉专项测试
 *
 * 纯展示组件（数据经 props 注入，无服务依赖）：概要卡（revision/
 * format/诊断计数色/contentHash）、诊断列表（空态 + 三 severity 真渲染
 * DiagnosticsList）、Operations 表全列矩阵（operationLabel 三臂、六类
 * 契约 Tag 有无二态、绑定态、写/只读操作列）、Bindings 表（删除
 * Popconfirm 确认回调、只读）、原始 JSON、extra 更新按钮与只读省略、
 * detailLoading 卡片 loading 态。
 */
import React from 'react';
import { configure, fireEvent, render, screen, waitFor } from '@testing-library/react';
import SourceDetailDrawer from '../SourceDetailDrawer';
import type {
  OpenAPISourceBinding,
  OpenAPISourceDetail,
  OpenAPISourceOperation,
} from '@/services/api/openapi';
import type { Diagnostic } from '@/types/dashboard';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

jest.mock('@umijs/max', () => ({
  FormattedMessage: ({ defaultMessage }: { id: string; defaultMessage?: string }) => (
    <>{defaultMessage ?? ''}</>
  ),
  useIntl: () => ({
    formatMessage: (opts: { defaultMessage?: string }) => opts.defaultMessage ?? '',
  }),
}));

const BASE_SOURCE = {
  sourceId: 'src-1',
  name: 'players.yaml',
  revision: 7,
  format: 'json',
  openapiVersion: '3.1.0',
  operationCount: 2,
  diagnosticCount: 0,
  contentHash: 'sha256:abc',
  createdAt: '2026-09-30T08:00:00Z',
  updatedAt: '2026-09-30T09:00:00Z',
};

// 全字段 operation：三臂 label（summary 命中）+ 契约六 Tag 全有 + 绑定态绿。
const FULL_OP: OpenAPISourceOperation = {
  operationId: 'op-full',
  method: 'GET',
  path: '/players',
  summary: '玩家列表',
  resource: 'player',
  operation: 'list',
  capability: 'collection_query',
  execution: 'sync',
  approval: { required: true, policyKey: 'gm_review' } as OpenAPISourceOperation['approval'],
  risk: 'low',
  permission: 'player:list',
  bound: true,
  bindingId: 'bd-1',
  functionId: 'fn.list',
};

// 缺省字段 operation：label 回退 operationId、契约 Tag 全占位、unbound。
const BARE_OP: OpenAPISourceOperation = {
  operationId: 'op-bare',
  method: 'POST',
  path: '/players',
  approval: { required: false } as OpenAPISourceOperation['approval'],
  bound: false,
};

const BINDING: OpenAPISourceBinding = {
  bindingId: 'bd-1',
  operationId: 'op-full',
  kind: 'provider',
  functionId: 'fn.list',
  createdAt: '2026-09-30T08:00:00Z',
  updatedAt: '2026-09-30T08:00:00Z',
};

const DIAGNOSTICS: Diagnostic[] = [
  { code: 'E_EMPTY', severity: 'error', message: 'operation 为空', field: 'paths' },
  { code: 'W_LEGACY', severity: 'warning', message: '旧字段' },
  { code: 'I_NOTE', severity: 'info', message: '提示' },
];

const makeDetail = (over: Partial<OpenAPISourceDetail> = {}): OpenAPISourceDetail => ({
  ...BASE_SOURCE,
  operations: [FULL_OP, BARE_OP],
  bindings: [BINDING],
  spec: { openapi: '3.1.0', info: { title: 'players', version: '1' } },
  ...over,
});

type Handlers = {
  onClose?: jest.Mock;
  onUpdateSource?: jest.Mock;
  onBindOperation?: jest.Mock;
  onRemoveBinding?: jest.Mock;
};

const renderDrawer = (
  detail: OpenAPISourceDetail | null = makeDetail(),
  canWrite = true,
  detailLoading = false,
  handlers: Handlers = {},
) =>
  render(
    <SourceDetailDrawer
      detail={detail}
      detailLoading={detailLoading}
      canWrite={canWrite}
      onClose={handlers.onClose ?? jest.fn()}
      onUpdateSource={handlers.onUpdateSource ?? jest.fn()}
      onBindOperation={handlers.onBindOperation ?? jest.fn()}
      onRemoveBinding={handlers.onRemoveBinding ?? jest.fn()}
    />,
  );

const clickPopconfirmOk = async () => {
  let ok: HTMLElement | null = null;
  await waitFor(() => {
    ok = document.querySelector('.ant-popconfirm .ant-btn-primary');
    expect(ok).not.toBeNull();
  });
  fireEvent.click(ok as HTMLElement);
};

describe('概要与头部', () => {
  it('detail=null：抽屉关闭，内容不渲染', () => {
    const { container } = renderDrawer(null);
    expect(container.querySelector('.ant-drawer-open')).toBeNull();
    expect(screen.queryByText('Operations')).not.toBeInTheDocument();
  });

  it('概要卡：rev/format/openapiVersion/诊断色/contentHash + extra 更新回调', async () => {
    const onUpdateSource = jest.fn();
    renderDrawer(makeDetail({ diagnosticCount: 3 }), true, false, { onUpdateSource });

    expect(await screen.findByText('rev 7')).toBeInTheDocument();
    expect(screen.getByText('json')).toBeInTheDocument();
    expect(screen.getByText('3.1.0')).toBeInTheDocument();
    expect(screen.getByText('diagnostics 3')).toHaveClass('ant-tag-orange');
    expect(screen.getByText('sha256:abc')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /更新 Source/ }));
    expect(onUpdateSource).toHaveBeenCalledWith(expect.objectContaining({ sourceId: 'src-1' }));
  });

  it('诊断 0 计数绿色 Tag', async () => {
    renderDrawer(makeDetail({ diagnosticCount: 0 }));
    expect(await screen.findByText('diagnostics 0')).toHaveClass('ant-tag-green');
  });

  it('detailLoading：卡片 loading 态渲染', async () => {
    renderDrawer(makeDetail(), true, true);
    await waitFor(() => {
      expect(document.querySelector('.ant-card-loading')).not.toBeNull();
    });
  });
});

describe('诊断列表（DiagnosticsList 真渲染）', () => {
  it('空诊断 → 「无诊断」占位（[] 与 undefined 双臂）', async () => {
    // antd Drawer 经 portal 挂 document.body：两臂须先后 unmount，
    // 否则「无诊断」同文两处 findByText 报 multiple。
    const first = renderDrawer(makeDetail({ diagnostics: [] }));
    expect(await screen.findByText('无诊断')).toBeInTheDocument();
    first.unmount();

    renderDrawer(makeDetail({ diagnostics: undefined }));
    expect(await screen.findByText('无诊断')).toBeInTheDocument();
  });

  it('三 severity：Alert 类型 + Tag 色 + field 有无', async () => {
    renderDrawer(makeDetail({ diagnostics: DIAGNOSTICS }));

    expect(await screen.findByText('operation 为空')).toBeInTheDocument();
    expect(screen.getByText('E_EMPTY')).toBeInTheDocument();
    expect(screen.getByText('paths')).toBeInTheDocument();
    // error → 红 Tag、warning → 橙、info → 蓝。
    expect(screen.getByText('error')).toHaveClass('ant-tag-red');
    expect(screen.getByText('warning')).toHaveClass('ant-tag-orange');
    expect(screen.getByText('info')).toHaveClass('ant-tag-blue');
    expect(screen.getByText('旧字段')).toBeInTheDocument();
    expect(screen.getByText('提示')).toBeInTheDocument();
    // field 缺省的 warning 项：无 field 文本节点（code 仍渲染）。
    expect(screen.getByText('W_LEGACY')).toBeInTheDocument();
  });
});

describe('Operations 表', () => {
  it('label 三臂：summary 命中 / 回退 operationId + method/path', async () => {
    renderDrawer();
    expect(await screen.findByText('玩家列表')).toBeInTheDocument();
    // op-full 同时出现在 Operation 列与 Bindings 表 operationId 列。
    expect(screen.getAllByText('op-full').length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText('GET /players')).toBeInTheDocument();
    // label 回退臂与 code 行同文两处。
    expect(screen.getAllByText('op-bare').length).toBeGreaterThanOrEqual(2);
    expect(screen.getByText('POST /players')).toBeInTheDocument();
  });

  it('契约六 Tag：全字段 vs 全占位', async () => {
    renderDrawer();
    expect(await screen.findByText('player')).toBeInTheDocument();
    expect(screen.getByText('list')).toBeInTheDocument();
    expect(screen.getByText('collection_query')).toBeInTheDocument();
    expect(screen.getByText('sync')).toBeInTheDocument();
    expect(screen.getByText('approval:gm_review')).toBeInTheDocument();
    expect(screen.getByText('low')).toBeInTheDocument();
    expect(screen.getByText('player:list')).toBeInTheDocument();

    // 占位 Tag（无 resource/operation/capability/execution/risk/permission）。
    expect(screen.getAllByText('无 resource').length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText('无 operation').length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText('无 capability').length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText('无 execution').length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText('无 approval').length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText('无 risk').length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText('无 permission').length).toBeGreaterThanOrEqual(1);
  });

  it('approval：required 无 policyKey → approval:required 兜底', async () => {
    renderDrawer(
      makeDetail({
        operations: [
          {
            ...BARE_OP,
            operationId: 'op-req',
            approval: { required: true } as OpenAPISourceOperation['approval'],
          },
        ],
        bindings: [],
      }),
    );
    expect(await screen.findByText('approval:required')).toBeInTheDocument();
  });

  it('绑定态：bound 绿 + bindingId/functionId；unbound 橙无 id', async () => {
    renderDrawer();
    expect(await screen.findByText('bound')).toHaveClass('ant-tag-green');
    expect(screen.getByText('unbound')).toHaveClass('ant-tag-orange');
    // bd-1 同时出现在 Operations 绑定列与 Bindings 表首列。
    expect(screen.getAllByText('bd-1').length).toBeGreaterThanOrEqual(2);
    expect(screen.getAllByText('fn.list').length).toBeGreaterThanOrEqual(2);
  });

  it('canWrite：绑定按钮回调透传 operation', async () => {
    const onBindOperation = jest.fn();
    renderDrawer(makeDetail(), true, false, { onBindOperation });
    const binds = await screen.findAllByRole('button', { name: /绑定/ });
    fireEvent.click(binds[0]);
    expect(onBindOperation).toHaveBeenCalledWith(
      expect.objectContaining({ operationId: 'op-full' }),
    );
  });
});

describe('Bindings 表', () => {
  it('行渲染：bindingId/operationId/functionId/kind Tag', async () => {
    renderDrawer();
    expect(await screen.findByText('Provider Bindings')).toBeInTheDocument();
    expect(screen.getAllByText('bd-1').length).toBeGreaterThanOrEqual(2);
    expect(screen.getAllByText('op-full').length).toBeGreaterThanOrEqual(2);
    expect(screen.getByText('provider')).toBeInTheDocument();
  });

  it('删除 Popconfirm 确认 → onRemoveBinding 透传', async () => {
    const onRemoveBinding = jest.fn();
    renderDrawer(makeDetail(), true, false, { onRemoveBinding });
    fireEvent.click(await screen.findByRole('button', { name: /删除/ }));
    expect(await screen.findByText('删除此 binding？')).toBeInTheDocument();
    await clickPopconfirmOk();
    expect(onRemoveBinding).toHaveBeenCalledWith(expect.objectContaining({ bindingId: 'bd-1' }));
  });

  it('bindings 缺省：空表不渲染行', async () => {
    renderDrawer(makeDetail({ bindings: undefined }));
    expect(await screen.findByText('Provider Bindings')).toBeInTheDocument();
    expect(screen.queryByText('provider')).not.toBeInTheDocument();
  });
});

describe('只读与原始 JSON', () => {
  it('canWrite=false：无更新/绑定/删除按钮，两表显只读文本', async () => {
    renderDrawer(makeDetail(), false);
    expect(await screen.findByText('Operations')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /更新 Source/ })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /绑定/ })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /删除/ })).not.toBeInTheDocument();
    expect(screen.getAllByText('只读')).toHaveLength(3);
  });

  it('原始 JSON 卡：spec 序列化可复制；spec 缺省回退 {}', async () => {
    const { unmount } = renderDrawer(makeDetail());
    expect(
      await screen.findByText((content) => content.includes('"openapi": "3.1.0"')),
    ).toBeInTheDocument();
    unmount();

    renderDrawer(makeDetail({ spec: undefined }));
    expect(await screen.findByText('{}')).toBeInTheDocument();
  });
});
