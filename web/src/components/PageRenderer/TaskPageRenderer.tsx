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
import {
  ProForm,
  ProFormText,
  ProFormTextArea,
  ProFormSelect,
  ProFormDigit,
} from '@ant-design/pro-components';
import {
  Card,
  Button,
  Space,
  message,
  Result,
  Typography,
  Timeline,
  Progress,
  Tag,
  Descriptions,
  Alert,
} from 'antd';
import {
  PlayCircleOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  SyncOutlined,
  ClockCircleOutlined,
  StopOutlined,
  ReloadOutlined,
} from '@ant-design/icons';
import type {
  TaskPageSpec,
  FormFieldSpec,
  PageFunctionBindingV2,
} from '@/types/dashboard-vnext';

const { Text } = Typography;

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

interface TaskStatus {
  taskId: string;
  status: 'pending' | 'running' | 'completed' | 'failed' | 'cancelled';
  progress?: number;
  message?: string;
  result?: unknown;
  error?: string;
  events?: TaskEvent[];
}

interface TaskEvent {
  timestamp: string;
  type: 'info' | 'warning' | 'error' | 'progress';
  message: string;
  data?: unknown;
}

// ---------------------------------------------------------------------------
// Props
// ---------------------------------------------------------------------------

export interface TaskPageRendererProps {
  /** 任务页面规格 */
  spec: TaskPageSpec;
  /** 页面绑定 */
  bindings: PageFunctionBindingV2[];
  /** 执行绑定函数 */
  onExecute: (bindingId: string, payload: unknown) => Promise<unknown>;
  /** 查询任务状态 */
  onQueryStatus?: (taskId: string) => Promise<TaskStatus>;
  /** 取消任务 */
  onCancelTask?: (taskId: string) => Promise<void>;
  /** 重试任务 */
  onRetryTask?: (taskId: string) => Promise<void>;
  /** 页面标题 */
  title?: string;
}

// ---------------------------------------------------------------------------
// 表单字段渲染
// ---------------------------------------------------------------------------

