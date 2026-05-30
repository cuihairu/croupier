/**
 * 布局结构缩略图组件
 *
 * 显示布局的整体结构，点击可跳转到对应配置区域。
 *
 * @module pages/WorkspaceEditor/components/LayoutThumbnail
 */

import React from 'react';
import { Card, Space, Typography, Tag, Collapse } from 'antd';
import {
  AppstoreOutlined,
  UnorderedListOutlined,
  FormOutlined,
  FileTextOutlined,
  EyeOutlined,
  ColumnHeightOutlined,
  BorderOutlined,
  CheckSquareOutlined,
} from '@ant-design/icons';
import type { TabConfig, TabLayout } from '@/types/workspace';

const { Text } = Typography;

export interface LayoutThumbnailProps {
  tab: TabConfig;
  onFieldClick?: (fieldKey: string) => void;
  onSectionClick?: (sectionIndex: number) => void;
}

/** 获取布局类型图标 */
function getLayoutIcon(type: TabLayout['type']) {
  switch (type) {
    case 'list':
      return <UnorderedListOutlined />;
    case 'form':
      return <FormOutlined />;
    case 'detail':
      return <FileTextOutlined />;
    case 'form-detail':
      return <AppstoreOutlined />;
    case 'kanban':
      return <ColumnHeightOutlined />;
    default:
      return <BorderOutlined />;
  }
}

/** 获取布局类型标签颜色 */
function getLayoutTypeColor(type: TabLayout['type']): string {
  switch (type) {
    case 'list':
      return 'blue';
    case 'form':
      return 'green';
    case 'detail':
      return 'cyan';
    case 'form-detail':
      return 'purple';
    case 'kanban':
      return 'orange';
    case 'timeline':
      return 'gold';
    case 'split':
      return 'magenta';
    default:
      return 'default';
  }
}

/**
 * 渲染列表布局的缩略图
 */
function renderListThumbnail(layout: any) {
  const columnCount = layout.columns?.length || 0;
  const searchFieldCount = layout.searchFields?.length || 0;

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
        <Tag color="blue">列表</Tag>
        <Text type="secondary" style={{ fontSize: 11 }}>
          {columnCount} 列
        </Text>
      </div>
      {searchFieldCount > 0 && (
        <div style={{ paddingLeft: 12 }}>
          <Text type="secondary" style={{ fontSize: 11 }}>
            🔍 {searchFieldCount} 个搜索条件
          </Text>
        </div>
      )}
      <div
        style={{
          display: 'flex',
          gap: 2,
          flexWrap: 'wrap',
          padding: '4px 8px',
          background: '#f5f5f5',
          borderRadius: 4,
          minHeight: 24,
        }}
      >
        {layout.columns?.slice(0, 8).map((col: any, i: number) => (
          <div
            key={i}
            style={{
              padding: '2px 6px',
              background: '#fff',
              border: '1px solid #d9d9d9',
              borderRadius: 2,
              fontSize: 10,
              whiteSpace: 'nowrap',
            }}
          >
            {col.title || col.key}
          </div>
        ))}
        {columnCount > 8 && (
          <div style={{ padding: '2px 6px', fontSize: 10, color: '#999' }}>+{columnCount - 8}</div>
        )}
      </div>
    </div>
  );
}

/**
 * 渲染表单布局的缩略图
 */
function renderFormThumbnail(layout: any) {
  const fieldCount = layout.fields?.length || 0;

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
        <Tag color="green">表单</Tag>
        <Text type="secondary" style={{ fontSize: 11 }}>
          {fieldCount} 个字段
        </Text>
      </div>
      <div
        style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fill, minmax(60px, 1fr))',
          gap: 4,
          padding: '4px 8px',
          background: '#f5f5f5',
          borderRadius: 4,
          minHeight: 40,
        }}
      >
        {layout.fields?.slice(0, 12).map((field: any, i: number) => (
          <div
            key={i}
            style={{
              padding: '4px 6px',
              background: field.required ? '#e6f7ff' : '#fff',
              border: '1px solid #d9d9d9',
              borderRadius: 3,
              fontSize: 10,
              textAlign: 'center',
            }}
          >
            {field.label || field.key}
            {field.required && <span style={{ color: '#ff4d4f' }}>*</span>}
          </div>
        ))}
        {fieldCount > 12 && (
          <div
            style={{
              gridColumn: '1 / -1',
              textAlign: 'center',
              fontSize: 10,
              color: '#999',
            }}
          >
            还有 {fieldCount - 12} 个字段...
          </div>
        )}
      </div>
    </div>
  );
}

