/** 函数调用工作台（Functions/Invoke/index.tsx）编排层单测（纯补测，不修改源文件）。
 *
 * 策略：六个子组件与 approvalPolling 以薄桩/捕获桩替换（子组件各有独立测试），
 * 通过桩上按钮与输入直接驱动回调，逐支路覆盖编排层状态机：
 * - 加载：scope 未就绪早退/就绪加载、listDescriptors 成败、localStorage 历史读取异常
 * - 选择：fid 解析、displayName 三级回退、输入/输出 schema 解析（字符串/对象/数组/数字/缺省）
 * - 调用：targeted/hash 前置校验、表单/JSON 双模式取值与校验失败、审批轮询、异步任务、异常归类
 * - 快捷键 Ctrl/Cmd+Enter、历史恢复 restore、格式化与剪贴板复制
 */
import React from 'react';
import { act, configure, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App } from 'antd';
import { getLocale, history, useLocation } from '@umijs/max';
import { invokeFunction, listDescriptors, type FunctionDescriptor } from '@/services/api';
import { queryApprovalStatus } from '@/services/console';
import { isScopeReady, subscribeScope } from '@/stores/scope';
import FunctionInvokePage from '../index';
import type { RequestHistoryItem } from '../types';

// coverage instrumentation 下渲染更慢：放宽异步查询与用例超时
configure({ asyncUtilTimeout: 5000 });
jest.setTimeout(20000);

const HISTORY_KEY = 'croupier.function-invoke.history.v1';

// ---------------------------------------------------------------------------
// 模块 mock
// ---------------------------------------------------------------------------

jest.mock('@umijs/max', () => {
  const interpolate = (msg: string, values?: Record<string, unknown>) =>
    Object.entries(values ?? {}).reduce(
      (out: string, [key, val]) => out.split(`{${key}}`).join(String(val)),
      msg,
    );
  return {
    __esModule: true,
    // 与 tests/setupTests.jsx 同语义：返回 defaultMessage 并做 {placeholder} 插值
    useIntl: () => ({
      formatMessage: (descriptor: { defaultMessage: string }, values?: Record<string, unknown>) =>
        interpolate(descriptor.defaultMessage, values),
    }),
    FormattedMessage: ({
      defaultMessage,
      values,
    }: {
      defaultMessage: string;
      values?: Record<string, unknown>;
    }) => interpolate(defaultMessage, values),
    getLocale: jest.fn(() => 'zh-CN'),
    useLocation: jest.fn(() => ({ search: '' })),
    history: { push: jest.fn() },
  };
});

jest.mock('@ant-design/pro-components', () => ({
  __esModule: true,
  PageContainer: ({ extra, children }: { extra?: React.ReactNode; children?: React.ReactNode }) => (
    <div data-testid="page-container">
      <div data-testid="page-extra">{extra}</div>
      {children}
    </div>
  ),
}));

jest.mock('@/services/api', () => ({
  __esModule: true,
  invokeFunction: jest.fn(),
  listDescriptors: jest.fn(),
}));

jest.mock('@/services/console', () => ({
  __esModule: true,
  queryApprovalStatus: jest.fn(),
}));

jest.mock('@/stores/scope', () => ({
  __esModule: true,
  isScopeReady: jest.fn(() => true),
  subscribeScope: jest.fn(() => jest.fn()),
}));

// ---------------------------------------------------------------------------
// 子组件薄桩：显示关键 props，并以按钮/输入直接驱动回调
// ---------------------------------------------------------------------------

jest.mock('../ExecutionOptions', () => ({
  __esModule: true,
  default: (props: {
    route: string;
    targetServiceId: string;
    hashKey: string;
    asyncMode: boolean;
    onRouteChange: (route: 'lb' | 'broadcast' | 'targeted' | 'hash') => void;
    onTargetServiceIdChange: (value: string) => void;
    onHashKeyChange: (value: string) => void;
    onAsyncModeChange: (value: boolean) => void;
  }) => (
    <div data-testid="execution-options">
      <span data-testid="eo-route">{props.route}</span>
      <span data-testid="eo-target">{props.targetServiceId}</span>
      <span data-testid="eo-hash">{props.hashKey}</span>
      <button type="button" onClick={() => props.onRouteChange('targeted')}>
        eo-route-targeted
      </button>
      <button type="button" onClick={() => props.onRouteChange('hash')}>
        eo-route-hash
      </button>
      <button type="button" onClick={() => props.onTargetServiceIdChange('svc-9')}>
        eo-set-target
      </button>
      <button type="button" onClick={() => props.onHashKeyChange('player:1')}>
        eo-set-hash
      </button>
      <button type="button" onClick={() => props.onAsyncModeChange(!props.asyncMode)}>
        eo-toggle-async
      </button>
    </div>
  ),
}));

jest.mock('../RequestBodyEditor', () => {
  const React = jest.requireActual<typeof import('react')>('react');
  type FormHandle = {
    getValues?: () => Record<string, unknown> | undefined;
    validate?: () => boolean;
  };
  // 模块级可变句柄：formRef.current 在每次渲染后同步，测试通过 set 后触发一次
  // 模式切换渲染来刷新（点击/输入均会引发重渲染）
  let handle: FormHandle | null = null;
  // 大写命名组件：rules-of-hooks 要求 hooks 只在大写组件/以 use 开头的函数内调用
  const RequestBodyEditorStub = (props: {
    mode: string;
    rawJson: string;
    formState: { status: 'idle' | 'ready' | 'unavailable'; error?: string };
    formValues: Record<string, unknown>;
    formRef: {
      current: {
        getValues: () => Record<string, unknown> | undefined;
        validate: () => boolean;
      } | null;
    };
    onModeChange: (mode: 'form' | 'json') => void;
    onRawJsonChange: (value: string) => void;
    onFormValuesChange: (values: Record<string, unknown>) => void;
    onFormat: () => void;
  }) => {
    React.useEffect(() => {
      props.formRef.current =
        props.formState.status === 'ready' && handle
          ? {
              getValues: () => handle?.getValues?.() ?? props.formValues,
              validate: () => handle?.validate?.() ?? true,
            }
          : null;
    });
    return (
      <div data-testid="request-body-editor">
        <span data-testid="rbe-mode">{props.mode}</span>
        <span data-testid="rbe-rawjson">{props.rawJson}</span>
        <span data-testid="rbe-formstate">{props.formState.status}</span>
        <span data-testid="rbe-formvalues">{JSON.stringify(props.formValues)}</span>
        <button
          type="button"
          onClick={() => props.onModeChange(props.mode === 'form' ? 'json' : 'form')}
        >
          rbe-toggle-mode
        </button>
        <input
          data-testid="rbe-json-input"
          value={props.rawJson}
          onChange={(event) => props.onRawJsonChange(event.target.value)}
        />
        <button type="button" onClick={() => props.onFormValuesChange({ viaForm: 1 })}>
          rbe-form-values
        </button>
        <button type="button" onClick={() => props.onFormat()}>
          rbe-format
        </button>
      </div>
    );
  };
  return {
    __esModule: true,
    default: RequestBodyEditorStub,
    __setFormHandle: (next: FormHandle | null) => {
      handle = next;
    },
  };
});

