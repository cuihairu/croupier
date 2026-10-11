/**
 * 外部服务清单卡（运维 Tab，server-status-provider 批 S3）。
 *
 * 只读呈现文件侧平台接口位外部服务的启用态（GET /site/external-services，
 * L2 配置文件真值，L3 数据库覆盖不适用）：herald 告警出口 / alertInbound
 * 告警入站（alertmanager/generic）/ serverStatus 维护状态源。状态三态：
 * active=启用态且必填项全齐（与 svc 装配判定同口径）；configured=配置文件
 * 有该段但缺项（缺项点名配置键或环境变量名）；未配置。
 *
 * 脱敏：只显示环境变量名与「已设置/未设置」，永不回显值（与 notification
 * 快照 smtpPasswordSet 口径一致）。
 *
 * 边界（诚实）：页面侧配置编辑属「配置整理」单（docs/design/
 * config-organization.md，待用户过目）；本卡不可编辑，变更需编辑
 * server.yaml 后重启。
 */
import React, { useEffect, useRef, useState } from 'react';
import { Alert, Card, Space, Tag, Typography } from 'antd';
import { FormattedMessage, useIntl } from '@umijs/max';
import {
  fetchExternalServices,
  type ExternalServiceEntry,
  type ExternalServicesSnapshot,
} from '@/services/api/sites';
import { extractErrorMessage } from '@/utils/errors';

const { Text } = Typography;

/** 服务名→展示文案：动态 provider 名（serverStatus.<provider>）走前缀匹配 */
function serviceLabel(
  format: (id: string, def: string, values?: Record<string, string>) => string,
  name: string,
): string {
  if (name === 'herald') {
    return format(
      'pages.systemSiteSettings.maintenance.externalServices.service.herald',
      '告警出口（herald）',
    );
  }
  if (name.startsWith('alertInbound.')) {
    return format(
      'pages.systemSiteSettings.maintenance.externalServices.service.alertInbound',
      '告警入站（{source}）',
      {
        source: name.slice('alertInbound.'.length),
      },
    );
  }
  if (name.startsWith('serverStatus.')) {
    return format(
      'pages.systemSiteSettings.maintenance.externalServices.service.serverStatus',
      '状态源（{provider}）',
      {
        provider: name.slice('serverStatus.'.length),
      },
    );
  }
  return name;
}

function stateTag(svc: ExternalServiceEntry): React.ReactNode {
  if (svc.active) {
    return (
      <Tag color="success" data-state="active">
        <FormattedMessage
          id="pages.systemSiteSettings.maintenance.externalServices.state.active"
          defaultMessage="已启用"
        />
      </Tag>
    );
  }
  if (svc.configured) {
    return (
      <Tag color="warning" data-state="inactive">
        <FormattedMessage
          id="pages.systemSiteSettings.maintenance.externalServices.state.inactive"
          defaultMessage="配置未生效"
        />
      </Tag>
    );
  }
  return (
    <Tag data-state="unconfigured">
      <FormattedMessage
        id="pages.systemSiteSettings.maintenance.externalServices.state.unconfigured"
        defaultMessage="未配置"
      />
    </Tag>
  );
}

export default function ExternalServicesCard() {
  const intl = useIntl();
  const [snapshot, setSnapshot] = useState<ExternalServicesSnapshot | null>(null);
  const [error, setError] = useState<string>('');
  const mountedRef = useRef(true);
  useEffect(() => {
    mountedRef.current = true;
    fetchExternalServices()
      .then((snap) => {
        if (mountedRef.current) setSnapshot(snap);
      })
      .catch((err: unknown) => {
        if (!mountedRef.current) return;
        setError(
          extractErrorMessage(
            err,
            intl.formatMessage({
              id: 'pages.systemSiteSettings.maintenance.externalServices.loadFailed',
              defaultMessage: '加载外部服务清单失败',
            }),
          ),
        );
      });
    return () => {
      mountedRef.current = false;
    };
  }, []);

  const format = (id: string, def: string, values?: Record<string, string>) =>
    intl.formatMessage({ id, defaultMessage: def }, values);

  return (
    <Card
      size="small"
      title={
        <FormattedMessage
          id="pages.systemSiteSettings.maintenance.externalServices.title"
          defaultMessage="外部服务（配置文件）"
        />
      }
    >
      <Space orientation="vertical" style={{ width: '100%' }} size="small">
        <Text type="secondary">
          <FormattedMessage
            id="pages.systemSiteSettings.maintenance.externalServices.subtitle"
            defaultMessage="只读呈现 server.yaml 配置的外部服务启用态；修改需编辑配置文件并重启。"
          />
        </Text>
        {error ? (
          <Alert
            type="error"
            showIcon
            title={
              <FormattedMessage
                id="pages.systemSiteSettings.maintenance.externalServices.loadFailed"
                defaultMessage="加载外部服务清单失败"
              />
            }
            description={error}
          />
        ) : null}
        {(snapshot?.services ?? []).map((svc) => (
          <div key={svc.name} data-service={svc.name}>
            <Space size="small" wrap>
              <Text strong>{serviceLabel(format, svc.name)}</Text>
              {stateTag(svc)}
              {svc.secretEnv ? (
                <Tag
                  color={svc.secretSet ? 'processing' : 'error'}
                  data-secret={svc.secretSet ? 'set' : 'unset'}
                >
                  {svc.secretEnv}:{' '}
                  {svc.secretSet ? (
                    <FormattedMessage
                      id="pages.systemSiteSettings.maintenance.externalServices.secretSet"
                      defaultMessage="已设置"
                    />
                  ) : (
                    <FormattedMessage
                      id="pages.systemSiteSettings.maintenance.externalServices.secretUnset"
                      defaultMessage="未设置"
                    />
                  )}
                </Tag>
              ) : null}
            </Space>
            {svc.missing && svc.missing.length > 0 ? (
              <div>
                <Text type="secondary">
                  <FormattedMessage
                    id="pages.systemSiteSettings.maintenance.externalServices.missing"
                    defaultMessage="缺项"
                  />
                  ：
                </Text>
                {svc.missing.map((m) => (
                  <Text key={m} code style={{ marginRight: 4 }}>
                    {m}
                  </Text>
                ))}
              </div>
            ) : null}
            {svc.detail ? (
              <div>
                <Text type="secondary" code>
                  {svc.detail}
                </Text>
              </div>
            ) : null}
          </div>
        ))}
      </Space>
    </Card>
  );
}
