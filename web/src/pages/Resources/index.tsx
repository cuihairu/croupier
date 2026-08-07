import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { Button, Card, Drawer, Input, Space, Tag, Typography } from 'antd';
import { ProColumns, PageContainer, ProTable } from '@ant-design/pro-components';
import { FileSearchOutlined, ReloadOutlined } from '@ant-design/icons';
import { history } from '@umijs/max';
import { listResourceOperations, listResources } from '@/services/api/resources';
import type { OperationSpec, ResourceSpec } from '@/types/dashboard';

function localizedText(text: Record<string, string> | undefined, fallback: string): string {
  if (!text) return fallback;
  return (
    text['zh-CN'] || text['en-US'] || Object.values(text).find((value) => value.trim()) || fallback
  );
}

function riskColor(risk?: string) {
  if (risk === 'danger') return 'red';
  if (risk === 'high') return 'volcano';
  if (risk === 'warning') return 'orange';
  return 'green';
}

export default function ResourcesPage() {
  const [resources, setResources] = useState<ResourceSpec[]>([]);
  const [loading, setLoading] = useState(false);
  const [query, setQuery] = useState('');
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [detailLoading, setDetailLoading] = useState(false);
  const [selectedResource, setSelectedResource] = useState<ResourceSpec | null>(null);
  const [operations, setOperations] = useState<OperationSpec[]>([]);

  const loadResources = useCallback(async () => {
    setLoading(true);
    try {
      setResources(await listResources(query ? { q: query } : undefined));
    } finally {
      setLoading(false);
    }
  }, [query]);

  useEffect(() => {
    loadResources();
  }, [loadResources]);

  const openResource = async (resource: ResourceSpec) => {
    setSelectedResource(resource);
    setDrawerOpen(true);
    setDetailLoading(true);
    try {
      const nextOperations = await listResourceOperations(resource.key);
      setOperations(nextOperations);
    } finally {
      setDetailLoading(false);
    }
  };

  const totalOperations = useMemo(
    () => resources.reduce((sum, resource) => sum + (resource.operations?.length || 0), 0),
    [resources],
  );

  const columns: ProColumns<ResourceSpec>[] = [
    {
      title: '资源',
      dataIndex: 'key',
      width: 220,
      render: (_, record) => (
        <Space direction="vertical" size={0}>
          <Typography.Text strong>{localizedText(record.labels, record.key)}</Typography.Text>
          <Typography.Text code>{record.key}</Typography.Text>
        </Space>
      ),
    },
    {
      title: '分类',
      dataIndex: ['category', 'key'],
      width: 180,
      render: (_, record) => (
        <Space>
          <Tag color="blue">
            {localizedText(record.category?.labels, record.category?.key || '-')}
          </Tag>
          <Typography.Text code>{record.category?.key}</Typography.Text>
        </Space>
      ),
    },
    {
      title: '操作数',
      dataIndex: 'operations',
      width: 100,
      render: (_, record) => record.operations?.length || 0,
      sorter: (a, b) => (a.operations?.length || 0) - (b.operations?.length || 0),
    },
    {
      title: '标签',
      dataIndex: 'tags',
      render: (_, record) => (
        <Space size={4} wrap>
          {(record.tags || []).map((tag) => (
            <Tag key={tag}>{tag}</Tag>
          ))}
        </Space>
      ),
    },
    {
      title: '操作',
      key: 'actions',
      width: 220,
      render: (_, record) => (
        <Space>
          <Button
            type="link"
            size="small"
            icon={<FileSearchOutlined />}
            onClick={() => openResource(record)}
          >
            查看操作
          </Button>
          <Button
            type="link"
            size="small"
            onClick={() =>
              history.push(`/system/functions/catalog?resource=${encodeURIComponent(record.key)}`)
            }
          >
            函数目录
          </Button>
        </Space>
      ),
    },
  ];

  return (
    <PageContainer
      title="资源/操作"
      subTitle="从函数描述符归一化得到的业务资源和操作，不是数据库实体，也不提供通用 CRUD。"
    >
      <Space direction="vertical" size={16} style={{ width: '100%' }}>
        <Card>
          <Space wrap>
            <Input.Search
              allowClear
              placeholder="搜索资源 key 或标题"
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              onSearch={loadResources}
              style={{ width: 320 }}
            />
            <Button icon={<ReloadOutlined />} onClick={loadResources} loading={loading}>
              刷新
            </Button>
            <Tag color="blue">{`资源 ${resources.length}`}</Tag>
            <Tag color="green">{`操作 ${totalOperations}`}</Tag>
          </Space>
        </Card>

        <Card>
          <ProTable<ResourceSpec>
            dataSource={resources}
            loading={loading}
            rowKey="key"
            columns={columns}
            search={false}
            pagination={{ pageSize: 20 }}
            options={false}
          />
        </Card>
      </Space>

      <Drawer
        title={
          selectedResource
            ? localizedText(selectedResource.labels, selectedResource.key)
            : '资源详情'
        }
        open={drawerOpen}
        onClose={() => setDrawerOpen(false)}
        width={760}
      >
        <Space direction="vertical" size={16} style={{ width: '100%' }}>
          {selectedResource && (
            <Card size="small" title="资源边界">
              <Space direction="vertical" size={8}>
                <Typography.Text code>{selectedResource.key}</Typography.Text>
                <Typography.Text type="secondary">
                  Resource 表示页面组织用的业务能力域，不表示数据库表，也不是 CRUD 实体。
                </Typography.Text>
              </Space>
            </Card>
          )}

          <Card size="small" title="操作" loading={detailLoading}>
            <Space direction="vertical" style={{ width: '100%' }}>
              {operations.map((operation) => (
                <Card key={operation.functionId} size="small">
                  <Space direction="vertical" size={8} style={{ width: '100%' }}>
                    <Space wrap>
                      <Typography.Text strong>{operation.operation}</Typography.Text>
                      <Tag color={riskColor(operation.risk)}>{operation.risk || 'safe'}</Tag>
                      {operation.permission && <Tag>{operation.permission}</Tag>}
                      {!operation.enabled && <Tag color="default">disabled</Tag>}
                    </Space>
                    <Typography.Text code>{operation.functionId}</Typography.Text>
                  </Space>
                </Card>
              ))}
              {operations.length === 0 && (
                <Typography.Text type="secondary">暂无操作</Typography.Text>
              )}
            </Space>
          </Card>

          <Card size="small" title="默认页面 Proposal">
            <Space direction="vertical" size={8}>
              <Typography.Text type="secondary">
                默认页面由 FunctionContract 与 Resource Catalog 语义生成
                PageProposal；资源页不再临时生成页面候选。
              </Typography.Text>
              <Button
                type="primary"
                onClick={() =>
                  selectedResource &&
                  history.push(
                    `/system/functions/proposals?resourceKey=${encodeURIComponent(selectedResource.key)}`,
                  )
                }
              >
                打开 Proposal 队列
              </Button>
            </Space>
          </Card>
        </Space>
      </Drawer>
    </PageContainer>
  );
}
