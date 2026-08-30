import { render, screen, fireEvent } from '@testing-library/react';
import { App } from 'antd';
import Canvas from '../Canvas';
import type { PageNode } from '../model';

const formNode: PageNode = {
  id: 'form-1',
  type: 'fnForm',
  props: { functionId: 'mail.send', title: '发邮件', display: 'inline' },
};

const modalNode: PageNode = {
  id: 'modal-1',
  type: 'modal',
  props: { title: '发邮件弹窗', width: 'medium' },
  children: [formNode],
};

const tableNode: PageNode = {
  id: 'table-1',
  type: 'fnTable',
  props: { functionId: 'player.list', title: '玩家列表', span: 24, autoRun: true },
};

function setup(over?: { selectedId?: string | null }) {
  const calls: string[] = [];
  const utils = render(
    <App>
      <Canvas
        tree={[tableNode, modalNode]}
        selectedId={over?.selectedId ?? null}
        fnById={new Map()}
        onSelect={(id) => calls.push(id)}
        onDelete={() => undefined}
        onDuplicate={() => undefined}
        onSpanChange={() => undefined}
        canvasWidthRef={{ current: null }}
      >
        <div>canvas-children</div>
      </Canvas>
    </App>,
  );
  return { calls, ...utils };
}

describe('Canvas 弹窗收纳区（编辑死穴修复的可测面）', () => {
  it('显示弹窗与其内部表单（函数 id 可见，不再只有摘要）', () => {
    setup();
    expect(screen.getByText('发邮件弹窗')).toBeInTheDocument();
    expect(screen.getByText('mail.send')).toBeInTheDocument();
  });

  it('点击收纳卡片选中弹窗；点击内部表单卡片选中表单节点', () => {
    const { calls } = setup();
    fireEvent.click(screen.getByText('发邮件弹窗'));
    expect(calls).toContain('modal-1');
    fireEvent.click(screen.getByText('mail.send'));
    expect(calls).toContain('form-1');
  });

  it('空弹窗显示拖入引导文案', () => {
    const emptyModal: PageNode = { id: 'm2', type: 'modal', props: { title: '空' }, children: [] };
    render(
      <App>
        <Canvas
          tree={[emptyModal]}
          selectedId={null}
          fnById={new Map()}
          onSelect={() => undefined}
          onDelete={() => undefined}
          onDuplicate={() => undefined}
          onSpanChange={() => undefined}
          canvasWidthRef={{ current: null }}
        >
          <div>children</div>
        </Canvas>
      </App>,
    );
    expect(screen.getByText(/从函数组件拖入表单/)).toBeInTheDocument();
  });
});
