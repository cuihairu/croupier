/** TaskPageEditor 覆盖：任务视图开关（含 events/cancel binding 缺失禁用
 * 与提示、retry 恒禁用）、Lifecycle 绑定回退展示、启动表单面板计数与
 * form 透传、结果视图字段编辑与重排、readonly 下开关走 Form 禁用上下文
 * 与拖拽手柄隐藏。 */
import React from 'react';
import { fireEvent, render, screen, within } from '@testing-library/react';
import TaskPageEditor from '../TaskPageEditor';
import type { TaskPageSpec, TaskViewSpec } from '@/types/dashboard';

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

/** 按 label 文案定位 .ant-form-item（LocalizedTextEditor 不转发 id） */
const itemOf = (labelText: string): HTMLElement => {
  const label = screen.getByText(labelText);
  const item = label.closest('.ant-form-item');
  if (!item) throw new Error(`未找到表单项：${labelText}`);
  return item as HTMLElement;
};

/** 表单项内唯一 Switch（label 与开关同处一个 Form.Item） */
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

/** 全量绑定的任务视图（events/cancel/result 均已生成） */
const taskViewFull: TaskViewSpec = {
  taskIdStateKey: 'jobId',
  statusBindingId: 'b-status',
  statusStatePath: '/status',
  eventsBindingId: 'b-events',
  resultBindingId: 'b-result',
  cancelBindingId: 'b-cancel',
  showTimeline: true,
  showProgress: false,
  showEvents: false,
  cancelable: false,
  retryable: false,
};

/** 缺失绑定的任务视图（events/cancel 未生成，state key 走回退） */
const taskViewBare: TaskViewSpec = {
  taskIdStateKey: '',
  statusBindingId: '',
  statusStatePath: '',
  showTimeline: false,
  showProgress: true,
  showEvents: true,
  cancelable: true,
  retryable: false,
};

const taskSpec = (taskView: TaskViewSpec): TaskPageSpec => ({
  form: { jsonSchema: {}, fields: [{ key: 'count', widget: 'InputNumber' }] },
  taskView,
  resultView: {
    fields: [
      { key: 'score', title: { 'zh-CN': '得分' }, dataType: 'number' },
      { key: 'level', title: { 'zh-CN': '等级' }, dataType: 'string' },
    ],
  },
});

describe('TaskPageEditor：任务视图', () => {
  it('全量绑定：开关状态、events/cancel 可用、retry 恒禁用与 Lifecycle 展示', async () => {
    render(<TaskPageEditor value={taskSpec(taskViewFull)} onChange={jest.fn()} />);
    expect(await screen.findByText('显示时间线')).toBeInTheDocument();
    expect(switchOf('显示时间线')).toBeChecked();
    expect(switchOf('显示进度')).not.toBeChecked();
    expect(switchOf('显示事件')).toBeEnabled();
    expect(switchOf('允许取消')).toBeEnabled();
    // retry：无真实 retry function → 恒禁用 + 提示
    expect(switchOf('允许重试')).toBeDisabled();
    expect(
      screen.getByText('当前未配置真实 retry function，不能生成重试入口。'),
    ).toBeInTheDocument();
    // Lifecycle bindings：全量展示绑定 code
    expect(screen.getByText('jobId')).toBeInTheDocument();
    expect(screen.getByText('b-status')).toBeInTheDocument();
    expect(screen.getByText('/status')).toBeInTheDocument();
    expect(screen.getByText('b-events')).toBeInTheDocument();
    expect(screen.getByText('b-result')).toBeInTheDocument();
    expect(screen.getByText('b-cancel')).toBeInTheDocument();
  });

  it('开关切换：showTimeline 关闭、showProgress 开启透传 taskView 变更', async () => {
    const onChange = jest.fn();
    render(<TaskPageEditor value={taskSpec(taskViewFull)} onChange={onChange} />);
    await screen.findByText('显示时间线');
    fireEvent.click(switchOf('显示时间线'));
    expect(onChange).toHaveBeenLastCalledWith({
      ...taskSpec(taskViewFull),
      taskView: { ...taskViewFull, showTimeline: false },
    });
    fireEvent.click(switchOf('显示进度'));
    expect(onChange).toHaveBeenLastCalledWith({
      ...taskSpec(taskViewFull),
      taskView: { ...taskViewFull, showProgress: true },
    });
  });

  it('events/cancel binding 缺失：开关禁用 + 提示；Lifecycle 全部回退', async () => {
    render(<TaskPageEditor value={taskSpec(taskViewBare)} onChange={jest.fn()} />);
    await screen.findByText('显示事件');
    expect(switchOf('显示事件')).toBeDisabled();
    expect(screen.getByText('未生成 events binding，不能开启事件展示。')).toBeInTheDocument();
    expect(switchOf('允许取消')).toBeDisabled();
    expect(screen.getByText('未生成 cancel binding，不能开启取消入口。')).toBeInTheDocument();
    // taskIdStateKey 缺省回退 'taskId'；其余 5 项绑定回退「未配置」
    expect(screen.getByText('taskId')).toBeInTheDocument();
    expect(screen.getAllByText('未配置')).toHaveLength(5);
  });
});

