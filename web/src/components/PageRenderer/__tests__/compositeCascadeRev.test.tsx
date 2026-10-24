/** B2 refreshOn 级联修订号驱动覆盖。
 *
 * 旧级联信号只看「区块 key 集合 + 各区块 o/e 失败标记」：上游连续两次成功但
 * 数据不同时信号不变，下游永不刷新。改为「修订号:成败标记」信号——每次产出新
 * 结果（成功或失败）修订号 +1，同态新数据同样驱动下游。同批验收：
 * 请求取消/旧响应拒收（并发同区块重跑仅最新响应落账）、依赖循环拒绝联动。
 * 既有 pause/keep/clear 失败策略与失败→恢复续跑见 compositeCascade.test.tsx。
 */
import React from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App } from 'antd';
import { CompositeRenderer } from '../CompositeRenderer';
import type { CompositeSection } from '@/types/dashboard';

jest.mock('@/services/api/functions', () => ({ invokeFunction: jest.fn() }));

const upDown = (upstreamAutoRun = false): CompositeSection[] => [
  {
    key: 'upstream',
    bindingId: 'b-up',
    view: 'fields',
    title: { 'zh-CN': '上游' },
    ...(upstreamAutoRun ? { autoRun: true } : {}),
  },
  {
    key: 'downstream',
    bindingId: 'b-down',
    view: 'fields',
    refreshOn: ['upstream'],
    title: { 'zh-CN': '下游' },
  },
];

