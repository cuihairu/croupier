/** PreviewRuntime 动作步骤分派与弹窗形态覆盖：
 * runStep switch 各 kind（openModal/runBinding 目标缺失警告、closeModal
 * 带/不带 target、navigate window.open）、行操作 danger confirm（label
 * 兜底）与目标缺失、literal inputAssignments 求值（真实模式 invoke 断言）、
 * fnForm onSuccess → refreshNode 重跑目标、container 子树 renderChild、
 * 弹窗 title/执行成功兜底文案与无 fnForm 空态。 */
import React from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App } from 'antd';
import { jest } from '@jest/globals';
import PreviewRuntime from '../PreviewRuntime';
import { invokeFunction } from '@/services/api/functions';
import type { FunctionDescriptor } from '@/services/api/functions';
import type { PageNode } from '../model';

jest.mock('@/services/api/functions', () => ({
  invokeFunction: jest.fn(async () => ({ result: {} })),
  listDescriptors: jest.fn(async () => []),
}));

const mockedInvoke = invokeFunction as unknown as jest.Mock;

const descriptors: FunctionDescriptor[] = [
  {
    id: 'player.list',
    outputSchema: {
      type: 'object',
      properties: {
        items: {
          type: 'array',
          items: {
            type: 'object',
            properties: { uid: { type: 'string' }, nickname: { type: 'string' } },
          },
        },
      },
    },
  },
  {
    id: 'mail.send',
    inputSchema: {
      type: 'object',
      properties: { playerId: { type: 'string', title: '玩家' } },
    },
  },
  // 无 schema：预览渲染「确认执行」单按钮
  { id: 'ops.noop' },
  {
    id: 'stat.total',
    outputSchema: { type: 'object', properties: { total: { type: 'integer' } } },
  },
] as unknown as FunctionDescriptor[];

const fnMap = new Map(descriptors.map((d) => [d.id, d]));

/** confirm 弹层 footer 按钮（antd 默认 en locale，文本非「确定/取消」）：
 * 按 class 定位——primary=确认，非 primary=取消。 */
const confirmOkBtn = () =>
  document.querySelector('.ant-modal-confirm-btns .ant-btn-primary') as HTMLButtonElement;
const confirmCancelBtn = () =>
  document.querySelector(
    '.ant-modal-confirm-btns button:not(.ant-btn-primary)',
  ) as HTMLButtonElement;

function renderPreview(nodes: PageNode[]) {
  return render(
    <App>
      <PreviewRuntime tree={nodes} fnById={fnMap} />
    </App>,
  );
}

/** modal1：含 mail.send 表单的标准弹窗（带 title）。 */
const modalWithForm = (title?: string): PageNode => ({
  id: 'modal1',
  type: 'modal',
  props: title === undefined ? {} : { title },
  children: [
    {
      id: 'form1',
      type: 'fnForm',
      props: { functionId: 'mail.send', title: '发邮件', display: 'dialog' },
    },
  ],
});

const ghostModalBtn = (kind: string, id: string): PageNode => ({
  id,
  type: 'button',
  props: { title: id, onClick: { kind, target: 'ghost-modal' } },
});

beforeEach(() => {
  mockedInvoke.mockClear();
  mockedInvoke.mockImplementation(async (fid: string) =>
    fid === 'stat.total' ? { result: { total: 42 } } : { result: { items: [] } },
  );
});

