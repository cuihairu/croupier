/** U10 区块级条件显示渲染覆盖。
 *
 * 覆盖路径：visibleWhen 不满足时区块不渲染但 autoRun 执行照常、
 * staticForm 值（values 寻址段）驱动条件翻转区块出现/消失、
 * tab 区块条件过滤（页签页内）。 */
import React from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App } from 'antd';
import { CompositeRenderer } from '../CompositeRenderer';
import type { CompositeSection } from '@/types/dashboard';

jest.mock('@/services/api/functions', () => ({ invokeFunction: jest.fn() }));

const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));

function renderComposite(sections: CompositeSection[], onExecute: jest.Mock) {
  return render(
    <App>
      <CompositeRenderer
        sections={sections}
        bindings={[]}
        onExecute={onExecute as never}
        preview={false}
      />
    </App>,
  );
}

describe('CompositeRenderer U10：区块级条件显示', () => {
  it('条件不满足：区块不渲染，但 autoRun 执行照常（显隐不剔除执行）', async () => {
    const sections: CompositeSection[] = [
      {
        key: 'vipTable',
        bindingId: 'b-vip',
        view: 'fields',
        autoRun: true,
        title: { 'zh-CN': 'VIP 视图' },
        visibleWhen: {
          kind: 'equals',
          key: 'modeFilter',
          path: '/values/mode',
          value: 'advanced',
        },
      },
    ];
    const onExecute = jest.fn().mockResolvedValue({ data: { x: 1 } });
    renderComposite(sections, onExecute);
    await waitFor(() => expect(onExecute).toHaveBeenCalledWith('b-vip', expect.anything()));
    await sleep(100);
    // 来源区块无值（undefined ≠ 'advanced'）→ 不渲染
    expect(screen.queryByText('VIP 视图')).not.toBeInTheDocument();
  });

  it('staticForm 值变化驱动：条件满足后区块出现，改回后消失', async () => {
    const sections: CompositeSection[] = [
      {
        key: 'modeFilter',
        bindingId: 'b-filter',
        view: 'form',
        static: true,
        title: { 'zh-CN': '模式筛选' },
        form: {
          jsonSchema: {
            type: 'object',
            properties: { mode: { type: 'string', title: '模式' } },
          },
        },
      },
      {
        key: 'vipTable',
        bindingId: 'b-vip',
        view: 'fields',
        autoRun: true,
        title: { 'zh-CN': 'VIP 视图' },
        visibleWhen: {
          kind: 'equals',
          key: 'modeFilter',
          path: '/values/mode',
          value: 'advanced',
        },
      },
    ];
    const onExecute = jest.fn().mockResolvedValue({ data: {} });
    renderComposite(sections, onExecute);
    expect(screen.queryByText('VIP 视图')).not.toBeInTheDocument();

    const modeInput = (await screen.findByLabelText('模式')) as HTMLInputElement;
    fireEvent.change(modeInput, { target: { value: 'advanced' } });
    // static 值 400ms 防抖写入 results → 条件翻转
    await waitFor(() => expect(screen.getByText('VIP 视图')).toBeInTheDocument(), {
      timeout: 1500,
    });

    fireEvent.change(modeInput, { target: { value: 'basic' } });
    await waitFor(() => expect(screen.queryByText('VIP 视图')).not.toBeInTheDocument(), {
      timeout: 1500,
    });
  });

  it('函数输出驱动（data 寻址段）：exists 条件命中后渲染', async () => {
    const sections: CompositeSection[] = [
      {
        key: 'playerTable',
        bindingId: 'b-player',
        view: 'fields',
        autoRun: true,
        title: { 'zh-CN': '玩家概览' },
      },
      {
        key: 'detailPanel',
        bindingId: 'b-detail',
        view: 'fields',
        title: { 'zh-CN': '详情面板' },
        visibleWhen: { kind: 'exists', key: 'playerTable', path: '/data/selected' },
      },
    ];
    const onExecute = jest.fn().mockResolvedValue({ data: { selected: { uid: 'u1' } } });
    renderComposite(sections, onExecute);
    // 上游 autoRun 结果写入 results → 下游条件求值命中
    await waitFor(() => expect(screen.getByText('详情面板')).toBeInTheDocument(), {
      timeout: 2000,
    });
  });

  it('tab 区块条件不满足：页签页内不渲染（Tabs 本身保留）', async () => {
    const sections: CompositeSection[] = [
      {
        key: 'hiddenTab',
        bindingId: 'b-t1',
        view: 'fields',
        display: 'tab',
        group: 'main',
        tab: { 'zh-CN': '隐藏页' },
        title: { 'zh-CN': '隐藏区块' },
        visibleWhen: {
          kind: 'equals',
          key: 'modeFilter',
          path: '/values/mode',
          value: 'never',
        },
      },
      {
        key: 'shownTab',
        bindingId: 'b-t2',
        view: 'fields',
        display: 'tab',
        group: 'main',
        tab: { 'zh-CN': '展示页' },
        title: { 'zh-CN': '展示区块' },
      },
    ];
    const onExecute = jest.fn().mockResolvedValue({ data: {} });
    renderComposite(sections, onExecute);
    expect(await screen.findByText('展示页')).toBeInTheDocument();
    expect(screen.getByText('隐藏页')).toBeInTheDocument();
    // 条件不满足的区块不在文档中（其页签容器仍在）
    expect(screen.queryByText('隐藏区块')).not.toBeInTheDocument();
    expect(screen.queryByText('展示区块')).not.toBeInTheDocument(); // 非激活页不渲染属 Tabs 惰性
  });
});
