/** PreviewNode（预览节点渲染）覆盖：text 三层级与 onClick 包裹、button
 * 三样式、tabs（空页 null/label 回退/子节点两路/空页签）、fnTable（声明列
 * 与 schema 列/截 8/行操作列 label 兜底与 danger/行点击/选中行/执行按钮
 * 条件）、fnFields（items·total 过滤/对象值 stringify/空值 -/onClick 包裹）、
 * fnForm 行内、staticForm（坏 schema 警告/防抖/卸载清 timer）、container
 * （renderChild 两路/空容器）、ModalForm（无 fn·无 properties→确认执行
 * /schema 数组）、StaticFormLive 对象形态 schema。 */
import React from 'react';
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import PreviewNode, { ModalForm, StaticFormLive } from '../PreviewNode';
import type { FunctionDescriptor } from '@/services/api/functions';
import type { PageNode } from '../model';

const fn: FunctionDescriptor = {
  id: 'player.query',
  operation: 'query',
  resource: 'player',
  inputSchema: {
    type: 'object',
    properties: { playerId: { type: 'string', title: '玩家ID' } },
  },
  outputSchema: {
    type: 'object',
    properties: { id: { type: 'string' }, name: { type: 'string' }, zone: { type: 'string' } },
  },
};

interface RenderOptions {
  node: PageNode;
  fn?: FunctionDescriptor | undefined;
  data?: unknown;
  running?: boolean;
  cascadeInputs?: Record<string, unknown>;
  handlers?: Partial<
    Record<
      | 'onAction'
      | 'onRowAction'
      | 'onSubmit'
      | 'onFormValues'
      | 'onStaticChange'
      | 'onSelectionChange',
      jest.Mock
    >
  >;
  renderChild?: (child: PageNode) => React.ReactNode;
}

function renderPreview(options: RenderOptions) {
  const h = options.handlers ?? {};
  return render(
    <PreviewNode
      node={options.node}
      fn={options.fn}
      data={options.data ?? null}
      running={options.running ?? false}
      cascadeInputs={options.cascadeInputs}
      onAction={h.onAction ?? jest.fn()}
      onRowAction={h.onRowAction}
      onSubmit={h.onSubmit ?? jest.fn()}
      onFormValues={h.onFormValues}
      onStaticChange={h.onStaticChange}
      onSelectionChange={h.onSelectionChange}
      renderChild={options.renderChild}
    />,
  );
}

describe('text / button', () => {
  it('text 三层级 + onClick 包裹两态', () => {
    const onAction = jest.fn();
    const { unmount } = renderPreview({
      node: {
        id: 't1',
        type: 'text',
        props: {
          content: '标题文',
          level: 'h2',
          onClick: { kind: 'showMessage', target: '', params: {} },
        },
      },
      handlers: { onAction },
    });
    expect(screen.getByRole('heading', { level: 4 })).toHaveTextContent('标题文');
    fireEvent.click(screen.getByText('标题文'));
    expect(onAction).toHaveBeenCalledTimes(1);
    unmount();

    renderPreview({ node: { id: 't2', type: 'text', props: { content: '小标题', level: 'h3' } } });
    expect(screen.getByRole('heading', { level: 5 })).toHaveTextContent('小标题');
  });

  it('text 正文与无 onClick 纯文本', () => {
    const onAction = jest.fn();
    renderPreview({
      node: { id: 't3', type: 'text', props: { content: '正文内容' } },
      handlers: { onAction },
    });
    fireEvent.click(screen.getByText('正文内容'));
    expect(onAction).not.toHaveBeenCalled();
  });

  it('text 无 content：?? 兜底空串', () => {
    const { container } = renderPreview({ node: { id: 't5', type: 'text', props: {} } });
    expect(container.textContent).toBe('');
  });

  it('button 三样式与动作派发', () => {
    const onAction = jest.fn();
    const { unmount } = renderPreview({
      node: {
        id: 'b1',
        type: 'button',
        props: {
          title: '主按钮',
          btnStyle: 'primary',
          onClick: { kind: 'closeModal', target: '', params: {} },
        },
      },
      handlers: { onAction },
    });
    const primary = screen.getByRole('button', { name: '主按钮' });
    expect(primary.className).toContain('ant-btn-primary');
    fireEvent.click(primary);
    expect(onAction).toHaveBeenCalledWith({ kind: 'closeModal', target: '', params: {} });
    unmount();

    renderPreview({
      node: { id: 'b2', type: 'button', props: { title: '危险', btnStyle: 'danger' } },
    });
    expect((screen.getByRole('button', { name: /危\s*险/ }) as HTMLElement).className).toContain(
      'ant-btn-dangerous',
    );
  });
});

