/**
 * InvocationResponse 分支全覆盖测试：
 * 空态 / 成功 / 失败 / errorDetails 结构化明细 / traceId（Jaeger 链接与复制）
 * / 结构化视图（object 卡片、对象数组表格、schema 与数据形态不匹配回退 JSON）
 */
import React from 'react';
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import InvocationResponse from './InvocationResponse';
import { fetchOpsConfig, type OpsConfig } from '@/services/api/ops';
import { history } from '@umijs/max';
import type { JSONSchema, JSONValue } from '@/types/dashboard';

jest.mock('@/services/api/ops');
jest.mock('@/components/MonacoDynamic', () => ({
  CodeEditor: ({ value }: { value: string }) => <div data-testid="code-editor">{value}</div>,
}));

const mockedFetchOpsConfig = jest.mocked(fetchOpsConfig);

const schemaOf = (value: Record<string, unknown>): JSONSchema => value as unknown as JSONSchema;

const OBJECT_SCHEMA = schemaOf({
  type: 'object',
  properties: {
    banned: { type: 'boolean', title: '已封禁' },
    gold: { type: 'integer', title: '金币' },
  },
});

const ARRAY_SCHEMA = schemaOf({
  type: 'array',
  items: {
    type: 'object',
    properties: { id: { type: 'string', title: 'ID' } },
  },
});

const TRACE_ID = 'a'.repeat(20);

function findCopyButton(container: HTMLElement): HTMLElement {
  const icon = container.querySelector('.anticon-copy');
  expect(icon).toBeTruthy();
  return icon?.closest('button') as HTMLElement;
}

