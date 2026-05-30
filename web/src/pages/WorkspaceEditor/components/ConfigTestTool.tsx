/**
 * 配置测试工具
 *
 * 使用模拟数据测试配置效果，包括表单提交、列表渲染、详情展示等。
 *
 * @module pages/WorkspaceEditor/components/ConfigTestTool
 */

import React, { useState } from 'react';
import {
  Modal,
  Tabs,
  Button,
  Space,
  Typography,
  Card,
  Form,
  Input,
  Alert,
  Descriptions,
  Table,
  Tag,
  Switch,
  InputNumber,
  message,
  Spin,
  Empty,
} from 'antd';
import {
  PlayCircleOutlined,
  BugOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  ReloadOutlined,
} from '@ant-design/icons';
import type {
  TabConfig,
  TabLayout,
  FieldConfig,
  ColumnConfig,
  DetailSection,
} from '@/types/workspace';
import type { FunctionDescriptor } from '@/services/api/functions';

const { Text, Paragraph } = Typography;
const { TabPane } = Tabs;

interface ConfigTestToolProps {
  visible: boolean;
  onClose: () => void;
  tab: TabConfig;
  descriptors: FunctionDescriptor[];
}

/** 模拟数据生成器 */
function generateMockData(type: string, schema?: any): any {
  const mockData: Record<string, any> = {
    string: '测试文本',
    number: 12345,
    date: '2024-01-15',
    datetime: '2024-01-15 14:30:00',
    email: 'test@example.com',
    url: 'https://example.com',
    phone: '13800138000',
    boolean: true,
    array: ['选项1', '选项2', '选项3'],
    object: { key: 'value', nested: { data: 'test' } },
  };

  // 根据字段类型生成对应数据
  if (type === 'input' || type === 'textarea') return mockData.string;
  if (type === 'number') return mockData.number;
  if (type === 'date') return mockData.date;
  if (type === 'datetime') return mockData.datetime;
  if (type === 'select' || type === 'radio') {
    return schema?.options?.[0]?.value || 'option1';
  }
  if (type === 'checkbox') return [schema?.options?.[0]?.value || 'option1'];
  if (type === 'switch') return true;

  return mockData.string;
}

/** 生成列表模拟数据 */
function generateListMockData(columns: ColumnConfig[], count: number = 5): any[] {
  return Array.from({ length: count }, (_, i) => {
    const item: any = { id: `mock_${i + 1}` };
    columns.forEach((col) => {
      item[col.key] = generateMockData(
        col.render === 'money'
          ? 'number'
          : col.render === 'datetime' || col.render === 'date'
          ? 'datetime'
          : col.render === 'tag'
          ? 'select'
          : 'string',
      );
    });
    return item;
  });
}

/** 生成表单模拟数据 */
function generateFormMockData(fields: FieldConfig[]): Record<string, any> {
  const data: Record<string, any> = {};
  fields.forEach((field) => {
    if (!field.key) return;
    data[field.key] = generateMockData(field.type, field);
  });
  return data;
}

/** 生成详情模拟数据 */
function generateDetailMockData(sections: DetailSection[]): Record<string, any> {
  const data: Record<string, any> = { id: 'mock_001' };
  sections.forEach((section) => {
    section.fields.forEach((field) => {
      if (!field.key) return;
      data[field.key] = generateMockData('string');
    });
  });
  return data;
}

