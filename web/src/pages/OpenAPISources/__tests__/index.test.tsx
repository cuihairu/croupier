/*
 * OpenAPI Sources 页面（index.tsx）专项测试
 *
 * 子组件（SourceModal/BindingModal/SourceDetailDrawer/PipelineSummaryModal）
 * 用替身接管：把回调透传到 DOM，聚焦驱动页面自身的全部分支——
 * 加载失败/刷新/scope 重载、只读模式、来源表渲染、详情打开、
 * 创建/更新 Source（空 spec 警告、校验诊断、管线摘要）、绑定/解绑
 * （proposal CTA 跳转、无 proposal 警告）与绑定候选去重合成。
 */
import React from 'react';
import { act, configure, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App, ConfigProvider } from 'antd';
import OpenAPISourcesPage from '../index';
import { formatDate } from '../shared';
import type {
  OpenAPISourceBinding,
  OpenAPISourceDetail,
  OpenAPISourceOperation,
  OpenAPISourcePipelineSummary,
  OpenAPISourceSummary,
} from '@/services/api/openapi';
import type { FunctionDescriptor } from '@/services/api/functions';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

jest.mock('@/services/api/openapi', () => ({
  listOpenAPISources: jest.fn(),
  listRuntimeSources: jest.fn(),
  getOpenAPISource: jest.fn(),
  createOpenAPISource: jest.fn(),
  updateOpenAPISource: jest.fn(),
  uploadOpenAPISourceFile: jest.fn(),
  bindOpenAPISourceProvider: jest.fn(),
  deleteOpenAPISourceBinding: jest.fn(),
}));
jest.mock('@/services/api/functions', () => ({
  listDescriptors: jest.fn(),
}));
jest.mock('@/stores/scope', () => ({
  isScopeReady: jest.fn(),
  subscribeScope: jest.fn(),
}));
jest.mock('@umijs/max', () => ({
  FormattedMessage: ({ defaultMessage }: { id: string; defaultMessage?: string }) => (
    <>{defaultMessage ?? ''}</>
  ),
  useIntl: () => ({
    formatMessage: (
      opts: { id: string; defaultMessage?: string },
      values?: Record<string, string>,
    ) => {
      // 语言包外置词条：mock 下按 id 退化（runtimeAgent 需要注入 agent 名）。
      let text =
        opts.id === 'pages.openapiSources.bindingModal.function.runtimeAgent'
          ? 'agent:{agent}'
          : (opts.defaultMessage ?? '');
      if (values) {
        for (const [k, v] of Object.entries(values)) text = text.replace(`{${k}}`, v);
      }
      return text;
    },
  }),
  // parseOpenAPIDocument 失败路径经 getIntl() 解析文案，mock 与运行态同形。
  getIntl: () => ({
    formatMessage: (opts: { defaultMessage?: string }) => opts.defaultMessage ?? '',
  }),
  useAccess: () => ({ canOpenAPISourcesWrite: canWriteAccess }),
  history: { push: jest.fn() },
}));

// SourceDetailDrawer 替身：detail 数据与三组回调全部透出到 DOM。
jest.mock('../SourceDetailDrawer', () => ({
  __esModule: true,
  default: ({
    detail,
    detailLoading,
    onClose,
    onUpdateSource,
    onBindOperation,
    onRemoveBinding,
  }: {
    detail: OpenAPISourceDetail | null;
    detailLoading: boolean;
    canWrite: boolean;
    onClose: () => void;
    onUpdateSource: (detail: OpenAPISourceDetail) => void;
    onBindOperation: (op: OpenAPISourceOperation) => void;
    onRemoveBinding: (b: OpenAPISourceBinding) => void;
  }) => (
    <div
      data-testid="detail-drawer"
      data-open={detail ? 'true' : 'false'}
      data-loading={detailLoading ? 'true' : 'false'}
    >
      {(detail?.operations || []).map((op) => (
        <button
          key={op.operationId}
          type="button"
          data-testid={`bind-${op.operationId}`}
          onClick={() => onBindOperation(op)}
        >
          bind:{op.operationId}
        </button>
      ))}
      {(detail?.bindings || []).map((b) => (
        <button
          key={b.bindingId}
          type="button"
          data-testid={`unbind-${b.bindingId}`}
          onClick={() => onRemoveBinding(b)}
        >
          unbind:{b.bindingId}
        </button>
      ))}
      {detail && (
        <button type="button" data-testid="drawer-update" onClick={() => onUpdateSource(detail)}>
          drawer-update
        </button>
      )}
      <button type="button" data-testid="drawer-close" onClick={onClose}>
        drawer-close
      </button>
    </div>
  ),
}));

