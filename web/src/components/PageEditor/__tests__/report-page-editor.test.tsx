/** ReportPageEditor 覆盖：数据集头部计数与维度/指标卡片编辑（dataType、
 * aggType、format、标题、重排）、空数据集分支、图表面板（charts 缺省时
 * 添加、已有图表的标题回退/删除/类型/字段/标题编辑）、表格与导出（开关、
 * 列数 Tag 与未配置回退）、查询表单面板 form 透传、readonly 下按钮禁用
 * 与拖拽手柄/删除入口隐藏。 */
import React from 'react';
import { fireEvent, render, screen, within } from '@testing-library/react';
import ReportPageEditor from '../ReportPageEditor';
import type {
  ChartSpec,
  DimensionSpec,
  ListViewSpec,
  MetricSpec,
  ReportPageSpec,
} from '@/types/dashboard';

jest.setTimeout(15000);

jest.mock('@umijs/max', () => ({
  __esModule: true,
  FormattedMessage: ({ defaultMessage }: { defaultMessage?: string }) => <>{defaultMessage}</>,
  useIntl: () => ({
    locale: 'zh-CN',
    formatMessage: (
      { defaultMessage }: { defaultMessage?: string },
      values?: Record<string, unknown>,
    ) =>
      Object.entries(values || {}).reduce(
        (msg, [key, val]) => msg.split(`{${key}}`).join(String(val)),
        defaultMessage ?? '',
      ),
  }),
}));

// SortableList 替身：条目 + 「重排」按钮（倒序回调），覆盖 onReorder 传播
jest.mock('@/components/SortableList', () => {
  const R = require('react') as typeof import('react');
  interface StubProps {
    items: unknown[];
    onReorder: (items: unknown[]) => void;
    children: (
      item: unknown,
      index: number,
      dragHandleProps: React.HTMLAttributes<HTMLElement>,
    ) => React.ReactNode;
  }
  return {
    __esModule: true,
    SortableList: (props: StubProps) =>
      R.createElement(
        'div',
        { 'data-testid': 'sortable-stub' },
        R.createElement(
          'button',
          {
            type: 'button',
            'data-testid': 'stub-reorder',
            onClick: () => props.onReorder([...props.items].reverse()),
          },
          '重排',
        ),
        ...props.items.map((item, index) =>
          R.createElement(R.Fragment, { key: index }, props.children(item, index, {})),
        ),
      ),
  };
});

/** 面板头部（标题与计数 Tag 始终渲染；内容惰性渲染需先展开） */
const headerOf = (title: string): HTMLElement => {
  const header = Array.from(document.querySelectorAll('.ant-collapse-header')).find((el) =>
    el.textContent?.includes(title),
  );
  if (!header) throw new Error(`未找到面板头部：${title}`);
  return header as HTMLElement;
};

const openPanel = (title: string): void => {
  const header = headerOf(title);
  if (header.parentElement?.className.includes('ant-collapse-item-active')) return;
  fireEvent.click(header);
};

/** 面板作用域（面板间卡片不串扰） */
const panelOf = (title: string): HTMLElement => {
  const panel = headerOf(title).parentElement;
  if (!panel) throw new Error(`未找到面板容器：${title}`);
  return panel as HTMLElement;
};

/** 按 label 文案定位 .ant-form-item（LocalizedTextEditor 不转发 id） */
const itemOf = (labelText: string): HTMLElement => {
  const label = screen.getByText(labelText);
  const item = label.closest('.ant-form-item');
  if (!item) throw new Error(`未找到表单项：${labelText}`);
  return item as HTMLElement;
};

const switchOf = (labelText: string): HTMLElement => within(itemOf(labelText)).getByRole('switch');

/** 打开 Select 下拉并点击匹配文案的选项（限定当前可见下拉） */
const pickOption = async (combobox: HTMLElement, label: string): Promise<void> => {
  fireEvent.mouseDown(combobox);
  let found: HTMLElement | undefined;
  for (let i = 0; i < 60 && !found; i += 1) {
    await new Promise((resolve) => setTimeout(resolve, 25));
    const dropdowns = document.querySelectorAll<HTMLElement>(
      '.ant-select-dropdown:not(.ant-select-dropdown-hidden)',
    );
    for (const dropdown of dropdowns) {
      const hit = Array.from(
        dropdown.querySelectorAll<HTMLElement>('.ant-select-item-option'),
      ).find((item) => item.textContent?.includes(label));
      if (hit) {
        found = hit;
        break;
      }
    }
  }
  if (!found) throw new Error(`下拉选项未出现：${label}`);
  fireEvent.click(found);
};

const dimDate: DimensionSpec = { key: 'date', title: { 'zh-CN': '日期' }, dataType: 'date' };
const dimChannel: DimensionSpec = {
  key: 'channel',
  title: { 'zh-CN': '渠道' },
  dataType: 'string',
};
const metricUv: MetricSpec = {
  key: 'uv',
  title: { 'zh-CN': '访客数' },
  dataType: 'number',
  aggType: 'sum',
};
const chartLine: ChartSpec = {
  type: 'line',
  title: { 'zh-CN': '趋势线' },
  xField: 'date',
  yField: 'uv',
};
// 标题空对象 → localizedText 逐级回退到 chart.type
const chartPie: ChartSpec = { type: 'pie', title: {} };

