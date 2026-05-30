/**
 * 表单-详情布局渲染器
 *
 * 先通过表单查询，然后显示详情和操作按钮。
 *
 * @module components/WorkspaceRenderer/renderers/FormDetailRenderer
 */

import React, { useState } from 'react';
import {
  Card,
  Form,
  Input,
  InputNumber,
  Select,
  Button,
  Space,
  Descriptions,
  message,
  Badge,
  Tag,
  Modal,
  Drawer,
  Popconfirm,
  Alert,
} from 'antd';
import type { FormDetailLayout, ActionConfig } from '@/types/workspace';
import { invokeFunction } from '@/services/functionInvoke';
import * as Icons from '@ant-design/icons';
import type { RendererProps } from './types';
import {
  RendererEmpty,
  RendererError,
  RendererModeNotice,
  isTemplatePreviewContext,
} from './state';

export type FormDetailRendererProps = RendererProps<FormDetailLayout>;

/**
 * 表单-详情布局渲染器组件
 */
export default function FormDetailRenderer({
  layout,
  objectKey,
  context,
}: FormDetailRendererProps) {
  const isTemplatePreview = isTemplatePreviewContext(context);
  const [form] = Form.useForm();
  const [actionForm] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const [actionLoading, setActionLoading] = useState(false);
  const [detailData, setDetailData] = useState<any>(null);
  const [queryError, setQueryError] = useState('');
  const [modalVisible, setModalVisible] = useState(false);
  const [drawerVisible, setDrawerVisible] = useState(false);
  const [currentAction, setCurrentAction] = useState<ActionConfig | null>(null);
  const [hasQueried, setHasQueried] = useState(false);
  const effectiveQueryFields = React.useMemo(() => {
    const fields = Array.isArray(layout.queryFields) ? layout.queryFields : [];
    if (fields.length > 0) return fields;
    if (!isTemplatePreview) return fields;
    return [{ key: 'keyword', label: '关键字', type: 'input', required: true }] as any[];
  }, [isTemplatePreview, layout.queryFields]);
  const effectiveDetailSections = React.useMemo(() => {
    const sections = Array.isArray(layout.detailSections) ? layout.detailSections : [];
    if (sections.length > 0) return sections;
    if (!isTemplatePreview) return sections;
    return [
      {
        title: '详情信息',
        column: 2,
        fields: [
          { key: 'id', label: 'ID' },
          { key: 'name', label: '名称' },
          { key: 'status', label: '状态' },
        ],
      },
    ] as any[];
  }, [isTemplatePreview, layout.detailSections]);
  const previewDetailData = React.useMemo(() => {
    if (!isTemplatePreview) return null;
    const obj: Record<string, any> = {};
    (layout.detailSections || []).forEach((section: any) => {
      (section?.fields || []).forEach((field: any) => {
        if (!field?.key) return;
        obj[field.key] = `${field.label || field.key}示例值`;
      });
    });
    if (Object.keys(obj).length === 0) {
      obj.id = 'demo-001';
      obj.name = '示例对象';
    }
    return obj;
  }, [effectiveDetailSections, isTemplatePreview]);

  // 处理查询
  const handleQuery = async (values: any) => {
    if (!layout.queryFunction) {
      if (isTemplatePreview) {
        setHasQueried(true);
        setDetailData(previewDetailData || null);
        message.info('模板预览模式：查询函数待绑定');
        return;
      }
      message.error('未配置查询函数');
      return;
    }

    setHasQueried(true);
    setQueryError('');
    setLoading(true);
    try {
      const result = await invokeFunction(layout.queryFunction, values);
      setDetailData(result);
      message.success('查询成功');
    } catch (error: any) {
      setQueryError(error?.message || '查询失败');
      message.error(error.message || '查询失败');
      setDetailData(null);
    } finally {
      setLoading(false);
    }
  };

  // 执行函数调用
  const executeAction = async (action: ActionConfig, extraParams?: Record<string, any>) => {
    setActionLoading(true);
    try {
      await invokeFunction(action.function, { ...detailData, ...extraParams });
      message.success(`${action.label}成功`);
      form.submit();
    } catch (error: any) {
      message.error(error.message || `${action.label}失败`);
    } finally {
      setActionLoading(false);
    }
  };

  // 处理操作按钮点击
  const handleAction = (action: ActionConfig) => {
    if (!action.function) {
      message.error('未配置操作函数');
      return;
    }

    setCurrentAction(action);

    switch (action.type) {
      case 'modal':
        actionForm.resetFields();
        setModalVisible(true);
        break;
      case 'drawer':
        actionForm.resetFields();
        setDrawerVisible(true);
        break;
      case 'popconfirm':
        // popconfirm 在按钮上直接处理
        break;
      case 'direct':
      default:
        executeAction(action);
        break;
    }
  };

  // 提交 modal/drawer 表单
  const handleFormSubmit = async () => {
    if (!currentAction) return;
    try {
      const values = await actionForm.validateFields();
      await executeAction(currentAction, values);
      setModalVisible(false);
      setDrawerVisible(false);
    } catch {
      // 表单验证失败
    }
  };

  // 自动查询
  React.useEffect(() => {
    if (isTemplatePreview) {
      setHasQueried(true);
      setDetailData(previewDetailData || null);
      return;
    }
    if (layout.autoQuery) {
      setHasQueried(true);
      form.submit();
    }
  }, [layout.autoQuery, isTemplatePreview, previewDetailData]);

  if (!layout.queryFunction && !isTemplatePreview) {
    return (
      <>
        <RendererModeNotice
          context={context}
          sampleTitle="查询详情预览"
          sampleDescription="当前查询详情页正在使用示例查询条件和示例详情帮助你预览主路径，正式运行仍需要真实 queryFunction 与真实结果。"
        />
        <RendererEmpty description="当前查询未绑定函数，请在布局配置中选择 queryFunction" />
      </>
    );
  }

  return (
    <div>
      <RendererModeNotice
        context={context}
        sampleTitle="查询详情预览"
        sampleDescription="当前查询详情页正在使用示例查询条件和示例详情帮助你预览主路径，正式运行仍需要真实 queryFunction 与真实结果。"
      />
      {!layout.queryFunction && isTemplatePreview && (
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 12 }}
          message="模板预览"
          description="当前查询未绑定函数，应用模板后可在编辑器中绑定。"
        />
      )}
      {queryError && <RendererError message="查询失败" description={queryError} />}
      {/* 查询区 */}
      <Card title="查询条件" style={{ marginBottom: 16 }}>
        <Form form={form} layout="inline" onFinish={handleQuery}>
          {(effectiveQueryFields || []).map((field) => (
            <Form.Item
              key={field.key}
              name={field.key}
              label={field.label}
              rules={[{ required: field.required, message: `请输入${field.label}` }]}
              tooltip={field.tooltip}
            >
              {renderField(field)}
            </Form.Item>
          ))}
          <Form.Item>
            <Button type="primary" htmlType="submit" loading={loading}>
              查询
            </Button>
          </Form.Item>
        </Form>
      </Card>

      {/* 详情区 */}
      {detailData && (
        <>
          {(effectiveDetailSections || []).map((section, index) => (
            <Card key={index} title={section.title} style={{ marginBottom: 16 }}>
              <Descriptions column={section.column || 2}>
                {section.fields.map((field) => (
                  <Descriptions.Item key={field.key} label={field.label} span={field.span}>
                    {renderDetailField(field, detailData[field.key])}
                  </Descriptions.Item>
                ))}
              </Descriptions>
            </Card>
          ))}

          {/* 操作区 */}
          {layout.actions && layout.actions.length > 0 && (
            <Card title="操作">
              <Space>
                {layout.actions.map((action) =>
                  action.type === 'popconfirm' ? (
                    <Popconfirm
                      key={action.key}
                      title={action.confirmMessage || `确认执行"${action.label}"？`}
                      onConfirm={() => executeAction(action)}
                      okButtonProps={{ danger: action.danger, loading: actionLoading }}
                      disabled={action.disabled}
                    >
                      <Button
                        type={action.buttonType || 'default'}
                        danger={action.danger}
                        icon={getIcon(action.icon)}
                        disabled={action.disabled}
                      >
                        {action.label}
                      </Button>
                    </Popconfirm>
                  ) : (
                    <Button
                      key={action.key}
                      type={action.buttonType || 'default'}
                      danger={action.danger}
                      icon={getIcon(action.icon)}
                      onClick={() => handleAction(action)}
                      disabled={action.disabled}
                      loading={actionLoading && currentAction?.key === action.key}
                    >
                      {action.label}
                    </Button>
                  ),
                )}
              </Space>
            </Card>
          )}
        </>
      )}
      {hasQueried && !loading && !detailData && !queryError && (
        <RendererEmpty description={isTemplatePreview ? '当前查询详情预览暂无示例结果' : '未查询到数据'} />
      )}

      <Modal
        title={currentAction?.label}
        open={modalVisible}
        onOk={handleFormSubmit}
        onCancel={() => setModalVisible(false)}
        confirmLoading={actionLoading}
      >
        <Form form={actionForm} layout="vertical">
          {(currentAction?.fields || []).map((field) => (
            <Form.Item
              key={field.key}
              name={field.key}
              label={field.label}
              rules={[{ required: field.required, message: `请输入${field.label}` }]}
            >
              {renderField(field)}
            </Form.Item>
          ))}
        </Form>
      </Modal>

      <Drawer
        title={currentAction?.label}
        open={drawerVisible}
        onClose={() => setDrawerVisible(false)}
        extra={
          <Button type="primary" loading={actionLoading} onClick={handleFormSubmit}>
            提交
          </Button>
        }
      >
        <Form form={actionForm} layout="vertical">
          {(currentAction?.fields || []).map((field) => (
            <Form.Item
              key={field.key}
              name={field.key}
              label={field.label}
              rules={[{ required: field.required, message: `请输入${field.label}` }]}
            >
              {renderField(field)}
            </Form.Item>
          ))}
        </Form>
      </Drawer>
    </div>
  );
}

