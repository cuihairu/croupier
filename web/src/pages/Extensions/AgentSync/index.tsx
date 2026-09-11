import React, { useState } from 'react';
import { App, Button, Card, Form, Input, Space, Typography } from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { getAgentSyncPayload } from '@/services/api/extensions';

const { Text } = Typography;

export default function ExtensionAgentSyncPage() {
  const { message } = App.useApp();
  const intl = useIntl();
  const [loading, setLoading] = useState(false);
  const [agentID, setAgentID] = useState('');
  const [payloadText, setPayloadText] = useState('');

  const runSyncQuery = async () => {
    const value = agentID.trim();
    if (!value) {
      message.warning(
        intl.formatMessage({
          id: 'pages.extensionsAgentSync.query.emptyWarning',
          defaultMessage: '请输入 Agent ID',
        }),
      );
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
    <PageContainer
      title={intl.formatMessage({
        id: 'pages.extensionsAgentSync.page.title',
        defaultMessage: 'Agent 扩展同步调试',
      })}
      subTitle={intl.formatMessage({
        id: 'pages.extensionsAgentSync.page.subTitle',
        defaultMessage: '查看指定 Agent 的扩展同步载荷',
      })}
    >
      <Card>
        <Form layout="vertical" onFinish={runSyncQuery}>
          <Form.Item label="Agent ID" required>
            <Input
              placeholder={intl.formatMessage({
                id: 'pages.extensionsAgentSync.form.agentIdPlaceholder',
                defaultMessage: '例如: agent-001',
              })}
              value={agentID}
              onChange={(e) => setAgentID(e.target.value)}
              onPressEnter={() => runSyncQuery()}
            />
          </Form.Item>
          <Space>
            <Button type="primary" loading={loading} onClick={runSyncQuery}>
              <FormattedMessage
                id="pages.extensionsAgentSync.query.submit"
                defaultMessage="查询同步载荷"
              />
            </Button>
            <Button
              onClick={() => {
                setAgentID('');
                setPayloadText('');
              }}
            >
              <FormattedMessage id="pages.extensionsAgentSync.query.reset" defaultMessage="清空" />
            </Button>
          </Space>
        </Form>
      </Card>

      <Card style={{ marginTop: 16 }} title="Payload JSON">
        {!payloadText ? (
          <Text type="secondary">
            <FormattedMessage
              id="pages.extensionsAgentSync.payload.empty"
              defaultMessage="暂无数据"
            />
          </Text>
        ) : (
          <Input.TextArea value={payloadText} readOnly autoSize={{ minRows: 14, maxRows: 28 }} />
        )}
      </Card>
    </PageContainer>
  );
}
