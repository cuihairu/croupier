/** D5 显式字段语义覆盖。
 *
 * 确认框语义强调（identity 置前加粗、expiry 红字、amount 单位后缀）、
 * 组合页列语义渲染（unit 列头 + number 单元格后缀、enum→Tag、copy→可复制、
 * datetime→带浏览器时区标签）。资源页列 unit/tz 接线在
 * ResourcePageRenderer.test.tsx 追加（其 formatDateTime 已 mock 解耦时区）。
 */
import React from 'react';
import { render, screen, waitFor } from '@testing-library/react';
import { App } from 'antd';
import { CompositeRenderer } from '../CompositeRenderer';
import { renderConfirmContent } from '../compositeRuntime';
import { describeConfirmEntries } from '../confirmContent';
import type { CompositeSection, FormPresentationSpec } from '@/types/dashboard';

jest.mock('@/services/api/functions', () => ({ invokeFunction: jest.fn() }));
// 重 DOM 套件在 coverage instrumentation 负载下撞默认 5s 用例预算
jest.setTimeout(20000);

const SEM_SPEC: FormPresentationSpec = {
  jsonSchema: { type: 'object', properties: {} },
  fields: [
    { key: 'playerId', label: { 'zh-CN': '玩家 ID' }, role: 'identity' },
    { key: 'expireAt', label: { 'zh-CN': '到期时间' }, role: 'expiry' },
    {
      key: 'coins',
      label: { 'zh-CN': '金币' },
      role: 'amount',
      unit: { 'zh-CN': '金币', 'en-US': 'coins' },
    },
    { key: 'reason', label: { 'zh-CN': '原因' }, role: 'reason' },
  ],
};

describe('D5 确认框语义强调', () => {
  it('describeConfirmEntries 携带 role/unit（从 FormFieldSpec 填充）', () => {
    const entries = describeConfirmEntries(SEM_SPEC, {
      playerId: 'p1',
      expireAt: '2030-01-01',
      coins: 100,
      reason: '作弊',
    });
    const byKey = Object.fromEntries(entries.map((e) => [e.key, e]));
    expect(byKey.playerId.role).toBe('identity');
    expect(byKey.expireAt.role).toBe('expiry');
    expect(byKey.coins.role).toBe('amount');
    expect(byKey.coins.unit).toEqual({ 'zh-CN': '金币', 'en-US': 'coins' });
    expect(byKey.reason.role).toBe('reason');
  });

  it('identity 置前加粗；expiry 红字；amount 值带单位后缀', () => {
    render(
      <App>
        <div>
          {renderConfirmContent({
            entries: describeConfirmEntries(SEM_SPEC, {
              expireAt: '2030-01-01',
              coins: 100,
              reason: '作弊',
              playerId: 'p1',
            }),
          })}
        </div>
      </App>,
    );
    // identity 置前：第一行条目是 playerId（原序中它最后）
    const labels = Array.from(document.querySelectorAll('.ant-descriptions-item-label')).map(
      (el) => el.textContent,
    );
    expect(labels[0]).toContain('玩家 ID');
    // identity 加粗
    expect(document.querySelector('strong')?.textContent).toContain('p1');
    // expiry 红字
    expect(document.querySelector('.ant-typography-danger')?.textContent).toContain('2030-01-01');
    // amount 单位后缀
    expect(screen.getByText('100 金币')).toBeInTheDocument();
  });
});

describe('D5 组合页列语义渲染', () => {
  const ROWS = [
    {
      uid: 'u01',
      coins: 100,
      state: 'active',
      id: 'id-1',
      createdAt: '2024-01-01T12:00:00Z',
    },
  ];

  const columns = [
    { key: 'uid', title: { 'zh-CN': 'UID' }, dataType: 'string' },
    { key: 'coins', title: { 'zh-CN': '金币' }, dataType: 'number', unit: { 'zh-CN': '金币' } },
    {
      key: 'state',
      title: { 'zh-CN': '状态' },
      dataType: 'enum',
      render: 'tag',
      enum: [{ value: 'active', label: { 'zh-CN': '活跃' }, color: 'green' }],
    },
    { key: 'id', title: { 'zh-CN': 'ID' }, dataType: 'string', render: 'copy' },
    { key: 'createdAt', title: { 'zh-CN': '创建时间' }, dataType: 'datetime' },
  ];

  const section: CompositeSection = {
    key: 'list',
    bindingId: 'b-table',
    view: 'table',
    title: { 'zh-CN': '玩家' },
    autoRun: true,
    table: {
      columns,
      pagination: { enabled: false },
    } as CompositeSection['table'],
  };

  const executor = jest.fn().mockResolvedValue({ data: { items: ROWS, total: 1 } });

  it('unit 列头带单位、number 单元格加单位后缀', async () => {
    render(
      <App>
        <CompositeRenderer sections={[section]} bindings={[]} onExecute={executor as never} />
      </App>,
    );
    await waitFor(() => expect(screen.getByText('u01')).toBeInTheDocument());
    expect(screen.getByText('金币 (金币)')).toBeInTheDocument();
    expect(screen.getByText('100 金币')).toBeInTheDocument();
  });

  it('enum 列渲染 Tag 标签', async () => {
    render(
      <App>
        <CompositeRenderer sections={[section]} bindings={[]} onExecute={executor as never} />
      </App>,
    );
    await waitFor(() => expect(screen.getByText('u01')).toBeInTheDocument());
    const tag = document.querySelector('.ant-tag');
    expect(tag?.textContent).toBe('活跃');
  });

  it('copy 列渲染可复制文本；datetime 列带时区标签', async () => {
    render(
      <App>
        <CompositeRenderer sections={[section]} bindings={[]} onExecute={executor as never} />
      </App>,
    );
    await waitFor(() => expect(screen.getByText('u01')).toBeInTheDocument());
    expect(document.querySelector('.ant-typography-copy')).toBeTruthy();
    expect(screen.getByText(/\(UTC[+-]\d{2}:\d{2}\)$/)).toBeInTheDocument();
  });
});
