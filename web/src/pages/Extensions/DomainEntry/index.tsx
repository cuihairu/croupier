import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { App, Button, Card, Empty, Space, Tag, Typography } from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import { FormattedMessage, history, useIntl, useLocation } from '@umijs/max';
import { listExtensionInstallations, listExtensionPages } from '@/services/api/extensions';

const { Text } = Typography;

// 域入口元信息：文案以 id + defaultMessage 双字段承载，渲染处经 intl 解析
type DomainMeta = {
  domain: string;
  extensionId: string;
  titleId: string;
  titleDefault: string;
  descriptionId: string;
  descriptionDefault: string;
};

function resolveDomainMeta(pathname: string): DomainMeta {
  if (pathname.includes('/approvals')) {
    return {
      domain: 'approvals',
      extensionId: 'official.approval',
      titleId: 'pages.extensionsDomainEntry.domain.approvals.title',
      titleDefault: '审批中心（扩展化）',
      descriptionId: 'pages.extensionsDomainEntry.domain.approvals.description',
      descriptionDefault: '该能力已切换到官方扩展 official.approval，请通过扩展安装实例管理。',
    };
  }
  if (pathname.includes('/alerts')) {
    return {
      domain: 'alerts',
      extensionId: 'official.alerting',
      titleId: 'pages.extensionsDomainEntry.domain.alerts.title',
      titleDefault: '告警中心（扩展化）',
      descriptionId: 'pages.extensionsDomainEntry.domain.alerts.description',
      descriptionDefault: '该能力已切换到官方扩展 official.alerting，请通过扩展安装实例管理。',
    };
  }
  if (pathname.includes('/backups')) {
    return {
      domain: 'backups',
      extensionId: 'official.backup-advanced',
      titleId: 'pages.extensionsDomainEntry.domain.backups.title',
      titleDefault: '备份管理（扩展化）',
      descriptionId: 'pages.extensionsDomainEntry.domain.backups.description',
      descriptionDefault:
        '该能力已切换到官方扩展 official.backup-advanced，请通过扩展安装实例管理。',
    };
  }
  return {
    domain: 'notifications',
    extensionId: 'official.notification',
    titleId: 'pages.extensionsDomainEntry.domain.notifications.title',
    titleDefault: '通知中心（扩展化）',
    descriptionId: 'pages.extensionsDomainEntry.domain.notifications.description',
    descriptionDefault: '该能力已切换到官方扩展 official.notification，请通过扩展安装实例管理。',
  };
}

export default function ExtensionDomainEntryPage() {
  const { message } = App.useApp();
  const intl = useIntl();
  const location = useLocation();
  const meta = useMemo(() => resolveDomainMeta(location.pathname), [location.pathname]);
  // load 是 useCallback + useEffect 链：intl 直接进依赖会在测试 mock（每次渲染新实例）下
  // 造成无限重请求，故经 ref 取值（先例 Ops/Jobs）
  const intlRef = useRef(intl);
  intlRef.current = intl;

  const [loading, setLoading] = useState(false);
  const [installedCount, setInstalledCount] = useState(0);
  const [pages, setPages] = useState<Array<{ title?: string; path?: string }>>([]);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [installationsResp, pagesResp] = await Promise.all([
        listExtensionInstallations({ extensionId: meta.extensionId, page: 1, pageSize: 20 }),
        listExtensionPages(meta.extensionId).catch(() => ({
          items: [] as Array<{ id?: string; title?: string; path?: string }>,
        })),
      ]);
      setInstalledCount(installationsResp?.total || 0);
      setPages((pagesResp?.items || []).map((x) => ({ title: x.title, path: x.path })));
    } catch {
      message.error(
        intlRef.current.formatMessage({
          id: 'pages.extensionsDomainEntry.loadError',
          defaultMessage: '加载扩展入口信息失败',
        }),
      );
      setInstalledCount(0);
      setPages([]);
    } finally {
      setLoading(false);
    }
  }, [meta.extensionId, message]);

  useEffect(() => {
    load();
  }, [load]);

  return (
    <PageContainer
      title={intl.formatMessage({ id: meta.titleId, defaultMessage: meta.titleDefault })}
      subTitle={intl.formatMessage({
        id: meta.descriptionId,
        defaultMessage: meta.descriptionDefault,
      })}
    >
      <Card loading={loading}>
        <Space orientation="vertical" size="middle" style={{ width: '100%' }}>
          <Space wrap>
            <Tag color={installedCount > 0 ? 'green' : 'default'}>
              {intl.formatMessage(
                {
                  id: 'pages.extensionsDomainEntry.tag.extension',
                  defaultMessage: `扩展: ${meta.extensionId}`,
                },
                { value: meta.extensionId },
              )}
            </Tag>
            <Tag color={installedCount > 0 ? 'blue' : 'default'}>
              {intl.formatMessage(
                {
                  id: 'pages.extensionsDomainEntry.tag.installations',
                  defaultMessage: `安装实例: ${installedCount}`,
                },
                { count: installedCount },
              )}
            </Tag>
          </Space>

          {pages.length > 0 ? (
            <Card
              size="small"
              title={intl.formatMessage({
                id: 'pages.extensionsDomainEntry.pages.title',
                defaultMessage: '扩展页面入口',
              })}
            >
              <Space orientation="vertical" style={{ width: '100%' }}>
                {pages.map((p, idx) => (
                  <Space key={`${p.path || p.title || 'page'}-${idx}`} wrap>
                    <Text strong>{p.title || `Page ${idx + 1}`}</Text>
                    <Text type="secondary">{p.path || '-'}</Text>
                  </Space>
                ))}
              </Space>
            </Card>
          ) : (
            <Empty
              image={Empty.PRESENTED_IMAGE_SIMPLE}
              description={
                <FormattedMessage
                  id="pages.extensionsDomainEntry.pages.empty"
                  defaultMessage="尚未发现扩展页面绑定，请先安装扩展或完成绑定。"
                />
              }
            />
          )}

          <Space wrap>
            <Button type="primary" onClick={() => history.push('/system/extensions/store')}>
              <FormattedMessage
                id="pages.extensionsDomainEntry.action.goStore"
                defaultMessage="前往扩展商店"
              />
            </Button>
            <Button onClick={() => history.push('/system/extensions/installations')}>
              <FormattedMessage
                id="pages.extensionsDomainEntry.action.goInstallations"
                defaultMessage="前往安装管理"
              />
            </Button>
            <Button onClick={load}>
              <FormattedMessage
                id="pages.extensionsDomainEntry.action.refresh"
                defaultMessage="刷新"
              />
            </Button>
          </Space>
        </Space>
      </Card>
    </PageContainer>
  );
}
