import { render, screen, fireEvent } from '@testing-library/react';
import { App } from 'antd';
import { ModalPlaceholder } from '../Canvas';
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

const emptyModal: PageNode = { id: 'm2', type: 'modal', props: { title: '空' }, children: [] };

function setup(modal: PageNode, calls: { select: string[]; enter: string[] }) {
  render(
    <App>
      <ModalPlaceholder
        modal={modal}
        selected={false}
        fnById={new Map()}
        onSelect={() => calls.select.push(modal.id)}
        onEnterModal={() => calls.enter.push(modal.id)}
      />
    </App>,
  );
}

describe('ModalPlaceholder（弹窗占位卡：D 项内嵌编辑的可测面）', () => {
  it('显示弹窗标题与内部表单（函数 id 可见）', () => {
    const calls = { select: [], enter: [] };
    setup(modalNode, calls);
    expect(screen.getByText('发邮件弹窗')).toBeInTheDocument();
    expect(screen.getByText('mail.send')).toBeInTheDocument();
  });

  it('单击选中弹窗；双击/按钮进入内部编辑', () => {
    const calls = { select: [], enter: [] };
    setup(modalNode, calls);
    fireEvent.click(screen.getByText('发邮件弹窗'));
    expect(calls.select).toContain('modal-1');
    fireEvent.doubleClick(screen.getByText('发邮件弹窗'));
    expect(calls.enter).toContain('modal-1');
    fireEvent.click(screen.getByText('进入弹窗编辑 →'));
    expect(calls.enter).toHaveLength(2);
  });

  it('空弹窗显示拖入/进入引导文案', () => {
    const calls = { select: [], enter: [] };
    setup(emptyModal, calls);
    expect(screen.getByText(/拖入函数表单/)).toBeInTheDocument();
  });
});
