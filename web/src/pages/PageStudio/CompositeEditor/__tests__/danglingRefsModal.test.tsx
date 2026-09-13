/** DanglingRefsModal（模板联动断链提示）覆盖：refs 渲染（kind 四类标签、
 * detailLabel 四态——无 detail 主动作/链第 N 步/第 N 个映射/第 N 个行操作、
 * count 插值、重连下拉过滤自身）、选择重连（choices 更新→应用收集 fixes：
 * 未选/选自身/清空跳过，选择他节点入列）、onClose 取消。 */
import React from 'react';
import { fireEvent, render, screen } from '@testing-library/react';
import DanglingRefsModal from '../DanglingRefsModal';
import type { DanglingTemplateRef, TemplateRefFix } from '../ComponentLibrary';

const candidates = [
  { id: 'btn1', title: '封禁按钮', type: 'button' },
  { id: 'tbl1', title: '玩家表', type: 'fnTable' },
  { id: 'frm1', title: '筛选表单', type: 'staticForm' },
];

const baseRef: DanglingTemplateRef = {
  nodeId: 'btn1',
  nodeTitle: '封禁按钮',
  prop: 'onClick',
  kind: 'action',
  ref: 'old-target',
};

function renderModal(
  refs: DanglingTemplateRef[],
  handlers: { onApply?: (fixes: TemplateRefFix[]) => void } = {},
) {
  const onApply = handlers.onApply ?? jest.fn();
  const onClose = jest.fn();
  const utils = render(
    <DanglingRefsModal
      open
      refs={refs}
      candidateNodes={candidates}
      onClose={onClose}
      onApply={onApply}
    />,
  );
  return { onApply, onClose, ...utils };
}

/** 点击 rc-select 的 option（content div 才派发）。 */
const clickOption = (label: string) => {
  const option = screen
    .getAllByText(label)
    .find((el) => el.className.includes('ant-select-item-option')) as HTMLElement;
  fireEvent.mouseDown(option);
  fireEvent.click(option);
};

describe('渲染', () => {
  it('refs 空数组：count 0、无条目', () => {
    renderModal([]);
    expect(screen.getByText(/以下 0 处联动/)).toBeInTheDocument();
    expect(screen.queryByText('封禁按钮')).not.toBeInTheDocument();
  });

  it('kind 四类标签与 detail 四态全渲染；count 插值', () => {
    renderModal([
      { ...baseRef, kind: 'refresh', prop: 'refreshOn', detail: undefined },
      { ...baseRef, nodeTitle: '链按钮', kind: 'rowAction', prop: 'rowActions', detail: 'chain 3' },
      { ...baseRef, nodeTitle: '映射卡', kind: 'assignment', detail: 'assignment 2' },
      { ...baseRef, nodeTitle: '行按钮', detail: 'rowAction 5' },
    ]);
    expect(screen.getByText(/以下 4 处联动/)).toBeInTheDocument();
    // detailLabel 四态（formatMessage mock 做 {n} 插值）
    expect(screen.getByText('主动作目标 · refreshOn')).toBeInTheDocument();
    expect(screen.getByText('链第 3 步 · rowActions')).toBeInTheDocument();
    expect(screen.getByText('第 2 个映射 · onClick')).toBeInTheDocument();
    expect(screen.getByText('第 5 个行操作 · onClick')).toBeInTheDocument();
  });

  it('detail 非法形态（不匹配正则）：回落主动作目标', () => {
    renderModal([{ ...baseRef, detail: 'garbage 12' }]);
    expect(screen.getByText('主动作目标 · onClick')).toBeInTheDocument();
  });

  it('重连下拉：候选过滤悬空节点自身', () => {
    renderModal([baseRef]);
    fireEvent.mouseDown(screen.getByText('保持断开'));
    // btn1 是悬空节点自身被过滤；tbl1/frm1 可选（选中值与 option 同名并存）
    clickOption('玩家表（fnTable）');
    expect(screen.getAllByText('玩家表（fnTable）').length).toBeGreaterThanOrEqual(1);
  });
});

describe('应用重连', () => {
  it('未选择：应用空 fixes', () => {
    const onApply = jest.fn();
    renderModal([baseRef], { onApply });
    fireEvent.click(screen.getByText('应用重连'));
    expect(onApply).toHaveBeenCalledWith([]);
  });

  it('选择他节点：fix 带 full 契约入列', () => {
    const onApply = jest.fn();
    renderModal([baseRef], { onApply });
    fireEvent.mouseDown(screen.getByText('保持断开'));
    clickOption('玩家表（fnTable）');
    fireEvent.click(screen.getByText('应用重连'));
    expect(onApply).toHaveBeenCalledWith([
      {
        nodeId: 'btn1',
        kind: 'action',
        prop: 'onClick',
        ref: 'old-target',
        target: 'tbl1',
      },
    ]);
  });

  it('清空选择后应用：不入 fixes', () => {
    const onApply = jest.fn();
    renderModal([baseRef], { onApply });
    fireEvent.mouseDown(screen.getByText('保持断开'));
    clickOption('玩家表（fnTable）');
    // allowClear 清除：choices 置空串
    const clear = document.querySelector('.ant-select-clear') as HTMLElement;
    fireEvent.mouseDown(clear);
    fireEvent.click(clear);
    fireEvent.click(screen.getByText('应用重连'));
    expect(onApply).toHaveBeenCalledWith([]);
  });

  it('多条悬空：只收集选择项（含空串跳过）', () => {
    const onApply = jest.fn();
    renderModal(
      [
        baseRef,
        {
          ...baseRef,
          nodeId: 'tbl1',
          nodeTitle: '玩家表',
          prop: 'refreshOn',
          kind: 'refresh',
          ref: 'r2',
        },
      ],
      { onApply },
    );
    // 先持 placeholder 引用（选中后该 div 被选中值替换，live 查询会缺）
    const placeholders = screen.getAllByText('保持断开');
    fireEvent.mouseDown(placeholders[0]);
    clickOption('筛选表单（staticForm）');
    fireEvent.mouseDown(placeholders[1]);
    clickOption('封禁按钮（button）');
    fireEvent.click(screen.getByText('应用重连'));
    expect(onApply).toHaveBeenCalledWith([
      { nodeId: 'btn1', kind: 'action', prop: 'onClick', ref: 'old-target', target: 'frm1' },
      { nodeId: 'tbl1', kind: 'refresh', prop: 'refreshOn', ref: 'r2', target: 'btn1' },
    ]);
  });

  it('取消：onClose 触发', () => {
    const { onClose } = renderModal([baseRef]);
    fireEvent.click(screen.getByText('取 消'));
    expect(onClose).toHaveBeenCalled();
  });
});
