import React, { useCallback, useEffect, useRef, useState } from 'react';
import { App, Alert, Button, Card, Input, Popconfirm, Space, Tag, Typography } from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import { ReloadOutlined, SnippetsOutlined } from '@ant-design/icons';
import { FormattedMessage, useIntl } from '@umijs/max';
import {
  getCaptureGates,
  publishCaptureGates,
  type CaptureGatesState,
} from '@/services/api/opsCapture';
import { extractErrorMessage } from '@/utils/errors';

const { TextArea } = Input;
const { Text, Paragraph } = Typography;

// 空骨架：三段闸的规则数组留空 = 只采不限（与配置文件语义一致），
// 供运维复制后按游戏拓扑填规则，而不是对着空白 JSON 猜形状。
const GATES_TEMPLATE = `{
  "reconcile": {
    "rules": [
      {
        "name": "item-ledger",
        "itemTable": "game.items",
        "ledgerTable": "game.ledger",
        "itemKey": "1",
        "ledgerKey": "0",
        "windowSec": 300
      }
    ]
  },
  "cep": {
    "offHours": [],
    "burst": [],
    "probing": []
  },
  "lineage": {
    "rules": [
      {
        "name": "item-writes",
        "table": "game.items",
        "allowAccounts": ["game-app"]
      }
    ]
  }
}`;

export default function OpsCapturePage() {
  const { message } = App.useApp();
  const intl = useIntl();
  const intlRef = useRef(intl);
  intlRef.current = intl;

  const [state, setState] = useState<CaptureGatesState | null>(null);
  const [content, setContent] = useState('');
  const [loading, setLoading] = useState(false);
  const [publishing, setPublishing] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const s = await getCaptureGates();
      setState(s);
      setContent(s.content || '');
    } catch (error) {
      message.error(
        extractErrorMessage(
          error,
          intlRef.current.formatMessage({
            id: 'pages.opsCapture.error.loadFailed',
            defaultMessage: '加载闸规则失败',
          }),
        ),
      );
    } finally {
      setLoading(false);
    }
  }, [message]);

  useEffect(() => {
    load();
  }, [load]);

  const publish = useCallback(async () => {
    setPublishing(true);
    try {
      const resp = await publishCaptureGates({ content });
      message.success(
        intlRef.current.formatMessage(
          {
            id: 'pages.opsCapture.publish.success',
            defaultMessage: '已发布版本 {version}，capture-agent 将在轮询周期内热生效',
          },
          { version: resp.version },
        ),
      );
      await load();
    } catch (error) {
      message.error(
        extractErrorMessage(
          error,
          intlRef.current.formatMessage({
            id: 'pages.opsCapture.error.publishFailed',
            defaultMessage: '发布失败',
          }),
        ),
      );
    } finally {
      setPublishing(false);
    }
  }, [content, load, message]);

  const unpublished = !state || state.version === 0;

  return (
    <PageContainer>
      <Card
        title={<FormattedMessage id="pages.opsCapture.gates.title" defaultMessage="闸规则下发" />}
        extra={
          <Space>
            <Button icon={<ReloadOutlined />} onClick={load} loading={loading}>
              <FormattedMessage id="pages.opsCapture.reload" defaultMessage="重载" />
            </Button>
            <Button
              icon={<SnippetsOutlined />}
              onClick={() => setContent(GATES_TEMPLATE)}
              disabled={publishing}
            >
              <FormattedMessage id="pages.opsCapture.template" defaultMessage="插入模板" />
            </Button>
            <Popconfirm
              title={intlRef.current.formatMessage({
                id: 'pages.opsCapture.publish.confirm',
                defaultMessage: '发布为整份新版本并热下发到该 scope 的 capture-agent？',
              })}
              onConfirm={publish}
            >
              <Button type="primary" loading={publishing} disabled={loading || !content.trim()}>
                <FormattedMessage id="pages.opsCapture.publish" defaultMessage="发布" />
              </Button>
            </Popconfirm>
          </Space>
        }
      >
        <Space direction="vertical" style={{ width: '100%' }} size={12}>
          <Space wrap size={8}>
            <Text type="secondary">
              <FormattedMessage id="pages.opsCapture.gates.keyLabel" defaultMessage="配置键" />
            </Text>
            <Tag color="geekblue">capture.gates</Tag>
            {unpublished ? (
              <Tag>
                <FormattedMessage
                  id="pages.opsCapture.gates.unpublished"
                  defaultMessage="未发布（agent 用文件规则）"
                />
              </Tag>
            ) : (
              <>
                <Tag color="green">
                  <FormattedMessage
                    id="pages.opsCapture.gates.version"
                    defaultMessage="版本 {version}"
                    values={{ version: state?.version ?? 0 }}
                  />
                </Tag>
                {state?.updatedAt && (
                  <Text type="secondary">
                    {state.updatedAt}
                    {state.createdBy ? ` · ${state.createdBy}` : ''}
                  </Text>
                )}
              </>
            )}
          </Space>
          <Alert
            type="info"
            showIcon
            message={intlRef.current.formatMessage({
              id: 'pages.opsCapture.gates.boundary',
              defaultMessage:
                '服务端只做结构检查；语义校验（表路由/阈值）由 capture-agent 生效时全量执行，非法规则保留旧版。发布成功 ≠ 已生效，生效真值见 agent 日志 capture gates version applied。',
            })}
          />
          <TextArea
            value={content}
            onChange={(e) => setContent(e.target.value)}
            rows={18}
            style={{ fontFamily: 'monospace' }}
            placeholder={intlRef.current.formatMessage({
              id: 'pages.opsCapture.gates.placeholder',
              defaultMessage: 'capture.gates JSON（reconcile / cep / lineage 三段）',
            })}
          />
          <Paragraph type="secondary" style={{ marginBottom: 0 }}>
            <FormattedMessage
              id="pages.opsCapture.gates.help"
              defaultMessage="发布按顶栏 game/env scope 存 game 库（config_versions），capture-agent 经上行通道 30s 版本轮询拉取并整体热替换；断连或校验失败时按旧规则继续运行。"
            />
          </Paragraph>
        </Space>
      </Card>
    </PageContainer>
  );
}
