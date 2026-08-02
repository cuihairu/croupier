/**
 * ResourceCatalogPage - 资源目录页面
 *
 * 展示资源目录，包括：
 * - 资源列表
 * - 语义信息
 * - 函数列表
 * - 诊断信息
 *
 * @module pages/ResourceCatalog
 */

import React, { useState, useCallback, useEffect } from 'react';
import {
  Card,
  Table,
  Tag,
  Space,
  Button,
  Input,
  Select,
  Descriptions,
  Modal,
  Form,
  message,
  Typography,
  Tooltip,
  Badge,
} from 'antd';
import {
  SearchOutlined,
  ReloadOutlined,
  EditOutlined,
  EyeOutlined,
  FunctionOutlined,
  CheckCircleOutlined,
  WarningOutlined,
  CloseCircleOutlined,
  QuestionCircleOutlined,
} from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import type {
  ResourceCatalogItem,
  FunctionInfo,
  SemanticsInfo,
  DiagnosticInfo,
} from '@/types/dashboard';
import {
  listResourceCatalog,
  getResourceDetail,
  updateResourceSemantics,
} from '@/services/dashboard';

const { Text, Title } = Typography;
const { Option } = Select;

// ---------------------------------------------------------------------------
// 状态颜色映射
// ---------------------------------------------------------------------------

const statusColors: Record<string, string> = {
  identified: 'success',
  pending: 'warning',
  conflict: 'error',
  not_executable: 'default',
};

const statusLabels: Record<string, string> = {
  identified: '已识别',
  pending: '待确认',
  conflict: '冲突',
  not_executable: '不可执行',
};

// ---------------------------------------------------------------------------
// ResourceCatalogPage 组件
// ---------------------------------------------------------------------------

