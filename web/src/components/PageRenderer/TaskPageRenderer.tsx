/**
 * TaskPageRenderer - 任务页面渲染器
 *
 * 渲染异步任务页面，包括：
 * - 表单输入
 * - 任务进度展示
 * - 事件时间线
 * - 结果展示
 *
 * @module components/PageRenderer/TaskPageRenderer
 */

import React, { useState, useCallback, useEffect, useRef } from 'react';
import { Card, Button, Space, message, Typography, Timeline, Progress, Tag, Alert } from 'antd';
import {
  CheckCircleOutlined,
  CloseCircleOutlined,
  SyncOutlined,
  ClockCircleOutlined,
  StopOutlined,
} from '@ant-design/icons';
import SchemaFormRenderer from '@/components/SchemaFormRenderer';
import ResultViewRenderer, { renderJSONValueSummary } from './ResultViewRenderer';
import { outputPatchFromResult } from './runtime';
import type {
  JSONValue,
  TaskPageSpec,
  PageFunctionBinding,
  PageExecuteFn,
  TaskStatusResult,
  TaskEvent,
  ApprovalStatusResult,
  FormValues,
} from '@/types/dashboard';

const { Text } = Typography;

// ---------------------------------------------------------------------------
// Props
// ---------------------------------------------------------------------------

export interface TaskPageRendererProps {
  /** 任务页面规格 */
  spec: TaskPageSpec;
  /** 页面绑定 */
  bindings: PageFunctionBinding[];
  /** 执行绑定函数 */
  onExecute: PageExecuteFn;
  /** 预览模式只展示页面结构，禁止触发真实函数执行 */
  preview?: boolean;
  /** 查询任务状态 */
  onQueryStatus?: (taskId: string) => Promise<TaskStatusResult>;
  /** 取消任务 */
  onCancelTask?: (taskId: string) => Promise<void>;
  /** 查询审批状态 */
  onQueryApprovalStatus?: (approvalId: string) => Promise<ApprovalStatusResult>;
  /** 页面标题 */
  title?: string;
}

// ---------------------------------------------------------------------------
// 状态颜色
// ---------------------------------------------------------------------------

function getStatusColor(status: string): string {
  switch (status) {
    case 'completed':
      return 'success';
    case 'failed':
      return 'error';
    case 'running':
      return 'processing';
    case 'cancelled':
      return 'warning';
    default:
      return 'default';
  }
}

function getStatusIcon(status: string): React.ReactNode {
  switch (status) {
    case 'completed':
      return <CheckCircleOutlined />;
    case 'failed':
      return <CloseCircleOutlined />;
    case 'running':
      return <SyncOutlined spin />;
    case 'cancelled':
      return <StopOutlined />;
    default:
      return <ClockCircleOutlined />;
  }
}

function normalizeTaskStatus(status?: string): TaskStatusResult['status'] {
  switch (status) {
    case 'queued':
    case 'dispatching':
      return 'pending';
    case 'running':
      return 'running';
    case 'succeeded':
    case 'completed':
      return 'completed';
    case 'failed':
    case 'timed_out':
      return 'failed';
    case 'cancel_requested':
    case 'cancelled':
      return 'cancelled';
    default:
      return 'pending';
  }
}

function isJsonRecord(value: JSONValue | undefined): value is Record<string, JSONValue> {
  return !!value && typeof value === 'object' && !Array.isArray(value);
}

function readString(value: JSONValue | undefined): string | undefined {
  return typeof value === 'string' ? value : undefined;
}

function readNumber(value: JSONValue | undefined): number | undefined {
  return typeof value === 'number' && Number.isFinite(value) ? value : undefined;
}

function selectByJsonPointer(value: JSONValue | undefined, pointer: string): JSONValue | undefined {
  if (value === undefined || value === null) {
    return undefined;
  }
  if (!pointer) {
    return value;
  }
  if (!pointer.startsWith('/')) {
    return undefined;
  }
  const tokens = pointer
    .slice(1)
    .split('/')
    .map((token) => token.replace(/~1/g, '/').replace(/~0/g, '~'));
  let current: JSONValue = value;
  for (const token of tokens) {
    if (Array.isArray(current)) {
      const index = Number(token);
      if (!Number.isInteger(index) || index < 0 || index >= current.length) {
        return undefined;
      }
      current = current[index];
      continue;
    }
    if (isJsonRecord(current) && Object.prototype.hasOwnProperty.call(current, token)) {
      current = current[token];
      continue;
    }
    return undefined;
  }
  return current;
}

