/**
 * SUI-022 验收：date / date-time / time widgets 端到端（真实 RJSF 引擎 +
 * 真实 SchemaFormRenderer + antd picker 桩记录 props）。
 *
 * 桩只替换 DatePicker / TimePicker 的 DOM 交互（jsdom 下 picker 弹层不可靠），
 * 原样透传/记录 props；rjsf、validator-ajv8、registry 解析、提交投影全真实。
 * 通过触发桩捕获的 onChange 回调驱动 widget → rjsf formData → onFinish。
 *
 * 时区边界：用例期望按进程时区用 dayjs 推导（任意 TZ 自洽）；精确字面量
 * 断言放在 TZ 守卫 describe，跑对应用时区才生效。进程级 TZ pin 各跑一轮：
 *   TZ=UTC npx jest --testPathPatterns datetime-widgets
 *   TZ=America/New_York npx jest --testPathPatterns datetime-widgets
 */
import React, { createRef } from 'react';
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import dayjs from 'dayjs';
import SchemaFormRenderer, { type SchemaFormRendererHandle } from '@/components/SchemaFormRenderer';
import type { FormPresentationSpec, JSONSchema } from '@/types/dashboard';
import { customWidgets, DateWidget, DateTimeWidget, TimeWidget } from '../widgets';

type StubProps = Record<string, unknown>;
type PickerStub = ((props: StubProps) => React.ReactElement) & { __registry: StubProps[] };

jest.mock('antd', () => {
  const React = require('react') as typeof import('react');
  const actual = jest.requireActual('antd');
  const dateRegistry: StubProps[] = [];
  const timeRegistry: StubProps[] = [];
  const makeStub = (registry: StubProps[]): PickerStub => {
    const Stub = (props: StubProps) => {
      registry.push(props);
      const value = props['value'];
      return React.createElement('input', {
        'data-testid': String(props['id'] ?? 'picker'),
        'data-picker-value':
          value && typeof value.format === 'function' ? value.format() : '',
        readOnly: true,
      });
    };
    (Stub as PickerStub).__registry = registry;
    return Stub as PickerStub;
  };
  return {
    ...actual,
    DatePicker: makeStub(dateRegistry),
    TimePicker: makeStub(timeRegistry),
  };
});

const antdMock = jest.requireMock('antd') as unknown as {
  DatePicker: PickerStub;
  TimePicker: PickerStub;
};

function lastProps(stub: PickerStub, testid: string): StubProps {
  const found = stub.__registry.filter((props) => props['id'] === testid);
  if (!found.length) throw new Error(`no picker render captured for ${testid}`);
  return found[found.length - 1];
}

function onChangeOf(props: StubProps): (value: unknown) => void {
  const handler = props['onChange'];
  if (typeof handler !== 'function') throw new Error('picker onChange handler missing');
  return handler as (value: unknown) => void;
}

/** 显示值（Dayjs | undefined）转可断言的 format 字符串。 */
function displayText(props: StubProps, pattern: string): string | undefined {
  const value = props['value'];
  if (!value || typeof value.format !== 'function') return undefined;
  return (value as { format: (p: string) => string }).format(pattern);
}

function specWith(
  properties: Record<string, unknown>,
  fields?: Record<string, unknown>[],
): FormPresentationSpec {
  return {
    jsonSchema: { type: 'object', properties } as unknown as JSONSchema,
    fields: fields as FormPresentationSpec['fields'],
  };
}

function submitForm(): void {
  fireEvent.click(screen.getByRole('button', { name: /提\s*交|submit/i }));
}

function resetStubs(): void {
  antdMock.DatePicker.__registry.length = 0;
  antdMock.TimePicker.__registry.length = 0;
}

const LOCAL_OFFSET = dayjs('2026-01-31T08:00:00').format('Z');
const resolvedTz = Intl.DateTimeFormat().resolvedOptions().timeZone;
const describeWhenTz = (tz: string): typeof describe => (resolvedTz === tz ? describe : describe.skip);