jest.mock('../RequestHistory', () => ({
  __esModule: true,
  default: (props: {
    items: RequestHistoryItem[];
    onClear: () => void;
    onSelect: (item: RequestHistoryItem) => void;
  }) => (
    <div data-testid="request-history">
      <span data-testid="rh-count">{props.items.length}</span>
      <span data-testid="rh-last-status">{props.items[0]?.status ?? ''}</span>
      <button type="button" onClick={props.onClear}>
        rh-clear
      </button>
      <button
        type="button"
        disabled={props.items.length === 0}
        onClick={() => props.onSelect(props.items[0])}
      >
        rh-select-first
      </button>
      <button
        type="button"
        disabled={props.items.length < 2}
        onClick={() => props.onSelect(props.items[props.items.length - 1])}
      >
        rh-select-last
      </button>
    </div>
  ),
}));

jest.mock('../ServerHistory', () => ({
  __esModule: true,
  default: (props: { functionId?: string }) => (
    <div data-testid="server-history">{props.functionId ?? 'no-function'}</div>
  ),
}));

jest.mock('../TaskProgressPanel', () => {
  let result: unknown = { done: true };
  return {
    __esModule: true,
    default: (props: { taskId: string; onCompleted: (value: unknown) => void }) => (
      <div data-testid="task-progress">
        <span data-testid="tp-task-id">{props.taskId}</span>
        <button type="button" onClick={() => props.onCompleted(result)}>
          tp-complete
        </button>
        <button type="button" onClick={() => props.onCompleted(null)}>
          tp-complete-null
        </button>
      </div>
    ),
    __setResult: (value: unknown) => {
      result = value;
    },
  };
});

jest.mock('../InvocationResponse', () => ({
  __esModule: true,
  default: (props: {
    responseRaw: string;
    error: string;
    errorDetails?: Array<{ field: string; message: string }>;
    outputSchema: unknown;
    duration: number;
    traceId: string;
    onCopy: (value: string) => void;
  }) => (
    <div data-testid="invocation-response">
      <span data-testid="ir-response-raw">{props.responseRaw}</span>
      <span data-testid="ir-error">{props.error}</span>
      <span data-testid="ir-details">{JSON.stringify(props.errorDetails ?? [])}</span>
      <span data-testid="ir-schema">
        {props.outputSchema === null || props.outputSchema === undefined
          ? 'no-schema'
          : JSON.stringify(props.outputSchema)}
      </span>
      <span data-testid="ir-duration">{String(props.duration)}</span>
      <span data-testid="ir-trace">{props.traceId}</span>
      <button type="button" onClick={() => props.onCopy('copy-me')}>
        ir-copy
      </button>
    </div>
  ),
}));

jest.mock('../approvalPolling', () => {
  type Update = {
    status: 'approved' | 'rejected' | 'expired';
    reason?: string;
    continuation?: boolean;
    resultKind?: 'sync' | 'task';
    taskId?: string;
    result?: unknown;
  };
  type Fetcher = (id: string) => Promise<{ status: string; reason?: string }>;
  const captured: {
    approvalId: string;
    onUpdate: ((update: Update) => void) | null;
    fetcher: Fetcher | null;
    interval: number;
  } = { approvalId: '', onUpdate: null, fetcher: null, interval: -1 };
  const unsubscribe = jest.fn();
  return {
    __esModule: true,
    startApprovalPolling: jest.fn(
      (
        approvalId: string,
        onUpdate: (update: Update) => void,
        fetcher: Fetcher,
        intervalMs: number,
      ) => {
        captured.approvalId = approvalId;
        captured.onUpdate = onUpdate;
        captured.fetcher = fetcher;
        captured.interval = intervalMs;
        return unsubscribe;
      },
    ),
    __captured: captured,
    __unsubscribe: unsubscribe,
  };
});

// ---------------------------------------------------------------------------
// mock 句柄与夹具
// ---------------------------------------------------------------------------

const mockedInvoke = jest.mocked(invokeFunction);
const mockedList = jest.mocked(listDescriptors);
const mockedQueryApproval = jest.mocked(queryApprovalStatus);
const mockedScopeReady = jest.mocked(isScopeReady);
const mockedSubscribe = jest.mocked(subscribeScope);
const mockedUseLocation = jest.mocked(useLocation);
const mockedPush = jest.mocked(history.push);
const mockedGetLocale = jest.mocked(getLocale);

type LocationLike = ReturnType<typeof useLocation>;
const asLocation = (search: string): LocationLike => ({ search }) as unknown as LocationLike;

type ApprovalUpdate = {
  status: 'approved' | 'rejected' | 'expired';
  reason?: string;
  continuation?: boolean;
  resultKind?: 'sync' | 'task';
  taskId?: string;
  result?: unknown;
};
const pollingMock = jest.requireMock('../approvalPolling') as unknown as {
  startApprovalPolling: jest.Mock;
  __captured: {
    approvalId: string;
    onUpdate: ((update: ApprovalUpdate) => void) | null;
    fetcher: ((id: string) => Promise<{ status: string; reason?: string }>) | null;
    interval: number;
  };
  __unsubscribe: jest.Mock;
};
const requestBodyMock = jest.requireMock('../RequestBodyEditor') as unknown as {
  __setFormHandle: (
    handle: {
      getValues?: () => Record<string, unknown> | undefined;
      validate?: () => boolean;
    } | null,
  ) => void;
};

const INPUT_SCHEMA_JSON = JSON.stringify({
  type: 'object',
  properties: {
    playerId: { type: 'string', default: 'p-1' },
    count: { type: 'integer', default: 2 },
  },
  required: ['playerId'],
});
const OUTPUT_SCHEMA_JSON = JSON.stringify({
  type: 'object',
  properties: { ok: { type: 'boolean' } },
});

const DESCRIPTORS: FunctionDescriptor[] = [
  {
    id: 'fn.echo',
    displayName: { 'zh-CN': '回声函数' },
    description: { 'zh-CN': '原样返回请求' },
    resource: 'player',
    inputSchema: INPUT_SCHEMA_JSON,
    outputSchema: OUTPUT_SCHEMA_JSON,
  },
  { id: 'fn.bare', summary: { 'zh-CN': '仅有摘要' } },
  { id: 'fn.plain' },
  {
    id: 'fn.schema-fallback',
    inputSchema: 'not-valid-json',
    schema: { type: 'object', properties: { viaSchema: { type: 'string', default: 's' } } },
  },
  {
    id: 'fn.params-fallback',
    inputSchema: [],
    params: { type: 'object', properties: { viaParams: { type: 'boolean', default: true } } },
  },
  { id: 'fn.number-input', inputSchema: 42 },
  { id: 'fn.out-obj', outputSchema: { type: 'array' }, params: { type: 'object' } },
  { id: 'fn.out-arr', outputSchema: [1], params: { type: 'object' } },
  { id: 'fn.out-num', outputSchema: 7, params: { type: 'object' } },
];

