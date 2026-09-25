/**
 * SortableList 替身按真实组件语义调用 getKey(item) 作 React key，
 * 补齐 Resource/Report 两个页面编辑器列表的 getKey 与 onReorder 覆盖：
 * 1. ResourcePageEditor 列表列：getKey + 列重排 onReorder；
 * 2. ReportPageEditor 指标列表：getKey + 指标重排 onReorder（维度侧既有用例已覆盖）。
 */
import React from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import ResourcePageEditor from '../ResourcePageEditor';
import ReportPageEditor from '../ReportPageEditor';
import type { ReportPageSpec, ResourcePageSpec } from '@/types/dashboard';

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

// SortableList 替身：与真实组件一致地用 getKey(item) 作 key，另给「重排」按钮
jest.mock('@/components/SortableList', () => {
  const R = require('react') as typeof import('react');
  interface StubProps {
    items: unknown[];
    getKey: (item: unknown) => string;
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
          R.createElement(R.Fragment, { key: props.getKey(item) }, props.children(item, index, {})),
        ),
      ),
  };
});

const openPanel = (title: string): void => {
  const header = Array.from(document.querySelectorAll('.ant-collapse-header')).find((el) =>
    el.textContent?.includes(title),
  );
  if (!header) throw new Error(`未找到面板头部：${title}`);
  if (header.parentElement?.className.includes('ant-collapse-item-active')) return;
  fireEvent.click(header);
};

const resourceValue = (): ResourcePageSpec => ({
  listView: {
    columns: [
      { key: 'name', title: { 'zh-CN': '名称' }, dataType: 'string', visible: true },
      { key: 'level', title: { 'zh-CN': '等级' }, dataType: 'number' },
    ],
    rowActions: [],
    batchActions: [],
    toolbarActions: [],
  },
});

const reportValue = (): ReportPageSpec => ({
  queryForm: { jsonSchema: {}, fields: [{ key: 'from' }] },
  dataset: {
    dimensions: [
      { key: 'date', title: { 'zh-CN': '日期' }, dataType: 'date' },
      { key: 'channel', title: { 'zh-CN': '渠道' }, dataType: 'string' },
    ],
    metrics: [
      { key: 'uv', title: { 'zh-CN': '访客数' }, dataType: 'number', aggType: 'sum' },
      { key: 'pv', title: { 'zh-CN': '浏览量' }, dataType: 'number', aggType: 'sum' },
    ],
  },
  charts: [],
  table: { columns: [] },
  exportable: false,
});

describe('SortableList getKey / onReorder 覆盖', () => {
  it('ResourcePageEditor 列表列：getKey 取列 key，重排倒序回写 columns', async () => {
    const onChange = jest.fn();
    render(<ResourcePageEditor value={resourceValue()} onChange={onChange} />);
    openPanel('列表视图');

    expect(await screen.findByTestId('sortable-stub')).toBeInTheDocument();
    fireEvent.click(screen.getByTestId('stub-reorder'));

    await waitFor(() => expect(onChange).toHaveBeenCalledTimes(1));
    expect(onChange.mock.calls[0][0]).toEqual({
      listView: {
        ...resourceValue().listView,
        columns: [
          { key: 'level', title: { 'zh-CN': '等级' }, dataType: 'number' },
          { key: 'name', title: { 'zh-CN': '名称' }, dataType: 'string', visible: true },
        ],
      },
    });
  });

  it('ReportPageEditor 指标列表：getKey 取指标 key，重排倒序回写 metrics', async () => {
    const onChange = jest.fn();
    render(<ReportPageEditor value={reportValue()} onChange={onChange} />);

    // 数据集面板内两个列表：[0]=维度、[1]=指标
    expect((await screen.findAllByTestId('sortable-stub')).length).toBe(2);
    fireEvent.click(screen.getAllByTestId('stub-reorder')[1]);

    await waitFor(() => expect(onChange).toHaveBeenCalledTimes(1));
    expect(onChange.mock.calls[0][0]).toEqual({
      ...reportValue(),
      dataset: {
        ...reportValue().dataset,
        metrics: [
          { key: 'pv', title: { 'zh-CN': '浏览量' }, dataType: 'number', aggType: 'sum' },
          { key: 'uv', title: { 'zh-CN': '访客数' }, dataType: 'number', aggType: 'sum' },
        ],
      },
    });
  });
});
