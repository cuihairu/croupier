import React, { useState, useEffect } from 'react';
import {
  Card,
  Table,
  Space,
  Button,
  Tag,
  Modal,
  Tree,
  Descriptions,
  Form,
  Input,
  Select,
  Switch,
  Popconfirm,
  message,
  Drawer,
  Tabs,
  Badge,
  Tooltip,
  Alert
} from 'antd';
import {
  PlusOutlined,
  EditOutlined,
  DeleteOutlined,
  EyeOutlined,
  ApartmentOutlined,
  FunctionOutlined,
  SettingOutlined,
  PlayCircleOutlined,
  PauseCircleOutlined
} from '@ant-design/icons';
import { apiUrl } from '@/utils/api';

const { Option } = Select;
const { TextArea } = Input;

interface EntityDefinition {
  id: string;
  name: string;
  description: string;
  version: string;
  schema: any;
  operations: {
    [key: string]: {
      function: string;
      description: string;
    };
  };
  resources?: {
    [key: string]: {
      title: string;
      functions: string[];
      ui?: any;
    };
  };
  relationships?: {
    [key: string]: {
      type: string;
      target: string;
      cardinality: string;
    };
  };
  createdAt: string;
  updatedAt: string;
  status: 'active' | 'inactive' | 'draft';
  usageCount: number;
}

interface FunctionInfo {
  id: string;
  name: string;
  description: string;
  parameters: any;
  category: string;
}