describe('TaskPageEditor：启动表单', () => {
  it('头部字段计数，展开后布局切换透传 form 变更', async () => {
    const value = taskSpec(taskViewFull);
    const onChange = jest.fn();
    render(<TaskPageEditor value={value} onChange={onChange} />);
    expect(headerOf('启动表单').textContent).toContain('1 字段');
    openPanel('启动表单');
    expect(await screen.findByText(/这里只调整展示/)).toBeInTheDocument();
    // FPE 的布局下拉在字段列表之前（DOM 序首个 combobox）
    await pickOption(screen.getAllByRole('combobox')[0], '横向');
    expect(onChange).toHaveBeenLastCalledWith({
      ...value,
      form: {
        jsonSchema: {},
        fields: [{ key: 'count', widget: 'InputNumber' }],
        layout: 'horizontal',
      },
    });
  });
});

describe('TaskPageEditor：结果视图', () => {
  it('字段卡片：头部计数、dataType 换选（其余字段保位）、标题编辑与重排', async () => {
    const value = taskSpec(taskViewFull);
    const onChange = jest.fn();
    render(<TaskPageEditor value={value} onChange={onChange} />);
    expect(headerOf('结果视图').textContent).toContain('2 字段');
    openPanel('结果视图');
    const card = (await screen.findByText('score')).closest('.ant-card') as HTMLElement;
    expect(card.querySelector('.anticon-holder')).toBeTruthy();
    // dataType 下拉在卡片标题；其后是标题 LocalizedTextEditor 的语言选择
    await pickOption(within(card).getAllByRole('combobox')[0], '布尔');
    expect(onChange).toHaveBeenLastCalledWith({
      ...value,
      resultView: {
        ...value.resultView,
        fields: [
          { key: 'score', title: { 'zh-CN': '得分' }, dataType: 'boolean' },
          { key: 'level', title: { 'zh-CN': '等级' }, dataType: 'string' },
        ],
      },
    });

    fireEvent.change(screen.getByDisplayValue('得分'), { target: { value: '总分' } });
    expect(onChange).toHaveBeenLastCalledWith({
      ...value,
      resultView: {
        ...value.resultView,
        fields: [
          { key: 'score', title: { 'zh-CN': '总分' }, dataType: 'number' },
          { key: 'level', title: { 'zh-CN': '等级' }, dataType: 'string' },
        ],
      },
    });

    fireEvent.click(screen.getByTestId('stub-reorder'));
    expect(onChange).toHaveBeenLastCalledWith({
      ...value,
      resultView: {
        ...value.resultView,
        fields: [
          { key: 'level', title: { 'zh-CN': '等级' }, dataType: 'string' },
          { key: 'score', title: { 'zh-CN': '得分' }, dataType: 'number' },
        ],
      },
    });
  });

  it('resultView 缺省：头部计数为 0，展开无字段卡片', async () => {
    const value: TaskPageSpec = { ...taskSpec(taskViewFull), resultView: undefined };
    render(<TaskPageEditor value={value} onChange={jest.fn()} />);
    expect(headerOf('结果视图').textContent).toContain('0 字段');
    openPanel('结果视图');
    expect(await screen.findByText(/结果字段来自已发布输出映射/)).toBeInTheDocument();
    expect(screen.queryByTestId('sortable-stub')).not.toBeInTheDocument();
  });
});

describe('TaskPageEditor：readonly', () => {
  it('readonly：任务视图开关全部禁用，结果字段拖拽手柄隐藏', async () => {
    const { container } = render(
      <TaskPageEditor value={taskSpec(taskViewFull)} onChange={jest.fn()} readonly />,
    );
    await screen.findByText('显示时间线');
    expect(switchOf('显示时间线')).toBeDisabled();
    expect(switchOf('显示进度')).toBeDisabled();
    expect(switchOf('显示事件')).toBeDisabled();
    openPanel('结果视图');
    await screen.findByText('score');
    expect(container.querySelector('.anticon-holder')).toBeNull();
  });
});