describe('SUI-022: date/date-time/time registry 覆盖（format 与 x-widget 双路径）', () => {
  beforeEach(resetStubs);

  it('schema.format 缺省路径命中覆盖实现（date/date-time/time 三组都渲染）', () => {
    const spec = specWith({
      d: { type: 'string', format: 'date', title: '日期' },
      dt: { type: 'string', format: 'date-time', title: '日期时间' },
      t: { type: 'string', format: 'time', title: '时刻' },
    });
    render(<SchemaFormRenderer spec={spec} hideSubmit />);
    expect(antdMock.DatePicker.__registry.length).toBeGreaterThanOrEqual(2);
    expect(antdMock.TimePicker.__registry.length).toBeGreaterThanOrEqual(1);
    // date-time 走 showTime 面板；date 不带
    expect(lastProps(antdMock.DatePicker, 'root_dt')['showTime']).toBe(true);
    expect(lastProps(antdMock.DatePicker, 'root_d')['showTime']).toBe(false);
  });

  it('x-widget DatePicker/TimePicker 路径命中同一实现', () => {
    const spec = specWith(
      {
        d: { type: 'string', title: '日期' },
        t: { type: 'string', title: '时刻' },
      },
      [
        { key: 'd', widget: 'DatePicker' },
        { key: 't', widget: 'TimePicker' },
      ],
    );
    render(<SchemaFormRenderer spec={spec} hideSubmit />);
    expect(antdMock.DatePicker.__registry.length).toBeGreaterThanOrEqual(1);
    expect(antdMock.TimePicker.__registry.length).toBeGreaterThanOrEqual(1);
    expect(lastProps(antdMock.TimePicker, 'root_t')).toBeTruthy();
  });
});

describe('SUI-022: 提交值与清空（真实 RJSF 提交路径）', () => {
  beforeEach(resetStubs);

  it('(a) format:date + x-widget DatePicker 提交 YYYY-MM-DD（无时间、无偏移）', async () => {
    const onFinish = jest.fn();
    const spec = specWith({ day: { type: 'string', format: 'date', title: '日期' } }, [
      { key: 'day', widget: 'DatePicker' },
    ]);
    render(<SchemaFormRenderer spec={spec} onFinish={onFinish} />);

    await act(async () => {
      onChangeOf(lastProps(antdMock.DatePicker, 'root_day'))(dayjs('2026-01-31T16:45:12'));
    });
    submitForm();
    await waitFor(() => expect(onFinish).toHaveBeenCalled());
    expect(onFinish.mock.calls[0][0]).toEqual({ day: '2026-01-31' });
  });

  it('(b) format:date-time 提交带显式本地偏移的 RFC3339（墙钟保留，非 toISOString 的 UTC Z）', async () => {
    const onFinish = jest.fn();
    const spec = specWith({ at: { type: 'string', format: 'date-time', title: '时间' } });
    render(<SchemaFormRenderer spec={spec} onFinish={onFinish} />);

    await act(async () => {
      onChangeOf(lastProps(antdMock.DatePicker, 'root_at'))(dayjs('2026-01-31T08:30:45'));
    });
    submitForm();
    await waitFor(() => expect(onFinish).toHaveBeenCalled());
    const submitted = onFinish.mock.calls[0][0] as Record<string, unknown>;
    // 期望偏移由同一进程时区推导；UTC 下为 +00:00（toISOString 会给 .000Z，必不等）
    expect(submitted['at']).toBe(`2026-01-31T08:30:45${LOCAL_OFFSET}`);
  });

  it('(c) 清空 picker 从提交值移除键（undefined，不是滞留 null）', async () => {
    const onFinish = jest.fn();
    const ref = createRef<SchemaFormRendererHandle>();
    const spec = specWith({
      day: { type: 'string', format: 'date', title: '日期' },
      at: { type: 'string', format: 'date-time', title: '时间' },
      t: { type: 'string', format: 'time', title: '时刻' },
    });
    render(<SchemaFormRenderer ref={ref} spec={spec} onFinish={onFinish} />);

    await act(async () => {
      onChangeOf(lastProps(antdMock.DatePicker, 'root_day'))(dayjs('2026-01-31'));
      onChangeOf(lastProps(antdMock.DatePicker, 'root_at'))(dayjs('2026-01-31T08:30:45'));
      onChangeOf(lastProps(antdMock.TimePicker, 'root_t'))(dayjs('2026-01-31T08:37:19'));
    });
    const filled = ref.current?.getSubmitValues() ?? {};
    expect(filled).toEqual({
      day: '2026-01-31',
      at: `2026-01-31T08:30:45${LOCAL_OFFSET}`,
      t: '08:37:19',
    });

    // antd allowClear 清空传 null（内置实现透传 null 滞留 formData）
    await act(async () => {
      onChangeOf(lastProps(antdMock.DatePicker, 'root_day'))(null);
      onChangeOf(lastProps(antdMock.DatePicker, 'root_at'))(null);
      onChangeOf(lastProps(antdMock.TimePicker, 'root_t'))(null);
    });
    const cleared = ref.current?.getSubmitValues() ?? {};
    expect(cleared).not.toHaveProperty('day');
    expect(cleared).not.toHaveProperty('at');
    expect(cleared).not.toHaveProperty('t');
    submitForm();
    await waitFor(() => expect(onFinish).toHaveBeenCalled());
    expect(onFinish.mock.calls[0][0]).toEqual({});
  });

  it('(e) format:time 提交 HH:mm:ss（分秒真实，非拼 :00）', async () => {
    const onFinish = jest.fn();
    const spec = specWith({ when: { type: 'string', format: 'time', title: '时刻' } });
    render(<SchemaFormRenderer spec={spec} onFinish={onFinish} />);

    await act(async () => {
      onChangeOf(lastProps(antdMock.TimePicker, 'root_when'))(dayjs('2026-01-31T08:37:19'));
    });
    submitForm();
    await waitFor(() => expect(onFinish).toHaveBeenCalled());
    expect(onFinish.mock.calls[0][0]).toEqual({ when: '08:37:19' });
  });
});