const reportTable: ListViewSpec = {
  columns: [
    { key: 'c1', title: { 'zh-CN': '列一' }, dataType: 'string' },
    { key: 'c2', title: { 'zh-CN': '列二' }, dataType: 'number' },
    { key: 'c3', title: { 'zh-CN': '列三' }, dataType: 'date' },
  ],
};

const reportSpec = (overrides: Partial<ReportPageSpec> = {}): ReportPageSpec => ({
  queryForm: { jsonSchema: {}, fields: [{ key: 'from' }] },
  dataset: { dimensions: [dimDate, dimChannel], metrics: [metricUv] },
  charts: [chartLine, chartPie],
  table: reportTable,
  exportable: false,
  ...overrides,
});

const cardOf = (panel: HTMLElement, key: string): HTMLElement => {
  const el = within(panel).getByText(key).closest('.ant-card');
  if (!el) throw new Error(`未找到卡片：${key}`);
  return el as HTMLElement;
};

/** 卡片内删除按钮（icon-only，danger text button） */
const deleteButtonOf = (card: HTMLElement): HTMLElement => {
  const btn = Array.from(card.querySelectorAll('button')).find((b) =>
    b.querySelector('.anticon-delete'),
  );
  if (!btn) throw new Error('未找到删除按钮');
  return btn as HTMLElement;
};

describe('ReportPageEditor：数据集', () => {
  it('头部计数与字段编辑：维度 dataType/标题、指标 aggType/format', async () => {
    const value = reportSpec();
    const onChange = jest.fn();
    render(<ReportPageEditor value={value} onChange={onChange} />);
    expect(headerOf('数据集').textContent).toContain('2 维度');
    expect(headerOf('数据集').textContent).toContain('1 指标');
    const datasetPanel = panelOf('数据集');

    // 维度卡片：标题区 dataType 下拉在前，正文是标题 LocalizedTextEditor
    const dimCard = cardOf(datasetPanel, 'date');
    await pickOption(within(dimCard).getAllByRole('combobox')[0], '数字');
    expect(onChange).toHaveBeenLastCalledWith({
      ...value,
      dataset: {
        ...value.dataset,
        dimensions: [{ ...dimDate, dataType: 'number' }, dimChannel],
      },
    });

    // 指标卡片：aggType、format 两个下拉都在标题区
    const metricCard = cardOf(datasetPanel, 'uv');
    await pickOption(within(metricCard).getAllByRole('combobox')[0], 'avg');
    expect(onChange).toHaveBeenLastCalledWith({
      ...value,
      dataset: { ...value.dataset, metrics: [{ ...metricUv, aggType: 'avg' }] },
    });
    await pickOption(within(metricCard).getAllByRole('combobox')[1], 'percent');
    expect(onChange).toHaveBeenLastCalledWith({
      ...value,
      dataset: { ...value.dataset, metrics: [{ ...metricUv, format: 'percent' }] },
    });

    // 维度标题编辑（另一维度保位）
    fireEvent.change(screen.getByDisplayValue('日期'), { target: { value: '统计日期' } });
    expect(onChange).toHaveBeenLastCalledWith({
      ...value,
      dataset: {
        ...value.dataset,
        dimensions: [{ ...dimDate, title: { 'zh-CN': '统计日期' } }, dimChannel],
      },
    });
  });

  it('维度重排：onReorder 倒序透传，指标保位', async () => {
    const value = reportSpec();
    const onChange = jest.fn();
    render(<ReportPageEditor value={value} onChange={onChange} />);
    await screen.findByText('date');
    // 数据集面板内两个列表：[0]=维度、[1]=指标
    fireEvent.click(screen.getAllByTestId('stub-reorder')[0]);
    expect(onChange).toHaveBeenLastCalledWith({
      ...value,
      dataset: { ...value.dataset, dimensions: [dimChannel, dimDate] },
    });
  });

  it('空数据集：头部 0 计数、不渲染字段列表与提示保留', async () => {
    const value = reportSpec({ dataset: { dimensions: [], metrics: [] } });
    render(<ReportPageEditor value={value} onChange={jest.fn()} />);
    expect(headerOf('数据集').textContent).toContain('0 维度');
    expect(headerOf('数据集').textContent).toContain('0 指标');
    expect(await screen.findByText(/维度 key 来自已审核的报表语义/)).toBeInTheDocument();
    expect(screen.getByText(/指标 key 来自已审核的报表语义/)).toBeInTheDocument();
    expect(screen.queryByTestId('sortable-stub')).not.toBeInTheDocument();
  });
});

