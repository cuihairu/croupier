/** D2 先行片：危险动作打开弹窗后的「确认内容与实际提交一致」。
 *
 * 危险动作在弹窗打开前已按预填参数确认一次，但弹窗内参数仍可编辑——用户在弹窗
 * 内改动后，实际提交内容已超出原确认范围，执行前按最终值重新确认；未改动则
 * 沿用原确认不重复打扰。取消重新确认则保持弹窗与已改参数，不执行。
 */
import React from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App } from 'antd';
import { CompositeRenderer } from '../CompositeRenderer';
import { confirmPayloadChanged } from '../confirmContent';
import type { CompositeSection, FormPresentationSpec } from '@/types/dashboard';

jest.mock('@/services/api/functions', () => ({ invokeFunction: jest.fn() }));
// SchemaFormRenderer 替身：可编辑输入 + 提交（覆盖「弹窗内改动参数」路径）
jest.mock('@/components/SchemaFormRenderer', () => {
  const SchemaFormMock = ({
    initialValues,
    onFinish,
    onValuesChange,
  }: {
    initialValues?: Record<string, unknown>;
    onFinish?: (values: Record<string, unknown>) => void;
    onValuesChange?: (changed: unknown, values: Record<string, unknown>) => void;
  }) => {
    const [values, setValues] = React.useState<Record<string, unknown>>(initialValues ?? {});
    return (
      <div>
        <input
          data-testid="fld-reason"
          value={String(values.reason ?? '')}
          onChange={(e) => {
            const next = { ...values, reason: e.target.value };
            setValues(next);
            onValuesChange?.(null, next);
          }}
        />
        <button type="button" data-testid="form-submit" onClick={() => onFinish?.(values)}>
          form-submit
        </button>
      </div>
    );
  };
  return { __esModule: true, default: SchemaFormMock };
});

// 重 DOM 套件在 coverage instrumentation 负载下撞默认 5s 用例预算
jest.setTimeout(20000);

const ROW = { uid: 'u1001', note: '违规', punish: 'kick' };

const BAN_FORM: FormPresentationSpec = {
  jsonSchema: {
    type: 'object',
    properties: {
      playerId: { type: 'string' },
      reason: { type: 'string' },
      punishType: { type: 'string' },
    },
  },
  fields: [
    { key: 'playerId', label: { 'zh-CN': '玩家 ID' } },
    { key: 'reason', label: { 'zh-CN': '封禁原因' } },
    {
      key: 'punishType',
      label: { 'zh-CN': '处置类型' },
      enumOptions: [
        { value: 'kick', label: { 'zh-CN': '强制下线' } },
        { value: 'ban', label: { 'zh-CN': '封禁' }, color: 'red' },
      ],
    },
  ],
};

const CONFIRM = '.ant-modal-title';

/** 行操作：参数映射为表格字段表达式（弹窗预填的来源）。 */
const rowAction = (danger: boolean) => ({
  label: { 'zh-CN': '封禁' },
  targetSection: 'banForm',
  danger,
  params: { playerId: 'uid', reason: 'note', punishType: 'punish' },
});

const sections = (danger: boolean): CompositeSection[] => [
  {
    key: 'list',
    bindingId: 'b-list',
    view: 'table',
    autoRun: true,
    title: { 'zh-CN': '玩家列表' },
    table: {
      columns: [{ key: 'uid', title: { 'zh-CN': 'UID' } }],
      rowActions: [rowAction(danger)],
    },
  },
  {
    key: 'banForm',
    bindingId: 'b-ban',
    view: 'form',
    display: 'dialog',
    title: { 'zh-CN': '封禁玩家' },
    form: BAN_FORM,
  },
];

/** 执行器替身：列表返回单行，弹窗表单提交返回同步成功。 */
const makeExec = () =>
  jest.fn(async (bindingId: string) =>
    bindingId === 'b-list' ? { data: { items: [ROW] } } : { data: { ok: true } },
  );

const formPayloads = (exec: jest.Mock): Record<string, unknown>[] =>
  exec.mock.calls
    .filter(([id]) => id === 'b-ban')
    .map(([, ctx]) => (ctx as { form: Record<string, unknown> }).form);

const renderComposite = (list: CompositeSection[], onExecute: jest.Mock): void => {
  render(
    <App>
      <CompositeRenderer sections={list} bindings={[]} onExecute={onExecute as never} />
    </App>,
  );
};

/** 危险行操作 → 确认 → 弹窗打开（后续用例在此处继续操作弹窗表单）。 */
const openDangerDialog = async (): Promise<void> => {
  await screen.findByText('u1001');
  fireEvent.click(screen.getByRole('button', { name: /封\s*禁/ }));
  await screen.findByText('确认执行「封禁」', { selector: CONFIRM });
  fireEvent.click(screen.getByRole('button', { name: 'OK' }));
  await screen.findByText('封禁玩家', { selector: CONFIRM });
};

