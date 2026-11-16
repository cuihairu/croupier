import React from 'react';
import { render } from '@testing-library/react';
import { Terminal } from '@xterm/xterm';
import DevopsTerminal from '../DevopsTerminal';
import { openDevopsSessionStream, sendDevopsSessionInput } from '@/services/api/devopsSessions';
import type { SSEStream } from '@/services/core/http';

// xterm mock + antd 链路在本机高负载下可超 jest 默认 5s。
jest.setTimeout(30000);

jest.mock('@xterm/xterm', () => ({
  Terminal: jest.fn().mockImplementation(() => ({
    loadAddon: jest.fn(),
    open: jest.fn(),
    write: jest.fn(),
    onData: jest.fn(),
    onResize: jest.fn(),
    dispose: jest.fn(),
  })),
}));
jest.mock('@xterm/addon-fit', () => ({
  FitAddon: jest.fn().mockImplementation(() => ({ fit: jest.fn(), dispose: jest.fn() })),
}));
jest.mock('@/services/api/devopsSessions', () => ({
  openDevopsSessionStream: jest.fn(),
  sendDevopsSessionInput: jest.fn(),
  resizeDevopsSession: jest.fn(),
}));

const { Terminal: MockTerminal } = jest.requireMock('@xterm/xterm') as { Terminal: jest.Mock };
const { FitAddon: MockFitAddon } = jest.requireMock('@xterm/addon-fit') as { FitAddon: jest.Mock };
const mockedOpenStream = openDevopsSessionStream as jest.MockedFunction<
  typeof openDevopsSessionStream
>;
const mockedInput = sendDevopsSessionInput as jest.MockedFunction<typeof sendDevopsSessionInput>;

/** FetchSSEStream 消费面最小替身：记录监听器，测试手工派发帧。 */
function makeFakeStream(): SSEStream & {
  listeners: Map<string, Array<(event: MessageEvent) => void>>;
} {
  const listeners = new Map<string, Array<(event: MessageEvent) => void>>();
  return {
    listeners,
    addEventListener(type, listener) {
      const arr = listeners.get(type) || [];
      arr.push(listener);
      listeners.set(type, arr);
    },
    removeEventListener(type, listener) {
      listeners.set(
        type,
        (listeners.get(type) || []).filter((l) => l !== listener),
      );
    },
    close: jest.fn(),
    onopen: null,
    onerror: null,
  };
}

// jsdom 没有 ResizeObserver：终端容器首帧 fit 依赖它。
class ResizeObserverStub {
  observe(): void {}
  unobserve(): void {}
  disconnect(): void {}
}
(globalThis as Record<string, unknown>).ResizeObserver = ResizeObserverStub;

const b64 = (text: string) => Buffer.from(text, 'binary').toString('base64');

describe('DevopsTerminal', () => {
  beforeEach(() => {
    MockTerminal.mockClear();
    MockFitAddon.mockClear();
    mockedOpenStream.mockReset();
    mockedInput.mockReset().mockResolvedValue(undefined);
  });

  it('opens the terminal, attaches the stream, and writes decoded output frames', () => {
    const stream = makeFakeStream();
    mockedOpenStream.mockReturnValue(stream);

    const { getByTestId } = render(<DevopsTerminal sessionId="s1" cols={120} rows={32} />);
    expect(getByTestId('devops-terminal')).toBeInTheDocument();
    expect(mockedOpenStream).toHaveBeenCalledWith('s1');
    expect(stream.listeners.get('output')).toHaveLength(1);
    expect(stream.listeners.get('end')).toHaveLength(1);

    const term = MockTerminal.mock.results[0].value;
    const outputListener = stream.listeners.get('output')![0];
    outputListener({
      data: JSON.stringify({ kind: 'output', data: b64('login:') }),
    } as MessageEvent);
    expect(term.write).toHaveBeenCalledTimes(1);
    const written = term.write.mock.calls[0][0] as Uint8Array;
    expect(Buffer.from(written).toString('binary')).toBe('login:');
  });

  it('forwards keystrokes to the input endpoint and stops after the end frame', () => {
    const stream = makeFakeStream();
    mockedOpenStream.mockReturnValue(stream);

    const onStatus = jest.fn();
    render(<DevopsTerminal sessionId="s1" cols={120} rows={32} onStatus={onStatus} />);
    const term = MockTerminal.mock.results[0].value;

    const dataHandler = (term.onData as jest.Mock).mock.calls[0][0] as (data: string) => void;
    dataHandler('ls\r');
    expect(mockedInput).toHaveBeenCalledWith('s1', 'ls\r');

    const endListener = stream.listeners.get('end')![0];
    endListener({
      data: JSON.stringify({ kind: 'end', outcome: 'agent closed' }),
    } as MessageEvent);
    expect(stream.close).toHaveBeenCalled();
    expect(onStatus).toHaveBeenCalledWith('ended', 'agent closed');

    dataHandler('rm -rf\r');
    expect(mockedInput).toHaveBeenCalledTimes(1);
  });

  // 中继/网络注入的畸形帧不能炸掉终端：output 帧丢弃、end 帧仍收束会话。
  it('survives malformed SSE frames without throwing', () => {
    const stream = makeFakeStream();
    mockedOpenStream.mockReturnValue(stream);

    const onStatus = jest.fn();
    render(<DevopsTerminal sessionId="s1" cols={120} rows={32} onStatus={onStatus} />);
    const term = MockTerminal.mock.results[0].value;
    const consoleError = jest.spyOn(console, 'error').mockImplementation(() => {});

    const outputListener = stream.listeners.get('output')![0];
    expect(() => outputListener({ data: '{not json' } as MessageEvent)).not.toThrow();
    expect(() => outputListener({ data: '' } as MessageEvent)).not.toThrow();
    expect(term.write).not.toHaveBeenCalled();

    const endListener = stream.listeners.get('end')![0];
    expect(() => endListener({ data: '{broken' } as MessageEvent)).not.toThrow();
    // end 帧畸形也必须收束：流关闭 + ended 状态（outcome 缺省）。
    expect(stream.close).toHaveBeenCalled();
    expect(onStatus).toHaveBeenCalledWith('ended', undefined);

    consoleError.mockRestore();
  });
});
