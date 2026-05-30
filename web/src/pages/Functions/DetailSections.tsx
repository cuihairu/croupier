import React from 'react';
import {
  Alert,
  Button,
  Card,
  Col,
  Descriptions,
  Divider,
  Form,
  Input,
  Row,
  Select,
  Space,
  Switch,
  Tag,
} from 'antd';
import type { FormInstance } from 'antd/es/form';
import { CopyOutlined } from '@ant-design/icons';
import { CodeEditor } from '@/components/MonacoDynamic';

const { TextArea } = Input;

export type FunctionDetailData = {
  id: string;
  description?: string;
  category?: string;
  version?: string;
  enabled: boolean;
  tags?: string[];
  createdAt: string;
  updatedAt: string;
  provider?: string;
  agentCount?: number;
  health?: 'healthy' | 'unhealthy' | 'unknown';
};

export function JsonViewer({
  data,
  onCopySuccess,
  onCopyError,
}: {
  data: any;
  onCopySuccess: () => void;
  onCopyError: () => void;
}) {
  const pretty = JSON.stringify(data || {}, null, 2);

  const beforeMount = (monaco: any) => {
    if (!monaco?.editor || monaco.editor.getTheme?.() === 'sublime-monokai') return;
    monaco.editor.defineTheme('sublime-monokai', {
      base: 'vs-dark',
      inherit: true,
      rules: [
        { token: 'string.key.json', foreground: '66D9EF' },
        { token: 'string.value.json', foreground: 'A6E22E' },
        { token: 'number', foreground: 'E6DB74' },
        { token: 'keyword', foreground: 'F92672' },
      ],
      colors: {
        'editor.background': '#272822',
        'editorLineNumber.foreground': '#75715E',
        'editorLineNumber.activeForeground': '#F8F8F2',
      },
    });
  };

  const copyJson = async () => {
    try {
      await navigator.clipboard.writeText(pretty);
      onCopySuccess();
    } catch {
      onCopyError();
    }
  };

  return (
    <div
      style={{
        border: '1px solid #f0f0f0',
        borderRadius: 6,
        overflow: 'hidden',
        background: '#fafafa',
      }}
    >
      <div
        style={{
          display: 'flex',
          justifyContent: 'flex-end',
          alignItems: 'center',
          padding: '8px 12px',
          borderBottom: '1px solid #f0f0f0',
          background: '#fff',
        }}
      >
        <Button size="small" icon={<CopyOutlined />} onClick={copyJson}>
          复制
        </Button>
      </div>
      <CodeEditor
        value={pretty}
        language="json"
        height={500}
        readOnly
        theme="sublime-monokai"
        beforeMount={beforeMount}
        options={{
          lineNumbers: 'on',
          renderLineHighlight: 'line',
          scrollBeyondLastLine: false,
          automaticLayout: true,
          minimap: { enabled: false },
        }}
      />
    </div>
  );
}

export function BasicInfoTab({
  functionDetail,
  effectiveCategory,
  editing,
  onStatusToggle,
}: {
  functionDetail: FunctionDetailData | null;
  effectiveCategory: string;
  editing: boolean;
  onStatusToggle: (enabled: boolean) => void;
}) {
  return (
    <>
      <Descriptions bordered column={2}>
        <Descriptions.Item label="函数ID">
          <code>{functionDetail?.id}</code>
        </Descriptions.Item>
        <Descriptions.Item label="版本">
          <Tag>{functionDetail?.version || '1.0.0'}</Tag>
        </Descriptions.Item>
        <Descriptions.Item label="分类">
          <Tag color="blue">{effectiveCategory || '默认'}</Tag>
        </Descriptions.Item>
        <Descriptions.Item label="状态">
          <Space>
            <Switch checked={functionDetail?.enabled || false} onChange={onStatusToggle} />
            <span>{functionDetail?.enabled ? '已启用' : '已禁用'}</span>
          </Space>
        </Descriptions.Item>
        <Descriptions.Item label="Provider">{functionDetail?.provider || '-'}</Descriptions.Item>
        <Descriptions.Item label="健康状态">
          <Tag
            color={
              functionDetail?.health === 'healthy'
                ? 'green'
                : functionDetail?.health === 'unhealthy'
                ? 'red'
                : 'gray'
            }
          >
            {functionDetail?.health === 'healthy'
              ? '健康'
              : functionDetail?.health === 'unhealthy'
              ? '异常'
              : '未知'}
          </Tag>
        </Descriptions.Item>
        <Descriptions.Item label="Agent 数量">{functionDetail?.agentCount || 0}</Descriptions.Item>
        <Descriptions.Item label="创建时间">
          {functionDetail?.createdAt ? new Date(functionDetail.createdAt).toLocaleString() : '-'}
        </Descriptions.Item>
        <Descriptions.Item label="更新时间">
          {functionDetail?.updatedAt ? new Date(functionDetail.updatedAt).toLocaleString() : '-'}
        </Descriptions.Item>
      </Descriptions>

      {editing && (
        <>
          <Divider>编辑信息</Divider>
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item
                label="函数名称"
                name="name"
                rules={[{ required: true, message: '请输入函数名称' }]}
              >
                <Input placeholder="请输入函数名称" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item label="分类" name="category">
                <Input placeholder="请输入分类" />
              </Form.Item>
            </Col>
          </Row>
          <Form.Item label="描述" name="description">
            <TextArea rows={3} placeholder="请输入函数描述" />
          </Form.Item>
          <Form.Item label="标签" name="tags">
            <Input placeholder="请输入标签，多个标签用逗号分隔" />
          </Form.Item>
        </>
      )}

      {!editing && (
        <>
          <Divider>描述</Divider>
          <p>{functionDetail?.description || '暂无描述'}</p>
        </>
      )}

      {!editing && functionDetail?.tags && functionDetail.tags.length > 0 && (
        <>
          <Divider>标签</Divider>
          <Space wrap>
            {functionDetail.tags.map((tag) => (
              <Tag key={tag} color="geekblue">
                {tag}
              </Tag>
            ))}
          </Space>
        </>
      )}
    </>
  );
}

