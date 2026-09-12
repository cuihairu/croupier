/** #94 container 卡片分组渲染覆盖。
 *
 * 覆盖路径：display='card' 区块按 group 聚合进同一 Card（整行）+ 卡片
 * 标题（cardTitle 缺省回退组名）、卡内区块垂直堆叠、卡内 autoRun 照常
 * 执行（不因分组被排除）、卡内 visibleWhen 条件过滤（执行不变）、
 * inline/card 两桶共存互不干扰。 */
import React from 'react';
import { render, screen, waitFor } from '@testing-library/react';
import { App } from 'antd';
import { CompositeRenderer } from '../index';
import type { CompositeSection } from '@/types/dashboard';

jest.mock('@/services/api/functions', () => ({ invokeFunction: jest.fn() }));

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

describe('CompositeRenderer #94：卡片分组聚合', () => {
  it('同 group 聚合进一个 Card；cardTitle 作标题、缺省回退组名；inline 不受影响', async () => {
    const sections: CompositeSection[] = [
      {
        key: 'inlineSummary',
        bindingId: 'b-inline',
        view: 'fields',
        title: { 'zh-CN': '页首概览' },
      },
      {
        key: 'vipRank',
        bindingId: 'b-rank',
        view: 'table',
        title: { 'zh-CN': 'VIP 榜' },
        autoRun: true,
        display: 'card',
        group: 'vip-zone',
        cardTitle: { 'zh-CN': 'VIP 专区' },
        table: { columns: [{ key: 'uid', title: { 'zh-CN': 'UID' } }] },
      },
      {
        key: 'vipDetail',
        bindingId: 'b-detail',
        view: 'fields',
        title: { 'zh-CN': 'VIP 详情' },
        display: 'card',
        group: 'vip-zone',
        cardTitle: { 'zh-CN': 'VIP 专区' },
      },
      {
        key: 'opsFields',
        bindingId: 'b-ops',
        view: 'fields',
        title: { 'zh-CN': '运维卡' },
        display: 'card',
        group: 'ops-zone',
      },
    ];
    const onExecute = jest.fn().mockResolvedValue({
      data: { items: [{ uid: 'u1' }] },
    });
    renderComposite(sections, onExecute);
    // 卡片标题：声明 cardTitle 用声明值；缺省回退组名 ops-zone
    expect(screen.getByText('VIP 专区')).toBeInTheDocument();
    expect(screen.getByText('ops-zone')).toBeInTheDocument();
    // inline 区块仍在卡片之外渲染
    expect(screen.getByText('页首概览')).toBeInTheDocument();
    // 卡内表格渲染真实数据（autoRun 结果）
    await waitFor(() => expect(screen.getByText('u1')).toBeInTheDocument());
    // 同卡内字段卡也在
    expect(screen.getByText('VIP 详情')).toBeInTheDocument();
    expect(onExecute).toHaveBeenCalledWith('b-rank', expect.anything());
  });

  it('卡内 autoRun 照常执行（分组不排除执行）；不同 group 各自成卡', async () => {
    const sections: CompositeSection[] = [
      {
        key: 'statA',
        bindingId: 'b-a',
        view: 'fields',
        autoRun: true,
        title: { 'zh-CN': '统计A' },
        display: 'card',
        group: 'g-a',
      },
      {
        key: 'statB',
        bindingId: 'b-b',
        view: 'fields',
        autoRun: true,
        title: { 'zh-CN': '统计B' },
        display: 'card',
        group: 'g-b',
      },
    ];
    const onExecute = jest.fn().mockImplementation(async (bindingId: string) => ({
      data: { who: bindingId },
    }));
    renderComposite(sections, onExecute);
    await waitFor(() => expect(onExecute).toHaveBeenCalledTimes(2));
    expect(onExecute).toHaveBeenCalledWith('b-a', expect.anything());
    expect(onExecute).toHaveBeenCalledWith('b-b', expect.anything());
    expect(screen.getByText('g-a')).toBeInTheDocument();
    expect(screen.getByText('g-b')).toBeInTheDocument();
  });

  it('卡内 visibleWhen 过滤：条件不满足的区块不渲染（执行照常），满足的正常出现', async () => {
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
        key: 'vipOnly',
        bindingId: 'b-vip',
        view: 'fields',
        autoRun: true,
        title: { 'zh-CN': 'VIP 专属区块' },
        display: 'card',
        group: 'zone',
        visibleWhen: {
          kind: 'equals',
          key: 'modeFilter',
          path: '/values/mode',
          value: 'advanced',
        },
      },
      {
        key: 'commonBlock',
        bindingId: 'b-common',
        view: 'fields',
        title: { 'zh-CN': '常规区块' },
        display: 'card',
        group: 'zone',
      },
    ];
    const onExecute = jest.fn().mockResolvedValue({ data: {} });
    renderComposite(sections, onExecute);
    await waitFor(() => expect(onExecute).toHaveBeenCalledWith('b-vip', expect.anything()));
    // 条件不满足：VIP 专属区块不渲染，但同卡常规区块在、卡片本身在（组名兜底标题）
    expect(screen.queryByText('VIP 专属区块')).not.toBeInTheDocument();
    expect(screen.getByText('常规区块')).toBeInTheDocument();
    expect(screen.getByText('zone')).toBeInTheDocument();
  });
});