describe('SUI-022: 种子与跨日边界（按进程时区自洽断言）', () => {
  beforeEach(resetStubs);

  it('(d) date 字段种入带 Z 的时刻：显示按 dayjs 本地日历日（跨日为显式边界）', async () => {
    const onFinish = jest.fn();
    const seed = '2026-01-01T00:00:00Z';
    // 「保持瞬时」语义：显示 = 该时刻的本地日历日（UTC 时区 2026-01-01，
    // America/New_York 2025-12-31——跨日是边界而非缺陷，见 datetime.ts 文档）
    const expectedCalendarDate = dayjs(seed).format('YYYY-MM-DD');
    const spec = specWith({ day: { type: 'string', format: 'date', title: '日期' } });
    render(<SchemaFormRenderer spec={spec} initialValues={{ day: seed }} onFinish={onFinish} />);
    expect(displayText(lastProps(antdMock.DatePicker, 'root_day'), 'YYYY-MM-DD')).toBe(
      expectedCalendarDate,
    );

    // 用户再选一次日期后提交本地日历日，不受种子时区影响
    await act(async () => {
      onChangeOf(lastProps(antdMock.DatePicker, 'root_day'))(dayjs('2026-02-02T23:59:59'));
    });
    submitForm();
    await waitFor(() => expect(onFinish).toHaveBeenCalled());
    expect(onFinish.mock.calls[0][0]).toEqual({ day: '2026-02-02' });
  });

  it('naive DB 风格种子按本地墙钟显示（不被静默平移）', () => {
    const spec = specWith({ at: { type: 'string', format: 'date-time', title: '时间' } });
    render(<SchemaFormRenderer spec={spec} initialValues={{ at: '2026-01-31T08:00:00' }} hideSubmit />);
    expect(displayText(lastProps(antdMock.DatePicker, 'root_at'), 'YYYY-MM-DDTHH:mm:ss')).toBe(
      '2026-01-31T08:00:00',
    );
  });

  it('time 字段种入 HH:mm 与 HH:mm:ss 均可回显', () => {
    const spec = specWith({ t: { type: 'string', format: 'time', title: '时刻' } });
    render(
      <SchemaFormRenderer spec={spec} initialValues={{ t: '09:05' }} hideSubmit />,
    );
    expect(displayText(lastProps(antdMock.TimePicker, 'root_t'), 'HH:mm:ss')).toBe('09:05:00');
  });

  it('三个覆盖实现经 customWidgets 注册（与直接导出同源）', () => {
    expect(customWidgets.DateWidget).toBe(DateWidget);
    expect(customWidgets.DateTimeWidget).toBe(DateTimeWidget);
    expect(customWidgets.TimeWidget).toBe(TimeWidget);
  });
});