export function PermissionsTab({
  functionId,
  permError,
  permLoading,
  permSaving,
  permForm,
  onSave,
}: {
  functionId?: string;
  permError: string;
  permLoading: boolean;
  permSaving: boolean;
  permForm: FormInstance;
  onSave: () => Promise<void>;
}) {
  return (
    <>
      <Alert
        message="权限配置"
        description="用于控制哪些角色可以调用该函数（actions 建议使用 invoke/execute；roles 填角色名）。"
        type="info"
        showIcon
      />

      {permError && (
        <Alert
          style={{ marginTop: 16 }}
          type="error"
          showIcon
          message="无法读取权限"
          description={permError}
        />
      )}

      <Card style={{ marginTop: 16 }} loading={permLoading} size="small" title="函数权限规则">
        <Form form={permForm} layout="vertical">
          <Form.List name="items">
            {(fields, { add, remove }) => (
              <Space direction="vertical" style={{ width: '100%' }} size="middle">
                {fields.map((field) => (
                  <Card
                    key={field.key}
                    size="small"
                    type="inner"
                    title={`规则 #${field.name + 1}`}
                    extra={
                      <Button danger size="small" onClick={() => remove(field.name)}>
                        删除
                      </Button>
                    }
                  >
                    <Row gutter={16}>
                      <Col span={6}>
                        <Form.Item
                          {...field}
                          label="resource"
                          name={[field.name, 'resource']}
                          rules={[{ required: true, message: 'resource 必填' }]}
                        >
                          <Input placeholder="function" />
                        </Form.Item>
                      </Col>
                      <Col span={6}>
                        <Form.Item
                          {...field}
                          label="actions"
                          name={[field.name, 'actions']}
                          rules={[{ required: true, message: 'actions 必填' }]}
                        >
                          <Select mode="tags" placeholder="invoke / execute" />
                        </Form.Item>
                      </Col>
                      <Col span={6}>
                        <Form.Item
                          {...field}
                          label="roles"
                          name={[field.name, 'roles']}
                          rules={[{ required: true, message: 'roles 必填（至少 1 个）' }]}
                        >
                          <Select mode="tags" placeholder="例如：ops / admin / functions:manage" />
                        </Form.Item>
                      </Col>
                      <Col span={3}>
                        <Form.Item {...field} label="gameId" name={[field.name, 'gameId']}>
                          <Input placeholder="(all)" />
                        </Form.Item>
                      </Col>
                      <Col span={3}>
                        <Form.Item {...field} label="env" name={[field.name, 'env']}>
                          <Input placeholder="(all)" />
                        </Form.Item>
                      </Col>
                    </Row>
                  </Card>
                ))}

                <Space>
                  <Button
                    onClick={() => add({ resource: 'function', actions: ['invoke'], roles: [] })}
                  >
                    添加规则
                  </Button>
                  <Button
                    type="primary"
                    loading={permSaving}
                    disabled={!functionId}
                    onClick={onSave}
                  >
                    保存权限
                  </Button>
                </Space>
              </Space>
            )}
          </Form.List>
        </Form>
      </Card>
    </>
  );
}
