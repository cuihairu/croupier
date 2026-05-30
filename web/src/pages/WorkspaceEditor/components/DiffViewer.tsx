/**
 * 配置 Diff 视图组件
 *
 * 用于对比两个配置版本的差异，高亮显示变更内容。
 *
 * @module pages/WorkspaceEditor/components/DiffViewer
 */

import React from 'react';
import { Card, Tag, Typography, Collapse, Space } from 'antd';
import { PlusOutlined, MinusOutlined, DiffOutlined } from '@ant-design/icons';

const { Text } = Typography;

type DiffType = 'unchanged' | 'added' | 'removed' | 'updated';

interface DiffItem {
  path: string;
  type: DiffType;
  oldValue?: any;
  newValue?: any;
}

interface DiffViewerProps {
  title?: string;
  oldData?: Record<string, any> | null;
  newData?: Record<string, any> | null;
  expanded?: boolean;
}

/** 生成两个对象的差异 */
function generateDiff(
  oldObj: Record<string, any> | null,
  newObj: Record<string, any> | null,
  prefix = '',
): DiffItem[] {
  const diffs: DiffItem[] = [];
  const oldKeys = oldObj ? Object.keys(oldObj) : [];
  const newKeys = newObj ? Object.keys(newObj) : [];
  const allKeys = new Set([...oldKeys, ...newKeys]);

  for (const key of allKeys) {
    const path = prefix ? `${prefix}.${key}` : key;

    if (!(key in (oldObj || {}))) {
      // 新增字段
      diffs.push({ path, type: 'added', newValue: newObj![key] });
    } else if (!(key in (newObj || {}))) {
      // 删除字段
      diffs.push({ path, type: 'removed', oldValue: oldObj![key] });
    } else {
      const oldValue = oldObj![key];
      const newValue = newObj![key];

      if (JSON.stringify(oldValue) !== JSON.stringify(newValue)) {
        if (
          typeof oldValue === 'object' &&
          oldValue !== null &&
          typeof newValue === 'object' &&
          newValue !== null &&
          !Array.isArray(oldValue) &&
          !Array.isArray(newValue)
        ) {
          // 递归对比嵌套对象
          diffs.push(...generateDiff(oldValue, newValue, path));
        } else {
          // 值发生变化
          diffs.push({ path, type: 'updated', oldValue, newValue });
        }
      }
    }
  }

  return diffs;
}

/** 渲染差异值的显示 */
function renderDiffValue(value: any): React.ReactNode {
  if (value === null) return <Text type="secondary">null</Text>;
  if (value === undefined) return <Text type="secondary">undefined</Text>;
  if (typeof value === 'boolean') return <Text>{value ? 'true' : 'false'}</Text>;
  if (typeof value === 'object') {
    return <Text code>{JSON.stringify(value, null, 2)}</Text>;
  }
  if (typeof value === 'string' && value.length > 50) {
    return <Text code>{value.slice(0, 50)}...</Text>;
  }
  return <Text code>{String(value)}</Text>;
}

