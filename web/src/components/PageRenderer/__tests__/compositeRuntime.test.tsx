/** CompositeRenderer 内联区块与运行时联动覆盖。
 *
 * 覆盖路径：preview 禁执行、无表单区块默认执行按钮（success/error/null 三态）、
 * actions onSuccessRefresh 刷新、fields 区块与 click 事件、refreshOn 联动与
 * 上游字段合并、static 常量表单防抖并入 page_state（含卸载清理）、fnForm
 * 当前值防抖写入 values + 表达式消费、onFinish 提交链、动作链
 * navigate/showMessage、栅格 span 边界与卡片「执行」按钮。 */
import React from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App } from 'antd';
import { CompositeRenderer } from '../CompositeRenderer';
import type { CompositeSection } from '@/types/dashboard';

jest.mock('@/services/api/functions', () => ({ invokeFunction: jest.fn() }));

type AppApi = ReturnType<typeof App.useApp>;

function renderComposite(
  sections: CompositeSection[],
  onExecute: jest.Mock,
  opts?: { preview?: boolean; onPageStateMerge?: jest.Mock },
) {
  const captured: { current?: AppApi } = {};
  const Probe: React.FC = () => {
    captured.current = App.useApp();
    return null;
  };
  const utils = render(
    <App>
      <Probe />
      <CompositeRenderer
        sections={sections}
        bindings={[]}
        onExecute={onExecute as never}
        preview={opts?.preview ?? false}
        onPageStateMerge={opts?.onPageStateMerge as never}
      />
    </App>,
  );
  return { ...utils, app: captured };
}

const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));