describe('CompositeRenderer B2 级联修订号', () => {
  it('连续两次成功但结果不同：下游用新上游数据重跑', async () => {
    const onExecute = jest.fn((bindingId: string) =>
      bindingId === 'b-up'
        ? Promise.resolve({ data: { q: '第一次' } })
        : Promise.resolve({ data: { r: 1 } }),
    );
    render(
      <App>
        <CompositeRenderer sections={upDown()} bindings={[]} onExecute={onExecute as never} />
      </App>,
    );

    const buttons = await screen.findAllByRole('button', { name: '执行' });
    fireEvent.click(buttons[0]); // 上游第一次成功 → 级联
    await waitFor(() =>
      expect(onExecute).toHaveBeenCalledWith(
        'b-down',
        expect.objectContaining({ form: { q: '第一次' } }),
      ),
    );
    expect(onExecute.mock.calls.filter(([id]) => id === 'b-down')).toHaveLength(1);

    // 上游第二次成功（仍为成功态、数据不同）——旧信号不变、下游不刷新；
    // 修订号驱动下下游必须带新入参重跑
    onExecute.mockImplementationOnce((bindingId: string) =>
      bindingId === 'b-up'
        ? Promise.resolve({ data: { q: '第二次' } })
        : Promise.resolve({ data: { r: 1 } }),
    );
    fireEvent.click(buttons[0]);
    await waitFor(() =>
      expect(onExecute).toHaveBeenCalledWith(
        'b-down',
        expect.objectContaining({ form: { q: '第二次' } }),
      ),
    );
    expect(onExecute.mock.calls.filter(([id]) => id === 'b-down')).toHaveLength(2);
  });

  it('旧响应拒收：并发同区块重跑仅最新响应落账', async () => {
    const resolves: Array<(v: unknown) => void> = [];
    const onExecute = jest.fn((bindingId: string) =>
      bindingId === 'b-up'
        ? new Promise((resolve) => resolves.push(resolve))
        : Promise.resolve({ data: {} }),
    );
    render(
      <App>
        <CompositeRenderer sections={upDown()} bindings={[]} onExecute={onExecute as never} />
      </App>,
    );

    const buttons = await screen.findAllByRole('button', { name: '执行' });
    fireEvent.click(buttons[0]); // 第一次请求（悬挂）
    fireEvent.click(buttons[0]); // 立即重跑（悬挂）
    await waitFor(() =>
      expect(onExecute.mock.calls.filter(([id]) => id === 'b-up')).toHaveLength(2),
    );

    resolves[0]({ data: { latest: '旧响应' } }); // 最旧响应后到：必须被拒收
    resolves[1]({ data: { latest: '最新响应' } }); // 最新响应落账
    await screen.findByText('最新响应');
    expect(screen.queryByText('旧响应')).not.toBeInTheDocument();
    expect(onExecute.mock.calls.filter(([id]) => id === 'b-up')).toHaveLength(2);
    // 仅按最新修订号联动一次
    expect(onExecute.mock.calls.filter(([id]) => id === 'b-down')).toHaveLength(1);
  });

  it('依赖循环拒绝：A→B→A 不形成无限级联', async () => {
    const sections: CompositeSection[] = [
      { key: 'a', bindingId: 'b-a', view: 'fields', title: { 'zh-CN': 'A' } },
      {
        key: 'b',
        bindingId: 'b-b',
        view: 'fields',
        refreshOn: ['a'],
        title: { 'zh-CN': 'B' },
      },
      {
        key: 'a2',
        bindingId: 'b-a2',
        view: 'fields',
        refreshOn: ['b'],
        title: { 'zh-CN': 'A2' },
      },
    ];
    // a2 依赖 b、b 依赖 a、a 依赖 a2 组成闭环
    sections[0].refreshOn = ['a2'];

    const onExecute = jest.fn(() => Promise.resolve({ data: {} }));
    render(
      <App>
        <CompositeRenderer sections={sections} bindings={[]} onExecute={onExecute as never} />
      </App>,
    );

    const buttons = await screen.findAllByRole('button', { name: '执行' });
    fireEvent.click(buttons[0]); // 手动触发 a
    await waitFor(() =>
      expect(onExecute).toHaveBeenCalledWith('b-a', expect.objectContaining({ form: {} })),
    );
    // 整个环（a→a2→b→a）被判定为循环：除手动触发外环上区块一律拒绝联动
    await new Promise((r) => setTimeout(r, 0));
    expect(onExecute.mock.calls.filter(([id]) => id === 'b-a')).toHaveLength(1);
    expect(onExecute.mock.calls.filter(([id]) => id === 'b-b')).toHaveLength(0);
    expect(onExecute.mock.calls.filter(([id]) => id === 'b-a2')).toHaveLength(0);
  });

  it('动作链失败中止：执行类步骤失败即终止后续步骤', async () => {
    const sections: CompositeSection[] = [
      {
        key: 'list',
        bindingId: 'b-list',
        view: 'table',
        title: { 'zh-CN': '列表' },
        table: { columns: [{ key: 'uid', title: { 'zh-CN': 'UID' } }] },
        toolbar: {
          actions: [
            {
              label: { 'zh-CN': '链式操作' },
              chain: [
                { kind: 'runBinding', target: 'willFail' },
                { kind: 'runBinding', target: 'neverRuns' },
              ],
            },
          ],
        },
      },
      { key: 'willFail', bindingId: 'b-fail', view: 'fields', title: { 'zh-CN': '必失败' } },
      { key: 'neverRuns', bindingId: 'b-after', view: 'fields', title: { 'zh-CN': '后续' } },
    ];
    const onExecute = jest.fn((bindingId: string) =>
      bindingId === 'b-fail' ? Promise.reject(new Error('boom')) : Promise.resolve({ data: {} }),
    );
    render(
      <App>
        <CompositeRenderer sections={sections} bindings={[]} onExecute={onExecute as never} />
      </App>,
    );

    fireEvent.click(await screen.findByRole('button', { name: '链式操作' }));
    await waitFor(() =>
      expect(onExecute).toHaveBeenCalledWith('b-fail', expect.objectContaining({ form: {} })),
    );
    // 失败步骤之后的链步骤不再执行（不吞错继续跑）
    await new Promise((r) => setTimeout(r, 0));
    expect(onExecute.mock.calls.filter(([id]) => id === 'b-after')).toHaveLength(0);
  });
});
