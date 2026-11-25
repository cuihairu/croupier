/**
 * 在线调试弹窗单测（SUI-018 调试页验收）：
 * - 初始请求参数 = 默认值层（只含 schema 声明 default；示例与可选占位不入初值，
 *   可选未填写即保持缺省）；
 * - 「填入示例」显式应用模板层（default > example > enum 首项 > 类型占位），
 *   覆盖当前参数——示例只有用户点按钮才出现；
 * - schema 拉取失败或无 schema：保留空模板、按钮禁用，不阻断手写调试。
 */
import React from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App as AntdApp } from 'antd';
import DebugModal from '../DebugModal';
import { getFunctionDetail } from '@/services/api';
import type { FunctionDescriptor, FunctionInstance } from '@/services/api';

jest.setTimeout(30000);

jest.mock('@umijs/max', () => {
  const makeIntl = () => ({
    locale: 'zh-CN',
    formatMessage: (opts: { defaultMessage?: string }, values?: Record<string, unknown>) => {
      let text = opts.defaultMessage ?? '';
      if (values) {
        for (const [k, v] of Object.entries(values)) text = text.split(`{${k}}`).join(String(v));
      }
      return text;
    },
  });
  return {
    FormattedMessage: ({ defaultMessage }: { defaultMessage?: string }) => (
      <>{defaultMessage ?? ''}</>
    ),
    useIntl: makeIntl,
    getIntl: makeIntl,
  };
});

jest.mock('@/services/api', () => ({
  __esModule: true,
  getFunctionDetail: jest.fn(),
  invokeFunction: jest.fn(),
}));

// JSON 编辑器桩：受控 textarea，暴露当前请求参数文本
jest.mock('@/components/JsonCodeEditor', () => {
  const ReactActual = jest.requireActual<typeof import('react')>('react');
  return {
    __esModule: true,
    default: (props: { value: string; onChange: (value: string) => void }) =>
      ReactActual.createElement('textarea', {
        'data-testid': 'debug-json',
        value: props.value,
        onChange: (event: { target: { value: string } }) => props.onChange(event.target.value),
      }),
  };
});

const mockedGetFunctionDetail = getFunctionDetail as jest.Mock;

/** 调试弹窗请求参数编辑器当前文本（JSON 原文）。jest-dom 的 toHaveValue
 *  正则形态对 textarea 不适用，统一走 value + toMatch 断言。 */
const payloadValue = (): string => screen.getByTestId('debug-json').value;

const instance = {
  functionId: 'demo.grant',
  serviceId: 'svc-1',
  agentId: 'agent-1',
  gameId: 'demo',
  env: 'dev',
} as FunctionInstance;

const renderModal = (props?: { open?: boolean; instance?: FunctionInstance | null }) =>
  render(
    <AntdApp>
      <DebugModal
        open={props?.open ?? true}
        instance={props?.instance ?? instance}
        onClose={() => undefined}
      />
    </AntdApp>,
  );

describe('在线调试弹窗：请求参数初值规则', () => {
  beforeEach(() => {
    mockedGetFunctionDetail.mockReset();
  });

  it('初始参数只含声明 default；示例与可选占位不入初值', async () => {
    mockedGetFunctionDetail.mockResolvedValue({
      inputSchema: JSON.stringify({
        type: 'object',
        properties: {
          declared: { type: 'string', default: 'D' },
          withExample: { type: 'string', example: 'EX' },
          optNum: { type: 'integer' },
          flag: { type: 'boolean' },
        },
      }),
    } as FunctionDescriptor);
    renderModal();

    await waitFor(() => expect(payloadValue()).toMatch(/"declared": "D"/));
    // 示例（example）与类型占位不入默认值层
    expect(payloadValue()).not.toMatch(/EX/);
    expect(payloadValue()).not.toMatch(/optNum/);
    expect(payloadValue()).not.toMatch(/flag/);
  });

  it('填入示例：模板层（example + 类型占位）显式应用到请求参数', async () => {
    mockedGetFunctionDetail.mockResolvedValue({
      inputSchema: JSON.stringify({
        type: 'object',
        properties: {
          declared: { type: 'string', default: 'D' },
          withExample: { type: 'string', example: 'EX' },
          optNum: { type: 'integer' },
          flag: { type: 'boolean' },
        },
      }),
    } as FunctionDescriptor);
    renderModal();
    await waitFor(() => expect(payloadValue()).toMatch(/"declared": "D"/));

    fireEvent.click(screen.getByRole('button', { name: '填入示例' }));
    await waitFor(() => expect(payloadValue()).toMatch(/"withExample": "EX"/));
    expect(payloadValue()).toMatch(/"optNum": 0/);
    expect(payloadValue()).toMatch(/"flag": false/);
  });

  it('无 schema：保留空模板，填入示例按钮禁用', async () => {
    mockedGetFunctionDetail.mockResolvedValue({} as FunctionDescriptor);
    renderModal();
    expect(screen.getByTestId('debug-json').value).toBe('{\n  \n}');
    expect(screen.getByRole('button', { name: '填入示例' })).toBeDisabled();
  });

  it('用户手写参数不被初值重算覆盖', async () => {
    mockedGetFunctionDetail.mockResolvedValue({
      inputSchema: JSON.stringify({
        type: 'object',
        properties: { declared: { type: 'string', default: 'D' } },
      }),
    } as FunctionDescriptor);
    renderModal();
    await waitFor(() => expect(payloadValue()).toMatch(/"declared": "D"/));
    fireEvent.change(screen.getByTestId('debug-json'), {
      target: { value: '{\n  "declared": false\n}' },
    });
    await waitFor(() => expect(payloadValue()).toMatch(/"declared": false/));
  });
});
