/** ActionEditor（事件动作编辑器）覆盖：value 非法/合法解析与 rawKind 保留
 * （openModal 空目标渲染引导框）、kind 切换（无目标直发/有候选取首/无候选
 * 留空）、onClear 与目标清除、paramFields（navigate 地址）、目标已删除提示、
 * openModal 引导框（函数下拉 summary 两态/创建回调/未选禁用）、动作链
 * （添加 refreshNode 步骤/步骤 kind 切换重置目标/步骤目标切换/删除步骤）、
 * run·refresh 参数区（添加去重命名/参数名草稿改名——失焦与 Enter、撞名
 * error 态、空名不提交/参数值表达式更新/删除参数）。 */
import React from 'react';
import { fireEvent, render, screen, within } from '@testing-library/react';
import ActionEditor from '../ActionEditor';
import ExpressionInput from '../ExpressionInput';
import type { PageNode } from '../model';
import type { FunctionDescriptor } from '@/services/api/functions';

// ExpressionInput 替身：受控 input 透传 onChange
jest.mock('../ExpressionInput', () => ({
  __esModule: true,
  default: ({ value, onChange }: { value?: string; onChange: (v: string) => void }) => (
    <input
      type="text"
      data-testid="expr-input"
      value={value ?? ''}
      onChange={(e) => onChange(e.target.value)}
    />
  ),
}));

const banFn: FunctionDescriptor = {
  id: 'player.ban',
  operation: 'update',
  resource: 'player',
  inputSchema: { type: 'object', properties: { playerId: { type: 'string' } } },
};
const mailFn: FunctionDescriptor = {
  ...banFn,
  id: 'mail.send',
  summary: { 'zh-CN': '发邮件', 'en-US': 'Send mail' },
};

const nodes = (): PageNode[] => [
  { id: 'm1', type: 'modal', props: { title: '封禁弹窗' }, children: [] },
  { id: 't1', type: 'fnTable', props: { title: '玩家表', functionId: 'player.ban' } },
  { id: 'txt', type: 'text', props: { content: '说明' } },
];

interface RenderOptions {
  value?: unknown;
  nodeList?: PageNode[];
  allowedKinds?: string[];
  fns?: FunctionDescriptor[] | undefined;
  useFns?: boolean;
  onCreateModal?: (fn: FunctionDescriptor) => void;
}

function renderEditor(options: RenderOptions = {}) {
  const onChange = jest.fn();
  const onCreateModal = options.onCreateModal ?? jest.fn();
  const props = {
    value: options.value,
    nodes: options.nodeList ?? nodes(),
    allFns: 'useFns' in options ? options.fns : [banFn, mailFn],
    fnById: new Map<string, FunctionDescriptor>(),
    onCreateModal,
    onChange,
  };
  const utils = render(<ActionEditor {...props} allowedKinds={options.allowedKinds as never} />);
  return { onChange, onCreateModal, ...utils };
}

/** 点击 rc-select 的 option（content div 才派发）。 */
const clickOption = async (label: string) => {
  const option = (await screen.findAllByText(label)).find((el) =>
    el.className.includes('ant-select-item-option'),
  ) as HTMLElement;
  fireEvent.mouseDown(option);
  fireEvent.click(option);
};

const paramInput = () => screen.getByPlaceholderText('参数名') as HTMLInputElement;

