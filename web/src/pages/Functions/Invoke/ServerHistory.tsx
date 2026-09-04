import React, { useCallback, useEffect, useState } from 'react';
import { Button, Drawer, Space, Table, Tag, Typography } from 'antd';
import {
  getExecutionLog,
  listExecutionLogs,
  type ExecutionLogItem,
} from '@/services/api/executionLogs';

const { Text } = Typography;

const PAGE_SIZE = 10;

/** 服务端调用记录（R5）：我的执行留痕，替代仅本地的调用历史。 */
export default function ServerHistory({ open, onClose }: { open: boolean; onClose: () => void }) {
  const [items, setItems] = useState<ExecutionLogItem[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(false);
  const [detail, setDetail] = useState<null | {
    id: number;
    loading: boolean;
    request?: unknown;
    response?: unknown;
  }>(null);

  const list = useCallback(async () => {
    setLoading(true);
    try {
      const json = await listExecutionLogs({ mine: true, page, pageSize: PAGE_SIZE });
      setItems(json.items || []);
      setTotal(json.total || 0);
    } catch {
      /* 失败静默：Drawer 内下次翻页重试 */
    } finally {
      setLoading(false);
    }
  }, [page]);

  useEffect(() => {
    if (open) void list();
  }, [open, list]);

  const viewDetail = async (id: number) => {
    setDetail({ id, loading: true });
    try {
      const json = await getExecutionLog(id);
      setDetail({ id, loading: false, request: json.requestPayload, response: json.responseBody });
    } catch {
      setDetail({ id, loading: false });
    }
  };

  return (
    <Drawer title="服务端调用记录" width={720} open={open} onClose={onClose}>
      <Table
        rowKey="id"
        size="small"
        loading={loading}
        dataSource={items}
        pagination={{
          current: page,
          pageSize: PAGE_SIZE,
          total,
          onChange: setPage,
        }}
        expandable={{
          expandedRowRender: (record) => (
            <div>
              {detail?.id === record.id ? (
                detail.loading ? (
                  <Text type="secondary">加载中…</Text>
                ) : (
                  <Space direction="vertical" size={8} style={{ width: '100%' }}>
                    <div>
                      <Text type="secondary">请求：</Text>
                      <pre style={preStyle}>
                        {detail.request ? JSON.stringify(detail.request, null, 2) : '（无）'}
                      </pre>
                    </div>
                    <div>
                      <Text type="secondary">响应：</Text>
                      <pre style={preStyle}>
                        {detail.response ? JSON.stringify(detail.response, null, 2) : '（无）'}
                      </pre>
                    </div>
                  </Space>
                )
              ) : (
                <Button size="small" onClick={() => void viewDetail(record.id)}>
                  查看载荷
                </Button>
              )}
            </div>
          ),
        }}
        columns={[
          {
            title: '时间',
            dataIndex: 'createdAt',
            width: 150,
            render: (v: string) => new Date(v).toLocaleString(),
          },
          { title: '函数', dataIndex: 'functionId', ellipsis: true },
          {
            title: '来源',
            dataIndex: 'source',
            width: 70,
            render: (v: string) => (v === 'page' ? <Tag>页面</Tag> : <Tag>调用</Tag>),
          },
          {
            title: '状态',
            dataIndex: 'status',
            width: 70,
            render: (v: string) => (
              <Tag color={v === 'ok' ? 'green' : 'red'} style={{ marginInlineEnd: 0 }}>
                {v === 'ok' ? '成功' : '失败'}
              </Tag>
            ),
          },
          { title: '耗时(ms)', dataIndex: 'durationMs', width: 90 },
        ]}
      />
    </Drawer>
  );
}

const preStyle: React.CSSProperties = {
  whiteSpace: 'pre-wrap',
  background: '#f6f6f6',
  padding: 8,
  borderRadius: 6,
  maxHeight: 220,
  overflow: 'auto',
  fontSize: 12,
};
