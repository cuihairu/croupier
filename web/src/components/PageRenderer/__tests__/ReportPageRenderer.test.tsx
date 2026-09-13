/** ReportPageRenderer 覆盖。
 *
 * 覆盖路径：查询（预览拦截/成功/未命中 dataset 映射/Error·非 Error 异常）、
 * 语义与绑定前置校验（dimensions·metrics 缺失、缺绑定、缺 dataset 输出选择器）、
 * 图表渲染（line·bar·area·pie·未知类型兜底、seriesField 有无、标题兜底、
 * 表格 tab 切换）、表格列（维度 valueType 三态、指标 render 四分支）、
 * 导出（本地 CSV 含 object/null 单元格、onExport 成功与异常）、
 * 空状态、清空、exportable 开关、标题透传。 */
import React from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App } from 'antd';
import { Area, Column, Line, Pie } from '@ant-design/charts';
import { ProTable } from '@ant-design/pro-components';
import { exportToCSV } from '@/utils/export';
import SchemaFormRenderer from '@/components/SchemaFormRenderer';
import ReportPageRenderer from '../ReportPageRenderer';
import type { PageExecutionResult, PageFunctionBinding, ReportPageSpec } from '@/types/dashboard';

// SchemaFormRenderer 替身：渲染提交按钮直接触发 onFinish
jest.mock('@/components/SchemaFormRenderer', () => ({
  __esModule: true,
  default: jest.fn(({ onFinish }: { onFinish: (v: Record<string, unknown>) => void }) => (
    <button type="button" data-testid="form-submit" onClick={() => onFinish({ range: '7d' })}>
      form-submit
    </button>
  )),
}));

// 图表组件替身：data-testid 标识类型，seriesField 经 data 属性透出
jest.mock('@ant-design/charts', () => {
  const stub = (name: string) =>
    jest.fn((props: Record<string, unknown>) => (
      <div
        data-testid={`chart-${name}`}
        data-series={props.seriesField === undefined ? '' : String(props.seriesField)}
      />
    ));
  return { Line: stub('line'), Column: stub('column'), Area: stub('area'), Pie: stub('pie') };
});

// ProTable 替身：渲染各列 render 输出与分页 total 文案，columns 经 mock.calls 捕获；
// rowKey 按行调用（含一次 index 缺省调用，覆盖其 '0' 兜底分支）
jest.mock('@ant-design/pro-components', () => ({
  ProTable: jest.fn(
    ({
      columns,
      dataSource,
      pagination,
      rowKey,
    }: {
      columns: Array<{
        key?: string;
        dataIndex?: string;
        valueType?: string;
        render?: (dom: unknown, record: Record<string, unknown>) => React.ReactNode;
      }>;
      dataSource: Array<Record<string, unknown>>;
      pagination?: { showTotal?: (total: number) => React.ReactNode };
      rowKey?: (record: Record<string, unknown>, index?: number) => string;
    }) => (
      <table data-fallback-key={rowKey ? rowKey(dataSource[0] ?? {}, undefined) : ''}>
        <tbody>
          {dataSource.map((row, index) => (
            <tr key={rowKey ? rowKey(row, index) : String(index)}>
              {columns.map((column) => (
                <td key={column.key ?? ''} data-key={column.key ?? ''}>
                  {column.render
                    ? column.render(null, row)
                    : String(row[column.dataIndex ?? ''] ?? '')}
                </td>
              ))}
            </tr>
          ))}
          <tr>
            <td colSpan={columns.length}>{pagination?.showTotal?.(dataSource.length)}</td>
          </tr>
        </tbody>
      </table>
    ),
  ),
}));

jest.mock('@/utils/export', () => ({
  __esModule: true,
  exportToCSV: jest.fn(),
}));

type ExecuteMock = jest.Mock<Promise<PageExecutionResult>, [string, unknown]>;

const ok = (data?: unknown): PageExecutionResult => ({ kind: 'invoke', requestId: 'r1', data });

const reportBinding: PageFunctionBinding = {
  id: 'b-report',
  functionId: 'fn-report',
  usage: 'report',
  execution: { mode: 'sync' },
  selectors: {
    input: { assignments: [] },
    output: [{ stateKey: 'dataset', source: '/items', shape: 'dataset' }],
  },
};