const ResourceCatalogPage: React.FC = () => {
  const [loading, setLoading] = useState(false);
  const [data, setData] = useState<ResourceCatalogItem[]>([]);
  const [total, setTotal] = useState(0);
  const [category, setCategory] = useState<string>('');
  const [query, setQuery] = useState<string>('');
  const [selectedResource, setSelectedResource] = useState<ResourceCatalogItem | null>(null);
  const [detailVisible, setDetailVisible] = useState(false);
  const [editVisible, setEditVisible] = useState(false);
  const [editForm] = Form.useForm();

  // 加载数据
  const fetchData = useCallback(async () => {
    setLoading(true);
    try {
      const result = await listResourceCatalog({
        category: category || undefined,
        query: query || undefined,
      });
      setData(result.items);
      setTotal(result.total);
    } catch (error) {
      const errMsg = error instanceof Error ? error.message : '操作失败';
      message.error(errMsg);
    } finally {
      setLoading(false);
    }
  }, [category, query]);

  // 初始加载
  useEffect(() => {
    fetchData();
  }, [fetchData]);

  // 查看详情
  const handleViewDetail = useCallback(async (resourceKey: string) => {
    try {
      const detail = await getResourceDetail(resourceKey);
      setSelectedResource(detail);
      setDetailVisible(true);
    } catch (error) {
      const errMsg = error instanceof Error ? error.message : '未知错误';
      message.error('获取详情失败: ' + errMsg);
    }
  }, []);

  // 编辑语义
  const handleEditSemantics = useCallback((resource: ResourceCatalogItem) => {
    setSelectedResource(resource);
    editForm.setFieldsValue({
      identityField: resource.semantics?.hasIdentity ? 'id' : '',
      collectionQueryId: '',
      itemQueryId: '',
      createId: '',
      updateId: '',
      deleteId: '',
    });
    setEditVisible(true);
  }, [editForm]);

  // 保存语义
  const handleSaveSemantics = useCallback(async () => {
    if (!selectedResource) {
      return;
    }

    try {
      const values = await editForm.validateFields();
      await updateResourceSemantics(selectedResource.resourceKey, values);
      message.success('语义更新成功');
      setEditVisible(false);
      fetchData();
    } catch (error) {
      const errMsg = error instanceof Error ? error.message : '未知错误';
      message.error('更新失败: ' + errMsg);
    }
  }, [selectedResource, editForm, fetchData]);

  // 表格列定义
  const columns: ColumnsType<ResourceCatalogItem> = [
    {
      title: '资源标识',
      dataIndex: 'resourceKey',
      key: 'resourceKey',
      render: (text) => <Text strong>{text}</Text>,
    },
    {
      title: '名称',
      dataIndex: 'labels',
      key: 'labels',
      render: (labels: Record<string, string>) => labels?.['zh-CN'] || labels?.['en'] || '-',
    },
    {
      title: '分类',
      dataIndex: 'categoryKey',
      key: 'categoryKey',
      render: (text) => text || '-',
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      render: (status: string) => (
        <Tag color={statusColors[status] || 'default'}>
          {statusLabels[status] || status}
        </Tag>
      ),
    },
    {
      title: '函数数量',
      dataIndex: 'functions',
      key: 'functions',
      render: (functions: FunctionInfo[]) => functions?.length || 0,
    },
    {
      title: '语义版本',
      dataIndex: 'semantics',
      key: 'semantics',
      render: (semantics: SemanticsInfo) => semantics?.version || '-',
    },
    {
      title: '诊断',
      dataIndex: 'diagnostics',
      key: 'diagnostics',
      render: (diagnostics: DiagnosticInfo[]) => {
        if (!diagnostics || diagnostics.length === 0) {
          return <Tag color="success">无</Tag>;
        }
        const errors = diagnostics.filter((d) => d.severity === 'error').length;
        const warnings = diagnostics.filter((d) => d.severity === 'warning').length;
        return (
          <Space>
            {errors > 0 && <Tag color="error">{errors} 错误</Tag>}
            {warnings > 0 && <Tag color="warning">{warnings} 警告</Tag>}
          </Space>
        );
      },
    },
    {
      title: '操作',
      key: 'action',
      render: (_, record) => (
        <Space>
          <Button
            type="link"
            icon={<EyeOutlined />}
            onClick={() => handleViewDetail(record.resourceKey)}
          >
            查看
          </Button>
          <Button
            type="link"
            icon={<EditOutlined />}
            onClick={() => handleEditSemantics(record)}
          >
            编辑语义
          </Button>
        </Space>
      ),
    },
  ];

  return (
    <div>
      {/* 搜索栏 */}
      <Card style={{ marginBottom: 16 }}>
        <Space>
          <Input
            placeholder="搜索资源"
            prefix={<SearchOutlined />}
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onPressEnter={fetchData}
            style={{ width: 200 }}
          />
          <Select
            placeholder="选择分类"
            value={category || undefined}
            onChange={(value) => setCategory(value || '')}
            allowClear
            style={{ width: 150 }}
          >
            <Option value="">全部</Option>
          </Select>
          <Button type="primary" icon={<SearchOutlined />} onClick={fetchData}>
            搜索
          </Button>
          <Button icon={<ReloadOutlined />} onClick={fetchData}>
            刷新
          </Button>
        </Space>
      </Card>

      {/* 资源列表 */}
      <Card title="资源目录">
        <Table
          columns={columns}
          dataSource={data}
          rowKey="resourceKey"
          loading={loading}
          pagination={{
            total,
            pageSize: 20,
            showSizeChanger: true,
            showTotal: (total) => `共 ${total} 条`,
          }}
        />
      </Card>

      {/* 详情弹窗 */}
      <Modal
        title="资源详情"
        open={detailVisible}
        onCancel={() => setDetailVisible(false)}
        footer={null}
        width={800}
      >
        {selectedResource && (
          <div>
            <Descriptions column={2} bordered>
              <Descriptions.Item label="资源标识">{selectedResource.resourceKey}</Descriptions.Item>
              <Descriptions.Item label="名称">
                {selectedResource.labels?.['zh-CN'] || selectedResource.labels?.['en'] || '-'}
              </Descriptions.Item>
              <Descriptions.Item label="分类">{selectedResource.categoryKey || '-'}</Descriptions.Item>
              <Descriptions.Item label="状态">
                <Tag color={statusColors[selectedResource.status]}>
                  {statusLabels[selectedResource.status]}
                </Tag>
              </Descriptions.Item>
            </Descriptions>

            {/* 函数列表 */}
            <Title level={5} style={{ marginTop: 16 }}>
              <FunctionOutlined /> 函数列表
            </Title>
            <Table
              dataSource={selectedResource.functions}
              rowKey="functionId"
              pagination={false}
              size="small"
              columns={[
                { title: '函数 ID', dataIndex: 'functionId', key: 'functionId' },
                { title: '版本', dataIndex: 'version', key: 'version' },
                {
                  title: '能力',
                  dataIndex: 'capability',
                  key: 'capability',
                  render: (text) => <Tag>{text}</Tag>,
                },
                {
                  title: '执行方式',
                  dataIndex: 'execution',
                  key: 'execution',
                  render: (text) => <Tag>{text}</Tag>,
                },
                {
                  title: '风险',
                  dataIndex: 'risk',
                  key: 'risk',
                  render: (text) => (
                    <Tag
                      color={
                        text === 'danger' ? 'red' : text === 'high' ? 'orange' : text === 'warning' ? 'yellow' : 'green'
                      }
                    >
                      {text}
                    </Tag>
                  ),
                },
                {
                  title: '状态',
                  dataIndex: 'enabled',
                  key: 'enabled',
                  render: (enabled) => (
                    <Tag color={enabled ? 'success' : 'default'}>
                      {enabled ? '启用' : '禁用'}
                    </Tag>
                  ),
                },
              ]}
            />

            {/* 语义信息 */}
            {selectedResource.semantics && (
              <>
                <Title level={5} style={{ marginTop: 16 }}>
                  <CheckCircleOutlined /> 语义信息
                </Title>
                <Descriptions column={2} bordered size="small">
                  <Descriptions.Item label="版本">{selectedResource.semantics.version}</Descriptions.Item>
                  <Descriptions.Item label="来源">{selectedResource.semantics.source}</Descriptions.Item>
                  <Descriptions.Item label="Identity">
                    {selectedResource.semantics.hasIdentity ? '✓' : '✗'}
                  </Descriptions.Item>
                  <Descriptions.Item label="Collection">
                    {selectedResource.semantics.hasCollection ? '✓' : '✗'}
                  </Descriptions.Item>
                  <Descriptions.Item label="Create">
                    {selectedResource.semantics.hasCreate ? '✓' : '✗'}
                  </Descriptions.Item>
                  <Descriptions.Item label="Update">
                    {selectedResource.semantics.hasUpdate ? '✓' : '✗'}
                  </Descriptions.Item>
                  <Descriptions.Item label="Delete">
                    {selectedResource.semantics.hasDelete ? '✓' : '✗'}
                  </Descriptions.Item>
                  <Descriptions.Item label="Actions">
                    {selectedResource.semantics.hasActions ? '✓' : '✗'}
                  </Descriptions.Item>
                </Descriptions>
              </>
            )}

            {/* 诊断信息 */}
            {selectedResource.diagnostics && selectedResource.diagnostics.length > 0 && (
              <>
                <Title level={5} style={{ marginTop: 16 }}>
                  <WarningOutlined /> 诊断信息
                </Title>
                <Table
                  dataSource={selectedResource.diagnostics}
                  rowKey="code"
                  pagination={false}
                  size="small"
                  columns={[
                    { title: '代码', dataIndex: 'code', key: 'code' },
                    {
                      title: '级别',
                      dataIndex: 'severity',
                      key: 'severity',
                      render: (text) => (
                        <Tag
                          color={
                            text === 'error' ? 'red' : text === 'warning' ? 'orange' : 'blue'
                          }
                        >
                          {text}
                        </Tag>
                      ),
                    },
                    { title: '消息', dataIndex: 'message', key: 'message' },
                  ]}
                />
              </>
            )}
          </div>
        )}
      </Modal>

      {/* 编辑语义弹窗 */}
      <Modal
        title="编辑语义"
        open={editVisible}
        onOk={handleSaveSemantics}
        onCancel={() => setEditVisible(false)}
        okText="保存"
        cancelText="取消"
      >
        <Form form={editForm} layout="vertical">
          <Form.Item label="Identity 字段" name="identityField">
            <Input placeholder="例如: id, player_id" />
          </Form.Item>
          <Form.Item label="Collection Query ID" name="collectionQueryId">
            <Input placeholder="函数 ID" />
          </Form.Item>
          <Form.Item label="Item Query ID" name="itemQueryId">
            <Input placeholder="函数 ID" />
          </Form.Item>
          <Form.Item label="Create ID" name="createId">
            <Input placeholder="函数 ID" />
          </Form.Item>
          <Form.Item label="Update ID" name="updateId">
            <Input placeholder="函数 ID" />
          </Form.Item>
          <Form.Item label="Delete ID" name="deleteId">
            <Input placeholder="函数 ID" />
          </Form.Item>
          <Form.Item label="变更原因" name="changeReason">
            <Input.TextArea placeholder="说明变更原因" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default ResourceCatalogPage;
