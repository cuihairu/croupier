/**
 * 字段库组件
 *
 * 提供常用字段模板，可拖拽到配置区域快速添加。
 *
 * @module pages/WorkspaceEditor/components/FieldLibrary
 */

import React, { useState } from 'react';
import { Card, Collapse, Tag, Space, Typography, Input, Tooltip, Button, Empty } from 'antd';
import type { CollapseProps } from 'antd';
import { DragOutlined, AppstoreOutlined, PlusOutlined, SearchOutlined } from '@ant-design/icons';
import {
  DndContext,
  DragEndEvent,
  DragOverlay,
  PointerSensor,
  useSensor,
  useSensors,
} from '@dnd-kit/core';
import { DraggableOptions } from '@dnd-kit/core';
import { SortableContext, useSortable, verticalListSortingStrategy } from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import type { FieldConfig } from '@/types/workspace';

const { Text } = Typography;

/** 字段模板分类 */
export type FieldCategory =
  | 'basic' // 基础字段
  | 'number' // 数字字段
  | 'datetime' // 日期时间
  | 'selection' // 选择字段
  | 'advanced'; // 高级字段

/** 字段模板 */
export interface FieldTemplate {
  key: string;
  name: string;
  category: FieldCategory;
  description: string;
  template: Partial<FieldConfig>;
  icon?: React.ReactNode;
  tags?: string[];
}

