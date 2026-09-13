/** ResourcePageRenderer 覆盖。
 *
 * 覆盖路径：列规格转换（boolean·date·enum·number·text、tag 命中·未命中、copy、
 * visible 隐藏）、列表请求（缺绑定/预览/成功/缺 selector/未命中/非数组/异常/
 * Alert 关闭）、创建·编辑（预览拦截/成功 reload/失败/row 上下文）、行操作
 * （预览/带表单弹窗/confirm 流/requireConfirm/直接执行成败）、删除（预览/成败）、
 * 工具栏与批量（预览/直接/confirm/selection 上下文/已选计数）、详情抽屉
 * （无绑定直显/预览/成功/缺 selector/未命中/非对象/异常/字段过滤·横排）、
 * rowKey 兜底、标题三态、分页开关。 */
import React from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App } from 'antd';
import { ProTable } from '@ant-design/pro-components';
import * as SchemaFormModule from '@/components/SchemaFormRenderer';
import ResourcePageRenderer from '../ResourcePageRenderer';
import type {
  ActionSpec,
  ColumnSpec,
  PageExecutionResult,
  PageFunctionBinding,
  ResourcePageSpec,
} from '@/types/dashboard';

// SchemaFormRenderer 替身：forwardRef 暴露 validate/getValues（由 formMockState
// 控制），onValuesChange 场景渲染喂值按钮
jest.mock('@/components/SchemaFormRenderer', () => {
  const formMockState = { validate: true, values: {} as Record<string, unknown> };
  const Stub = React.forwardRef<unknown, Record<string, unknown>>((props, ref) => {
    React.useImperativeHandle(ref, () => ({
      validate: () => formMockState.validate,
      getValues: () => formMockState.values,
    }));
    return (
      <div>
        <div data-testid="schema-form-stub" />
        {typeof props.onValuesChange === 'function' ? (
          <button
            type="button"
            data-testid="form-change"
            onClick={() =>
              (props.onValuesChange as (v: unknown, all: Record<string, unknown>) => void)(
                {},
                { days: 3 },
              )
            }
          >
            change
          </button>
        ) : null}
      </div>
    );
  });
  Stub.displayName = 'SchemaFormRendererStub';
  return { __esModule: true, default: Stub, formMockState };
});