/** 表单测试面板 */
function FormTestPanel({
  tab,
  descriptors,
}: {
  tab: TabConfig;
  descriptors: FunctionDescriptor[];
}) {
  const layout = tab.layout as any;
  const fields = layout?.fields || [];

  const mockData = React.useMemo(() => generateFormMockData(fields), [fields]);

  const [form] = Form.useForm();

  const handleSubmit = () => {
    form
      .validateFields()
      .then((values) => {
        console.log('表单提交数据:', values);
        message.success('表单验证通过！查看控制台输出');
      })
      .catch((err) => {
        console.error('表单验证失败:', err);
        message.error('表单验证失败，请检查必填项');
      });
  };

  const renderField = (field: FieldConfig) => {
    const commonProps = {
      placeholder: field.placeholder,
      disabled: field.disabled,
    };

    switch (field.type) {
      case 'input':
        return <Input {...commonProps} />;
      case 'number':
        return <InputNumber {...commonProps} style={{ width: '100%' }} />;
      case 'textarea':
        return <Input.TextArea rows={3} {...commonProps} />;
      case 'select':
        return (
          <select className="ant-input" {...commonProps}>
            {field.options?.map((opt) => (
              <option key={opt.value} value={opt.value}>
                {opt.label}
              </option>
            ))}
          </select>
        );
      case 'switch':
        return <Switch />;
      default:
        return <Input {...commonProps} />;
    }
  };

  return (
    <div>
      <Alert
        message="表单测试"
        description="使用模拟数据测试表单布局和验证规则"
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
      />
      <Form form={form} layout={layout.formLayout || 'vertical'} initialValues={mockData}>
        {fields.map((field) => (
          <Form.Item
            key={field.key}
            name={field.key}
            label={field.label}
            required={field.required}
            rules={field.rules?.map((r) => ({
              required: r.type === 'required',
              message: r.message,
            }))}
            tooltip={field.tooltip}
          >
            {renderField(field)}
          </Form.Item>
        ))}
      </Form>
      <Space style={{ marginTop: 16 }}>
        <Button type="primary" onClick={handleSubmit}>
          测试提交
        </Button>
        <Button onClick={() => form.resetFields()}>重置</Button>
        <Button onClick={() => form.setFieldsValue(mockData)}>填充模拟数据</Button>
      </Space>
    </div>
  );
}

/** 列表测试面板 */
function ListTestPanel({ tab }: { tab: TabConfig }) {
  const layout = tab.layout as any;
  const columns = layout?.columns || [];
  const [mockData, setMockData] = React.useState<any[]>([]);
  const [loading, setLoading] = useState(false);

  const handleLoadData = () => {
    setLoading(true);
    setTimeout(() => {
      setMockData(generateListMockData(columns, 5));
      setLoading(false);
    }, 500);
  };

  React.useEffect(() => {
    handleLoadData();
  }, [columns]);

  const tableColumns = columns.map((col: ColumnConfig) => ({
    title: col.title,
    dataIndex: col.key,
    key: col.key,
    width: col.width,
    fixed: col.fixed,
    align: col.align || 'left',
    ellipsis: col.ellipsis,
    render: (value: any) => {
      if (col.render === 'tag') {
        return <Tag>{value || '-'}</Tag>;
      }
      if (col.render === 'datetime') {
        return <Text type="secondary">{value || '-'}</Text>;
      }
      if (col.render === 'money') {
        return <Text>¥{Number(value || 0).toLocaleString()}</Text>;
      }
      return value ?? '-';
    },
  }));

  return (
    <div>
      <Alert
        message="列表测试"
        description="使用模拟数据测试列表布局和渲染效果"
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
      />
      <Space style={{ marginBottom: 12 }}>
        <Button icon={<ReloadOutlined />} onClick={handleLoadData}>
          刷新数据
        </Button>
        <Text type="secondary">共 {mockData.length} 条模拟数据</Text>
      </Space>
      <Table
        columns={tableColumns}
        dataSource={mockData}
        loading={loading}
        size="small"
        pagination={{ pageSize: 10 }}
        scroll={{ x: 'max-content' }}
      />
    </div>
  );
}

/** 详情测试面板 */
function DetailTestPanel({ tab }: { tab: TabConfig }) {
  const layout = tab.layout as any;
  const sections = layout?.sections || [];

  const mockData = React.useMemo(() => generateDetailMockData(sections), [sections]);

  return (
    <div>
      <Alert
        message="详情测试"
        description="使用模拟数据测试详情布局和字段显示"
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
      />
      {sections.map((section: DetailSection, idx: number) => (
        <Card key={idx} title={section.title} size="small" style={{ marginBottom: 12 }}>
          <Descriptions column={section.column || 2} size="small" bordered>
            {section.fields.map((field) => (
              <Descriptions.Item key={field.key} label={field.label} span={field.span || 1}>
                {mockData[field.key] || '-'}
              </Descriptions.Item>
            ))}
          </Descriptions>
        </Card>
      ))}
    </div>
  );
}

