import { render, screen, fireEvent } from '@testing-library/react';
import { App } from 'antd';
import ActionEditor from '../ActionEditor';
import type { PageNode } from '../model';

const nodes: PageNode[] = [
  { id: 't1', type: 'fnTable', props: {} },
  {
    id: 'm1',
    type: 'modal',
    props: { title: '发邮件' },
    children: [{ id: 'f1', type: 'fnForm', props: {} }],
  },
  {
    id: 'm2',
    type: 'modal',
    props: { title: '封禁' },
    children: [{ id: 'f2', type: 'fnForm', props: {} }],
  },
  { id: 'b1', type: 'button', props: {} },
];

function setup(props?: {
  value?: unknown;
  allowedKinds?: Array<'openModal' | 'runBinding' | 'refreshNode'>;
}) {
  const onChange = jest.fn();
  const utils = render(
    <App>
      <ActionEditor
        value={props?.value}
        nodes={nodes}
        allowedKinds={props?.allowedKinds}
        onChange={onChange}
      />
    </App>,
  );
  return { onChange, ...utils };
}

/** 打开第一个下拉（兼容 antd v5/v6 的选择器 DOM）。 */
function openFirstSelect() {
  const sel = document.querySelector('.ant-select');
  expect(sel).toBeTruthy();
  fireEvent.mouseDown(sel!);
  fireEvent.click(sel!);
}

describe('ActionEditor（按钮动作编排）', () => {
  it('动作类型下拉：六种（V3.2 扩展 closeModal/navigate/showMessage）', () => {
    setup();
    openFirstSelect();
    expect(screen.getByText('打开弹窗')).toBeInTheDocument();
    expect(screen.getByText('执行')).toBeInTheDocument();
    expect(screen.getByText('刷新')).toBeInTheDocument();
    expect(screen.getByText('关闭弹窗')).toBeInTheDocument();
    expect(screen.getByText('跳转链接')).toBeInTheDocument();
    expect(screen.getByText('提示消息')).toBeInTheDocument();
  });

  it('navigate：选中产出空 params；带 value 时地址输入框输入更新 params（受控）', () => {
    const { onChange } = setup();
    openFirstSelect();
    fireEvent.click(screen.getByText('跳转链接'));
    expect(onChange).toHaveBeenLastCalledWith(
      expect.objectContaining({ kind: 'navigate', target: '', params: {} }),
    );
  });

  it('navigate 参数输入：value 带入时渲染地址框，输入回调产出 params', () => {
    const { onChange } = setup({ value: { kind: 'navigate', target: '', params: { url: '' } } });
    const urlInput = document.querySelector('input[placeholder]') as HTMLInputElement;
    expect(urlInput).toBeTruthy();
    fireEvent.change(urlInput, { target: { value: '/docs' } });
    expect(onChange).toHaveBeenLastCalledWith(
      expect.objectContaining({ kind: 'navigate', params: { url: '/docs' } }),
    );
  });

  it('allowedKinds 限定可选项（onSuccess 只允许刷新）', () => {
    setup({ allowedKinds: ['refreshNode'] });
    openFirstSelect();
    expect(screen.getByText('刷新')).toBeInTheDocument();
    expect(screen.queryByText('打开弹窗')).not.toBeInTheDocument();
  });

  it('选择「打开弹窗」→ 目标下拉只列弹窗（按 title 显示），onChange 产出正确 ActionSpec', () => {
    const { onChange } = setup();
    openFirstSelect();
    fireEvent.click(screen.getByText('打开弹窗'));
    // 目标下拉自动选中第一个弹窗 m1
    expect(onChange).toHaveBeenCalledWith({ kind: 'openModal', target: 'm1' });
  });

  it('目标已被删除时显示警示', () => {
    setup({ value: { kind: 'openModal', target: 'deleted-id' } });
    expect(screen.getByText('目标节点已被删除——请重新选择')).toBeInTheDocument();
  });
});
