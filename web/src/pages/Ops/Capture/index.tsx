import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { App, Alert, Button, Card, Input, Popconfirm, Space, Tag, Tooltip, Typography } from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import { ReloadOutlined, SnippetsOutlined } from '@ant-design/icons';
import { FormattedMessage, useIntl } from '@umijs/max';
import {
  getCaptureGates,
  publishCaptureGates,
  type CaptureGatesState,
} from '@/services/api/opsCapture';
import { listOpsNodes, type OpsNode } from '@/services/api/ops';
import { summarizeGatesApply } from '@/utils/captureGatesApply';
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
  const [nodes, setNodes] = useState<OpsNode[]>([]);

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

  // 生效对账数据源：节点 labels 由 agent 注册/重注册上报
  // （captureGatesRevision=最近生效版本；role=capture 标识 capture-agent）。
  // 对账失败不打断编辑面（节点列表不可得 ≠ 规则不可用）。
  const loadNodes = useCallback(async () => {
    try {
      const resp = await listOpsNodes();
      setNodes(resp.nodes);
    } catch {
      setNodes([]);
    }
  }, []);

  useEffect(() => {
    load();
    loadNodes();
  }, [load, loadNodes]);

  const publish = useCallback(async () => {
    setPublishing(true);
    try {
      // 乐观锁基线=编辑面加载时看到的版本：被抢先发布则 409（服务端消息
      // 透传），防两人基于同一旧内容互相覆盖；未发布过（version=0）不带锁。
      const resp = await publishCaptureGates({
        content,
        baseVersion: state?.version || undefined,
      });
      message.success(
        intlRef.current.formatMessage(
          {
            id: 'pages.opsCapture.publish.success',
            defaultMessage: '已发布版本 {version}，capture-agent 将在轮询周期内热生效',
          },
          { version: resp.version },
        ),
      );
      await Promise.all([load(), loadNodes()]);
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
  }, [content, load, loadNodes, message, state?.version]);

  const unpublished = !state || state.version === 0;

  // desired vs applied（appliedRevision 上报链）：capture agent 自报的
  // 生效版本与当前 desired 版本对账。汇总逻辑在 utils/captureGatesApply
  // （/risk/strategies 风控策略页复用）；自报滞后正常（30s 拉取 + 重注册）。
  const applySummary = useMemo(
    () => summarizeGatesApply(nodes, state?.version ?? 0),
    [nodes, state?.version],
  );

  return (
    <PageContainer>
      <Card
        title={<FormattedMessage id="pages.opsCapture.gates.title" defaultMessage="闸规则下发" />}
        extra={
          <Space>
            <Button
              icon={<ReloadOutlined />}
              onClick={() => {
                load();
                loadNodes();
              }}
              loading={loading}
            >
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
          {applySummary && (
            <div>
              <Text type="secondary">
                <FormattedMessage
                  id="pages.opsCapture.apply.summary"
                  defaultMessage="生效对账（agent 自报）："
                />
              </Text>{' '}
              {applySummary.total === 0 ? (
                <Tag>
                  <FormattedMessage
                    id="pages.opsCapture.apply.noAgents"
                    defaultMessage="scope 内无在线 capture-agent"
                  />
                </Tag>
              ) : applySummary.stale.length === 0 ? (
                <Tag color="green">
                  <FormattedMessage
                    id="pages.opsCapture.apply.allApplied"
                    defaultMessage="{count}/{total} 台已生效"
                    values={{ count: applySummary.applied.length, total: applySummary.total }}
                  />
                </Tag>
              ) : (
                <Tooltip
                  title={applySummary.stale
                    .map(
                      (n) =>
                        `${n.id}: ${n.labels?.captureGatesRevision || intlRef.current.formatMessage({ id: 'pages.opsCapture.apply.unknown', defaultMessage: '未上报' })}`,
                    )
                    .join('; ')}
                >
                  <Tag color="gold">
                    <FormattedMessage
                      id="pages.opsCapture.apply.partial"
                      defaultMessage="{count}/{total} 台已生效（其余未上报或滞后）"
                      values={{ count: applySummary.applied.length, total: applySummary.total }}
                    />
                  </Tag>
                </Tooltip>
              )}
            </div>
          )}
          <Alert
            type="info"
            showIcon
            message={intlRef.current.formatMessage({
              id: 'pages.opsCapture.gates.boundary',
              defaultMessage:
                '服务端只做结构检查；语义校验（表路由/阈值）由 capture-agent 生效时全量执行，非法规则保留旧版。发布成功 ≠ 已生效，以上「生效对账」为 agent 自报（轮询 30s + 重注册滞后），日志 capture gates version applied 仍是逐次生效的原始真值。',
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