/** 内置字段模板 */
const BUILTIN_FIELD_TEMPLATES: FieldTemplate[] = [
  // 基础字段
  {
    key: 'text-input',
    name: '单行文本',
    category: 'basic',
    description: '基础文本输入框',
    tags: ['常用', '基础'],
    template: {
      key: '',
      label: '文本字段',
      type: 'input',
      placeholder: '请输入',
      required: false,
    },
  },
  {
    key: 'textarea',
    name: '多行文本',
    category: 'basic',
    description: '多行文本输入，适合长内容',
    tags: ['常用'],
    template: {
      key: '',
      label: '备注',
      type: 'textarea',
      placeholder: '请输入',
      required: false,
    },
  },
  {
    key: 'email',
    name: '邮箱',
    category: 'basic',
    description: '邮箱格式输入，带格式验证',
    tags: ['验证'],
    template: {
      key: '',
      label: '邮箱',
      type: 'input',
      placeholder: 'example@mail.com',
      required: false,
      rules: [{ type: 'email', pattern: '^[^@]+@[^@]+$', message: '请输入有效的邮箱地址' }],
    },
  },
  {
    key: 'phone',
    name: '手机号',
    category: 'basic',
    description: '手机号输入，11位数字验证',
    tags: ['验证'],
    template: {
      key: '',
      label: '手机号',
      type: 'input',
      placeholder: '请输入手机号',
      required: false,
      rules: [{ type: 'pattern', pattern: '^1[3-9]\\d{9}$', message: '请输入有效的手机号' }],
    },
  },
  {
    key: 'url',
    name: '链接',
    category: 'basic',
    description: 'URL链接输入',
    tags: ['验证'],
    template: {
      key: '',
      label: '链接',
      type: 'input',
      placeholder: 'https://',
      required: false,
      rules: [{ type: 'url', pattern: '^https?://', message: '请输入有效的URL' }],
    },
  },
  // 数字字段
  {
    key: 'number',
    name: '数字',
    category: 'number',
    description: '数字输入框',
    tags: ['常用'],
    template: {
      key: '',
      label: '数量',
      type: 'number',
      placeholder: '请输入',
      required: false,
    },
  },
  {
    key: 'amount',
    name: '金额',
    category: 'number',
    description: '金额输入，保留两位小数',
    tags: ['金额'],
    template: {
      key: '',
      label: '金额',
      type: 'number',
      placeholder: '0.00',
      required: false,
      rules: [{ type: 'number', min: 0, message: '金额不能为负数' }],
    },
  },
  {
    key: 'percent',
    name: '百分比',
    category: 'number',
    description: '百分比输入 (0-100)',
    tags: ['百分比'],
    template: {
      key: '',
      label: '百分比',
      type: 'number',
      placeholder: '0-100',
      required: false,
      rules: [{ type: 'number', min: 0, max: 100, message: '请输入0-100之间的数值' }],
    },
  },
  // 日期时间
  {
    key: 'date',
    name: '日期',
    category: 'datetime',
    description: '日期选择器',
    tags: ['常用'],
    template: {
      key: '',
      label: '日期',
      type: 'date',
      placeholder: '请选择日期',
      required: false,
    },
  },
  {
    key: 'datetime',
    name: '日期时间',
    category: 'datetime',
    description: '日期时间选择器',
    tags: ['常用'],
    template: {
      key: '',
      label: '日期时间',
      type: 'datetime',
      placeholder: '请选择日期时间',
      required: false,
    },
  },
  {
    key: 'date-range',
    name: '日期范围',
    category: 'datetime',
    description: '日期范围选择',
    tags: ['范围'],
    template: {
      key: '',
      label: '日期范围',
      type: 'date',
      placeholder: '请选择',
      required: false,
    },
  },
  // 选择字段
  {
    key: 'select',
    name: '下拉选择',
    category: 'selection',
    description: '下拉单选',
    tags: ['常用'],
    template: {
      key: '',
      label: '类型',
      type: 'select',
      placeholder: '请选择',
      required: false,
      options: [
        { label: '选项1', value: 'option1' },
        { label: '选项2', value: 'option2' },
        { label: '选项3', value: 'option3' },
      ],
    },
  },
  {
    key: 'radio',
    name: '单选框',
    category: 'selection',
    description: '单选按钮组',
    tags: [],
    template: {
      key: '',
      label: '状态',
      type: 'radio',
      required: false,
      options: [
        { label: '启用', value: 'enabled' },
        { label: '禁用', value: 'disabled' },
      ],
    },
  },
  {
    key: 'checkbox',
    name: '复选框',
    category: 'selection',
    description: '多选复选框',
    tags: [],
    template: {
      key: '',
      label: '标签',
      type: 'checkbox',
      required: false,
      options: [
        { label: '标签1', value: 'tag1' },
        { label: '标签2', value: 'tag2' },
        { label: '标签3', value: 'tag3' },
      ],
    },
  },
  {
    key: 'switch',
    name: '开关',
    category: 'selection',
    description: '开关切换',
    tags: [],
    template: {
      key: '',
      label: '启用',
      type: 'switch',
      required: false,
    },
  },
  // 高级字段
  {
    key: 'id',
    name: 'ID字段',
    category: 'advanced',
    description: '只读ID显示',
    tags: ['只读'],
    template: {
      key: 'id',
      label: 'ID',
      type: 'input',
      required: false,
      disabled: true,
    },
  },
  {
    key: 'status-tag',
    name: '状态标签',
    category: 'advanced',
    description: '状态下拉选择（带状态色）',
    tags: ['状态'],
    template: {
      key: 'status',
      label: '状态',
      type: 'select',
      placeholder: '请选择',
      required: false,
      options: [
        { label: '启用', value: 'enabled' },
        { label: '禁用', value: 'disabled' },
        { label: '待审核', value: 'pending' },
      ],
    },
  },
  {
    key: 'create-time',
    name: '创建时间',
    category: 'advanced',
    description: '创建时间（只读）',
    tags: ['只读', '时间'],
    template: {
      key: 'createTime',
      label: '创建时间',
      type: 'datetime',
      required: false,
      disabled: true,
    },
  },
  {
    key: 'update-time',
    name: '更新时间',
    category: 'advanced',
    description: '更新时间（只读）',
    tags: ['只读', '时间'],
    template: {
      key: 'updateTime',
      label: '更新时间',
      type: 'datetime',
      required: false,
      disabled: true,
    },
  },
];

/** 分类颜色 */
const CATEGORY_COLORS: Record<FieldCategory, string> = {
  basic: 'blue',
  number: 'green',
  datetime: 'orange',
  selection: 'purple',
  advanced: 'cyan',
};

/** 分类名称 */
const CATEGORY_NAMES: Record<FieldCategory, string> = {
  basic: '基础字段',
  number: '数字字段',
  datetime: '日期时间',
  selection: '选择字段',
  advanced: '高级字段',
};

