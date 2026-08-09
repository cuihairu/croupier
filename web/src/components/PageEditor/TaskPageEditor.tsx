/**
 * TaskPageEditor - 任务页面语义化编辑器。
 *
 * 只编辑 TaskPageSpec 中已有的强类型展示配置；任务函数绑定、
 * taskId/status/events/result selector 由 PageSpec 校验链约束。
 */

import React, { useCallback, useState } from 'react';
import { Card, Collapse, Form, Input, Select, Space, Switch, Tag, Typography } from 'antd';
import { FileTextOutlined, ProfileOutlined, ScheduleOutlined } from '@ant-design/icons';
import type { ResultViewSpec, TaskPageSpec, TaskViewSpec } from '@/types/dashboard';
import FormPresentationEditor from './FormPresentationEditor';

const { Text } = Typography;
const { Panel } = Collapse;

export interface TaskPageEditorProps {
  value: TaskPageSpec;
  onChange: (value: TaskPageSpec) => void;
  readonly?: boolean;
}

export default function TaskPageEditor({ value, onChange, readonly = false }: TaskPageEditorProps) {
  const [activeKey, setActiveKey] = useState<string[]>(['taskView']);

  const handleTaskViewChange = useCallback(
    (updates: Partial<TaskViewSpec>) => {
      onChange({
        ...value,
        taskView: {
          ...value.taskView,
          ...updates,
        },
      });
    },
    [onChange, value],
  );

  const handleResultViewChange = useCallback(
    (updates: Partial<ResultViewSpec>) => {
      onChange({
        ...value,
        resultView: {
          ...value.resultView,
          ...updates,
        },
      });
    },
    [onChange, value],
  );

  return (
    <Collapse activeKey={activeKey} onChange={setActiveKey} bordered={false}>
      <Panel
        header={
          <Space>
            <ScheduleOutlined />
            <Text strong>任务视图</Text>
          </Space>
        }
        key="taskView"
      >
        <Form layout="vertical" disabled={readonly}>
          <Form.Item label="显示时间线">
            <Switch
              checked={value.taskView.showTimeline}
              onChange={(showTimeline) => handleTaskViewChange({ showTimeline })}
            />
          </Form.Item>
          <Form.Item label="显示进度">
            <Switch
              checked={value.taskView.showProgress}
              onChange={(showProgress) => handleTaskViewChange({ showProgress })}
            />
          </Form.Item>
          <Form.Item label="显示事件">
            <Switch
              checked={value.taskView.showEvents}
              disabled={readonly || !value.taskView.eventsBindingId}
              onChange={(showEvents) => handleTaskViewChange({ showEvents })}
            />
            {!value.taskView.eventsBindingId ? (
              <Text type="secondary" style={{ marginLeft: 8 }}>
                未生成 events binding，不能开启事件展示。
              </Text>
            ) : null}
          </Form.Item>
          <Form.Item label="允许取消">
            <Switch
              checked={value.taskView.cancelable}
              disabled={readonly || !value.taskView.cancelBindingId}
              onChange={(cancelable) => handleTaskViewChange({ cancelable })}
            />
            {!value.taskView.cancelBindingId ? (
              <Text type="secondary" style={{ marginLeft: 8 }}>
                未生成 cancel binding，不能开启取消入口。
              </Text>
            ) : null}
          </Form.Item>
          <Form.Item label="允许重试">
            <Switch checked={false} disabled />
            <Text type="secondary" style={{ marginLeft: 8 }}>
              当前未配置真实 retry function，不能生成重试入口。
            </Text>
          </Form.Item>
          <Form.Item label="Lifecycle bindings">
            <Space direction="vertical" size={4}>
              <Text type="secondary">
                taskId state: <Text code>{value.taskView.taskIdStateKey || 'taskId'}</Text>
              </Text>
              <Text type="secondary">
                status: <Text code>{value.taskView.statusBindingId || '未配置'}</Text>
              </Text>
              <Text type="secondary">
                status path: <Text code>{value.taskView.statusStatePath || '未配置'}</Text>
              </Text>
              <Text type="secondary">
                events: <Text code>{value.taskView.eventsBindingId || '未配置'}</Text>
              </Text>
              <Text type="secondary">
                result: <Text code>{value.taskView.resultBindingId || '未配置'}</Text>
              </Text>
              <Text type="secondary">
                cancel: <Text code>{value.taskView.cancelBindingId || '未配置'}</Text>
              </Text>
            </Space>
          </Form.Item>
        </Form>
      </Panel>

      <Panel
        header={
          <Space>
            <ProfileOutlined />
            <Text strong>启动表单</Text>
            <Tag>{value.form?.fields?.length || 0} 字段</Tag>
          </Space>
        }
        key="form"
      >
        <FormPresentationEditor
          value={value.form}
          onChange={(form) => onChange({ ...value, form })}
          readonly={readonly}
        />
      </Panel>

      <Panel
        header={
          <Space>
            <FileTextOutlined />
            <Text strong>结果视图</Text>
            <Tag>{value.resultView?.fields?.length || 0} 字段</Tag>
          </Space>
        }
        key="resultView"
      >
        <Text type="secondary">结果字段来自已发布输出映射；这里只调整展示标题和格式。</Text>

        {value.resultView?.fields?.map((field, index) => (
          <Card
            key={field.key}
            size="small"
            style={{ marginBottom: 8 }}
            title={
              <Space>
                <Text code>{field.key}</Text>
                <Tag>{field.dataType}</Tag>
              </Space>
            }
          >
            <Form layout="inline" disabled={readonly}>
              <Form.Item label="标题">
                <Input
                  size="small"
                  value={field.title?.['zh-CN'] || ''}
                  onChange={(event) => {
                    const fields = (value.resultView?.fields || []).map((current, currentIndex) =>
                      currentIndex === index
                        ? { ...current, title: { ...field.title, 'zh-CN': event.target.value } }
                        : current,
                    );
                    handleResultViewChange({ fields });
                  }}
                />
              </Form.Item>
              <Form.Item label="类型">
                <Select
                  size="small"
                  value={field.dataType}
                  onChange={(dataType) => {
                    const fields = (value.resultView?.fields || []).map((current, currentIndex) =>
                      currentIndex === index ? { ...current, dataType } : current,
                    );
                    handleResultViewChange({ fields });
                  }}
                  style={{ width: 120 }}
                  options={[
                    { value: 'string', label: '字符串' },
                    { value: 'number', label: '数字' },
                    { value: 'boolean', label: '布尔' },
                    { value: 'datetime', label: '日期时间' },
                  ]}
                />
              </Form.Item>
            </Form>
          </Card>
        ))}
      </Panel>
    </Collapse>
  );
}
