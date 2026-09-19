/** CompositeRenderer 分支缺口补齐（v8 branch 覆盖专项）。
 *
 * 覆盖点（对应源码行）：
 * - L74/L102/L115 防抖定时器：fnForm 连续输入重置 values 定时器、static
 *   连续输入只取末次值、卸载时清理未触发的 values 定时器；
 * - L132/L136 空结果形态：onExecute resolve null / 无 data 字段时
 *   result||null 与 data ?? null 兜底（经 onPageStateMerge 断言）；
 * - L301 showMessage 链步骤缺 params / params 缺 message → info('')；
 * - L348 rowAction 无 params 声明 → 空参数打开弹窗（mapping || {} 兜底）；
 * - L363/L595/L624 分组缺省：dialog/tab/card 区块无 group 时以 key 为组名；
 * - L434 unbound 空态：bindingId 不在 bindings（?. 短路）/ bindingId 空串
 *   （三元 false 侧）的 functionId 兜底；
 * - L451-453 选中行 rowSelected 事件（含 page_state merge 复守）；
 * - L457/L520/L746 区块声明缺省：table/toolbar 视图与弹窗 table 缺 spec；
 * - L512/L736 fields 渲染空值（inline null / dialog undefined → '-'）；
 * - L757 弹窗内 actions 区块渲染占位 null。
 *
 * 文末注释列出判定为不可达的防御分支。 */
import React from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App } from 'antd';
import { CompositeRenderer } from '../CompositeRenderer';
import type { CompositeSection, PageFunctionBinding } from '@/types/dashboard';

jest.mock('@/services/api/functions', () => ({ invokeFunction: jest.fn() }));

// 全量并行高负载下曾撞默认 5s 超时（隔离跑恒绿）——放宽用例级预算
jest.setTimeout(20000);

type AppApi = ReturnType<typeof App.useApp>;

interface RenderOptions {
  bindings?: PageFunctionBinding[];
  onPageStateMerge?: jest.Mock;
}

/** 渲染并捕获 App.useApp 实例（message spy 用） */
function renderComposite(sections: CompositeSection[], onExecute: jest.Mock, opts?: RenderOptions) {
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
        bindings={opts?.bindings ?? []}
        onExecute={onExecute as never}
        preview={false}
        onPageStateMerge={opts?.onPageStateMerge as never}
      />
    </App>,
  );
  return { ...utils, app: captured };
}

const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));

// 409 executor_unbound 同构错误（extractApiErrorCode 只读 err.data.error）
const unboundError = () =>
  Object.assign(new Error('executor unbound'), { data: { error: 'executor_unbound' } });

describe('CompositeRenderer 防抖定时器分支', () => {
  it('fnForm 连续两次输入（300ms 内）：重置首个 values 定时器，仅末次值生效（L74）', async () => {
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
    renderComposite(sections, onExecute);

    const toInput = (await screen.findByLabelText('收件人')) as HTMLInputElement;
    fireEvent.change(toInput, { target: { value: 'first@test' } });
    fireEvent.change(toInput, { target: { value: 'second@test' } });
    // 第二次输入重置定时器后 300ms 防抖仅 flush 一次，值为末次输入
    await sleep(450);
    fireEvent.click(screen.getByRole('button', { name: '取表单值' }));
    await waitFor(() =>
      expect(onExecute).toHaveBeenCalledWith(
        'b-consumer',
        expect.objectContaining({ form: { v: 'second@test' } }),
      ),
    );
  });

  it('static 常量表单连续两次输入（400ms 内）：重置定时器，仅末次值并入 page_state（L102）', async () => {
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
    renderComposite(sections, onExecute, { onPageStateMerge });

    const nameInput = (await screen.findByLabelText('玩家名')) as HTMLInputElement;
    fireEvent.change(nameInput, { target: { value: 'alice' } });
    fireEvent.change(nameInput, { target: { value: 'bob' } });
    await waitFor(
      () =>
        expect(onPageStateMerge).toHaveBeenCalledWith('staticForm', {
          name: 'bob',
          values: { name: 'bob' },
        }),
      { timeout: 1500 },
    );
  });

  it('fnForm 输入后立即卸载：清理未触发的 values 防抖定时器（L115）', async () => {
    const clearSpy = jest.spyOn(globalThis, 'clearTimeout');
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
      },
    ];
    const onExecute = jest.fn();
    const utils = renderComposite(sections, onExecute);
    const toInput = (await screen.findByLabelText('收件人')) as HTMLInputElement;
    fireEvent.change(toInput, { target: { value: 'pending@test' } });
    utils.unmount();
    // 卸载清理 effect：pending 中的 values 定时器被 clearTimeout 清掉
    expect(clearSpy).toHaveBeenCalled();
    await sleep(450); // 卸载后防抖不再触发（无渲染目标、无报错）
    clearSpy.mockRestore();
  });
});