describe('tabs', () => {
  const tabNode = (pages: PageNode[]): PageNode => ({
    id: 'tabs1',
    type: 'tabs',
    props: {},
    children: pages,
  });

  it('无页签：null（含 children 缺省的 ?? [] 空侧）', () => {
    const { container } = renderPreview({ node: tabNode([]) });
    expect(container).toBeEmptyDOMElement();
    // children 键缺省（undefined）同样回落空数组
    const { container: c2 } = renderPreview({ node: { id: 'tabs0', type: 'tabs', props: {} } });
    expect(c2).toBeEmptyDOMElement();
  });

  it('label 回退「页签 N」；子节点 renderChild 两路；空页签提示', () => {
    renderPreview({
      node: tabNode([
        {
          id: 'p1',
          type: 'container',
          props: {},
          children: [{ id: 'x1', type: 'text', props: { content: '页一内容' } }],
        },
        { id: 'p2', type: 'container', props: { title: '   ' }, children: [] },
      ]),
      renderChild: (c) => <PreviewNodeMock child={c} />,
    });
    expect(screen.getByText('页签 1')).toBeInTheDocument();
    expect(screen.getByText('页签 2')).toBeInTheDocument();
    // 页一默认激活：renderChild 渲染（非文本 type）
    expect(screen.getByText('CHILD:text')).toBeInTheDocument();
    // antd Tabs 懒渲染：非激活面板初始不挂载，切换后才有空页签提示
    fireEvent.click(screen.getByRole('tab', { name: '页签 2' }));
    expect(screen.getByText('空页签——拖入组件')).toBeInTheDocument();
  });

  it('非 container 子节点过滤；title 显式名', () => {
    renderPreview({
      node: {
        id: 'tabs2',
        type: 'tabs',
        props: {},
        children: [
          { id: 'txt', type: 'text', props: { content: '游离' } },
          { id: 'p1', type: 'container', props: { title: '设置页' }, children: [] },
        ],
      },
    });
    expect(screen.getByText('设置页')).toBeInTheDocument();
    expect(screen.queryByText('游离')).not.toBeInTheDocument();
    // 无 renderChild：子节点渲染为 type 文本
  });

  const PreviewNodeMock = ({ child }: { child: PageNode }) => ['CHILD:', child.type].join('');

  it('无 renderChild：子节点渲染为 type 文本', () => {
    renderPreview({
      node: tabNode([
        {
          id: 'p1',
          type: 'container',
          props: { title: '数据页' },
          children: [{ id: 'k', type: 'fnTable', props: {} }],
        },
      ]),
    });
    expect(screen.getByText('数据页')).toBeInTheDocument();
    expect(screen.getByText('fnTable')).toBeInTheDocument();
  });
});

describe('fnTable', () => {
  const tableNode = (props: Record<string, unknown>): PageNode => ({
    id: 'tb1',
    type: 'fnTable',
    props: { title: '玩家表', ...props },
  });
  const data = {
    result: {
      items: [
        { id: 'p1', name: '张三' },
        { id: 'p2', name: '李四' },
      ],
    },
  };

  it('schema 列渲染（截前 8）；执行按钮条件（autoRun=false）', () => {
    const onSubmit = jest.fn();
    renderPreview({ node: tableNode({}), fn, data, handlers: { onSubmit } });
    expect(screen.getByText('id')).toBeInTheDocument();
    expect(screen.getByText('张三')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: /执\s*行/ }));
    expect(onSubmit).toHaveBeenCalledWith({});
  });

  it('autoRun=true：无执行按钮', () => {
    renderPreview({ node: tableNode({ autoRun: true }), fn, data });
    expect(screen.queryByRole('button', { name: /执\s*行/ })).not.toBeInTheDocument();
  });

  it('声明列优先', () => {
    renderPreview({ node: tableNode({ columns: ['name'] }), fn, data });
    expect(screen.getByText('name')).toBeInTheDocument();
    expect(screen.queryByText('id')).not.toBeInTheDocument();
  });

  it('行操作列：label 兜底与显式、danger、点击派发', () => {
    const onRowAction = jest.fn();
    renderPreview({
      node: tableNode({
        rowActions: [
          { label: '封禁', danger: true },
          // 无 label 键 → 兜底「操作」（label:'' 非 nullish 不触发兜底）
          { nodeId: 'x' },
        ],
      }),
      fn,
      data,
      handlers: { onRowAction },
    });
    // 列头与行内兜底按钮同名「操作」，列头用 columnheader 定位
    expect(screen.getByRole('columnheader', { name: '操作' })).toBeInTheDocument();
    const row1 = screen.getByText('张三').closest('tr') as HTMLElement;
    const btns = [...row1.querySelectorAll('button')].filter(
      (b) => !/执\s*行/.test(String(b.textContent)),
    );
    const dangerBtn = btns.find((b) => b.textContent === '封禁') as HTMLElement;
    expect(dangerBtn.className).toContain('ant-btn-dangerous');
    fireEvent.click(dangerBtn);
    expect(onRowAction).toHaveBeenCalledWith(
      { label: '封禁', danger: true },
      { id: 'p1', name: '张三' },
    );
    fireEvent.click(btns.find((b) => b.textContent === '操作') as HTMLElement);
    expect(onRowAction).toHaveBeenLastCalledWith({ nodeId: 'x' }, { id: 'p1', name: '张三' });
  });

  it('行点击（onRowClick）与选中行（onSelectionChange）', () => {
    const onAction = jest.fn();
    const onSelectionChange = jest.fn();
    renderPreview({
      node: tableNode({ onRowClick: { kind: 'runBinding', target: 'f1', params: {} } }),
      fn,
      data,
      handlers: { onAction, onSelectionChange },
    });
    fireEvent.click(screen.getByText('李四'));
    expect(onAction).toHaveBeenCalledWith(
      { kind: 'runBinding', target: 'f1', params: {} },
      { id: 'p2', name: '李四' },
    );
    // 选中行由 radio 触发（行文本点击不派发 onChange）
    fireEvent.click(screen.getAllByRole('radio')[1]);
    expect(onSelectionChange).toHaveBeenCalledWith(expect.objectContaining({ id: 'tb1' }), [
      { id: 'p2', name: '李四' },
    ]);
  });

  it('无 onRowClick：行点击不派发 onAction', () => {
    const onAction = jest.fn();
    renderPreview({ node: tableNode({}), fn, data, handlers: { onAction } });
    fireEvent.click(screen.getByText('张三'));
    expect(onAction).not.toHaveBeenCalled();
  });
});

