/** U9 refreshOn 级联失败策略（cascadePolicy）覆盖。
 *
 * 上游区块执行失败（onExecute reject → runSection 写入 error 标记）时，
 * 下游按 cascadePolicy 处理：pause（默认，不重跑 + 提示）/ keep（不重跑、
 * 静默保留旧数据）/ clear（不重跑、清空数据）；上游恢复（error→ok）后
 * 级联自动续跑。 */
import React from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App } from 'antd';
import { CompositeRenderer } from '../CompositeRenderer';
import type { CompositeSection } from '@/types/dashboard';

jest.mock('@/services/api/functions', () => ({ invokeFunction: jest.fn() }));

type AppApi = ReturnType<typeof App.useApp>;

function renderComposite(sections: CompositeSection[], onExecute: jest.Mock) {
  const captured: { current?: AppApi } = {};
  const Probe: React.FC = () => {
    captured.current = App.useApp();
    return null;
  };
  render(
    <App>
      <Probe />
      <CompositeRenderer sections={sections} bindings={[]} onExecute={onExecute as never} />
    </App>,
  );
  return captured;
}

const upDown = (policy?: 'clear' | 'keep' | 'pause'): CompositeSection[] => [
  { key: 'upstream', bindingId: 'b-up', view: 'fields', title: { 'zh-CN': '上游' } },
  {
    key: 'downstream',
    bindingId: 'b-down',
    view: 'fields',
    refreshOn: ['upstream'],
    title: { 'zh-CN': '下游' },
    ...(policy ? { cascadePolicy: policy } : {}),
  },
];

describe('CompositeRenderer U9 级联失败策略', () => {
  it('pause（缺省）：上游失败下游不重跑，提示「联动已暂停」', async () => {
    const sections: CompositeSection[] = [
      {
        key: 'upstream',
        bindingId: 'b-up',
        view: 'fields',
        autoRun: true,
        title: { 'zh-CN': '上游' },
      },
      {
        key: 'downstream',
        bindingId: 'b-down',
        view: 'fields',
        refreshOn: ['upstream'],
        title: { 'zh-CN': '下游' },
      },
    ];
    const onExecute = jest
      .fn()
      .mockImplementation((bindingId: string) =>
        bindingId === 'b-up'
          ? Promise.reject(new Error('boom'))
          : Promise.resolve({ data: { r: 1 } }),
      );
    const app = renderComposite(sections, onExecute);
    const warningSpy = jest.spyOn(app.current!.message, 'warning');

    await waitFor(() => expect(onExecute).toHaveBeenCalledWith('b-up', expect.anything()));
    await waitFor(() => expect(warningSpy).toHaveBeenCalled());
    // 下游不被联动触发（缺省 pause）
    expect(onExecute).not.toHaveBeenCalledWith('b-down', expect.anything());
  });

  it('pause 恢复：上游失败→成功（error→ok）后级联自动续跑', async () => {
    const sections: CompositeSection[] = [
      // autoRun=false：两次手动点击构造 失败→恢复 序列
      { key: 'upstream', bindingId: 'b-up', view: 'fields', title: { 'zh-CN': '上游' } },
      {
        key: 'downstream',
        bindingId: 'b-down',
        view: 'fields',
        refreshOn: ['upstream'],
        title: { 'zh-CN': '下游' },
      },
    ];
    const onExecute = jest.fn((bindingId: string) =>
      bindingId === 'b-up'
        ? Promise.resolve({ data: { q: 'x' } })
        : Promise.resolve({ data: { r: 1 } }),
    );
    onExecute.mockImplementationOnce(() => Promise.reject(new Error('boom')));
    renderComposite(sections, onExecute);

    const buttons = await screen.findAllByRole('button', { name: '执行' });
    fireEvent.click(buttons[0]); // 上游第一次：失败
    await waitFor(() => expect(onExecute).toHaveBeenCalledWith('b-up', expect.anything()));
    expect(onExecute).not.toHaveBeenCalledWith('b-down', expect.anything());

    fireEvent.click(buttons[0]); // 上游第二次：成功 → 级联恢复
    await waitFor(() => expect(onExecute).toHaveBeenCalledWith('b-down', expect.anything()), {
      timeout: 2500,
    });
  });

  it('clear：上游失败后下游旧数据被清空', async () => {
    const onExecute = jest.fn((bindingId: string) =>
      bindingId === 'b-up'
        ? Promise.reject(new Error('boom'))
        : Promise.resolve({ data: { stale: '旧数据' } }),
    );
    renderComposite(upDown('clear'), onExecute);

    // 先跑下游，产出旧数据
    const buttons = await screen.findAllByRole('button', { name: '执行' });
    fireEvent.click(buttons[1]);
    expect(await screen.findByText('stale')).toBeInTheDocument();

    // 上游失败 → 级联 clear 清空下游
    fireEvent.click(buttons[0]);
    await waitFor(() => expect(screen.queryByText('stale')).not.toBeInTheDocument());
    // 下游未被重跑（clear = 不重跑，仅清数据）
    expect(onExecute.mock.calls.filter(([id]) => id === 'b-down')).toHaveLength(1);
  });

  it('keep：上游失败后下游旧数据保留、无提示', async () => {
    const onExecute = jest.fn((bindingId: string) =>
      bindingId === 'b-up'
        ? Promise.reject(new Error('boom'))
        : Promise.resolve({ data: { kept: '保留数据' } }),
    );
    const app = renderComposite(upDown('keep'), onExecute);
    const warningSpy = jest.spyOn(app.current!.message, 'warning');

    const buttons = await screen.findAllByRole('button', { name: '执行' });
    fireEvent.click(buttons[1]);
    expect(await screen.findByText('kept')).toBeInTheDocument();

    fireEvent.click(buttons[0]);
    await waitFor(() => expect(onExecute).toHaveBeenCalledWith('b-up', expect.anything()));
    // 数据保留 + 无 warning 提示 + 不重跑
    expect(screen.getByText('kept')).toBeInTheDocument();
    expect(warningSpy).not.toHaveBeenCalled();
    expect(onExecute.mock.calls.filter(([id]) => id === 'b-down')).toHaveLength(1);
  });
});
