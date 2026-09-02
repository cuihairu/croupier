/**
 * F11：异步任务进度内嵌面板。
 *
 * async 调用拿到 taskId 后轮询任务详情（/api/v1/tasks/:id，含最终 result），
 * 展示状态机 + 错误；终态前可取消。结果通过 onCompleted 交回工作台渲染
 * （对接 F10 结构化结果视图）。
 */

import React, { useEffect, useRef, useState } from 'react';
import { Alert, Button, Space, Tag, Typography } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';
import { cancelTask, fetchTaskResult } from '@/services/api/functions';
import type { JSONValue } from '@/types/dashboard';

const POLL_INTERVAL_MS = 2000;

const TERMINAL_STATUSES = new Set(['succeeded', 'failed', 'cancelled', 'timed_out']);

const STATUS_META: Record<string, { label: string; color: string }> = {
  queued: { label: '排队中', color: 'default' },
  dispatching: { label: '派发中', color: 'processing' },
  running: { label: '执行中', color: 'processing' },
  succeeded: { label: '已完成', color: 'success' },
  failed: { label: '失败', color: 'error' },
  cancel_requested: { label: '取消中', color: 'warning' },
  cancelled: { label: '已取消', color: 'default' },
  timed_out: { label: '已超时', color: 'error' },
};

export interface TaskProgressPanelProps {
  taskId: string;
  /** 任务到达 succeeded 时回调（携带最终 result），供工作台渲染结果 */
  onCompleted?: (result: JSONValue | undefined) => void;
}

const { Text } = Typography;

export default function TaskProgressPanel({ taskId, onCompleted }: TaskProgressPanelProps) {
  const [status, setStatus] = useState<string>('');
  const [error, setError] = useState<string>('');
  const [cancelling, setCancelling] = useState(false);
  const [refreshTick, setRefreshTick] = useState(0);
  const completedRef = useRef(false);
  const onCompletedRef = useRef(onCompleted);
  onCompletedRef.current = onCompleted;

  useEffect(() => {
    completedRef.current = false;
    setStatus('');
    setError('');
    setRefreshTick(0);
  }, [taskId]);

  useEffect(() => {
    if (!taskId) return;
    let cancelled = false;
    let timer: ReturnType<typeof setTimeout> | null = null;

    const poll = async () => {
      try {
        const { state, payload, error: taskError } = await fetchTaskResult(taskId);
        if (cancelled) return;
        const next = typeof state === 'string' ? state : '';
        setStatus(next);
        setError(typeof taskError === 'string' ? taskError : '');
        if (next === 'succeeded' && !completedRef.current) {
          completedRef.current = true;
          onCompletedRef.current?.(payload);
          return;
        }
        if (TERMINAL_STATUSES.has(next)) return;
      } catch {
        // 单次轮询失败不打断（网络抖动/权限），继续下一轮
      }
      if (!cancelled) {
        timer = setTimeout(poll, POLL_INTERVAL_MS);
      }
    };
    poll();

    return () => {
      cancelled = true;
      if (timer) clearTimeout(timer);
    };
  }, [taskId, refreshTick]);

  const terminal = TERMINAL_STATUSES.has(status);
  const meta = STATUS_META[status] ?? { label: status || '查询中', color: 'default' };

  const handleCancel = async () => {
    setCancelling(true);
    try {
      await cancelTask(taskId);
    } catch {
      // 取消失败保持面板状态，用户可重试或等待终态
    } finally {
      setCancelling(false);
    }
  };

  return (
    <div data-testid="task-progress-panel">
      <Space wrap>
        <Text strong>任务</Text>
        <Text code copyable={{ text: taskId }}>
          {taskId}
        </Text>
        <Tag color={meta.color} data-testid="task-status">
          {meta.label}
        </Tag>
        {!terminal ? (
          <>
            <Button
              size="small"
              icon={<ReloadOutlined />}
              onClick={() => setRefreshTick((tick) => tick + 1)}
            >
              刷新
            </Button>
            <Button
              size="small"
              danger
              loading={cancelling}
              disabled={status === 'cancel_requested'}
              onClick={handleCancel}
              data-testid="task-cancel"
            >
              取消任务
            </Button>
          </>
        ) : null}
      </Space>
      {status === 'failed' && error ? (
        <Alert
          type="error"
          showIcon
          message="任务执行失败"
          description={error}
          style={{ marginTop: 8 }}
        />
      ) : null}
      {status === 'timed_out' ? (
        <Alert type="warning" showIcon message="任务已超时" style={{ marginTop: 8 }} />
      ) : null}
    </div>
  );
}