export default function VirtualObjectManager() {
  const [entities, setEntities] = useState<EntityDefinition[]>([]);
  const [functions, setFunctions] = useState<FunctionInfo[]>([]);
  const [loading, setLoading] = useState(false);
  const [detailVisible, setDetailVisible] = useState(false);
  const [composerVisible, setComposerVisible] = useState(false);
  const [selectedEntity, setSelectedEntity] = useState<EntityDefinition | null>(null);
  const [form] = Form.useForm();

  const authHeaders = () => {
    const headers: Record<string, string> = {};
    const token = localStorage.getItem('token');
    const gid = localStorage.getItem('game_id');
    const env = localStorage.getItem('env');
    const isASCII = (s?: string | null) => !!s && /^[\x00-\x7F]*$/.test(s);
    if (token) headers['Authorization'] = `Bearer ${token}`;
    if (isASCII(gid)) headers['X-Game-ID'] = gid as string;
    if (isASCII(env)) headers['X-Env'] = env as string;
    return headers;
  };

  useEffect(() => {
    loadEntities();
    loadFunctions();
  }, []);

  const loadEntities = async () => {
    setLoading(true);
    try {
      const response = await fetch(apiUrl('/api/entities'), { credentials: 'include', headers: authHeaders() });
      const data = await response.json();

      const list = Array.isArray(data) ? data : (Array.isArray((data as any)?.entities) ? (data as any).entities : null);

      // 如果API调用失败或返回错误格式，使用模拟数据
      if (!response.ok || !list) {
        console.warn('API返回格式错误或未授权，使用模拟数据');

        // 模拟数据
        const mockEntities: EntityDefinition[] = [
          {
            id: 'player-entity',
            name: '玩家实体',
            description: '游戏玩家的完整管理实体，包含账户、状态、经济等操作',
            version: '1.0.0',
            schema: {
              type: 'object',
              properties: {
                id: { type: 'string', title: '玩家ID' },
                nickname: { type: 'string', title: '昵称' },
                level: { type: 'integer', title: '等级' },
                status: { type: 'string', enum: ['active', 'banned', 'suspended'], title: '状态' }
              }
            },
            operations: {
              'get': { function: 'player.get', description: '获取玩家信息' },
              'ban': { function: 'player.ban', description: '封禁玩家' },
              'unban': { function: 'player.unban', description: '解封玩家' },
              'addGold': { function: 'economy.addGold', description: '增加金币' }
            },
            resources: {
              'player-profile': {
                title: '玩家档案',
                functions: ['player.get', 'player.updateProfile']
              },
              'player-economy': {
                title: '经济管理',
                functions: ['economy.addGold', 'economy.deductGold']
              }
            },
            relationships: {
              'guild': { type: 'belongsTo', target: 'guild-entity', cardinality: 'one' }
            },
            createdAt: '2024-01-15T10:00:00Z',
            updatedAt: '2024-01-20T15:30:00Z',
            status: 'active',
            usageCount: 1520
          },
          {
            id: 'guild-entity',
            name: '公会实体',
            description: '公会管理实体，包含公会创建、成员管理、等级系统等',
            version: '1.2.0',
            schema: {
              type: 'object',
              properties: {
                id: { type: 'string', title: '公会ID' },
                name: { type: 'string', title: '公会名称' },
                level: { type: 'integer', title: '公会等级' },
                memberCount: { type: 'integer', title: '成员数量' }
              }
            },
            operations: {
              'create': { function: 'guild.create', description: '创建公会' },
              'dissolve': { function: 'guild.dissolve', description: '解散公会' },
              'addMember': { function: 'guild.addMember', description: '添加成员' },
              'removeMember': { function: 'guild.removeMember', description: '移除成员' }
            },
            resources: {
              'guild-management': {
                title: '公会管理',
                functions: ['guild.create', 'guild.dissolve', 'guild.levelUp']
              },
              'member-management': {
                title: '成员管理',
                functions: ['guild.addMember', 'guild.removeMember', 'guild.listMembers']
              }
            },
            relationships: {
              'members': { type: 'hasMany', target: 'player-entity', cardinality: 'many' },
              'leader': { type: 'hasOne', target: 'player-entity', cardinality: 'one' }
            },
            createdAt: '2024-01-10T08:00:00Z',
            updatedAt: '2024-01-25T12:45:00Z',
            status: 'active',
            usageCount: 850
          }
        ];

        setEntities(mockEntities);
      } else {
        // API返回正确格式的数据
        setEntities(list);
      }
    } catch (error) {
      message.error('加载虚拟对象失败');
      console.error('Load entities error:', error);
      setEntities([]);
    } finally {
      setLoading(false);
    }
  };

  const loadFunctions = async () => {
    try {
      const response = await fetch(apiUrl('/api/descriptors'), { credentials: 'include', headers: authHeaders() });
      const data = await response.json();

      if (!response.ok || !data || typeof data !== 'object') {
        console.warn('无法获取函数列表，使用空数组');
        setFunctions([]);
        return;
      }

      // 转换函数数据
      const functionList = Object.entries(data).map(([id, desc]: [string, any]) => ({
        id,
        name: desc?.name || id,
        description: desc?.description || '无描述',
        parameters: desc?.parameters || {},
        category: desc?.category || 'general'
      }));

      setFunctions(functionList);
    } catch (error) {
      console.error('Load functions error:', error);
      setFunctions([]);
    }
  };

  const handleCreateEntity = async (values: any) => {
    try {
      const entityData = {
        ...values,
        id: values.name.toLowerCase().replace(/\s+/g, '-'),
        version: '1.0.0',
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString(),
        usageCount: 0
      };

      const response = await fetch(apiUrl('/api/entities'), {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', ...authHeaders() },
        credentials: 'include',
        body: JSON.stringify(entityData)
      });

      if (response.ok) {
        message.success('虚拟对象创建成功');
        setComposerVisible(false);
        form.resetFields();
        loadEntities();
      } else {
        throw new Error('创建失败');
      }
    } catch (error) {
      message.error('虚拟对象创建失败');
    }
  };

  const handleDeleteEntity = async (entityId: string) => {
    try {
      const response = await fetch(apiUrl(`/api/entities/${entityId}`), {
        method: 'DELETE',
        credentials: 'include',
        headers: authHeaders(),
      });

      if (response.ok) {
        message.success('虚拟对象删除成功');
        loadEntities();
      } else {
        throw new Error('删除失败');
      }
    } catch (error) {
      message.error('虚拟对象删除失败');
    }
  };

  const handleToggleStatus = async (entityId: string, enabled: boolean) => {
    try {
      const response = await fetch(apiUrl(`/api/entities/${entityId}/status`), {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json', ...authHeaders() },
        credentials: 'include',
        body: JSON.stringify({ status: enabled ? 'active' : 'inactive' })
      });

      if (response.ok) {
        message.success(`虚拟对象${enabled ? '激活' : '停用'}成功`);
        loadEntities();
      } else {
        throw new Error('状态更新失败');
      }
    } catch (error) {
      message.error('状态更新失败');
    }
  };

  const columns = [
    {
      title: '名称',
      dataIndex: 'name',
      key: 'name',
      render: (name: string, record: EntityDefinition) => (
        <Space direction="vertical" size={0}>
          <strong>{name}</strong>
          <span style={{ color: '#666', fontSize: '12px' }}>v{record.version}</span>
        </Space>
      )
    },
    {
      title: '描述',
      dataIndex: 'description',
      key: 'description',
      ellipsis: true
    },
    {
      title: '操作数量',
      dataIndex: 'operations',
      key: 'operations',
      render: (operations: any) => (
        <Tag color="blue">{Object.keys(operations || {}).length} 个操作</Tag>
      )
    },
    {
      title: '资源组',
      dataIndex: 'resources',
      key: 'resources',
      render: (resources: any) => (
        <Tag color="green">{Object.keys(resources || {}).length} 个资源组</Tag>
      )
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      render: (status: string) => {
        const statusConfig = {
          active: { color: 'success', text: '活跃' },
          inactive: { color: 'default', text: '停用' },
          draft: { color: 'warning', text: '草稿' }
        };
        const config = statusConfig[status as keyof typeof statusConfig] || statusConfig.draft;
        return <Tag color={config.color}>{config.text}</Tag>;
      }
    },
    {
      title: '使用次数',
      dataIndex: 'usageCount',
      key: 'usageCount',
      render: (count: number) => (
        <Badge count={count} showZero style={{ backgroundColor: '#52c41a' }} />
      )
    },
    {
      title: '启用/停用',
      key: 'toggle',
      render: (text: any, record: EntityDefinition) => (
        <Switch
          checked={record.status === 'active'}
          onChange={(checked) => handleToggleStatus(record.id, checked)}
          checkedChildren="启用"
          unCheckedChildren="停用"
        />
      )
    },
    {
      title: '操作',
      key: 'actions',
      render: (text: any, record: EntityDefinition) => (
        <Space>
          <Tooltip title="查看详情">
            <Button
              size="small"
              icon={<EyeOutlined />}
              onClick={() => {
                setSelectedEntity(record);
                setDetailVisible(true);
              }}
            />
          </Tooltip>
          <Tooltip title="编辑">
            <Button
              size="small"
              icon={<EditOutlined />}
              onClick={() => {
                setSelectedEntity(record);
                setComposerVisible(true);
              }}
            />
          </Tooltip>
          <Popconfirm
            title="确定要删除此虚拟对象吗？"
            description="删除后将无法恢复，所有相关操作将失效"
            onConfirm={() => handleDeleteEntity(record.id)}
            okText="确定"
            cancelText="取消"
          >
            <Tooltip title="删除">
              <Button
                size="small"
                danger
                icon={<DeleteOutlined />}
              />
            </Tooltip>
          </Popconfirm>
        </Space>
      )
    }
  ];

  return (
    <div>
      <Card
        title={
          <Space>
            <ApartmentOutlined />
            虚拟对象管理
          </Space>
        }
        extra={
          <Space>
            <Button
              type="primary"
              icon={<PlusOutlined />}
              onClick={() => {
                setSelectedEntity(null);
                form.resetFields();
                setComposerVisible(true);
              }}
            >
              创建虚拟对象
            </Button>
            <Button onClick={loadEntities}>
              刷新
            </Button>
          </Space>
        }
      >
        <Alert
          message="虚拟对象说明"
          description="虚拟对象是业务抽象实体，通过组合多个函数来实现复杂的业务逻辑。每个虚拟对象包含操作、资源组和关系定义。"
          type="info"
          showIcon
          style={{ marginBottom: 16 }}
        />

        <Table
          columns={columns}
          dataSource={entities}
          rowKey="id"
          loading={loading}
          pagination={{
            pageSize: 10,
            showSizeChanger: true,
            showQuickJumper: true,
            showTotal: (total) => `共 ${total} 个虚拟对象`
          }}
        />
      </Card>

      {/* 虚拟对象详情弹窗 */}
      <Modal
        title={`虚拟对象详情 - ${selectedEntity?.name}`}
        open={detailVisible}
        onCancel={() => setDetailVisible(false)}
        width={800}
        footer={null}
      >
        {selectedEntity && (
          <Tabs
            defaultActiveKey="basic"
            items={[
              {
                key: 'basic',
                label: '基本信息',
                children: (
                  <Descriptions column={2} bordered>
                    <Descriptions.Item label="对象ID">{selectedEntity.id}</Descriptions.Item>
                    <Descriptions.Item label="对象名称">{selectedEntity.name}</Descriptions.Item>
                    <Descriptions.Item label="版本">{selectedEntity.version}</Descriptions.Item>
                    <Descriptions.Item label="状态">
                      <Tag color={selectedEntity.status === 'active' ? 'green' : 'red'}>
                        {selectedEntity.status === 'active' ? '活跃' : '停用'}
                      </Tag>
                    </Descriptions.Item>
                    <Descriptions.Item label="使用次数" span={2}>
                      <Badge count={selectedEntity.usageCount} showZero />
                    </Descriptions.Item>
                    <Descriptions.Item label="描述" span={2}>
                      {selectedEntity.description}
                    </Descriptions.Item>
                    <Descriptions.Item label="创建时间">
                      {new Date(selectedEntity.createdAt).toLocaleString()}
                    </Descriptions.Item>
                    <Descriptions.Item label="更新时间">
                      {new Date(selectedEntity.updatedAt).toLocaleString()}
                    </Descriptions.Item>
                  </Descriptions>
                )
              },
              {
                key: 'operations',
                label: '操作列表',
                children: (
                  <Table
                    dataSource={Object.entries(selectedEntity.operations || {}).map(([key, op]) => ({
                      key,
                      operation: key,
                      function: op.function,
                      description: op.description
                    }))}
                    columns={[
                      { title: '操作名', dataIndex: 'operation', key: 'operation' },
                      { title: '函数', dataIndex: 'function', key: 'function' },
                      { title: '描述', dataIndex: 'description', key: 'description' }
                    ]}
                    pagination={false}
                    size="small"
                  />
                )
              },
              {
                key: 'resources',
                label: '资源组',
                children: (
                  <Table
                    dataSource={Object.entries(selectedEntity.resources || {}).map(([key, res]) => ({
                      key,
                      name: key,
                      title: res.title,
                      functions: res.functions
                    }))}
                    columns={[
                      { title: '资源组名', dataIndex: 'name', key: 'name' },
                      { title: '标题', dataIndex: 'title', key: 'title' },
                      {
                        title: '包含函数',
                        dataIndex: 'functions',
                        key: 'functions',
                        render: (funcs: string[]) => (
                          <Space wrap>
                            {funcs.map(f => <Tag key={f}>{f}</Tag>)}
                          </Space>
                        )
                      }
                    ]}
                    pagination={false}
                    size="small"
                  />
                )
              },
              {
                key: 'relationships',
                label: '关系',
                children: (
                  <Table
                    dataSource={Object.entries(selectedEntity.relationships || {}).map(([key, rel]) => ({
                      key,
                      name: key,
                      type: rel.type,
                      target: rel.target,
                      cardinality: rel.cardinality
                    }))}
                    columns={[
                      { title: '关系名称', dataIndex: 'name', key: 'name' },
                      { title: '类型', dataIndex: 'type', key: 'type' },
                      { title: '目标', dataIndex: 'target', key: 'target' },
                      { title: '基数', dataIndex: 'cardinality', key: 'cardinality' }
                    ]}
                    pagination={false}
                    size="small"
                  />
                )
              },
            ]}
          />
        )}
      </Modal>

      {/* 创建/编辑虚拟对象弹窗 */}
      <Drawer
        title={selectedEntity ? '编辑虚拟对象' : '创建虚拟对象'}
        open={composerVisible}
        onClose={() => setComposerVisible(false)}
        width={720}
        footer={
          <Space style={{ float: 'right' }}>
            <Button onClick={() => setComposerVisible(false)}>取消</Button>
            <Button type="primary" onClick={() => form.submit()}>
              {selectedEntity ? '保存' : '创建'}
            </Button>
          </Space>
        }
      >
        <Form
          form={form}
          layout="vertical"
          onFinish={handleCreateEntity}
          initialValues={selectedEntity || { status: 'active' }}
        >
          <Form.Item
            name="name"
            label="对象名称"
            rules={[{ required: true, message: '请输入对象名称' }]}
          >
            <Input placeholder="请输入虚拟对象名称" />
          </Form.Item>

          <Form.Item
            name="description"
            label="对象描述"
            rules={[{ required: true, message: '请输入对象描述' }]}
          >
            <TextArea rows={3} placeholder="请描述虚拟对象的用途和功能" />
          </Form.Item>

          <Form.Item
            name="status"
            label="初始状态"
            rules={[{ required: true, message: '请选择初始状态' }]}
          >
            <Select>
              <Option value="active">激活</Option>
              <Option value="inactive">停用</Option>
              <Option value="draft">草稿</Option>
            </Select>
          </Form.Item>
        </Form>
      </Drawer>
    </div>
  );
}