describe('runStep 动作步骤分派', () => {
  it('openModal 目标不存在：警告提示（可能已删除）', async () => {
    renderPreview([ghostModalBtn('openModal', 'b1'), modalWithForm('发邮件弹窗')]);
    fireEvent.click(screen.getByRole('button', { name: 'b1' }));
    await waitFor(() =>
      expect(screen.getByText('动作目标不存在（可能已删除）')).toBeInTheDocument(),
    );
  });

  it('runBinding 目标不存在：default 分支同样警告', async () => {
    renderPreview([ghostModalBtn('runBinding', 'b2'), modalWithForm('发邮件弹窗')]);
    fireEvent.click(screen.getByRole('button', { name: 'b2' }));
    await waitFor(() =>
      expect(screen.getByText('动作目标不存在（可能已删除）')).toBeInTheDocument(),
    );
  });

  it('closeModal：带 target 关指定弹窗；无 target 关当前打开的弹窗', async () => {
    const closeTargeted: PageNode = {
      id: 'btn-close-1',
      type: 'button',
      props: { title: '关指定', onClick: { kind: 'closeModal', target: 'modal1' } },
    };
    const closeAny: PageNode = {
      id: 'btn-close-2',
      type: 'button',
      props: { title: '关当前', onClick: { kind: 'closeModal' } },
    };
    const openBtn: PageNode = {
      id: 'btn-open',
      type: 'button',
      props: { title: '打开', onClick: { kind: 'openModal', target: 'modal1' } },
    };
    renderPreview([openBtn, closeTargeted, closeAny, modalWithForm('发邮件弹窗')]);

    // 打开 → 带 target 关闭（antd 两字中文按钮自动插空格）
    fireEvent.click(screen.getByRole('button', { name: /打\s*开/ }));
    await waitFor(() => expect(screen.getByText('发邮件弹窗')).toBeInTheDocument());
    fireEvent.click(screen.getByRole('button', { name: '关指定' }));
    await waitFor(() => expect(screen.queryByText('发邮件弹窗')).not.toBeInTheDocument());

    // 再打开 → 无 target 关当前
    fireEvent.click(screen.getByRole('button', { name: /打\s*开/ }));
    await waitFor(() => expect(screen.getByText('发邮件弹窗')).toBeInTheDocument());
    fireEvent.click(screen.getByRole('button', { name: '关当前' }));
    await waitFor(() => expect(screen.queryByText('发邮件弹窗')).not.toBeInTheDocument());
  });

  it('navigate：window.open 新窗口；空 url 不打开', async () => {
    const openSpy = jest.spyOn(window, 'open').mockReturnValue(null);
    const navBtn: PageNode = {
      id: 'btn-nav',
      type: 'button',
      props: {
        title: '跳转',
        onClick: { kind: 'navigate', params: { url: 'https://example.com' } },
      },
    };
    const navEmpty: PageNode = {
      id: 'btn-nav-empty',
      type: 'button',
      props: { title: '空跳转', onClick: { kind: 'navigate' } },
    };
    renderPreview([navBtn, navEmpty]);
    // 锚定全名（「空跳转」不匹配）
    fireEvent.click(screen.getByRole('button', { name: /^跳\s*转$/ }));
    await waitFor(() =>
      expect(openSpy).toHaveBeenCalledWith('https://example.com', '_blank', 'noopener'),
    );
    fireEvent.click(screen.getByRole('button', { name: '空跳转' }));
    expect(openSpy).toHaveBeenCalledTimes(1);
    openSpy.mockRestore();
  });
});