function normalizeTaskEventType(type?: string): TaskEvent['type'] {
  switch (type) {
    case 'failed':
    case 'error':
      return 'error';
    case 'cancel_requested':
    case 'cancelled':
    case 'warning':
      return 'warning';
    case 'progress':
      return 'progress';
    default:
      return 'info';
  }
}

function extractTaskEvents(value: JSONValue | undefined): TaskEvent[] {
  if (value === undefined || value === null) {
    return [];
  }
  const rows = Array.isArray(value)
    ? value
    : isJsonRecord(value) && Array.isArray(value.events)
      ? value.events
      : isJsonRecord(value) && Array.isArray(value.items)
        ? value.items
        : [];
  return rows.flatMap((item) => {
    if (!isJsonRecord(item)) {
      return [];
    }
    const timestamp =
      readString(item.timestamp) || readString(item.createdAt) || readString(item.createdAt);
    const message = readString(item.message) || '';
    if (!timestamp || !message) {
      return [];
    }
    return [
      {
        timestamp,
        type: normalizeTaskEventType(readString(item.type)),
        message,
        data: item.data ?? item.payload,
      },
    ];
  });
}

function extractTaskResult(value: JSONValue | undefined): JSONValue | undefined {
  if (value === undefined || value === null) {
    return undefined;
  }
  if (isJsonRecord(value) && Object.prototype.hasOwnProperty.call(value, 'result')) {
    return value.result;
  }
  return value;
}

function normalizeTaskStatusFromExecution(
  taskId: string,
  executionData: JSONValue | undefined,
  statusStatePath: string,
  previous?: TaskStatusResult | null,
): TaskStatusResult {
  const previousEvents = previous?.events || [];
  if (isJsonRecord(executionData)) {
    const nextEvents = extractTaskEvents(executionData.events);
    const stateValue = selectByJsonPointer(executionData, statusStatePath);
    return {
      taskId,
      status: normalizeTaskStatus(readString(stateValue)),
      progress: readNumber(executionData.progress) ?? previous?.progress,
      message: readString(executionData.message) ?? previous?.message,
      result: extractTaskResult(executionData.result) ?? previous?.result,
      error: readString(executionData.error) ?? previous?.error,
      events: nextEvents.length ? nextEvents : previousEvents,
    };
  }
  if (typeof executionData === 'string') {
    return {
      taskId,
      status: normalizeTaskStatus(executionData),
      progress: previous?.progress,
      message: previous?.message,
      result: previous?.result,
      error: previous?.error,
      events: previousEvents,
    };
  }
  return previous ? { ...previous, taskId } : { taskId, status: 'pending' };
}

function resolveTaskResultFromExecution(
  executionData: JSONValue | undefined,
  previous?: JSONValue,
): JSONValue | undefined {
  const result = extractTaskResult(executionData);
  return result !== undefined ? result : previous;
}

function pageStateForTask(taskId: string, key: string): Record<string, JSONValue> {
  return { [key || 'taskId']: taskId };
}

function patchValue(
  binding: PageFunctionBinding,
  result: Awaited<ReturnType<PageExecuteFn>>,
  stateKey: string,
): JSONValue | undefined {
  const patch = outputPatchFromResult(binding, result);
  return Object.prototype.hasOwnProperty.call(patch, stateKey) ? patch[stateKey] : undefined;
}

// ---------------------------------------------------------------------------
// TaskPageRenderer 组件
// ---------------------------------------------------------------------------

