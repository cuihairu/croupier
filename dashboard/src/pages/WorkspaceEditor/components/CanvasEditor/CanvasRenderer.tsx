/**
 * 画布渲染器
 *
 * 渲染可拖拽、可选择的画布组件。
 *
 * @module pages/WorkspaceEditor/components/CanvasEditor/CanvasRenderer
 */

import React, { useCallback, useMemo } from 'react';
import { Card, Input, Button, Select, DatePicker, Switch, Divider, Space, Typography } from 'antd';
import { useCanvasStore } from '../../utils/canvasStoreContext';
import { type CanvasComponent } from '../../utils/canvasStore';
import './CanvasRenderer.less';

const { Text } = Typography;

export interface CanvasRendererProps {
  /** 根组件 */
  component: CanvasComponent;
  /** 是否可编辑 */
  editable?: boolean;
  /** 是否渲染预览模式 */
  preview?: boolean;
}

/**
 * 渲染单个组件
 */
function RenderComponent({
  component,
  depth = 0,
  editable = true,
  preview = false,
}: {
  component: CanvasComponent;
  depth?: number;
  editable?: boolean;
  preview?: boolean;
}) {
  const { selectedId, hoveredId, selectComponent, hoverComponent, updateComponent } =
    useCanvasStore();

  const isSelected = selectedId === component.id;
  const isHovered = hoveredId === component.id;

  // 点击选中
  const handleClick = useCallback(
    (e: React.MouseEvent) => {
      e.stopPropagation();
      if (editable) {
        selectComponent(component.id);
      }
    },
    [component.id, editable, selectComponent],
  );

  // 鼠标悬停
  const handleMouseEnter = useCallback(() => {
    if (editable) {
      hoverComponent(component.id);
    }
  }, [component.id, editable, hoverComponent]);

  const handleMouseLeave = useCallback(() => {
    hoverComponent(null);
  }, [hoverComponent]);

  // 渲染组件内容
  const renderContent = () => {
    switch (component.type) {
      case 'container':
        return (
          <div className="canvas-container" style={component.style}>
            {component.children?.map((child) => (
              <RenderComponent
                key={child.id}
                component={child}
                depth={depth + 1}
                editable={editable}
                preview={preview}
              />
            ))}
          </div>
        );

      case 'section':
        return (
          <div className="canvas-section" style={component.style}>
            <div className="canvas-section-title">
              {component.props?.title || component.label || '分区'}
            </div>
            {component.children?.map((child) => (
              <RenderComponent
                key={child.id}
                component={child}
                depth={depth + 1}
                editable={editable}
                preview={preview}
              />
            ))}
          </div>
        );

      case 'row':
        return (
          <div className="canvas-row" style={component.style}>
            {component.children?.map((child) => (
              <RenderComponent
                key={child.id}
                component={child}
                depth={depth + 1}
                editable={editable}
                preview={preview}
              />
            ))}
          </div>
        );

      case 'col':
        return (
          <div
            className="canvas-col"
            style={{
              ...component.style,
              flex: component.span || 1,
            }}
          >
            {component.children?.map((child) => (
              <RenderComponent
                key={child.id}
                component={child}
                depth={depth + 1}
                editable={editable}
                preview={preview}
              />
            ))}
          </div>
        );

      case 'field':
        if (preview) {
          return renderFieldPreview(component);
        }
        return (
          <div className="canvas-field" style={component.style}>
            <FieldPreview component={component} />
          </div>
        );

      case 'button':
        if (preview) {
          return (
            <Button type={component.props?.type || 'primary'}>
              {component.props?.text || '按钮'}
            </Button>
          );
        }
        return (
          <div className="canvas-button" style={component.style}>
            <Button type={component.props?.type || 'primary'} disabled>
              {component.props?.text || '按钮'}
            </Button>
          </div>
        );

      case 'text':
        return (
          <div className="canvas-text" style={component.style}>
            <Text>{component.props?.content || '文本内容'}</Text>
          </div>
        );

      case 'divider':
        return <Divider style={component.style} />;

      case 'spacer':
        return (
          <div
            className="canvas-spacer"
            style={{
              height: component.props?.height || 16,
              ...component.style,
            }}
          />
        );

      default:
        return <div className="canvas-unknown">未知组件: {component.type}</div>;
    }
  };

  // 包装层：添加选择框和操作提示
  if (!editable && !preview) {
    return <>{renderContent()}</>;
  }

  return (
    <div
      className={`canvas-component-wrapper ${isSelected ? 'selected' : ''} ${
        isHovered ? 'hovered' : ''
      }`}
      onClick={handleClick}
      onMouseEnter={handleMouseEnter}
      onMouseLeave={handleMouseLeave}
      style={{ position: 'relative' }}
    >
      {renderContent()}
      {!preview && (
        <div className="canvas-component-overlay">
          <div className="canvas-component-label">{component.label || component.type}</div>
          {isSelected && (
            <div className="canvas-component-handles">
              <div className="handle handle-tl" />
              <div className="handle handle-tr" />
              <div className="handle handle-bl" />
              <div className="handle handle-br" />
            </div>
          )}
        </div>
      )}
    </div>
  );
}

