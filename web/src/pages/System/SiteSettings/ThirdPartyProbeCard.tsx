/**
 * 第三方服务健康探针卡（运维 Tab，OPEN-ISSUES #57）。
 *
 * 对已配置的第三方外呼目标做手动探活：通知 webhook 四渠道（HTTP GET
 * 只读探测，不真正发消息）、检查更新源（GET）、SMTP（TCP+EHLO，不发信
 * 不认证）。探测走出站守卫与 net.* 超时策略，结果展示延迟/状态/错误。
 *
 * 边界（诚实）：手动触发非周期监控（无后台探活/告警联动）；未配置目标
 * 显示「未配置」；LDAP/OIDC/GitHub 身份源不在本批探针范围。
 */
import React, { useState } from 'react';
import { Button, Card, Space, Tag, Typography } from 'antd';
import { FormattedMessage, useIntl } from '@umijs/max';
import {
  probeThirdParty,
  type ThirdPartyProbeChannel,
  type ThirdPartyProbeView,
} from '@/services/api/thirdPartyProbe';
import { extractErrorMessage } from '@/utils/errors';

const { Text } = Typography;

const CHANNELS: { key: ThirdPartyProbeChannel; labelId: string; defaultMessage: string }[] = [
  {
    key: 'dingtalk',
    labelId: 'pages.systemSiteSettings.maintenance.probe.channel.dingtalk',
    defaultMessage: '钉钉机器人',
  },
  {
    key: 'wecom',
    labelId: 'pages.systemSiteSettings.maintenance.probe.channel.wecom',
    defaultMessage: '企业微信机器人',
  },
  {
    key: 'feishu',
    labelId: 'pages.systemSiteSettings.maintenance.probe.channel.feishu',
    defaultMessage: '飞书机器人',
  },
  {
    key: 'webhook',
    labelId: 'pages.systemSiteSettings.maintenance.probe.channel.webhook',
    defaultMessage: '通用 Webhook',
  },
  {
    key: 'update',
    labelId: 'pages.systemSiteSettings.maintenance.probe.channel.update',
    defaultMessage: '检查更新源',
  },
  {
    key: 'smtp',
    labelId: 'pages.systemSiteSettings.maintenance.probe.channel.smtp',
    defaultMessage: 'SMTP 邮件服务',
  },
];

type ProbeState = { loading: boolean; result?: ThirdPartyProbeView };

export default function ThirdPartyProbeCard() {
  const intl = useIntl();
  const [states, setStates] = useState<Record<string, ProbeState>>({});

  const runProbe = async (channel: ThirdPartyProbeChannel) => {
    setStates((prev) => ({ ...prev, [channel]: { loading: true } }));
    try {
      const result = await probeThirdParty(channel);
      setStates((prev) => ({ ...prev, [channel]: { loading: false, result } }));
    } catch (error) {
      setStates((prev) => ({
        ...prev,
        [channel]: {
          loading: false,
          result: {
            channel,
            configured: true,
            ok: false,
            status: 0,
            latencyMs: 0,
            error: extractErrorMessage(
              error,
              intl.formatMessage({
                id: 'pages.systemSiteSettings.maintenance.probe.error.failed',
                defaultMessage: '探测失败',
              }),
            ),
          },
        },
      }));
    }
  };

  const renderResult = (channel: ThirdPartyProbeChannel) => {
    const st = states[channel];
    if (!st?.result) {
      return null;
    }
    const r = st.result;
    if (!r.configured) {
      return (
        <Tag data-channel={channel}>
          <FormattedMessage
            id="pages.systemSiteSettings.maintenance.probe.unconfigured"
            defaultMessage="未配置"
          />
        </Tag>
      );
    }
    if (r.ok) {
      return (
        <Tag color="green" data-channel={channel}>
          <FormattedMessage
            id="pages.systemSiteSettings.maintenance.probe.healthy"
            defaultMessage="健康"
          />{' '}
          · {r.latencyMs}ms{r.status > 0 ? ` · HTTP ${r.status}` : ''}
        </Tag>
      );
    }
    return (
      <Tag color="red" data-channel={channel}>
        <FormattedMessage
          id="pages.systemSiteSettings.maintenance.probe.unhealthy"
          defaultMessage="异常"
        />{' '}
        · {r.error || `HTTP ${r.status}`}
      </Tag>
    );
  };

  return (
    <Card
      title={
        <FormattedMessage
          id="pages.systemSiteSettings.maintenance.probe.title"
          defaultMessage="第三方服务健康探针"
        />
      }
    >
      <Text type="secondary">
        <FormattedMessage
          id="pages.systemSiteSettings.maintenance.probe.hint"
          defaultMessage="对已配置的第三方目标手动探活：webhook 渠道为只读 GET（不发送通知消息），SMTP 为 TCP+EHLO（不发信不认证）。"
        />
      </Text>
      <Space orientation="vertical" size="small" style={{ width: '100%', marginTop: 16 }}>
        {CHANNELS.map((ch) => (
          <Space key={ch.key} wrap>
            <Text strong style={{ minWidth: 120, display: 'inline-block' }}>
              <FormattedMessage id={ch.labelId} defaultMessage={ch.defaultMessage} />
            </Text>
            <Button
              size="small"
              loading={states[ch.key]?.loading}
              onClick={() => void runProbe(ch.key)}
            >
              <FormattedMessage
                id="pages.systemSiteSettings.maintenance.probe.action"
                defaultMessage="探测"
              />
            </Button>
            {renderResult(ch.key)}
          </Space>
        ))}
      </Space>
    </Card>
  );
}
