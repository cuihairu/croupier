/**
 * 属性面板
 *
 * 编辑选中组件的属性。
 *
 * @module pages/WorkspaceEditor/components/CanvasEditor/PropertyPanel
 */

import React, { useCallback, useMemo } from 'react';
import {
  Alert,
  Card,
  Collapse,
  Form,
  Input,
  InputNumber,
  Select,
  Switch,
  ColorPicker,
  Space,
  Divider,
  Typography,
  Button,
  Empty,
} from 'antd';
import { DeleteOutlined, CopyOutlined } from '@ant-design/icons';
import { useCanvasStore } from '../../utils/canvasStoreContext';
import { findComponent, type CanvasComponent } from '../../utils/canvasStore';
import './PropertyPanel.less';

const { Text } = Typography;
const { TextArea } = Input;

export interface PropertyPanelProps {
  /** 组件删除回调 */
  onDelete?: () => void;
  /** 组件复制回调 */
  onCopy?: () => void;
}

/**
 * 基础属性编辑器
 */
function BaseProperties({ component }: { component: CanvasComponent }) {
  const { updateComponent } = useCanvasStore();
  const [form] = Form.useForm();

  const handleValuesChange = useCallback(
    (changed: any, all: any) => {
      const updates: Partial<CanvasComponent> = {};

      // 处理标签
      if (changed.label !== undefined || all.label !== undefined) {
        updates.label = all.label;
      }

      // 处理样式
      if (
        changed.paddingTop !== undefined ||
        changed.paddingBottom !== undefined ||
        changed.paddingLeft !== undefined ||
        changed.paddingRight !== undefined ||
        all.paddingTop !== undefined ||
        all.paddingBottom !== undefined ||
        all.paddingLeft !== undefined ||
        all.paddingRight !== undefined
      ) {
        updates.style = {
          ...component.style,
          ...(all.paddingTop !== undefined && { paddingTop: all.paddingTop }),
          ...(all.paddingBottom !== undefined && { paddingBottom: all.paddingBottom }),
          ...(all.paddingLeft !== undefined && { paddingLeft: all.paddingLeft }),
          ...(all.paddingRight !== undefined && { paddingRight: all.paddingRight }),
        };
      }

      // 处理可见性
      if (changed.visible !== undefined) {
        updates.visible = changed.visible;
      }

      if (Object.keys(updates).length > 0) {
        updateComponent(component.id, updates);
      }
    },
    [component.id, component.style, updateComponent],
  );

  const initialValues = useMemo(
    () => ({
      label: component.label,
      visible: component.visible ?? true,
      paddingTop: component.style?.paddingTop,
      paddingBottom: component.style?.paddingBottom,
      paddingLeft: component.style?.paddingLeft,
      paddingRight: component.style?.paddingRight,
    }),
    [component],
  );

  return (
    <div className="property-section">
      <div className="property-section-title">常用设置</div>
      <Form
        form={form}
        layout="vertical"
        size="small"
        initialValues={initialValues}
        onValuesChange={handleValuesChange}
      >
        <Form.Item label="组件标签" name="label">
          <Input placeholder="输入组件标签" />
        </Form.Item>

        <Form.Item label="可见" name="visible" valuePropName="checked">
          <Switch />
        </Form.Item>
        <Collapse
          ghost
          size="small"
          items={[
            {
              key: 'padding',
              label: '高级样式',
              children: (
                <>
                  <Divider orientation="left" style={{ margin: '12px 0', fontSize: 12 }}>
                    内边距
                  </Divider>
                  <Space size={8} style={{ width: '100%' }}>
                    <Form.Item label="上" name="paddingTop" style={{ marginBottom: 0 }}>
                      <InputNumber size="small" min={0} suffix="px" style={{ width: 70 }} />
                    </Form.Item>
                    <Form.Item label="下" name="paddingBottom" style={{ marginBottom: 0 }}>
                      <InputNumber size="small" min={0} suffix="px" style={{ width: 70 }} />
                    </Form.Item>
                    <Form.Item label="左" name="paddingLeft" style={{ marginBottom: 0 }}>
                      <InputNumber size="small" min={0} suffix="px" style={{ width: 70 }} />
                    </Form.Item>
                    <Form.Item label="右" name="paddingRight" style={{ marginBottom: 0 }}>
                      <InputNumber size="small" min={0} suffix="px" style={{ width: 70 }} />
                    </Form.Item>
                  </Space>
                </>
              ),
            },
          ]}
        />
      </Form>
    </div>
  );
}