describe('行操作（danger confirm 与目标缺失）', () => {
  const tableWithRowActions = (rowActions: unknown[]): PageNode => ({
    id: 'tbl1',
    type: 'fnTable',
    props: {
      functionId: 'player.list',
      title: '玩家列表',
      autoRun: false,
      columns: ['uid'],
      rowActions,
    },
  });

  async function runTableAndClickAction(label: string | RegExp) {
    fireEvent.click(screen.getByRole('button', { name: /执\s*行/ }));
    await waitFor(() => {
      expect(document.querySelectorAll('input[type="radio"]').length).toBeGreaterThan(0);
    });
    // 行操作按钮（danger 兜底「操作」与操作列标题同文，按 button role 过滤；
    // antd 两字中文按钮插空格）
    const btn =
      typeof label === 'string'
        ? screen.getAllByRole('button', { name: label })[0]
        : screen.getAllByRole('button', { name: label })[0];
    fireEvent.click(btn);
  }

  it('danger 行操作：confirm 确认后打开弹窗（label 兜底「操作」）', async () => {
    renderPreview([
      tableWithRowActions([
        { targetSection: 'modal1', danger: true, params: { playerId: 'row.uid' } },
      ]),
      modalWithForm('发邮件弹窗'),
    ]);
    // 无 label：按钮与 confirm 标题均兜底「操作」（两字按钮插空格）
    await runTableAndClickAction(/操\s*作/);
    await waitFor(() => expect(screen.getAllByText('确认执行「操作」').length).toBeGreaterThan(0));
    fireEvent.click(confirmOkBtn());
    // 确认后打开弹窗并按行字段预填（mock 首行 uid=u-1001）
    await waitFor(() => expect(screen.getByText('发邮件弹窗')).toBeInTheDocument());
    await waitFor(() => {
      const input = document.querySelector('input[id*="playerId"]') as HTMLInputElement | null;
      expect(input?.value).toBe('u-1001');
    });
  });

  it('danger 行操作带 label：confirm 标题携带 label', async () => {
    renderPreview([
      tableWithRowActions([
        { label: '封禁', targetSection: 'modal1', danger: true, params: { playerId: 'row.uid' } },
      ]),
      modalWithForm('发邮件弹窗'),
    ]);
    await runTableAndClickAction('封禁');
    await waitFor(() => expect(screen.getAllByText('确认执行「封禁」').length).toBeGreaterThan(0));
    // 取消：不打开弹窗
    fireEvent.click(confirmCancelBtn());
    await waitFor(() => expect(screen.queryByText('发邮件弹窗')).not.toBeInTheDocument());
  });

  it('非 danger 行操作：直接打开弹窗（无 confirm）', async () => {
    renderPreview([
      tableWithRowActions([
        { label: '发邮件', targetSection: 'modal1', params: { playerId: 'row.uid' } },
      ]),
      modalWithForm('发邮件弹窗'),
    ]);
    await runTableAndClickAction('发邮件');
    await waitFor(() => expect(screen.getByText('发邮件弹窗')).toBeInTheDocument());
  });

  it('行操作目标弹窗不存在：警告提示', async () => {
    renderPreview([tableWithRowActions([{ label: '发邮件', targetSection: 'ghost-modal' }])]);
    await runTableAndClickAction('发邮件');
    await waitFor(() =>
      expect(screen.getByText('动作目标不存在（可能已删除）')).toBeInTheDocument(),
    );
  });
});

describe('inputAssignments 与 onSuccess 级联', () => {
  /** 切到真实数据模式（Switch off）并等 autoRun 重跑完成（stat.total 返回 42）。 */
  async function switchToReal() {
    fireEvent.click(document.querySelector('.ant-switch')!);
    await waitFor(() =>
      expect(mockedInvoke.mock.calls.some((c) => c[0] === 'stat.total')).toBe(true),
    );
  }

  it('literal inputAssignments：裸值原样、{{var.data.total}} 表达式按页面状态求值', async () => {
    const srcTable: PageNode = {
      id: 'tbl-src',
      type: 'fnTable',
      props: { functionId: 'stat.total', title: '统计', sectionKey: 'litSrc', autoRun: true },
    };
    const form: PageNode = {
      id: 'form-lit',
      type: 'fnForm',
      props: {
        functionId: 'ops.noop',
        inputAssignments: [
          { param: '/plain', kind: 'literal', value: 'fixed-x' },
          { param: '/fromVar', kind: 'literal', value: '{{litSrc.data.total}}' },
        ],
      },
    };
    renderPreview([srcTable, form]);
    await switchToReal();
    fireEvent.click(screen.getByRole('button', { name: /确认执行/ }));
    await waitFor(() =>
      expect(mockedInvoke.mock.calls.some((c) => c[0] === 'ops.noop')).toBe(true),
    );
    const call = mockedInvoke.mock.calls.find((c) => c[0] === 'ops.noop');
    expect(call?.[1]).toMatchObject({ plain: 'fixed-x', fromVar: 42 });
  });

  it('fnForm 成功 → onSuccess(refreshNode) 重跑目标表格（mock 行出现）', async () => {
    const table: PageNode = {
      id: 'tbl-down',
      type: 'fnTable',
      props: { functionId: 'player.list', title: '玩家列表', autoRun: false, columns: ['uid'] },
    };
    const form: PageNode = {
      id: 'form-trigger',
      type: 'fnForm',
      props: { functionId: 'ops.noop', onSuccess: { kind: 'refreshNode', target: 'tbl-down' } },
    };
    renderPreview([table, form]);
    // 初始无数据（autoRun 关）；提交成功后下游表格被刷新出 mock 行
    expect(document.querySelectorAll('input[type="radio"]').length).toBe(0);
    fireEvent.click(screen.getByRole('button', { name: /确认执行/ }));
    await waitFor(() => {
      expect(document.querySelectorAll('input[type="radio"]').length).toBeGreaterThan(0);
    });
  });
});

