/**
 * 组件库面板
 *
 * 提供可拖拽的组件模板。
 *
 * @module pages/WorkspaceEditor/components/CanvasEditor/ComponentLibrary
 */

import React, { useMemo } from 'react';
import { Card, Collapse, Typography, Tag } from 'antd';
import type { CollapseProps } from 'antd';
import { useCanvasStore } from '../../utils/canvasStoreContext';
import { createComponentInstance, type CanvasComponentTemplate } from '../../utils/canvasStore';
import './ComponentLibrary.less';

const { Text } = Typography;

export interface ComponentLibraryProps {
  /** 分类展开状态 */
  defaultActiveKey?: string | string[];
}

/**
 * 组件项
 */
function ComponentItem({ template }: { template: CanvasComponentTemplate }) {
  const { setDraggingComponent } = useCanvasStore();

  const handleDragStart = (e: React.DragEvent) => {
    const component = createComponentInstance(template);
    setDraggingComponent(component);
    e.dataTransfer.effectAllowed = 'copy';
  };

  const handleDragEnd = () => {
    setDraggingComponent(null);
  };

  return (
    <div
      className="component-library-item"
      draggable
      onDragStart={handleDragStart}
      onDragEnd={handleDragEnd}
    >
      <span className="component-icon">{template.icon}</span>
      <span className="component-label">{template.label}</span>
    </div>
  );
}

/**
 * 组件分类面板
 */
function ComponentCategory({
  title,
  templates,
  icon,
}: {
  title: string;
  templates: CanvasComponentTemplate[];
  icon: string;
}) {
  return (
    <div className="component-category">
      <div className="category-header">
        <span className="category-icon">{icon}</span>
        <span className="category-title">{title}</span>
        <Tag className="category-count">{templates.length}</Tag>
      </div>
      <div className="category-items">
        {templates.map((template, index) => (
          <ComponentItem key={`${template.type}-${index}`} template={template} />
        ))}
      </div>
    </div>
  );
}

/**
 * 组件库主组件
 */
export default function ComponentLibrary({
  defaultActiveKey = ['layout', 'form'],
}: ComponentLibraryProps) {
  const { componentTemplates } = useCanvasStore();

  // 按分类分组模板
  const categorizedTemplates = useMemo(() => {
    const categories: Record<string, CanvasComponentTemplate[]> = {
      layout: [],
      form: [],
      display: [],
      other: [],
    };

    componentTemplates.forEach((template) => {
      if (categories[template.category]) {
        categories[template.category].push(template);
      } else {
        categories.other.push(template);
      }
    });

    return categories;
  }, [componentTemplates]);

  // 折叠面板项
  const collapseItems: CollapseProps['items'] = useMemo(() => {
    const items: CollapseProps['items'] = [
      {
        key: 'layout',
        label: '布局组件',
        children: (
          <ComponentCategory title="布局组件" templates={categorizedTemplates.layout} icon="📐" />
        ),
      },
      {
        key: 'form',
        label: '表单组件',
        children: (
          <ComponentCategory title="表单组件" templates={categorizedTemplates.form} icon="📋" />
        ),
      },
      {
        key: 'display',
        label: '展示组件',
        children: (
          <ComponentCategory title="展示组件" templates={categorizedTemplates.display} icon="👁️" />
        ),
      },
    ];

    if (categorizedTemplates.other.length > 0) {
      items.push({
        key: 'other',
        label: '其他组件',
        children: (
          <ComponentCategory title="其他组件" templates={categorizedTemplates.other} icon="🔧" />
        ),
      });
    }

    return items;
  }, [categorizedTemplates]);

  return (
    <div className="component-library">
      <Card size="small" title="组件库" bordered={false}>
        <Collapse defaultActiveKey={defaultActiveKey} ghost size="small" items={collapseItems} />

        <div className="library-tips">
          <Text type="secondary" style={{ fontSize: 12 }}>
            💡 拖拽组件到画布添加
          </Text>
        </div>
      </Card>
    </div>
  );
}

/**
 * 紧凑模式组件库（用于工具栏）
 */
export function CompactComponentLibrary({
  onDragStart,
}: {
  onDragStart?: (template: CanvasComponentTemplate) => void;
}) {
  const { componentTemplates } = useCanvasStore();

  const handleDragStart = (template: CanvasComponentTemplate, e: React.DragEvent) => {
    e.dataTransfer.effectAllowed = 'copy';
    if (onDragStart) {
      onDragStart(template);
    }
  };

  return (
    <div className="compact-component-library">
      {componentTemplates.map((template) => (
        <div
          key={`${template.type}-${template.label}`}
          className="compact-component-item"
          draggable
          onDragStart={(e) => handleDragStart(template, e)}
          title={template.label}
        >
          {template.icon}
        </div>
      ))}
    </div>
  );
}