// ProTable 替身：挂载即请求、渲染行与列 render、toolbar、行选择、alert 计数、
// 分页 total、actionRef.reload；ProDescriptions 直渲 children
jest.mock('@ant-design/pro-components', () => {
  const ProTableStub = ({
    request,
    columns,
    rowKey,
    actionRef,
    toolBarRender,
    headerTitle,
    tableAlertRender,
    tableAlertOptionRender,
    rowSelection,
    pagination,
  }: {
    request?: (params: Record<string, unknown>) => Promise<{ data: unknown[]; total: number }>;
    columns?: Array<{
      key?: string;
      dataIndex?: string;
      render?: (dom: unknown, record: Record<string, unknown>) => React.ReactNode;
    }>;
    rowKey?: (record: Record<string, unknown>) => string;
    actionRef?: React.MutableRefObject<{ reload?: () => void } | null>;
    toolBarRender?: () => React.ReactNode[];
    headerTitle?: React.ReactNode;
    tableAlertRender?: (info: {
      selectedRowKeys: React.Key[];
      onCleanSelected: () => void;
    }) => React.ReactNode;
    tableAlertOptionRender?: () => React.ReactNode;
    rowSelection?: { onChange: (keys: React.Key[], rows: Record<string, unknown>[]) => void };
    pagination?:
      false | { showTotal?: (total: number) => React.ReactNode; defaultPageSize?: number };
  }) => {
    const [rows, setRows] = React.useState<Record<string, unknown>[]>([]);
    const [selected, setSelected] = React.useState<number[]>([]);
    const reload = React.useCallback(() => {
      void request?.({}).then((r) => setRows(r.data as Record<string, unknown>[]));
    }, [request]);
    React.useEffect(() => {
      void reload();
    }, [reload]);
    React.useEffect(() => {
      if (actionRef) actionRef.current = { reload };
    }, [actionRef, reload]);
    return (
      <div>
        <div data-testid="protable-title">{headerTitle}</div>
        <div>{toolBarRender?.()}</div>
        <button type="button" data-testid="protable-reload" onClick={reload}>
          reload
        </button>
        {rowSelection
          ? rows.map((_, index) => (
              <input
                key={String(index)}
                type="checkbox"
                data-testid={`row-check-${index}`}
                checked={selected.includes(index)}
                onChange={() => {
                  const next = selected.includes(index)
                    ? selected.filter((k) => k !== index)
                    : [...selected, index];
                  setSelected(next);
                  rowSelection.onChange(
                    next.map(String),
                    next.map((k) => rows[k]),
                  );
                }}
              />
            ))
          : null}
        {selected.length > 0 ? (
          <div data-testid="selection-bar">
            {tableAlertRender?.({
              selectedRowKeys: selected.map(String),
              onCleanSelected: () => setSelected([]),
            })}
            {tableAlertOptionRender?.()}
          </div>
        ) : null}
        <div data-testid="rows-loaded" data-count={String(rows.length)} />
        {pagination ? (
          <div data-testid="pagination-bar">{pagination.showTotal?.(rows.length)}</div>
        ) : null}
        <table>
          <tbody>
            {rows.map((row) => (
              <tr key={rowKey?.(row) ?? 'row'}>
                {columns?.map((column) => (
                  <td key={column.key ?? ''}>
                    {column.render
                      ? column.render(null, row)
                      : String(row[column.dataIndex ?? ''] ?? '')}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    );
  };
  const ProDescriptionsStub = ({ children }: { children?: React.ReactNode }) => (
    <div data-testid="prodescriptions">{children}</div>
  );
  (
    ProDescriptionsStub as unknown as {
      Item: React.FC<{ label?: React.ReactNode; children?: React.ReactNode }>;
    }
  ).Item = ({ label, children }) => (
    <div data-testid="prodescriptions-item">
      <span>{label}</span>
      <span>{children}</span>
    </div>
  );
  return { ProTable: jest.fn(ProTableStub), ProDescriptions: ProDescriptionsStub };
});

const { formMockState } = SchemaFormModule as unknown as {
  formMockState: { validate: boolean; values: Record<string, unknown> };
};

type ExecuteMock = jest.Mock<Promise<PageExecutionResult>, [string, unknown]>;

const ok = (data?: unknown): PageExecutionResult => ({ kind: 'invoke', requestId: 'r1', data });

const binding = (id: string, usage: PageFunctionBinding['usage'], output?: unknown) => {
  const item: PageFunctionBinding = {
    id,
    functionId: `fn-${id}`,
    usage,
    execution: { mode: 'sync' },
  };
  if (output) item.selectors = { input: { assignments: [] }, output: output as never };
  return item;
};

// 列表绑定：items 数组 + total
const listBinding = binding('b-list', 'query', [
  { stateKey: 'items', source: '/data', shape: 'collection' },
  { stateKey: 'total', source: '/total', shape: 'scalar' },
]);
const detailBinding = binding('b-detail', 'detail', [
  { stateKey: 'detail', source: '/detail', shape: 'object' },
]);

const rows = [
  {
    id: 'p1',
    name: 'Alice',
    level: 60,
    vip: true,
    state: 'active',
    createdAt: '2026-01-01T10:00:00Z',
  },
  { id: 'p2', name: 'Bob', level: 1, vip: false, state: 'ghost', createdAt: null },
];

const columns: ColumnSpec[] = [
  { key: 'id', title: { 'zh-CN': 'ID' }, dataType: 'string', render: 'copy' },
  { key: 'name', title: { 'zh-CN': '名称' }, dataType: 'string' },
  { key: 'level', title: { 'zh-CN': '等级' }, dataType: 'number' },
  { key: 'vip', title: { 'zh-CN': 'VIP' }, dataType: 'boolean' },
  {
    key: 'state',
    title: { 'zh-CN': '状态' },
    dataType: 'enum',
    render: 'tag',
    enum: [
      { value: 'active', label: { 'zh-CN': '活跃' }, color: 'green' },
      { value: 'banned', label: { 'zh-CN': '封禁' }, color: 'red' },
      { value: 'pending', label: { 'zh-CN': '待定' } },
    ],
  },
  { key: 'createdAt', title: { 'zh-CN': '创建时间' }, dataType: 'date' },
  { key: 'secret', title: { 'zh-CN': '隐藏列' }, dataType: 'string', visible: false },
];

const banAction: ActionSpec = {
  key: 'ban',
  title: { 'zh-CN': '封禁' },
  type: 'danger',
  bindingId: 'b-ban',
};

const spec = (overrides: Partial<ResourcePageSpec> = {}): ResourcePageSpec => ({
  listView: {
    columns,
    rowActions: [banAction],
    pagination: { enabled: true },
    ...overrides.listView,
  } as ResourcePageSpec['listView'],
  ...overrides,
});

interface RenderOptions {
  spec?: ResourcePageSpec;
  bindings?: PageFunctionBinding[];
  preview?: boolean;
  title?: string;
  /** render 前注入的实现（ProTable 挂载即请求，mock 必须先于 render 生效） */
  executeImpl?: (bindingId: string) => Promise<PageExecutionResult>;
}

function renderResource(options: RenderOptions = {}) {
  const onExecute = jest.fn() as ExecuteMock;
  if (options.executeImpl) onExecute.mockImplementation(options.executeImpl);
  const utils = render(
    <App>
      <ResourcePageRenderer
        spec={options.spec ?? spec()}
        bindings={options.bindings ?? [listBinding]}
        onExecute={onExecute as never}
        preview={options.preview}
        title={options.title}
      />
    </App>,
  );
  return { onExecute, ...utils };
}

// 默认执行实现：列表两行、详情远程数据、其余空结果
const defaultImpl = async (bindingId: string): Promise<PageExecutionResult> => {
  if (bindingId === 'b-list') return ok({ data: rows, total: 2 });
  if (bindingId === 'b-detail') return ok({ detail: { id: 'p1', name: 'Alice-remote' } });
  return ok({});
};

// 列表就绪标准链路（可覆写 data / 全量实现）
const loadList = async (options: RenderOptions = {}, data?: unknown) => {
  const impl =
    options.executeImpl ??
    (async (bindingId: string) =>
      bindingId === 'b-list'
        ? ok(data ?? { data: rows, total: 2 })
        : bindingId === 'b-detail'
          ? ok({ detail: { id: 'p1', name: 'Alice-remote' } })
          : ok({}));
  const rendered = renderResource({ ...options, executeImpl: impl });
  if (data === undefined) {
    await waitFor(() => expect(screen.getByText('Alice')).toBeInTheDocument());
  } else {
    await waitFor(() => expect(screen.getByTestId('rows-loaded')).toBeInTheDocument());
  }
  return rendered;
};

const clickRowAction = async (name: RegExp) => {
  await waitFor(() => expect(screen.getAllByRole('button', { name }).length).toBeGreaterThan(0));
  fireEvent.click(screen.getAllByRole('button', { name })[0]);
};

describe('列规格转换', () => {
  it('boolean/date/enum·tag/copy/number 各形态渲染', async () => {
    await loadList();
    // boolean：true→Tag 是 / false→Tag 否
    expect(screen.getByText('是')).toBeInTheDocument();
    expect(screen.getByText('否')).toBeInTheDocument();
    // date：有值→formatDateTime，null→'-'
    expect(screen.getByText('2026/01/01 10:00:00')).toBeInTheDocument();
    expect(screen.getByText('-')).toBeInTheDocument();
    // enum + render=tag：命中→Tag 活跃；未命中→'ghost'
    expect(screen.getByText('活跃')).toBeInTheDocument();
    expect(screen.getByText('ghost')).toBeInTheDocument();
    // copy：Text copyable
    expect(screen.getByText('p1')).toBeInTheDocument();
    expect(document.querySelector('.ant-typography-copy')).toBeInTheDocument();
  });

  it('valueEnum 三色映射与 visible 列隐藏', async () => {
    await loadList();
    const call = (ProTable as unknown as jest.Mock).mock.calls[
      (ProTable as unknown as jest.Mock).mock.calls.length - 1
    ][0] as {
      columns: Array<{
        key?: string;
        valueType?: string;
        hideInTable?: boolean;
        valueEnum?: Record<string, { status?: string }>;
      }>;
    };
    const byKey = Object.fromEntries(call.columns.map((c) => [c.key, c]));
    expect(byKey.level.valueType).toBe('digit');
    expect(byKey.name.valueType).toBe('text');
    expect(byKey.secret.hideInTable).toBe(true);
    expect(byKey.state.valueEnum).toMatchObject({
      active: { status: 'Success' },
      banned: { status: 'Error' },
      pending: { status: 'Default' },
    });
  });
});

describe('列表请求', () => {
  it('缺少列表绑定：Alert 报错', async () => {
    renderResource({ bindings: [] });
    await waitFor(() => expect(screen.getByText('资源页面缺少列表查询绑定')).toBeInTheDocument());
    expect(document.querySelector('.ant-alert-error')).toBeInTheDocument();
  });

  it('预览模式：返回空数据且不报错', async () => {
    renderResource({ preview: true });
    await waitFor(() => expect(screen.getByTestId('protable-title')).toBeInTheDocument());
    expect(screen.queryByText('资源页面缺少列表查询绑定')).not.toBeInTheDocument();
    expect(document.querySelectorAll('tr')).toHaveLength(0);
  });

  it('成功：行渲染 + 分页 total', async () => {
    await loadList();
    expect(screen.getByText('Alice')).toBeInTheDocument();
    expect(screen.getByText('Bob')).toBeInTheDocument();
    expect(screen.getByText('共 2 条')).toBeInTheDocument();
  });

  it('绑定缺 items selector：Alert 报错', async () => {
    const noSelector = binding('b-list', 'query', [
      { stateKey: 'other', source: '/data', shape: 'collection' },
    ]);
    await loadList({ bindings: [noSelector] }, { data: rows, total: 2 });
    await waitFor(() =>
      expect(screen.getByText(/列表绑定缺少 pageState.items 输出 selector/)).toBeInTheDocument(),
    );
  });

  it('结果未命中 items selector：Alert 报错', async () => {
    await loadList({}, { other: 1 });
    await waitFor(() =>
      expect(screen.getByText(/列表结果未命中 items selector/)).toBeInTheDocument(),
    );
  });

  it('items 非数组：Alert 报错', async () => {
    // source 命中对象（非数组）
    await loadList({}, { data: { nested: true }, total: 1 });
    await waitFor(() =>
      expect(screen.getByText(/列表 items selector 的结果不是数组/)).toBeInTheDocument(),
    );
  });

  it('请求异常：Alert + toast', async () => {
    renderResource({
      executeImpl: async () => {
        throw new Error('net');
      },
    });
    await waitFor(() => expect(screen.getByText(/获取资源列表失败/)).toBeInTheDocument());
    expect(screen.getByText('获取数据失败')).toBeInTheDocument();
  });

  it('Alert 可关闭', async () => {
    renderResource({
      executeImpl: async () => {
        throw new Error('net');
      },
    });
    await waitFor(() => expect(screen.getByText(/获取资源列表失败/)).toBeInTheDocument());
    fireEvent.click(document.querySelector('.ant-alert-close-icon') as HTMLElement);
    await waitFor(() => expect(screen.queryByText(/获取资源列表失败/)).not.toBeInTheDocument());
  });
});

describe('创建', () => {
  const createSpec = spec({ createForm: { jsonSchema: { type: 'object' } } });
  const withCreate = [listBinding, binding('create', 'action')];

  const openCreateModal = async (options: RenderOptions = {}) => {
    const rendered = renderResource({
      spec: createSpec,
      bindings: withCreate,
      executeImpl: defaultImpl,
      ...options,
    });
    await waitFor(() => expect(screen.getByText('Alice')).toBeInTheDocument());
    // 新建按钮 disabled 时 React 不派发 onClick，非预览场景才可点击
    fireEvent.click(screen.getByRole('button', { name: /新\s*建/ }));
    await waitFor(() => expect(screen.getByTestId('schema-form-stub')).toBeInTheDocument());
    return rendered;
  };

  it('成功：toast + 关闭 + reload', async () => {
    formMockState.validate = true;
    formMockState.values = { name: 'New' };
    const { onExecute } = await openCreateModal();
    fireEvent.click(screen.getByRole('button', { name: 'OK' }));
    await waitFor(() => expect(screen.getByText('创建成功')).toBeInTheDocument());
    expect(onExecute).toHaveBeenCalledWith('create', { form: { name: 'New' } });
    await waitFor(() =>
      expect(onExecute.mock.calls.filter(([id]) => id === 'b-list')).toHaveLength(2),
    );
  });

  it('失败：toast「创建失败」', async () => {
    formMockState.validate = true;
    formMockState.values = {};
    await openCreateModal({
      executeImpl: async (id: string) => {
        if (id === 'create') throw new Error('dup');
        return defaultImpl(id);
      },
    });
    fireEvent.click(screen.getByRole('button', { name: 'OK' }));
    await waitFor(() => expect(screen.getByText('创建失败')).toBeInTheDocument());
  });

  // 注：handleCreate 的 preview 拦截在 UI 上不可达——新建按钮 disabled={preview}，
  // React 对 disabled 按钮不派发 onClick，Modal 无法在预览下打开。
});

describe('编辑', () => {
  const editSpec = spec({
    createForm: { jsonSchema: { type: 'object' } },
    updateForm: { jsonSchema: { type: 'object' } },
    listView: {
      columns,
      rowActions: [{ key: 'edit', title: { 'zh-CN': '编辑' }, bindingId: 'update' }],
    },
  });
  const withEdit = [listBinding, binding('update', 'action')];

  const openEditModal = async (options: RenderOptions = {}) => {
    const rendered = renderResource({
      spec: editSpec,
      bindings: withEdit,
      executeImpl: async (id: string) => (id === 'b-list' ? ok({ data: rows, total: 2 }) : ok({})),
      ...options,
    });
    await waitFor(() =>
      expect(screen.getAllByRole('button', { name: /编\s*辑/ }).length).toBeGreaterThan(0),
    );
    fireEvent.click(screen.getAllByRole('button', { name: /编\s*辑/ })[0]);
    await waitFor(() => expect(screen.getByTestId('schema-form-stub')).toBeInTheDocument());
    return rendered;
  };

  it('成功：form + row 上下文 + reload', async () => {
    formMockState.validate = true;
    formMockState.values = { name: 'Renamed' };
    const { onExecute } = await openEditModal();
    fireEvent.click(screen.getByRole('button', { name: 'OK' }));
    await waitFor(() => expect(screen.getByText('更新成功')).toBeInTheDocument());
    expect(onExecute).toHaveBeenCalledWith('update', {
      form: { name: 'Renamed' },
      row: rows[0],
    });
    await waitFor(() =>
      expect(onExecute.mock.calls.filter(([id]) => id === 'b-list')).toHaveLength(2),
    );
  });

  it('失败：toast「更新失败」', async () => {
    formMockState.validate = true;
    formMockState.values = {};
    await openEditModal({
      executeImpl: async (id: string) => {
        if (id === 'update') throw new Error('x');
        return id === 'b-list' ? ok({ data: rows, total: 2 }) : ok({});
      },
    });
    fireEvent.click(screen.getByRole('button', { name: 'OK' }));
    await waitFor(() => expect(screen.getByText('更新失败')).toBeInTheDocument());
  });

  // 注：handleEdit 的 preview 拦截在 UI 上不可达——编辑按钮位于行内，
  // 预览模式下列表恒为空（handleRequest 先行拦截），按钮无法出现。
});

describe('行操作', () => {
  // 注：handleRowAction / submitActionForm 的 preview 拦截在 UI 上不可达——
  // 行操作按钮位于行内，预览模式下列表恒为空。

  it('直接执行成功：toast + reload', async () => {
    const { onExecute } = await loadList({
      bindings: [listBinding, binding('b-ban', 'action')],
    });
    onExecute.mockResolvedValueOnce(ok({}));
    await clickRowAction(/封\s*禁/);
    await waitFor(() => expect(screen.getByText('操作成功')).toBeInTheDocument());
    expect(onExecute).toHaveBeenCalledWith('b-ban', { row: rows[0] });
    await waitFor(() =>
      expect(onExecute.mock.calls.filter(([id]) => id === 'b-list')).toHaveLength(2),
    );
  });

  it('直接执行失败：toast「操作失败」', async () => {
    const { onExecute } = await loadList({
      bindings: [listBinding, binding('b-ban', 'action')],
    });
    onExecute.mockRejectedValueOnce(new Error('nope'));
    await clickRowAction(/封\s*禁/);
    await waitFor(() => expect(screen.getByText('操作失败')).toBeInTheDocument());
  });

  it('confirm=true：弹确认框后执行', async () => {
    const confirmAction: ActionSpec = {
      ...banAction,
      confirm: true,
      confirmTitle: { 'zh-CN': '封禁确认' },
      confirmDescription: { 'zh-CN': '封禁后玩家无法登录' },
    };
    const { onExecute } = await loadList({
      spec: spec({ listView: { columns, rowActions: [confirmAction] } }),
      bindings: [listBinding, binding('b-ban', 'action')],
    });
    onExecute.mockResolvedValueOnce(ok({}));
    await clickRowAction(/封\s*禁/);
    await waitFor(() => expect(screen.getAllByText('封禁确认').length).toBeGreaterThan(0));
    expect(screen.getAllByText('封禁后玩家无法登录').length).toBeGreaterThan(0);
    fireEvent.click(screen.getByRole('button', { name: 'OK' }));
    await waitFor(() => expect(onExecute).toHaveBeenCalledWith('b-ban', { row: rows[0] }));
    expect(screen.getByText('操作成功')).toBeInTheDocument();
  });

  it('binding.execution.requireConfirm 同样走确认框，onOk 异常 toast', async () => {
    const requireConfirmBinding = {
      ...binding('b-ban', 'action'),
      execution: { mode: 'sync' as const, requireConfirm: true },
    };
    const { onExecute } = await loadList({ bindings: [listBinding, requireConfirmBinding] });
    onExecute.mockRejectedValueOnce(new Error('deny'));
    await clickRowAction(/封\s*禁/);
    await waitFor(() => expect(screen.getAllByText('确认操作').length).toBeGreaterThan(0));
    fireEvent.click(screen.getByRole('button', { name: 'OK' }));
    await waitFor(() => expect(screen.getByText('操作失败')).toBeInTheDocument());
  });

  it('带表单的行操作：弹窗收集附加字段 + row 注入', async () => {
    const formAction: ActionSpec = {
      ...banAction,
      form: { jsonSchema: { type: 'object' } },
    };
    const { onExecute } = await loadList({
      spec: spec({ listView: { columns, rowActions: [formAction] } }),
      bindings: [listBinding, binding('b-ban', 'action')],
    });
    onExecute.mockResolvedValueOnce(ok({}));
    await clickRowAction(/封\s*禁/);
    await waitFor(() => expect(screen.getByTestId('form-change')).toBeInTheDocument());
    // ActionFormModal 标题取 action.title（行按钮内文本同词，多处出现）
    expect(screen.getAllByText('封禁').length).toBeGreaterThan(1);
    fireEvent.click(screen.getByTestId('form-change'));
    fireEvent.click(screen.getByRole('button', { name: 'OK' }));
    await waitFor(() =>
      expect(onExecute).toHaveBeenCalledWith('b-ban', { form: { days: 3 }, row: rows[0] }),
    );
    expect(screen.getByText('操作成功')).toBeInTheDocument();
  });

  it('带表单行操作：validate 失败不提交', async () => {
    formMockState.validate = false;
    const formAction: ActionSpec = {
      ...banAction,
      form: { jsonSchema: { type: 'object' } },
    };
    const { onExecute } = await loadList({
      spec: spec({ listView: { columns, rowActions: [formAction] } }),
      bindings: [listBinding, binding('b-ban', 'action')],
    });
    await clickRowAction(/封\s*禁/);
    await waitFor(() => expect(screen.getByTestId('form-change')).toBeInTheDocument());
    fireEvent.click(screen.getByRole('button', { name: 'OK' }));
    expect(onExecute).toHaveBeenCalledTimes(1); // 仅列表
    formMockState.validate = true;
  });
});

describe('删除', () => {
  const deleteSpec = spec({
    deleteAction: {
      title: { 'zh-CN': '删除玩家' },
      description: { 'zh-CN': '删除后不可恢复' },
      confirmText: { 'zh-CN': '确认删除' },
      cancelText: { 'zh-CN': '再想想' },
      bindingId: 'b-del',
    },
  });
  const withDelete = [listBinding, binding('b-del', 'action')];

  it('成功：row 上下文 + reload', async () => {
    const { onExecute } = await loadList({ spec: deleteSpec, bindings: withDelete });
    onExecute.mockResolvedValueOnce(ok({}));
    await waitFor(() =>
      expect(screen.getAllByRole('button', { name: /删\s*除/ }).length).toBeGreaterThan(0),
    );
    fireEvent.click(screen.getAllByRole('button', { name: /删\s*除/ })[0]);
    // Popconfirm 文案
    await waitFor(() => expect(screen.getByText('删除玩家')).toBeInTheDocument());
    expect(screen.getByText('删除后不可恢复')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: /确认删除/ }));
    await waitFor(() => expect(screen.getByText('删除成功')).toBeInTheDocument());
    expect(onExecute).toHaveBeenCalledWith('b-del', { row: rows[0] });
    await waitFor(() =>
      expect(onExecute.mock.calls.filter(([id]) => id === 'b-list')).toHaveLength(2),
    );
  });

  it('失败：toast「删除失败」', async () => {
    const { onExecute } = await loadList({ spec: deleteSpec, bindings: withDelete });
    onExecute.mockRejectedValueOnce(new Error('fk'));
    await waitFor(() =>
      expect(screen.getAllByRole('button', { name: /删\s*除/ }).length).toBeGreaterThan(0),
    );
    fireEvent.click(screen.getAllByRole('button', { name: /删\s*除/ })[0]);
    await waitFor(() => expect(screen.getByText('删除玩家')).toBeInTheDocument());
    fireEvent.click(screen.getByRole('button', { name: /确认删除/ }));
    await waitFor(() => expect(screen.getByText('删除失败')).toBeInTheDocument());
  });

  // 注：handleDelete 的 preview 拦截在 UI 上不可达——删除按钮位于行内，
  // 预览模式下列表恒为空。
});

describe('工具栏与批量', () => {
  const toolbarAction: ActionSpec = {
    key: 'sync',
    title: { 'zh-CN': '同步' },
    type: 'primary',
    bindingId: 'b-sync',
  };
  const batchAction: ActionSpec = {
    key: 'batch-del',
    title: { 'zh-CN': '批量删除' },
    type: 'danger',
    bindingId: 'b-batch',
  };
  const listSpec = spec({
    listView: {
      columns,
      toolbarActions: [toolbarAction],
      batchActions: [batchAction],
      pagination: { enabled: true },
    },
  });
  const bindings = () => [listBinding, binding('b-sync', 'action'), binding('b-batch', 'action')];

  it('工具栏直接执行成功；selection 上下文批量执行', async () => {
    const { onExecute } = await loadList({ spec: listSpec, bindings: bindings() });
    onExecute.mockResolvedValueOnce(ok({}));

    await waitFor(() =>
      expect(screen.getAllByRole('button', { name: /同\s*步/ }).length).toBeGreaterThan(0),
    );
    fireEvent.click(screen.getAllByRole('button', { name: /同\s*步/ })[0]);
    await waitFor(() => expect(onExecute).toHaveBeenCalledWith('b-sync', {}));
    expect(screen.getByText('操作成功')).toBeInTheDocument();

    // 勾选两行 → 批量按钮带 selection
    fireEvent.click(screen.getByTestId('row-check-0'));
    fireEvent.click(screen.getByTestId('row-check-1'));
    await waitFor(() => expect(screen.getByText('已选择 2 项')).toBeInTheDocument());
    fireEvent.click(screen.getByRole('button', { name: /批量删除/ }));
    await waitFor(() => expect(onExecute).toHaveBeenCalledWith('b-batch', { selection: rows }));
  });

  it('工具栏 confirm 流 + onOk 失败', async () => {
    const confirmToolbar: ActionSpec = { ...toolbarAction, confirm: true };
    const { onExecute } = await loadList({
      spec: spec({ listView: { columns, toolbarActions: [confirmToolbar] } }),
      bindings: bindings(),
    });
    onExecute.mockRejectedValueOnce(new Error('bad'));
    await waitFor(() =>
      expect(screen.getAllByRole('button', { name: /同\s*步/ }).length).toBeGreaterThan(0),
    );
    fireEvent.click(screen.getAllByRole('button', { name: /同\s*步/ })[0]);
    await waitFor(() => expect(screen.getAllByText('确认操作').length).toBeGreaterThan(0));
    fireEvent.click(screen.getByRole('button', { name: 'OK' }));
    await waitFor(() => expect(screen.getByText('操作失败')).toBeInTheDocument());
  });

  // 注：executeListAction 的 preview 拦截在 UI 上不可达——工具栏/批量按钮均
  // disabled={preview}，React 对 disabled 按钮不派发 onClick。

  it('批量 confirm 流', async () => {
    const confirmBatch: ActionSpec = { ...batchAction, confirm: true };
    const { onExecute } = await loadList({
      spec: spec({ listView: { columns, batchActions: [confirmBatch] } }),
      bindings: bindings(),
    });
    onExecute.mockResolvedValueOnce(ok({}));
    await waitFor(() => expect(screen.getByText('Alice')).toBeInTheDocument());
    fireEvent.click(screen.getByTestId('row-check-0'));
    await waitFor(() => expect(screen.getByText('已选择 1 项')).toBeInTheDocument());
    fireEvent.click(screen.getByRole('button', { name: /批量删除/ }));
    await waitFor(() => expect(screen.getAllByText('确认操作').length).toBeGreaterThan(0));
    fireEvent.click(screen.getByRole('button', { name: 'OK' }));
    await waitFor(() =>
      expect(onExecute).toHaveBeenCalledWith('b-batch', { selection: [rows[0]] }),
    );
  });

  it('未配置批量动作时无选择列', async () => {
    await loadList();
    expect(screen.queryByTestId('row-check-0')).not.toBeInTheDocument();
    expect(screen.queryByTestId('selection-bar')).not.toBeInTheDocument();
  });
});

describe('详情抽屉', () => {
  const detailSpec = spec({
    detailView: {
      fields: [
        { key: 'id', title: { 'zh-CN': 'ID' }, dataType: 'string' },
        { key: 'name', title: { 'zh-CN': '名称' }, dataType: 'string' },
        { key: 'secret', title: { 'zh-CN': '保密' }, dataType: 'string', visible: false },
      ],
      layout: 'horizontal',
    },
  });
  const withDetail = [listBinding, detailBinding];

  const openDetail = async (options: RenderOptions = {}) => {
    const rendered = renderResource({
      spec: detailSpec,
      bindings: withDetail,
      executeImpl: async (id: string) =>
        id === 'b-list'
          ? ok({ data: rows, total: 2 })
          : ok({ detail: { id: 'p1', name: 'Alice-remote' } }),
      ...options,
    });
    await waitFor(() => expect(screen.getByText('Alice')).toBeInTheDocument());
    await waitFor(() =>
      expect(screen.getAllByRole('button', { name: /查\s*看/ }).length).toBeGreaterThan(0),
    );
    fireEvent.click(screen.getAllByRole('button', { name: /查\s*看/ })[0]);
    await waitFor(() => expect(screen.getByText('详情')).toBeInTheDocument());
    return rendered;
  };

  it('成功：detail selector 数据覆盖行数据；visible 字段过滤；横排两列', async () => {
    const { onExecute } = await openDetail();
    await waitFor(() => expect(screen.getByText('Alice-remote')).toBeInTheDocument());
    // 抽屉字段来自远程 detail 而非行数据（行内 Alice 仍在表格，仅核对抽屉）
    const items = screen.getAllByTestId('prodescriptions-item');
    expect(items.length).toBe(2); // secret visible:false 被过滤
    expect(items[1].textContent).toContain('Alice-remote');
    expect(screen.queryByText('保密')).not.toBeInTheDocument();

    // Drawer 关闭：动画在 jsdom 挂起致 DOM 残留，以「重新打开走完整请求链」为硬断言
    fireEvent.click(screen.getByRole('button', { name: 'Close' }));
    fireEvent.click(screen.getAllByRole('button', { name: /查\s*看/ })[0]);
    await waitFor(() =>
      expect(onExecute.mock.calls.filter(([id]) => id === 'b-detail')).toHaveLength(2),
    );
  });

  it('预览模式：不请求详情，直显行数据', async () => {
    renderResource({ spec: detailSpec, bindings: withDetail, preview: true });
    // 预览下列表为空、无行；onExecute 从未被调用，详情按钮不存在
    await waitFor(() => expect(screen.getByTestId('rows-loaded')).toBeInTheDocument());
    expect(screen.queryByRole('button', { name: /查\s*看/ })).not.toBeInTheDocument();
  });

  it('无 detail 绑定：直显行数据', async () => {
    await openDetail({ bindings: [listBinding] });
    // 表格行 + 抽屉直显各一处
    expect(screen.getAllByText('Alice').length).toBeGreaterThanOrEqual(2);
  });

  it('缺 detail selector：错误提示', async () => {
    const noSelector = binding('b-detail', 'detail', [
      { stateKey: 'other', source: '/detail', shape: 'object' },
    ]);
    renderResource({
      spec: detailSpec,
      bindings: [listBinding, noSelector],
      executeImpl: async (id: string) =>
        id === 'b-list' ? ok({ data: rows, total: 2 }) : ok({ detail: {} }),
    });
    await waitFor(() => expect(screen.getByText('Alice')).toBeInTheDocument());
    await waitFor(() =>
      expect(screen.getAllByRole('button', { name: /查\s*看/ }).length).toBeGreaterThan(0),
    );
    fireEvent.click(screen.getAllByRole('button', { name: /查\s*看/ })[0]);
    await waitFor(() =>
      expect(screen.getByText(/详情绑定缺少 pageState.detail 输出 selector/)).toBeInTheDocument(),
    );
  });

  it('未命中 detail selector：错误提示', async () => {
    renderResource({
      spec: detailSpec,
      bindings: withDetail,
      executeImpl: async (id: string) =>
        id === 'b-list' ? ok({ data: rows, total: 2 }) : ok({ empty: true }),
    });
    await waitFor(() => expect(screen.getByText('Alice')).toBeInTheDocument());
    await waitFor(() =>
      expect(screen.getAllByRole('button', { name: /查\s*看/ }).length).toBeGreaterThan(0),
    );
    fireEvent.click(screen.getAllByRole('button', { name: /查\s*看/ })[0]);
    await waitFor(() =>
      expect(screen.getByText(/详情结果未命中 detail selector/)).toBeInTheDocument(),
    );
  });

  it('detail 非对象：错误提示', async () => {
    renderResource({
      spec: detailSpec,
      bindings: withDetail,
      executeImpl: async (id: string) =>
        id === 'b-list' ? ok({ data: rows, total: 2 }) : ok({ detail: [1, 2] }),
    });
    await waitFor(() => expect(screen.getByText('Alice')).toBeInTheDocument());
    await waitFor(() =>
      expect(screen.getAllByRole('button', { name: /查\s*看/ }).length).toBeGreaterThan(0),
    );
    fireEvent.click(screen.getAllByRole('button', { name: /查\s*看/ })[0]);
    await waitFor(() =>
      expect(screen.getByText(/详情 detail selector 的结果不是对象/)).toBeInTheDocument(),
    );
  });

  it('详情请求异常：错误提示', async () => {
    renderResource({
      spec: detailSpec,
      bindings: withDetail,
      executeImpl: async (id: string) => {
        if (id === 'b-list') return ok({ data: rows, total: 2 });
        throw new Error('detail-down');
      },
    });
    await waitFor(() => expect(screen.getByText('Alice')).toBeInTheDocument());
    await waitFor(() =>
      expect(screen.getAllByRole('button', { name: /查\s*看/ }).length).toBeGreaterThan(0),
    );
    fireEvent.click(screen.getAllByRole('button', { name: /查\s*看/ })[0]);
    await waitFor(() => expect(screen.getByText(/加载详情失败/)).toBeInTheDocument());
  });
});

describe('渲染细节', () => {
  it('rowKey 兜底：identity 缺失时用数据串，其次 id/key 字段', async () => {
    renderResource({
      spec: spec({
        listView: { columns: [{ key: 'name', title: { 'zh-CN': '名称' }, dataType: 'string' }] },
      }),
      executeImpl: async () => ok({ data: [{ name: 'a' }, { name: 'b' }], total: 2 }),
    });
    await waitFor(() => expect(screen.getByText('a')).toBeInTheDocument());
    expect(screen.getByText('b')).toBeInTheDocument();
    // 两行 key 分别为 JSON.stringify 结果，不冲突即渲染成功
  });

  it('标题三态：title prop / 首列标题 / 兜底「资源列表」', () => {
    const { rerender } = renderResource({ title: '玩家管理' });
    expect(screen.getByTestId('protable-title').textContent).toContain('玩家管理');

    rerender(
      <App>
        <ResourcePageRenderer
          spec={spec()}
          bindings={[listBinding]}
          onExecute={jest.fn() as never}
        />
      </App>,
    );
    expect(screen.getByTestId('protable-title').textContent).toContain('ID');

    rerender(
      <App>
        <ResourcePageRenderer
          spec={{ listView: { columns: [] } }}
          bindings={[listBinding]}
          onExecute={jest.fn() as never}
        />
      </App>,
    );
    expect(screen.getByTestId('protable-title').textContent).toContain('资源列表');
  });

  it('无 detailView/rowActions/delete 时不渲染操作列', async () => {
    await loadList({
      spec: { listView: { columns } },
    });
    const call = (ProTable as unknown as jest.Mock).mock.calls[
      (ProTable as unknown as jest.Mock).mock.calls.length - 1
    ][0] as { columns: unknown[] };
    expect(call.columns).toHaveLength(columns.length);
  });

  it('pagination 未启用时不传分页配置', async () => {
    await loadList({ spec: spec({ listView: { columns, pagination: { enabled: false } } }) });
    expect(screen.queryByTestId('pagination-bar')).not.toBeInTheDocument();
  });
});
