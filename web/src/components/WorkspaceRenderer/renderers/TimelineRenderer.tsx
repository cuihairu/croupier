/**
 * 时间线布局渲染器
 *
 * 支持时间线视图，展示事件流。
 *
 * @module components/WorkspaceRenderer/renderers/TimelineRenderer
 */

import React, { useState } from 'react';
import {
  Card,
  Timeline,
  Tag,
  Avatar,
  Button,
  Modal,
  Form,
  Input,
  Select,
  Space,
  Badge,
  Dropdown,
} from 'antd';
import {
  PlusOutlined,
  EditOutlined,
  DeleteOutlined,
  UserOutlined,
  ClockCircleOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  SyncOutlined,
  MoreOutlined,
} from '@ant-design/icons';
import type { MenuProps } from 'antd';
import { buildGamePreviewTimelineEvents } from './mockData';
import { RendererEmpty, RendererModeNotice, isTemplatePreviewContext } from './state';

export interface TimelineRendererProps {
  config: any;
  context?: Record<string, any>;
  onDataChange?: (data: TimelineData) => void;
}

export interface TimelineData {
  events: TimelineEvent[];
}

export interface TimelineEvent {
  id: string;
  type: 'created' | 'updated' | 'deleted' | 'approved' | 'rejected' | 'custom';
  title: string;
  description?: string;
  timestamp: string;
  actor?: {
    id: string;
    name: string;
    avatar?: string;
  };
  status?: 'success' | 'pending' | 'error' | 'warning';
  tags?: string[];
  metadata?: Record<string, any>;
}

// 事件类型颜色和图标
const EVENT_TYPE_CONFIG: Record<string, { color: string; icon: React.ReactNode }> = {
  created: { color: 'blue', icon: <PlusOutlined /> },
  updated: { color: 'orange', icon: <SyncOutlined /> },
  deleted: { color: 'red', icon: <DeleteOutlined /> },
  approved: { color: 'green', icon: <CheckCircleOutlined /> },
  rejected: { color: 'red', icon: <CloseCircleOutlined /> },
  custom: { color: 'purple', icon: <MoreOutlined /> },
};

// 状态颜色
const STATUS_COLORS: Record<string, string> = {
  success: 'green',
  pending: 'blue',
  error: 'red',
  warning: 'orange',
};