// SourceModal 替身：name/spec 受控输入 + 确定/取消直连回调。
jest.mock('../SourceModal', () => ({
  __esModule: true,
  default: ({
    open,
    mode,
    name,
    onNameChange,
    specText,
    onSpecChange,
    file,
    onFileChange,
    onCancel,
    onOk,
  }: {
    open: boolean;
    mode: 'create' | 'update';
    name: string;
    onNameChange: (v: string) => void;
    specText: string;
    onSpecChange: (v: string) => void;
    file: unknown;
    onFileChange: (f: unknown) => void;
    onCancel: () => void;
    onOk: () => void;
  }) =>
    open ? (
      <div data-testid="source-modal" data-mode={mode}>
        <input
          data-testid="source-name"
          value={name}
          onChange={(e) => onNameChange(e.target.value)}
        />
        <textarea
          data-testid="source-spec"
          value={specText}
          onChange={(e) => onSpecChange(e.target.value)}
        />
        <span data-file={file ? 'picked' : 'none'} hidden />
        <button
          type="button"
          data-testid="source-pick-file"
          onClick={() =>
            onFileChange({
              originFileObj: new File(['{}'], 'a.json', { type: 'application/json' }),
              name: 'a.json',
            })
          }
        >
          pick-file
        </button>
        <button type="button" data-testid="source-cancel" onClick={onCancel}>
          cancel
        </button>
        <button type="button" data-testid="source-ok" onClick={onOk}>
          ok
        </button>
      </div>
    ) : null,
}));

// BindingModal 替身：透出 functionOptions 供去重断言，绑定字段与确定直连。
jest.mock('../BindingModal', () => ({
  __esModule: true,
  default: ({
    open,
    onFunctionIdChange,
    providerId,
    onProviderIdChange,
    bindingId,
    onBindingIdChange,
    functionOptions,
    onCancel,
    onOk,
  }: {
    open: boolean;
    functionId?: string;
    onFunctionIdChange: (v: string) => void;
    providerId: string;
    onProviderIdChange: (v: string) => void;
    bindingId: string;
    onBindingIdChange: (v: string) => void;
    functionOptions: Array<{ label: string; value: string }>;
    onCancel: () => void;
    onOk: () => void;
  }) =>
    open ? (
      <div data-testid="binding-modal" data-binding-id={bindingId}>
        {functionOptions.map((opt) => (
          <button
            key={opt.value}
            type="button"
            data-testid={`fn-opt-${opt.value}`}
            onClick={() => onFunctionIdChange(opt.value)}
          >
            {opt.label}
          </button>
        ))}
        <input
          data-testid="binding-provider"
          value={providerId}
          onChange={(e) => onProviderIdChange(e.target.value)}
        />
        <input
          data-testid="binding-id"
          value={bindingId}
          onChange={(e) => onBindingIdChange(e.target.value)}
        />
        <button type="button" data-testid="binding-cancel" onClick={onCancel}>
          cancel
        </button>
        <button type="button" data-testid="binding-ok" onClick={onOk}>
          ok
        </button>
      </div>
    ) : null,
}));

// PipelineSummaryModal 替身：透出摘要计数。
jest.mock('../PipelineSummaryModal', () => ({
  __esModule: true,
  default: ({
    summary,
    onClose,
  }: {
    summary: OpenAPISourcePipelineSummary | null;
    onClose: () => void;
  }) =>
    summary ? (
      <div data-testid="pipeline-summary">
        <span>{`contracts:${summary.contractsCreated}`}</span>
        <button type="button" data-testid="pipeline-close" onClick={onClose}>
          close
        </button>
      </div>
    ) : null,
}));

let canWriteAccess = true;

const {
  listOpenAPISources,
  listRuntimeSources,
  getOpenAPISource,
  createOpenAPISource,
  updateOpenAPISource,
  uploadOpenAPISourceFile,
  bindOpenAPISourceProvider,
  deleteOpenAPISourceBinding,
} = jest.requireMock('@/services/api/openapi') as Record<string, jest.Mock>;
const { listDescriptors } = jest.requireMock('@/services/api/functions') as {
  listDescriptors: jest.Mock;
};
const { isScopeReady, subscribeScope } = jest.requireMock('@/stores/scope') as {
  isScopeReady: jest.Mock;
  subscribeScope: jest.Mock;
};
const { history } = jest.requireMock('@umijs/max') as { history: { push: jest.Mock } };