function renderFormField(field: FormFieldSpec): React.ReactNode {
  const label = field.label?.['zh-CN'] || field.label?.['en'] || field.key;
  const placeholder = field.placeholder?.['zh-CN'] || field.placeholder?.['en'];
  const required = field.required;
  const rules = required ? [{ required: true, message: `请输入${label}` }] : [];

  switch (field.widget) {
    case 'TextArea':
      return (
        <ProFormTextArea
          key={field.key}
          name={field.key}
          label={label}
          placeholder={placeholder}
          rules={rules}
        />
      );
    case 'InputNumber':
      return (
        <ProFormDigit
          key={field.key}
          name={field.key}
          label={label}
          placeholder={placeholder}
          rules={rules}
        />
      );
    case 'Select':
      return (
        <ProFormSelect
          key={field.key}
          name={field.key}
          label={label}
          placeholder={placeholder}
          rules={rules}
          options={field.enumOptions?.map((opt) => ({
            label: opt.label['zh-CN'] || opt.label['en'] || opt.value,
            value: opt.value,
          }))}
        />
      );
    default:
      return (
        <ProFormText
          key={field.key}
          name={field.key}
          label={label}
          placeholder={placeholder}
          rules={rules}
        />
      );
  }
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

// ---------------------------------------------------------------------------
// TaskPageRenderer 组件
// ---------------------------------------------------------------------------

const TaskPageRenderer: React.FC<TaskPageRendererProps> = ({
  spec,
  bindings,
  onExecute,
  onQueryStatus,
  onCancelTask,
  onRetryTask,
  title,
}) => {
  const [loading, setLoading] = useState(false);
  const [taskStatus, setTaskStatus] = useState<TaskStatus | null>(null);
  const [polling, setPolling] = useState(false);
  const pollingRef = useRef<NodeJS.Timeout | null>(null);

  // 查找主绑定
  const mainBinding = bindings.find((b) => b.usage === 'task') || bindings[0];

  // 轮询任务状态
  const pollTaskStatus = useCallback(
    async (taskId: string) => {
      if (!onQueryStatus) {
        return;
      }

      try {
        const status = await onQueryStatus(taskId);
        setTaskStatus(status);

        // 如果任务完成或失败，停止轮询
        if (status.status === 'completed' || status.status === 'failed' || status.status === 'cancelled') {
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
    [onQueryStatus]
  );

  // 开始轮询
  const startPolling = useCallback(
    (taskId: string) => {
      setPolling(true);
      pollingRef.current = setInterval(() => pollTaskStatus(taskId), 2000);
      // 立即查询一次
      pollTaskStatus(taskId);
    },
    [pollTaskStatus]
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
    async (values: unknown) => {
      if (!mainBinding) {
        message.error('未配置任务绑定');
        return;
      }

      setLoading(true);
      setTaskStatus(null);

      try {
        const response = await onExecute(mainBinding.id, values);
        const taskId = (response as any)?.taskId;

        if (taskId) {
          setTaskStatus({
            taskId,
            status: 'pending',
            message: '任务已提交',
          });
          startPolling(taskId);
          message.success('任务已提交');
        } else {
          message.warning('未获取到任务 ID');
        }
      } catch (error: any) {
        message.error('任务提交失败: ' + (error.message || '未知错误'));
      } finally {
        setLoading(false);
      }
    },
    [mainBinding, onExecute, startPolling]
  );

  // 取消任务
  const handleCancel = useCallback(async () => {
    if (!taskStatus?.taskId || !onCancelTask) {
      return;
    }

    try {
      await onCancelTask(taskStatus.taskId);
      message.success('任务已取消');
      stopPolling();
      setTaskStatus((prev) => prev ? { ...prev, status: 'cancelled' } : null);
    } catch (error: any) {
      message.error('取消任务失败: ' + (error.message || '未知错误'));
    }
  }, [taskStatus?.taskId, onCancelTask, stopPolling]);

  // 重试任务
  const handleRetry = useCallback(async () => {
    if (!taskStatus?.taskId || !onRetryTask) {
      return;
    }

    try {
      await onRetryTask(taskStatus.taskId);
      message.success('任务已重新提交');
      startPolling(taskStatus.taskId);
    } catch (error: any) {
      message.error('重试失败: ' + (error.message || '未知错误'));
    }
  }, [taskStatus?.taskId, onRetryTask, startPolling]);

  // 渲染任务进度
  const renderTaskProgress = () => {
    if (!taskStatus) {
      return null;
    }

    return (
      <Card title="任务状态" style={{ marginTop: 16 }}>
        <Space direction="vertical" style={{ width: '100%' }}>
          {/* 状态标签 */}
          <Space>
            <Text strong>状态:</Text>
            <Tag color={getStatusColor(taskStatus.status)} icon={getStatusIcon(taskStatus.status)}>
              {taskStatus.status.toUpperCase()}
            </Tag>
          </Space>

          {/* 进度条 */}
          {taskStatus.progress !== undefined && (
            <Progress
              percent={taskStatus.progress}
              status={taskStatus.status === 'failed' ? 'exception' : taskStatus.status === 'completed' ? 'success' : 'active'}
            />
          )}

          {/* 消息 */}
          {taskStatus.message && (
            <Alert message={taskStatus.message} type="info" />
          )}

          {/* 错误信息 */}
          {taskStatus.error && (
            <Alert message={taskStatus.error} type="error" />
          )}

          {/* 操作按钮 */}
          <Space>
            {spec.taskView.cancelable && taskStatus.status === 'running' && onCancelTask && (
              <Button danger icon={<StopOutlined />} onClick={handleCancel}>
                取消
              </Button>
            )}
            {spec.taskView.retryable && taskStatus.status === 'failed' && onRetryTask && (
              <Button icon={<ReloadOutlined />} onClick={handleRetry}>
                重试
              </Button>
            )}
            <Button icon={<SyncOutlined />} onClick={() => pollTaskStatus(taskStatus.taskId)} loading={polling}>
              刷新
            </Button>
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
                {event.data && (
                  <pre style={{ marginTop: 8, background: '#f5f5f5', padding: 8, fontSize: 12 }}>
                    {JSON.stringify(event.data, null, 2)}
                  </pre>
                )}
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
        {spec.resultView?.fields && spec.resultView.fields.length > 0 ? (
          <Descriptions column={1} bordered>
            {spec.resultView.fields.map((field) => (
              <Descriptions.Item
                key={field.key}
                label={field.title['zh-CN'] || field.title['en'] || field.key}
              >
                {(taskStatus.result as any)[field.key]?.toString() || '-'}
              </Descriptions.Item>
            ))}
          </Descriptions>
        ) : (
          <pre style={{ maxHeight: 400, overflow: 'auto' }}>
            {JSON.stringify(taskStatus.result, null, 2)}
          </pre>
        )}
      </Card>
    );
  };

  return (
    <div>
      {/* 表单 */}
      <Card title={title || '提交任务'}>
        <ProForm
          onFinish={handleSubmit}
          submitter={{
            submitButtonProps: { loading },
          }}
        >
          {spec.form.fields?.map(renderFormField)}
        </ProForm>
      </Card>

      {/* 任务进度 */}
      {renderTaskProgress()}

      {/* 事件时间线 */}
      {spec.taskView.showTimeline && renderTimeline()}

      {/* 任务结果 */}
      {taskStatus?.status === 'completed' && renderResult()}
    </div>
  );
};

export default TaskPageRenderer;