describe('ReportPageEditor：图表', () => {
  it('charts 缺省：头部 0 个，添加图表生成默认 line 卡片数据', async () => {
    const value = reportSpec({ charts: undefined });
    const onChange = jest.fn();
    render(<ReportPageEditor value={value} onChange={onChange} />);
    expect(headerOf('图表').textContent).toContain('0 个');
    openPanel('图表');
    // 图标 aria-label（plus）参与可访问名，用正则匹配按钮文案
    fireEvent.click(await screen.findByRole('button', { name: /添加图表/ }));
    expect(onChange).toHaveBeenLastCalledWith({
      ...value,
      charts: [{ type: 'line', title: { 'zh-CN': '新图表' } }],
    });
  });

  it('已有图表：标题回退 type、类型换选、X/Y 字段与标题编辑、删除', async () => {
    const value = reportSpec();
    const onChange = jest.fn();
    render(<ReportPageEditor value={value} onChange={onChange} />);
    expect(headerOf('图表').textContent).toContain('2 个');
    openPanel('图表');
    const chartsPanel = panelOf('图表');
    await screen.findByText('趋势线');
    // 卡片标题：首图用 zh-CN 文案；第二图 title 空对象回退 chart.type
    const titles = Array.from(chartsPanel.querySelectorAll('.ant-card-head-title')).map(
      (el) => el.textContent,
    );
    expect(titles).toContain('趋势线');
    expect(titles).toContain('pie');

    const firstCard = cardOf(chartsPanel, '趋势线');
    // 卡片内 combobox：[0]=标题语言选择（正文先渲染），[1]=类型下拉
    await pickOption(within(firstCard).getAllByRole('combobox')[1], 'bar');
    expect(onChange).toHaveBeenLastCalledWith({
      ...value,
      charts: [{ ...chartLine, type: 'bar' }, chartPie],
    });

    fireEvent.change(screen.getByDisplayValue('date'), { target: { value: '统计日' } });
    expect(onChange).toHaveBeenLastCalledWith({
      ...value,
      charts: [{ ...chartLine, xField: '统计日' }, chartPie],
    });
    fireEvent.change(screen.getByDisplayValue('uv'), { target: { value: '访客数' } });
    expect(onChange).toHaveBeenLastCalledWith({
      ...value,
      charts: [{ ...chartLine, yField: '访客数' }, chartPie],
    });
    fireEvent.change(screen.getByDisplayValue('趋势线'), { target: { value: '趋势' } });
    expect(onChange).toHaveBeenLastCalledWith({
      ...value,
      charts: [{ ...chartLine, title: { 'zh-CN': '趋势' } }, chartPie],
    });

    fireEvent.click(deleteButtonOf(firstCard));
    expect(onChange).toHaveBeenLastCalledWith({ ...value, charts: [chartPie] });
  });
});

describe('ReportPageEditor：表格与导出', () => {
  it('有表格：列数 Tag 与导出开关切换', async () => {
    const value = reportSpec();
    const onChange = jest.fn();
    render(<ReportPageEditor value={value} onChange={onChange} />);
    openPanel('表格与导出');
    await screen.findByText('允许导出');
    expect(within(itemOf('表格')).getByText('3 列')).toBeInTheDocument();
    expect(switchOf('允许导出')).not.toBeChecked();
    fireEvent.click(switchOf('允许导出'));
    expect(onChange).toHaveBeenLastCalledWith({ ...value, exportable: true });
  });

  it('无表格：Tag 回退未配置', async () => {
    const value = reportSpec({ table: undefined });
    render(<ReportPageEditor value={value} onChange={jest.fn()} />);
    openPanel('表格与导出');
    await screen.findByText('允许导出');
    expect(within(itemOf('表格')).getByText('未配置')).toBeInTheDocument();
  });
});

describe('ReportPageEditor：查询表单', () => {
  it('头部字段计数，展开后布局切换透传 queryForm 变更', async () => {
    const value = reportSpec();
    const onChange = jest.fn();
    render(<ReportPageEditor value={value} onChange={onChange} />);
    expect(headerOf('查询表单').textContent).toContain('1 字段');
    openPanel('查询表单');
    expect(await screen.findByText(/这里只调整展示/)).toBeInTheDocument();
    // 查询表单是首个面板，其 FPE 布局下拉是文档序首个 combobox
    await pickOption(screen.getAllByRole('combobox')[0], '网格');
    expect(onChange).toHaveBeenLastCalledWith({
      ...value,
      queryForm: { jsonSchema: {}, fields: [{ key: 'from' }], layout: 'grid' },
    });
  });
});

describe('ReportPageEditor：readonly', () => {
  it('readonly：添加图表禁用、删除入口与拖拽手柄隐藏、导出开关禁用', async () => {
    const { container } = render(
      <ReportPageEditor value={reportSpec()} onChange={jest.fn()} readonly />,
    );
    expect(container.querySelector('.anticon-holder')).toBeNull();
    openPanel('图表');
    expect(await screen.findByRole('button', { name: /添加图表/ })).toBeDisabled();
    const chartsPanel = panelOf('图表');
    await screen.findByText('趋势线');
    expect(chartsPanel.querySelectorAll('.anticon-delete')).toHaveLength(0);
    openPanel('表格与导出');
    await screen.findByText('允许导出');
    expect(switchOf('允许导出')).toBeDisabled();
  });
});