/** 后端统一错误体形态（见 API Response Contract）：extractErrorMessage 取 message */
const INVOKE_FAILURE = {
  response: {
    data: {
      error: 'validation_failed',
      message: '请求参数无效',
      details: [{ field: 'playerId', message: '不能为空' }],
    },
  },
};

const SEEDED_HISTORY_ITEM: RequestHistoryItem = {
  id: 'seed-1',
  functionId: 'fn.bare',
  timestamp: '2026-08-01T00:00:00.000Z',
  duration: 3,
  status: 'success',
  request: { seeded: true },
  options: {},
  response: { seededResponse: 1 },
};

// ---------------------------------------------------------------------------
// 渲染与断言辅助
// ---------------------------------------------------------------------------

type AppApi = ReturnType<typeof App.useApp>;

/** 包真实 antd App 提供 useApp 上下文，并以 Probe 捕获实例供 message spy */
function mountPage(search = '') {
  mockedUseLocation.mockReturnValue(asLocation(search));
  const captured: { current?: AppApi } = {};
  const Probe: React.FC = () => {
    captured.current = App.useApp();
    return null;
  };
  const utils = render(
    <App>
      <Probe />
      <FunctionInvokePage />
    </App>,
  );
  const rerenderPage = (nextSearch: string) => {
    mockedUseLocation.mockReturnValue(asLocation(nextSearch));
    utils.rerender(
      <App>
        <Probe />
        <FunctionInvokePage />
      </App>,
    );
  };
  return { ...utils, rerenderPage, app: captured };
}

function spyOnMessage(app: { current?: AppApi }) {
  const message = app.current?.message;
  if (!message) throw new Error('App 上下文实例未捕获');
  return {
    success: jest.spyOn(message, 'success'),
    error: jest.spyOn(message, 'error'),
    info: jest.spyOn(message, 'info'),
  };
}

const clickSend = () => fireEvent.click(screen.getByRole('button', { name: /发\s*送/ }));
const clickRefresh = () => fireEvent.click(screen.getByRole('button', { name: /刷新函数/ }));
const toggleHistory = () => fireEvent.click(screen.getByRole('button', { name: /历史记录/ }));
const ctrlEnter = () => fireEvent.keyDown(window, { key: 'Enter', ctrlKey: true });
const waitForFormReady = () =>
  waitFor(() => expect(screen.getByTestId('rbe-formstate')).toHaveTextContent('ready'));
const waitForPollingUpdate = async () => {
  await waitFor(() => expect(pollingMock.__captured.onUpdate).not.toBeNull());
  return pollingMock.__captured.onUpdate as (update: ApprovalUpdate) => void;
};
const getItemMock = () => localStorage.getItem as unknown as jest.Mock;
const setItemMock = () => localStorage.setItem as unknown as jest.Mock;

beforeEach(() => {
  jest.clearAllMocks();
  mockedInvoke.mockReset();
  mockedList.mockReset().mockResolvedValue(DESCRIPTORS);
  mockedQueryApproval.mockReset();
  mockedScopeReady.mockReset().mockImplementation(() => true);
  mockedSubscribe.mockReset().mockImplementation(() => jest.fn());
  mockedGetLocale.mockReset().mockImplementation(() => 'zh-CN');
  mockedUseLocation.mockReset().mockImplementation(() => asLocation(''));
  getItemMock().mockReset();
  setItemMock().mockReset();
  pollingMock.__captured.approvalId = '';
  pollingMock.__captured.onUpdate = null;
  pollingMock.__captured.fetcher = null;
  pollingMock.__captured.interval = -1;
  requestBodyMock.__setFormHandle(null);
});

// ---------------------------------------------------------------------------
// 加载与函数列表
// ---------------------------------------------------------------------------

describe('函数调用工作台：加载与函数列表', () => {
  it('scope 未就绪时不加载；就绪后点刷新触发加载', async () => {
    mockedScopeReady.mockImplementation(() => false);
    mountPage();
    await act(async () => {});
    expect(mockedList).not.toHaveBeenCalled();

    mockedScopeReady.mockImplementation(() => true);
    clickRefresh();
    await waitFor(() => expect(mockedList).toHaveBeenCalledTimes(1));
  });

  it('listDescriptors 失败：message.error 提取 err.message，页面不崩溃', async () => {
    mockedList.mockRejectedValue(new Error('db down'));
    const { app } = mountPage();
    const spies = spyOnMessage(app);
    await act(async () => {});
    clickRefresh();
    await waitFor(() => expect(spies.error).toHaveBeenCalledWith('db down'));
    // loading 复位后仍提示选择函数
    await waitFor(() =>
      expect(screen.getByText('请选择一个已注册函数后再发送请求')).toBeInTheDocument(),
    );
  });

  it('scope 变化触发重新加载；卸载时退订', async () => {
    mountPage();
    await waitFor(() => expect(mockedList).toHaveBeenCalledTimes(1));
    const listener = mockedSubscribe.mock.calls[0][0];
    // 空 scope：gameId/env 均缺省回退 ''（scopeKey 变化仍触发重载）
    act(() => listener({}));
    await waitFor(() => expect(mockedList).toHaveBeenCalledTimes(2));
    act(() => listener({ gameId: 'g1', env: 'prod' }));
    await waitFor(() => expect(mockedList).toHaveBeenCalledTimes(3));
    const off = mockedSubscribe.mock.results[0].value as jest.Mock;
    expect(off).not.toHaveBeenCalled();
  });

  it('本地历史读取：非数组 JSON 与坏 JSON 均降级为空', async () => {
    getItemMock().mockReturnValueOnce('{"nope":1}');
    const first = mountPage();
    await waitFor(() => expect(screen.getByTestId('page-container')).toBeInTheDocument());
    toggleHistory();
    await waitFor(() => expect(screen.getByTestId('rh-count')).toHaveTextContent('0'));
    first.unmount();

    getItemMock().mockReturnValueOnce('{bad');
    mountPage();
    await waitFor(() => expect(screen.getByTestId('page-container')).toBeInTheDocument());
    toggleHistory();
    await waitFor(() => expect(screen.getByTestId('rh-count')).toHaveTextContent('0'));
  });

  it('localStorage 写入失败不阻断页面（catch 静默）', async () => {
    getItemMock().mockReturnValue(JSON.stringify([SEEDED_HISTORY_ITEM]));
    setItemMock().mockImplementationOnce(() => {
      throw new Error('quota exceeded');
    });
    mountPage();
    await waitFor(() => expect(screen.getByTestId('request-body-editor')).toBeInTheDocument());
    toggleHistory();
    await waitFor(() => expect(screen.getByTestId('rh-count')).toHaveTextContent('1'));
  });
});

// ---------------------------------------------------------------------------
// 函数选择与 Schema 解析
// ---------------------------------------------------------------------------

