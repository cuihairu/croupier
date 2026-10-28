/** D2 批量操作目标可核对覆盖。
 *
 * 纯逻辑（batchSelection.ts）：行标识口径（identityKey>id>key>数据串）、筛选指纹
 * （分页不计、空值不计、键序无关）、变更判定（无选择/首查/同指纹/变指纹）、
 * 目标摘要（上限截断与超出计数）。
 * UI（ResourcePageRenderer + ProTable 契约级替身）：跨页保留选择时选择栏展示
 * 总数与本页可见数、目标清单入口列出全部对象、批量确认框展示目标标识与数量且
 * 与 onExecute 实际提交一致（含超出上限只报数）、单行确认展示目标对象、
 * 筛选变更后旧选择提示核对、清除后提示消失、翻页不计筛选变更。
 */
import React from 'react';
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { App } from 'antd';
import ResourcePageRenderer from '../ResourcePageRenderer';
import * as SchemaFormModule from '@/components/SchemaFormRenderer';
import {
  isStaleSelection,
  rowIdentity,
  selectionFilterFingerprint,
  summarizeQuantityTotal,
  summarizeTargets,
} from '../batchSelection';
import type {
  ActionSpec,
  ColumnSpec,
  PageExecutionResult,
  PageFunctionBinding,
  ResourcePageSpec,
} from '@/types/dashboard';

const { formMockState } = SchemaFormModule as unknown as {
  formMockState: { validate: boolean; values: Record<string, unknown> };
};

jest.mock('@/components/SchemaFormRenderer', () => {
  const formMockState = { validate: true, values: {} as Record<string, unknown> };
  const Stub = React.forwardRef<unknown, Record<string, unknown>>((_props, ref) => {
    React.useImperativeHandle(ref, () => ({
      validate: () => formMockState.validate,
      getValues: () => formMockState.values,
    }));
    return <div data-testid="schema-form-stub" />;
  });
  Stub.displayName = 'SchemaFormRendererStub';
  return { __esModule: true, default: Stub, formMockState };
});

