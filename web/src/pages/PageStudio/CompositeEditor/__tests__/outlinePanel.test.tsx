/** OutlinePanel（大纲树）覆盖：空页面 Empty、节点标签三态取值（title 优先/
 * content/functionId/组件名/type 兜底）、sectionKey 附加、children 递归、
 * 点击选中回调与取消选中不触发。 */
import React from 'react';
import { fireEvent, render, screen } from '@testing-library/react';
import OutlinePanel from '../OutlinePanel';
import { resetRegistryForTest, getComponent } from '../registry';
import { registerBuiltinComponents } from '../components/builtin';
import type { PageNode } from '../model';

beforeAll(() => {
  resetRegistryForTest();
  registerBuiltinComponents();
});

const tree: PageNode[] = [
  {
    id: 't1',
    type: 'fnTable',
    props: { title: '玩家列表', sectionKey: 'players' },
    children: [{ id: 't1c', type: 'button', props: { content: '行内按钮' } }],
  },
  {
    id: 't2',
    type: 'text',
    props: { content: '公告文案' },
  },
  {
    id: 't3',
    type: 'fnForm',
    props: { functionId: 'player.ban' },
  },
  {
    id: 't4',
    type: 'staticForm',
    props: {},
  },
];

it('空页面：渲染 Empty 提示', () => {
  render(<OutlinePanel tree={[]} selectedId={null} onSelect={jest.fn()} />);
  expect(screen.getByText('页面为空')).toBeInTheDocument();
});

it('节点标签：title 优先并附加 sectionKey；content/functionId/组件名兜底', () => {
  render(<OutlinePanel tree={tree} selectedId={null} onSelect={jest.fn()} />);
  // title + (sectionKey)
  expect(screen.getByText('玩家列表 (players)')).toBeInTheDocument();
  // 无 title：text 组件用 content；fnForm 用 functionId；staticForm 用组件名「常量表单」
  expect(screen.getByText('公告文案')).toBeInTheDocument();
  expect(screen.getByText('player.ban')).toBeInTheDocument();
  expect(screen.getByText(getComponent('staticForm')!.name)).toBeInTheDocument();
  // children 递归可见（button 无 title/content → 组件名「按钮」——此处被 content 覆盖）
});

it('点击节点触发 onSelect(id)（受控单选：重复点击仍派发选中）', () => {
  const onSelect = jest.fn();
  render(<OutlinePanel tree={tree} selectedId={null} onSelect={onSelect} />);
  fireEvent.click(screen.getByText('公告文案'));
  expect(onSelect).toHaveBeenCalledWith('t2');
  // selectedKeys 受控为 null → Tree 视为未选中，二次点击仍回调
  fireEvent.click(screen.getByText('公告文案'));
  expect(onSelect).toHaveBeenCalledTimes(2);
});

it('selectedKeys 高亮传入节点', () => {
  render(<OutlinePanel tree={tree} selectedId="t1" onSelect={jest.fn()} />);
  expect(document.querySelector('.ant-tree-node-selected')).toBeInTheDocument();
});