const SOURCE: OpenAPISourceSummary = {
  sourceId: 'src-1',
  name: 'players.yaml',
  revision: 3,
  format: 'json',
  openapiVersion: '3.0.0',
  operationCount: 2,
  diagnosticCount: 1,
  createdAt: '2026-09-30T08:00:00Z',
  updatedAt: '2026-09-30T09:30:00Z',
};

const SPEC_JSON = JSON.stringify({ openapi: '3.0.0', info: { title: 'x', version: '1' } });

const OPERATIONS: OpenAPISourceOperation[] = [
  {
    operationId: 'op-get',
    method: 'GET',
    path: '/players',
    bound: false,
    approval: 'never',
  },
  {
    operationId: 'op-post',
    method: 'POST',
    path: '/players',
    bound: true,
    approval: 'never',
    bindingId: 'bd-1',
    functionId: 'fn.post',
  },
];

const DETAIL: OpenAPISourceDetail = {
  ...SOURCE,
  spec: JSON.parse(SPEC_JSON) as OpenAPISourceDetail['spec'],
  operations: OPERATIONS,
  bindings: [
    {
      bindingId: 'bd-1',
      operationId: 'op-post',
      kind: 'provider',
      functionId: 'fn.post',
      createdAt: '',
      updatedAt: '',
    },
  ],
};

const DESCRIPTORS: FunctionDescriptor[] = [
  { id: 'fn.post', summary: { 'zh-CN': '发帖', 'en-US': 'Post' } },
  { id: 'fn.get', summary: { 'zh-CN': '查询', 'en-US': 'Get' } },
];

let scopeListener: ((scope: { gameId: string; env: string }) => void) | undefined;

const renderPage = () =>
  render(
    <App>
      <ConfigProvider>
        <OpenAPISourcesPage />
      </ConfigProvider>
    </App>,
  );

beforeEach(() => {
  jest.clearAllMocks();
  scopeListener = undefined;
  canWriteAccess = true;
  act(() => {
    isScopeReady.mockReturnValue(true);
    subscribeScope.mockImplementation((fn: typeof scopeListener) => {
      scopeListener = fn;
      return () => {};
    });
    listOpenAPISources.mockResolvedValue({ items: [SOURCE] });
    listRuntimeSources.mockResolvedValue({ items: [] });
    getOpenAPISource.mockResolvedValue({ source: DETAIL });
    listDescriptors.mockResolvedValue(DESCRIPTORS);
    createOpenAPISource.mockResolvedValue({ source: DETAIL });
    updateOpenAPISource.mockResolvedValue({ source: DETAIL });
    bindOpenAPISourceProvider.mockResolvedValue({ binding: {} });
    deleteOpenAPISourceBinding.mockResolvedValue({ binding: {} });
  });
});

describe('加载与渲染', () => {
  it('写模式：上传按钮 + 来源表全列渲染 + 行内打开/更新按钮', async () => {
    renderPage();
    expect(await screen.findByText('players.yaml')).toBeInTheDocument();
    expect(screen.getByText('src-1')).toBeInTheDocument();
    expect(screen.getByText('rev 3')).toBeInTheDocument();
    expect(screen.getByText('3.0.0')).toBeInTheDocument();
    // 操作数/诊断计数为纯数字，与分页等同文多处 → 只断言出现。
    expect(screen.getAllByText('2').length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText('1').length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText(formatDate(SOURCE.updatedAt))).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /上传 Source/ })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /更新/ })).toBeInTheDocument();
  });

  it('loadSources 失败：错误 toast + 列表清空', async () => {
    act(() => {
      listOpenAPISources.mockRejectedValue(new Error('network down'));
    });
    renderPage();
    expect(await screen.findByText('network down')).toBeInTheDocument();
    await waitFor(() => {
      expect(screen.queryByText('players.yaml')).not.toBeInTheDocument();
    });
  });

  it('刷新按钮重拉 sources 与 runtime', async () => {
    renderPage();
    await screen.findByText('players.yaml');
    fireEvent.click(screen.getByRole('button', { name: /刷新/ }));
    await waitFor(() => {
      expect(listOpenAPISources).toHaveBeenCalledTimes(2);
      expect(listRuntimeSources).toHaveBeenCalledTimes(2);
    });
  });

  it('scope 订阅回调触发整页重载', async () => {
    renderPage();
    await screen.findByText('players.yaml');
    expect(scopeListener).toBeDefined();
    act(() => {
      scopeListener!({ gameId: 'demo', env: 'prod' });
    });
    await waitFor(() => {
      expect(listOpenAPISources).toHaveBeenCalledTimes(2);
    });
  });
});