describe('CompositeRenderer 执行结果空形态', () => {
  it('onExecute resolve null / 无 data 字段：结果写 null、page_state 合并 data:null（L132/L136）', async () => {
    const sections: CompositeSection[] = [
      { key: 'nullSec', bindingId: 'b-null', view: 'fields', title: { 'zh-CN': '空结果区块' } },
      {
        key: 'noDataSec',
        bindingId: 'b-nodata',
        view: 'fields',
        title: { 'zh-CN': '无字段结果区块' },
      },
    ];
    const onExecute = jest
      .fn()
      .mockImplementation((bindingId: string) =>
        bindingId === 'b-null' ? Promise.resolve(null) : Promise.resolve({}),
      );
    const onPageStateMerge = jest.fn();
    renderComposite(sections, onExecute, { onPageStateMerge });

    // 非表单且非 autoRun 的 fields 区块在卡片头渲染「执行」按钮（按区块顺序）
    const runButtons = await screen.findAllByRole('button', { name: /执\s*行/ });
    expect(runButtons).toHaveLength(2);
    fireEvent.click(runButtons[0]);
    fireEvent.click(runButtons[1]);
    // null → result || null 写入；{} → data ?? null 兜底，两条链均合并 data:null
    await waitFor(() =>
      expect(onPageStateMerge).toHaveBeenCalledWith('nullSec', { data: null }, 'merge'),
    );
    await waitFor(() =>
      expect(onPageStateMerge).toHaveBeenCalledWith('noDataSec', { data: null }, 'merge'),
    );
  });
});

describe('CompositeRenderer 动作链与参数缺省', () => {
  it('showMessage 链步骤缺 params / params 缺 message：info 空串兜底（L301）', async () => {
    const sections: CompositeSection[] = [
      {
        key: 'noParams',
        bindingId: 'b-a',
        view: 'form',
        title: { 'zh-CN': '无参动作' },
        events: [{ event: 'success', action: { kind: 'showMessage', target: '' } }],
      },
      {
        key: 'noMessage',
        bindingId: 'b-b',
        view: 'form',
        title: { 'zh-CN': '空参动作' },
        events: [{ event: 'success', action: { kind: 'showMessage', target: '', params: {} } }],
      },
    ];
    const onExecute = jest.fn().mockResolvedValue({ data: {} });
    const { app } = renderComposite(sections, onExecute);
    const infoSpy = jest.spyOn(app.current!.message, 'info');

    // 无表单声明的区块降级为标题按钮，点击执行并派发 success 事件（按声明顺序）
    fireEvent.click(screen.getByRole('button', { name: '无参动作' }));
    fireEvent.click(screen.getByRole('button', { name: '空参动作' }));
    await waitFor(() => expect(infoSpy).toHaveBeenCalledTimes(2));
    expect(infoSpy).toHaveBeenNthCalledWith(1, '');
    expect(infoSpy).toHaveBeenNthCalledWith(2, '');
  });

  it('rowAction 无 params 声明：空参数映射打开弹窗（mapping || {} 兜底，L348）', async () => {
    const sections: CompositeSection[] = [
      {
        key: 'mainTable',
        bindingId: 'b-table',
        view: 'table',
        autoRun: true,
        title: { 'zh-CN': '玩家列表' },
        table: {
          columns: [{ key: 'uid', title: { 'zh-CN': 'UID' } }],
          rowActions: [{ label: { 'zh-CN': '编辑备注' }, targetSection: 'noteModal' }],
        },
      },
      {
        key: 'noteForm',
        bindingId: 'b-note',
        view: 'form',
        display: 'dialog',
        group: 'noteModal',
        title: { 'zh-CN': '备注弹窗' },
        form: {
          jsonSchema: {
            type: 'object',
            properties: { note: { type: 'string', title: '备注' } },
          },
        },
      },
    ];
    const onExecute = jest.fn().mockResolvedValue({ data: { items: [{ uid: 'u1' }] } });
    renderComposite(sections, onExecute);
    await screen.findByText('u1');

    fireEvent.click(screen.getByRole('button', { name: '编辑备注' }));
    // 无 params 声明 → 映射结果为空对象，弹窗照常打开且预填为空
    const noteInput = (await screen.findByLabelText('备注')) as HTMLInputElement;
    expect(noteInput.value).toBe('');
  });
});

