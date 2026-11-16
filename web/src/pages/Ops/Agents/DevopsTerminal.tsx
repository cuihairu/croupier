import React, { useEffect, useRef, useState } from 'react';
import { FitAddon } from '@xterm/addon-fit';
import { Terminal } from '@xterm/xterm';
import '@xterm/xterm/css/xterm.css';
import {
  openDevopsSessionStream,
  resizeDevopsSession,
  sendDevopsSessionInput,
  type DevopsSessionSSEPayload,
} from '@/services/api/devopsSessions';
import { message } from 'antd';
import type { SSEStream } from '@/services/core/http';

/** 终端连接状态（面向面板展示：抽屉头部与状态条）。 */
export type DevopsTerminalStatus = 'connecting' | 'connected' | 'reconnecting' | 'ended' | 'error';

export type DevopsTerminalProps = {
  sessionId: string;
  /** 初始窗口：容器 fit 之后按真实尺寸 resize 服务端。 */
  cols: number;
  rows: number;
  onStatus?: (status: DevopsTerminalStatus, outcome?: string) => void;
};

const INPUT_BUSY_WINDOW_MS = 1500;

/** 请求错误状态码（umi request 的 error.response.status 形状）。 */
function errorCode(err: unknown): number | undefined {
  if (typeof err !== 'object' || err === null) return undefined;
  const status = (err as { response?: { status?: unknown } }).response?.status;
  return typeof status === 'number' ? status : undefined;
}

function decodeFrame(data: string): Uint8Array {
  const binary = atob(data);
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i += 1) bytes[i] = binary.charCodeAt(i);
  return bytes;
}

/** 会话终端：xterm 渲染 pty 字节流，键盘经 input 端点下行，窗口变化经
 * resize 端点同步。SSE 断开（会话收束/被摘除）后写结束行并停止下行，
 * 不自动关会话——关闭是显式动作（留痕 devops.session_close）。 */
export default function DevopsTerminal({ sessionId, cols, rows, onStatus }: DevopsTerminalProps) {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const termRef = useRef<Terminal | null>(null);
  const streamRef = useRef<SSEStream | null>(null);
  const endedRef = useRef(false);
  const busyWarnedAtRef = useRef(0);
  const onStatusRef = useRef(onStatus);
  const [ended, setEnded] = useState<string | null>(null);

  useEffect(() => {
    onStatusRef.current = onStatus;
  });

  useEffect(() => {
    const container = containerRef.current;
    if (!container) return undefined;

    const term = new Terminal({
      cols,
      rows,
      cursorBlink: true,
      scrollback: 4000,
      // 面板内字体继承页面字体族，避免依赖等宽字体安装（容器/浏览器自带 monospace）。
      fontFamily: 'monospace',
      fontSize: 13,
    });
    const fit = new FitAddon();
    term.loadAddon(fit);
    term.open(container);
    termRef.current = term;
    try {
      fit.fit();
    } catch {
      // 抽屉首帧容器高度可能为 0：ResizeObserver 首帧会补 fit。
    }
    endedRef.current = false;

    const report = (status: DevopsTerminalStatus, outcome?: string) => {
      onStatusRef.current?.(status, outcome);
    };
    report('connecting');

    const stream = openDevopsSessionStream(sessionId);
    streamRef.current = stream;
    stream.addEventListener('output', (event) => {
      // 中继/网络注入的畸形帧不能炸掉整个终端（帧解析失败只丢该帧）。
      let payload: DevopsSessionSSEPayload;
      try {
        payload = JSON.parse(event.data || '{}') as DevopsSessionSSEPayload;
      } catch (err) {
        console.error('[devops-terminal] malformed output frame dropped', err);
        return;
      }
      if (!payload?.data) return;
      try {
        term.write(decodeFrame(payload.data));
      } catch (err) {
        console.error('[devops-terminal] failed to write output frame', err);
      }
    });
    stream.addEventListener('end', (event) => {
      let outcome = '';
      try {
        const payload = JSON.parse(event.data || '{}') as DevopsSessionSSEPayload;
        outcome = payload?.outcome || '';
      } catch (err) {
        // end 帧畸形也必须收束会话（outcome 缺省），不能把会话卡在打开态。
        console.error('[devops-terminal] malformed end frame', err);
      }
      endedRef.current = true;
      setEnded(outcome || 'ended');
      term.write(`\r\n\x1b[33m[会话结束${outcome ? `：${outcome}` : ''}]\x1b[0m\r\n`);
      report('ended', outcome || undefined);
      stream.close();
    });
    stream.addEventListener('error', () => {
      if (!endedRef.current) report('reconnecting');
    });
    stream.onopen = () => {
      if (!endedRef.current) report('connected');
    };

    term.onData((data) => {
      if (endedRef.current) return;
      void sendDevopsSessionInput(sessionId, data).catch((err) => {
        if (errorCode(err) === 429) {
          const now = Date.now();
          if (now - busyWarnedAtRef.current > INPUT_BUSY_WINDOW_MS) {
            busyWarnedAtRef.current = now;
            message.warning('输入通道繁忙（session_busy），本段输入已丢弃，请稍后重试');
          }
          return;
        }
        console.error('[devops-terminal] input failed', err);
      });
    });
    term.onResize(({ cols: nextCols, rows: nextRows }) => {
      if (endedRef.current) return;
      void resizeDevopsSession(sessionId, nextCols, nextRows).catch(() => {
        /* 窗口调整 best-effort：失败只丢这一次同步，不打断会话 */
      });
    });

    const observer = new ResizeObserver(() => {
      try {
        fit.fit();
      } catch {
        /* 容器尺寸为 0 时 fit 抛错：等下一次尺寸变化 */
      }
    });
    observer.observe(container);

    return () => {
      observer.disconnect();
      stream.close();
      streamRef.current = null;
      term.dispose();
      termRef.current = null;
    };
    // 会话绑定一次：sessionId/cols/rows 变化即重建（抽屉按会话 key 重挂）。
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sessionId]);

  return (
    <div
      ref={containerRef}
      data-testid="devops-terminal"
      style={{
        height: '100%',
        minHeight: 360,
        background: '#000',
        padding: 4,
        overflow: 'hidden',
      }}
    >
      {ended ? (
        <div
          style={{
            position: 'absolute',
            top: 8,
            right: 12,
            color: '#d9d9d9',
            background: '#222',
            padding: '2px 8px',
            borderRadius: 4,
          }}
        >
          会话已结束
        </div>
      ) : null}
    </div>
  );
}