describe('详情', () => {
  it('行内打开：拉详情 → 抽屉替身 open + diagnostics 渲染最近诊断卡', async () => {
    act(() => {
      getOpenAPISource.mockResolvedValue({
        source: {
          ...DETAIL,
          diagnostics: [
            { code: 'OP_EMPTY', severity: 'error', message: 'operation 缺失' },
            { code: 'LEGACY', severity: 'warning', message: '旧字段' },
            { code: 'NOTE', severity: 'info', message: '提示' },
          ],
        },
      });
    });
    renderPage();
    fireEvent.click(await screen.findByRole('button', { name: '打开' }));
    await waitFor(() => {
      expect(screen.getByTestId('detail-drawer')).toHaveAttribute('data-open', 'true');
    });
    expect(await screen.findByText('最近一次诊断')).toBeInTheDocument();
    expect(screen.getByText('OP_EMPTY')).toBeInTheDocument();
    expect(screen.getByText('operation 缺失')).toBeInTheDocument();
  });

  it('详情加载完成清空残留诊断：无诊断卡', async () => {
    renderPage();
    fireEvent.click(await screen.findByRole('button', { name: '打开' }));
    await waitFor(() => {
      expect(screen.getByTestId('detail-drawer')).toHaveAttribute('data-open', 'true');
    });
    expect(screen.queryByText('最近一次诊断')).not.toBeInTheDocument();
  });
});