describe('函数调用工作台：函数选择与 Schema 解析', () => {
  it('无 fid：提示选择函数、发送禁用；displayName 三级回退（名称→摘要→id）', async () => {
    const { container } = mountPage();
    await waitFor(() =>
      expect(screen.getByText('请选择一个已注册函数后再发送请求')).toBeInTheDocument(),
    );
    expect(screen.getByRole('button', { name: /发\s*送/ })).toBeDisabled();

    fireEvent.mouseDown(screen.getByRole('combobox'));
    await waitFor(() => {
      const labels = Array.from(document.querySelectorAll('.ant-select-item-option-content')).map(
        (el) => el.textContent,
      );
      expect(labels).toContain('fn.echo  ·  回声函数');
      expect(labels).toContain('fn.bare  ·  仅有摘要');
      expect(labels).toContain('fn.plain  ·  fn.plain');
    });
    expect(container).toBeInTheDocument();
  });

  it('加载中且未选择：不显示"请选择"提示（loading 分支）', async () => {
    mockedList.mockReturnValue(new Promise<FunctionDescriptor[]>(() => {}));
    const { unmount } = mountPage();
    await act(async () => {});
    expect(screen.queryByText('请选择一个已注册函数后再发送请求')).toBeNull();
    unmount();
  });

  it('下拉选择函数 → history.push 携带 fid', async () => {
    mountPage();
    fireEvent.mouseDown(screen.getByRole('combobox'));
    const option = await screen.findByText(/回声函数/);
    fireEvent.click(option);
    await waitFor(() => expect(mockedPush).toHaveBeenCalledWith('/functions/invoke?fid=fn.echo'));
  });

  it('fid 命中有 schema 函数：表单就绪、默认值注入、resource Tag 与描述展示、发送可用', async () => {
    mountPage('?fid=fn.echo');
    await waitForFormReady();
    expect(screen.getByTestId('rbe-formvalues')).toHaveTextContent('"playerId":"p-1"');
    expect(screen.getByTestId('rbe-rawjson')).toHaveTextContent('"count": 2');
    expect(screen.getByText('回声函数')).toBeInTheDocument();
    expect(screen.getByText('原样返回请求')).toBeInTheDocument();
    expect(screen.getByText('player')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /发\s*送/ })).toBeEnabled();
  });

  it('fid 命中无 schema 函数：表单不可用且无 resource Tag', async () => {
    mountPage('?fid=fn.bare');
    await waitFor(() =>
      expect(screen.getByTestId('rbe-formstate')).toHaveTextContent('unavailable'),
    );
    expect(screen.getByText('仅有摘要')).toBeInTheDocument();
    expect(screen.queryByText('player')).toBeNull();
  });

  it('输入 schema 解析回退链：坏字符串→schema 对象、数组 inputSchema→params、数字→不可用', async () => {
    const { rerenderPage } = mountPage('?fid=fn.schema-fallback');
    await waitForFormReady();
    expect(screen.getByTestId('rbe-rawjson')).toHaveTextContent('"viaSchema": "s"');

    rerenderPage('?fid=fn.params-fallback');
    await waitFor(() =>
      expect(screen.getByTestId('rbe-rawjson')).toHaveTextContent('"viaParams": true'),
    );

    rerenderPage('?id=1&fid=fn.number-input');
    await waitFor(() =>
      expect(screen.getByTestId('rbe-formstate')).toHaveTextContent('unavailable'),
    );
  });

  it('输出 schema 解析：字符串解析/对象透传/数组与数字归空/未选择归空', async () => {
    const { rerenderPage } = mountPage('?fid=fn.echo');
    await waitFor(() =>
      expect(screen.getByTestId('ir-schema').textContent).toBe(
        '{"type":"object","properties":{"ok":{"type":"boolean"}}}',
      ),
    );

    rerenderPage('?fid=fn.out-obj');
    await waitFor(() =>
      expect(screen.getByTestId('ir-schema').textContent).toBe('{"type":"array"}'),
    );

    for (const fid of ['fn.out-arr', 'fn.out-num', 'fn.bare']) {
      rerenderPage(`?x=${fid}&fid=${fid}`);
      await waitFor(() => expect(screen.getByTestId('ir-schema').textContent).toBe('no-schema'));
    }

    rerenderPage('');
    await waitFor(() => expect(screen.getByTestId('ir-schema').textContent).toBe('no-schema'));
  });
});

// ---------------------------------------------------------------------------
// 发送前置校验
// ---------------------------------------------------------------------------

describe('函数调用工作台：发送前置校验', () => {
  it('targeted 缺 service_id：按钮禁用，Ctrl+Enter 报校验错误且不发请求；补齐后恢复可用', async () => {
    mountPage('?fid=fn.echo');
    await waitForFormReady();
    fireEvent.click(screen.getByRole('button', { name: 'eo-route-targeted' }));
    await waitFor(() => expect(screen.getByRole('button', { name: /发\s*送/ })).toBeDisabled());
    ctrlEnter();
    await waitFor(() =>
      expect(screen.getByTestId('ir-error')).toHaveTextContent('指定实例路由需要填写 service_id'),
    );
    expect(mockedInvoke).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole('button', { name: 'eo-set-target' }));
    await waitFor(() => expect(screen.getByRole('button', { name: /发\s*送/ })).toBeEnabled());
  });

  it('hash 缺 key：按钮禁用，Ctrl+Enter 报校验错误；补齐后恢复可用', async () => {
    mountPage('?fid=fn.echo');
    await waitForFormReady();
    fireEvent.click(screen.getByRole('button', { name: 'eo-route-hash' }));
    await waitFor(() => expect(screen.getByRole('button', { name: /发\s*送/ })).toBeDisabled());
    ctrlEnter();
    await waitFor(() =>
      expect(screen.getByTestId('ir-error')).toHaveTextContent('哈希路由需要填写 hash key'),
    );
    expect(mockedInvoke).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole('button', { name: 'eo-set-hash' }));
    await waitFor(() => expect(screen.getByRole('button', { name: /发\s*送/ })).toBeEnabled());
  });

  it('未选择函数时 Ctrl+Enter：直接返回不发请求', async () => {
    mountPage();
    await act(async () => {});
    ctrlEnter();
    await act(async () => {});
    expect(mockedInvoke).not.toHaveBeenCalled();
  });
});

// ---------------------------------------------------------------------------
// 调用执行
// ---------------------------------------------------------------------------