/** Form-Detail 测试面板 */
function FormDetailTestPanel({ tab }: { tab: TabConfig }) {
  const layout = tab.layout as any;
  const queryFields = layout?.queryFields || [];
  const detailSections = layout?.detailSections || [];

  const mockQueryData = React.useMemo(() => generateFormMockData(queryFields), [queryFields]);
  const mockDetailData = React.useMemo(
    () => generateDetailMockData(detailSections),
    [detailSections],
  );

  return (
    <div>
      <Alert
        message="Form-Detail 测试"
        description="测试查询表单和详情展示的组合效果"
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
      />
      <Card title="查询表单" size="small" style={{ marginBottom: 12 }}>
        <Form layout="inline" initialValues={mockQueryData}>
          {queryFields.map((field: FieldConfig) => (
            <Form.Item key={field.key} name={field.key} label={field.label}>
              <Input placeholder={field.placeholder} style={{ width: 150 }} />
            </Form.Item>
          ))}
          <Form.Item>
            <Button type="primary">查询</Button>
          </Form.Item>
        </Form>
      </Card>
      {detailSections.map((section: DetailSection, idx: number) => (
        <Card key={idx} title={section.title} size="small" style={{ marginBottom: 12 }}>
          <Descriptions column={section.column || 2} size="small" bordered>
            {section.fields.map((field) => (
              <Descriptions.Item key={field.key} label={field.label} span={field.span || 1}>
                {mockDetailData[field.key] || '-'}
              </Descriptions.Item>
            ))}
          </Descriptions>
        </Card>
      ))}
    </div>
  );
}

export default function ConfigTestTool({
  visible,
  onClose,
  tab,
  descriptors,
}: ConfigTestToolProps) {
  const [activeTab, setActiveTab] = useState('form');
  const layout = tab.layout as any;

  // 根据布局类型确定可用的测试面板
  const getAvailableTests = () => {
    const tests: { key: string; title: string; icon: React.ReactNode }[] = [];

    switch (layout?.type) {
      case 'form':
        tests.push({ key: 'form', title: '表单测试', icon: <BugOutlined /> });
        break;
      case 'list':
        tests.push({ key: 'list', title: '列表测试', icon: <BugOutlined /> });
        break;
      case 'detail':
        tests.push({ key: 'detail', title: '详情测试', icon: <BugOutlined /> });
        break;
      case 'form-detail':
        tests.push({ key: 'form-detail', title: '表单-详情测试', icon: <BugOutlined /> });
        break;
      default:
        tests.push({ key: 'form', title: '表单测试', icon: <BugOutlined /> });
        tests.push({ key: 'list', title: '列表测试', icon: <BugOutlined /> });
        tests.push({ key: 'detail', title: '详情测试', icon: <BugOutlined /> });
    }

    return tests;
  };

  const availableTests = getAvailableTests();

  const renderTestPanel = () => {
    switch (activeTab) {
      case 'form':
        return <FormTestPanel tab={tab} descriptors={descriptors} />;
      case 'list':
        return <ListTestPanel tab={tab} />;
      case 'detail':
        return <DetailTestPanel tab={tab} />;
      case 'form-detail':
        return <FormDetailTestPanel tab={tab} />;
      default:
        return <Empty description="不支持的布局类型" />;
    }
  };

  return (
    <Modal
      title={
        <Space>
          <PlayCircleOutlined />
          配置测试工具 - {tab.title}
        </Space>
      }
      open={visible}
      onCancel={onClose}
      width={800}
      footer={[
        <Button key="close" onClick={onClose}>
          关闭
        </Button>,
      ]}
    >
      <Tabs activeKey={activeTab} onChange={setActiveTab} size="small">
        {availableTests.map((test) => (
          <TabPane
            key={test.key}
            tab={
              <Space>
                {test.icon}
                {test.title}
              </Space>
            }
          >
            {renderTestPanel()}
          </TabPane>
        ))}
      </Tabs>
    </Modal>
  );
}