describe('CompositeRenderer 分组缺省（group ?? key 兜底）', () => {
  it('dialog 区块无 group：目标按区块 key 命中弹窗（L363）', async () => {
    const sections: CompositeSection[] = [
      {
        key: 'opBar',
        bindingId: 'b-tb',
        view: 'toolbar',
        title: { 'zh-CN': '操作区' },
        toolbar: { actions: [{ label: { 'zh-CN': '打开独立弹窗' }, targetSection: 'soloForm' }] },
      },
      {
        key: 'soloForm',
        bindingId: 'b-solo',
        view: 'form',
        display: 'dialog',
        title: { 'zh-CN': '独立弹窗表单' },
        form: {
          jsonSchema: {
            type: 'object',
            properties: { q: { type: 'string', title: '参数' } },
          },
        },
      },
    ];
    const onExecute = jest.fn().mockResolvedValue({ data: {} });
    renderComposite(sections, onExecute);
    fireEvent.click(screen.getByRole('button', { name: '打开独立弹窗' }));
    await screen.findByText('独立弹窗表单', { selector: '.ant-modal-title' });
  });

  it('tab 区块无 group：以区块 key 为页签组名独立成组（L595）', async () => {
    const sections: CompositeSection[] = [
      {
        key: 'soloTab',
        bindingId: 'b-tab',
        view: 'fields',
        display: 'tab',
        tab: { 'zh-CN': '独立页' },
        title: { 'zh-CN': '独立页区块' },
      },
    ];
    const onExecute = jest.fn().mockResolvedValue({ data: {} });
    renderComposite(sections, onExecute);
    expect(screen.getByRole('tab', { name: '独立页' })).toBeInTheDocument();
    expect(screen.getByText('独立页区块')).toBeInTheDocument();
  });

  it('card 区块无 group：以区块 key 为卡片组名，外层标题回落 key（L624）', async () => {
    const sections: CompositeSection[] = [
      {
        key: 'soloCard',
        bindingId: 'b-card',
        view: 'fields',
        display: 'card',
        title: { 'zh-CN': '卡内区块' },
      },
    ];
    const onExecute = jest.fn().mockResolvedValue({ data: {} });
    renderComposite(sections, onExecute);
    // 外层卡片标题 = 组名（key 兜底）；内层卡片标题 = 区块标题
    expect(screen.getByText('soloCard')).toBeInTheDocument();
    expect(screen.getByText('卡内区块')).toBeInTheDocument();
  });
});