describe('Source 创建/更新', () => {
  it('上传按钮打开 create 弹窗；空 spec 确定给警告且不触服务', async () => {
    renderPage();
    fireEvent.click(await screen.findByRole('button', { name: /上传 Source/ }));
    expect(screen.getByTestId('source-modal')).toHaveAttribute('data-mode', 'create');
    fireEvent.click(screen.getByTestId('source-ok'));
    expect(await screen.findByText('请上传文件或粘贴 OpenAPI JSON')).toBeInTheDocument();
    expect(createOpenAPISource).not.toHaveBeenCalled();
  });

  it('粘贴合法 spec 提交：createOpenAPISource + 成功 toast + 重载 + 打开详情', async () => {
    renderPage();
    fireEvent.click(await screen.findByRole('button', { name: /上传 Source/ }));
    fireEvent.change(screen.getByTestId('source-spec'), { target: { value: SPEC_JSON } });
    fireEvent.click(screen.getByTestId('source-ok'));
    expect(await screen.findByText('OpenAPI Source 已创建')).toBeInTheDocument();
    expect(createOpenAPISource).toHaveBeenCalledWith(JSON.parse(SPEC_JSON), undefined);
    await waitFor(() => {
      expect(listOpenAPISources).toHaveBeenCalledTimes(2);
      expect(screen.getByTestId('detail-drawer')).toHaveAttribute('data-open', 'true');
    });
    // 成功后弹窗关闭、表单复位。
    expect(screen.queryByTestId('source-modal')).not.toBeInTheDocument();
  });

  it('create 响应带 summary：渲染管线摘要弹窗（不再叠加 toast）', async () => {
    act(() => {
      createOpenAPISource.mockResolvedValue({
        source: DETAIL,
        summary: { operations: 5, contractsCreated: 4, templatesUpdated: 2, proposalsCreated: 1 },
      });
    });
    renderPage();
    fireEvent.click(await screen.findByRole('button', { name: /上传 Source/ }));
    fireEvent.change(screen.getByTestId('source-spec'), { target: { value: SPEC_JSON } });
    fireEvent.click(screen.getByTestId('source-ok'));
    expect(await screen.findByText('contracts:4')).toBeInTheDocument();
    expect(screen.queryByText('OpenAPI Source 已创建')).not.toBeInTheDocument();
    fireEvent.click(screen.getByTestId('pipeline-close'));
    expect(screen.queryByTestId('pipeline-summary')).not.toBeInTheDocument();
  });

  it('create 失败带 diagnostics：渲染最近诊断卡 + 校验失败提示', async () => {
    act(() => {
      createOpenAPISource.mockRejectedValue({
        response: {
          data: {
            details: {
              diagnostics: [{ code: 'BAD_FIELD', severity: 'error', message: '不支持字段' }],
            },
          },
        },
      });
    });
    renderPage();
    fireEvent.click(await screen.findByRole('button', { name: /上传 Source/ }));
    fireEvent.change(screen.getByTestId('source-spec'), { target: { value: SPEC_JSON } });
    fireEvent.click(screen.getByTestId('source-ok'));
    expect(await screen.findByText('OpenAPI Source 校验失败，请查看诊断')).toBeInTheDocument();
    expect(screen.getByText('BAD_FIELD')).toBeInTheDocument();
    expect(screen.getByText('不支持字段')).toBeInTheDocument();
    // 校验失败不关弹窗（修复后重试）。
    expect(screen.getByTestId('source-modal')).toBeInTheDocument();
  });

  it('行内更新：拉详情回填 update 模式 + 提交走 updateOpenAPISource', async () => {
    renderPage();
    fireEvent.click(await screen.findByRole('button', { name: /更新/ }));
    await waitFor(() => {
      expect(screen.getByTestId('source-modal')).toHaveAttribute('data-mode', 'update');
    });
    expect(screen.getByTestId('source-name')).toHaveValue('players.yaml');
    expect((screen.getByTestId('source-spec') as HTMLTextAreaElement).value).toContain('"openapi"');
    fireEvent.change(screen.getByTestId('source-spec'), { target: { value: SPEC_JSON } });
    fireEvent.click(screen.getByTestId('source-ok'));
    expect(await screen.findByText('OpenAPI Source 已更新')).toBeInTheDocument();
    expect(updateOpenAPISource).toHaveBeenCalledWith(
      'src-1',
      JSON.parse(SPEC_JSON),
      'players.yaml',
    );
  });

  it('update 空 spec：警告且不触服务', async () => {
    renderPage();
    fireEvent.click(await screen.findByRole('button', { name: /更新/ }));
    await waitFor(() => {
      expect(screen.getByTestId('source-modal')).toBeInTheDocument();
    });
    fireEvent.change(screen.getByTestId('source-spec'), { target: { value: '   ' } });
    fireEvent.click(screen.getByTestId('source-ok'));
    expect(await screen.findByText('请粘贴新的 OpenAPI JSON')).toBeInTheDocument();
    expect(updateOpenAPISource).not.toHaveBeenCalled();
  });

  it('抽屉内更新复用已打开 detail，不再二次请求', async () => {
    renderPage();
    fireEvent.click(await screen.findByRole('button', { name: '打开' }));
    await waitFor(() => {
      expect(screen.getByTestId('detail-drawer')).toHaveAttribute('data-open', 'true');
    });
    expect(getOpenAPISource).toHaveBeenCalledTimes(1);
    fireEvent.click(screen.getByTestId('drawer-update'));
    await waitFor(() => {
      expect(screen.getByTestId('source-modal')).toHaveAttribute('data-mode', 'update');
    });
    expect(getOpenAPISource).toHaveBeenCalledTimes(1);
  });

  it('取消弹窗复位 create 模式', async () => {
    renderPage();
    fireEvent.click(await screen.findByRole('button', { name: /上传 Source/ }));
    fireEvent.click(screen.getByTestId('source-cancel'));
    expect(screen.queryByTestId('source-modal')).not.toBeInTheDocument();
    // 重开回到干净 create 态。
    fireEvent.click(screen.getByRole('button', { name: /上传 Source/ }));
    expect(screen.getByTestId('source-modal')).toHaveAttribute('data-mode', 'create');
    expect(screen.getByTestId('source-name')).toHaveValue('');
  });
});