describe('函数调用工作台：调用执行', () => {
  it('JSON 模式成功：payload/选项透传、traceId、结果与历史落库、message.success', async () => {
    mockedInvoke.mockResolvedValueOnce({ traceId: 'tr-1', result: { ok: true } });
    const { app } = mountPage('?fid=fn.echo');
    const spies = spyOnMessage(app);
    await waitForFormReady();
    clickSend();
    await waitFor(() => expect(mockedInvoke).toHaveBeenCalledTimes(1));
    expect(mockedInvoke).toHaveBeenLastCalledWith(
      'fn.echo',
      { playerId: 'p-1', count: 2 },
      { route: 'lb' },
    );
    await waitFor(() => expect(screen.getByTestId('ir-trace')).toHaveTextContent('tr-1'));
    expect(screen.getByTestId('ir-response-raw')).toHaveTextContent('"ok": true');
    expect(spies.success).toHaveBeenCalledWith('调用成功');
    // 成功条目写入本地历史与 localStorage
    toggleHistory();
    await waitFor(() => expect(screen.getByTestId('rh-count')).toHaveTextContent('1'));
    expect(screen.getByTestId('rh-last-status')).toHaveTextContent('success');
    expect(setItemMock()).toHaveBeenCalledWith(HISTORY_KEY, expect.stringContaining('fn.echo'));
  });

  it('result 缺省：响应取整个 result；无 traceId 时留空', async () => {
    mockedInvoke.mockResolvedValueOnce({ someOther: 1 });
    mountPage('?fid=fn.echo');
    await waitForFormReady();
    clickSend();
    await waitFor(() => expect(mockedInvoke).toHaveBeenCalledTimes(1));
    await waitFor(() =>
      expect(screen.getByTestId('ir-response-raw')).toHaveTextContent('someOther'),
    );
    expect(screen.getByTestId('ir-trace')).toHaveTextContent('');
  });

  it('路由参数透传：targeted 携带 service_id，hash 携带 hashKey', async () => {
    mockedInvoke.mockResolvedValue({ result: { ok: 1 } });
    mountPage('?fid=fn.echo');
    await waitForFormReady();
    fireEvent.click(screen.getByRole('button', { name: 'eo-route-targeted' }));
    fireEvent.click(screen.getByRole('button', { name: 'eo-set-target' }));
    clickSend();
    await waitFor(() => expect(mockedInvoke).toHaveBeenCalledTimes(1));
    expect(mockedInvoke).toHaveBeenNthCalledWith(
      1,
      'fn.echo',
      { playerId: 'p-1', count: 2 },
      { route: 'targeted', targetServiceId: 'svc-9' },
    );

    fireEvent.click(screen.getByRole('button', { name: 'eo-route-hash' }));
    fireEvent.click(screen.getByRole('button', { name: 'eo-set-hash' }));
    clickSend();
    await waitFor(() => expect(mockedInvoke).toHaveBeenCalledTimes(2));
    expect(mockedInvoke).toHaveBeenNthCalledWith(
      2,
      'fn.echo',
      { playerId: 'p-1', count: 2 },
      { route: 'hash', hashKey: 'player:1' },
    );
  });

  it('异步任务：taskId 存在时挂载进度面板并提示任务已创建', async () => {
    mockedInvoke.mockResolvedValueOnce({ taskId: 'task-7', result: { queued: true } });
    const { app } = mountPage('?fid=fn.echo');
    const spies = spyOnMessage(app);
    await waitForFormReady();
    fireEvent.click(screen.getByRole('button', { name: 'eo-toggle-async' }));
    clickSend();
    await waitFor(() => expect(mockedInvoke).toHaveBeenCalledTimes(1));
    expect(mockedInvoke).toHaveBeenLastCalledWith(
      'fn.echo',
      { playerId: 'p-1', count: 2 },
      { route: 'lb', mode: 'async' },
    );
    await waitFor(() => expect(screen.getByTestId('tp-task-id')).toHaveTextContent('task-7'));
    expect(spies.success).toHaveBeenCalledWith('任务已创建：task-7');
  });

  it('异步无 taskId：按普通成功处理，不挂任务面板', async () => {
    mockedInvoke.mockResolvedValueOnce({ result: { ok: 1 } });
    const { app } = mountPage('?fid=fn.echo');
    const spies = spyOnMessage(app);
    await waitForFormReady();
    fireEvent.click(screen.getByRole('button', { name: 'eo-toggle-async' }));
    clickSend();
    await waitFor(() => expect(spies.success).toHaveBeenCalledWith('调用成功'));
    await act(async () => {});
    expect(screen.queryByTestId('task-progress')).toBeNull();
  });

  it('调用失败：提取 message 与 details、写错误历史、message.error', async () => {
    mockedInvoke.mockRejectedValueOnce(INVOKE_FAILURE);
    const { app } = mountPage('?fid=fn.echo');
    const spies = spyOnMessage(app);
    await waitForFormReady();
    clickSend();
    await waitFor(() => expect(screen.getByTestId('ir-error')).toHaveTextContent('请求参数无效'));
    expect(screen.getByTestId('ir-details')).toHaveTextContent('playerId');
    expect(screen.getByTestId('ir-details')).toHaveTextContent('不能为空');
    expect(spies.error).toHaveBeenCalledWith('请求参数无效');
    toggleHistory();
    await waitFor(() => expect(screen.getByTestId('rh-last-status')).toHaveTextContent('error'));
  });

  it('响应体为空对象（undefined）：按失败处理不崩溃（result?. 防御分支）', async () => {
    mockedInvoke.mockResolvedValueOnce(undefined);
    const { app } = mountPage('?fid=fn.echo');
    const spies = spyOnMessage(app);
    await waitForFormReady();
    clickSend();
    await waitFor(() => expect(spies.error).toHaveBeenCalled());
    await waitFor(() => expect(screen.getByTestId('ir-error').textContent).not.toBe(''));
  });
});

// ---------------------------------------------------------------------------
// 请求体编辑（双模式取值 / JSON→表单回写 / 格式化）
// ---------------------------------------------------------------------------