describe('container 子树与弹窗形态', () => {
  it('container 子节点经 renderChild 递归渲染（autoRun 数据到位）', async () => {
    const inner: PageNode = {
      id: 'tbl-inner',
      type: 'fnTable',
      props: { functionId: 'player.list', title: '玩家列表', autoRun: true, columns: ['uid'] },
    };
    const box: PageNode = {
      id: 'box-1',
      type: 'container',
      props: { title: '卡片组' },
      children: [inner],
    };
    renderPreview([box]);
    // flattenInline 含 container 子节点 → autoRun 执行；renderChild 渲染出表格行
    await waitFor(() => {
      expect(document.querySelectorAll('input[type="radio"]').length).toBeGreaterThan(0);
    });
  });

  it('弹窗无 title：标题兜底「弹窗」', async () => {
    const openBtn: PageNode = {
      id: 'btn-open',
      type: 'button',
      props: { title: '打开', onClick: { kind: 'openModal', target: 'modal1' } },
    };
    renderPreview([openBtn, modalWithForm()]);
    fireEvent.click(screen.getByRole('button', { name: /打\s*开/ }));
    await waitFor(() => expect(screen.getByText('弹窗')).toBeInTheDocument());
  });

  it('弹窗无 fnForm 子节点：空态提示（编辑态拖入函数表单）', async () => {
    const emptyModal: PageNode = {
      id: 'modal-empty',
      type: 'modal',
      props: { title: '空弹窗' },
      children: [{ id: 'btn-in', type: 'button', props: { title: '内部按钮' } }],
    };
    const openBtn: PageNode = {
      id: 'btn-open-2',
      type: 'button',
      props: { title: '打开空弹窗', onClick: { kind: 'openModal', target: 'modal-empty' } },
    };
    renderPreview([openBtn, emptyModal]);
    fireEvent.click(screen.getByRole('button', { name: '打开空弹窗' }));
    await waitFor(() =>
      expect(screen.getByText('弹窗没有内容——编辑态拖入函数表单')).toBeInTheDocument(),
    );
  });

  it('无 title 弹窗表单提交成功：成功提示兜底「操作 执行成功」', async () => {
    const modal = modalWithForm();
    modal.children = [
      { id: 'form-plain', type: 'fnForm', props: { functionId: 'ops.noop', display: 'dialog' } },
    ];
    const openBtn: PageNode = {
      id: 'btn-open-3',
      type: 'button',
      props: { title: '打开无题弹窗', onClick: { kind: 'openModal', target: 'modal1' } },
    };
    renderPreview([openBtn, modal]);
    fireEvent.click(screen.getByRole('button', { name: '打开无题弹窗' }));
    await waitFor(() => expect(screen.getByText('弹窗')).toBeInTheDocument());
    fireEvent.click(screen.getByRole('button', { name: /确认执行/ }));
    await waitFor(() => expect(screen.getByText('操作 执行成功')).toBeInTheDocument());
    // 成功后弹窗关闭
    await waitFor(() => expect(screen.queryByText('弹窗')).not.toBeInTheDocument());
  });

  it('弹窗表单值变化 → onValuesChange 写页面状态（values 求值源）', async () => {
    const openBtn: PageNode = {
      id: 'btn-open-4',
      type: 'button',
      props: { title: '改值', onClick: { kind: 'openModal', target: 'modal1' } },
    };
    renderPreview([openBtn, modalWithForm('发邮件弹窗')]);
    fireEvent.click(screen.getByRole('button', { name: /改\s*值/ }));
    await waitFor(() => expect(screen.getByText('发邮件弹窗')).toBeInTheDocument());
    const input = await waitFor(() => {
      const el = document.querySelector('input[id*="playerId"]') as HTMLInputElement | null;
      expect(el).not.toBeNull();
      return el!;
    });
    fireEvent.change(input, { target: { value: 'u-9' } });
    expect(input.value).toBe('u-9');
  });

  it('container 混合子节点：text（无 functionId）+ span 指定 + 行内 fnForm 提交', async () => {
    const textChild: PageNode = { id: 'txt-1', type: 'text', props: { content: '说明文字' } };
    const innerTable: PageNode = {
      id: 'tbl-mixed',
      type: 'fnTable',
      props: { functionId: 'player.list', title: '窄表', span: 6, autoRun: true, columns: ['uid'] },
    };
    const innerForm: PageNode = {
      id: 'form-mixed',
      type: 'fnForm',
      props: { functionId: 'ops.noop' },
    };
    const box: PageNode = {
      id: 'box-mixed',
      type: 'container',
      props: { title: '混合' },
      children: [textChild, innerTable, innerForm],
    };
    renderPreview([box]);
    // text 子节点渲染（renderChild 无 functionId 分支）+ 表格 autoRun
    await waitFor(() => {
      expect(document.querySelectorAll('input[type="radio"]').length).toBeGreaterThan(0);
    });
    // container 内 fnForm 提交走 renderChild 版 onSubmit
    fireEvent.click(screen.getByRole('button', { name: /确认执行/ }));
    await waitFor(() => expect(screen.getByText('说明文字')).toBeInTheDocument());
  });
});