/**
 * 渲染详情布局的缩略图
 */
function renderDetailThumbnail(layout: any) {
  const sectionCount = layout.sections?.length || 0;
  const totalFields = (layout.sections || []).reduce(
    (sum: number, s: any) => sum + (s.fields?.length || 0),
    0,
  );

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
        <Tag color="cyan">详情</Tag>
        <Text type="secondary" style={{ fontSize: 11 }}>
          {sectionCount} 个分区 · {totalFields} 个字段
        </Text>
      </div>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
        {layout.sections?.slice(0, 5).map((section: any, i: number) => (
          <div
            key={i}
            style={{
              padding: '4px 8px',
              background: '#fff',
              border: '1px solid #d9d9d9',
              borderRadius: 4,
            }}
          >
            <Text strong style={{ fontSize: 11 }}>
              {section.title}
            </Text>
            <Text type="secondary" style={{ fontSize: 10 }}>
              {' '}
              ({section.fields?.length || 0} 字段)
            </Text>
          </div>
        ))}
        {sectionCount > 5 && (
          <Text type="secondary" style={{ fontSize: 10, paddingLeft: 8 }}>
            还有 {sectionCount - 5} 个分区...
          </Text>
        )}
      </div>
    </div>
  );
}

/**
 * 渲染表单-详情布局的缩略图
 */
function renderFormDetailThumbnail(layout: any) {
  const queryFieldCount = layout.queryFields?.length || 0;

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
        <Tag color="purple">查询-详情</Tag>
        <Text type="secondary" style={{ fontSize: 11 }}>
          {queryFieldCount} 个查询条件
        </Text>
      </div>
      <div
        style={{
          padding: '6px 10px',
          background: '#f5f5f5',
          borderRadius: 4,
        }}
      >
        <Text style={{ fontSize: 11 }}>查询区 → 详情展示区</Text>
      </div>
    </div>
  );
}

export default function LayoutThumbnail({
  tab,
  onFieldClick,
  onSectionClick,
}: LayoutThumbnailProps) {
  const layout = tab.layout;

  const renderLayoutThumbnail = () => {
    switch (layout.type) {
      case 'list':
        return renderListThumbnail(layout);
      case 'form':
        return renderFormThumbnail(layout);
      case 'detail':
        return renderDetailThumbnail(layout);
      case 'form-detail':
        return renderFormDetailThumbnail(layout);
      default:
        return (
          <div style={{ padding: '8px', textAlign: 'center', color: '#999' }}>
            <BorderOutlined style={{ fontSize: 24, marginBottom: 8 }} />
            <div style={{ fontSize: 12 }}>{layout.type}</div>
          </div>
        );
    }
  };

  return (
    <Card
      size="small"
      title={
        <Space size={8}>
          {getLayoutIcon(layout.type)}
          <span>布局结构</span>
          <Tag color={getLayoutTypeColor(layout.type)}>{layout.type}</Tag>
        </Space>
      }
    >
      {renderLayoutThumbnail()}
    </Card>
  );
}

/**
 * 多 Tab 布局结构概览
 */
export interface MultiTabThumbnailProps {
  tabs: TabConfig[];
  activeTab?: string;
  onTabClick?: (tabKey: string) => void;
}

export function MultiTabThumbnail({ tabs, activeTab, onTabClick }: MultiTabThumbnailProps) {
  return (
    <Space direction="vertical" style={{ width: '100%' }} size={8}>
      {tabs.map((tab) => {
        const isActive = tab.key === activeTab;
        return (
          <div
            key={tab.key}
            onClick={() => onTabClick?.(tab.key)}
            style={{
              cursor: onTabClick ? 'pointer' : 'default',
              border: `1px solid ${isActive ? '#1677ff' : '#f0f0f0'}`,
              borderRadius: 6,
              padding: 8,
              background: isActive ? '#f6ffed' : '#fff',
              transition: 'all 0.2s',
            }}
          >
            <Space size={8}>
              {tab.icon && <span>{tab.icon}</span>}
              <Text strong style={{ fontSize: 12 }}>
                {tab.title}
              </Text>
              <Tag color="blue">{tab.key}</Tag>
              <Tag>{tab.functions?.length || 0} 函数</Tag>
            </Space>
            <div style={{ marginTop: 4, marginLeft: 32 }}>
              <Text type="secondary" style={{ fontSize: 11 }}>
                {tab.layout.type}
              </Text>
            </div>
          </div>
        );
      })}
    </Space>
  );
}