describe('D2 确认内容一致性判定', () => {
  it('内容一致不判定改动（键序无关）', () => {
    expect(confirmPayloadChanged({ a: 1, b: 'x' }, { b: 'x', a: 1 })).toBe(false);
    expect(confirmPayloadChanged({}, {})).toBe(false);
    expect(confirmPayloadChanged(undefined, {})).toBe(false);
  });

  it('改写参数值判定改动', () => {
    expect(confirmPayloadChanged({ reason: '刷分' }, { reason: '恶意' })).toBe(true);
  });

  it('新增或删除参数都判定改动', () => {
    expect(confirmPayloadChanged({ a: 1 }, { a: 1, b: 2 })).toBe(true);
    expect(confirmPayloadChanged({ a: 1 }, {})).toBe(true);
  });

  it('嵌套对象与数组逐项比较，键序无关', () => {
    expect(confirmPayloadChanged({ o: { a: 1, b: 2 } }, { o: { b: 2, a: 1 } })).toBe(false);
    expect(confirmPayloadChanged({ o: { a: 1, b: 2 } }, { o: { b: 2, a: 2 } })).toBe(true);
    expect(confirmPayloadChanged({ list: ['x', 'y'] }, { list: ['x', 'z'] })).toBe(true);
  });

  it('空值归一：表单初始化补出空值不算改动，清空已确认值算改动', () => {
    expect(confirmPayloadChanged({ a: 1 }, { a: 1, b: '' })).toBe(false);
    expect(confirmPayloadChanged({ a: 1 }, { a: 1, b: undefined })).toBe(false);
    expect(confirmPayloadChanged({ a: 'x' }, { a: '' })).toBe(true);
    expect(confirmPayloadChanged({ a: 'x' }, {})).toBe(true);
  });
});

describe('D2 危险弹窗内改动的重新确认', () => {
  it('改动参数：按最终值重新确认后才执行', async () => {
    const exec = makeExec();
    renderComposite(sections(true), exec);
    await openDangerDialog();

    fireEvent.change(screen.getByTestId('fld-reason'), { target: { value: '恶意刷分' } });
    fireEvent.click(screen.getByTestId('form-submit'));
    expect(formPayloads(exec)).toHaveLength(0);

    const confirm = (
      await screen.findByText('确认执行「封禁玩家」', { selector: CONFIRM })
    ).closest('.ant-modal-confirm') as HTMLElement;
    // 重新确认展示最终值，原确认内容不残留
    expect(confirm).toHaveTextContent('恶意刷分');
    expect(confirm).not.toHaveTextContent('违规');
    expect(formPayloads(exec)).toHaveLength(0);

    fireEvent.click(screen.getByRole('button', { name: 'OK' }));
    await waitFor(() =>
      expect(formPayloads(exec)).toEqual([
        { playerId: 'u1001', reason: '恶意刷分', punishType: 'kick' },
      ]),
    );
  });

  it('未改动参数：沿用原确认直接执行，不重复确认', async () => {
    const exec = makeExec();
    renderComposite(sections(true), exec);
    await openDangerDialog();

    fireEvent.click(screen.getByTestId('form-submit'));
    await waitFor(() =>
      expect(formPayloads(exec)).toEqual([
        { playerId: 'u1001', reason: '违规', punishType: 'kick' },
      ]),
    );
    expect(screen.queryByText('确认执行「封禁玩家」', { selector: CONFIRM })).toBeNull();
    expect(screen.queryByText('封禁玩家', { selector: CONFIRM })).toBeNull();
  });

  it('取消重新确认：不执行，弹窗与已改参数保留', async () => {
    const exec = makeExec();
    renderComposite(sections(true), exec);
    await openDangerDialog();

    fireEvent.change(screen.getByTestId('fld-reason'), { target: { value: '恶意刷分' } });
    fireEvent.click(screen.getByTestId('form-submit'));
    await screen.findByText('确认执行「封禁玩家」', { selector: CONFIRM });
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }));

    await waitFor(() => expect(screen.queryByRole('button', { name: 'Cancel' })).toBeNull());
    expect(formPayloads(exec)).toHaveLength(0);
    expect(screen.getByText('封禁玩家', { selector: CONFIRM })).toBeInTheDocument();
    expect(screen.getByTestId('fld-reason')).toHaveValue('恶意刷分');
  });

  it('非危险打开的弹窗不带确认基线：编辑后提交不重新确认', async () => {
    const exec = makeExec();
    renderComposite(sections(false), exec);

    await screen.findByText('u1001');
    fireEvent.click(screen.getByRole('button', { name: /封\s*禁/ }));
    await screen.findByText('封禁玩家', { selector: CONFIRM });
    expect(screen.queryByText('确认执行「封禁」', { selector: CONFIRM })).toBeNull();

    fireEvent.change(screen.getByTestId('fld-reason'), { target: { value: '恶意刷分' } });
    fireEvent.click(screen.getByTestId('form-submit'));
    await waitFor(() =>
      expect(formPayloads(exec)).toEqual([
        { playerId: 'u1001', reason: '恶意刷分', punishType: 'kick' },
      ]),
    );
    expect(screen.queryByText('确认执行「封禁玩家」', { selector: CONFIRM })).toBeNull();
  });
});