// ---------------------------------------------------------------------------
// TZ 守卫固化断言：仅在对应时区下生效（本地双轮验证：TZ=UTC /
// TZ=America/New_York 各跑一次），CI 任意时区下自动 skip。
// ---------------------------------------------------------------------------

describeWhenTz('UTC')('TZ=UTC 固化：', () => {
  beforeEach(resetStubs);

  it('date-time 提交 +00:00 显式偏移（UTC 也不退回 Z/.000Z 形态）', async () => {
    const onFinish = jest.fn();
    const spec = specWith({ at: { type: 'string', format: 'date-time', title: '时间' } });
    render(<SchemaFormRenderer spec={spec} onFinish={onFinish} />);
    await act(async () => {
      onChangeOf(lastProps(antdMock.DatePicker, 'root_at'))(dayjs('2026-01-31T08:30:45'));
    });
    submitForm();
    await waitFor(() => expect(onFinish).toHaveBeenCalled());
    expect(onFinish.mock.calls[0][0]).toEqual({ at: '2026-01-31T08:30:45+00:00' });
  });

  it('date-time naive 种子再选提交 +00:00 墙钟保持', async () => {
    const onFinish = jest.fn();
    const spec = specWith({ at: { type: 'string', format: 'date-time', title: '时间' } });
    render(<SchemaFormRenderer spec={spec} initialValues={{ at: '2026-01-31T08:00:00' }} onFinish={onFinish} />);
    await act(async () => {
      onChangeOf(lastProps(antdMock.DatePicker, 'root_at'))(lastProps(antdMock.DatePicker, 'root_at')['value']);
    });
    submitForm();
    await waitFor(() => expect(onFinish).toHaveBeenCalled());
    expect(onFinish.mock.calls[0][0]).toEqual({ at: '2026-01-31T08:00:00+00:00' });
  });
});

describeWhenTz('America/New_York')('TZ=America/New_York 固化：', () => {
  beforeEach(resetStubs);

  it('date 字段 Z 种子跨日：显示并提交 2025-12-31（UTC 日历日前一天）', async () => {
    const onFinish = jest.fn();
    const spec = specWith({ day: { type: 'string', format: 'date', title: '日期' } });
    render(
      <SchemaFormRenderer
        spec={spec}
        initialValues={{ day: '2026-01-01T00:00:00Z' }}
        onFinish={onFinish}
      />,
    );
    expect(displayText(lastProps(antdMock.DatePicker, 'root_day'), 'YYYY-MM-DD')).toBe(
      '2025-12-31',
    );
    await act(async () => {
      onChangeOf(lastProps(antdMock.DatePicker, 'root_day'))(lastProps(antdMock.DatePicker, 'root_day')['value']);
    });
    submitForm();
    await waitFor(() => expect(onFinish).toHaveBeenCalled());
    expect(onFinish.mock.calls[0][0]).toEqual({ day: '2025-12-31' });
  });

  it('naive 种子墙钟保持：08:00 显示为 08:00，提交 -05:00 偏移', async () => {
    const onFinish = jest.fn();
    const spec = specWith({ at: { type: 'string', format: 'date-time', title: '时间' } });
    render(<SchemaFormRenderer spec={spec} initialValues={{ at: '2026-01-31T08:00:00' }} onFinish={onFinish} />);
    expect(displayText(lastProps(antdMock.DatePicker, 'root_at'), 'HH:mm')).toBe('08:00');
    await act(async () => {
      onChangeOf(lastProps(antdMock.DatePicker, 'root_at'))(lastProps(antdMock.DatePicker, 'root_at')['value']);
    });
    submitForm();
    await waitFor(() => expect(onFinish).toHaveBeenCalled());
    expect(onFinish.mock.calls[0][0]).toEqual({ at: '2026-01-31T08:00:00-05:00' });
  });
});