describe('函数调用工作台：请求体编辑', () => {
  it('JSON→表单值回写：合法对象同步；null/数组/标量/坏 JSON 不覆盖', async () => {
    mountPage('?fid=fn.echo');
    await waitFor(() =>
      expect(screen.getByTestId('rbe-formvalues')).toHaveTextContent('"playerId":"p-1"'),
    );
    const input = screen.getByTestId('rbe-json-input');
    fireEvent.change(input, { target: { value: '{"step": 5}' } });
    await waitFor(() => expect(screen.getByTestId('rbe-formvalues')).toHaveTextContent('"step":5'));

    for (const raw of ['null', '[1,2]', '"text"', '{bad']) {
      fireEvent.change(input, { target: { value: raw } });
      await waitFor(() => expect(screen.getByTestId('rbe-rawjson')).toHaveTextContent(raw));
      // 未同步：表单值保持上一次合法对象
      expect(screen.getByTestId('rbe-formvalues')).toHaveTextContent('"step":5');
    }
  });

  it('未选择函数时编辑 JSON：不回写表单值也不崩溃', async () => {
    mountPage();
    await waitFor(() => expect(screen.getByTestId('rbe-formstate')).toHaveTextContent('idle'));
    fireEvent.change(screen.getByTestId('rbe-json-input'), { target: { value: '{bad' } });
    await waitFor(() => expect(screen.getByTestId('rbe-rawjson')).toHaveTextContent('{bad'));
    expect(screen.getByTestId('rbe-formvalues')).toHaveTextContent('{}');
  });

  it('表单值变更回调：formValues 与 rawJson 同步', async () => {
    mountPage('?fid=fn.echo');
    await waitForFormReady();
    fireEvent.click(screen.getByRole('button', { name: 'rbe-form-values' }));
    await waitFor(() =>
      expect(screen.getByTestId('rbe-formvalues')).toHaveTextContent('"viaForm":1'),
    );
    expect(screen.getByTestId('rbe-rawjson')).toHaveTextContent('"viaForm": 1');
  });

  it('格式化：合法 JSON 压缩转缩进；非法 JSON 报错提示', async () => {
    const { app } = mountPage('?fid=fn.echo');
    const spies = spyOnMessage(app);
    await waitForFormReady();
    const input = screen.getByTestId('rbe-json-input');
    fireEvent.change(input, { target: { value: '{"a":1}' } });
    fireEvent.click(screen.getByRole('button', { name: 'rbe-format' }));
    await waitFor(() =>
      expect(screen.getByTestId('rbe-rawjson').textContent).toBe('{\n  "a": 1\n}'),
    );

    fireEvent.change(input, { target: { value: '{bad' } });
    fireEvent.click(screen.getByRole('button', { name: 'rbe-format' }));
    await waitFor(() => expect(spies.error).toHaveBeenCalledWith('请求体不是有效 JSON'));
  });

  it('表单模式取值三分支：formRef 空→校验失败；getValues 空回退 formValues；getValues 有值优先', async () => {
    mockedInvoke.mockResolvedValue({ result: { ok: 1 } });
    mountPage('?fid=fn.echo');
    await waitForFormReady();
    const toggleMode = () =>
      fireEvent.click(screen.getByRole('button', { name: 'rbe-toggle-mode' }));
    const waitForMode = (mode: string) =>
      waitFor(() => expect(screen.getByTestId('rbe-mode')).toHaveTextContent(mode));

    // 1) formRef.current 为 null：payload 回退 formValues，validate 缺失按失败处理
    toggleMode(); // json → form
    await waitForMode('form');
    clickSend();
    await waitFor(() => expect(screen.getByTestId('ir-error')).toHaveTextContent('表单校验失败'));
    expect(mockedInvoke).not.toHaveBeenCalled();

    // 2) getValues() 返回 undefined：回退 formValues（Schema 默认值）
    toggleMode(); // form → json
    await waitForMode('json');
    requestBodyMock.__setFormHandle({ getValues: () => undefined });
    toggleMode(); // json → form
    await waitForMode('form');
    clickSend();
    await waitFor(() => expect(mockedInvoke).toHaveBeenCalledTimes(1));
    expect(mockedInvoke).toHaveBeenNthCalledWith(
      1,
      'fn.echo',
      { playerId: 'p-1', count: 2 },
      { route: 'lb' },
    );

    // 3) getValues() 有值：优先于 formValues
    toggleMode(); // form → json
    await waitForMode('json');
    requestBodyMock.__setFormHandle({ getValues: () => ({ fromForm: 9 }), validate: () => true });
    toggleMode(); // json → form
    await waitForMode('form');
    clickSend();
    await waitFor(() => expect(mockedInvoke).toHaveBeenCalledTimes(2));
    expect(mockedInvoke).toHaveBeenNthCalledWith(2, 'fn.echo', { fromForm: 9 }, { route: 'lb' });
  });

  it('表单校验失败（validate 返回 false）：不发请求', async () => {
    mountPage('?fid=fn.echo');
    await waitForFormReady();
    fireEvent.click(screen.getByRole('button', { name: 'rbe-toggle-mode' }));
    await waitFor(() => expect(screen.getByTestId('rbe-mode')).toHaveTextContent('form'));
    requestBodyMock.__setFormHandle({ validate: () => false });
    // 触发一次重渲染让桩刷新 formRef
    toggleHistory();
    clickSend();
    await waitFor(() => expect(screen.getByTestId('ir-error')).toHaveTextContent('表单校验失败'));
    expect(mockedInvoke).not.toHaveBeenCalled();
  });

  it('getValues 抛非 Error 值：String(err) 分支渲染错误详情', async () => {
    mountPage('?fid=fn.echo');
    await waitForFormReady();
    fireEvent.click(screen.getByRole('button', { name: 'rbe-toggle-mode' }));
    await waitFor(() => expect(screen.getByTestId('rbe-mode')).toHaveTextContent('form'));
    requestBodyMock.__setFormHandle({
      getValues: () => {
        throw 'raw-failure';
      },
    });
    toggleHistory();
    clickSend();
    await waitFor(() =>
      expect(screen.getByTestId('ir-error')).toHaveTextContent('请求体不是有效 JSON：raw-failure'),
    );
    expect(mockedInvoke).not.toHaveBeenCalled();
  });

  it('JSON 模式发送非法 JSON：报错且不发请求', async () => {
    mountPage('?fid=fn.echo');
    await waitForFormReady();
    fireEvent.change(screen.getByTestId('rbe-json-input'), { target: { value: '{oops' } });
    clickSend();
    await waitFor(() =>
      expect(screen.getByTestId('ir-error')).toHaveTextContent('请求体不是有效 JSON'),
    );
    expect(mockedInvoke).not.toHaveBeenCalled();
  });
});

// ---------------------------------------------------------------------------
// 快捷键
// ---------------------------------------------------------------------------

describe('函数调用工作台：快捷键', () => {
  it('Cmd+Enter 触发执行；Ctrl+非 Enter 与裸 Enter 不触发', async () => {
    mockedInvoke.mockResolvedValue({ result: { ok: 1 } });
    mountPage('?fid=fn.echo');
    await waitForFormReady();

    fireEvent.keyDown(window, { key: 'Enter', metaKey: true });
    await waitFor(() => expect(mockedInvoke).toHaveBeenCalledTimes(1));

    fireEvent.keyDown(window, { key: 'Escape', ctrlKey: true });
    await act(async () => {});
    expect(mockedInvoke).toHaveBeenCalledTimes(1);

    fireEvent.keyDown(window, { key: 'Enter' });
    await act(async () => {});
    expect(mockedInvoke).toHaveBeenCalledTimes(1);
  });
});

// ---------------------------------------------------------------------------
// 审批流（A4）
// ---------------------------------------------------------------------------