describe('CompositeRenderer 区块声明缺省', () => {
  it('table 区块缺 table 声明：渲染空表格（columns || [] 兜底，L457）', async () => {
    const sections: CompositeSection[] = [
      { key: 'bareTable', bindingId: 'b-bare', view: 'table', title: { 'zh-CN': '裸表格' } },
    ];
    const onExecute = jest.fn().mockResolvedValue({ data: {} });
    const { container } = renderComposite(sections, onExecute);
    expect(screen.getByText('裸表格')).toBeInTheDocument();
    // columns 兜底为空 → 表头仅剩 radio 选择列，无任何数据列
    const heads = container.querySelectorAll('.ant-table-thead th');
    expect(heads).toHaveLength(1);
    expect(heads[0].className).toContain('ant-table-selection-column');
  });

  it('toolbar 区块缺 toolbar 声明：渲染空操作区且无执行按钮（L520）', async () => {
    const sections: CompositeSection[] = [
      { key: 'bareBar', bindingId: 'b-bar', view: 'toolbar', title: { 'zh-CN': '空操作区' } },
    ];
    const onExecute = jest.fn().mockResolvedValue({ data: {} });
    renderComposite(sections, onExecute);
    expect(screen.getByText('空操作区')).toBeInTheDocument();
    // toolbar 视图不渲染卡片头执行按钮（L420 条件排除）
    expect(screen.queryByRole('button', { name: /执\s*行/ })).not.toBeInTheDocument();
  });

  it('弹窗 table 区块缺 table 声明：弹窗内渲染空表格（L746）', async () => {
    const sections: CompositeSection[] = [
      {
        key: 'opBar',
        bindingId: 'b-tb',
        view: 'toolbar',
        title: { 'zh-CN': '操作区' },
        toolbar: { actions: [{ label: { 'zh-CN': '打开表格弹窗' }, targetSection: 'tModal' }] },
      },
      {
        key: 'dlgTable',
        bindingId: 'b-dt',
        view: 'table',
        display: 'dialog',
        group: 'tModal',
        title: { 'zh-CN': '弹窗表格' },
      },
    ];
    const onExecute = jest.fn().mockResolvedValue({ data: {} });
    renderComposite(sections, onExecute);
    fireEvent.click(screen.getByRole('button', { name: '打开表格弹窗' }));
    await screen.findByText('弹窗表格', { selector: '.ant-modal-title' });
    // 弹窗内 columns 兜底为空 → 表头仅一列且无任何数据列标题
    const heads = document.querySelectorAll('.ant-modal .ant-table-thead th');
    expect(heads).toHaveLength(1);
    expect(heads[0].textContent).toBe('');
  });

  it('弹窗 actions 区块：无对应渲染分支，弹窗主体为空（占位 null，L757）', async () => {
    const sections: CompositeSection[] = [
      {
        key: 'opBar',
        bindingId: 'b-tb',
        view: 'toolbar',
        title: { 'zh-CN': '操作区' },
        toolbar: { actions: [{ label: { 'zh-CN': '打开动作弹窗' }, targetSection: 'aModal' }] },
      },
      {
        key: 'dlgActions',
        bindingId: 'b-da',
        view: 'actions',
        display: 'dialog',
        group: 'aModal',
        title: { 'zh-CN': '弹窗动作' },
      },
    ];
    const onExecute = jest.fn().mockResolvedValue({ data: {} });
    renderComposite(sections, onExecute);
    fireEvent.click(screen.getByRole('button', { name: '打开动作弹窗' }));
    await screen.findByText('弹窗动作', { selector: '.ant-modal-title' });
    // actions 视图在弹窗内无渲染分支：既无表单提交也无「确认执行」降级按钮
    expect(screen.queryByRole('button', { name: /提\s*交/ })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /确认执行/ })).not.toBeInTheDocument();
  });
});

describe('CompositeRenderer fields 渲染空值', () => {
  it('inline fields 区块 null 字段值渲染「-」（L512）', async () => {
    const sections: CompositeSection[] = [
      {
        key: 'profile',
        bindingId: 'b-profile',
        view: 'fields',
        autoRun: true,
        title: { 'zh-CN': '档案' },
      },
    ];
    const onExecute = jest.fn().mockResolvedValue({ data: { name: 'bob', vipLevel: null } });
    renderComposite(sections, onExecute);
    await screen.findByText('bob');
    expect(screen.getByText('-')).toBeInTheDocument();
  });

  it('弹窗 fields 区块 undefined 字段值渲染「-」（L736）', async () => {
    const sections: CompositeSection[] = [
      {
        key: 'opBar',
        bindingId: 'b-tb',
        view: 'toolbar',
        title: { 'zh-CN': '操作区' },
        toolbar: {
          actions: [
            {
              label: { 'zh-CN': '查看详情' },
              targetSection: 'dModal',
              chain: [{ kind: 'runBinding', target: 'dlgFields' }],
            },
          ],
        },
      },
      {
        key: 'dlgFields',
        bindingId: 'b-df',
        view: 'fields',
        display: 'dialog',
        group: 'dModal',
        title: { 'zh-CN': '弹窗字段' },
      },
    ];
    const onExecute = jest
      .fn()
      .mockImplementation((bindingId: string) =>
        bindingId === 'b-df'
          ? Promise.resolve({ data: { name: 'alice', addr: undefined } })
          : Promise.resolve({ data: {} }),
      );
    renderComposite(sections, onExecute);
    fireEvent.click(screen.getByRole('button', { name: '查看详情' }));
    await screen.findByText('弹窗字段', { selector: '.ant-modal-title' });
    await waitFor(() => expect(screen.getByText('alice')).toBeInTheDocument());
    // undefined 非 object → String(v ?? '-') 兜底
    expect(screen.getByText('-')).toBeInTheDocument();
  });
});

