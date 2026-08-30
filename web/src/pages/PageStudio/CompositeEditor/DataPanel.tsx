import React, { useState } from 'react';
import { Button, Space, Table, Typography } from 'antd';
import { CaretDownOutlined, CaretRightOutlined, PlayCircleOutlined } from '@ant-design/icons';
import { invokeFunction, type FunctionDescriptor } from '@/services/api/functions';
import { extractErrorMessage } from '@/utils/errors';
import { sectionParams } from './types';
import type { PageNode } from './model';

const { Text } = Typography;

type JSONRecord = Record<string, unknown>;

/** 编辑态底部数据面板：选中函数组件一键试跑，结果即席展示
 * （Appsmith 底部 Query 面板形态——编辑中随时看数据）。 */
export default function DataPanel({
  node,
  fn,
}: {
  node: PageNode | undefined;
  fn: FunctionDescriptor | undefined;
}) {
  const [open, setOpen] = useState(false);
  const [running, setRunning] = useState(false);
  const [error, setError] = useState('');
  const [data, setData] = useState<JSONRecord | null>(null);

  if (
    !node ||
    !fn ||
    (node.type !== 'fnTable' && node.type !== 'fnFields' && node.type !== 'fnForm')
  ) {
    return null;
  }

  const params = sectionParams(fn);
  const run = async () => {
    setRunning(true);
    setError('');
    try {
      const resp = (await invokeFunction(fn.id, {} as never)) as JSONRecord | null;
      setData(resp ?? {});
    } catch (err) {
      setError(extractErrorMessage(err, '执行失败'));
    } finally {
      setRunning(false);
    }
  };

  const rawInner = data ? (data.result ?? data) : undefined;
  const inner =
    rawInner && typeof rawInner === 'object' && !Array.isArray(rawInner)
      ? (rawInner as JSONRecord)
      : {};
  const items = Array.isArray(inner?.items) ? (inner!.items as JSONRecord[]) : [];

  return (
    <div style={{ borderTop: '1px solid #f0f0f0', background: '#fff' }}>
      <div
        style={{
          padding: '6px 16px',
          cursor: 'pointer',
          display: 'flex',
          alignItems: 'center',
          gap: 8,
        }}
        onClick={() => setOpen(!open)}
      >
        {open ? <CaretDownOutlined /> : <CaretRightOutlined />}
        <Text strong style={{ fontSize: 12 }}>
          数据
        </Text>
        <Text type="secondary" style={{ fontSize: 11 }}>
          试跑 {fn.id}（{params.length} 个参数，默认空跑）
        </Text>
        <Button
          size="small"
          type="primary"
          ghost
          icon={<PlayCircleOutlined />}
          loading={running}
          onClick={(e) => {
            e.stopPropagation();
            setOpen(true);
            void run();
          }}
        >
          执行
        </Button>
      </div>
      {open && (
        <div style={{ padding: '0 16px 12px', maxHeight: 260, overflow: 'auto' }}>
          {error && (
            <Text type="danger" style={{ fontSize: 12 }}>
              {error}
            </Text>
          )}
          {items.length > 0 && (
            <Table<JSONRecord>
              size="small"
              rowKey={(_, i) => String(i)}
              pagination={{ pageSize: 5, size: 'small' }}
              dataSource={items}
              columns={Object.keys(items[0])
                .slice(0, 8)
                .map((k) => ({
                  title: k,
                  key: k,
                  render: (_: unknown, row: JSONRecord) => (
                    <Text style={{ fontSize: 12 }}>{String(row[k] ?? '')}</Text>
                  ),
                }))}
            />
          )}
          {data !== null && !error && items.length === 0 && (
            <pre style={{ fontSize: 12, background: '#fafafa', padding: 8, borderRadius: 4 }}>
              {String(JSON.stringify(inner, null, 2) ?? '')}
            </pre>
          )}
        </div>
      )}
    </div>
  );
}
