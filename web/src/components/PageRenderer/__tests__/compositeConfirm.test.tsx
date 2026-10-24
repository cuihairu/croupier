/** D2 危险操作确认覆盖（确认内容 + 确认闭环半程）。
 *
 * 确认框按字段标题与枚举文案展示将提交的内容（不显示原始字段 key），
 * 动作链步骤逐行列目标；危险动作在确认前既不开弹窗也不执行动作链。
 * 非危险动作保持既有语义：带 target 开弹窗同时执行动作链（详情弹窗用链
 * 预载自身数据），无 target 直接执行动作链。
 */
import React from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App } from 'antd';
import { CompositeRenderer } from '../CompositeRenderer';
import {
  confirmFieldTitle,
  confirmValueText,
  describeConfirmEntries,
  humanizeParamKey,
} from '../confirmContent';
import type { CompositeSection, FormPresentationSpec } from '@/types/dashboard';

jest.mock('@/services/api/functions', () => ({ invokeFunction: jest.fn() }));
// SchemaFormRenderer 替身：暴露 initialValues（验证确认内容与弹窗预填一致）
jest.mock('@/components/SchemaFormRenderer', () => ({
  __esModule: true,
  default: ({ initialValues }: { initialValues?: Record<string, unknown> }) => (
    <div data-testid="form-initial" data-values={JSON.stringify(initialValues ?? {})} />
  ),
}));

// 重 DOM 套件在 coverage instrumentation 负载下撞默认 5s 用例预算
jest.setTimeout(20000);

