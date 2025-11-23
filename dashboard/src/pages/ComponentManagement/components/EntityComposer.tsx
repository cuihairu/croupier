import React, { useState, useEffect, useCallback } from 'react';
import {
  Card, Steps, Form, Input, Select, Transfer, Button, Space, Typography,
  Table, Tag, Modal, Tree, Divider, Alert, Tabs, Switch, Tooltip,
  Row, Col, message, Popconfirm, InputNumber, Collapse
} from 'antd';
import {
  PlusOutlined, DeleteOutlined, LinkOutlined, FunctionOutlined,
  ApartmentOutlined, SettingOutlined, SaveOutlined, PlayCircleOutlined,
  EyeOutlined, ArrowRightOutlined, ApiOutlined, CodeOutlined
} from '@ant-design/icons';
import { apiUrl } from '@/utils/api';

const { Step } = Steps;
const { TextArea } = Input;
const { Text, Title } = Typography;
const { Option } = Select;
const { Panel } = Collapse;

interface FunctionInfo {
  key: string;
  title: string;
  description: string;
  category: string;
  parameters?: any;
}

interface EntityOperation {
  id: string;
  name: string;
  functionId: string;
  description: string;
  parameters?: any;
  ui?: any;
  order: number;
}

interface EntityResource {
  id: string;
  title: string;
  description: string;
  functions: string[];
  ui?: any;
  layout?: 'tabs' | 'collapse' | 'list';
}

interface EntityRelationship {
  id: string;
  name: string;
  type: 'hasOne' | 'hasMany' | 'belongsTo' | 'manyToMany';
  targetEntity: string;
  description: string;
  cardinality: string;
  constraints?: any;
}

interface EntityComposition {
  id: string;
  name: string;
  description: string;
  version: string;
  schema: any;
  operations: EntityOperation[];
  resources: EntityResource[];
  relationships: EntityRelationship[];
  metadata?: any;
}

interface EntityComposerProps {
  entity?: EntityComposition | null;
  visible: boolean;
  onSave: (entity: EntityComposition) => void;
  onCancel: () => void;
}