const spec = (overrides: Partial<ReportPageSpec> = {}): ReportPageSpec => ({
  queryForm: { jsonSchema: { type: 'object' } },
  dataset: {
    dimensions: [
      { key: 'date', title: { 'zh-CN': '日期' }, dataType: 'date' },
      { key: 'channel', title: { 'zh-CN': '渠道' }, dataType: 'string' },
      { key: 'seq', title: { 'zh-CN': '序号' }, dataType: 'number' },
    ],
    metrics: [
      { key: 'pay', title: { 'zh-CN': '充值' }, dataType: 'number', format: 'currency' },
      { key: 'ratio', title: { 'zh-CN': '占比' }, dataType: 'number', format: 'percent' },
      { key: 'count', title: { 'zh-CN': '次数' }, dataType: 'number' },
      { key: 'meta', title: { 'zh-CN': '附加' }, dataType: 'number' },
    ],
  },
  ...overrides,
});

// 两行数据覆盖指标 render 四分支：currency/percent/普通 toLocaleString/
// undefined·null 兜底 '-'，以及 CSV 的 object·null 单元格
const rows = [
  {
    date: '2026-01-01',
    channel: 'ios',
    seq: 1,
    pay: 1234.5,
    ratio: 0.1234,
    count: 1000,
    meta: { deep: 1 },
  },
  { date: '2026-01-02', channel: 'and', seq: 2, pay: 500, ratio: null, count: 2 },
];

const charts = [
  {
    type: 'line' as const,
    title: { 'zh-CN': '趋势' },
    xField: 'date',
    yField: 'pay',
    seriesField: 'channel',
  },
  { type: 'bar' as const, title: { 'en-US': 'Bar' }, xField: 'date', yField: 'count' },
  { type: 'area' as const, title: { 'zh-CN': '面积' } },
  { type: 'pie' as const, title: { 'zh-CN': '占比分布' }, xField: 'channel', yField: 'pay' },
  { type: 'scatter' as const, title: { 'zh-CN': '散点' }, xField: 'seq', yField: 'count' },
];

interface RenderOptions {
  spec?: ReportPageSpec;
  bindings?: PageFunctionBinding[];
  preview?: boolean;
  onExport?: (format: 'csv' | 'excel') => Promise<void>;
  title?: string;
}

function renderReport(options: RenderOptions = {}) {
  const onExecute = jest.fn() as ExecuteMock;
  const utils = render(
    <App>
      <ReportPageRenderer
        spec={options.spec ?? spec()}
        bindings={options.bindings ?? [reportBinding]}
        onExecute={onExecute as never}
        preview={options.preview}
        onExport={options.onExport}
        title={options.title}
      />
    </App>,
  );
  return { onExecute, ...utils };
}

const submit = () => fireEvent.click(screen.getByTestId('form-submit'));

// 查询出数据的标准链路
const queryRows = async (options: RenderOptions = {}) => {
  const rendered = renderReport({ spec: spec({ charts, exportable: true }), ...options });
  rendered.onExecute.mockResolvedValueOnce(ok({ items: rows }));
  submit();
  await waitFor(() => expect(screen.getByText('数据展示')).toBeInTheDocument());
  return rendered;
};

describe('查询', () => {
  it('预览模式拦截查询', async () => {
    renderReport({ preview: true });
    submit();
    await waitFor(() => expect(screen.getByText('预览模式不执行报表查询')).toBeInTheDocument());
  });

  it('查询成功：结果落 dataset 并渲染数据展示', async () => {
    const { onExecute } = renderReport();
    onExecute.mockResolvedValueOnce(ok({ items: rows }));
    submit();
    await waitFor(() => expect(screen.getByText('查询成功')).toBeInTheDocument());
    expect(screen.getByText('数据展示')).toBeInTheDocument();
    expect(onExecute).toHaveBeenCalledWith('b-report', { form: { range: '7d' } });
  });

  it('结果未命中 dataset 映射时报错并保持空态', async () => {
    const { onExecute } = renderReport();
    onExecute.mockResolvedValueOnce(ok({ other: 1 }));
    submit();
    await waitFor(() =>
      expect(screen.getByText('报表查询结果未命中 dataset 映射')).toBeInTheDocument(),
    );
    expect(screen.getByText('请先查询数据')).toBeInTheDocument();
  });

  it('查询异常：Error 取 message', async () => {
    const { onExecute } = renderReport();
    onExecute.mockRejectedValueOnce(new Error('boom'));
    submit();
    await waitFor(() => expect(screen.getByText('查询失败: boom')).toBeInTheDocument());
  });

  it('查询异常：非 Error 兜底「未知错误」', async () => {
    const { onExecute } = renderReport();
    onExecute.mockRejectedValueOnce('plain');
    submit();
    await waitFor(() => expect(screen.getByText('查询失败: 未知错误')).toBeInTheDocument());
  });
});