/**
 * 渲染表单字段
 */
function renderField(field: any): React.ReactNode {
  switch (field.type) {
    case 'input':
      return <Input placeholder={field.placeholder} disabled={field.disabled} />;

    case 'number':
      return (
        <InputNumber
          placeholder={field.placeholder}
          disabled={field.disabled}
          style={{ width: '100%' }}
        />
      );

    case 'select':
      return (
        <Select placeholder={field.placeholder} disabled={field.disabled} style={{ width: 200 }}>
          {(field.options || []).map((opt: any) => (
            <Select.Option key={opt.value} value={opt.value}>
              {opt.label}
            </Select.Option>
          ))}
        </Select>
      );

    case 'textarea':
      return <Input.TextArea rows={4} placeholder={field.placeholder} disabled={field.disabled} />;

    default:
      return <Input placeholder={field.placeholder} disabled={field.disabled} />;
  }
}

/**
 * 渲染详情字段
 */
function renderDetailField(field: any, value: any): React.ReactNode {
  if (!value && value !== 0 && value !== false) return '-';

  const renderType = field.render || 'text';
  const options = field.renderOptions || {};

  switch (renderType) {
    case 'status':
      return renderStatus(value, options);

    case 'datetime':
      return renderDateTime(value, options);

    case 'date':
      return renderDate(value, options);

    case 'tag':
      return renderTag(value, options);

    case 'money':
      return renderMoney(value, options);

    case 'link':
      return renderLink(value, options);

    case 'image':
      return renderImage(value, options);

    case 'text':
    default:
      return value;
  }
}