describe('函数调用工作台：审批流', () => {
  it('approvalRequired + approvalId：进入轮询、info 提示、不写成功历史、附审批中心链接', async () => {
    mockedInvoke.mockResolvedValueOnce({
      approvalRequired: true,
      approvalId: 'ap-1',
      traceId: 'tr-a',
    });
    const { app } = mountPage('?fid=fn.echo');
    const spies = spyOnMessage(app);
    await waitForFormReady();
    clickSend();
    await waitFor(() => expect(pollingMock.startApprovalPolling).toHaveBeenCalledTimes(1));
    expect(pollingMock.__captured.approvalId).toBe('ap-1');
    expect(pollingMock.__captured.interval).toBe(10000);
    expect(spies.info).toHaveBeenCalledWith('该操作需要审批，已提交审批流程');
    expect(screen.getByText('审批中：该操作需要审批通过后才会执行')).toBeInTheDocument();
    expect(document.querySelector('.ant-alert-warning')).not.toBeNull();
    const link = screen.getByText('前往审批中心查看').closest('a');
    expect(link).toHaveAttribute('href', '/approvals?approvalId=ap-1');
    // 不写成功历史
    toggleHistory();
    await waitFor(() => expect(screen.getByTestId('rh-count')).toHaveTextContent('0'));
  });

  it('轮询 fetcher：透传 queryApprovalStatus 完整结果（含续跑字段，不再裁剪）', async () => {
    mockedInvoke.mockResolvedValueOnce({ approvalRequired: true, approvalId: 'ap-4' });
    mountPage('?fid=fn.echo');
    await waitForFormReady();
    clickSend();
    await waitFor(() => expect(pollingMock.__captured.fetcher).not.toBeNull());
    const detail = {
      approvalId: 'ap-4',
      status: 'approved',
      continuation: true,
      resultKind: 'sync',
      result: { ok: 7 },
    } as unknown as Awaited<ReturnType<typeof queryApprovalStatus>>;
    mockedQueryApproval.mockResolvedValueOnce(detail);
    let mapped: unknown;
    await act(async () => {
      mapped = await pollingMock.__captured.fetcher?.('ap-4');
    });
    expect(mockedQueryApproval).toHaveBeenCalledWith('ap-4');
    expect(mapped).toEqual(detail);
  });

  it('approved 未自动续跑：提示可重新发起 + 保留重新调用按钮（此时无副作用发生，重放安全）', async () => {
    mockedInvoke
      .mockResolvedValueOnce({ approvalRequired: true, approvalId: 'ap-2' })
      .mockResolvedValueOnce({ result: { ok: 1 } });
    const { app } = mountPage('?fid=fn.echo');
    const spies = spyOnMessage(app);
    await waitForFormReady();
    clickSend();
    const onUpdate = await waitForPollingUpdate();
    act(() => onUpdate({ status: 'approved' }));
    await waitFor(() =>
      expect(screen.getByText('审批已通过：未自动续跑，可重新发起调用')).toBeInTheDocument(),
    );
    expect(document.querySelector('.ant-alert-success')).not.toBeNull();
    expect(pollingMock.__unsubscribe).toHaveBeenCalled();

    fireEvent.click(screen.getByRole('button', { name: /重新调用/ }));
    await waitFor(() => expect(mockedInvoke).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(spies.success).toHaveBeenCalledWith('调用成功'));
    await waitFor(() => expect(screen.queryByText(/审批已通过/)).toBeNull());
  });

  it('approved + continuation(sync)：直接展示续跑结果，不出现重新调用按钮（防二次副作用）', async () => {
    mockedInvoke.mockResolvedValueOnce({ approvalRequired: true, approvalId: 'ap-c1' });
    mountPage('?fid=fn.echo');
    await waitForFormReady();
    clickSend();
    const onUpdate = await waitForPollingUpdate();
    act(() =>
      onUpdate({
        status: 'approved',
        continuation: true,
        resultKind: 'sync',
        result: { banId: 'b-1' },
      }),
    );
    await waitFor(() =>
      expect(screen.getByText('审批已通过：服务端已按原请求自动续跑执行')).toBeInTheDocument(),
    );
    expect(screen.queryByText('审批已通过：未自动续跑，可重新发起调用')).toBeNull();
    expect(screen.queryByRole('button', { name: /重新调用/ })).toBeNull();
    expect(screen.getByTestId('ir-response-raw')).toHaveTextContent('banId');
  });

  it('approved + continuation(task)：任务面板接管续跑任务，不出现重新调用按钮', async () => {
    mockedInvoke.mockResolvedValueOnce({ approvalRequired: true, approvalId: 'ap-c2' });
    mountPage('?fid=fn.echo');
    await waitForFormReady();
    clickSend();
    expect(screen.queryByTestId('task-progress')).toBeNull();
    const onUpdate = await waitForPollingUpdate();
    act(() =>
      onUpdate({
        status: 'approved',
        continuation: true,
        resultKind: 'task',
        taskId: 't-cont',
      }),
    );
    await waitFor(() => expect(screen.getByTestId('task-progress')).toBeInTheDocument());
    expect(screen.getByTestId('tp-task-id')).toHaveTextContent('t-cont');
    expect(screen.queryByRole('button', { name: /重新调用/ })).toBeNull();
  });

  it('rejected（含/不含原因）与 expired 文案与告警类型', async () => {
    mockedInvoke.mockResolvedValue({ approvalRequired: true, approvalId: 'ap-3' });
    mountPage('?fid=fn.echo');
    await waitForFormReady();
    clickSend();
    const onUpdate = await waitForPollingUpdate();

    act(() => onUpdate({ status: 'rejected', reason: '风控拦截' }));
    await waitFor(() => expect(screen.getByText('审批已拒绝：风控拦截')).toBeInTheDocument());
    expect(document.querySelector('.ant-alert-error')).not.toBeNull();

    act(() => onUpdate({ status: 'expired' }));
    await waitFor(() => expect(screen.getByText('审批已过期')).toBeInTheDocument());
    expect(document.querySelector('.ant-alert-warning')).not.toBeNull();

    act(() => onUpdate({ status: 'rejected' }));
    await waitFor(() => expect(screen.getByText('审批已拒绝')).toBeInTheDocument());
  });

  it('approvalRequired 缺 approvalId：按普通成功处理', async () => {
    mockedInvoke.mockResolvedValueOnce({ approvalRequired: true, result: { ok: 2 } });
    const { app } = mountPage('?fid=fn.echo');
    const spies = spyOnMessage(app);
    await waitForFormReady();
    clickSend();
    await waitFor(() => expect(spies.success).toHaveBeenCalledWith('调用成功'));
    expect(pollingMock.startApprovalPolling).not.toHaveBeenCalled();
    expect(screen.queryByText(/审批中/)).toBeNull();
  });

  it('审批待定被历史恢复清空后：后续 onUpdate 忽略（prev 为 null）', async () => {
    getItemMock().mockReturnValue(JSON.stringify([SEEDED_HISTORY_ITEM]));
    mockedInvoke.mockResolvedValueOnce({ approvalRequired: true, approvalId: 'ap-5' });
    mountPage('?fid=fn.echo');
    await waitForFormReady();
    clickSend();
    const onUpdate = await waitForPollingUpdate();
    await waitFor(() =>
      expect(screen.getByText('审批中：该操作需要审批通过后才会执行')).toBeInTheDocument(),
    );

    toggleHistory();
    fireEvent.click(screen.getByRole('button', { name: 'rh-select-first' }));
    await waitFor(() =>
      expect(screen.queryByText('审批中：该操作需要审批通过后才会执行')).toBeNull(),
    );
    act(() => onUpdate({ status: 'rejected' }));
    await act(async () => {});
    expect(screen.queryByText(/审批已拒绝/)).toBeNull();
  });
});

// ---------------------------------------------------------------------------
// 异步任务面板回调
// ---------------------------------------------------------------------------

