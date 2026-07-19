import React, { useEffect, useState } from 'react';
import { Button, Card, Drawer, Space, Tag, Typography } from 'antd';
import { ProColumns, PageContainer } from '@ant-design/pro-components';
import { ReloadOutlined, FunctionOutlined } from '@ant-design/icons';
import { history } from '@umijs/max';
import { request } from '@umijs/max';
import XResourceTable from '@/components/XResourceTable';

interface EntityIndexItem {
  name: string;
  displayName?: string;
  category?: string;
  operations: string[];
  functions: string[];
  functionCount: number;
}

interface EntityFunctionItem {
  id: string;
  operation: string;
  name: string;
}

export default function EntitiesPage() {
  const [entities, setEntities] = useState<EntityIndexItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [functionsVisible, setFunctionsVisible] = useState(false);
  const [functionsLoading, setFunctionsLoading] = useState(false);
  const [selectedEntity, setSelectedEntity] = useState<EntityIndexItem | null>(null);
  const [entityFunctions, setEntityFunctions] = useState<EntityFunctionItem[]>([]);

  const loadEntities = async () => {
    setLoading(true);
    try {
      const res = await request<{ items: EntityIndexItem[]; total: number }>(
        '/api/v1/entity-index',
        { method: 'GET' },
      );
      setEntities(Array.isArray(res?.items) ? res.items : []);
    } catch (e: any) {
      console.error('Failed to load entity index:', e);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadEntities();
  }, []);

  const handleShowFunctions = async (entity: EntityIndexItem) => {
    setSelectedEntity(entity);
    setFunctionsVisible(true);
    setFunctionsLoading(true);
    try {
      const res = await request<{ items: EntityFunctionItem[] }>(
        `/api/v1/entity-index/${encodeURIComponent(entity.name)}/functions`,
        { method: 'GET' },
      );
      setEntityFunctions(Array.isArray(res?.items) ? res.items : []);
    } catch (e: any) {
      console.error('Failed to load entity functions:', e);
      setEntityFunctions([]);
    } finally {
      setFunctionsLoading(false);
    }
  };

  const operationColors: Record<string, string> = {
    create: 'green',
    read: 'blue',
    get: 'blue',
    list: 'blue',
    update: 'orange',
    delete: 'red',
    custom: 'default',
  };

  const columns: ProColumns<EntityIndexItem>[] = [
    {
      title: 'Entity',
      dataIndex: 'name',
      key: 'name',
      width: 180,
      render: (_, record) => (
        <Typography.Text strong>{record.displayName || record.name}</Typography.Text>
      ),
    },
    {
      title: 'Category',
      dataIndex: 'category',
      key: 'category',
      width: 120,
      render: (_: any, record: EntityIndexItem) => record.category ? <Tag color="blue">{record.category}</Tag> : '-',
    },
    {
      title: 'Operations',
      dataIndex: 'operations',
      key: 'operations',
      width: 200,
      render: (_, record) => (
        <Space size={4} wrap>
          {record.operations.map((op) => (
            <Tag key={op} color={operationColors[op] || 'default'}>{op}</Tag>
          ))}
        </Space>
      ),
    },
    {
      title: 'Functions',
      dataIndex: 'functionCount',
      key: 'functionCount',
      width: 100,
      sorter: (a: EntityIndexItem, b: EntityIndexItem) => a.functionCount - b.functionCount,
    },
    {
      title: 'Actions',
      key: 'actions',
      width: 200,
      render: (_, record) => (
        <Space>
          <Button
            type="link"
            size="small"
            icon={<FunctionOutlined />}
            onClick={() => handleShowFunctions(record)}
          >
            View Functions
          </Button>
          <Button
            type="link"
            size="small"
            onClick={() =>
              history.push(`/system/functions/catalog?entity=${encodeURIComponent(record.name)}`)
            }
          >
            Go to Catalog
          </Button>
        </Space>
      ),
    },
  ];

  return (
    <PageContainer
      title="Entity Index"
      subTitle="Auto-derived from function registrations. No manual CRUD needed."
    >
      <Card
        extra={
          <Button icon={<ReloadOutlined />} onClick={loadEntities} loading={loading}>
            Refresh
          </Button>
        }
      >
        <XResourceTable<EntityIndexItem>
          dataSource={entities}
          loading={loading}
          rowKey="name"
          columns={columns}
          pagination={false}
          search={false}
        />

        <Drawer
          title={`${selectedEntity?.displayName || selectedEntity?.name || ''} — Functions`}
          open={functionsVisible}
          onClose={() => setFunctionsVisible(false)}
          width={560}
        >
          {functionsLoading ? (
            <Typography.Text type="secondary">Loading...</Typography.Text>
          ) : entityFunctions.length === 0 ? (
            <Typography.Text type="secondary">No functions found.</Typography.Text>
          ) : (
            <Space direction="vertical" style={{ width: '100%' }} size="middle">
              {entityFunctions.map((item) => (
                <Card
                  key={item.id}
                  size="small"
                  title={
                    <Space>
                      <Typography.Text>{item.name || item.id}</Typography.Text>
                      <Tag color={operationColors[item.operation] || 'default'}>
                        {item.operation}
                      </Tag>
                    </Space>
                  }
                  extra={
                    <Button
                      type="link"
                      size="small"
                      onClick={() =>
                        history.push(`/system/functions/${encodeURIComponent(item.id)}`)
                      }
                    >
                      Detail
                    </Button>
                  }
                >
                  <Typography.Text type="secondary" code>{item.id}</Typography.Text>
                </Card>
              ))}
            </Space>
          )}
        </Drawer>
      </Card>
    </PageContainer>
  );
}