const TaskPageRenderer: React.FC<TaskPageRendererProps> = ({
  spec,
  bindings,
  onExecute,
  preview = false,
  onQueryStatus,
  onCancelTask,
  onQueryApprovalStatus,
  title,
}) => {
  const [loading, setLoading] = useState(false);
  const [taskStatus, setTaskStatus] = useState<TaskStatusResult | null>(null);
  const [approvalId, setApprovalId] = useState<string>('');
  const [approvalStatus, setApprovalStatus] = useState<ApprovalStatusResult | null>(null);
  const [polling, setPolling] = useState(false);
  const pollingRef = useRef<NodeJS.Timeout | null>(null);
  const taskStatusRef = useRef<TaskStatusResult | null>(null);

  useEffect(() => {
    taskStatusRef.current = taskStatus;
  }, [taskStatus]);

  // 查找主绑定
  const mainBinding = bindings.find((b) => b.usage === 'task');
  const statusBinding = spec.taskView.statusBindingId
    ? bindings.find((binding) => binding.id === spec.taskView.statusBindingId)
    : bindings.find((binding) => binding.usage === 'task_status');
  const eventsBinding = spec.taskView.eventsBindingId
    ? bindings.find((binding) => binding.id === spec.taskView.eventsBindingId)
    : bindings.find((binding) => binding.usage === 'task_events');
  const resultBinding = spec.taskView.resultBindingId
    ? bindings.find((binding) => binding.id === spec.taskView.resultBindingId)
    : bindings.find((binding) => binding.usage === 'task_result');
  const cancelBinding = spec.taskView.cancelBindingId
    ? bindings.find((binding) => binding.id === spec.taskView.cancelBindingId)
    : bindings.find((binding) => binding.usage === 'task_cancel');
  const taskIdStateKey = spec.taskView.taskIdStateKey || 'taskId';
  const statusStatePath = spec.taskView.statusStatePath || '';
  const canQueryTaskStatus = Boolean(statusBinding || onQueryStatus);

  // 轮询任务状态
  const pollTaskStatus = useCallback(
    async (taskId: string) => {
      if (statusBinding) {
        try {
          const statusResponse = await onExecute(statusBinding.id, {
            pageState: pageStateForTask(taskId, taskIdStateKey),
          });
          const statusData = patchValue(statusBinding, statusResponse, 'taskStatus');
          let resultData: JSONValue | undefined;
          let eventsData: JSONValue | undefined;
          if (eventsBinding) {
            const eventsResponse = await onExecute(eventsBinding.id, {
              pageState: pageStateForTask(taskId, taskIdStateKey),
            });
            eventsData = patchValue(eventsBinding, eventsResponse, 'taskEvents');
          }
          if (resultBinding) {
            const resultResponse = await onExecute(resultBinding.id, {
              pageState: pageStateForTask(taskId, taskIdStateKey),
            });
            resultData = patchValue(resultBinding, resultResponse, 'taskResult');
          }
          if (statusData === undefined) {
            message.error('任务状态绑定未映射到 pageState.taskStatus');
            setPolling(false);
            if (pollingRef.current) {
              clearInterval(pollingRef.current);
              pollingRef.current = null;
            }
            return;
          }
          const nextStatus = normalizeTaskStatusFromExecution(
            taskId,
            statusData,
            statusStatePath,
            taskStatusRef.current,
          );
          const events = extractTaskEvents(eventsData);
          const result = resolveTaskResultFromExecution(resultData, nextStatus.result);
          const next: TaskStatusResult = {
            ...nextStatus,
            events: events.length ? events : nextStatus.events,
            result,
          };
          taskStatusRef.current = next;
          setTaskStatus(next);
          if (
            next.status === 'completed' ||
            next.status === 'failed' ||
            next.status === 'cancelled'
          ) {
            setPolling(false);
            if (pollingRef.current) {
              clearInterval(pollingRef.current);
              pollingRef.current = null;
            }
          }
        } catch (error) {
          console.error('Failed to poll task status via binding:', error);
        }
        return;
      }

      if (!onQueryStatus) {
        return;
      }

      try {
        const status = await onQueryStatus(taskId);
        const next: TaskStatusResult = {
          taskId: status.taskId,
          status: status.status,
          progress: status.progress,
          message: status.message,
          result: status.result,
          error: status.error,
          events: status.events,
        };
        taskStatusRef.current = next;
        setTaskStatus(next);

        // 如果任务完成或失败，停止轮询
        if (
          status.status === 'completed' ||
          status.status === 'failed' ||
          status.status === 'cancelled'
        ) {
          setPolling(false);
          if (pollingRef.current) {
            clearInterval(pollingRef.current);
            pollingRef.current = null;
          }
        }
      } catch (error) {
        console.error('Failed to poll task status:', error);
      }
    },
    [
      eventsBinding,
      onExecute,
      onQueryStatus,
      resultBinding,
      statusBinding,
      statusStatePath,
      taskIdStateKey,
    ],
  );

  // 开始轮询
  const startPolling = useCallback(
    (taskId: string) => {
      if (!canQueryTaskStatus) {
        message.warning('任务已提交，但页面未配置状态查询绑定');
        return;
      }
      setPolling(true);
      pollingRef.current = setInterval(() => pollTaskStatus(taskId), 2000);
      // 立即查询一次
      pollTaskStatus(taskId);
    },
    [canQueryTaskStatus, pollTaskStatus],
  );

  // 停止轮询
  const stopPolling = useCallback(() => {
    setPolling(false);
    if (pollingRef.current) {
      clearInterval(pollingRef.current);
      pollingRef.current = null;
    }
  }, []);

  // 组件卸载时停止轮询
  useEffect(() => {
    return () => {
      stopPolling();
    };
  }, [stopPolling]);

  // 处理表单提交
  const handleSubmit = useCallback(
    async (values: FormValues) => {
      if (!mainBinding) {
        message.error('未配置任务绑定');
        return;
      }
      if (preview) {
        message.info('预览模式不提交任务');
        return;
      }

      setLoading(true);
      setTaskStatus(null);
      taskStatusRef.current = null;
      setApprovalId('');
      setApprovalStatus(null);

      try {
        const response = await onExecute(mainBinding.id, { form: values });
        if (response.kind === 'approval') {
          const nextApprovalId = response.approvalId || response.requestId;
          const next: TaskStatusResult = {
            taskId: nextApprovalId,
            status: 'pending',
            message: '任务已提交审批，审批通过后才会启动任务',
          };
          setApprovalId(nextApprovalId);
          taskStatusRef.current = next;
          setTaskStatus(next);
          message.info('任务已提交审批');
          return;
        }
        const taskIdFromSelector = patchValue(mainBinding, response, taskIdStateKey);
        const taskId = response.taskId || readString(taskIdFromSelector);

        if (taskId) {
          const next: TaskStatusResult = {
            taskId,
            status: 'pending',
            message: '任务已提交',
          };
          taskStatusRef.current = next;
          setTaskStatus(next);
          startPolling(taskId);
          message.success('任务已提交');
        } else {
          message.warning('未获取到任务 ID');
        }
      } catch (error) {
        const msg = error instanceof Error ? error.message : '未知错误';
        message.error('任务提交失败: ' + msg);
      } finally {
        setLoading(false);
      }
    },
    [mainBinding, onExecute, preview, startPolling, taskIdStateKey],
  );

  // 取消任务
  const handleCancel = useCallback(async () => {
    if (!taskStatus?.taskId) {
      return;
    }

    try {
      if (cancelBinding) {
        await onExecute(cancelBinding.id, {
          pageState: pageStateForTask(taskStatus.taskId, taskIdStateKey),
        });
      } else if (onCancelTask) {
        await onCancelTask(taskStatus.taskId);
      } else {
        message.warning('未配置取消任务绑定');
        return;
      }
      message.success('任务已取消');
      stopPolling();
      const next = taskStatusRef.current
        ? { ...taskStatusRef.current, status: 'cancelled' as const }
        : null;
      taskStatusRef.current = next;
      setTaskStatus(next);
    } catch (error) {
      const msg = error instanceof Error ? error.message : '未知错误';
      message.error('取消任务失败: ' + msg);
    }
  }, [cancelBinding, onCancelTask, onExecute, stopPolling, taskIdStateKey, taskStatus?.taskId]);

  const refreshApproval = useCallback(async () => {
    if (!approvalId || !onQueryApprovalStatus) {
      return;
    }
    try {
      const nextStatus = await onQueryApprovalStatus(approvalId);
      setApprovalStatus(nextStatus);
      if (nextStatus.status !== 'approved') {
        return;
      }
      if (nextStatus.resultKind === 'task' && nextStatus.taskId) {
        const next: TaskStatusResult = {
          taskId: nextStatus.taskId,
          status: 'pending',
          message: '审批已通过，任务已启动',
        };
        setApprovalId('');
        taskStatusRef.current = next;
        setTaskStatus(next);
        startPolling(nextStatus.taskId);
        return;
      }
      if (nextStatus.resultKind === 'sync') {
        const next: TaskStatusResult = {
          taskId: approvalId,
          status: 'completed',
          message: '审批已通过，执行已完成',
          result: nextStatus.result,
        };
        setApprovalId('');
        taskStatusRef.current = next;
        setTaskStatus(next);
      }
    } catch (error) {
      const msg = error instanceof Error ? error.message : '审批状态查询失败';
      message.error(msg);
    }
  }, [approvalId, onQueryApprovalStatus, startPolling]);

  // 渲染任务进度
  const renderTaskProgress = () => {
    if (!taskStatus) {
      return null;
    }

    return (
      <Card title="任务状态" style={{ marginTop: 16 }}>
        <Space orientation="vertical" style={{ width: '100%' }}>
          {/* 状态标签 */}
          <Space>
            <Text strong>状态:</Text>
            <Tag color={getStatusColor(taskStatus.status)} icon={getStatusIcon(taskStatus.status)}>
              {taskStatus.status.toUpperCase()}
            </Tag>
          </Space>

          {/* 进度条 */}
          {spec.taskView.showProgress && taskStatus.progress !== undefined && (
            <Progress
              percent={taskStatus.progress}
              status={
                taskStatus.status === 'failed'
                  ? 'exception'
                  : taskStatus.status === 'completed'
                    ? 'success'
                    : 'active'
              }
            />
          )}

          {/* 消息 */}
          {taskStatus.message && <Alert message={taskStatus.message} type="info" />}

          {approvalId ? (
            <Alert
              message={approvalStatus ? `审批状态：${approvalStatus.status}` : '等待审批'}
              description={approvalStatus?.reason || approvalStatus?.updatedAt}
              type={
                approvalStatus?.status === 'rejected'
                  ? 'error'
                  : approvalStatus?.status === 'approved'
                    ? 'success'
                    : 'info'
              }
              showIcon
            />
          ) : null}

          {/* 错误信息 */}
          {taskStatus.error && <Alert message={taskStatus.error} type="error" />}

          {/* 操作按钮 */}
          <Space>
            {spec.taskView.cancelable &&
              taskStatus.status === 'running' &&
              (cancelBinding || onCancelTask) && (
                <Button danger icon={<StopOutlined />} onClick={handleCancel}>
                  取消
                </Button>
              )}
            {!approvalId && canQueryTaskStatus ? (
              <Button
                icon={<SyncOutlined />}
                onClick={() => pollTaskStatus(taskStatus.taskId)}
                loading={polling}
              >
                刷新
              </Button>
            ) : null}
            {approvalId && onQueryApprovalStatus ? (
              <Button onClick={refreshApproval}>刷新审批状态</Button>
            ) : null}
          </Space>
        </Space>
      </Card>
    );
  };

  // 渲染事件时间线
  const renderTimeline = () => {
    if (!taskStatus?.events || taskStatus.events.length === 0) {
      return null;
    }

    return (
      <Card title="任务事件" style={{ marginTop: 16 }}>
        <Timeline
          items={taskStatus.events.map((event) => ({
            color: event.type === 'error' ? 'red' : event.type === 'warning' ? 'orange' : 'blue',
            children: (
              <div>
                <Text type="secondary">{new Date(event.timestamp).toLocaleString()}</Text>
                <br />
                <Text>{event.message}</Text>
                {event.data ? (
                  <div style={{ marginTop: 8 }}>{renderJSONValueSummary(event.data)}</div>
                ) : null}
              </div>
            ),
          }))}
        />
      </Card>
    );
  };

  // 渲染结果
  const renderResult = () => {
    if (!taskStatus?.result) {
      return null;
    }

    return (
      <Card title="任务结果" style={{ marginTop: 16 }}>
        <ResultViewRenderer
          data={taskStatus.result}
          resultView={spec.resultView}
          emptyTitle="任务结果视图未配置"
        />
      </Card>
    );
  };

  return (
    <div>
      {/* 表单 */}
      <Card title={title || '提交任务'}>
        <SchemaFormRenderer
          spec={spec.form}
          onFinish={handleSubmit}
          disabled={loading || preview}
        />
      </Card>

      {/* 任务进度 */}
      {renderTaskProgress()}

      {/* 事件时间线 */}
      {spec.taskView.showTimeline && spec.taskView.showEvents && renderTimeline()}

      {/* 任务结果 */}
      {taskStatus?.status === 'completed' && renderResult()}
    </div>
  );
};

export default TaskPageRenderer;