/** 差异项渲染 */
function DiffItemRow({ item }: { item: DiffItem }) {
  const renderIcon = () => {
    switch (item.type) {
      case 'added':
        return <PlusOutlined style={{ color: '#52c41a' }} />;
      case 'removed':
        return <MinusOutlined style={{ color: '#ff4d4f' }} />;
      case 'updated':
        return <DiffOutlined style={{ color: '#faad14' }} />;
      default:
        return null;
    }
  };

  const renderTag = () => {
    switch (item.type) {
      case 'added':
        return <Tag color="success">新增</Tag>;
      case 'removed':
        return <Tag color="error">删除</Tag>;
      case 'updated':
        return <Tag color="warning">变更</Tag>;
      default:
        return null;
    }
  };

  return (
    <div
      style={{
        padding: '8px 12px',
        borderBottom: '1px solid #f0f0f0',
        backgroundColor:
          item.type === 'added'
            ? '#f6ffed'
            : item.type === 'removed'
            ? '#fff1f0'
            : item.type === 'updated'
            ? '#fffbe6'
            : 'transparent',
      }}
    >
      <Space size={8}>
        {renderIcon()}
        {renderTag()}
        <Text strong style={{ fontSize: 12 }}>
          {item.path}
        </Text>
      </Space>

      <div style={{ marginTop: 8, marginLeft: 24, fontSize: 12 }}>
        {item.type === 'added' && (
          <div>
            <Text type="secondary">新值: </Text>
            {renderDiffValue(item.newValue)}
          </div>
        )}
        {item.type === 'removed' && (
          <div>
            <Text type="secondary">旧值: </Text>
            {renderDiffValue(item.oldValue)}
          </div>
        )}
        {item.type === 'updated' && (
          <div>
            <div>
              <Text type="secondary" delete>
                旧值:{' '}
              </Text>
              {renderDiffValue(item.oldValue)}
            </div>
            <div style={{ marginTop: 4 }}>
              <Text type="success">新值: </Text>
              {renderDiffValue(item.newValue)}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

/** 统计差异信息 */
function DiffSummary({ diffs }: { diffs: DiffItem[] }) {
  const added = diffs.filter((d) => d.type === 'added').length;
  const removed = diffs.filter((d) => d.type === 'removed').length;
  const updated = diffs.filter((d) => d.type === 'updated').length;

  if (diffs.length === 0) {
    return <Text type="secondary">配置无变化</Text>;
  }

  return (
    <Space size={8}>
      {added > 0 && <Tag color="success">新增 {added}</Tag>}
      {removed > 0 && <Tag color="error">删除 {removed}</Tag>}
      {updated > 0 && <Tag color="warning">变更 {updated}</Tag>}
    </Space>
  );
}

export default function DiffViewer({
  title = '配置变更对比',
  oldData,
  newData,
  expanded = false,
}: DiffViewerProps) {
  const diffs = React.useMemo(() => {
    return generateDiff(oldData || {}, newData || {});
  }, [oldData, newData]);

  if (diffs.length === 0) {
    return (
      <Card size="small" title={title}>
        <Text type="secondary">配置无变化</Text>
      </Card>
    );
  }

  // 按路径分组
  const groupedDiffs = diffs.reduce((acc, diff) => {
    const parts = diff.path.split('.');
    const groupKey = parts.length > 1 ? parts[0] : '_root';
    if (!acc[groupKey]) {
      acc[groupKey] = [];
    }
    acc[groupKey].push(diff);
    return acc;
  }, {} as Record<string, DiffItem[]>);

  const groupKeys = Object.keys(groupedDiffs).sort();

  return (
    <Card
      size="small"
      title={
        <Space size={8}>
          <span>{title}</span>
          <DiffSummary diffs={diffs} />
        </Space>
      }
    >
      <Collapse
        defaultActiveKey={expanded ? groupKeys : []}
        ghost
        size="small"
        items={groupKeys.map((groupKey) => ({
          key: groupKey,
          label: (
            <Space size={8}>
              <Text strong>{groupKey === '_root' ? '根配置' : groupKey}</Text>
              <Tag>{groupedDiffs[groupKey].length}</Tag>
            </Space>
          ),
          children: (
            <div style={{ border: '1px solid #f0f0f0', borderRadius: 4 }}>
              {groupedDiffs[groupKey].map((item, index) => (
                <DiffItemRow key={`${item.path}-${index}`} item={item} />
              ))}
            </div>
          ),
        }))}
      />
    </Card>
  );
}

/**
 * 简化的 Diff 视图，用于内嵌显示
 */
export interface SimpleDiffViewerProps {
  oldData?: Record<string, any> | null;
  newData?: Record<string, any> | null;
  maxItems?: number;
}

export function SimpleDiffViewer({ oldData, newData, maxItems = 5 }: SimpleDiffViewerProps) {
  const diffs = React.useMemo(() => {
    return generateDiff(oldData || {}, newData || {});
  }, [oldData, newData]);

  const displayDiffs = diffs.slice(0, maxItems);
  const hasMore = diffs.length > maxItems;

  if (diffs.length === 0) {
    return <Text type="secondary">配置无变化</Text>;
  }

  return (
    <div>
      {displayDiffs.map((item, index) => (
        <DiffItemRow key={`${item.path}-${index}`} item={item} />
      ))}
      {hasMore && (
        <div style={{ padding: '8px 12px', textAlign: 'center' }}>
          <Text type="secondary">还有 {diffs.length - maxItems} 项变更...</Text>
        </div>
      )}
    </div>
  );
}
