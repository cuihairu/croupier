/**
 * 看板布局渲染器
 *
 * 支持看板视图，可拖拽卡片。
 *
 * @module components/WorkspaceRenderer/renderers/KanbanRenderer
 */

import React, { useState, useCallback } from 'react';
import {
  Card,
  Empty,
  Tag,
  Avatar,
  Dropdown,
  Button,
  Modal,
  Form,
  Input,
  Select,
  Space,
  Badge,
  Tooltip,
} from 'antd';
import {
  PlusOutlined,
  MoreOutlined,
  EditOutlined,
  DeleteOutlined,
  UserOutlined,
  ClockCircleOutlined,
  DragOutlined,
} from '@ant-design/icons';
import type { MenuProps } from 'antd';
import { buildGamePreviewKanbanColumns } from './mockData';
import { RendererEmpty, RendererModeNotice, isTemplatePreviewContext } from './state';

export interface KanbanRendererProps {
  config: any;
  context?: Record<string, any>;
  onDataChange?: (data: KanbanData) => void;
}

export interface KanbanData {
  columns: KanbanColumn[];
}

export interface KanbanColumn {
  id: string;
  title: string;
  color?: string;
  limit?: number; // 卡片数量限制
  cards: KanbanCard[];
}

export interface KanbanCard {
  id: string;
  title: string;
  description?: string;
  priority?: 'high' | 'medium' | 'low';
  tags?: string[];
  assignee?: {
    id: string;
    name: string;
    avatar?: string;
  };
  dueDate?: string;
  metadata?: Record<string, any>;
}

// 优先级颜色
const PRIORITY_COLORS = {
  high: 'red',
  medium: 'orange',
  low: 'green',
};

const PREVIEW_KANBAN_DATA: KanbanData = { columns: buildGamePreviewKanbanColumns() as any };