describe('语义与绑定前置校验', () => {
  it('dimensions 或 metrics 为空渲染「报表语义未完成」', () => {
    renderReport({ spec: spec({ dataset: { dimensions: [], metrics: [] } }) });
    expect(screen.getByText('报表语义未完成')).toBeInTheDocument();
  });

  it('缺报表绑定时渲染「报表绑定未完成」', () => {
    renderReport({ bindings: [] });
    expect(screen.getByText('报表绑定未完成')).toBeInTheDocument();
  });

  it('绑定缺 dataset 输出选择器时渲染「报表绑定未完成」', () => {
    const noSelector: PageFunctionBinding = { ...reportBinding, selectors: undefined };
    renderReport({ bindings: [noSelector] });
    expect(screen.getByText('报表绑定未完成')).toBeInTheDocument();
  });
});

describe('图表与表格', () => {
  it('五类图表渲染：line/bar/area/pie + 未知类型兜底 Line；seriesField 有无', async () => {
    await queryRows();
    // 有 charts 时默认激活图表 tab
    expect((Line as unknown as jest.Mock).mock.calls.length).toBeGreaterThanOrEqual(2); // line + scatter 兜底
    expect((Column as unknown as jest.Mock).mock.calls.length).toBeGreaterThanOrEqual(1);
    expect((Area as unknown as jest.Mock).mock.calls.length).toBeGreaterThanOrEqual(1);
    expect((Pie as unknown as jest.Mock).mock.calls.length).toBeGreaterThanOrEqual(1);

    // 标题兜底：bar 图 title 无 zh-CN → 取 en-US 值（fallback 仅在无任何 locale 时生效）
    expect(screen.getByText('Bar')).toBeInTheDocument();
  });

  it('seriesField 配置时图表收到 series，未配置时为 undefined', async () => {
    await queryRows();
    const line = screen.getAllByTestId('chart-line')[0];
    expect(line).toHaveAttribute('data-series', 'series');
    const column = screen.getByTestId('chart-column');
    expect(column).toHaveAttribute('data-series', '');
  });

  it('表格列：维度 valueType 三态 + 指标 render 四分支', async () => {
    await queryRows({ spec: spec({}) });
    // 无 charts 时默认表格 tab
    const call = (ProTable as unknown as jest.Mock).mock.calls[
      (ProTable as unknown as jest.Mock).mock.calls.length - 1
    ][0] as { columns: Array<{ key?: string; valueType?: string }> };
    const byKey = Object.fromEntries(call.columns.map((c) => [c.key, c]));
    expect(byKey.date.valueType).toBe('date');
    expect(byKey.channel.valueType).toBe('text');
    expect(byKey.seq.valueType).toBe('digit');

    // 指标渲染：currency / percent / toLocaleString / undefined·null → '-'
    expect(screen.getByText('¥1,234.5')).toBeInTheDocument();
    expect(screen.getByText('12.34%')).toBeInTheDocument();
    expect(screen.getByText('1,000')).toBeInTheDocument();
    expect(screen.getAllByText('-').length).toBeGreaterThanOrEqual(2); // 行2 ratio null + 行2 meta undefined
    // 分页 total 文案
    expect(screen.getByText('共 2 条')).toBeInTheDocument();
  });

  it('切换图表/表格 tab', async () => {
    await queryRows({ spec: spec({ charts }) });
    expect(screen.getAllByTestId('chart-line').length).toBeGreaterThan(0);
    fireEvent.click(screen.getByText('表格'));
    await waitFor(() => expect(screen.getByText('¥1,234.5')).toBeInTheDocument());
    fireEvent.click(screen.getByText('图表'));
    await waitFor(() => expect(screen.getAllByTestId('chart-line').length).toBeGreaterThan(0));
  });
});

describe('导出', () => {
  it('无 onExport：本地 CSV 含 object/null 单元格转换', async () => {
    await queryRows();
    fireEvent.click(screen.getByRole('button', { name: /导出 CSV/ }));
    await waitFor(() => expect(screen.getByText('导出成功')).toBeInTheDocument());
    const [fileName, csvRows] = (exportToCSV as jest.Mock).mock.calls[
      (exportToCSV as jest.Mock).mock.calls.length - 1
    ];
    expect(fileName).toBe('report.csv');
    expect(csvRows[0]).toEqual(['日期', '渠道', '序号', '充值', '占比', '次数', '附加']);
    expect(csvRows[1]).toEqual(['2026-01-01', 'ios', 1, 1234.5, 0.1234, 1000, '{"deep":1}']);
    expect(csvRows[2]).toEqual(['2026-01-02', 'and', 2, 500, '', 2, '']);
  });

  it('有 onExport：Excel 按钮走回调', async () => {
    const onExport = jest.fn<() => Promise<void>>().mockResolvedValue(undefined);
    await queryRows({ onExport });
    fireEvent.click(screen.getByRole('button', { name: /导出 Excel/ }));
    await waitFor(() => expect(onExport).toHaveBeenCalledWith('excel'));
    expect(screen.getByText('导出成功')).toBeInTheDocument();
  });

  it('有 onExport 且异常：展示导出失败', async () => {
    const onExport = jest.fn<() => Promise<void>>().mockRejectedValue(new Error('io'));
    await queryRows({ onExport });
    fireEvent.click(screen.getByRole('button', { name: /导出 CSV/ }));
    await waitFor(() => expect(screen.getByText('导出失败: io')).toBeInTheDocument());
  });
});