export default function EntityComposer({ entity, visible, onSave, onCancel }: EntityComposerProps) {
  const [currentStep, setCurrentStep] = useState(0);
  const [composition, setComposition] = useState<EntityComposition>({
    id: '',
    name: '',
    description: '',
    version: '1.0.0',
    schema: {},
    operations: [],
    resources: [],
    relationships: []
  });
  const [availableFunctions, setAvailableFunctions] = useState<FunctionInfo[]>([]);
  const [availableEntities, setAvailableEntities] = useState<string[]>([]);
  const [transferTargetKeys, setTransferTargetKeys] = useState<string[]>([]);
  const [selectedFunctions, setSelectedFunctions] = useState<string[]>([]);
  const [previewModalVisible, setPreviewModalVisible] = useState(false);
  const [testModalVisible, setTestModalVisible] = useState(false);

  const [form] = Form.useForm();

  useEffect(() => {
    if (visible) {
      loadAvailableFunctions();
      loadAvailableEntities();

      if (entity) {
        setComposition(entity);
        form.setFieldsValue(entity);
        setSelectedFunctions(entity.operations.map(op => op.functionId));
        setTransferTargetKeys(entity.operations.map(op => op.functionId));
      } else {
        resetComposer();
      }
    }
  }, [visible, entity, form]);

  const resetComposer = () => {
    setCurrentStep(0);
    setComposition({
      id: '',
      name: '',
      description: '',
      version: '1.0.0',
      schema: {},
      operations: [],
      resources: [],
      relationships: []
    });
    setSelectedFunctions([]);
    setTransferTargetKeys([]);
    form.resetFields();
  };

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

  const loadAvailableFunctions = async () => {
    try {
      const response = await fetch(apiUrl('/api/descriptors'), { credentials: 'include', headers: authHeaders() });
      const data = await response.json();

      const functions: FunctionInfo[] = Object.entries(data || {}).map(([key, desc]: [string, any]) => ({
        key,
        title: desc.name || key,
        description: desc.description || '无描述',
        category: desc.category || 'general',
        parameters: desc.parameters
      }));

      setAvailableFunctions(functions);
    } catch (error) {
      console.error('Failed to load functions:', error);
    }
  };

  const loadAvailableEntities = async () => {
    try {
      const response = await fetch(apiUrl('/api/entities'), { credentials: 'include', headers: authHeaders() });
      const data = await response.json();
      const list = Array.isArray(data) ? data : (Array.isArray(data?.entities) ? data.entities : []);
      setAvailableEntities(list.map((e: any) => e.id) || []);
    } catch (error) {
      console.error('Failed to load entities:', error);
    }
  };

  const handleBasicInfoNext = async () => {
    try {
      const values = await form.validateFields(['id', 'name', 'description', 'version', 'schema']);
      setComposition(prev => ({
        ...prev,
        ...values,
        schema: typeof values.schema === 'string' ? JSON.parse(values.schema) : values.schema
      }));
      setCurrentStep(1);
    } catch (error) {
      message.error('请完善基本信息');
    }
  };

  const handleFunctionChange = (targetKeys: string[], direction: string, moveKeys: string[]) => {
    setTransferTargetKeys(targetKeys);
    setSelectedFunctions(targetKeys);

    if (direction === 'right') {
      // 添加新操作
      const newOperations = moveKeys.map((functionId, index) => {
        const func = availableFunctions.find(f => f.key === functionId);
        return {
          id: `${functionId}_${Date.now()}_${index}`,
          name: func?.title || functionId,
          functionId,
          description: func?.description || '',
          parameters: func?.parameters || {},
          order: composition.operations.length + index + 1
        };
      });

      setComposition(prev => ({
        ...prev,
        operations: [...prev.operations, ...newOperations]
      }));
    } else {
      // 移除操作
      setComposition(prev => ({
        ...prev,
        operations: prev.operations.filter(op => !moveKeys.includes(op.functionId))
      }));
    }
  };

  const addResource = () => {
    const newResource: EntityResource = {
      id: `resource_${Date.now()}`,
      title: '新资源',
      description: '',
      functions: [],
      layout: 'tabs'
    };

    setComposition(prev => ({
      ...prev,
      resources: [...prev.resources, newResource]
    }));
  };

  const updateResource = (resourceId: string, updates: Partial<EntityResource>) => {
    setComposition(prev => ({
      ...prev,
      resources: prev.resources.map(res =>
        res.id === resourceId ? { ...res, ...updates } : res
      )
    }));
  };

  const removeResource = (resourceId: string) => {
    setComposition(prev => ({
      ...prev,
      resources: prev.resources.filter(res => res.id !== resourceId)
    }));
  };

  const addRelationship = () => {
    const newRelationship: EntityRelationship = {
      id: `relation_${Date.now()}`,
      name: '新关系',
      type: 'hasMany',
      targetEntity: '',
      description: '',
      cardinality: 'many'
    };

    setComposition(prev => ({
      ...prev,
      relationships: [...prev.relationships, newRelationship]
    }));
  };

  const updateRelationship = (relationshipId: string, updates: Partial<EntityRelationship>) => {
    setComposition(prev => ({
      ...prev,
      relationships: prev.relationships.map(rel =>
        rel.id === relationshipId ? { ...rel, ...updates } : rel
      )
    }));
  };

  const removeRelationship = (relationshipId: string) => {
    setComposition(prev => ({
      ...prev,
      relationships: prev.relationships.filter(rel => rel.id !== relationshipId)
    }));
  };

  const handleSave = async () => {
    try {
      if (!composition.id || !composition.name) {
        message.error('请填写基本信息');
        setCurrentStep(0);
        return;
      }

      if (composition.operations.length === 0) {
        message.error('请至少添加一个操作');
        setCurrentStep(1);
        return;
      }

      onSave(composition);
      message.success('虚拟对象保存成功');
      resetComposer();
    } catch (error) {
      message.error('保存失败');
    }
  };

  const generatePreview = () => {
    return {
      manifest: {
        id: composition.id,
        name: composition.name,
        description: composition.description,
        version: composition.version,
        entities: {
          [composition.id]: {
            schema: composition.schema,
            operations: composition.operations.reduce((ops, op) => {
              ops[op.name] = {
                function: op.functionId,
                description: op.description,
                parameters: op.parameters
              };
              return ops;
            }, {} as any),
            resources: composition.resources.reduce((resources, res) => {
              resources[res.id] = {
                title: res.title,
                functions: res.functions,
                ui: res.ui
              };
              return resources;
            }, {} as any),
            relationships: composition.relationships.reduce((rels, rel) => {
              rels[rel.name] = {
                type: rel.type,
                target: rel.targetEntity,
                cardinality: rel.cardinality
              };
              return rels;
            }, {} as any)
          }
        }
      }
    };
  };

  const renderStepContent = () => {
    switch (currentStep) {
      case 0:
        return (
          <Card title="基本信息配置" size="small">
            <Form form={form} layout="vertical">
              <Row gutter={16}>
                <Col span={12}>
                  <Form.Item
                    name="id"
                    label="实体ID"
                    rules={[{ required: true, message: '请输入实体ID' }]}
                  >
                    <Input placeholder="例如: player-entity" />
                  </Form.Item>
                </Col>
                <Col span={12}>
                  <Form.Item
                    name="version"
                    label="版本"
                    rules={[{ required: true, message: '请输入版本' }]}
                  >
                    <Input placeholder="例如: 1.0.0" />
                  </Form.Item>
                </Col>
              </Row>

              <Form.Item
                name="name"
                label="实体名称"
                rules={[{ required: true, message: '请输入实体名称' }]}
              >
                <Input placeholder="例如: 玩家实体" />
              </Form.Item>

              <Form.Item
                name="description"
                label="描述"
                rules={[{ required: true, message: '请输入描述' }]}
              >
                <TextArea rows={3} placeholder="描述这个虚拟对象的用途和功能" />
              </Form.Item>

              <Form.Item
                name="schema"
                label="数据结构定义 (JSON Schema)"
                rules={[{ required: true, message: '请输入Schema定义' }]}
              >
                <TextArea
                  rows={8}
                  placeholder="输入JSON Schema定义"
                  defaultValue={JSON.stringify({
                    type: 'object',
                    properties: {
                      id: { type: 'string', title: 'ID' },
                      name: { type: 'string', title: '名称' }
                    },
                    required: ['id']
                  }, null, 2)}
                />
              </Form.Item>
            </Form>
          </Card>
        );

      case 1:
        return (
          <Card title="函数操作配置" size="small">
            <Alert
              message="选择要包含在此虚拟对象中的函数操作"
              description="从左侧选择可用函数，移动到右侧来创建操作。每个函数可以配置参数映射和UI展现。"
              type="info"
              style={{ marginBottom: 16 }}
            />

            <Transfer
              dataSource={availableFunctions}
              targetKeys={transferTargetKeys}
              selectedKeys={selectedFunctions}
              onChange={handleFunctionChange}
              render={item => `${item.title} - ${item.description}`}
              titles={['可用函数', '已选操作']}
              style={{ marginBottom: 16 }}
            />

            {composition.operations.length > 0 && (
              <Card size="small" title="操作配置">
                <Table
                  size="small"
                  dataSource={composition.operations}
                  rowKey="id"
                  pagination={false}
                  columns={[
                    { title: '操作名称', dataIndex: 'name', key: 'name' },
                    { title: '函数', dataIndex: 'functionId', key: 'functionId' },
                    { title: '描述', dataIndex: 'description', key: 'description', ellipsis: true },
                    {
                      title: '顺序',
                      dataIndex: 'order',
                      key: 'order',
                      width: 80,
                      render: (order, record) => (
                        <InputNumber
                          size="small"
                          value={order}
                          min={1}
                          onChange={(value) => {
                            setComposition(prev => ({
                              ...prev,
                              operations: prev.operations.map(op =>
                                op.id === record.id ? { ...op, order: value || 1 } : op
                              )
                            }));
                          }}
                        />
                      )
                    },
                    {
                      title: '操作',
                      key: 'actions',
                      width: 100,
                      render: (_, record) => (
                        <Space>
                          <Button size="small" icon={<SettingOutlined />} />
                          <Button
                            size="small"
                            danger
                            icon={<DeleteOutlined />}
                            onClick={() => {
                              setComposition(prev => ({
                                ...prev,
                                operations: prev.operations.filter(op => op.id !== record.id)
                              }));
                              setTransferTargetKeys(prev => prev.filter(key => key !== record.functionId));
                            }}
                          />
                        </Space>
                      )
                    }
                  ]}
                />
              </Card>
            )}
          </Card>
        );

      case 2:
        return (
          <Card title="资源组织配置" size="small">
            <Alert
              message="将操作组织成资源组"
              description="资源组是UI层面的函数组合，可以将相关的操作组织在一起展示。"
              type="info"
              style={{ marginBottom: 16 }}
            />

            <Button
              type="dashed"
              icon={<PlusOutlined />}
              onClick={addResource}
              style={{ marginBottom: 16, width: '100%' }}
            >
              添加资源组
            </Button>

            <Collapse defaultActiveKey={composition.resources.map(res => res.id)}>
              {composition.resources.map((resource, index) => (
                <Panel
                  header={
                    <Space>
                      <Text strong>{resource.title}</Text>
                      <Tag>{resource.functions.length} 个函数</Tag>
                    </Space>
                  }
                  key={resource.id}
                  extra={
                    <Button
                      size="small"
                      danger
                      icon={<DeleteOutlined />}
                      onClick={(e) => {
                        e.stopPropagation();
                        removeResource(resource.id);
                      }}
                    />
                  }
                >
                  <Row gutter={16}>
                    <Col span={12}>
                      <Form.Item label="资源标题">
                        <Input
                          value={resource.title}
                          onChange={(e) => updateResource(resource.id, { title: e.target.value })}
                        />
                      </Form.Item>
                    </Col>
                    <Col span={12}>
                      <Form.Item label="布局方式">
                        <Select
                          value={resource.layout}
                          onChange={(value) => updateResource(resource.id, { layout: value })}
                        >
                          <Option value="tabs">标签页</Option>
                          <Option value="collapse">折叠面板</Option>
                          <Option value="list">列表</Option>
                        </Select>
                      </Form.Item>
                    </Col>
                  </Row>

                  <Form.Item label="描述">
                    <TextArea
                      value={resource.description}
                      rows={2}
                      onChange={(e) => updateResource(resource.id, { description: e.target.value })}
                    />
                  </Form.Item>

                  <Form.Item label="包含的函数">
                    <Select
                      mode="multiple"
                      value={resource.functions}
                      onChange={(values) => updateResource(resource.id, { functions: values })}
                      placeholder="选择要包含在此资源组中的函数"
                    >
                      {composition.operations.map(op => (
                        <Option key={op.functionId} value={op.functionId}>
                          {op.name}
                        </Option>
                      ))}
                    </Select>
                  </Form.Item>
                </Panel>
              ))}
            </Collapse>
          </Card>
        );

      case 3:
        return (
          <Card title="关系定义配置" size="small">
            <Alert
              message="定义与其他实体的关系"
              description="关系定义了虚拟对象之间的关联关系，支持一对一、一对多、多对多等关系类型。"
              type="info"
              style={{ marginBottom: 16 }}
            />

            <Button
              type="dashed"
              icon={<PlusOutlined />}
              onClick={addRelationship}
              style={{ marginBottom: 16, width: '100%' }}
            >
              添加关系定义
            </Button>

            {composition.relationships.map((relationship, index) => (
              <Card
                key={relationship.id}
                size="small"
                title={`关系 ${index + 1}: ${relationship.name}`}
                extra={
                  <Button
                    size="small"
                    danger
                    icon={<DeleteOutlined />}
                    onClick={() => removeRelationship(relationship.id)}
                  />
                }
                style={{ marginBottom: 16 }}
              >
                <Row gutter={16}>
                  <Col span={12}>
                    <Form.Item label="关系名称">
                      <Input
                        value={relationship.name}
                        onChange={(e) => updateRelationship(relationship.id, { name: e.target.value })}
                      />
                    </Form.Item>
                  </Col>
                  <Col span={12}>
                    <Form.Item label="关系类型">
                      <Select
                        value={relationship.type}
                        onChange={(value) => updateRelationship(relationship.id, { type: value })}
                      >
                        <Option value="hasOne">一对一 (hasOne)</Option>
                        <Option value="hasMany">一对多 (hasMany)</Option>
                        <Option value="belongsTo">属于 (belongsTo)</Option>
                        <Option value="manyToMany">多对多 (manyToMany)</Option>
                      </Select>
                    </Form.Item>
                  </Col>
                </Row>

                <Row gutter={16}>
                  <Col span={12}>
                    <Form.Item label="目标实体">
                      <Select
                        value={relationship.targetEntity}
                        onChange={(value) => updateRelationship(relationship.id, { targetEntity: value })}
                        placeholder="选择目标实体"
                      >
                        {availableEntities.map(entityId => (
                          <Option key={entityId} value={entityId}>
                            {entityId}
                          </Option>
                        ))}
                      </Select>
                    </Form.Item>
                  </Col>
                  <Col span={12}>
                    <Form.Item label="基数">
                      <Input
                        value={relationship.cardinality}
                        onChange={(e) => updateRelationship(relationship.id, { cardinality: e.target.value })}
                        placeholder="例如: one, many, 0..1, 1..*"
                      />
                    </Form.Item>
                  </Col>
                </Row>

                <Form.Item label="关系描述">
                  <TextArea
                    value={relationship.description}
                    rows={2}
                    onChange={(e) => updateRelationship(relationship.id, { description: e.target.value })}
                    placeholder="描述这个关系的含义和用途"
                  />
                </Form.Item>
              </Card>
            ))}
          </Card>
        );

      case 4:
        return (
          <Card title="预览与确认" size="small">
            <Alert
              message="请确认虚拟对象配置"
              description="检查配置是否正确，然后保存虚拟对象。"
              type="success"
              style={{ marginBottom: 16 }}
            />

            <Tabs
              defaultActiveKey="summary"
              items={[
                {
                  key: 'summary',
                  label: '配置概要',
                  children: (
                    <>
                      <Descriptions column={2} bordered size="small">
                        <Descriptions.Item label="实体ID">{composition.id}</Descriptions.Item>
                        <Descriptions.Item label="名称">{composition.name}</Descriptions.Item>
                        <Descriptions.Item label="版本">{composition.version}</Descriptions.Item>
                        <Descriptions.Item label="操作数量">{composition.operations.length}</Descriptions.Item>
                        <Descriptions.Item label="资源组数量">{composition.resources.length}</Descriptions.Item>
                        <Descriptions.Item label="关系数量">{composition.relationships.length}</Descriptions.Item>
                        <Descriptions.Item label="描述" span={2}>{composition.description}</Descriptions.Item>
                      </Descriptions>

                      <Divider />

                      <Space style={{ marginBottom: 16 }}>
                        <Button icon={<EyeOutlined />} onClick={() => setPreviewModalVisible(true)}>
                          预览配置
                        </Button>
                        <Button icon={<PlayCircleOutlined />} onClick={() => setTestModalVisible(true)}>
                          测试验证
                        </Button>
                      </Space>
                    </>
                  )
                },
                {
                  key: 'operations',
                  label: '操作列表',
                  children: (
                    <Table
                      size="small"
                      dataSource={composition.operations.sort((a, b) => a.order - b.order)}
                      rowKey="id"
                      pagination={false}
                      columns={[
                        { title: '顺序', dataIndex: 'order', key: 'order', width: 60 },
                        { title: '操作名', dataIndex: 'name', key: 'name' },
                        { title: '函数', dataIndex: 'functionId', key: 'functionId' },
                        { title: '描述', dataIndex: 'description', key: 'description' }
                      ]}
                    />
                  )
                },
                {
                  key: 'resources',
                  label: '资源组织',
                  children: (
                    <>
                      {composition.resources.map(resource => (
                        <Card key={resource.id} size="small" style={{ marginBottom: 8 }}>
                          <Text strong>{resource.title}</Text>
                          <br />
                          <Text type="secondary">{resource.description}</Text>
                          <br />
                          <Tag color="blue">布局: {resource.layout}</Tag>
                          <Tag color="green">函数: {resource.functions.length}</Tag>
                        </Card>
                      ))}
                    </>
                  )
                },
                {
                  key: 'relationships',
                  label: '关系定义',
                  children: (
                    <>
                      {composition.relationships.map(rel => (
                        <Card key={rel.id} size="small" style={{ marginBottom: 8 }}>
                          <Text strong>{rel.name}</Text>
                          <Tag color="blue" style={{ marginLeft: 8 }}>{rel.type}</Tag>
                          <ArrowRightOutlined style={{ margin: '0 8px' }} />
                          <Text code>{rel.targetEntity}</Text>
                          <br />
                          <Text type="secondary">{rel.description}</Text>
                        </Card>
                      ))}
                    </>
                  )
                },
              ]}
            />
          </Card>
        );

      default:
        return null;
    }
  };

  const steps = [
    { title: '基本信息', description: '配置实体基础信息' },
    { title: '函数操作', description: '选择和配置操作函数' },
    { title: '资源组织', description: '组织UI资源组' },
    { title: '关系定义', description: '定义实体关系' },
    { title: '预览确认', description: '预览并保存配置' }
  ];

  return (
    <Modal
      title={
        <Space>
          <ApartmentOutlined />
          {entity ? '编辑虚拟对象' : '创建虚拟对象'}
        </Space>
      }
      open={visible}
      onCancel={onCancel}
      width="90%"
      style={{ top: 20 }}
      footer={null}
    >
      <Steps current={currentStep} style={{ marginBottom: 24 }}>
        {steps.map((step, index) => (
          <Step
            key={index}
            title={step.title}
            description={step.description}
            style={{ cursor: 'pointer' }}
            onClick={() => setCurrentStep(index)}
          />
        ))}
      </Steps>

      {renderStepContent()}

      <div style={{ textAlign: 'right', marginTop: 24 }}>
        <Space>
          {currentStep > 0 && (
            <Button onClick={() => setCurrentStep(currentStep - 1)}>
              上一步
            </Button>
          )}
          {currentStep < steps.length - 1 ? (
            <Button
              type="primary"
              onClick={() => {
                if (currentStep === 0) {
                  handleBasicInfoNext();
                } else {
                  setCurrentStep(currentStep + 1);
                }
              }}
            >
              下一步
            </Button>
          ) : (
            <Button type="primary" icon={<SaveOutlined />} onClick={handleSave}>
              保存虚拟对象
            </Button>
          )}
        </Space>
      </div>

      {/* 配置预览Modal */}
      <Modal
        title="配置预览"
        open={previewModalVisible}
        onCancel={() => setPreviewModalVisible(false)}
        footer={null}
        width={800}
      >
        <pre style={{ background: '#f5f5f5', padding: 16, borderRadius: 4, maxHeight: 500, overflow: 'auto' }}>
          {JSON.stringify(generatePreview(), null, 2)}
        </pre>
      </Modal>

      {/* 测试验证Modal */}
      <Modal
        title="测试验证"
        open={testModalVisible}
        onCancel={() => setTestModalVisible(false)}
        footer={null}
        width={600}
      >
        <div style={{ textAlign: 'center', padding: '40px 0' }}>
          <PlayCircleOutlined style={{ fontSize: 48, color: '#ccc' }} />
          <div style={{ marginTop: 16, color: '#666' }}>
            测试功能开发中...
          </div>
        </div>
      </Modal>
    </Modal>
  );
}