/**
 * 字段属性编辑器
 */
function FieldProperties({ component }: { component: CanvasComponent }) {
  const { updateComponent } = useCanvasStore();
  const [form] = Form.useForm();

  const handleValuesChange = useCallback(
    (changed: any, all: any) => {
      const newProps = { ...component.props };

      if (changed.label !== undefined) {
        newProps.label = changed.label;
      }
      if (changed.placeholder !== undefined) {
        newProps.placeholder = changed.placeholder;
      }
      if (changed.defaultValue !== undefined) {
        newProps.defaultValue = changed.defaultValue;
      }
      if (changed.required !== undefined) {
        newProps.required = changed.required;
      }
      if (changed.dataKey !== undefined) {
        updateComponent(component.id, { dataKey: changed.dataKey });
      }

      updateComponent(component.id, { props: newProps });
    },
    [component.id, component.props, updateComponent],
  );

  const initialValues = useMemo(
    () => ({
      ...component.props,
      dataKey: component.dataKey,
    }),
    [component],
  );

  return (
    <div className="property-section">
      <div className="property-section-title">字段设置</div>
      <Form
        form={form}
        layout="vertical"
        size="small"
        initialValues={initialValues}
        onValuesChange={handleValuesChange}
      >
        <Form.Item label="字段标签" name="label">
          <Input placeholder="输入字段标签" />
        </Form.Item>

        <Form.Item label="数据 Key" name="dataKey">
          <Input placeholder="输入字段绑定 key" />
        </Form.Item>

        <Form.Item label="占位提示" name="placeholder">
          <Input placeholder="输入占位文本" />
        </Form.Item>

        <Form.Item label="默认值" name="defaultValue">
          <Input placeholder="输入默认值" />
        </Form.Item>

        <Form.Item label="必填" name="required" valuePropName="checked">
          <Switch />
        </Form.Item>
      </Form>
    </div>
  );
}

/**
 * 按钮属性编辑器
 */
function ButtonProperties({ component }: { component: CanvasComponent }) {
  const { updateComponent } = useCanvasStore();
  const [form] = Form.useForm();

  const handleValuesChange = useCallback(
    (changed: any, all: any) => {
      const newProps = { ...component.props };

      if (changed.text !== undefined) {
        newProps.text = changed.text;
      }
      if (changed.type !== undefined) {
        newProps.type = changed.type;
      }

      updateComponent(component.id, { props: newProps });
    },
    [component.id, component.props, updateComponent],
  );

  const initialValues = useMemo(
    () => ({
      text: component.props?.text || '按钮',
      type: component.props?.type || 'primary',
    }),
    [component.props],
  );

  return (
    <div className="property-section">
      <div className="property-section-title">按钮设置</div>
      <Form
        form={form}
        layout="vertical"
        size="small"
        initialValues={initialValues}
        onValuesChange={handleValuesChange}
      >
        <Form.Item label="按钮文本" name="text">
          <Input placeholder="输入按钮文本" />
        </Form.Item>

        <Form.Item label="按钮类型" name="type">
          <Select>
            <Select.Option value="primary">主要按钮</Select.Option>
            <Select.Option value="default">默认按钮</Select.Option>
            <Select.Option value="dashed">虚线按钮</Select.Option>
            <Select.Option value="link">链接按钮</Select.Option>
            <Select.Option value="text">文本按钮</Select.Option>
          </Select>
        </Form.Item>
      </Form>
    </div>
  );
}

/**
 * 文本属性编辑器
 */