// ProTable 契约级替身（只实现批量核对所需面）：
// - request(params)：挂载即请求；真实 ProTable 在有查询表单时把筛选值并入
//   params，替身用按钮按同一契约注入（apply-filter / goto-page-N）
// - rowSelection.onChange(keys, rows)：preserveSelectedRowKeys 语义——key 集跨页
//   保留，不在当前页的行对象从缓存取回（antd 实现同语义）
jest.mock('@ant-design/pro-components', () => {
  const ProTableStub = ({
    request,
    columns,
    rowKey,
    tableAlertRender,
    tableAlertOptionRender,
    rowSelection,
  }: {
    request?: (params: Record<string, unknown>) => Promise<{ data: unknown[]; total: number }>;
    columns?: Array<{
      key?: string;
      dataIndex?: string;
      render?: (dom: unknown, record: Record<string, unknown>) => React.ReactNode;
    }>;
    rowKey?: (record: Record<string, unknown>) => string;
    tableAlertRender?: (info: {
      selectedRowKeys: React.Key[];
      onCleanSelected: () => void;
    }) => React.ReactNode;
    tableAlertOptionRender?: () => React.ReactNode;
    rowSelection?: { onChange: (keys: React.Key[], rows: Record<string, unknown>[]) => void };
  }) => {
    const [rows, setRows] = React.useState<Record<string, unknown>[]>([]);
    const [selected, setSelected] = React.useState<string[]>([]);
    const selectedKeysRef = React.useRef<string[]>([]);
    const rowCacheRef = React.useRef(new Map<string, Record<string, unknown>>());
    const queryRef = React.useRef<Record<string, unknown>>({});
    const keyOf = (row: Record<string, unknown>) => String(rowKey?.(row) ?? '');
    const load = React.useCallback(
      (params: Record<string, unknown>) => {
        queryRef.current = { ...queryRef.current, ...params };
        void request?.(queryRef.current).then((r) => setRows(r.data as Record<string, unknown>[]));
      },
      [request],
    );
    React.useEffect(() => {
      load({});
    }, [load]);
    const toggle = (row: Record<string, unknown>) => {
      const key = keyOf(row);
      const has = selectedKeysRef.current.includes(key);
      const next = has
        ? selectedKeysRef.current.filter((k) => k !== key)
        : [...selectedKeysRef.current, key];
      if (!has) rowCacheRef.current.set(key, row);
      selectedKeysRef.current = next;
      setSelected(next);
      rowSelection?.onChange(
        next,
        next.map((k) => rowCacheRef.current.get(k) as Record<string, unknown>),
      );
    };
    const clean = () => {
      selectedKeysRef.current = [];
      setSelected([]);
      rowSelection?.onChange([], []);
    };
    return (
      <div>
        <button type="button" data-testid="apply-filter" onClick={() => load({ status: 'banned' })}>
          apply-filter
        </button>
        {[2, 3].map((page) => (
          <button
            key={page}
            type="button"
            data-testid={`goto-page-${page}`}
            onClick={() => load({ current: page, pageSize: 2 })}
          >
            page-{page}
          </button>
        ))}
        {rows.map((row) => {
          const key = keyOf(row);
          return (
            <label key={key} data-testid={`row-label-${key}`}>
              <input
                type="checkbox"
                data-testid={`row-check-${key}`}
                checked={selected.includes(key)}
                onChange={() => toggle(row)}
              />
              <span data-testid={`row-id-${key}`}>{String(row.id ?? '')}</span>
            </label>
          );
        })}
        {selected.length > 0 ? (
          <div data-testid="selection-bar">
            {tableAlertRender?.({ selectedRowKeys: selected, onCleanSelected: clean })}
            {tableAlertOptionRender?.()}
          </div>
        ) : null}
        <table>
          <tbody>
            {rows.map((row) => (
              <tr key={keyOf(row)}>
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
  const ProFormShellStub: React.FC<{
    open?: boolean;
    children?: React.ReactNode;
    onFinish?: () => Promise<boolean>;
  }> = ({ open, children, onFinish }) =>
    open ? (
      <div data-testid="proform-shell">
        {children}
        <button type="button" onClick={() => void onFinish?.()}>
          OK
        </button>
      </div>
    ) : null;
  return {
    ProTable: ProTableStub,
    ProDescriptions: ProDescriptionsStub,
    ModalForm: ProFormShellStub,
    DrawerForm: ProFormShellStub,
  };
});

jest.setTimeout(20000);

type Row = { id: string; name: string };
type ExecuteMock = jest.Mock<
  Promise<PageExecutionResult>,
  [string, { form?: Record<string, unknown> }]
>;

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

const listBinding = binding('b-list', 'query', [
  { stateKey: 'items', source: '/data', shape: 'collection' },
  { stateKey: 'total', source: '/total', shape: 'scalar' },
]);

const columns: ColumnSpec[] = [
  { key: 'id', title: { 'zh-CN': 'ID' }, dataType: 'string' },
  { key: 'name', title: { 'zh-CN': '名称' }, dataType: 'string' },
];

const batchAction: ActionSpec = {
  key: 'batch-ban',
  title: { 'zh-CN': '批量封禁' },
  type: 'danger',
  bindingId: 'b-batch',
  confirm: true,
};
const rowConfirmAction: ActionSpec = {
  key: 'ban',
  title: { 'zh-CN': '封禁' },
  type: 'danger',
  bindingId: 'b-row',
  confirm: true,
};

const spec = (
  listOverrides: Partial<NonNullable<ResourcePageSpec['listView']>> = {},
): ResourcePageSpec => ({
  listView: {
    columns,
    batchActions: [batchAction],
    rowActions: [rowConfirmAction],
    pagination: { enabled: true },
    ...listOverrides,
  } as ResourcePageSpec['listView'],
});

const P = (id: string, name: string): Row => ({ id, name });
const fourRows: Row[] = [P('p1', 'Alice'), P('p2', 'Bob'), P('p3', 'Carol'), P('p4', 'Dave')];
const sixRows: Row[] = [...fourRows, P('p5', 'Eve'), P('p6', 'Frank')];

/** 执行器替身：按 current/pageSize 真实切片；status 过滤命中时只回 p2（模拟 p1
 * 已不在新筛选的结果集里）。 */
const pagedExecutor =
  (all: Row[]) =>
  async (
    _bindingId: string,
    context: { form?: Record<string, unknown> },
  ): Promise<PageExecutionResult> => {
    if (_bindingId !== 'b-list') return ok({});
    const form = context.form ?? {};
    const rowsNow =
      form.status !== undefined && form.status !== '' ? all.filter((row) => row.id === 'p2') : all;
    const page = Number(form.current) || 1;
    const size = Number(form.pageSize) || 2;
    const offset = (page - 1) * size;
    return ok({ data: rowsNow.slice(offset, offset + size), total: rowsNow.length });
  };

async function renderBatch(options: { spec?: ResourcePageSpec; all?: Row[] } = {}) {
  const onExecute = jest.fn() as ExecuteMock;
  onExecute.mockImplementation(pagedExecutor(options.all ?? fourRows));
  render(
    <App>
      <ResourcePageRenderer
        spec={options.spec ?? spec()}
        bindings={[listBinding, binding('b-batch', 'action'), binding('b-row', 'action')]}
        onExecute={onExecute as never}
      />
    </App>,
  );
  await waitFor(() => expect(screen.getByTestId('row-check-p1')).toBeInTheDocument());
  return { onExecute };
}

/** 确认弹窗内的断言作用域（portal 渲染，正文节点为 .ant-modal-confirm） */
const inConfirm = () => within(document.querySelector('.ant-modal-confirm') as HTMLElement);

/** 跨页勾选：第 1 页选 p1/p2，翻到第 2 页选 p3（p4 可选），翻到第 3 页选 p5/p6 */
async function selectAcrossPages(options: { page2: boolean[]; page3?: boolean[] }) {
  fireEvent.click(screen.getByTestId('row-check-p1'));
  fireEvent.click(screen.getByTestId('row-check-p2'));
  fireEvent.click(screen.getByTestId('goto-page-2'));
  await waitFor(() => expect(screen.getByTestId('row-check-p3')).toBeInTheDocument());
  options.page2.forEach((flag, index) => {
    if (flag) fireEvent.click(screen.getByTestId(`row-check-p${3 + index}`));
  });
  if (options.page3) {
    fireEvent.click(screen.getByTestId('goto-page-3'));
    await waitFor(() => expect(screen.getByTestId('row-check-p5')).toBeInTheDocument());
    options.page3.forEach((flag, index) => {
      if (flag) fireEvent.click(screen.getByTestId(`row-check-p${5 + index}`));
    });
  }
}

describe('D2 纯逻辑：行标识与筛选指纹', () => {
  it('rowIdentity 口径：identityKey > id > key，缺失兜底数据串', () => {
    expect(rowIdentity({ id: 'p1' })).toBe('p1');
    expect(rowIdentity({ id: 'p1' }, 'uid')).toBe('p1'); // 无 uid 时回落 id
    expect(rowIdentity({ uid: 'u9', id: 'p1' }, 'uid')).toBe('u9');
    expect(rowIdentity({ key: 'k1' })).toBe('k1');
    expect(rowIdentity({ id: null })).toBe(JSON.stringify({ id: null }));
    expect(rowIdentity({ id: 0 })).toBe('0');
  });

  it('指纹不计分页与空值，键序无关', () => {
    expect(selectionFilterFingerprint({ current: 1, pageSize: 20, page: 1 })).toBe('');
    expect(selectionFilterFingerprint({ current: 2, pageSize: 20 })).toBe('');
    expect(selectionFilterFingerprint({ status: '', zone: null, tag: undefined })).toBe('');
    expect(selectionFilterFingerprint({ status: 'banned' })).toBe('status=banned');
    // 同条件不同键序 → 同指纹（翻页后重放不误报）
    expect(selectionFilterFingerprint({ status: 'banned', zone: 's1', current: 3 })).toBe(
      selectionFilterFingerprint({ zone: 's1', status: 'banned', current: 9, pageSize: 50 }),
    );
    expect(selectionFilterFingerprint({ status: 'banned' })).not.toBe(
      selectionFilterFingerprint({ status: 'active' }),
    );
  });

  it('变更判定：无选择/首查/同指纹不计，变指纹且有选择才计', () => {
    expect(isStaleSelection('a=1', 'b=2', 0)).toBe(false); // 无选择
    expect(isStaleSelection(null, 'b=2', 3)).toBe(false); // 首次查询无上一指纹
    expect(isStaleSelection('', '', 3)).toBe(false); // 同指纹（含翻页）
    expect(isStaleSelection('', 'b=2', 3)).toBe(true); // 无筛选切到有筛选
    expect(isStaleSelection('a=1', 'b=2', 3)).toBe(true); // 筛选变了且选择还在
  });

  it('summarizeTargets：上限截断 + 超出计数', () => {
    const rows = [1, 2, 3, 4, 5, 6, 7].map((n) => ({ id: `p${n}` }));
    expect(summarizeTargets(rows.slice(0, 3), 'id')).toEqual({
      identities: ['p1', 'p2', 'p3'],
      omitted: 0,
    });
    expect(summarizeTargets(rows, 'id')).toEqual({
      identities: ['p1', 'p2', 'p3', 'p4', 'p5'],
      omitted: 2,
    });
    expect(summarizeTargets(rows, 'id', 2)).toEqual({ identities: ['p1', 'p2'], omitted: 5 });
  });

  it('summarizeQuantityTotal：每人数量 × 目标数，非法输入不给总量', () => {
    expect(summarizeQuantityTotal(100, 3)).toBe(300);
    expect(summarizeQuantityTotal('50', 4)).toBe(200);
    expect(summarizeQuantityTotal(0, 5)).toBe(0);
    expect(summarizeQuantityTotal(undefined, 3)).toBeUndefined();
    expect(summarizeQuantityTotal(null, 3)).toBeUndefined();
    expect(summarizeQuantityTotal(-1, 3)).toBeUndefined();
    expect(summarizeQuantityTotal('abc', 3)).toBeUndefined();
    expect(summarizeQuantityTotal(100, 0)).toBeUndefined();
    expect(summarizeQuantityTotal(100, 2.5)).toBeUndefined();
  });
});

describe('D2 选择栏与目标清单', () => {
  it('跨页保留选择：选择栏展示总数与本页可见数', async () => {
    await renderBatch();
    fireEvent.click(screen.getByTestId('row-check-p1'));
    await waitFor(() => expect(screen.getByText('已选择 1 项')).toBeInTheDocument());
    // 同页选择：不出现本页可见数（无跨页无需提示）
    expect(screen.queryByTestId('selection-visible')).not.toBeInTheDocument();

    fireEvent.click(screen.getByTestId('goto-page-2'));
    await waitFor(() => expect(screen.getByTestId('row-check-p3')).toBeInTheDocument());
    fireEvent.click(screen.getByTestId('row-check-p3'));
    await waitFor(() => expect(screen.getByText('已选择 2 项')).toBeInTheDocument());
    // p1 在第 1 页：本页可见只有 p3
    expect(screen.getByTestId('selection-visible')).toHaveTextContent('本页可见 1 项');
  });

  it('目标清单入口：列出全部已选对象（含不在当前页的）', async () => {
    await renderBatch();
    await selectAcrossPages({ page2: [true] });
    await waitFor(() => expect(screen.getByText('已选择 3 项')).toBeInTheDocument());

    fireEvent.click(screen.getByRole('button', { name: '查看目标清单' }));
    await screen.findByText('已选目标清单（3 项）');
    const table = screen.getByTestId('target-list-table');
    expect(within(table).getByText('p1')).toBeInTheDocument();
    expect(within(table).getByText('p2')).toBeInTheDocument();
    expect(within(table).getByText('p3')).toBeInTheDocument();
    // 标识列在前（列标题 ID 在名称之前）
    expect(within(table).getAllByRole('columnheader')[0]).toHaveTextContent('ID');
  });
});

describe('D2 确认内容与实际提交一致', () => {
  it('带 quantity 表单的批量操作：提交前展示总量，onExecute 携带 form+selection', async () => {
    const { onExecute } = await renderBatch({
      spec: spec({
        batchActions: [
          {
            key: 'batch-compensate',
            title: { 'zh-CN': '批量补偿' },
            type: 'danger',
            bindingId: 'b-batch',
            confirm: true,
            form: {
              jsonSchema: {
                type: 'object',
                properties: { perAmount: { type: 'number', title: '每人数量' } },
                required: ['perAmount'],
              },
              fields: [{ key: 'perAmount', role: 'quantity', unit: { 'zh-CN': '金' } }],
            },
          },
        ],
      }),
    });
    await selectAcrossPages({ page2: [true] });
    await waitFor(() => expect(screen.getByText('已选择 3 项')).toBeInTheDocument());

    fireEvent.click(screen.getByRole('button', { name: /批量补偿/ }));
    await waitFor(() => expect(screen.getByTestId('schema-form-stub')).toBeInTheDocument());
    // DrawerForm 替身提交时 getValues 返回表单 mock 值
    formMockState.values = { perAmount: 100 };
    fireEvent.click(
      within(screen.getByTestId('proform-shell')).getByRole('button', { name: 'OK' }),
    );

    // 总量确认弹窗
    await waitFor(() => expect(screen.getAllByText('核对跨目标总量').length).toBeGreaterThan(0));
    const confirm = inConfirm();
    expect(confirm.getByText('每人数量')).toBeInTheDocument();
    expect(confirm.getByText('100 金')).toBeInTheDocument();
    expect(confirm.getByText('目标数量')).toBeInTheDocument();
    expect(confirm.getByText('3')).toBeInTheDocument();
    expect(confirm.getByText('总量')).toBeInTheDocument();
    expect(confirm.getByText('300 金')).toBeInTheDocument();

    fireEvent.click(confirm.getByRole('button', { name: 'OK' }));
    await waitFor(() =>
      expect(onExecute).toHaveBeenCalledWith('b-batch', {
        form: { perAmount: 100 },
        selection: fourRows.slice(0, 3),
      }),
    );
  });

  it('批量确认框展示目标标识与数量，onExecute 收到同一批对象', async () => {
    const { onExecute } = await renderBatch();
    await selectAcrossPages({ page2: [true] });
    await waitFor(() => expect(screen.getByText('已选择 3 项')).toBeInTheDocument());

    fireEvent.click(screen.getByRole('button', { name: /批量封禁/ }));
    await waitFor(() => expect(screen.getAllByText('确认操作').length).toBeGreaterThan(0));
    const confirm = inConfirm();
    // 目标条目取自选择状态本身，与将要提交的 selection 同源
    expect(confirm.getByText('目标对象')).toBeInTheDocument();
    expect(confirm.getByText('p1、p2、p3')).toBeInTheDocument();
    expect(confirm.getByText('目标数量')).toBeInTheDocument();
    expect(confirm.getByText('3')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'OK' }));
    await waitFor(() =>
      expect(onExecute).toHaveBeenCalledWith('b-batch', { selection: fourRows.slice(0, 3) }),
    );
  });

  it('目标超出展示上限只报数，onExecute 收到全部对象', async () => {
    const { onExecute } = await renderBatch({ all: sixRows });
    await selectAcrossPages({ page2: [true, true], page3: [true, true] });
    await waitFor(() => expect(screen.getByText('已选择 6 项')).toBeInTheDocument());

    fireEvent.click(screen.getByRole('button', { name: /批量封禁/ }));
    await waitFor(() => expect(screen.getAllByText('确认操作').length).toBeGreaterThan(0));
    // 展示前 5 个标识值 + 超出部分只报数，完整清单走选择栏入口
    const confirm = inConfirm();
    expect(confirm.getByText('p1、p2、p3、p4、p5 等 6 项')).toBeInTheDocument();
    expect(confirm.getByText('6')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'OK' }));
    await waitFor(() => expect(onExecute).toHaveBeenCalledWith('b-batch', { selection: sixRows }));
  });

  it('单行确认框展示该行目标对象', async () => {
    const { onExecute } = await renderBatch();
    // 每行一个封禁按钮：取第一行
    fireEvent.click(screen.getAllByRole('button', { name: '封禁' })[0]);
    await waitFor(() => expect(screen.getAllByText('确认操作').length).toBeGreaterThan(0));
    const confirm = inConfirm();
    expect(confirm.getByText('目标对象')).toBeInTheDocument();
    expect(confirm.getByText('p1')).toBeInTheDocument();
    expect(confirm.queryByText('目标数量')).not.toBeInTheDocument(); // 单目标不给数量条目

    fireEvent.click(screen.getByRole('button', { name: 'OK' }));
    await waitFor(() => expect(onExecute).toHaveBeenCalledWith('b-row', { row: fourRows[0] }));
  });
});

describe('D2 筛选变更后的旧选择处理', () => {
  it('筛选变化后提示核对，清除选择后提示消失', async () => {
    await renderBatch();
    fireEvent.click(screen.getByTestId('row-check-p1'));
    await waitFor(() => expect(screen.getByText('已选择 1 项')).toBeInTheDocument());
    expect(screen.queryByTestId('selection-stale')).not.toBeInTheDocument();

    fireEvent.click(screen.getByTestId('apply-filter'));
    await waitFor(() => expect(screen.getByTestId('selection-stale')).toBeInTheDocument());
    expect(screen.getByTestId('selection-stale')).toHaveTextContent('筛选条件已变更');

    fireEvent.click(screen.getByRole('button', { name: '取消选择' }));
    await waitFor(() => expect(screen.queryByTestId('selection-stale')).not.toBeInTheDocument());
    expect(screen.queryByTestId('selection-bar')).not.toBeInTheDocument();
  });

  it('翻页不算筛选变更：跨页选择不触发核对提示', async () => {
    await renderBatch();
    fireEvent.click(screen.getByTestId('row-check-p1'));
    await waitFor(() => expect(screen.getByText('已选择 1 项')).toBeInTheDocument());
    fireEvent.click(screen.getByTestId('goto-page-2'));
    await waitFor(() => expect(screen.getByTestId('row-check-p3')).toBeInTheDocument());
    expect(screen.queryByTestId('selection-stale')).not.toBeInTheDocument();
    expect(screen.getByText('已选择 1 项')).toBeInTheDocument();
  });
});
