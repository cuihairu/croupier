import React from 'react';
import { Collapse, Space, Tag } from 'antd';
import { getIntl } from '@umijs/max';

export const formatDateTime = (value?: string | number | Date) => {
  if (value === null || value === undefined || value === '') return '-';
  if (value instanceof Date) {
    return Number.isNaN(value.getTime()) ? '-' : value.toLocaleString('zh-CN');
  }
  if (typeof value === 'number') {
    const ts = value < 1e12 ? value * 1000 : value;
    const d = new Date(ts);
    return Number.isNaN(d.getTime()) ? '-' : d.toLocaleString('zh-CN');
  }

  const raw = String(value).trim();
  if (!raw) return '-';
  if (/^\d+$/.test(raw)) {
    const n = Number(raw);
    const ts = n < 1e12 ? n * 1000 : n;
    const d = new Date(ts);
    return Number.isNaN(d.getTime()) ? '-' : d.toLocaleString('zh-CN');
  }

  const d = new Date(raw);
  return Number.isNaN(d.getTime()) ? '-' : d.toLocaleString('zh-CN');
};

export const renderHistoryDetail = (key: string, value: unknown) => {
  if (Array.isArray(value)) {
    if (value.length === 0) return <Tag>0</Tag>;
    if (value.length <= 8) {
      return (
        <Space wrap>
          {value.map((x, idx) => (
            <Tag key={`${key}-${idx}`}>{String(x)}</Tag>
          ))}
        </Space>
      );
    }
    // 展示文案经 getIntl() 在调用时解析（非组件模块无 intl 上下文，先例 Ops/Terms）
    return (
      <Tag>
        {getIntl().formatMessage(
          { id: 'pages.assignments.history.itemCount', defaultMessage: `${value.length} 项` },
          { count: value.length },
        )}
      </Tag>
    );
  }
  if (value && typeof value === 'object') {
    return <pre style={{ margin: 0 }}>{JSON.stringify(value, null, 2)}</pre>;
  }
  return String(value);
};

// #37：变更历史逐条明细的 diff 摘要。details 由后端 Update 写入
// （before/after/added/removed/unknown），此前前端只按原始英文 key 平铺
// 渲染，与「数量级记录」 complaint 无异——这里按语义分组：新增/移除/
// 未识别逐项着色展示，全量 before/after 折叠为计数（展开查完整清单）。
const historyStrList = (value: unknown): string[] =>
  Array.isArray(value) ? value.map((v) => String(v)) : [];

const historyDiffRow = (
  key: string,
  labelText: string,
  items: string[],
  color?: string,
  truncate = true,
): React.ReactNode => {
  if (items.length === 0) return null;
  const shown = truncate ? items.slice(0, 8) : items;
  const rest = truncate ? items.length - shown.length : 0;
  return (
    <Space key={key} wrap size={4}>
      <Tag color={color}>{labelText}</Tag>
      {shown.map((fn) => (
        <Tag key={`${key}-${fn}`}>{fn}</Tag>
      ))}
      {rest > 0 ? <Tag>{`+${rest}`}</Tag> : null}
    </Space>
  );
};

export const renderHistoryDiff = (details?: Record<string, unknown>) => {
  if (!details) return null;
  const intl = getIntl();
  const added = historyStrList(details.added);
  const removed = historyStrList(details.removed);
  const unknown = historyStrList(details.unknown);
  const before = historyStrList(details.before);
  const after = historyStrList(details.after);
  if (added.length + removed.length + unknown.length + before.length + after.length === 0) {
    return null;
  }
  const label = (id: string, defaultMessage: string) =>
    intl.formatMessage({ id, defaultMessage });

  return (
    <Space orientation="vertical" size={4} style={{ width: '100%' }}>
      {before.length + after.length > 0 ? (
        <span>
          {label('pages.assignments.history.diff.before', '变更前')} {before.length} →{' '}
          {label('pages.assignments.history.diff.after', '变更后')} {after.length}
        </span>
      ) : null}
      {historyDiffRow(
        'added',
        label('pages.assignments.history.diff.added', '新增'),
        added,
        'green',
      )}
      {historyDiffRow(
        'removed',
        label('pages.assignments.history.diff.removed', '移除'),
        removed,
        'red',
      )}
      {historyDiffRow(
        'unknown',
        label('pages.assignments.history.diff.unknown', '未识别'),
        unknown,
      )}
      {before.length + after.length > 0 ? (
        <Collapse
          size="small"
          items={[
            {
              key: 'full-lists',
              label: label('pages.assignments.history.diff.fullList', '展开全量清单'),
              children: (
                <Space orientation="vertical" size={4} style={{ width: '100%' }}>
                  {historyDiffRow(
                    'before-full',
                    label('pages.assignments.history.diff.before', '变更前'),
                    before,
                    undefined,
                    false,
                  )}
                  {historyDiffRow(
                    'after-full',
                    label('pages.assignments.history.diff.after', '变更后'),
                    after,
                    undefined,
                    false,
                  )}
                </Space>
              ),
            },
          ]}
        />
      ) : null}
    </Space>
  );
};
