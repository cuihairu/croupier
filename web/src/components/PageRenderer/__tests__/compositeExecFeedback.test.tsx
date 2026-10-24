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
import { describeExecFailure, describeExecOutcome, isErrorExecResult } from '../execFeedback';
import type { CompositeSection, FormPresentationSpec } from '@/types/dashboard';

jest.mock('@/services/api/functions', () => ({ invokeFunction: jest.fn() }));

// 重 DOM 套件在 coverage instrumentation 负载下撞默认 5s 用例预算
jest.setTimeout(20000);

type AppApi = ReturnType<typeof App.useApp>;

/** umi request 非 2xx 的 ResponseError 形态：结构化体在 .data。 */
const apiError = (
  error: string,
  message: string,
  details?: Record<string, string> | Array<{ field: string; message: string }>,
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
    });
  });

  it('权限不足：稳定错误码与原因文案都保留（字段明细为空）', () => {
    const f = describeExecFailure(apiError('forbidden', '无权限执行 punish.ban：缺少 ban 权限'));
    expect(f.code).toBe('forbidden');
    expect(f.message).toContain('缺少 ban 权限');
    expect(f.fieldErrors).toEqual([]);
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
    const pushSpy = jest.spyOn(history, 'push');
    fireEvent.click(screen.getByRole('button', { name: /前\s*往\s*审\s*批\s*中\s*心/ }));
    expect(pushSpy).toHaveBeenCalledWith('/approvals');
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
    expect(onExecute).not.toHaveBeenCalledWith('b-list', expect.anything());
  });

  it('参数错误：details 定位到字段（展示 label + 服务端文案），弹窗与输入保留', async () => {
    const onExecute = jest.fn().mockRejectedValue(
      apiError('validation_failed', '请求参数无效', {
        playerId: '玩家 ID 不能为空',
        durationDays: '封禁天数需为正整数',
      }),
    );
    const { app } = await openBan(onExecute);
    const errorSpy = jest.spyOn(app.current!.message, 'error');

    clickSubmit();
    await waitFor(() => expect(errorSpy).toHaveBeenCalledWith('请求参数无效'));
    // 稳定错误码进标题，字段错误逐条定位（字段名按展示层 label）
    expect(screen.getByText('执行失败（validation_failed）')).toBeInTheDocument();
    expect(screen.getByText('玩家 ID：玩家 ID 不能为空')).toBeInTheDocument();
    expect(screen.getByText('封禁天数：封禁天数需为正整数')).toBeInTheDocument();
    // 失败保留弹窗与已填参数
    expect(screen.getByText('处罚玩家', { selector: '.ant-modal-title' })).toBeInTheDocument();
    expect((screen.getByLabelText('玩家 ID') as HTMLInputElement).value).toBe('u1001');
  });

  it('权限不足：展示稳定错误码与服务端原因，不落字段明细', async () => {
    const onExecute = jest
      .fn()
      .mockRejectedValue(apiError('forbidden', '无权限执行 punish.ban：缺少 ban 权限'));
    await openBan(onExecute);

    clickSubmit();
    await waitFor(() =>
      expect(
        screen.getByText('执行失败（forbidden）', { selector: '.ant-alert-title' }),
      ).toBeInTheDocument(),
    );
    // 原因文案同时出现在 toast 与弹窗内反馈条
    const reasons = screen.getAllByText('无权限执行 punish.ban：缺少 ban 权限');
    expect(reasons.length).toBeGreaterThan(1);
    expect(reasons.some((n) => !!n.closest('.ant-alert'))).toBe(true);
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
});