export interface FieldLibraryProps {
  /** 字段拖拽开始事件 */
  onFieldDragStart?: (template: FieldTemplate) => void;
  /** 点击添加字段事件 */
  onFieldClick?: (template: FieldTemplate) => void;
  /** 是否启用拖拽 */
  draggable?: boolean;
  /** 折叠面板默认展开的分类 */
  defaultActiveCategories?: FieldCategory[];
}

/** 可拖拽字段项 */
function DraggableFieldItem({
  template,
  draggable = true,
}: {
  template: FieldTemplate;
  draggable?: boolean;
}) {
  const { attributes, listeners, setNodeRef, transform, isDragging } = useSortable({
    id: template.key,
    disabled: !draggable,
  });

  const style: React.CSSProperties = {
    transform: CSS.Transform.toString(transform),
    opacity: isDragging ? 0.5 : 1,
    cursor: draggable ? 'grab' : 'pointer',
  };

  return (
    <div
      ref={setNodeRef}
      style={style}
      {...attributes}
      {...listeners}
      className="field-library-item"
    >
      <Space
        size={8}
        style={{
          width: '100%',
          padding: '8px 12px',
          borderRadius: 4,
          transition: 'background 0.2s',
        }}
        onMouseEnter={(e) => {
          e.currentTarget.style.backgroundColor = '#f5f5f5';
        }}
        onMouseLeave={(e) => {
          e.currentTarget.style.backgroundColor = 'transparent';
        }}
      >
        {draggable && <DragOutlined style={{ color: '#999', fontSize: 12 }} />}
        <Space size={4} style={{ flex: 1 }}>
          <Text strong style={{ fontSize: 13 }}>
            {template.name}
          </Text>
          <Text type="secondary" style={{ fontSize: 12 }}>
            {template.description}
          </Text>
        </Space>
        {template.tags?.slice(0, 2).map((tag) => (
          <Tag key={tag} style={{ fontSize: 11, margin: 0 }}>
            {tag}
          </Tag>
        ))}
      </Space>
    </div>
  );
}

/** 拖拽预览 */
function FieldDragOverlay({ template }: { template: FieldTemplate }) {
  return (
    <div
      style={{
        padding: '8px 12px',
        background: '#fff',
        border: '1px solid #d9d9d9',
        borderRadius: 4,
        boxShadow: '0 4px 12px rgba(0,0,0,0.15)',
        minWidth: 200,
      }}
    >
      <Space>
        <PlusOutlined style={{ color: '#1677ff' }} />
        <Text strong>{template.name}</Text>
      </Space>
    </div>
  );
}

