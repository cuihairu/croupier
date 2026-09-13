/** InsertTemplateModal（带参模板插入配置）覆盖：关闭态（null 不渲染）、
 * 标题 name 插值与回退 key、params 三控件（autoRun→Switch checked·
 * span→InputNumber·其他→Input）、label 回退 key、default 初值、
 * 确认（值收集→onConfirm 契约→onClose+重置）、取消、无 params 空表单。 */
import React from 'react';
import { fireEvent, render, screen } from '@testing-library/react';
import InsertTemplateModal from '../InsertTemplateModal';
import type { ComponentTemplateDTO } from '../ComponentLibrary';

const tpl = (params?: ComponentTemplateDTO['params']): ComponentTemplateDTO => ({
  key: 'player-card',
  name: { 'zh-CN': '玩家卡片', 'en-US': 'Player Card' },
  tree: [],
  builtin: false,
  params,
});

function renderModal(state: { tpl: ComponentTemplateDTO; overId: string } | null) {
  const onConfirm = jest.fn();
  const onClose = jest.fn();
  const utils = render(
    <InsertTemplateModal tplState={state} onClose={onClose} onConfirm={onConfirm} />,
  );
  return { onConfirm, onClose, ...utils };
}

describe('渲染', () => {
  it('tplState null：弹窗关闭（内容不渲染）', () => {
    renderModal(null);
    expect(screen.queryByText('插入')).not.toBeInTheDocument();
    expect(screen.queryByRole('switch')).not.toBeInTheDocument();
  });

  it('标题 name 插值；params 三控件与 label 回退', () => {
    renderModal({
      tpl: tpl([
        { key: 'auto', nodeId: 'n1', prop: 'autoRun', default: true },
        { key: '宽度', nodeId: 'n1', prop: 'span', default: 12 },
        {
          key: 'title',
          label: { 'zh-CN': '标题' },
          nodeId: 'n1',
          prop: 'props.title',
          default: '玩家',
        },
      ]),
      overId: 'canvas-root',
    });
    expect(screen.getByText('配置组件参数：玩家卡片')).toBeInTheDocument();
    // autoRun → Switch（default true → checked）
    const sw = screen.getByRole('switch') as HTMLInputElement;
    expect(sw).toBeChecked();
    // span → InputNumber（default 12）
    const num = screen.getByRole('spinbutton') as HTMLInputElement;
    expect(num.value).toBe('12');
    // 无 label 的参数回退 key；有 label 用 label
    expect(screen.getByText('auto')).toBeInTheDocument();
    expect(screen.getByText('宽度')).toBeInTheDocument();
    expect(screen.getByText('标题')).toBeInTheDocument();
    // 其他 prop → Input（maxLength 60）
    const text = screen.getByDisplayValue('玩家') as HTMLInputElement;
    expect(text.maxLength).toBe(60);
  });

  it('name 无 zh-CN：回退 key', () => {
    renderModal({ tpl: { ...tpl(), name: {} }, overId: 'x' });
    expect(screen.getByText('配置组件参数：player-card')).toBeInTheDocument();
  });

  it('无 params：空表单（无控件）', () => {
    renderModal({ tpl: tpl(), overId: 'x' });
    expect(screen.queryByRole('switch')).not.toBeInTheDocument();
    expect(screen.queryByRole('spinbutton')).not.toBeInTheDocument();
  });
});

describe('确认与取消', () => {
  const state = () => ({
    tpl: tpl([
      { key: 'auto', nodeId: 'n1', prop: 'autoRun', default: false },
      { key: 'title', nodeId: 'n1', prop: 'props.title', default: '旧值' },
    ]),
    overId: 'container-1',
  });

  it('确认：收集表单值 → onConfirm 契约 → onClose', () => {
    const { onConfirm, onClose } = renderModal(state());
    // 改 Switch 与 Input
    fireEvent.click(screen.getByRole('switch'));
    fireEvent.change(screen.getByDisplayValue('旧值'), { target: { value: '新值' } });
    fireEvent.click(screen.getByText('插 入'));
    expect(onConfirm).toHaveBeenCalledWith(
      expect.objectContaining({ key: 'player-card' }),
      { auto: true, title: '新值' },
      'container-1',
    );
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('未改动：default 值原样提交', () => {
    const { onConfirm } = renderModal(state());
    fireEvent.click(screen.getByText('插 入'));
    expect(onConfirm).toHaveBeenCalledWith(
      expect.anything(),
      { auto: false, title: '旧值' },
      'container-1',
    );
  });

  it('取消：仅 onClose', () => {
    const { onConfirm, onClose } = renderModal(state());
    fireEvent.click(screen.getByText('取 消'));
    expect(onClose).toHaveBeenCalledTimes(1);
    expect(onConfirm).not.toHaveBeenCalled();
  });
});