describe('绑定/解绑', () => {
  const openBinding = async () => {
    renderPage();
    fireEvent.click(await screen.findByRole('button', { name: '打开' }));
    await waitFor(() => {
      expect(screen.getByTestId('detail-drawer')).toHaveAttribute('data-open', 'true');
    });
    fireEvent.click(screen.getByTestId('bind-op-get'));
    expect(await screen.findByTestId('binding-modal')).toBeInTheDocument();
  };

  it('打开绑定弹窗：候选 = 描述符 + 运行时去重（运行时 agent 标注）', async () => {
    act(() => {
      listRuntimeSources.mockResolvedValue({
        items: [
          {
            providerId: 'p1',
            name: 'players',
            agentId: 'agent-a',
            gameId: 'demo',
            env: 'dev',
            firstSeenUnix: 0,
            functionCount: 2,
            functions: ['fn.post', 'fn.runtimeOnly'],
            lastSeenUnix: 0,
          },
        ],
      });
    });
    await openBinding();
    // fn.post 已有描述符 → 只保留描述符版本；fn.get 描述符直出；runtimeOnly 标注 agent。
    expect(screen.getByTestId('fn-opt-fn.post')).toHaveTextContent('发帖 (fn.post)');
    expect(screen.getByTestId('fn-opt-fn.get')).toBeInTheDocument();
    expect(screen.getByTestId('fn-opt-fn.runtimeOnly')).toHaveTextContent(
      'fn.runtimeOnly（agent:agent-a）',
    );
    expect(screen.queryByText(/fn.post（/)).not.toBeInTheDocument();
  });

  it('提交绑定：载荷带 operation/function/bindingId + provider 归一', async () => {
    await openBinding();
    fireEvent.change(screen.getByTestId('binding-id'), { target: { value: '  bd-custom  ' } });
    fireEvent.change(screen.getByTestId('binding-provider'), { target: { value: '  prov-1  ' } });
    fireEvent.click(screen.getByTestId('fn-opt-fn.get'));
    fireEvent.click(screen.getByTestId('binding-ok'));
    await waitFor(() => {
      expect(bindOpenAPISourceProvider).toHaveBeenCalledWith('src-1', {
        operationId: 'op-get',
        functionId: 'fn.get',
        providerId: 'prov-1',
        bindingId: 'bd-custom',
      });
    });
    // 保存后重拉详情与列表。
    await waitFor(() => {
      expect(listOpenAPISources).toHaveBeenCalledTimes(2);
      expect(getOpenAPISource).toHaveBeenCalledTimes(2);
    });
  });

  it('绑定保存返回 proposal：成功 modal CTA 跳转 Proposal 收件箱', async () => {
    act(() => {
      bindOpenAPISourceProvider.mockResolvedValue({
        binding: {},
        proposal: {
          proposalKey: 'pk-1',
          pageKey: 'pg',
          pageType: 'crud',
          resourceKey: 'res-9',
          quality: 'good',
          status: 'draft',
        },
      });
    });
    await openBinding();
    fireEvent.click(screen.getByTestId('fn-opt-fn.get'));
    fireEvent.click(screen.getByTestId('binding-ok'));
    expect(await screen.findByText(/已生成默认页面 Proposal：pk-1/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: /打开 Proposal/ }));
    expect(history.push).toHaveBeenCalledWith(
      '/functions/pages?resourceKey=res-9&proposalKey=pk-1',
    );
  });

  it('绑定保存无 proposal：警告文案', async () => {
    await openBinding();
    fireEvent.click(screen.getByTestId('fn-opt-fn.get'));
    fireEvent.click(screen.getByTestId('binding-ok'));
    expect(await screen.findByText(/未返回可发布 Proposal/)).toBeInTheDocument();
  });

  it('绑定保存失败：错误透传', async () => {
    act(() => {
      bindOpenAPISourceProvider.mockRejectedValue(new Error('bind boom'));
    });
    await openBinding();
    fireEvent.click(screen.getByTestId('fn-opt-fn.get'));
    fireEvent.click(screen.getByTestId('binding-ok'));
    expect(await screen.findByText('bind boom')).toBeInTheDocument();
  });

  it('解绑：deleteOpenAPISourceBinding + 重拉详情/列表', async () => {
    renderPage();
    fireEvent.click(await screen.findByRole('button', { name: '打开' }));
    await waitFor(() => {
      expect(screen.getByTestId('detail-drawer')).toHaveAttribute('data-open', 'true');
    });
    fireEvent.click(screen.getByTestId('unbind-bd-1'));
    expect(await screen.findByText('binding 已删除')).toBeInTheDocument();
    expect(deleteOpenAPISourceBinding).toHaveBeenCalledWith('src-1', 'bd-1');
    await waitFor(() => {
      expect(listOpenAPISources).toHaveBeenCalledTimes(2);
    });
  });
});