export default function FieldLibrary({
  onFieldDragStart,
  onFieldClick,
  draggable = true,
  defaultActiveCategories = ['basic', 'number'],
}: FieldLibraryProps) {
  const [searchText, setSearchText] = useState('');
  const [activeId, setActiveId] = useState<string | null>(null);
  const sensors = useSensors(
    useSensor(PointerSensor, {
      activationConstraint: {
        distance: 8,
      },
    }),
  );

  // 按分类组织字段模板
  const templatesByCategory = React.useMemo(() => {
    const groups: Record<FieldCategory, FieldTemplate[]> = {
      basic: [],
      number: [],
      datetime: [],
      selection: [],
      advanced: [],
    };
    BUILTIN_FIELD_TEMPLATES.forEach((template) => {
      groups[template.category].push(template);
    });
    return groups;
  }, []);

  // 过滤搜索
  const filteredTemplates = React.useMemo(() => {
    if (!searchText) return templatesByCategory;

    const filtered: Record<FieldCategory, FieldTemplate[]> = {
      basic: [],
      number: [],
      datetime: [],
      selection: [],
      advanced: [],
    };

    const text = searchText.toLowerCase();
    Object.entries(templatesByCategory).forEach(([category, templates]) => {
      filtered[category as FieldCategory] = templates.filter(
        (t) =>
          t.name.toLowerCase().includes(text) ||
          t.description.toLowerCase().includes(text) ||
          t.tags?.some((tag) => tag.toLowerCase().includes(text)),
      );
    });

    return filtered;
  }, [templatesByCategory, searchText]);

  const handleDragStart = (event: DragEndEvent) => {
    setActiveId(event.active.id as string);
  };

  const handleDragEnd = (event: DragEndEvent) => {
    setActiveId(null);
    // 通知外部拖拽开始
    if (onFieldDragStart) {
      const template = BUILTIN_FIELD_TEMPLATES.find((t) => t.key === event.active.id);
      if (template) {
        onFieldDragStart(template);
      }
    }
  };

  const activeTemplate = React.useMemo(() => {
    return activeId ? BUILTIN_FIELD_TEMPLATES.find((t) => t.key === activeId) : null;
  }, [activeId]);

  // 计算是否有搜索结果
  const hasResults = Object.values(filteredTemplates).some((templates) => templates.length > 0);

  // 折叠面板项
  const collapseItems: CollapseProps['items'] = React.useMemo(() => {
    return Object.entries(filteredTemplates)
      .map(([category, templates]) => {
        if (templates.length === 0) return null;
        const cat = category as FieldCategory;
        return {
          key: category,
          label: (
            <Space size={4}>
              <Tag color={CATEGORY_COLORS[cat]}>{CATEGORY_NAMES[cat]}</Tag>
              <Text type="secondary">{templates.length}</Text>
            </Space>
          ),
          children: (
            <SortableContext
              items={templates.map((t) => t.key)}
              strategy={verticalListSortingStrategy}
            >
              {templates.map((template) => (
                <div key={template.key}>
                  <DraggableFieldItem template={template} draggable={draggable} />
                  {onFieldClick && (
                    <Button
                      type="link"
                      size="small"
                      icon={<PlusOutlined />}
                      onClick={() => onFieldClick(template)}
                      style={{ padding: '4px 8px', marginLeft: 28 }}
                    >
                      添加
                    </Button>
                  )}
                </div>
              ))}
            </SortableContext>
          ),
        };
      })
      .filter(Boolean);
  }, [filteredTemplates, draggable, onFieldClick]);

  return (
    <DndContext sensors={sensors} onDragStart={handleDragStart} onDragEnd={handleDragEnd}>
      <Card
        title={
          <Space size={4}>
            <AppstoreOutlined />
            <span>字段库</span>
          </Space>
        }
        size="small"
        extra={
          onFieldClick && (
            <Text type="secondary" style={{ fontSize: 12 }}>
              点击或拖拽添加
            </Text>
          )
        }
      >
        {/* 搜索框 */}
        <Input
          placeholder="搜索字段模板"
          prefix={<SearchOutlined />}
          value={searchText}
          onChange={(e) => setSearchText(e.target.value)}
          allowClear
          style={{ marginBottom: 12 }}
          size="small"
        />

        {!hasResults ? (
          <Empty description="没有找到匹配的字段模板" style={{ padding: '20px 0' }} />
        ) : (
          <Collapse
            defaultActiveKey={defaultActiveCategories}
            size="small"
            ghost
            items={collapseItems}
          />
        )}
      </Card>

      {/* 拖拽预览 */}
      <DragOverlay>{activeTemplate && <FieldDragOverlay template={activeTemplate} />}</DragOverlay>
    </DndContext>
  );
}

/**
 * 从模板生成唯一字段配置
 */
export function createFieldFromTemplate(
  template: FieldTemplate,
  existingKeys: string[],
): FieldConfig {
  // 生成唯一 key
  let baseKey = template.template.key || template.key;
  let uniqueKey = baseKey;
  let counter = 1;

  while (existingKeys.includes(uniqueKey)) {
    uniqueKey = `${baseKey}_${counter}`;
    counter++;
  }

  return {
    ...template.template,
    key: uniqueKey,
    label: template.template.label || template.name,
  } as FieldConfig;
}

/**
 * 获取所有字段模板
 */
export function getAllFieldTemplates(): FieldTemplate[] {
  return BUILTIN_FIELD_TEMPLATES;
}

/**
 * 按分类获取字段模板
 */
export function getFieldTemplatesByCategory(category: FieldCategory): FieldTemplate[] {
  return BUILTIN_FIELD_TEMPLATES.filter((t) => t.category === category);
}
