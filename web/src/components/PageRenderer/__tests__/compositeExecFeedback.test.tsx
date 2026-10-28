/** D4 执行反馈覆盖（渲染端半程）。
 *
 * 三类执行结果不再共用一条「成功」语义：
 * 1. 审批待处理 / 任务已提交 ≠ 业务执行成功——弹窗保持开启、给审批 / 任务
 *    入口，且不触发 success 事件与 onSuccessRefresh 刷新级联；
 * 2. 参数错误把 details 定位到字段（展示层 label + 服务端文案），权限不足
 *    展示稳定错误码与服务端原因；
 * 3. 失败保留弹窗与已填参数（不静默清掉用户输入）。
 */
import React from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App } from 'antd';
import { history } from '@umijs/max';
import { CompositeRenderer } from '../CompositeRenderer';
import {
  describeExecFailure,
  describeExecOutcome,
  describePermissionReason,
  isErrorExecResult,
} from '../execFeedback';
import type { CompositeSection, FormPresentationSpec } from '@/types/dashboard';

jest.mock('@/services/api/functions', () => ({ invokeFunction: jest.fn() }));

// 重 DOM 套件在 coverage instrumentation 负载下撞默认 5s 用例预算
jest.setTimeout(20000);

type AppApi = ReturnType<typeof App.useApp>;

/** umi request 非 2xx 的 ResponseError 形态：结构化体在 .data。 */
const apiError = (
  error: string,
  message: string,
  details?: Record<string, unknown> | Array<{ field: string; message: string }>,
): Error => {
  const e = new Error(message) as Error & { data: Record<string, unknown> };
  e.data = { error, message, ...(details ? { details } : {}) };
  return e;
};

describe('D4 失败反馈构造', () => {
  it('details 映射形态定位字段，稳定错误码取自 body.error', () => {
    const f = describeExecFailure(
      apiError('validation_failed', '请求参数无效', { to: '收件人不能为空' }),
    );
    expect(f).toEqual({
      code: 'validation_failed',
      message: '请求参数无效',
      fieldErrors: [{ field: 'to', message: '收件人不能为空' }],
      outcome: 'failed',
    });
  });

  it('details 数组形态可解析，body 挂在 .response.data 时同样读取', () => {
    const err = apiError('validation_failed', '参数错误', [
      { field: 'durationDays', message: '需为正整数' },
    ]);
    expect(describeExecFailure(err).fieldErrors).toEqual([
      { field: 'durationDays', message: '需为正整数' },
    ]);
    expect(describeExecFailure({ response: { data: err.data } }).fieldErrors).toEqual([
      { field: 'durationDays', message: '需为正整数' },
    ]);
  });

  it('非结构化异常回退 Error.message，字段明细为空', () => {
    expect(describeExecFailure(new Error('网络错误'))).toEqual({
      code: '',
      message: '网络错误',
      fieldErrors: [],
      reason: undefined,
      outcome: 'failed',
    });
  });

  it('传输层失败（超时/网络中断）结果未知，不按「已失败」归类', () => {
    expect(describeExecFailure(new Error('timeout of 5000ms exceeded')).outcome).toBe('unknown');
    expect(describeExecFailure(new TypeError('Failed to fetch')).outcome).toBe('unknown');
    const abort = new Error('aborted');
    abort.name = 'AbortError';
    expect(describeExecFailure(abort).outcome).toBe('unknown');
    // 服务端已应答（ResponseError 形态）即使 message 含 timeout 也不属未知
    expect(describeExecFailure(apiError('gateway_timeout', '网关超时')).outcome).toBe('failed');
  });

  it('权限不足：稳定错误码、服务端原因与 details 都保留（不暴露原始 key）', () => {
    const f = describeExecFailure(
      apiError('permission_denied', '无权限执行 punish.ban：缺少 ban 权限', {
        required: ['ban:write'],
        allowedRoles: ['operator'],
        functionId: 'punish.ban',
      }),
    );
    expect(f.code).toBe('permission_denied');
    expect(f.fieldErrors).toEqual([]);
    expect(f.reason).toBe('缺少权限：ban:write；需要角色：operator');
  });

  it('权限不足（forbidden 无 details）：稳定码与文案保留，无原因行', () => {
    const f = describeExecFailure(apiError('forbidden', '无权限执行 punish.ban：缺少 ban 权限'));
    expect(f.code).toBe('forbidden');
    expect(f.message).toContain('缺少 ban 权限');
    expect(f.fieldErrors).toEqual([]);
    expect(f.reason).toBeUndefined();
  });

  it('权限 details 收敛：缺 required 时用函数标识，非权限码不解析', () => {
    expect(describePermissionReason('permission_denied', { functionId: 'mail.send' })).toBe(
      '函数：mail.send',
    );
    expect(describePermissionReason('validation_failed', { required: ['x'] })).toBeUndefined();
  });
});

