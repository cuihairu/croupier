/** D2 跨目标总量：x-field-per-target 字段在批量确认（目标数 >1）时追加
 * 「总量 = 每目标值 × 目标数」条目。
 *
 * 纯函数：describeConfirmEntries 只对声明 per-target、值为有限数字、目标数
 * >1 的字段乘总量，单位/语义强调复用该字段声明（不做语义猜测）。闭环：表格
 * 区块多选 + 危险工具栏动作打开弹窗，确认框同时列出每目标值与总量。
 */
import React from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App } from 'antd';
import { CompositeRenderer } from '../CompositeRenderer';
import { describeConfirmEntries } from '../confirmContent';
import type { CompositeSection, FormPresentationSpec } from '@/types/dashboard';

jest.mock('@/services/api/functions', () => ({ invokeFunction: jest.fn() }));
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
          data-testid="fld-coins"
          value={String(values.coins ?? '')}
          onChange={(e) => {
            const next = { ...values, coins: e.target.value };
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

jest.setTimeout(20000);

const CONFIRM = '.ant-modal-title';
const COIN_FORM: FormPresentationSpec = {
  jsonSchema: {
    type: 'object',
    properties: { coins: { type: 'number' }, reason: { type: 'string' } },
  },
  fields: [
    {
      key: 'coins',
      label: { 'zh-CN': '每人数量' },
      role: 'amount',
      unit: { 'zh-CN': '金币', 'en-US': 'coins' },
      perTarget: true,
    },
    { key: 'reason', label: { 'zh-CN': '原因' } },
  ],
};

const ROWS = Array.from({ length: 4 }, (_, i) => ({
  uid: `u${String(i + 1).padStart(2, '0')}`,
  name: `r${String(i + 1).padStart(2, '0')}`,
}));

const section = (): CompositeSection[] => [
  {
    key: 'list',
    bindingId: 'b-list',
    view: 'table',
    autoRun: true,
    title: { 'zh-CN': '玩家列表' },
    toolbar: {
      actions: [
        {
          label: { 'zh-CN': '批量补偿' },
          targetSection: 'coinForm',
          danger: true,
          params: { coins: '100' },
        },
      ],
    },
    table: {
      columns: [
        { key: 'uid', title: { 'zh-CN': 'UID' }, dataType: 'string' },
        { key: 'name', title: { 'zh-CN': '昵称' }, dataType: 'string' },
      ],
      selection: { multiple: true, identityKey: 'uid' },
    },
  },
  {
    key: 'coinForm',
    bindingId: 'b-coin',
    view: 'form',
    display: 'dialog',
    title: { 'zh-CN': '批量补偿' },
    form: COIN_FORM,
  },
];

const makeExec = () =>
  jest.fn(async (bindingId: string) =>
    bindingId === 'b-list' ? { data: { items: ROWS } } : { data: { ok: true } },
  );

const rowCheckbox = (name: string): HTMLInputElement => {
  const cell = screen.getByText(name).closest('tr');
  return cell!.querySelector('input[type="checkbox"]') as HTMLInputElement;
};

describe('D2 跨目标总量（纯函数）', () => {
  it('per-target + 目标数 >1：追加总量条目（值×计数，单位与语义保留）', () => {
    const entries = describeConfirmEntries(
      COIN_FORM,
      { coins: 100, reason: '补偿' },
      {
        targetCount: 3,
      },
    );
    const coins = entries.find((e) => e.key === 'coins')!;
    expect(coins.value).toBe('100');
    expect(coins.unit?.['zh-CN']).toBe('金币');
    const total = entries.find((e) => e.key === 'coins__batchTotal')!;
    expect(total).toBeDefined();
    expect(total.value).toBe('300');
    expect(total.title).toBe('每人数量总量');
    expect(total.role).toBe('amount');
    expect(total.unit?.['zh-CN']).toBe('金币');
  });

  it('目标数 =1 或未传：不追加总量', () => {
    expect(describeConfirmEntries(COIN_FORM, { coins: 100 }, { targetCount: 1 })).toHaveLength(1);
    expect(describeConfirmEntries(COIN_FORM, { coins: 100 })).toHaveLength(1);
  });

  it('未声明 per-target：即使批量也不乘总量', () => {
    const plain: FormPresentationSpec = {
      ...COIN_FORM,
      fields: [
        { key: 'coins', label: { 'zh-CN': '数量' }, role: 'amount', unit: { 'zh-CN': '个' } },
      ],
    };
    expect(describeConfirmEntries(plain, { coins: 100 }, { targetCount: 3 })).toHaveLength(1);
  });

  it('非数字值（空/文本）：不追加总量', () => {
    expect(describeConfirmEntries(COIN_FORM, { coins: '' }, { targetCount: 3 })).toHaveLength(1);
    expect(describeConfirmEntries(COIN_FORM, { coins: 'abc' }, { targetCount: 3 })).toHaveLength(1);
  });
});

describe('D2 跨目标总量（批量确认闭环）', () => {
  it('多选后危险工具栏动作：确认框列出每目标值与总量', async () => {
    const exec = makeExec();
    render(
      <App>
        <CompositeRenderer sections={section()} bindings={[]} onExecute={exec as never} />
      </App>,
    );
    await screen.findByText('r01');

    fireEvent.click(rowCheckbox('r01'));
    fireEvent.click(rowCheckbox('r02'));
    await waitFor(() => expect(screen.getByTestId('selection-count')).toBeInTheDocument());

    fireEvent.click(screen.getByRole('button', { name: /批量补偿/ }));
    await screen.findByText('确认执行「批量补偿」', { selector: CONFIRM });

    const confirmBox = screen.getByRole('dialog');
    expect(confirmBox).toHaveTextContent('每人数量');
    expect(confirmBox).toHaveTextContent('100 金币');
    expect(confirmBox).toHaveTextContent('每人数量总量');
    expect(confirmBox).toHaveTextContent('200 金币');
  });
});