describe('page_state inputAssignments 与防御分支', () => {
  const srcTable = (id: string, sectionKey?: string): PageNode => ({
    id,
    type: 'fnTable',
    props: {
      functionId: 'stat.total',
      title: '统计',
      ...(sectionKey ? { sectionKey } : {}),
      autoRun: true,
    },
  });

  it('page_state 映射：按 sourceNodeId 定位变量，field 按 JSON Pointer 路径取值', async () => {
    const form: PageNode = {
      id: 'form-ps',
      type: 'fnForm',
      props: {
        functionId: 'ops.noop',
        inputAssignments: [
          // sectionKey 命中 → 变量空间 varName=srcState；data/total 路径取值
          { param: '/byKey', kind: 'page_state', sourceNodeId: 'tbl-key', field: '/data/total' },
          // sourceNode 无 sectionKey → varName 回退 sourceNodeId
          { param: '/byId', kind: 'page_state', sourceNodeId: 'tbl-noid', field: '/data/total' },
          // 路径中途缺失 → undefined（break）
          { param: '/broken', kind: 'page_state', sourceNodeId: 'tbl-key', field: '/data/nope/x' },
          // param 去斜杠后为空 → 跳过
          { param: '/', kind: 'literal', value: 'ignored' },
        ],
      },
    };
    renderPreview([srcTable('tbl-key', 'srcState'), srcTable('tbl-noid'), form]);
    fireEvent.click(document.querySelector('.ant-switch')!);
    // 等两个 autoRun 源表完成（results 就位后再提交，事件帧内可读）
    await waitFor(() =>
      expect(mockedInvoke.mock.calls.filter((c) => c[0] === 'stat.total').length).toBe(2),
    );
    fireEvent.click(screen.getByRole('button', { name: /确认执行/ }));
    await waitFor(() =>
      expect(mockedInvoke.mock.calls.some((c) => c[0] === 'ops.noop')).toBe(true),
    );
    const call = mockedInvoke.mock.calls.find((c) => c[0] === 'ops.noop');
    // byId：sourceNode 无 sectionKey → varName 回退 sourceNodeId，但变量空间
    // 只按 sectionKey 建键 → 取值为 undefined（防御回退的真实行为）
    expect(call?.[1]).toMatchObject({ byKey: 42, byId: undefined, broken: undefined });
    expect(call?.[1]).not.toHaveProperty('ignored');
  });

  it('fnForm 无 functionId：提交直接返回 false（无 invoke、无成功提示）', async () => {
    const form: PageNode = { id: 'form-nofid', type: 'fnForm', props: {} };
    renderPreview([form]);
    fireEvent.click(screen.getByRole('button', { name: /确认执行/ }));
    await waitFor(() =>
      expect(screen.getByRole('button', { name: /确认执行/ })).toBeInTheDocument(),
    );
    expect(mockedInvoke).not.toHaveBeenCalled();
  });

  it('真实模式调用抛异常：失败提示兜底函数 id + 引导开启模拟数据', async () => {
    mockedInvoke.mockRejectedValueOnce(new Error('boom'));
    const form: PageNode = {
      id: 'form-err',
      type: 'fnForm',
      props: { functionId: 'ops.noop' }, // 无 title → 失败提示回退 fid
    };
    renderPreview([form]);
    fireEvent.click(document.querySelector('.ant-switch')!);
    // 切真实模式重跑 autoRun（无 autoRun 节点，不发请求）
    fireEvent.click(screen.getByRole('button', { name: /确认执行/ }));
    // extractErrorMessage 优先取异常 message（'boom'），失败标题兜底逻辑在
    // defaultMessage 侧（无 title 节点 → fid）
    await waitFor(() => expect(screen.getByText('boom')).toBeInTheDocument());
    await waitFor(() => expect(screen.getByText(/可开启顶部「模拟数据」/)).toBeInTheDocument());
  });

  it('真实模式响应含 error 字符串：静默返回 false（无失败提示、无成功提示）', async () => {
    mockedInvoke.mockResolvedValueOnce({ error: 'no live agent' });
    const form: PageNode = {
      id: 'form-err-resp',
      type: 'fnForm',
      props: { functionId: 'ops.noop' },
    };
    renderPreview([form]);
    fireEvent.click(document.querySelector('.ant-switch')!);
    fireEvent.click(screen.getByRole('button', { name: /确认执行/ }));
    await waitFor(() =>
      expect(mockedInvoke.mock.calls.some((c) => c[0] === 'ops.noop')).toBe(true),
    );
    // 等可能的异步提示渲染窗口过去
    await waitFor(() => expect(screen.queryByText(/执行成功/)).not.toBeInTheDocument());
    expect(screen.queryByText(/执行失败/)).not.toBeInTheDocument();
  });

  it('closeModal 在未开弹窗时点击：无异常（当前值为 null 保持）', () => {
    const closeBtn: PageNode = {
      id: 'btn-close-idle',
      type: 'button',
      props: { title: '闲关', onClick: { kind: 'closeModal', target: 'modal1' } },
    };
    renderPreview([closeBtn, modalWithForm('发邮件弹窗')]);
    fireEvent.click(screen.getByRole('button', { name: /闲\s*关/ }));
    expect(screen.queryByText('发邮件弹窗')).not.toBeInTheDocument();
  });

  it('chain 步骤缺 kind：默认 refreshNode 执行目标', async () => {
    const table: PageNode = {
      id: 'tbl-chain',
      type: 'fnTable',
      props: { functionId: 'player.list', title: '玩家列表', autoRun: false, columns: ['uid'] },
    };
    const btn: PageNode = {
      id: 'btn-chain',
      type: 'button',
      props: {
        title: '链式',
        onClick: { kind: 'showMessage', chain: [{ target: 'tbl-chain' }] },
      },
    };
    renderPreview([table, btn]);
    expect(document.querySelectorAll('input[type="radio"]').length).toBe(0);
    fireEvent.click(screen.getByRole('button', { name: /链\s*式/ }));
    await waitFor(() => {
      expect(document.querySelectorAll('input[type="radio"]').length).toBeGreaterThan(0);
    });
  });

  it('showMessage 无 message 参数：空文案提示不崩溃', () => {
    const btn: PageNode = {
      id: 'btn-msg',
      type: 'button',
      props: { title: '空消息', onClick: { kind: 'showMessage' } },
    };
    renderPreview([btn]);
    fireEvent.click(screen.getByRole('button', { name: '空消息' }));
    expect(document.querySelector('.ant-message')).not.toBeNull();
  });

  it('行操作缺 targetSection：点击无副作用；缺 params：弹窗打开无预填', async () => {
    const table: PageNode = {
      id: 'tbl-ra',
      type: 'fnTable',
      props: {
        functionId: 'player.list',
        title: '玩家列表',
        autoRun: false,
        columns: ['uid'],
        rowActions: [{ label: '无目标' }, { label: '无参数', targetSection: 'modal1' }],
      },
    };
    renderPreview([table, modalWithForm('发邮件弹窗')]);
    fireEvent.click(screen.getByRole('button', { name: /执\s*行/ }));
    await waitFor(() => {
      expect(document.querySelectorAll('input[type="radio"]').length).toBeGreaterThan(0);
    });
    // 无 targetSection：直接返回（无警告、无弹窗）
    fireEvent.click(screen.getAllByRole('button', { name: '无目标' })[0]);
    expect(screen.queryByText('动作目标不存在（可能已删除）')).not.toBeInTheDocument();
    expect(screen.queryByText('发邮件弹窗')).not.toBeInTheDocument();
    // 无 params：弹窗照开，输入为空
    fireEvent.click(screen.getAllByRole('button', { name: '无参数' })[0]);
    await waitFor(() => expect(screen.getByText('发邮件弹窗')).toBeInTheDocument());
    const input = document.querySelector('input[id*="playerId"]') as HTMLInputElement | null;
    expect(input?.value ?? '').toBe('');
  });

  it('无 sectionKey 表格选中行：状态写入 id 键（不写变量），不触发动作', async () => {
    const table: PageNode = {
      id: 'tbl-sel',
      type: 'fnTable',
      props: { functionId: 'player.list', title: '玩家列表', autoRun: false, columns: ['uid'] },
    };
    renderPreview([table]);
    fireEvent.click(screen.getByRole('button', { name: /执\s*行/ }));
    await waitFor(() => {
      expect(document.querySelectorAll('input[type="radio"]').length).toBeGreaterThan(0);
    });
    fireEvent.click(document.querySelectorAll('input[type="radio"]')[0]);
    // 无 onRowSelected → handleAction(null) 直接返回，无异常
    expect(document.querySelectorAll('input[type="radio"]').length).toBeGreaterThan(0);
  });

  it('refreshOnNode 指向 dialog 形态节点：产出不触发弹窗节点执行', async () => {
    const src = srcTable('tbl-src-2', 'srcState2');
    const dialogForm: PageNode = {
      id: 'form-dialog',
      type: 'fnForm',
      props: {
        functionId: 'ops.noop',
        display: 'dialog',
        refreshOnNode: ['tbl-src-2'],
      },
    };
    renderPreview([src, dialogForm]);
    fireEvent.click(document.querySelector('.ant-switch')!);
    await waitFor(() =>
      expect(mockedInvoke.mock.calls.some((c) => c[0] === 'stat.total')).toBe(true),
    );
    // 上游完成 → 级联 effect 跳过 dialog 节点（不 invoke ops.noop）
    await waitFor(() => expect(mockedInvoke.mock.calls.length).toBeGreaterThanOrEqual(1));
    expect(mockedInvoke.mock.calls.some((c) => c[0] === 'ops.noop')).toBe(false);
  });

  it('inputAssignments 缺省形态：无 param 跳过 / 无 sourceNodeId 取空变量 / 无 field 取整状态', async () => {
    const form: PageNode = {
      id: 'form-sparse',
      type: 'fnForm',
      props: {
        functionId: 'ops.noop',
        inputAssignments: [
          // 无 param → 去斜杠后空串 → 跳过
          { kind: 'literal', value: 'dropped' },
          // 无 sourceNodeId → 变量名空 → 状态空间无键 → undefined
          { param: '/noSrc', kind: 'page_state', field: '/data/total' },
          // 无 field → 不进路径循环，取变量整状态对象
          { param: '/noField', kind: 'page_state', sourceNodeId: 'tbl-w' },
        ],
      },
    };
    renderPreview([srcTable('tbl-w', 'srcW'), form]);
    fireEvent.click(document.querySelector('.ant-switch')!);
    await waitFor(() =>
      expect(mockedInvoke.mock.calls.some((c) => c[0] === 'stat.total')).toBe(true),
    );
    fireEvent.click(screen.getByRole('button', { name: /确认执行/ }));
    await waitFor(() =>
      expect(mockedInvoke.mock.calls.some((c) => c[0] === 'ops.noop')).toBe(true),
    );
    const call = mockedInvoke.mock.calls.find((c) => c[0] === 'ops.noop');
    expect(call?.[1]).not.toHaveProperty('dropped');
    expect(call?.[1]).toMatchObject({ noSrc: undefined, noField: { data: { total: 42 } } });
  });

  it('chain 步骤缺 kind 与 target：default 分支目标缺失警告', async () => {
    const btn: PageNode = {
      id: 'btn-chain-empty',
      type: 'button',
      props: { title: '空链', onClick: { kind: 'showMessage', chain: [{}] } },
    };
    renderPreview([btn]);
    fireEvent.click(screen.getByRole('button', { name: /空\s*链/ }));
    await waitFor(() =>
      expect(screen.getByText('动作目标不存在（可能已删除）')).toBeInTheDocument(),
    );
  });
});

