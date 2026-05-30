/**
 * 详情布局渲染器
 *
 * 显示详情信息，只读。
 *
 * @module components/WorkspaceRenderer/renderers/DetailRenderer
 */

import React, { useEffect, useState } from 'react';
import { Card, Descriptions, message, Badge, Tag, Button, Space } from 'antd';
import type { DetailLayout } from '@/types/workspace';
import { invokeFunction } from '@/services/functionInvoke';
import * as Icons from '@ant-design/icons';
import type { RendererProps } from './types';
import {
  RendererEmpty,
  RendererError,
  RendererLoading,
  RendererModeNotice,
  isTemplatePreviewContext,
} from './state';

export type DetailRendererProps = RendererProps<DetailLayout>;

/**
 * 详情布局渲染器组件
 */
export default function DetailRenderer({ layout, objectKey, context }: DetailRendererProps) {
  const isTemplatePreview = isTemplatePreviewContext(context);
  const [data, setData] = useState<any>(null);
  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState('');
  const previewData = React.useMemo(() => {
    if (!isTemplatePreview) return null;
    const obj: Record<string, any> = {};
    (layout.sections || []).forEach((section: any) => {
      (section?.fields || []).forEach((field: any) => {
        if (!field?.key) return;
        obj[field.key] = getPreviewDetailValue(field.key, field.label);
      });
    });
    if (Object.keys(obj).length === 0) {
      obj.playerId = '900001';
      obj.nickname = '示例玩家A';
      obj.level = 36;
      obj.vip = 3;
      obj.status = 'active';
    }
    return obj;
  }, [isTemplatePreview, layout.sections]);
  const effectiveSections = React.useMemo(() => {
    const sections = Array.isArray(layout.sections) ? layout.sections : [];
    if (sections.length > 0) return sections;
    if (!isTemplatePreview) return sections;
    return [
      {
        title: '基础信息',
        column: 2,
        fields: [
          { key: 'id', label: 'ID' },
          { key: 'name', label: '名称' },
          { key: 'status', label: '状态' },
          { key: 'updatedAt', label: '更新时间' },
        ],
      },
    ] as any[];
  }, [isTemplatePreview, layout.sections]);

  // 加载数据
  const loadData = async () => {
    if (!layout.detailFunction) {
      return;
    }

    setLoadError('');
    setLoading(true);
    try {
      // 使用函数调用服务
      const result = await invokeFunction(layout.detailFunction, context || {});

      setData(result);
    } catch (error: any) {
      setLoadError(error?.message || '加载详情失败');
      setData(null);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (isTemplatePreview) {
      return;
    }
    loadData();
  }, [layout.detailFunction, isTemplatePreview]);

  // 处理操作
  const handleAction = async (action: any) => {
    if (!action.function) {
      message.error('未配置操作函数');
      return;
    }

    try {
      // 使用函数调用服务
      await invokeFunction(action.function, data);

      message.success(`${action.label}成功`);
      // 重新加载
      loadData();
    } catch (error: any) {
      message.error(error.message || `${action.label}失败`);
    }
  };

  if (loading) return <RendererLoading tip="加载详情中..." />;

  const resolvedData = isTemplatePreview ? previewData : data;

  return (
    <div>
      <RendererModeNotice
        context={context}
        sampleTitle="详情预览"
        sampleDescription="当前详情正在使用示例字段值帮助你预览阅读结构，正式运行仍需要真实 detailFunction 与真实数据。"
      />
      {!layout.detailFunction && !isTemplatePreview && (
        <RendererEmpty description="当前详情未绑定函数，请在布局配置中选择 detailFunction" />
      )}
      {loadError && <RendererError message="详情加载失败" description={loadError} />}
      {!loadError && layout.detailFunction && !resolvedData && (
        <RendererEmpty description={isTemplatePreview ? '当前详情预览暂无示例数据' : '当前详情暂无真实数据'} />
      )}
      {/* 详情分区 */}
      {resolvedData &&
        (effectiveSections || []).map((section, index) => (
        <Card key={index} title={section.title} style={{ marginBottom: 16 }}>
          <Descriptions column={section.column || 2}>
            {section.fields.map((field) => (
              <Descriptions.Item key={field.key} label={field.label} span={field.span}>
                {renderDetailField(field, resolvedData[field.key])}
              </Descriptions.Item>
            ))}
          </Descriptions>
        </Card>
      ))}

      {/* 操作区 */}
      {resolvedData && layout.actions && layout.actions.length > 0 && (
        <Card title="操作">
          <Space>
            {layout.actions.map((action) => (
              <Button
                key={action.key}
                type={action.buttonType || 'default'}
                danger={action.danger}
                icon={getIcon(action.icon)}
                onClick={() => handleAction(action)}
                disabled={action.disabled}
              >
                {action.label}
              </Button>
            ))}
          </Space>
        </Card>
      )}
    </div>
  );
}

function getPreviewDetailValue(key: string, label?: string): any {
  const lower = key.toLowerCase();
  if (lower === 'id' || lower.endsWith('id')) return 'ID-1001';
  if (lower.includes('player') && lower.includes('id')) return '900001';
  if (lower.includes('user') && lower.includes('id')) return '700001';
  if (lower.includes('nickname')) return '示例玩家A';
  if (lower.includes('name') || lower.includes('title')) return `示例${label || key}`;
  if (lower.includes('server')) return 'S12';
  if (lower.includes('level')) return 36;
  if (lower.includes('vip')) return 3;
  if (lower.includes('status')) return 'active';
  if (lower.includes('count') || lower.includes('num')) return 42;
  if (lower.includes('time') || lower.includes('date')) return new Date().toISOString();
  if (lower.includes('desc') || lower.includes('remark')) return `这是${label || key}的示例说明`;
  return `${label || key}示例值`;
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
