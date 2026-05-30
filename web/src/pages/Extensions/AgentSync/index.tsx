import React, { useState } from 'react';
import { App, Button, Card, Form, Input, Space, Typography } from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import { getAgentSyncPayload } from '@/services/api/extensions';

const { Text } = Typography;

export default function ExtensionAgentSyncPage() {
  const { message } = App.useApp();
  const [loading, setLoading] = useState(false);
  const [agentID, setAgentID] = useState('');
  const [payloadText, setPayloadText] = useState('');

  const runSyncQuery = async () => {
    const value = agentID.trim();
    if (!value) {
      message.warning('请输入 Agent ID');
      return;
    }
    setLoading(true);
    try {
      const resp = await getAgentSyncPayload(value);
      setPayloadText(JSON.stringify(resp?.payload || {}, null, 2));
    } finally {
      setLoading(false);
    }
  };

  return (
    <PageContainer title="Agent 扩展同步调试" subTitle="查看指定 Agent 的扩展同步载荷">
      <Card>
        <Form layout="vertical" onFinish={runSyncQuery}>
          <Form.Item label="Agent ID" required>
            <Input
              placeholder="例如: agent-001"
              value={agentID}
              onChange={(e) => setAgentID(e.target.value)}
              onPressEnter={() => runSyncQuery()}
            />
          </Form.Item>
          <Space>
            <Button type="primary" loading={loading} onClick={runSyncQuery}>
              查询同步载荷
            </Button>
            <Button
              onClick={() => {
                setAgentID('');
                setPayloadText('');
              }}
            >
              清空
            </Button>
          </Space>
        </Form>
      </Card>

      <Card style={{ marginTop: 16 }} title="Payload JSON">
        {!payloadText ? (
          <Text type="secondary">暂无数据</Text>
        ) : (
          <Input.TextArea value={payloadText} readOnly autoSize={{ minRows: 14, maxRows: 28 }} />
        )}
      </Card>
    </PageContainer>
  );
}