describe('失败分支与守卫', () => {
  it('runtime/函数描述符加载失败：静默清空不阻塞页面', async () => {
    act(() => {
      listRuntimeSources.mockRejectedValue(new Error('runtime boom'));
      listDescriptors.mockRejectedValue(new Error('desc boom'));
    });
    renderPage();
    expect(await screen.findByText('players.yaml')).toBeInTheDocument();
    expect(screen.getByText('当前 scope 没有运行中的 openapi provider')).toBeInTheDocument();
    // 描述符缺席时绑定候选只剩空（详情仍可开）。
    fireEvent.click(screen.getByRole('button', { name: '打开' }));
    await waitFor(() => {
      expect(screen.getByTestId('detail-drawer')).toHaveAttribute('data-open', 'true');
    });
    fireEvent.click(screen.getByTestId('bind-op-get'));
    expect(await screen.findByTestId('binding-modal')).toBeInTheDocument();
    // 候选为空（描述符与运行时都失败）。
    expect(screen.queryByTestId(/^fn-opt-/)).not.toBeInTheDocument();
  });

  it('更新入口详情拉取失败：toast 透传且弹窗不开', async () => {
    act(() => {
      getOpenAPISource.mockRejectedValue(new Error('fetch boom'));
    });
    renderPage();
    fireEvent.click(await screen.findByRole('button', { name: /更新/ }));
    expect(await screen.findByText('fetch boom')).toBeInTheDocument();
    expect(screen.queryByTestId('source-modal')).not.toBeInTheDocument();
  });

  it('文件上传分支：pick file → uploadOpenAPISourceFile 带 name 兜底文件名', async () => {
    renderPage();
    fireEvent.click(await screen.findByRole('button', { name: /上传 Source/ }));
    fireEvent.click(screen.getByTestId('source-pick-file'));
    fireEvent.click(screen.getByTestId('source-ok'));
    await waitFor(() => {
      expect(uploadOpenAPISourceFile).toHaveBeenCalledTimes(1);
      expect(uploadOpenAPISourceFile.mock.calls[0][1]).toBe('a.json');
    });
  });

  it('create 失败无诊断：普通错误 toast 透传（非校验失败文案）', async () => {
    act(() => {
      createOpenAPISource.mockRejectedValue({ response: { data: { message: 'upstream 500' } } });
    });
    renderPage();
    fireEvent.click(await screen.findByRole('button', { name: /上传 Source/ }));
    fireEvent.change(screen.getByTestId('source-spec'), { target: { value: SPEC_JSON } });
    fireEvent.click(screen.getByTestId('source-ok'));
    expect(await screen.findByText('upstream 500')).toBeInTheDocument();
    expect(screen.queryByText('OpenAPI Source 校验失败，请查看诊断')).not.toBeInTheDocument();
  });

  it('绑定未选函数：警告文案且不触服务', async () => {
    renderPage();
    fireEvent.click(await screen.findByRole('button', { name: '打开' }));
    await waitFor(() => {
      expect(screen.getByTestId('detail-drawer')).toHaveAttribute('data-open', 'true');
    });
    fireEvent.click(screen.getByTestId('bind-op-get'));
    fireEvent.click(screen.getByTestId('binding-ok'));
    expect(await screen.findByText('请选择要绑定的函数')).toBeInTheDocument();
    expect(bindOpenAPISourceProvider).not.toHaveBeenCalled();
  });

  it('抽屉关闭回调置空详情；绑定弹窗取消只关不提交', async () => {
    renderPage();
    fireEvent.click(await screen.findByRole('button', { name: '打开' }));
    await waitFor(() => {
      expect(screen.getByTestId('detail-drawer')).toHaveAttribute('data-open', 'true');
    });
    fireEvent.click(screen.getByTestId('drawer-close'));
    await waitFor(() => {
      expect(screen.getByTestId('detail-drawer')).toHaveAttribute('data-open', 'false');
    });
    // 重新打开详情后再触发绑定（关闭后替身随 detail 清空）。
    fireEvent.click(screen.getByRole('button', { name: '打开' }));
    await waitFor(() => {
      expect(screen.getByTestId('detail-drawer')).toHaveAttribute('data-open', 'true');
    });
    fireEvent.click(screen.getByTestId('bind-op-get'));
    expect(await screen.findByTestId('binding-modal')).toBeInTheDocument();
    fireEvent.click(screen.getByTestId('binding-cancel'));
    expect(screen.queryByTestId('binding-modal')).not.toBeInTheDocument();
    expect(bindOpenAPISourceProvider).not.toHaveBeenCalled();
  });

  it('页头 info Alert：进入 Page Studio 跳转', async () => {
    renderPage();
    fireEvent.click(await screen.findByRole('button', { name: /进入 Page Studio/ }));
    expect(history.push).toHaveBeenCalledWith('/functions/pages');
  });

  it('运行时来源 Agent 链接深链 + 空地址占位 + 时间 0 占位 + 版本高亮', async () => {
    act(() => {
      listRuntimeSources.mockResolvedValue({
        items: [
          {
            providerId: 'p-zero',
            name: 'zero',
            agentId: 'agent-zero',
            gameId: 'demo',
            env: 'dev',
            firstSeenUnix: 0,
            lastSeenUnix: 0,
            functionCount: 0,
            functions: [],
          },
          {
            providerId: 'p-same',
            name: 'same',
            agentId: 'agent-same',
            gameId: 'demo',
            env: 'dev',
            version: '2.0.0',
            latestVersion: '2.0.0',
            serviceAddr: '10.0.0.9:9001',
            firstSeenUnix: 1789000000,
            lastSeenUnix: 1789000100,
            functionCount: 1,
            functions: ['f.one'],
          },
        ],
      });
    });
    renderPage();
    expect(await screen.findByText('zero')).toBeInTheDocument();
    // 时间 0 → '-' 占位；无地址 → '-'；无 metadata → '-'；latestVersion 缺省 → '-'。
    expect(screen.getAllByText('-').length).toBeGreaterThanOrEqual(4);
    // 版本一致 → 注册版本与最新版本两列 Tag 均不高亮橙色
    // （诊断列的橙色 Tag 属另一列，不在此断言范围）。
    const versionTags = screen.getAllByText('2.0.0').map((el) => el.closest('.ant-tag'));
    expect(versionTags).toHaveLength(2);
    expect(versionTags.every((t) => !t?.classList.contains('ant-tag-orange'))).toBe(true);
    // Agent 深链。
    fireEvent.click(screen.getAllByText('agent-zero')[0]);
    expect(history.push).toHaveBeenCalledWith('/ops/nodes?agentId=agent-zero');
  });
});