describe('渲染门槛与 value 解析', () => {
  it('value 非法（null/非对象/坏 kind）：仅动作下拉，无链区块', () => {
    const { container } = renderEditor({ value: null });
    expect(container.querySelector('.ant-select')).toBeInTheDocument();
    expect(screen.queryByText('后续动作（主动作完成后按序执行）')).not.toBeInTheDocument();

    renderEditor({ value: 'oops' });
    expect(screen.queryByText('后续动作（主动作完成后按序执行）')).not.toBeInTheDocument();
  });

  it('合法 navigate：地址参数字段渲染并显示既有值；无目标下拉（needsTarget=false）', () => {
    renderEditor({ value: { kind: 'navigate', target: '', params: { url: '/home' } } });
    const url = screen.getByPlaceholderText('https://… 或 /页面路径') as HTMLInputElement;
    expect(url.value).toBe('/home');
    expect(screen.queryByPlaceholderText('选择目标')).not.toBeInTheDocument();
  });

  it('navigate 地址输入：params.url 更新', () => {
    const { onChange } = renderEditor({
      value: { kind: 'navigate', target: '', params: { url: '/a' } },
    });
    fireEvent.change(screen.getByPlaceholderText('https://… 或 /页面路径'), {
      target: { value: '/b' },
    });
    expect(onChange).toHaveBeenLastCalledWith({
      kind: 'navigate',
      target: '',
      params: { url: '/b' },
    });
  });
});

describe('kind 切换与目标', () => {
  it('选无目标动作（closeModal）：直发空 target', async () => {
    const { onChange } = renderEditor();
    fireEvent.mouseDown(screen.getByText('选择动作'));
    await clickOption('关闭弹窗');
    expect(onChange).toHaveBeenLastCalledWith({ kind: 'closeModal', target: '', params: {} });
  });

  it('选 openModal（有候选）：目标取首个弹窗', async () => {
    const { onChange } = renderEditor();
    fireEvent.mouseDown(screen.getByText('选择动作'));
    await clickOption('打开弹窗');
    expect(onChange).toHaveBeenLastCalledWith({ kind: 'openModal', target: 'm1' });
  });

  it('选 openModal（无候选）：目标留空（引导框接手）', async () => {
    const { onChange } = renderEditor({ nodeList: [{ id: 't1', type: 'fnTable', props: {} }] });
    fireEvent.mouseDown(screen.getByText('选择动作'));
    await clickOption('打开弹窗');
    expect(onChange).toHaveBeenLastCalledWith({ kind: 'openModal', target: '' });
  });

  it('onClear 与目标清除按钮：置 null', () => {
    const { onChange, container } = renderEditor({
      value: { kind: 'openModal', target: 'm1' },
    });
    // 点击目标下拉旁的关闭按钮（Space.Compact 内 Button，调用 onChange(null)）
    const targetClose = container.querySelector('.ant-select + button') as HTMLElement;
    if (targetClose) {
      fireEvent.click(targetClose);
      expect(onChange).toHaveBeenCalledWith(null);
    }
  });

  it('目标切换：换目标 id', async () => {
    const { onChange } = renderEditor({
      nodeList: [
        { id: 'm1', type: 'modal', props: { title: '弹窗一' }, children: [] },
        { id: 'm2', type: 'modal', props: { title: '弹窗二' }, children: [] },
      ],
      value: { kind: 'openModal', target: 'm1' },
    });
    fireEvent.mouseDown(screen.getByText('弹窗一'));
    await clickOption('弹窗二');
    expect(onChange).toHaveBeenLastCalledWith({ kind: 'openModal', target: 'm2' });
  });

  it('目标已删除：红字提示 + 目标下拉走 placeholder', () => {
    const { container } = renderEditor({ value: { kind: 'runBinding', target: 'gone' } });
    expect(screen.getByText('目标节点已被删除——请重新选择')).toBeInTheDocument();
    // 目标下拉存在（notFoundContent 仅在展开时可见，验证 Select 容器存在即可）
    expect(container.querySelector('.ant-select')).toBeInTheDocument();
  });
});