function TextProperties({ component }: { component: CanvasComponent }) {
  const { updateComponent } = useCanvasStore();
  const [form] = Form.useForm();

  const handleValuesChange = useCallback(
    (changed: any) => {
      const newProps = { ...component.props };

      if (changed.content !== undefined) {
        newProps.content = changed.content;
      }

      updateComponent(component.id, { props: newProps });
    },
    [component.id, component.props, updateComponent],
  );

  const initialValues = useMemo(
    () => ({
      content: component.props?.content || '文本内容',
    }),
    [component.props],
  );

  return (
    <div className="property-section">
      <div className="property-section-title">文本设置</div>
      <Form
        form={form}
        layout="vertical"
        size="small"
        initialValues={initialValues}
        onValuesChange={handleValuesChange}
      >
        <Form.Item label="文本内容" name="content">
          <TextArea rows={4} placeholder="输入文本内容" />
        </Form.Item>
      </Form>
    </div>
  );
}

/**
 * 属性面板主组件
 */
export default function PropertyPanel({ onDelete, onCopy }: PropertyPanelProps) {
  const { rootComponent, selectedId, removeComponent } = useCanvasStore();

  const selectedComponent = useMemo(() => {
    if (!rootComponent || !selectedId) return null;
    return findComponent(rootComponent, selectedId);
  }, [rootComponent, selectedId]);

  const handleDelete = useCallback(() => {
    if (selectedId) {
      removeComponent(selectedId);
      if (onDelete) onDelete();
    }
  }, [selectedId, removeComponent, onDelete]);

  const handleCopy = useCallback(() => {
    if (selectedComponent && onCopy) {
      onCopy();
    }
  }, [selectedComponent, onCopy]);

  const componentTypeLabel = useMemo(() => {
    if (!selectedComponent) return '';
    if (selectedComponent.type === 'field') return '字段';
    if (selectedComponent.type === 'button') return '按钮';
    if (selectedComponent.type === 'text') return '文本';
    return selectedComponent.type;
  }, [selectedComponent]);

  const quickSummary = useMemo(() => {
    if (!selectedComponent) return [];
    const summary = [{ color: 'blue', text: componentTypeLabel }];
    if (selectedComponent.dataKey) {
      summary.push({ color: 'default', text: `数据 ${selectedComponent.dataKey}` });
    }
    if (selectedComponent.visible === false) {
      summary.push({ color: 'warning', text: '已隐藏' });
    }
    return summary;
  }, [componentTypeLabel, selectedComponent]);

  if (!selectedComponent) {
    return (
      <Card size="small" title="属性" bordered={false}>
        <Empty
          image={Empty.PRESENTED_IMAGE_SIMPLE}
          description="先在画布中选择一个组件，再编辑它的常用属性"
          style={{ padding: '20px 0' }}
        />
      </Card>
    );
  }

  return (
    <div className="property-panel">
      <Card
        size="small"
        title={
          <Space>
            <span>属性</span>
            <Text type="secondary" style={{ fontSize: 12 }}>
              {selectedComponent.label || selectedComponent.type}
            </Text>
          </Space>
        }
        bordered={false}
        extra={
          <Space size={4}>
            <Button
              type="text"
              size="small"
              icon={<CopyOutlined />}
              onClick={handleCopy}
              title="复制"
            />
            <Button
              type="text"
              size="small"
              icon={<DeleteOutlined />}
              onClick={handleDelete}
              danger
              title="删除"
            />
          </Space>
        }
      >
        <div className="property-content">
          <Space wrap size={[8, 8]} style={{ marginBottom: 8 }}>
            {quickSummary.map((item) => (
              <Button key={`${item.color}-${item.text}`} size="small" type="default">
                {item.text}
              </Button>
            ))}
          </Space>

          <Alert
            type="info"
            showIcon
            style={{ marginBottom: 12 }}
            message="先改常用项，再展开高级设置"
            description="建议优先处理标签、数据绑定和是否可见。间距等样式项已收进高级区域，避免影响主要编辑流程。"
          />

          <BaseProperties component={selectedComponent} />

          {selectedComponent.type === 'field' && <FieldProperties component={selectedComponent} />}

          {selectedComponent.type === 'button' && (
            <ButtonProperties component={selectedComponent} />
          )}

          {selectedComponent.type === 'text' && <TextProperties component={selectedComponent} />}
        </div>
      </Card>
    </div>
  );
}