describe('CompositeRenderer unbound 空态 functionId 兜底（L434）', () => {
  it('bindingId 不在 bindings：functionId 经 ?. 短路为空，无 code 定位', async () => {
    const sections: CompositeSection[] = [
      { key: 'ghost', bindingId: 'b-ghost', view: 'fields', title: { 'zh-CN': '幽灵区块' } },
    ];
    const onExecute = jest.fn().mockRejectedValue(unboundError());
    renderComposite(sections, onExecute, { bindings: [] });
    fireEvent.click(await screen.findByRole('button', { name: /执\s*行/ }));
    await waitFor(() => expect(screen.getByText('未绑定执行器')).toBeInTheDocument());
    // bindingById 无该 bindingId → functionId undefined → 不渲染 code 定位片段
    expect(document.querySelector('code')).toBeNull();
  });

  it('bindingId 为空串：三元 false 侧直接传 undefined', async () => {
    const sections: CompositeSection[] = [
      { key: 'emptyBind', bindingId: '', view: 'fields', title: { 'zh-CN': '空绑定区块' } },
    ];
    const onExecute = jest.fn().mockRejectedValue(unboundError());
    renderComposite(sections, onExecute, { bindings: [] });
    fireEvent.click(await screen.findByRole('button', { name: /执\s*行/ }));
    await waitFor(() => expect(screen.getByText('未绑定执行器')).toBeInTheDocument());
    expect(onExecute).toHaveBeenCalledWith('', expect.objectContaining({}));
    expect(document.querySelector('code')).toBeNull();
  });
});

describe('CompositeRenderer 表格选中行事件（L451-453 复守）', () => {
  it('选中行触发 rowSelected 事件：参数取 selectedRow 且选中态 merge 进 page_state', async () => {
    const sections: CompositeSection[] = [
      {
        key: 'mainTable',
        bindingId: 'b-table',
        view: 'table',
        autoRun: true,
        title: { 'zh-CN': '玩家列表' },
        table: {
          columns: [
            { key: 'uid', title: { 'zh-CN': 'UID' } },
            { key: 'nickname', title: { 'zh-CN': '昵称' } },
          ],
        },
        events: [
          {
            event: 'rowSelected',
            action: {
              kind: 'runBinding',
              target: 'echo',
              params: { who: '{{mainTable.selectedRow.nickname}}' },
            },
          },
        ],
      },
      { key: 'echo', bindingId: 'b-echo', view: 'fields', title: { 'zh-CN': '回显' } },
    ];
    const onExecute = jest
      .fn()
      .mockImplementation((bindingId: string) =>
        bindingId === 'b-table'
          ? Promise.resolve({ data: { items: [{ uid: 'u1', nickname: 'bob' }], total: 1 } })
          : Promise.resolve({ data: {} }),
      );
    const onPageStateMerge = jest.fn();
    renderComposite(sections, onExecute, { onPageStateMerge });
    await screen.findByText('bob');

    const radios = document.querySelectorAll('input[type="radio"]');
    expect(radios.length).toBeGreaterThan(0);
    fireEvent.click(radios[0]);

    // 选中态 merge 进 page_state（L450）+ rowSelected 事件触发（L451-453）
    await waitFor(() =>
      expect(onPageStateMerge).toHaveBeenCalledWith(
        'mainTable',
        expect.objectContaining({
          selectedRow: expect.objectContaining({ nickname: 'bob' }),
        }),
        'merge',
      ),
    );
    await waitFor(() =>
      expect(onExecute).toHaveBeenCalledWith(
        'b-echo',
        expect.objectContaining({ form: { who: 'bob' } }),
      ),
    );
  });
});

// ---------------------------------------------------------------------------
// 判定为不可达的防御分支（放弃覆盖）：
// - L93 `resultsRef.current[key] ?? {}`：行选中必先有执行结果（数据行来源于
//   results[key].data.items），选中时左侧恒非空；
// - L309 `resolveStepParams(...) ?? {}`：resolveStepParams 恒返回对象，右侧死支；
// - L363/L595/L624 第二级 `sec.key ?? sec.bindingId`：key 为区块必填字段；
// - L374 `sec.toolbar?.actions || []`（表格卡片头工具栏）：调用点受 L408
//   `actions.length > 0` 守卫，右侧死支；
// - L451 `rows[0]` 假值侧：radio 型行选择不可反选，onChange 恒带选中行；
// - L692 `sectionInputs[dialogKey] || {}`：openDialog 恒先写入同 key 输入。
// ---------------------------------------------------------------------------