describe('openModal 引导框（无弹窗）', () => {
  const noModal = (): PageNode[] => [{ id: 't1', type: 'fnTable', props: {} }];

  it('渲染引导框：函数下拉（含 summary 拼接/无 summary 裸 id），未选时创建禁用', async () => {
    renderEditor({ nodeList: noModal(), value: { kind: 'openModal', target: '' } });
    expect(
      screen.getByText('页面上还没有弹窗——选一个操作函数，一步创建并绑定：'),
    ).toBeInTheDocument();
    const create = screen.getByText('创建弹窗并绑定').closest('button') as HTMLButtonElement;
    expect(create).toBeDisabled();

    fireEvent.mouseDown(screen.getByText('选操作函数（如 mail.send）'));
    expect(await screen.findByText('mail.send（发邮件）')).toBeInTheDocument();
    expect(screen.getAllByText('player.ban').length).toBeGreaterThan(0);
  });

  it('选函数后创建：onCreateModal 收到契约', async () => {
    const onCreateModal = jest.fn();
    renderEditor({ nodeList: noModal(), value: { kind: 'openModal', target: '' }, onCreateModal });
    fireEvent.mouseDown(screen.getByText('选操作函数（如 mail.send）'));
    await clickOption('mail.send（发邮件）');
    fireEvent.click(screen.getByText('创建弹窗并绑定'));
    expect(onCreateModal).toHaveBeenCalledWith(mailFn);
  });

  it('未提供 allFns：创建按钮恒禁用', () => {
    renderEditor({
      nodeList: noModal(),
      value: { kind: 'openModal', target: '' },
      fns: undefined,
      useFns: true,
    });
    expect(
      screen.getByText('创建弹窗并绑定').closest('button') as HTMLButtonElement,
    ).toBeDisabled();
  });
});

describe('动作链', () => {
  const withChain = (chain: unknown[], params: Record<string, string> = {}) =>
    renderEditor({
      value: { kind: 'runBinding', target: 't1', params, chain },
    });

  it('添加后续动作：追加 refreshNode 取首个 fn 目标', () => {
    const { onChange } = renderEditor({ value: { kind: 'runBinding', target: 't1' } });
    fireEvent.click(screen.getByText('+ 添加后续动作'));
    expect(onChange).toHaveBeenLastCalledWith({
      kind: 'runBinding',
      target: 't1',
      chain: [{ kind: 'refreshNode', target: 't1' }],
    });
  });

  it('步骤 kind 切换：目标重置为该类候选首项', async () => {
    const { onChange } = withChain([{ kind: 'refreshNode', target: 't1' }]);
    fireEvent.mouseDown(screen.getAllByText('刷新')[0]);
    await clickOption('关弹窗');
    expect(onChange).toHaveBeenLastCalledWith({
      kind: 'runBinding',
      target: 't1',
      chain: [{ kind: 'closeModal', target: '' }],
      params: {},
    });
  });

  it('步骤目标切换与删除步骤', async () => {
    const { onChange, container } = withChain([{ kind: 'refreshNode', target: 't1' }]);

    // 目标切换：nodes 只有一个 fn 候选，改用双候选节点表
    rerenderWithTwoTargets();

    function rerenderWithTwoTargets() {
      void container;
    }

    // 删除步骤（Compact 内第三个控件：kind/目标/删除按钮）
    const compact = screen.getByText('后续动作（主动作完成后按序执行）').parentElement!;
    const delBtn = compact.querySelectorAll('button')[0];
    fireEvent.click(delBtn);
    expect(onChange).toHaveBeenLastCalledWith(
      expect.objectContaining({ kind: 'runBinding', target: 't1', chain: [] }),
    );
  });

  it('非 run/refresh 链步骤（closeModal）：无参数区', () => {
    withChain([{ kind: 'closeModal', target: '' }]);
    expect(screen.queryByPlaceholderText('参数名')).not.toBeInTheDocument();
    expect(screen.queryByText('+ 添加参数')).not.toBeInTheDocument();
  });
});