describe('函数调用工作台：任务面板完成回调', () => {
  it('onCompleted(null)：响应与请求体置空占位；onCompleted(result)：回填', async () => {
    mockedInvoke.mockResolvedValueOnce({ taskId: 'task-9', result: { queued: true } });
    mountPage('?fid=fn.echo');
    await waitForFormReady();
    fireEvent.click(screen.getByRole('button', { name: 'eo-toggle-async' }));
    clickSend();
    await waitFor(() => expect(screen.getByTestId('tp-task-id')).toHaveTextContent('task-9'));

    fireEvent.click(screen.getByRole('button', { name: 'tp-complete-null' }));
    await waitFor(() => expect(screen.getByTestId('ir-response-raw').textContent).toBe(''));
    await waitFor(() => expect(screen.getByTestId('rbe-rawjson').textContent).toBe('null'));

    fireEvent.click(screen.getByRole('button', { name: 'tp-complete' }));
    await waitFor(() =>
      expect(screen.getByTestId('ir-response-raw')).toHaveTextContent('"done": true'),
    );
  });
});

// ---------------------------------------------------------------------------
// 历史面板与恢复
// ---------------------------------------------------------------------------

describe('函数调用工作台：历史面板与恢复', () => {
  it('历史开关：默认隐藏，打开后本地/服务端双 Tab，清空写回空数组', async () => {
    const { rerenderPage } = mountPage();
    await act(async () => {});
    expect(screen.queryByTestId('request-history')).toBeNull();

    toggleHistory();
    await waitFor(() => expect(screen.getByTestId('request-history')).toBeInTheDocument());
    expect(screen.getByText('本地草稿')).toBeInTheDocument();
    expect(screen.getByText('服务端记录')).toBeInTheDocument();

    // 未选择函数：服务端面板 functionId 为空
    fireEvent.click(screen.getByText('服务端记录'));
    await waitFor(() =>
      expect(screen.getByTestId('server-history')).toHaveTextContent('no-function'),
    );

    // 选中函数后服务端面板携带 fid
    rerenderPage('?fid=fn.echo');
    await waitForFormReady();
    await waitFor(() => expect(screen.getByTestId('server-history')).toHaveTextContent('fn.echo'));

    // 清空本地历史
    fireEvent.click(screen.getByText('本地草稿'));
    await waitFor(() => expect(screen.getByTestId('request-history')).toBeInTheDocument());
    fireEvent.click(screen.getByRole('button', { name: 'rh-clear' }));
    await waitFor(() => expect(screen.getByTestId('rh-count')).toHaveTextContent('0'));
    expect(setItemMock()).toHaveBeenCalledWith(HISTORY_KEY, '[]');

    toggleHistory();
    await waitFor(() => expect(screen.queryByTestId('request-history')).toBeNull());
  });

  it('恢复成功条目：切 JSON 模式回填请求体/响应/路由，error 留空', async () => {
    mockedInvoke.mockResolvedValueOnce({ traceId: 'tr-9', result: { ok: true } });
    mountPage('?fid=fn.echo');
    await waitForFormReady();
    clickSend();
    await waitFor(() => expect(mockedInvoke).toHaveBeenCalledTimes(1));

    toggleHistory();
    fireEvent.click(screen.getByRole('button', { name: 'rh-select-first' }));
    await waitFor(() => expect(mockedPush).toHaveBeenCalledWith('/functions/invoke?fid=fn.echo'));
    expect(screen.getByTestId('rbe-mode')).toHaveTextContent('json');
    expect(screen.getByTestId('rbe-rawjson')).toHaveTextContent('"playerId": "p-1"');
    expect(screen.getByTestId('ir-response-raw')).toHaveTextContent('"ok": true');
    expect(screen.getByTestId('ir-error').textContent).toBe('');
    expect(screen.getByTestId('eo-route')).toHaveTextContent('lb');
    expect(screen.getByTestId('eo-target').textContent).toBe('');
    expect(screen.getByTestId('eo-hash').textContent).toBe('');
  });

  it('恢复失败条目：hash/targeted 选项与错误信息回填；seed 空 options 回退 lb', async () => {
    mockedInvoke.mockRejectedValue(INVOKE_FAILURE);
    mountPage('?fid=fn.echo');
    await waitForFormReady();
    // targeted + service_id 失败调用
    fireEvent.click(screen.getByRole('button', { name: 'eo-route-targeted' }));
    fireEvent.click(screen.getByRole('button', { name: 'eo-set-target' }));
    clickSend();
    await waitFor(() => expect(screen.getByTestId('ir-error')).toHaveTextContent('请求参数无效'));
    // hash + key 失败调用
    fireEvent.click(screen.getByRole('button', { name: 'eo-route-hash' }));
    fireEvent.click(screen.getByRole('button', { name: 'eo-set-hash' }));
    clickSend();
    await waitFor(() => expect(mockedInvoke).toHaveBeenCalledTimes(2));

    toggleHistory();
    // items[0] = hash 调用
    fireEvent.click(screen.getByRole('button', { name: 'rh-select-first' }));
    await waitFor(() => expect(screen.getByTestId('eo-route')).toHaveTextContent('hash'));
    expect(screen.getByTestId('eo-hash')).toHaveTextContent('player:1');
    expect(screen.getByTestId('ir-error')).toHaveTextContent('请求参数无效');
    expect(screen.getByTestId('ir-duration').textContent).not.toBe('');

    // items[1] = targeted 调用
    fireEvent.click(screen.getByRole('button', { name: 'rh-select-last' }));
    await waitFor(() => expect(screen.getByTestId('eo-route')).toHaveTextContent('targeted'));
    expect(screen.getByTestId('eo-target')).toHaveTextContent('svc-9');
  });

  it('seed 空 options 历史条目：恢复时路由回退 lb', async () => {
    getItemMock().mockReturnValue(JSON.stringify([SEEDED_HISTORY_ITEM]));
    mountPage();
    await act(async () => {});
    toggleHistory();
    fireEvent.click(screen.getByRole('button', { name: 'rh-select-first' }));
    await waitFor(() => expect(screen.getByTestId('eo-route')).toHaveTextContent('lb'));
    expect(mockedPush).toHaveBeenCalledWith('/functions/invoke?fid=fn.bare');
    expect(screen.getByTestId('rbe-rawjson')).toHaveTextContent('"seeded": true');
    expect(screen.getByTestId('ir-response-raw')).toHaveTextContent('"seededResponse": 1');
  });
});

// ---------------------------------------------------------------------------
// 结果面板
// ---------------------------------------------------------------------------

describe('函数调用工作台：结果面板复制回调', () => {
  it('onCopy：clipboard.writeText 成功后提示已复制', async () => {
    const writeText = jest.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, 'clipboard', {
      value: { writeText },
      configurable: true,
    });
    const { app } = mountPage('?fid=fn.echo');
    const spies = spyOnMessage(app);
    await waitForFormReady();
    fireEvent.click(screen.getByRole('button', { name: 'ir-copy' }));
    await waitFor(() => expect(writeText).toHaveBeenCalledWith('copy-me'));
    await waitFor(() => expect(spies.success).toHaveBeenCalledWith('已复制'));
  });
});