/**
 * 渲染状态
 */
function renderStatus(value: any, options: any): React.ReactNode {
  const statusMap = options.statusMap || {
    1: { text: '启用', status: 'success' },
    0: { text: '禁用', status: 'default' },
  };

  const status = statusMap[value];
  if (!status) return value;

  return <Badge status={status.status} text={status.text} />;
}

/**
 * 渲染日期时间
 */
function renderDateTime(value: any, options: any): React.ReactNode {
  if (!value) return '-';
  const date = new Date(value);
  return date.toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  });
}

/**
 * 渲染日期
 */
function renderDate(value: any, options: any): React.ReactNode {
  if (!value) return '-';
  const date = new Date(value);
  return date.toLocaleDateString('zh-CN');
}

/**
 * 渲染标签
 */
function renderTag(value: any, options: any): React.ReactNode {
  const color = options.tagColor || 'blue';
  return <Tag color={typeof color === 'function' ? color(value) : color}>{value}</Tag>;
}

/**
 * 渲染金额
 */
function renderMoney(value: any, options: any): React.ReactNode {
  const symbol = options.currencySymbol || '¥';
  const precision = options.currencyPrecision || 2;
  return `${symbol}${Number(value).toFixed(precision)}`;
}

/**
 * 渲染链接
 */
function renderLink(value: any, options: any): React.ReactNode {
  const target = options.linkTarget || '_blank';
  return (
    <a href={value} target={target} rel="noopener noreferrer">
      {value}
    </a>
  );
}

/**
 * 渲染图片
 */
function renderImage(value: any, options: any): React.ReactNode {
  return <img src={value} alt="" style={{ width: 100, height: 100, objectFit: 'cover' }} />;
}

/**
 * 根据图标名称获取图标组件
 */
function getIcon(iconName?: string): React.ReactNode {
  if (!iconName) return null;
  const IconComponent = (Icons as any)[iconName];
  return IconComponent ? <IconComponent /> : null;
}