/**
 * 字段预览
 */
function FieldPreview({ component }: { component: CanvasComponent }) {
  const fieldType = component.props?.type || 'input';

  switch (fieldType) {
    case 'input':
      return (
        <div>
          <div className="field-label">
            {component.props?.label || '输入框'}
            {component.props?.required && <span className="required">*</span>}
          </div>
          <Input placeholder={component.props?.placeholder} disabled />
        </div>
      );

    case 'number':
      return (
        <div>
          <div className="field-label">
            {component.props?.label || '数字'}
            {component.props?.required && <span className="required">*</span>}
          </div>
          <Input type="number" disabled />
        </div>
      );

    case 'textarea':
      return (
        <div>
          <div className="field-label">
            {component.props?.label || '多行文本'}
            {component.props?.required && <span className="required">*</span>}
          </div>
          <Input.TextArea rows={4} disabled />
        </div>
      );

    case 'select':
      return (
        <div>
          <div className="field-label">
            {component.props?.label || '下拉选择'}
            {component.props?.required && <span className="required">*</span>}
          </div>
          <Select disabled style={{ width: '100%' }} placeholder="请选择" />
        </div>
      );

    case 'date':
      return (
        <div>
          <div className="field-label">
            {component.props?.label || '日期'}
            {component.props?.required && <span className="required">*</span>}
          </div>
          <DatePicker disabled style={{ width: '100%' }} />
        </div>
      );

    case 'daterange':
      return (
        <div>
          <div className="field-label">
            {component.props?.label || '日期范围'}
            {component.props?.required && <span className="required">*</span>}
          </div>
          <DatePicker.RangePicker disabled style={{ width: '100%' }} />
        </div>
      );

    case 'switch':
      return (
        <div className="field-switch">
          <span className="field-label">{component.props?.label || '开关'}</span>
          <Switch disabled />
        </div>
      );

    case 'checkbox':
      return (
        <div className="field-checkbox">
          <span className="field-label">{component.props?.label || '复选框'}</span>
          <input type="checkbox" disabled />
        </div>
      );

    case 'radio':
      return (
        <div>
          <div className="field-label">
            {component.props?.label || '单选框'}
            {component.props?.required && <span className="required">*</span>}
          </div>
          <Space>
            <label>
              <input type="radio" name={component.id} disabled /> 选项1
            </label>
            <label>
              <input type="radio" name={component.id} disabled /> 选项2
            </label>
          </Space>
        </div>
      );

    default:
      return (
        <div>
          <div className="field-label">{component.props?.label || '字段'}</div>
          <Input disabled />
        </div>
      );
  }
}

/**
 * 渲染字段预览（实际渲染）
 */
function renderFieldPreview(component: CanvasComponent): React.ReactNode {
  const fieldType = component.props?.type || 'input';
  const required = component.props?.required;

  const renderLabel = () => (
    <div className="field-label">
      {component.props?.label || component.label || '字段'}
      {required && <span className="required">*</span>}
    </div>
  );

  switch (fieldType) {
    case 'textarea':
      return (
        <div>
          {renderLabel()}
          <Input.TextArea
            rows={component.props?.rows || 4}
            placeholder={component.props?.placeholder}
          />
        </div>
      );

    case 'number':
      return (
        <div>
          {renderLabel()}
          <Input.Number
            style={{ width: '100%' }}
            placeholder={component.props?.placeholder}
            min={component.props?.min}
            max={component.props?.max}
          />
        </div>
      );

    case 'select':
      return (
        <div>
          {renderLabel()}
          <Select
            style={{ width: '100%' }}
            placeholder={component.props?.placeholder || '请选择'}
            options={component.props?.options || []}
          />
        </div>
      );

    case 'date':
      return (
        <div>
          {renderLabel()}
          <DatePicker style={{ width: '100%' }} />
        </div>
      );

    case 'daterange':
      return (
        <div>
          {renderLabel()}
          <DatePicker.RangePicker style={{ width: '100%' }} />
        </div>
      );

    case 'switch':
      return (
        <div className="field-switch">
          <span className="field-label">{component.props?.label || component.label}</span>
          <Switch />
        </div>
      );

    case 'checkbox':
      return (
        <div className="field-checkbox">
          <input type="checkbox" id={component.id} />
          <label htmlFor={component.id}>{component.props?.label || component.label}</label>
        </div>
      );

    default:
      return (
        <div>
          {renderLabel()}
          <Input placeholder={component.props?.placeholder} />
        </div>
      );
  }
}

/**
 * 画布渲染器主组件
 */
export default function CanvasRenderer({
  component,
  editable = true,
  preview = false,
}: CanvasRendererProps) {
  return (
    <div className="canvas-renderer">
      <RenderComponent component={component} editable={editable} preview={preview} />
    </div>
  );
}