describe('fnFields', () => {
  const fieldsNode = (props: Record<string, unknown>): PageNode => ({
    id: 'ff1',
    type: 'fnFields',
    props: { title: '字段卡', ...props },
  });
  const data = { total: 5, items: [1], id: 'p1', name: '张三', meta: { vip: 3 }, nullv: null };

  it('items/total 过滤、对象 stringify、空值 -、截前 10', () => {
    renderPreview({ node: fieldsNode({}), fn, data });
    expect(screen.getByText('id')).toBeInTheDocument();
    expect(screen.getByText('张三')).toBeInTheDocument();
    expect(screen.getByText('{"vip":3}')).toBeInTheDocument();
    expect(screen.getByText('-')).toBeInTheDocument();
    expect(screen.queryByText('total')).not.toBeInTheDocument();
    expect(screen.queryByText('items')).not.toBeInTheDocument();
  });

  it('onClick 包裹整卡', () => {
    const onAction = jest.fn();
    renderPreview({
      node: fieldsNode({ onClick: { kind: 'showMessage', target: '', params: {} } }),
      fn,
      data,
      handlers: { onAction },
    });
    fireEvent.click(screen.getByText('张三'));
    expect(onAction).toHaveBeenCalledTimes(1);
  });
});

describe('fnForm / ModalForm', () => {
  it('行内渲染 SchemaFormRenderer；fn 无 properties → 确认执行按钮', () => {
    const { unmount } = renderPreview({
      node: { id: 'fm1', type: 'fnForm', props: { title: '查询表单' } },
      fn,
    });
    expect(screen.getByText('玩家ID')).toBeInTheDocument();
    unmount();

    renderPreview({
      node: { id: 'fm2', type: 'fnForm', props: {} },
      fn: { ...fn, inputSchema: { type: 'object' } },
    });
    expect(screen.getByRole('button', { name: '确认执行' })).toBeInTheDocument();
  });

  it('fnForm 值变化与提交：onFormValues / onSubmit 贯通', async () => {
    const onFormValues = jest.fn();
    const onSubmit = jest.fn();
    const { container } = renderPreview({
      node: { id: 'fm3', type: 'fnForm', props: {} },
      fn,
      handlers: { onFormValues, onSubmit },
    });
    fireEvent.change(screen.getByLabelText('玩家ID'), { target: { value: 'p100' } });
    expect(onFormValues).toHaveBeenCalledWith('fm3', { playerId: 'p100' });
    fireEvent.submit(container.querySelector('form') as HTMLFormElement);
    await waitFor(() => expect(onSubmit).toHaveBeenCalledWith({ playerId: 'p100' }));
  });

  it('ModalForm：fn undefined / schema 数组 → 确认执行；提交派发', () => {
    const onSubmit = jest.fn();
    const { unmount } = render(<ModalForm fn={undefined} running={false} onSubmit={onSubmit} />);
    fireEvent.click(screen.getByRole('button', { name: '确认执行' }));
    expect(onSubmit).toHaveBeenCalledWith({});
    unmount();

    render(
      <ModalForm fn={{ ...fn, inputSchema: [1] as never }} running={false} onSubmit={onSubmit} />,
    );
    expect(screen.getByRole('button', { name: '确认执行' })).toBeInTheDocument();
  });
});