describe('D4 结果分类', () => {
  it('审批 / 任务与业务执行成功分开归类', () => {
    expect(describeExecOutcome({ kind: 'approval', requestId: 'r1', approvalId: 'a1' })).toBe(
      'approval',
    );
    expect(describeExecOutcome({ kind: 'task', requestId: 'r1', taskId: 't1' })).toBe('task');
    expect(describeExecOutcome({ kind: 'sync', requestId: 'r1', data: {} })).toBe('success');
  });

  it('空结果与顶层 error 标记单独判定，都不算业务执行成功', () => {
    expect(isErrorExecResult({ error: 'boom' })).toBe(true);
    expect(isErrorExecResult({ data: { error: '内层 error 不算' } })).toBe(false);
    expect(isErrorExecResult(undefined)).toBe(false);
    // 空结果（preview / 旧响应拒收）无 kind，需由调用方以 !r 门控
    expect(describeExecOutcome(undefined)).toBe('success');
    expect(describeExecOutcome(null)).toBe('success');
  });
});

describe('D4 执行反馈闭环', () => {
  const BAN_FORM: FormPresentationSpec = {
    jsonSchema: {
      type: 'object',
      properties: {
        playerId: { type: 'string' },
        durationDays: { type: 'number' },
      },
    },
    fields: [
      { key: 'playerId', label: { 'zh-CN': '玩家 ID' } },
      { key: 'durationDays', label: { 'zh-CN': '封禁天数' } },
    ],
  };

  const sections: CompositeSection[] = [
    {
      key: 'opBar',
      bindingId: 'b-tb',
      view: 'toolbar',
      title: { 'zh-CN': '操作区' },
      toolbar: { actions: [{ label: { 'zh-CN': '打开处罚' }, targetSection: 'banForm' }] },
    },
    {
      key: 'banForm',
      bindingId: 'b-ban',
      view: 'form',
      display: 'dialog',
      title: { 'zh-CN': '处罚玩家' },
      form: BAN_FORM,
      // 业务执行成功后才应刷新目标区块——审批 / 任务态不该触达
      onSuccessRefresh: ['list'],
    },
    {
      key: 'list',
      bindingId: 'b-list',
      view: 'table',
      title: { 'zh-CN': '玩家列表' },
      table: { columns: [{ key: 'uid', title: { 'zh-CN': 'UID' } }] },
    },
  ];

  function renderComposite(onExecute: jest.Mock) {
    const captured: { current?: AppApi } = {};
    const Probe: React.FC = () => {
      captured.current = App.useApp();
      return null;
    };
    const utils = render(
      <App>
        <Probe />
        <CompositeRenderer sections={sections} bindings={[]} onExecute={onExecute as never} />
      </App>,
    );
    return { ...utils, app: captured };
  }

  /** 打开弹窗并填玩家 ID（message spy 须在点提交前装好）。 */
  async function openBan(onExecute: jest.Mock) {
    const { app } = renderComposite(onExecute);
    fireEvent.click(screen.getByRole('button', { name: /打\s*开\s*处\s*罚/ }));
    await screen.findByText('处罚玩家', { selector: '.ant-modal-title' });
    fireEvent.change(screen.getByLabelText('玩家 ID'), { target: { value: 'u1001' } });
    return { app };
  }

  const clickSubmit = () => fireEvent.click(screen.getByRole('button', { name: /提\s*交/ }));

  /** D1 阶段条断言：标题所在 Steps item 的 ant-steps-item-<status> 状态类。
   * 弹窗提交按钮文案可能同为「执行」，必须在 .ant-steps 容器内查找。 */
  const expectStage = (title: string, status: string) => {
    const items = Array.from(document.querySelectorAll('.ant-steps-item'));
    const node = items.find((n) => n.textContent?.includes(title)) as HTMLElement | undefined;
    expect(node).toBeDefined();
    expect(node!.className).toContain(`ant-steps-item-${status}`);
  };
  /** 阶段条内是否出现某标题（弹窗按钮等外部同文案不干扰）。 */
  const stageHasText = (title: string) =>
    !!Array.from(document.querySelectorAll('.ant-steps-item')).find((n) =>
      n.textContent?.includes(title),
    );

  it('审批待处理：提示已提交审批、弹窗保持开启给审批入口，不刷新目标区块', async () => {
    const onExecute = jest
      .fn()
      .mockResolvedValue({ kind: 'approval', requestId: 'r-1', approvalId: 'ap-101', data: {} });
    const { app } = await openBan(onExecute);
    const infoSpy = jest.spyOn(app.current!.message, 'info');

    clickSubmit();
    await waitFor(() =>
      expect(onExecute).toHaveBeenCalledWith(
        'b-ban',
        expect.objectContaining({ form: expect.objectContaining({ playerId: 'u1001' }) }),
      ),
    );
    await waitFor(() => expect(infoSpy).toHaveBeenCalledWith('操作已提交审批，等待审批通过后生效'));
    // 弹窗保持开启 + 待处理反馈条（审批单号）+ 审批中心入口
    expect(screen.getByText('处罚玩家', { selector: '.ant-modal-title' })).toBeInTheDocument();
    expect(screen.getByText('已提交审批，等待审批通过后生效')).toBeInTheDocument();
    expect(screen.getByText('审批单 ap-101')).toBeInTheDocument();
    // D1 阶段条：提交完成、审批进行中、执行待审批
    expectStage('提交操作', 'finish');
    expectStage('审批', 'process');
    expectStage('执行', 'wait');
    const pushSpy = jest.spyOn(history, 'push');
    fireEvent.click(screen.getByRole('button', { name: /前\s*往\s*审\s*批\s*中\s*心/ }));
    expect(pushSpy).toHaveBeenCalledWith('/approvals?approvalId=ap-101');
    pushSpy.mockRestore();
    // 审批未通过 → 业务未变更，刷新级联不应触达目标区块
    expect(onExecute).not.toHaveBeenCalledWith('b-list', expect.anything());
  });

  it('任务已提交：提示任务已提交、弹窗保持开启给任务号，不刷新目标区块', async () => {
    const onExecute = jest
      .fn()
      .mockResolvedValue({ kind: 'task', requestId: 'r-2', taskId: 'task-7', data: {} });
    const { app } = await openBan(onExecute);
    const successSpy = jest.spyOn(app.current!.message, 'success');

    clickSubmit();
    await waitFor(() => expect(successSpy).toHaveBeenCalledWith('任务已提交，正在后台执行'));
    expect(screen.getByText('处罚玩家', { selector: '.ant-modal-title' })).toBeInTheDocument();
    // 提示同时出现在 toast 与弹窗内反馈条（任务提交 ≠ 业务成功，弹窗留着）
    const notices = screen.getAllByText('任务已提交，正在后台执行');
    expect(notices.length).toBeGreaterThan(1);
    expect(notices.some((n) => !!n.closest('.ant-alert'))).toBe(true);
    expect(screen.getByText('任务 task-7')).toBeInTheDocument();
    const pushSpy = jest.spyOn(history, 'push');
    fireEvent.click(screen.getByRole('button', { name: /前\s*往\s*任\s*务\s*中\s*心/ }));
    expect(pushSpy).toHaveBeenCalledWith('/ops/jobs');
    pushSpy.mockRestore();
    expect(onExecute).not.toHaveBeenCalledWith('b-list', expect.anything());
    // D1 阶段条：提交完成、执行进行中（无审批阶段）
    expectStage('提交操作', 'finish');
    expectStage('执行', 'process');
    expect(stageHasText('审批')).toBe(false);
  });

  it('参数错误：字段错误注入控件，反馈条只留表单外字段，弹窗与输入保留', async () => {
    const onExecute = jest.fn().mockRejectedValue(
      apiError('validation_failed', '请求参数无效', {
        playerId: '玩家 ID 不能为空',
        durationDays: '封禁天数需为正整数',
        channel: '渠道不可用', // 不在表单 schema 内 → 不进控件，留反馈条汇总
      }),
    );
    const { app } = await openBan(onExecute);
    const errorSpy = jest.spyOn(app.current!.message, 'error');

    clickSubmit();
    await waitFor(() => expect(errorSpy).toHaveBeenCalledWith('请求参数无效'));
    expect(screen.getByText('执行失败（validation_failed）')).toBeInTheDocument();
    // 表单内字段错误由 RJSF extraErrors 注入控件下方，不在反馈条重复列出
    const alert = screen
      .getByText('执行失败（validation_failed）', { selector: '.ant-alert-title' })
      .closest('.ant-alert')!;
    for (const text of ['玩家 ID 不能为空', '封禁天数需为正整数']) {
      expect(screen.getByText(text).closest('.ant-alert')).toBeNull();
      expect(alert).not.toHaveTextContent(text);
    }
    // 表单外未知字段仍留在反馈条，按 key 人性化展示
    expect(alert).toHaveTextContent('channel：渠道不可用');
    // D1 阶段条：失败终态停在提交阶段 error，不渲染未到达的阶段
    expectStage('提交操作', 'error');
    expect(stageHasText('审批')).toBe(false);
    expect(stageHasText('执行')).toBe(false);
    // 失败保留弹窗与已填参数
    expect(screen.getByText('处罚玩家', { selector: '.ant-modal-title' })).toBeInTheDocument();
    expect((screen.getByLabelText('玩家 ID') as HTMLInputElement).value).toBe('u1001');
  });

  it('写操作超时：结果未知提示（不按「已失败」），阶段条不落 submit error', async () => {
    const onExecute = jest.fn().mockRejectedValue(new Error('timeout of 5000ms exceeded'));
    const { app } = await openBan(onExecute);
    const errorSpy = jest.spyOn(app.current!.message, 'error');

    clickSubmit();
    await waitFor(() => expect(errorSpy).toHaveBeenCalled());
    // 反馈条为「执行结果未知」warning，不出现「执行失败」error
    expect(screen.getByText('执行结果未知', { selector: '.ant-alert-title' })).toBeInTheDocument();
    expect(screen.queryByText(/执行失败/)).toBeNull();
    // 明示请求可能已在服务端执行（不得盲目重复提交）
    expect(screen.getByText(/可能已在服务端执行/)).toBeInTheDocument();
    // D1 阶段条：结果未知不落提交阶段 error（无阶段渲染）
    expect(screen.queryByText('提交操作')).toBeNull();
    // 失败保留弹窗与已填参数（沿用失败分支）
    expect(screen.getByText('处罚玩家', { selector: '.ant-modal-title' })).toBeInTheDocument();
  });

  it('参数错误输入改动后：控件下错误消失，反馈条一并清除', async () => {
    const onExecute = jest
      .fn()
      .mockRejectedValue(
        apiError('validation_failed', '请求参数无效', { playerId: '玩家 ID 不能为空' }),
      );
    await openBan(onExecute);

    clickSubmit();
    await waitFor(() => expect(screen.getByText('玩家 ID 不能为空')).toBeInTheDocument());
    // 输入变化清掉旧的失败反馈（D4 反馈与输入状态保持一致）
    fireEvent.change(screen.getByLabelText('玩家 ID'), { target: { value: 'u1002' } });
    await waitFor(() => expect(screen.queryByText('玩家 ID 不能为空')).toBeNull());
    expect(screen.queryByText('执行失败（validation_failed）')).toBeNull();
  });

  it('权限不足：展示稳定错误码与服务端原因，不落字段明细', async () => {
    const onExecute = jest.fn().mockRejectedValue(
      apiError('permission_denied', '无权限执行 punish.ban：缺少 ban 权限', {
        required: ['ban:write'],
        functionId: 'punish.ban',
      }),
    );
    await openBan(onExecute);

    clickSubmit();
    await waitFor(() =>
      expect(
        screen.getByText('执行失败（permission_denied）', { selector: '.ant-alert-title' }),
      ).toBeInTheDocument(),
    );
    // 原因文案同时出现在 toast 与弹窗内反馈条
    const reasons = screen.getAllByText('无权限执行 punish.ban：缺少 ban 权限');
    expect(reasons.length).toBeGreaterThan(1);
    expect(reasons.some((n) => !!n.closest('.ant-alert'))).toBe(true);
    // details 收敛为可读原因，不暴露原始 key（required 不再作为字段错误）
    expect(screen.getByText('缺少权限：ban:write')).toBeInTheDocument();
    expect(screen.queryByText(/required/)).toBeNull();
    expect(screen.getByText('处罚玩家', { selector: '.ant-modal-title' })).toBeInTheDocument();
  });

  it('同步执行成功：提示执行成功、关闭弹窗并刷新目标区块', async () => {
    const onExecute = jest.fn().mockImplementation((bindingId: string) => {
      if (bindingId === 'b-list') return Promise.resolve({ data: { items: [{ uid: 'u1' }] } });
      return Promise.resolve({ data: { ok: 1 } });
    });
    const { app } = await openBan(onExecute);
    const successSpy = jest.spyOn(app.current!.message, 'success');

    clickSubmit();
    await waitFor(() => expect(successSpy).toHaveBeenCalledWith('处罚玩家 执行成功'));
    await waitFor(() =>
      expect(screen.queryByText('处罚玩家', { selector: '.ant-modal-title' })).toBeNull(),
    );
    // 业务执行成功 → 目标区块刷新（D4 只门控审批 / 任务态）
    await waitFor(() => expect(onExecute).toHaveBeenCalledWith('b-list', expect.anything()));
  });

  it('幂等重放：结果带 replayed 标记时明确告知未重复执行（结果取自服务端记录）', async () => {
    const onExecute = jest.fn().mockResolvedValue({
      kind: 'sync',
      requestId: 'r-9',
      replayed: true,
      data: { ok: 1 },
    });
    const { app } = await openBan(onExecute);
    const successSpy = jest.spyOn(app.current!.message, 'success');

    clickSubmit();
    await waitFor(() =>
      expect(successSpy).toHaveBeenCalledWith(
        '处罚玩家 已在先前提交过，本次返回服务端记录的结果（未重复执行）',
      ),
    );
    // 重放语义仍是业务执行成功：关弹窗 + 刷新目标区块
    await waitFor(() =>
      expect(screen.queryByText('处罚玩家', { selector: '.ant-modal-title' })).toBeNull(),
    );
    await waitFor(() => expect(onExecute).toHaveBeenCalledWith('b-list', expect.anything()));
  });
});

