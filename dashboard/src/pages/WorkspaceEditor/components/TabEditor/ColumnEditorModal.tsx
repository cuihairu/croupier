import React from 'react';
import { Modal, Form, Input, Select, Switch, Divider, Space, InputNumber } from 'antd';
import type { ColumnConfig } from '@/types/workspace';
import { HelpTooltip } from '../HelpTooltip';

export interface ColumnEditorModalProps {
  open: boolean;
  editingColumn: ColumnConfig | null;
  onOk: (values: ColumnConfig) => void;
  onCancel: () => void;
}

const BUILTIN_RENDER_TYPES = [
  { value: 'text', label: '文本' },
  { value: 'status', label: '状态标签' },
  { value: 'datetime', label: '日期时间' },
  { value: 'date', label: '日期' },
  { value: 'tag', label: '标签' },
  { value: 'money', label: '金额' },
  { value: 'link', label: '链接' },
  { value: 'image', label: '图片' },
];

const BUILTIN_RENDER_VALUES = BUILTIN_RENDER_TYPES.map((r) => r.value);

export default function ColumnEditorModal({
  open,
  editingColumn,
  onOk,
  onCancel,
}: ColumnEditorModalProps) {
  const [form] = Form.useForm();
  const renderType = Form.useWatch('renderType', form);

  React.useEffect(() => {
    if (open && editingColumn) {
      const render = editingColumn.render;
      const isBuiltin = !render || BUILTIN_RENDER_VALUES.includes(render as string);
      form.setFieldsValue({
        ...editingColumn,
        renderType: isBuiltin ? render || undefined : 'expression',
        renderExpression: isBuiltin ? undefined : render,
      });
    } else if (open) {
      form.resetFields();
    }
  }, [open, editingColumn, form]);

  const handleOk = async () => {
    const values = await form.validateFields();
    const { renderType: rt, renderExpression, ...rest } = values;
    const render = rt === 'expression' ? renderExpression : rt;
    onOk({ ...rest, render });
    form.resetFields();
  };

  const handleCancel = () => {
    form.resetFields();
    onCancel();
  };

  return (
    <Modal
      title={editingColumn ? '编辑列' : '添加列'}
      open={open}
      onOk={handleOk}
      onCancel={handleCancel}
      width={560}
    >
      <Form form={form} layout="vertical" size="small">
        <Form.Item
          name="key"
          label={
            <Space size={4}>
              <span>字段名</span>
              <HelpTooltip helpKey="column.key" />
            </Space>
          }
          rules={[{ required: true, message: '请输入字段名' }]}
        >
          <Input placeholder="如: playerId" />
        </Form.Item>
        <Form.Item
          name="title"
          label={
            <Space size={4}>
              <span>列标题</span>
              <HelpTooltip helpKey="column.title" />
            </Space>
          }
          rules={[{ required: true, message: '请输入列标题' }]}
        >
          <Input placeholder="如: 玩家ID" />
        </Form.Item>
        <Form.Item
          name="renderType"
          label={
            <Space size={4}>
              <span>渲染方式</span>
              <HelpTooltip helpKey="column.render" />
            </Space>
          }
        >
          <Select allowClear placeholder="默认文本">
            {BUILTIN_RENDER_TYPES.map((r) => (
              <Select.Option key={r.value} value={r.value}>
                {r.label}
              </Select.Option>
            ))}
            <Select.Option value="expression">自定义表达式</Select.Option>
          </Select>
        </Form.Item>
        {renderType === 'expression' && (
          <Form.Item
            name="renderExpression"
            label="渲染表达式"
            rules={[{ required: true, message: '请输入渲染表达式' }]}
          >
            <Input.TextArea rows={2} placeholder={'如: ${status === 1 ? "启用" : "禁用"}'} />
          </Form.Item>
        )}
        <Form.Item
          name="width"
          label={
            <Space size={4}>
              <span>列宽</span>
              <HelpTooltip helpKey="column.width" />
            </Space>
          }
        >
          <InputNumber min={40} max={800} placeholder="如: 120" style={{ width: '100%' }} />
        </Form.Item>
        <Form.Item
          name="fixed"
          label={
            <Space size={4}>
              <span>固定方向</span>
              <HelpTooltip helpKey="column.fixed" />
            </Space>
          }
        >
          <Select allowClear placeholder="不固定">
            <Select.Option value="left">左固定</Select.Option>
            <Select.Option value="right">右固定</Select.Option>
          </Select>
        </Form.Item>
        <Form.Item
          name="align"
          label={
            <Space size={4}>
              <span>对齐方式</span>
              <HelpTooltip helpKey="column.align" />
            </Space>
          }
        >
          <Select allowClear placeholder="默认左对齐">
            <Select.Option value="left">左对齐</Select.Option>
            <Select.Option value="center">居中</Select.Option>
            <Select.Option value="right">右对齐</Select.Option>
          </Select>
        </Form.Item>
        <Form.Item
          name="ellipsis"
          label={
            <Space size={4}>
              <span>超长省略</span>
              <HelpTooltip helpKey="column.ellipsis" />
            </Space>
          }
          valuePropName="checked"
        >
          <Switch checkedChildren="开启" unCheckedChildren="关闭" />
        </Form.Item>
        <Form.Item
          name="sortable"
          label={
            <Space size={4}>
              <span>可排序</span>
              <HelpTooltip helpKey="column.sortable" />
            </Space>
          }
          valuePropName="checked"
        >
          <Switch />
        </Form.Item>

        {/* 条件显隐 */}
        <Divider plain style={{ margin: '8px 0' }}>
          条件显隐
        </Divider>
        <Form.Item
          name="visiblePermission"
          label={
            <Space size={4}>
              <span>权限控制</span>
              <HelpTooltip helpKey="column.visiblePermission" />
            </Space>
          }
        >
          <Input placeholder="如: workspace:edit（留空表示不限制）" />
        </Form.Item>
        <Form.Item
          name="visibleCondition"
          label={
            <Space size={4}>
              <span>数据条件</span>
              <HelpTooltip helpKey="column.visibleCondition" />
            </Space>
          }
        >
          <Input.TextArea rows={2} placeholder={"如: context.status !== 'archived'"} />
        </Form.Item>
      </Form>
    </Modal>
  );
}