describe('InvocationResponse', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockedFetchOpsConfig.mockResolvedValue({});
  });

  describe('空态与基础展示', () => {
    it('无响应且无错误时渲染 Empty，且不拉取 ops 配置', () => {
      const { container } = render(
        <InvocationResponse responseRaw="" error="" duration={0} onCopy={jest.fn()} />,
      );
      expect(screen.getByText('发送请求后，响应结果将显示在这里')).toBeInTheDocument();
      expect(screen.queryByText('成功')).toBeNull();
      expect(screen.queryByText('失败')).toBeNull();
      expect(container.querySelector('.anticon-copy')).toBeNull();
      expect(mockedFetchOpsConfig).not.toHaveBeenCalled();
    });

    it('成功响应：成功 Tag + 耗时 Tag + 复制按钮回调整体响应', () => {
      const onCopy = jest.fn();
      const { container } = render(
        <InvocationResponse responseRaw='{"ok":true}' error="" duration={12} onCopy={onCopy} />,
      );
      expect(screen.getByText('成功')).toBeInTheDocument();
      expect(screen.getByText('12 ms')).toBeInTheDocument();
      expect(screen.getByTestId('code-editor').textContent).toBe('{"ok":true}');
      // 原始数据 Tab 懒挂载：切换后断言 TextArea 内容
      fireEvent.click(screen.getByRole('tab', { name: '原始数据' }));
      expect((screen.getByRole('textbox') as HTMLTextAreaElement).value).toBe('{"ok":true}');
      fireEvent.click(findCopyButton(container));
      expect(onCopy).toHaveBeenCalledWith('{"ok":true}');
    });

    it('duration 达到秒级展示 s；duration 为 0 时不渲染耗时 Tag', () => {
      const { rerender } = render(
        <InvocationResponse responseRaw="{}" error="" duration={1500} onCopy={jest.fn()} />,
      );
      expect(screen.getByText('1.50 s')).toBeInTheDocument();

      rerender(<InvocationResponse responseRaw="{}" error="" duration={0} onCopy={jest.fn()} />);
      expect(screen.queryByText('1.50 s')).toBeNull();
      expect(screen.queryByText('0 ms')).toBeNull();
    });
  });

  describe('错误分支', () => {
    it('错误优先：失败 Tag + Alert + 复制回调透传 error（responseRaw 为空）', () => {
      const onCopy = jest.fn();
      const { container } = render(
        <InvocationResponse responseRaw="" error="请求参数无效" duration={5} onCopy={onCopy} />,
      );
      expect(screen.getByText('失败')).toBeInTheDocument();
      expect(screen.getByText('调用失败')).toBeInTheDocument();
      expect(screen.getByText('请求参数无效')).toBeInTheDocument();
      fireEvent.click(findCopyButton(container));
      expect(onCopy).toHaveBeenCalledWith('请求参数无效');
    });

    it('errorDetails 有字段项渲染 field + 分隔符 + message', () => {
      render(
        <InvocationResponse
          responseRaw=""
          error="请求参数无效"
          errorDetails={[
            { field: 'playerId', message: '不能为空' },
            { field: 'amount', message: '必须大于 0' },
          ]}
          duration={5}
          onCopy={jest.fn()}
        />,
      );
      const alert = screen.getByRole('alert');
      const items = within(alert).getAllByRole('listitem');
      expect(items.map((li) => li.textContent)).toEqual([
        'playerId：不能为空',
        'amount：必须大于 0',
      ]);
    });

    it('errorDetails 项无 field 时仅渲染 message（无 code 与分隔符）', () => {
      render(
        <InvocationResponse
          responseRaw=""
          error="boom"
          errorDetails={[{ field: '', message: '仅消息' }]}
          duration={5}
          onCopy={jest.fn()}
        />,
      );
      const alert = screen.getByRole('alert');
      expect(within(alert).getByText('仅消息')).toBeInTheDocument();
      expect(within(alert).queryByText('：')).toBeNull();
    });

    it('errorDetails 为空数组时不渲染明细列表', () => {
      render(
        <InvocationResponse
          responseRaw=""
          error="boom"
          errorDetails={[]}
          duration={5}
          onCopy={jest.fn()}
        />,
      );
      const alert = screen.getByRole('alert');
      expect(within(alert).queryByRole('list')).toBeNull();
    });

    it('错误存在时即便 outputSchema 可结构化也不出结构化 Tab', () => {
      render(
        <InvocationResponse
          responseRaw="{}"
          error="调用失败"
          duration={5}
          outputSchema={OBJECT_SCHEMA}
          response={{ banned: true }}
          onCopy={jest.fn()}
        />,
      );
      expect(screen.queryByText('结构化')).toBeNull();
      // Alert 标题与错误正文均为「调用失败」
      expect(screen.getAllByText('调用失败').length).toBeGreaterThan(0);
    });
  });

  describe('traceId 分支', () => {
    it('未配置 Jaeger：Tag 点击复制 traceId，不打开新窗口', async () => {
      mockedFetchOpsConfig.mockResolvedValue(null as unknown as OpsConfig);
      const onCopy = jest.fn();
      const { container } = render(
        <InvocationResponse
          responseRaw="{}"
          error=""
          duration={10}
          traceId={TRACE_ID}
          onCopy={onCopy}
        />,
      );
      await waitFor(() => expect(mockedFetchOpsConfig).toHaveBeenCalledTimes(1));
      const tag = screen.getByText(`${TRACE_ID.slice(0, 16)}…`);
      expect(tag.closest('.ant-tag')).toHaveStyle({ cursor: 'copy' });
      fireEvent.click(tag);
      expect(onCopy).toHaveBeenCalledWith(TRACE_ID);
      expect(container).toBeTruthy();
    });

    it('配置了 Jaeger（尾斜杠归一）：Tag 点击 window.open 打开 trace 链接', async () => {
      const openSpy = jest.spyOn(window, 'open').mockImplementation(() => null);
      mockedFetchOpsConfig.mockResolvedValue({ jaegerUrl: 'http://jaeger.io//' });
      render(
        <InvocationResponse
          responseRaw="{}"
          error=""
          duration={10}
          traceId={TRACE_ID}
          onCopy={jest.fn()}
        />,
      );
      const tag = await screen.findByText(`${TRACE_ID.slice(0, 16)}…`);
      await waitFor(() => expect(tag.closest('.ant-tag')).toHaveStyle({ cursor: 'pointer' }));
      fireEvent.click(tag);
      expect(openSpy).toHaveBeenCalledWith(
        `http://jaeger.io/trace/${encodeURIComponent(TRACE_ID)}`,
        '_blank',
        'noopener,noreferrer',
      );
      openSpy.mockRestore();
    });

    it('配置了 Jaeger（无尾斜杠）：直接拼接 trace 路径', async () => {
      const openSpy = jest.spyOn(window, 'open').mockImplementation(() => null);
      mockedFetchOpsConfig.mockResolvedValue({ jaegerUrl: 'http://j.io' });
      render(
        <InvocationResponse
          responseRaw="{}"
          error=""
          duration={10}
          traceId={TRACE_ID}
          onCopy={jest.fn()}
        />,
      );
      await waitFor(() => expect(mockedFetchOpsConfig).toHaveBeenCalledTimes(1));
      fireEvent.click(await screen.findByText(`${TRACE_ID.slice(0, 16)}…`));
      expect(openSpy).toHaveBeenCalledWith(
        `http://j.io/trace/${encodeURIComponent(TRACE_ID)}`,
        '_blank',
        'noopener,noreferrer',
      );
      openSpy.mockRestore();
    });

    it('fetchOpsConfig 失败静默降级（无 Jaeger 行为）', async () => {
      mockedFetchOpsConfig.mockRejectedValue(new Error('network'));
      const onCopy = jest.fn();
      render(
        <InvocationResponse
          responseRaw="{}"
          error=""
          duration={10}
          traceId={TRACE_ID}
          onCopy={onCopy}
        />,
      );
      const tag = await screen.findByText(`${TRACE_ID.slice(0, 16)}…`);
      await waitFor(() => expect(mockedFetchOpsConfig).toHaveBeenCalledTimes(1));
      fireEvent.click(tag);
      expect(onCopy).toHaveBeenCalledWith(TRACE_ID);
    });

    it('traceId 存在时渲染底部链路区块，按钮跳转链路追踪页', async () => {
      render(
        <InvocationResponse
          responseRaw="{}"
          error=""
          duration={10}
          traceId={TRACE_ID}
          onCopy={jest.fn()}
        />,
      );
      await waitFor(() => expect(mockedFetchOpsConfig).toHaveBeenCalledTimes(1));
      expect(screen.getByText('链路追踪：')).toBeInTheDocument();
      expect(screen.getByText(TRACE_ID)).toBeInTheDocument();
      fireEvent.click(screen.getByRole('button', { name: /链路追踪页/ }));
      expect(history.push).toHaveBeenCalledWith(
        `/dev/traces?traceId=${encodeURIComponent(TRACE_ID)}`,
      );
    });

    it('无 traceId 时不渲染底部链路区块', () => {
      render(<InvocationResponse responseRaw="{}" error="" duration={10} onCopy={jest.fn()} />);
      expect(screen.queryByText('链路追踪：')).toBeNull();
    });
  });

  describe('结构化视图分支', () => {
    it('object schema + object 响应 → 结构化 Tab（Descriptions 字段卡片）', () => {
      render(
        <InvocationResponse
          responseRaw='{"banned":true}'
          error=""
          duration={12}
          outputSchema={OBJECT_SCHEMA}
          response={{ banned: true, gold: 88 }}
          onCopy={jest.fn()}
        />,
      );
      expect(screen.getByText('结构化')).toBeInTheDocument();
      expect(screen.getByText('已封禁')).toBeInTheDocument();
      expect(screen.getByText('金币')).toBeInTheDocument();
      expect(screen.getByText('88')).toBeInTheDocument();
    });

    it('array schema + 对象数组响应 → 结构化 Tab（表格）', () => {
      render(
        <InvocationResponse
          responseRaw='[{"id":"p1"}]'
          error=""
          duration={12}
          outputSchema={ARRAY_SCHEMA}
          response={[{ id: 'p1' }, { id: 'p2' }] as JSONValue}
          onCopy={jest.fn()}
        />,
      );
      expect(screen.getByText('结构化')).toBeInTheDocument();
      expect(screen.getByRole('table')).toBeInTheDocument();
      expect(screen.getByText('p1')).toBeInTheDocument();
      expect(screen.getByText('p2')).toBeInTheDocument();
    });

    it('object schema + 数组响应（形态不匹配）→ 回退 JSON 两档', () => {
      render(
        <InvocationResponse
          responseRaw="[]"
          error=""
          duration={12}
          outputSchema={OBJECT_SCHEMA}
          response={[{ id: 'p1' }] as JSONValue}
          onCopy={jest.fn()}
        />,
      );
      expect(screen.queryByText('结构化')).toBeNull();
      expect(screen.getByText('格式化')).toBeInTheDocument();
      expect(screen.getByText('原始数据')).toBeInTheDocument();
    });

    it('array schema + 对象响应（形态不匹配）→ 回退 JSON', () => {
      render(
        <InvocationResponse
          responseRaw="{}"
          error=""
          duration={12}
          outputSchema={ARRAY_SCHEMA}
          response={{ id: 'p1' }}
          onCopy={jest.fn()}
        />,
      );
      expect(screen.queryByText('结构化')).toBeNull();
      expect(screen.getByText('格式化')).toBeInTheDocument();
    });

    it('array schema + 空数组响应 → 回退 JSON', () => {
      render(
        <InvocationResponse
          responseRaw="[]"
          error=""
          duration={12}
          outputSchema={ARRAY_SCHEMA}
          response={[] as JSONValue}
          onCopy={jest.fn()}
        />,
      );
      expect(screen.queryByText('结构化')).toBeNull();
    });

    it('object schema + 标量字符串响应（形态不匹配）→ 回退 JSON', () => {
      render(
        <InvocationResponse
          responseRaw='"text"'
          error=""
          duration={12}
          outputSchema={OBJECT_SCHEMA}
          response="text"
          onCopy={jest.fn()}
        />,
      );
      expect(screen.queryByText('结构化')).toBeNull();
    });

    it('schema 可结构化但响应缺失（undefined）→ 无结构化 Tab', () => {
      render(
        <InvocationResponse
          responseRaw='{"banned":true}'
          error=""
          duration={12}
          outputSchema={OBJECT_SCHEMA}
          onCopy={jest.fn()}
        />,
      );
      expect(screen.queryByText('结构化')).toBeNull();
      expect(screen.getByText('格式化')).toBeInTheDocument();
    });

    it('schema 可结构化但响应为 null → 无结构化 Tab', () => {
      render(
        <InvocationResponse
          responseRaw="null"
          error=""
          duration={12}
          outputSchema={OBJECT_SCHEMA}
          response={null}
          onCopy={jest.fn()}
        />,
      );
      expect(screen.queryByText('结构化')).toBeNull();
    });

    it('schema 不可结构化（标量）→ 仅 JSON 两档', () => {
      render(
        <InvocationResponse
          responseRaw='"text"'
          error=""
          duration={12}
          outputSchema={schemaOf({ type: 'string' })}
          response="text"
          onCopy={jest.fn()}
        />,
      );
      expect(screen.queryByText('结构化')).toBeNull();
      expect(screen.getByText('格式化')).toBeInTheDocument();
      expect(screen.getByText('原始数据')).toBeInTheDocument();
    });
  });
});