const BAN_FORM: FormPresentationSpec = {
  jsonSchema: {
    type: 'object',
    properties: {
      playerId: { type: 'string' },
      reason: { type: 'string' },
      punishType: { type: 'string' },
      durationDays: { type: 'number' },
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
    { key: 'durationDays', label: { 'zh-CN': '封禁天数' } },
  ],
};

const PUNISH_FIELD = BAN_FORM.fields?.find((f) => f.key === 'punishType');

const renderComposite = (sections: CompositeSection[], onExecute: jest.Mock): void => {
  render(
    <App>
      <CompositeRenderer sections={sections} bindings={[]} onExecute={onExecute as never} />
    </App>,
  );
};

describe('D2 确认内容构造', () => {
  it('参数名人性化：snake / camel 均转空格分隔词组', () => {
    expect(humanizeParamKey('player_id')).toBe('player id');
    expect(humanizeParamKey('game_env')).toBe('game env');
    expect(humanizeParamKey('playerId')).toBe('player id');
  });

  it('字段标题：展示 label 优先，未声明字段回退人性化 key', () => {
    expect(confirmFieldTitle(PUNISH_FIELD, 'punishType')).toBe('处置类型');
    expect(confirmFieldTitle(undefined, 'playerId')).toBe('player id');
  });

  it('枚举值映射为文案，数组逐项映射，缺值显示占位', () => {
    expect(confirmValueText(PUNISH_FIELD, 'ban')).toBe('封禁');
    expect(confirmValueText(PUNISH_FIELD, 'unknown')).toBe('unknown');
    expect(confirmValueText(PUNISH_FIELD, ['kick', 'ban'])).toBe('强制下线、封禁');
    expect(confirmValueText(PUNISH_FIELD, undefined)).toBe('-');
    expect(confirmValueText(undefined, { a: 1 })).toBe('{"a":1}');
  });

  it('参数按表单字段声明序排列并带上枚举文案', () => {
    const entries = describeConfirmEntries(BAN_FORM, {
      reason: '刷分',
      playerId: 'u1001',
      punishType: 'ban',
    });
    expect(entries.map((e) => [e.title, e.value])).toEqual([
      ['玩家 ID', 'u1001'],
      ['封禁原因', '刷分'],
      ['处置类型', '封禁'],
    ]);
  });
});

describe('D2 危险操作确认闭环', () => {
  /** 表格执行器替身：返回单行数据供行操作断言。 */
  const rowsExecutor = () =>
    jest
      .fn()
      .mockResolvedValue({ data: { items: [{ uid: 'u1001', note: '违规', punish: 'kick' }] } });

  const tableSection = (rowActions: NonNullable<CompositeSection['table']>): CompositeSection => ({
    key: 'list',
    bindingId: 'b-list',
    view: 'table',
    autoRun: true,
    title: { 'zh-CN': '玩家列表' },
    table: { columns: [{ key: 'uid', title: { 'zh-CN': 'UID' } }], ...rowActions },
  });

  it('带目标的危险行操作：确认框展示目标字段与枚举文案，确认后弹窗预填同一批参数', async () => {
    renderComposite(
      [
        tableSection({
          rowActions: [
            {
              label: { 'zh-CN': '封禁' },
              targetSection: 'banForm',
              danger: true,
              params: { playerId: 'uid', reason: 'note', punishType: 'punish' },
            },
          ],
        }),
        {
          key: 'banForm',
          bindingId: 'b-ban',
          view: 'form',
          display: 'dialog',
          title: { 'zh-CN': '封禁玩家' },
          form: BAN_FORM,
        },
      ],
      rowsExecutor(),
    );

    await screen.findByText('u1001');
    fireEvent.click(screen.getByRole('button', { name: /封\s*禁/ }));
    const confirm = (
      await screen.findByText('确认执行「封禁」', {
        selector: '.ant-modal-title',
      })
    ).closest('.ant-modal-confirm') as HTMLElement;
    // 确认内容：字段标题 + 枚举文案；原始字段 key 不出现在确认框里
    expect(confirm).toHaveTextContent('玩家 ID');
    expect(confirm).toHaveTextContent('封禁原因');
    expect(confirm).toHaveTextContent('处置类型');
    expect(confirm).toHaveTextContent('强制下线');
    expect(confirm).not.toHaveTextContent('playerId');
    // 未确认前弹窗未开
    expect(screen.queryByText('封禁玩家', { selector: '.ant-modal-title' })).toBeNull();

    fireEvent.click(screen.getByRole('button', { name: 'OK' }));
    await screen.findByText('封禁玩家', { selector: '.ant-modal-title' });
    const initial = screen.getByTestId('form-initial').getAttribute('data-values');
    expect(JSON.parse(initial as string)).toEqual({
      playerId: 'u1001',
      reason: '违规',
      punishType: 'kick',
    });
  });

  it('无目标弹窗的危险行操作：先确认（列链步骤）再执行链', async () => {
    const onExecute = rowsExecutor();
    renderComposite(
      [
        tableSection({
          rowActions: [
            {
              label: { 'zh-CN': '批量补偿' },
              danger: true,
              chain: [{ kind: 'runBinding', target: 'comp' }],
            },
          ],
        }),
        {
          key: 'comp',
          bindingId: 'b-comp',
          view: 'form',
          title: { 'zh-CN': '补偿' },
          form: BAN_FORM,
        },
      ],
      onExecute,
    );

    await screen.findByText('u1001');
    fireEvent.click(screen.getByRole('button', { name: /批\s*量\s*补\s*偿/ }));
    await screen.findByText('确认执行「批量补偿」', { selector: '.ant-modal-title' });
    expect(screen.getByText('runBinding → comp')).toBeInTheDocument();
    expect(onExecute).not.toHaveBeenCalledWith('b-comp', expect.anything());

    fireEvent.click(screen.getByRole('button', { name: 'OK' }));
    await waitFor(() => expect(onExecute).toHaveBeenCalledWith('b-comp', expect.anything()));
  });

  it('工具栏危险按钮无目标：先确认再执行链', async () => {
    const onExecute = jest.fn().mockResolvedValue({ data: {} });
    renderComposite(
      [
        {
          key: 'opBar',
          view: 'toolbar',
          title: { 'zh-CN': '操作区' },
          toolbar: {
            actions: [
              {
                label: { 'zh-CN': '清理补偿' },
                danger: true,
                chain: [{ kind: 'runBinding', target: 'comp' }],
              },
            ],
          },
        },
        { key: 'comp', bindingId: 'b-comp', view: 'form', title: { 'zh-CN': '补偿' } },
      ],
      onExecute,
    );

    fireEvent.click(screen.getByRole('button', { name: /清\s*理\s*补\s*偿/ }));
    await screen.findByText('确认执行「清理补偿」', { selector: '.ant-modal-title' });
    expect(onExecute).not.toHaveBeenCalledWith('b-comp', expect.anything());
    fireEvent.click(screen.getByRole('button', { name: 'OK' }));
    await waitFor(() => expect(onExecute).toHaveBeenCalledWith('b-comp', expect.anything()));
  });

  it('带 target 的危险工具栏按钮：确认前既不开弹窗也不执行动作链', async () => {
    const onExecute = jest.fn().mockResolvedValue({ data: {} });
    renderComposite(
      [
        {
          key: 'opBar',
          view: 'toolbar',
          title: { 'zh-CN': '操作区' },
          toolbar: {
            actions: [
              {
                label: { 'zh-CN': '单笔补偿' },
                targetSection: 'compForm',
                danger: true,
                chain: [{ kind: 'runBinding', target: 'comp' }],
              },
            ],
          },
        },
        {
          key: 'compForm',
          bindingId: 'b-compForm',
          view: 'form',
          display: 'dialog',
          title: { 'zh-CN': '补偿表单' },
        },
        { key: 'comp', bindingId: 'b-comp', view: 'form', title: { 'zh-CN': '补偿' } },
      ],
      onExecute,
    );

    fireEvent.click(screen.getByRole('button', { name: /单\s*笔\s*补\s*偿/ }));
    const confirm = (
      await screen.findByText('确认执行「单笔补偿」', {
        selector: '.ant-modal-title',
      })
    ).closest('.ant-modal-confirm') as HTMLElement;
    expect(confirm).toHaveTextContent('runBinding → comp');
    // 未确认前：弹窗未开、动作链未执行
    expect(screen.queryByText('补偿表单', { selector: '.ant-modal-title' })).toBeNull();
    expect(onExecute).not.toHaveBeenCalledWith('b-comp', expect.anything());

    fireEvent.click(screen.getByRole('button', { name: 'OK' }));
    await screen.findByText('补偿表单', { selector: '.ant-modal-title' });
    await waitFor(() => expect(onExecute).toHaveBeenCalledWith('b-comp', expect.anything()));
  });

  it('带 target 的非危险工具栏按钮：开弹窗同时预载弹窗数据（链照常执行）', async () => {
    const onExecute = jest.fn().mockResolvedValue({ data: { name: 'alice' } });
    renderComposite(
      [
        {
          key: 'opBar',
          view: 'toolbar',
          title: { 'zh-CN': '操作区' },
          toolbar: {
            actions: [
              {
                label: { 'zh-CN': '查看详情' },
                targetSection: 'detailModal',
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
          group: 'detailModal',
          title: { 'zh-CN': '详情字段' },
        },
      ],
      onExecute,
    );

    fireEvent.click(screen.getByRole('button', { name: /查\s*看\s*详\s*情/ }));
    await screen.findByText('详情字段', { selector: '.ant-modal-title' });
    await waitFor(() => expect(screen.getByText('alice')).toBeInTheDocument());
  });

  it('非危险行操作链：不弹确认框，直接执行', async () => {
    const onExecute = rowsExecutor();
    renderComposite(
      [
        tableSection({
          rowActions: [
            { label: { 'zh-CN': '重跑' }, chain: [{ kind: 'runBinding', target: 'comp' }] },
          ],
        }),
        { key: 'comp', bindingId: 'b-comp', view: 'form', title: { 'zh-CN': '补偿' } },
      ],
      onExecute,
    );

    await screen.findByText('u1001');
    fireEvent.click(screen.getByRole('button', { name: /重\s*跑/ }));
    expect(screen.queryAllByText('确认执行「重跑」').length).toBe(0);
    await waitFor(() => expect(onExecute).toHaveBeenCalledWith('b-comp', expect.anything()));
  });
});