describe('D4 参数错误定位字段（inline 表单区块）', () => {
  // 非弹窗的表单区块走另一条渲染分支（CompositeRenderer 的 inline form），
  // 字段错误注入与弹窗共用 formFieldErrors 口径，两处都要断言。inline 区块
  // 提交期间不进整卡骨架（骨架卸载表单会丢值，已修），失败后已填值保留。
  const sections: CompositeSection[] = [
    {
      key: 'inlineBan',
      bindingId: 'b-ban',
      view: 'form',
      title: { 'zh-CN': '封禁玩家' },
      form: {
        jsonSchema: {
          type: 'object',
          properties: {
            playerId: { type: 'string' },
            durationDays: { type: 'number' },
          },
        },
        fields: [
          { key: 'playerId', label: { 'zh-CN': '玩家 ID' } },
          { key: 'durationDays', label: { 'zh-CN': '封禁天数' } },
        ],
      },
    },
  ];

  it('字段错误注入控件下方，反馈条只汇总表单外字段', async () => {
    const onExecute = jest.fn().mockRejectedValue(
      apiError('validation_failed', '请求参数无效', {
        playerId: '玩家 ID 不能为空',
        unknownField: '未知字段错误',
      }),
    );
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
    await waitFor(() => expect(captured.current).toBeTruthy());

    fireEvent.change(screen.getByLabelText('玩家 ID'), { target: { value: 'u9' } });
    fireEvent.click(screen.getByRole('button', { name: /提\s*交/ }));
    await waitFor(() =>
      expect(screen.getByText('执行失败（validation_failed）')).toBeInTheDocument(),
    );

    // 提交 payload 用当前输入，字段错误落在控件下方（RJSF extraErrors）不进反馈条
    await waitFor(() =>
      expect(onExecute).toHaveBeenCalledWith(
        'b-ban',
        expect.objectContaining({ form: expect.objectContaining({ playerId: 'u9' }) }),
      ),
    );
    expect(screen.getByText('玩家 ID 不能为空').closest('.ant-alert')).toBeNull();
    const alert = screen
      .getByText('执行失败（validation_failed）', { selector: '.ant-alert-title' })
      .closest('.ant-alert')!;
    expect(alert).not.toHaveTextContent('玩家 ID 不能为空');
    expect(alert).toHaveTextContent('unknown field：未知字段错误');
  });

  it('提交失败保留已填输入：表单不随整卡骨架卸载，续改后续提交沿用保留值', async () => {
    const onExecute = jest
      .fn()
      .mockRejectedValue(
        apiError('validation_failed', '请求参数无效', { playerId: '玩家 ID 不能为空' }),
      );
    render(
      <App>
        <CompositeRenderer sections={sections} bindings={[]} onExecute={onExecute as never} />
      </App>,
    );

    fireEvent.change(screen.getByLabelText('玩家 ID'), { target: { value: 'u9' } });
    fireEvent.click(screen.getByRole('button', { name: /提\s*交/ }));
    await waitFor(() =>
      expect(screen.getByText('执行失败（validation_failed）')).toBeInTheDocument(),
    );

    // 失败后表单仍在（不再被骨架卸载重挂），已填值保留
    expect(screen.getByLabelText('玩家 ID')).toHaveValue('u9');
    fireEvent.change(screen.getByLabelText('封禁天数'), { target: { value: '30' } });
    fireEvent.click(screen.getByRole('button', { name: /提\s*交/ }));
    await waitFor(() =>
      expect(onExecute).toHaveBeenLastCalledWith(
        'b-ban',
        expect.objectContaining({
          form: expect.objectContaining({ playerId: 'u9', durationDays: 30 }),
        }),
      ),
    );
  });
});
