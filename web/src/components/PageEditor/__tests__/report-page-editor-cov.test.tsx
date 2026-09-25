/** ReportPageEditor 残余分支：指标卡片正文的标题编辑（LocalizedTextEditor
 * onChange → updateMetric 回写 metrics）。维度侧编辑由既有用例覆盖。 */
import React from 'react';
import { fireEvent, render, screen, configure } from '@testing-library/react';
import ReportPageEditor from '../ReportPageEditor';
import type { MetricSpec, ReportPageSpec } from '@/types/dashboard';

configure({ asyncUtilTimeout: 5000 });
jest.setTimeout(20000);

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

// SortableList 替身：条目 + 「重排」按钮
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
        ...props.items.map((item, index) =>
          R.createElement(R.Fragment, { key: index }, props.children(item, index, {})),
        ),
      ),
  };
});

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

const panelOf = (title: string): HTMLElement => {
  const panel = headerOf(title).parentElement;
  if (!panel) throw new Error(`未找到面板容器：${title}`);
  return panel as HTMLElement;
};

const metricUv: MetricSpec = {
  key: 'uv',
  title: { 'zh-CN': '访客数' },
  dataType: 'number',
  aggType: 'sum',
};

const reportSpec = (): ReportPageSpec => ({
  queryForm: { jsonSchema: {}, fields: [{ key: 'from' }] },
  dataset: { dimensions: [], metrics: [metricUv] },
  charts: [],
  exportable: false,
});

describe('ReportPageEditor 残余分支', () => {
  it('指标标题编辑 → updateMetric 回写 metrics（另一指标保位）', async () => {
    const value = reportSpec();
    const onChange = jest.fn();
    render(<ReportPageEditor value={value} onChange={onChange} />);
    openPanel('数据集');
    expect(await screen.findByText('uv')).toBeInTheDocument();

    fireEvent.change(screen.getByDisplayValue('访客数'), { target: { value: '访问量' } });
    expect(onChange).toHaveBeenLastCalledWith({
      ...value,
      dataset: { ...value.dataset, metrics: [{ ...metricUv, title: { 'zh-CN': '访问量' } }] },
    });
  });

  it('另一指标保位：index 不匹配时不被改写', async () => {
    const second: MetricSpec = { key: 'pv', title: { 'zh-CN': '浏览量' }, dataType: 'number' };
    const value: ReportPageSpec = {
      ...reportSpec(),
      dataset: { dimensions: [], metrics: [metricUv, second] },
    };
    const onChange = jest.fn();
    render(<ReportPageEditor value={value} onChange={onChange} />);
    openPanel('数据集');
    expect(await screen.findByText('pv')).toBeInTheDocument();

    fireEvent.change(screen.getByDisplayValue('浏览量'), { target: { value: '页面浏览' } });
    expect(onChange).toHaveBeenLastCalledWith({
      ...value,
      dataset: {
        ...value.dataset,
        metrics: [metricUv, { ...second, title: { 'zh-CN': '页面浏览' } }],
      },
    });
    expect(panelOf('数据集')).toBeInTheDocument();
  });
});