describe('CompositeRenderer 内联区块与运行时联动', () => {
  it('preview 模式：渲染结构但不执行任何绑定（autoRun 与手动执行均禁用）', async () => {
    const sections: CompositeSection[] = [
      {
        key: 'secA',
        bindingId: 'b-a',
        view: 'fields',
        autoRun: true,
        title: { 'zh-CN': '预览区块' },
      },
      {
        key: 'secB',
        bindingId: 'b-b',
        view: 'form',
        title: { 'zh-CN': '预览表单' },
      },
    ];
    const onExecute = jest.fn();
    renderComposite(sections, onExecute, { preview: true });
    expect(await screen.findByText('预览区块')).toBeInTheDocument();
    // 无表单区块降级为执行按钮，点击在 preview 下不触发执行
    fireEvent.click(screen.getByRole('button', { name: '预览表单' }));
    await sleep(100);
    expect(onExecute).not.toHaveBeenCalled();
  });

  it('无表单区块默认执行按钮：成功触发 success 事件；error 结果不触发', async () => {
    const sections: CompositeSection[] = [
      {
        key: 'okSec',
        bindingId: 'b-ok',
        view: 'form',
        title: { 'zh-CN': '成功动作' },
        events: [
          {
            event: 'success',
            action: { kind: 'showMessage', target: '', params: { message: 'done' } },
          },
        ],
      },
      {
        key: 'errSec',
        bindingId: 'b-err',
        view: 'form',
        title: { 'zh-CN': '失败动作' },
        events: [
          {
            event: 'success',
            action: { kind: 'showMessage', target: '', params: { message: 'should-not-fire' } },
          },
        ],
      },
    ];
    const onExecute = jest
      .fn()
      .mockImplementation((bindingId: string) =>
        bindingId === 'b-ok'
          ? Promise.resolve({ data: {} })
          : Promise.resolve({ error: 'boom', data: null }),
      );
    const { app } = renderComposite(sections, onExecute);
    const infoSpy = jest.spyOn(app.current!.message, 'info');

    fireEvent.click(screen.getByRole('button', { name: '成功动作' }));
    await waitFor(() => expect(infoSpy).toHaveBeenCalledWith('done'));

    fireEvent.click(screen.getByRole('button', { name: '失败动作' }));
    await waitFor(() => expect(onExecute).toHaveBeenCalledWith('b-err', expect.anything()));
    await sleep(100);
    expect(infoSpy).not.toHaveBeenCalledWith('should-not-fire');
  });

  it('actions 区块执行成功后按 onSuccessRefresh 刷新目标区块', async () => {
    const sections: CompositeSection[] = [
      {
        key: 'actSec',
        bindingId: 'b-act',
        view: 'actions',
        title: { 'zh-CN': '发邮件动作' },
        onSuccessRefresh: ['targetTable'],
      },
      {
        key: 'targetTable',
        bindingId: 'b-table',
        view: 'table',
        autoRun: true,
        title: { 'zh-CN': '玩家列表' },
        table: { columns: [{ key: 'uid', title: { 'zh-CN': 'UID' }, dataType: 'string' }] },
      },
    ];
    const onExecute = jest
      .fn()
      .mockImplementation((bindingId: string) =>
        bindingId === 'b-table'
          ? Promise.resolve({ data: { items: [{ uid: 'u1' }], total: 1 } })
          : Promise.resolve({ data: { ok: 1 } }),
      );
    renderComposite(sections, onExecute);
    // autoRun 首次执行
    await waitFor(() => expect(onExecute).toHaveBeenCalledWith('b-table', expect.anything()));
    expect(onExecute).toHaveBeenCalledTimes(1);
    fireEvent.click(screen.getByRole('button', { name: '发邮件动作' }));
    await waitFor(() => expect(onExecute).toHaveBeenCalledWith('b-act', expect.anything()));
    // 成功后目标表格被刷新
    await waitFor(() => expect(onExecute).toHaveBeenCalledTimes(3));
  });

  it('fields 区块渲染数据条目；click 事件触发 navigate 动作链', async () => {
    const openSpy = jest.fn();
    window.open = openSpy as unknown as typeof window.open;
    const sections: CompositeSection[] = [
      {
        key: 'detail',
        bindingId: 'b-detail',
        view: 'fields',
        autoRun: true,
        title: { 'zh-CN': '玩家详情' },
        events: [
          {
            event: 'click',
            action: { kind: 'navigate', target: '', params: { url: 'http://game.example/player' } },
          },
        ],
      },
      {
        key: 'emptyDetail',
        bindingId: 'b-empty',
        view: 'fields',
        title: { 'zh-CN': '空详情' },
      },
    ];
    const onExecute = jest
      .fn()
      .mockImplementation((bindingId: string) =>
        bindingId === 'b-detail'
          ? Promise.resolve({ data: { name: 'bob', vip: 'yes' } })
          : Promise.resolve(null),
      );
    renderComposite(sections, onExecute);
    await screen.findByText('bob');
    expect(screen.getByText('yes')).toBeInTheDocument();
    // 点击 fields 区块 → navigate
    fireEvent.click(screen.getByText('bob'));
    expect(openSpy).toHaveBeenCalledWith('http://game.example/player', '_blank');
  });

  it('refreshOn 联动：上游产出后自动重跑下游，上游 data 同名字段并入下游输入', async () => {
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
          ? Promise.resolve({ data: { q: 'x' } })
          : Promise.resolve({ data: { r: 1 } }),
      );
    renderComposite(sections, onExecute);
    // 下游被联动触发，且后续轮次输入并入上游同名字段 q
    await waitFor(
      () =>
        expect(onExecute).toHaveBeenCalledWith(
          'b-down',
          expect.objectContaining({ form: expect.objectContaining({ q: 'x' }) }),
        ),
      { timeout: 2500 },
    );
  });

  it('static 常量表单：输入防抖并入 page_state（扁平+values 双形态）；卸载后清理定时器', async () => {
    const sections: CompositeSection[] = [
      {
        key: 'staticForm',
        bindingId: 'b-static',
        view: 'form',
        static: true,
        title: { 'zh-CN': '筛选条件' },
        form: {
          jsonSchema: {
            type: 'object',
            properties: { name: { type: 'string', title: '玩家名' } },
          },
        },
      },
    ];
    const onExecute = jest.fn();
    const onPageStateMerge = jest.fn();
    const utils = renderComposite(sections, onExecute, { onPageStateMerge });
    const nameInput = (await screen.findByLabelText('玩家名')) as HTMLInputElement;
    fireEvent.change(nameInput, { target: { value: 'alice' } });
    // 400ms 防抖后并入：扁平值 + values 包装双形态
    await waitFor(
      () =>
        expect(onPageStateMerge).toHaveBeenCalledWith('staticForm', {
          name: 'alice',
          values: { name: 'alice' },
        }),
      { timeout: 1500 },
    );

    // 卸载清理：再次输入后立即卸载，防抖不再触发合并
    onPageStateMerge.mockClear();
    const again = await screen.findByLabelText('玩家名');
    fireEvent.change(again, { target: { value: 'bob' } });
    utils.unmount();
    await sleep(600);
    expect(onPageStateMerge).not.toHaveBeenCalled();
  });

  it('fnForm：当前值 300ms 防抖写入 values 运行时状态，动作链表达式可消费；onFinish 提交并入 page_state', async () => {
    const sections: CompositeSection[] = [
      {
        key: 'queryForm',
        bindingId: 'b-form',
        view: 'form',
        title: { 'zh-CN': '查询表单' },
        form: {
          jsonSchema: {
            type: 'object',
            properties: { to: { type: 'string', title: '收件人' } },
          },
        },
        events: [
          {
            event: 'success',
            action: { kind: 'showMessage', target: '', params: { message: 'submitted' } },
          },
        ],
      },
      {
        key: 'opBar',
        bindingId: 'b-tb',
        view: 'toolbar',
        title: { 'zh-CN': '操作区' },
        toolbar: {
          actions: [
            {
              label: { 'zh-CN': '取表单值' },
              chain: [
                {
                  kind: 'runBinding',
                  target: 'consumer',
                  params: { v: '{{queryForm.values.to}}' },
                },
              ],
            },
          ],
        },
      },
      {
        key: 'consumer',
        bindingId: 'b-consumer',
        view: 'fields',
        title: { 'zh-CN': '消费方' },
      },
    ];
    const onExecute = jest.fn().mockResolvedValue({ data: {} });
    const onPageStateMerge = jest.fn();
    const { app } = renderComposite(sections, onExecute, { onPageStateMerge });
    const infoSpy = jest.spyOn(app.current!.message, 'info');

    const toInput = (await screen.findByLabelText('收件人')) as HTMLInputElement;
    fireEvent.change(toInput, { target: { value: 'bob@test' } });
    // 等 300ms 防抖写入 values
    await sleep(450);
    fireEvent.click(screen.getByRole('button', { name: '取表单值' }));
    await waitFor(() =>
      expect(onExecute).toHaveBeenCalledWith(
        'b-consumer',
        expect.objectContaining({ form: { v: 'bob@test' } }),
      ),
    );

    // onFinish：提交值并入 page_state（扁平 + values）并触发 success 事件
    fireEvent.click(screen.getByRole('button', { name: /提\s*交/ }));
    await waitFor(() =>
      expect(onPageStateMerge).toHaveBeenCalledWith('queryForm', {
        to: 'bob@test',
        values: { to: 'bob@test' },
      }),
    );
    await waitFor(() => expect(onExecute).toHaveBeenCalledWith('b-form', expect.anything()));
    await waitFor(() => expect(infoSpy).toHaveBeenCalledWith('submitted'));
  });

  it('动作链 showMessage / navigate：提示与开窗，navigate 缺 url 不开窗', async () => {
    const sections: CompositeSection[] = [
      {
        key: 'opBar',
        bindingId: 'b-tb',
        view: 'toolbar',
        title: { 'zh-CN': '操作区' },
        toolbar: {
          actions: [
            {
              label: { 'zh-CN': '提示并跳转' },
              chain: [
                { kind: 'showMessage', target: '', params: { message: 'hello-chain' } },
                { kind: 'navigate', target: '', params: { url: 'http://docs.example' } },
                { kind: 'navigate', target: '' },
              ],
            },
          ],
        },
      },
    ];
    const onExecute = jest.fn();
    const { app } = renderComposite(sections, onExecute);
    const infoSpy = jest.spyOn(app.current!.message, 'info');
    const openSpy = jest.fn();
    window.open = openSpy as unknown as typeof window.open;

    fireEvent.click(screen.getByRole('button', { name: '提示并跳转' }));
    await waitFor(() => expect(infoSpy).toHaveBeenCalledWith('hello-chain'));
    // navigate 打开一次；缺 url 的步骤不重复开窗
    expect(openSpy).toHaveBeenCalledTimes(1);
    expect(openSpy).toHaveBeenCalledWith('http://docs.example', '_blank');
  });

  it('栅格 span 边界：合法 span 生效，非法值回落 24；非 autoRun 区块卡片头渲染「执行」按钮', async () => {
    const sections: CompositeSection[] = [
      {
        key: 'narrow',
        bindingId: 'b-narrow',
        view: 'fields',
        span: 8,
        title: { 'zh-CN': '窄区块' },
      },
      {
        key: 'oversize',
        bindingId: 'b-oversize',
        view: 'fields',
        span: 30,
        title: { 'zh-CN': '超宽区块' },
      },
      {
        key: 'zero',
        bindingId: 'b-zero',
        view: 'fields',
        span: 0,
        title: { 'zh-CN': '零宽区块' },
      },
    ];
    const onExecute = jest.fn().mockResolvedValue({ data: {} });
    const { container } = renderComposite(sections, onExecute);
    await screen.findByText('窄区块');
    expect(container.querySelector('.ant-col-8')).toBeInTheDocument();
    // 30/0 均回落整行
    expect(container.querySelectorAll('.ant-col-24').length).toBe(2);
    // 非 autoRun 且非 actions/toolbar：卡片 extra 渲染执行按钮
    const runButtons = screen.getAllByRole('button', { name: /执\s*行/ });
    expect(runButtons.length).toBe(3);
    fireEvent.click(runButtons[0]);
    await waitFor(() => expect(onExecute).toHaveBeenCalledWith('b-narrow', expect.anything()));
  });

  it('表格区块带工具栏且非 autoRun：卡片头同时渲染工具栏按钮与「执行」按钮', async () => {
    const sections: CompositeSection[] = [
      {
        key: 'mainTable',
        bindingId: 'b-table',
        view: 'table',
        title: { 'zh-CN': '玩家列表' },
        table: {
          columns: [{ key: 'uid', title: { 'zh-CN': 'UID' }, dataType: 'string' }],
        },
        toolbar: {
          actions: [{ label: { 'zh-CN': '新建玩家' }, targetSection: 'noneModal' }],
        },
      },
    ];
    const onExecute = jest.fn().mockResolvedValue({ data: { items: [{ uid: 'u1' }], total: 1 } });
    renderComposite(sections, onExecute);
    expect(screen.getByRole('button', { name: '新建玩家' })).toBeInTheDocument();
    const runBtn = screen.getByRole('button', { name: /执\s*行/ });
    fireEvent.click(runBtn);
    await waitFor(() => expect(onExecute).toHaveBeenCalledWith('b-table', expect.anything()));
  });

  it('表格选中行写入运行时状态并以 merge 模式并入 page_state', async () => {
    const sections: CompositeSection[] = [
      {
        key: 'mainTable',
        bindingId: 'b-table',
        view: 'table',
        autoRun: true,
        title: { 'zh-CN': '玩家列表' },
        table: {
          columns: [
            { key: 'uid', title: { 'zh-CN': 'UID' }, dataType: 'string' },
            { key: 'nickname', title: { 'zh-CN': '昵称' }, dataType: 'string' },
          ],
        },
      },
    ];
    const onExecute = jest
      .fn()
      .mockResolvedValue({ data: { items: [{ uid: 'u1', nickname: 'bob' }], total: 1 } });
    const onPageStateMerge = jest.fn();
    renderComposite(sections, onExecute, { onPageStateMerge });
    await screen.findByText('bob');

    const radios = document.querySelectorAll('input[type="radio"]');
    expect(radios.length).toBeGreaterThan(0);
    fireEvent.click(radios[0]);
    await waitFor(() =>
      expect(onPageStateMerge).toHaveBeenCalledWith(
        'mainTable',
        {
          selectedRow: { uid: 'u1', nickname: 'bob' },
          selectedRows: [{ uid: 'u1', nickname: 'bob' }],
        },
        'merge',
      ),
    );
  });
});