describe('渲染细节', () => {
  it('初始空状态与「清空结果」按钮', async () => {
    renderReport();
    expect(screen.getByText('请先查询数据')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: /清空结果/ }));
    expect(screen.getByText('请先查询数据')).toBeInTheDocument();
  });

  it('查询后点「清空」回到空状态', async () => {
    const { onExecute } = renderReport({ spec: spec({ exportable: true }) });
    onExecute.mockResolvedValueOnce(ok({ items: rows }));
    submit();
    await waitFor(() => expect(screen.getByText('数据展示')).toBeInTheDocument());
    fireEvent.click(screen.getByRole('button', { name: /reload 清空/ }));
    await waitFor(() => expect(screen.getByText('请先查询数据')).toBeInTheDocument());
    expect(screen.queryByText('数据展示')).not.toBeInTheDocument();
  });

  it('exportable 未配置时不渲染导出按钮', async () => {
    const { onExecute } = renderReport();
    onExecute.mockResolvedValueOnce(ok({ items: rows }));
    submit();
    await waitFor(() => expect(screen.getByText('数据展示')).toBeInTheDocument());
    expect(screen.queryByRole('button', { name: /导出/ })).not.toBeInTheDocument();
  });

  it('默认标题「报表查询」，自定义标题透传', () => {
    const { rerender } = renderReport();
    expect(screen.getByText('报表查询')).toBeInTheDocument();
    rerender(
      <App>
        <ReportPageRenderer
          spec={spec()}
          bindings={[reportBinding]}
          onExecute={jest.fn() as never}
          title="营收日报"
        />
      </App>,
    );
    expect(screen.getByText('营收日报')).toBeInTheDocument();
  });
});

describe('防御分支补充', () => {
  it('维度齐备但指标为空：同样判定语义未完成', () => {
    renderReport({
      spec: spec({ dataset: { dimensions: spec().dataset.dimensions, metrics: [] } }),
    });
    expect(screen.getByText('报表语义未完成')).toBeInTheDocument();
  });

  it('绑定 output 选择器存在但未映射 dataset：渲染「报表绑定未完成」', () => {
    const wrongSelector: PageFunctionBinding = {
      ...reportBinding,
      selectors: {
        input: { assignments: [] },
        output: [{ stateKey: 'dataset', source: '/items', shape: 'collection' }],
      },
    };
    renderReport({ bindings: [wrongSelector] });
    expect(screen.getByText('报表绑定未完成')).toBeInTheDocument();
  });

  it('绑定 selectors 无 output：渲染「报表绑定未完成」', () => {
    const noOutput: PageFunctionBinding = {
      ...reportBinding,
      selectors: { input: { assignments: [] }, output: undefined },
    };
    renderReport({ bindings: [noOutput] });
    expect(screen.getByText('报表绑定未完成')).toBeInTheDocument();
  });

  it('charts 为空数组：默认表格 tab，无图表 tab', async () => {
    const { onExecute } = renderReport({ spec: spec({ charts: [] }) });
    onExecute.mockResolvedValueOnce(ok({ items: rows }));
    submit();
    await waitFor(() => expect(screen.getByText('数据展示')).toBeInTheDocument());
    expect(screen.queryByRole('tab', { name: /图表/ })).not.toBeInTheDocument();
    expect(screen.getByText('¥1,234.5')).toBeInTheDocument();
  });

  it('查询出数据后切换到预览模式：导出被拦截', async () => {
    const rendered = renderReport({ spec: spec({ exportable: true }) });
    rendered.onExecute.mockResolvedValueOnce(ok({ items: rows }));
    submit();
    await waitFor(() => expect(screen.getByText('数据展示')).toBeInTheDocument());
    // 组件状态保留（数据仍在），仅 preview 翻转——预览导出守卫生效
    rendered.rerender(
      <App>
        <ReportPageRenderer
          spec={spec({ exportable: true })}
          bindings={[reportBinding]}
          onExecute={rendered.onExecute as never}
          preview
        />
      </App>,
    );
    fireEvent.click(screen.getByRole('button', { name: /导出 CSV/ }));
    await waitFor(() => expect(screen.getByText('预览模式不导出数据')).toBeInTheDocument());
  });
});