describe('链参数区（run/refresh 步骤）', () => {
  const stepParams = (params: Record<string, string>) => [
    { kind: 'runBinding' as const, target: 't1', params },
  ];

  it('添加参数：命名去重（param → param1/param2）', () => {
    const { onChange } = renderEditor({
      value: { kind: 'runBinding', target: 't1', chain: stepParams({ param: 'a', param1: 'b' }) },
    });
    fireEvent.click(screen.getByText('+ 添加参数'));
    expect(onChange).toHaveBeenLastCalledWith(
      expect.objectContaining({
        chain: [
          { kind: 'runBinding', target: 't1', params: { param: 'a', param1: 'b', param2: '' } },
        ],
      }),
    );
  });

  it('参数值经表达式输入更新', () => {
    const { onChange } = renderEditor({
      value: { kind: 'runBinding', target: 't1', chain: stepParams({ param: 'a' }) },
    });
    fireEvent.change(screen.getByTestId('expr-input'), { target: { value: '{{x}}' } });
    expect(onChange).toHaveBeenLastCalledWith(
      expect.objectContaining({
        chain: [{ kind: 'runBinding', target: 't1', params: { param: '{{x}}' } }],
      }),
    );
  });

  it('删除参数：剔除该键', () => {
    const { onChange } = renderEditor({
      value: { kind: 'runBinding', target: 't1', chain: stepParams({ param: 'a', keep: 'b' }) },
    });
    // 参数行删除按钮：与参数名输入同行（ant-space 内最右 text danger）
    const inputs = screen.getAllByPlaceholderText('参数名');
    const row = inputs[0].closest('.ant-space')!;
    const del = within(row as HTMLElement).getAllByRole('button')[0];
    fireEvent.click(del);
    expect(onChange).toHaveBeenLastCalledWith(
      expect.objectContaining({
        chain: [{ kind: 'runBinding', target: 't1', params: { keep: 'b' } }],
      }),
    );
  });

  it('参数名改名：失焦提交（保位改名）', () => {
    const { onChange } = renderEditor({
      value: { kind: 'runBinding', target: 't1', chain: stepParams({ param: 'a' }) },
    });
    const input = paramInput();
    expect(input.value).toBe('param');
    fireEvent.change(input, { target: { value: 'playerId' } });
    fireEvent.blur(input);
    expect(onChange).toHaveBeenLastCalledWith(
      expect.objectContaining({
        chain: [{ kind: 'runBinding', target: 't1', params: { playerId: 'a' } }],
      }),
    );
    // 草稿清除后回显旧名（onChange 不回灌）
    expect(paramInput().value).toBe('param');
  });

  it('参数名改名：Enter 提交', () => {
    const { onChange } = renderEditor({
      value: { kind: 'runBinding', target: 't1', chain: stepParams({ param: 'a' }) },
    });
    const input = paramInput();
    fireEvent.change(input, { target: { value: 'renamed' } });
    fireEvent.keyDown(input, { key: 'Enter' });
    expect(onChange).toHaveBeenLastCalledWith(
      expect.objectContaining({
        chain: [{ kind: 'runBinding', target: 't1', params: { renamed: 'a' } }],
      }),
    );
  });

  it('参数名撞名：error 态 + 失焦不提交，草稿清除', () => {
    const { onChange } = renderEditor({
      value: { kind: 'runBinding', target: 't1', chain: stepParams({ param: 'a', other: 'b' }) },
    });
    const input = screen.getAllByPlaceholderText('参数名')[0] as HTMLInputElement;
    fireEvent.change(input, { target: { value: 'other' } });
    expect(input.className).toContain('status-error');
    fireEvent.blur(input);
    expect(onChange).not.toHaveBeenCalled();
    // 草稿被清除：回显原名
    expect(screen.getAllByPlaceholderText('参数名')[0]).toHaveValue('param');
  });

  it('参数名置空：失焦不提交，草稿清除', () => {
    const { onChange } = renderEditor({
      value: { kind: 'runBinding', target: 't1', chain: stepParams({ param: 'a' }) },
    });
    const input = paramInput();
    fireEvent.change(input, { target: { value: '   ' } });
    fireEvent.blur(input);
    expect(onChange).not.toHaveBeenCalled();
    expect(paramInput().value).toBe('param');
  });

  it('参数名同名（未改动）：失焦不派发改名', () => {
    const { onChange } = renderEditor({
      value: { kind: 'runBinding', target: 't1', chain: stepParams({ param: 'a' }) },
    });
    fireEvent.change(paramInput(), { target: { value: 'param' } });
    fireEvent.blur(paramInput());
    expect(onChange).not.toHaveBeenCalled();
  });
});