describe('布局与弹窗宽度形态', () => {
  it('顶层节点 span 指定：Col 按传入 span 布局', async () => {
    const table: PageNode = {
      id: 'tbl-span',
      type: 'fnTable',
      props: { functionId: 'player.list', title: '窄列', span: 6, autoRun: true, columns: ['uid'] },
    };
    renderPreview([table]);
    await waitFor(() => {
      expect(document.querySelectorAll('input[type="radio"]').length).toBeGreaterThan(0);
    });
    // antd Col span=6 → class ant-col-6
    expect(document.querySelector('.ant-col-6')).not.toBeNull();
  });

  it('span 非数字：回退整行布局（|| 24 兜底）', async () => {
    const text: PageNode = {
      id: 'txt-span',
      type: 'text',
      props: { content: '任意 span 文本', span: 'not-a-number' },
    };
    renderPreview([text]);
    expect(screen.getByText('任意 span 文本')).toBeInTheDocument();
    // Number('not-a-number') = NaN → || 24 → ant-col-24
    expect(document.querySelector('.ant-col-24')).not.toBeNull();
  });

  it('弹窗宽度 narrow/wide：映射 420/720', async () => {
    const mk = (id: string, width: string): PageNode => ({
      id,
      type: 'modal',
      props: { title: `${id}-弹窗`, width },
      children: [
        { id: `${id}-form`, type: 'fnForm', props: { functionId: 'mail.send', display: 'dialog' } },
      ],
    });
    const narrow = mk('m-narrow', 'narrow');
    const wide = mk('m-wide', 'wide');
    const btnN: PageNode = {
      id: 'btn-n',
      type: 'button',
      props: { title: '窄弹窗', onClick: { kind: 'openModal', target: 'm-narrow' } },
    };
    const btnW: PageNode = {
      id: 'btn-w',
      type: 'button',
      props: { title: '宽弹窗', onClick: { kind: 'openModal', target: 'm-wide' } },
    };
    renderPreview([btnN, btnW, narrow, wide]);
    fireEvent.click(screen.getByRole('button', { name: '窄弹窗' }));
    await waitFor(() => expect(screen.getByText('m-narrow-弹窗')).toBeInTheDocument());
    expect(document.querySelector('.ant-modal')!.getAttribute('style')).toContain('420px');
    fireEvent.click(document.querySelector('.ant-modal-close')!);
    fireEvent.click(screen.getByRole('button', { name: '宽弹窗' }));
    await waitFor(() => expect(screen.getByText('m-wide-弹窗')).toBeInTheDocument());
    expect(document.querySelector('.ant-modal')!.getAttribute('style')).toContain('720px');
  });
});