export default function KanbanRenderer({ config, context, onDataChange }: KanbanRendererProps) {
  const isTemplatePreview = isTemplatePreviewContext(context);
  const [data, setData] = useState<KanbanData | null>(() => {
    if (config?.data) return config.data;
    return isTemplatePreview ? PREVIEW_KANBAN_DATA : null;
  });
  const [draggedCard, setDraggedCard] = useState<{
    card: KanbanCard;
    sourceColumnId: string;
  } | null>(null);
  const [showAddCard, setShowAddCard] = useState<string | null>(null); // columnId
  const [editingCard, setEditingCard] = useState<KanbanCard | null>(null);
  const [cardForm] = Form.useForm();

  // 通知数据变化
  const notifyChange = useCallback(
    (newData: KanbanData) => {
      setData(newData);
      onDataChange?.(newData);
    },
    [onDataChange],
  );

  // 拖拽开始
  const handleDragStart = (e: React.DragEvent, card: KanbanCard, columnId: string) => {
    setDraggedCard({ card, sourceColumnId: columnId });
    e.dataTransfer.effectAllowed = 'move';
  };

  // 拖拽进入列
  const handleDragOver = (e: React.DragEvent) => {
    e.preventDefault();
    e.dataTransfer.dropEffect = 'move';
  };

  // 放置卡片
  const handleDrop = (e: React.DragEvent, targetColumnId: string) => {
    e.preventDefault();

    if (!draggedCard) return;

    const { card, sourceColumnId } = draggedCard;

    if (sourceColumnId === targetColumnId) {
      setDraggedCard(null);
      return;
    }

    // 移动卡片
    const newData: KanbanData = {
      columns: (data?.columns || []).map((col) => {
        if (col.id === sourceColumnId) {
          return {
            ...col,
            cards: col.cards.filter((c) => c.id !== card.id),
          };
        }
        if (col.id === targetColumnId) {
          return {
            ...col,
            cards: [...col.cards, card],
          };
        }
        return col;
      }),
    };

    notifyChange(newData);
    setDraggedCard(null);
  };

  // 添加卡片
  const handleAddCard = async (columnId: string) => {
    if (!data) return;
    try {
      const values = await cardForm.validateFields();

      const newCard: KanbanCard = {
        id: `card_${Date.now()}`,
        title: values.title,
        description: values.description,
        priority: values.priority,
        tags: values.tags,
        assignee: values.assignee ? { id: values.assignee, name: values.assignee } : undefined,
        dueDate: values.dueDate,
      };

      const newData: KanbanData = {
        columns: data.columns.map((col) => {
          if (col.id === columnId) {
            return {
              ...col,
              cards: [...col.cards, newCard],
            };
          }
          return col;
        }),
      };

      notifyChange(newData);
      setShowAddCard(null);
      cardForm.resetFields();
    } catch (error) {
      // 表单验证失败
    }
  };

  // 更新卡片
  const handleUpdateCard = async () => {
    if (!editingCard) return;
    if (!data) return;

    try {
      const values = await cardForm.validateFields();

      const newData: KanbanData = {
        columns: data.columns.map((col) => ({
          ...col,
          cards: col.cards.map((card) =>
            card.id === editingCard.id
              ? {
                  ...card,
                  title: values.title,
                  description: values.description,
                  priority: values.priority,
                  tags: values.tags,
                  assignee: values.assignee
                    ? { id: values.assignee, name: values.assignee }
                    : undefined,
                  dueDate: values.dueDate,
                }
              : card,
          ),
        })),
      };

      notifyChange(newData);
      setEditingCard(null);
      cardForm.resetFields();
    } catch (error) {
      // 表单验证失败
    }
  };

  // 删除卡片
  const handleDeleteCard = (cardId: string) => {
    if (!data) return;
    Modal.confirm({
      title: '确认删除',
      content: '确定要删除这张卡片吗？',
      onOk: () => {
        const newData: KanbanData = {
          columns: data.columns.map((col) => ({
            ...col,
            cards: col.cards.filter((card) => card.id !== cardId),
          })),
        };
        notifyChange(newData);
      },
    });
  };

  // 卡片操作菜单
  const getCardMenuItems = (card: KanbanCard, columnId: string): MenuProps['items'] => [
    {
      key: 'edit',
      icon: <EditOutlined />,
      label: '编辑',
      onClick: () => {
        setEditingCard(card);
        cardForm.setFieldsValue(card);
      },
    },
    {
      key: 'delete',
      icon: <DeleteOutlined />,
      label: '删除',
      danger: true,
      onClick: () => handleDeleteCard(card.id),
    },
  ];

  if (!data || !data.columns || data.columns.length === 0) {
    return (
      <>
        <RendererModeNotice
          context={context}
          sampleTitle="看板预览"
          sampleDescription="当前看板正在使用示例卡片帮助你预览列结构，正式运行仍需要真实 data 绑定。"
        />
        <RendererEmpty
          description={
            isTemplatePreview ? '当前看板预览暂无示例列' : '当前看板未绑定真实数据，正式运行无法展示卡片'
          }
        />
      </>
    );
  }

  return (
    <>
      <RendererModeNotice
        context={context}
        sampleTitle="看板预览"
        sampleDescription="当前看板正在使用示例卡片帮助你预览列结构，正式运行仍需要真实 data 绑定。"
      />
      <div className="kanban-layout" style={{ display: 'flex', gap: 16, overflow: 'auto', padding: 4 }}>
      {data.columns.map((column) => (
        <div
          key={column.id}
          className="kanban-column"
          style={{
            width: 300,
            flexShrink: 0,
            background: '#f5f5f5',
            borderRadius: 8,
            padding: 12,
            display: 'flex',
            flexDirection: 'column',
          }}
          onDragOver={handleDragOver}
          onDrop={(e) => handleDrop(e, column.id)}
        >
          {/* 列标题 */}
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              marginBottom: 12,
              padding: '8px 0',
              borderBottom: `2px solid ${column.color || '#d9d9d9'}`,
            }}
          >
            <Badge color={column.color} />
            <span style={{ fontWeight: 600, marginLeft: 8, flex: 1 }}>{column.title}</span>
            <Badge count={column.cards.length} style={{ backgroundColor: column.color }} />
            <Button
              type="text"
              size="small"
              icon={<PlusOutlined />}
              onClick={() => setShowAddCard(column.id)}
            />
          </div>

          {/* 卡片列表 */}
          <div
            style={{ flex: 1, overflow: 'auto', display: 'flex', flexDirection: 'column', gap: 8 }}
          >
            {column.cards.map((card) => (
              <Card
                key={card.id}
                size="small"
                style={{
                  cursor: 'grab',
                  borderLeft: `3px solid ${PRIORITY_COLORS[card.priority || 'low']}`,
                }}
                draggable
                onDragStart={(e) => handleDragStart(e, card, column.id)}
              >
                <div
                  style={{
                    display: 'flex',
                    justifyContent: 'space-between',
                    alignItems: 'flex-start',
                  }}
                >
                  <div style={{ flex: 1 }}>
                    <div style={{ fontWeight: 500, marginBottom: 4 }}>{card.title}</div>
                    {card.description && (
                      <div style={{ fontSize: 12, color: '#666', marginBottom: 8 }}>
                        {card.description}
                      </div>
                    )}
                    {card.tags && card.tags.length > 0 && (
                      <div style={{ marginBottom: 8 }}>
                        {card.tags.map((tag) => (
                          <Tag key={tag} style={{ marginBottom: 4 }}>
                            {tag}
                          </Tag>
                        ))}
                      </div>
                    )}
                    <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                      {card.assignee && (
                        <Tooltip title={card.assignee.name}>
                          <Avatar size="small" icon={<UserOutlined />} src={card.assignee.avatar} />
                        </Tooltip>
                      )}
                      {card.dueDate && (
                        <Tooltip title={`截止日期: ${card.dueDate}`}>
                          <span style={{ fontSize: 12, color: '#999' }}>
                            <ClockCircleOutlined style={{ marginRight: 4 }} />
                            {card.dueDate}
                          </span>
                        </Tooltip>
                      )}
                    </div>
                  </div>
                  <Dropdown menu={{ items: getCardMenuItems(card, column.id) }} trigger={['click']}>
                    <Button type="text" size="small" icon={<MoreOutlined />} />
                  </Dropdown>
                </div>
              </Card>
            ))}

            {/* 添加卡片表单 */}
            {showAddCard === column.id && (
              <Card size="small" style={{ background: '#fff' }}>
                <Form form={cardForm} layout="vertical" size="small">
                  <Form.Item name="title" rules={[{ required: true, message: '请输入标题' }]}>
                    <Input placeholder="卡片标题" autoFocus />
                  </Form.Item>
                  <Form.Item name="description">
                    <Input.TextArea rows={2} placeholder="描述" />
                  </Form.Item>
                  <Form.Item name="priority">
                    <Select placeholder="优先级" allowClear>
                      <Select.Option value="high">高</Select.Option>
                      <Select.Option value="medium">中</Select.Option>
                      <Select.Option value="low">低</Select.Option>
                    </Select>
                  </Form.Item>
                  <Form.Item style={{ marginBottom: 0 }}>
                    <Space>
                      <Button type="primary" onClick={() => handleAddCard(column.id)}>
                        添加
                      </Button>
                      <Button onClick={() => setShowAddCard(null)}>取消</Button>
                    </Space>
                  </Form.Item>
                </Form>
              </Card>
            )}
          </div>
        </div>
      ))}

      {/* 编辑卡片弹窗 */}
      <Modal
        title="编辑卡片"
        open={!!editingCard}
        onOk={handleUpdateCard}
        onCancel={() => {
          setEditingCard(null);
          cardForm.resetFields();
        }}
      >
        <Form form={cardForm} layout="vertical">
          <Form.Item name="title" label="标题" rules={[{ required: true, message: '请输入标题' }]}>
            <Input placeholder="卡片标题" />
          </Form.Item>
          <Form.Item name="description" label="描述">
            <Input.TextArea rows={3} placeholder="卡片描述" />
          </Form.Item>
          <Form.Item name="priority" label="优先级">
            <Select placeholder="选择优先级" allowClear>
              <Select.Option value="high">高</Select.Option>
              <Select.Option value="medium">中</Select.Option>
              <Select.Option value="low">低</Select.Option>
            </Select>
          </Form.Item>
          <Form.Item name="tags" label="标签">
            <Select mode="tags" placeholder="输入标签后按回车" />
          </Form.Item>
          <Form.Item name="dueDate" label="截止日期">
            <Input type="date" />
          </Form.Item>
        </Form>
      </Modal>
      </div>
    </>
  );
}