describe('只读模式（无写权限）', () => {
  beforeEach(() => {
    canWriteAccess = false;
  });

  it('只读 Alert + 无上传按钮 + 行内无更新按钮', async () => {
    renderPage();
    expect(await screen.findByText('当前是只读模式')).toBeInTheDocument();
    expect(screen.getByText('players.yaml')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /上传 Source/ })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /更新/ })).not.toBeInTheDocument();
    // 行内操作只有「打开」。
    expect(screen.getAllByRole('button', { name: '打开' })).toHaveLength(1);
  });

  it('防御守卫：只读下更新/绑定/解绑入口早退（弹窗不开、服务不调）', async () => {
    renderPage();
    fireEvent.click(await screen.findByRole('button', { name: '打开' }));
    await waitFor(() => {
      expect(screen.getByTestId('detail-drawer')).toHaveAttribute('data-open', 'true');
    });

    // 详情更新入口（替身仍渲染按钮，页面守卫拦截）。
    fireEvent.click(screen.getByTestId('drawer-update'));
    expect(await screen.findByText('没有 OpenAPI Source 写权限')).toBeInTheDocument();
    expect(screen.queryByTestId('source-modal')).not.toBeInTheDocument();
    expect(getOpenAPISource).toHaveBeenCalledTimes(1);

    // 绑定入口：弹窗不打开。
    fireEvent.click(screen.getByTestId('bind-op-get'));
    expect(screen.queryByTestId('binding-modal')).not.toBeInTheDocument();
    expect(bindOpenAPISourceProvider).not.toHaveBeenCalled();

    // 解绑入口：服务不调。
    fireEvent.click(screen.getByTestId('unbind-bd-1'));
    await waitFor(() => {
      expect(screen.getAllByText('没有 OpenAPI Source 写权限').length).toBeGreaterThanOrEqual(2);
    });
    expect(deleteOpenAPISourceBinding).not.toHaveBeenCalled();
  });
});