export default function TimelineRenderer({ config, context, onDataChange }: TimelineRendererProps) {
  const isTemplatePreview = isTemplatePreviewContext(context);
  const [data, setData] = useState<TimelineData>(
    config?.data || {
      events: isTemplatePreview ? (buildGamePreviewTimelineEvents() as any) : [],
    },
  );
  const [showAddModal, setShowAddModal] = useState(false);
  const [editingEvent, setEditingEvent] = useState<TimelineEvent | null>(null);
  const [filterType, setFilterType] = useState<string | null>(null);
  const [form] = Form.useForm();

  // 过滤事件
  const filteredEvents = data.events.filter((event) => {
    if (filterType && event.type !== filterType) {
      return false;
    }
    return true;
  });

  // 添加事件
  const handleAddEvent = async () => {
    try {
      const values = await form.validateFields();
      const newEvent: TimelineEvent = {
        id: `event_${Date.now()}`,
        type: values.type || 'custom',
        title: values.title,
        description: values.description,
        timestamp: new Date().toISOString(),
        status: values.status,
        tags: values.tags || [],
      };

      const newData = {
        events: [...data.events, newEvent],
      };
      setData(newData);
      onDataChange?.(newData);
      setShowAddModal(false);
      form.resetFields();
    } catch (error) {
      // 验证失败
    }
  };

  // 更新事件
  const handleUpdateEvent = async () => {
    if (!editingEvent) return;

    try {
      const values = await form.validateFields();
      const updatedEvents = data.events.map((event) =>
        event.id === editingEvent.id ? { ...event, ...values } : event,
      );

      const newData = { events: updatedEvents };
      setData(newData);
      onDataChange?.(newData);
      setEditingEvent(null);
      form.resetFields();
    } catch (error) {
      // 验证失败
    }
  };

  // 删除事件
  const handleDeleteEvent = (eventId: string) => {
    const updatedEvents = data.events.filter((event) => event.id !== eventId);
    const newData = { events: updatedEvents };
    setData(newData);
    onDataChange?.(newData);
  };

  // 打开编辑模态框
  const openEditModal = (event: TimelineEvent) => {
    setEditingEvent(event);
    form.setFieldsValue({
      title: event.title,
      description: event.description,
      type: event.type,
      status: event.status,
      tags: event.tags,
    });
    setShowAddModal(true);
  };

  // 获取事件类型配置
  const getEventConfig = (type: string) => {
    return EVENT_TYPE_CONFIG[type] || EVENT_TYPE_CONFIG.custom;
  };

  if (data.events.length === 0) {
    return (
      <>
        <RendererModeNotice
          context={context}
          sampleTitle="时间线预览"
          sampleDescription="当前时间线正在使用示例事件帮助你预览结构，正式运行仍需要真实 events 数据。"
        />
        <Card>
          <RendererEmpty
            description={
              isTemplatePreview ? '当前时间线预览暂无示例事件' : '当前时间线未绑定真实事件数据'
            }
          />
        </Card>
      </>
    );
  }

  return (
    <div className="timeline-renderer">
      <RendererModeNotice
        context={context}
        sampleTitle="时间线预览"
        sampleDescription="当前时间线正在使用示例事件帮助你预览结构，正式运行仍需要真实 events 数据。"
      />
      {/* 工具栏 */}
      <div
        style={{
          marginBottom: 16,
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
        }}
      >
        <Space>
          <Select
            placeholder="筛选类型"
            allowClear
            style={{ width: 120 }}
            value={filterType}
            onChange={(value) => setFilterType(value)}
            options={[
              { value: null, label: '全部' },
              { value: 'created', label: '创建' },
              { value: 'updated', label: '更新' },
              { value: 'approved', label: '审批' },
              { value: 'rejected', label: '拒绝' },
            ]}
          />
        </Space>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => setShowAddModal(true)}>
          添加事件
        </Button>
      </div>

      {/* 时间线 */}
      <Timeline
        mode="left"
        items={filteredEvents.map((event) => {
          const config = getEventConfig(event.type);
          return {
            key: event.id,
            dot: <span style={{ color: config.color }}>{config.icon}</span>,
            children: (
              <Card
                size="small"
                title={
                  <Space>
                    <span>{event.title}</span>
                    {event.status && (
                      <Badge status={STATUS_COLORS[event.status] as any} text={event.status} />
                    )}
                  </Space>
                }
                extra={
                  <Space>
                    <Button
                      type="text"
                      size="small"
                      icon={<EditOutlined />}
                      onClick={() => openEditModal(event)}
                    />
                    <Button
                      type="text"
                      size="small"
                      danger
                      icon={<DeleteOutlined />}
                      onClick={() => handleDeleteEvent(event.id)}
                    />
                  </Space>
                }
              >
                {event.description && <p style={{ marginBottom: 8 }}>{event.description}</p>}
                <Space wrap>
                  {event.tags?.map((tag) => (
                    <Tag key={tag}>{tag}</Tag>
                  ))}
                  {event.actor && (
                    <span style={{ color: '#666', fontSize: 12 }}>
                      <UserOutlined style={{ marginRight: 4 }} />
                      {event.actor.name}
                    </span>
                  )}
                  <span style={{ color: '#999', fontSize: 12 }}>
                    <ClockCircleOutlined style={{ marginRight: 4 }} />
                    {new Date(event.timestamp).toLocaleString()}
                  </span>
                </Space>
              </Card>
            ),
          };
        })}
      />

      {/* 添加/编辑事件模态框 */}
      <Modal
        title={editingEvent ? '编辑事件' : '添加事件'}
        open={showAddModal}
        onOk={editingEvent ? handleUpdateEvent : handleAddEvent}
        onCancel={() => {
          setShowAddModal(false);
          setEditingEvent(null);
          form.resetFields();
        }}
      >
        <Form form={form} layout="vertical">
          <Form.Item name="title" label="标题" rules={[{ required: true, message: '请输入标题' }]}>
            <Input placeholder="事件标题" />
          </Form.Item>
          <Form.Item name="description" label="描述">
            <Input.TextArea rows={3} placeholder="事件描述" />
          </Form.Item>
          <Form.Item name="type" label="类型" initialValue="custom">
            <Select placeholder="选择类型">
              <Select.Option value="created">创建</Select.Option>
              <Select.Option value="updated">更新</Select.Option>
              <Select.Option value="approved">审批</Select.Option>
              <Select.Option value="rejected">拒绝</Select.Option>
              <Select.Option value="custom">自定义</Select.Option>
            </Select>
          </Form.Item>
          <Form.Item name="status" label="状态">
            <Select placeholder="选择状态" allowClear>
              <Select.Option value="success">成功</Select.Option>
              <Select.Option value="pending">待处理</Select.Option>
              <Select.Option value="error">错误</Select.Option>
              <Select.Option value="warning">警告</Select.Option>
            </Select>
          </Form.Item>
          <Form.Item name="tags" label="标签">
            <Select mode="tags" placeholder="输入标签后按回车" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