describe('staticForm / StaticFormLive', () => {
  const staticSchema = JSON.stringify({
    type: 'object',
    properties: { level: { type: 'string', title: '等级' } },
  });

  it('坏 schema：警告文案', () => {
    const { unmount } = renderPreview({
      node: { id: 'sf1', type: 'staticForm', props: { staticSchema: 'not-json' } },
    });
    expect(screen.getByText('字段定义 JSON 无效')).toBeInTheDocument();
    unmount();
    // staticSchema 缺省：解析结果 falsy → 同样无效
    renderPreview({ node: { id: 'sf1b', type: 'staticForm', props: {} } });
    expect(screen.getByText('字段定义 JSON 无效')).toBeInTheDocument();
  });

  it('经 PreviewNode：防抖 → onStaticChange(node.id, values)', () => {
    jest.useFakeTimers();
    const onStaticChange = jest.fn();
    renderPreview({
      node: { id: 'sf4', type: 'staticForm', props: { staticSchema } },
      handlers: { onStaticChange },
    });
    fireEvent.change(screen.getByLabelText('等级'), { target: { value: 'v9' } });
    act(() => jest.advanceTimersByTime(300));
    expect(onStaticChange).toHaveBeenCalledWith('sf4', { level: 'v9' });
    jest.useRealTimers();
  });

  it('schema 对象形态（非字符串）同样渲染', () => {
    render(
      <StaticFormLive
        node={{ id: 'sf2', type: 'staticForm', props: { staticSchema: { type: 'object' } } }}
        onChange={jest.fn()}
      />,
    );
    expect(screen.queryByText('字段定义 JSON 无效')).not.toBeInTheDocument();
  });

  it('无 onChange：值变化经防抖不炸（?. 空侧）', () => {
    jest.useFakeTimers();
    const { unmount } = render(
      <StaticFormLive node={{ id: 'sf5', type: 'staticForm', props: { staticSchema } }} />,
    );
    fireEvent.change(screen.getByLabelText('等级'), { target: { value: 'v1' } });
    act(() => jest.advanceTimersByTime(300));
    expect(screen.getByLabelText('等级')).toBeInTheDocument();
    unmount();
    jest.useRealTimers();
  });

  it('值变化防抖 → onChange；卸载清 timer 不派发', async () => {
    jest.useFakeTimers();
    const onChange = jest.fn();
    const { unmount } = render(
      <StaticFormLive
        node={{ id: 'sf3', type: 'staticForm', props: { staticSchema } }}
        onChange={onChange}
      />,
    );
    const input = screen.getByLabelText('等级');
    fireEvent.change(input, { target: { value: 'vip2' } });
    // 未到防抖窗口：不派发
    expect(onChange).not.toHaveBeenCalled();
    act(() => jest.advanceTimersByTime(300));
    expect(onChange).toHaveBeenCalledWith({ level: 'vip2' });

    // 卸载清 timer：改值后立即卸载，不再派发
    fireEvent.change(input, { target: { value: 'vip3' } });
    unmount();
    act(() => jest.advanceTimersByTime(300));
    expect(onChange).toHaveBeenCalledTimes(1);
    jest.useRealTimers();
  });
});

describe('container', () => {
  it('子节点 renderChild 两路与空容器提示', () => {
    const { unmount } = renderPreview({
      node: {
        id: 'c1',
        type: 'container',
        props: {},
        children: [{ id: 'k1', type: 'fnFields', props: {} }],
      },
      renderChild: (c) => <>{['RC:', c.type].join('')}</>,
    });
    expect(screen.getByText('RC:fnFields')).toBeInTheDocument();
    unmount();

    renderPreview({
      node: {
        id: 'c2',
        type: 'container',
        props: {},
        children: [{ id: 'k', type: 'text', props: {} }],
      },
    });
    expect(screen.getByText('text')).toBeInTheDocument();
  });

  it('空容器提示（children 缺省同样回落）', () => {
    const { unmount } = renderPreview({
      node: { id: 'c3', type: 'container', props: {}, children: [] },
    });
    expect(screen.getByText('空容器')).toBeInTheDocument();
    unmount();
    // children 键缺省（undefined）→ ?? [] 空侧
    renderPreview({ node: { id: 'c4', type: 'container', props: {} } });
    expect(screen.getByText('空容器')).toBeInTheDocument();
  });

  it('未识别类型：Card 空体（null 分支）', () => {
    const { container } = renderPreview({
      node: { id: 'u1', type: 'modal' as PageNode['type'], props: {} },
    });
    expect(container.querySelector('.ant-card')).not.toBeNull();
  });
});

// waitFor 引用保持（异步路径场景备用）
void waitFor;
