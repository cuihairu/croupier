import React, { useEffect, useMemo, useState } from 'react';
import { App, Button, Card, Empty, Space, Tag, Typography } from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import { history, useLocation } from '@umijs/max';
import { listExtensionInstallations, listExtensionPages } from '@/services/api/extensions';

const { Text } = Typography;

type DomainMeta = {
  domain: string;
  extensionId: string;
  title: string;
  description: string;
};

function resolveDomainMeta(pathname: string): DomainMeta {
  if (pathname.includes('/approvals')) {
    return {
      domain: 'approvals',
      extensionId: 'official.approval',
      title: '审批中心（扩展化）',
      description: '该能力已切换到官方扩展 official.approval，请通过扩展安装实例管理。',
    };
  }
  if (pathname.includes('/alerts')) {
    return {
      domain: 'alerts',
      extensionId: 'official.alerting',
      title: '告警中心（扩展化）',
      description: '该能力已切换到官方扩展 official.alerting，请通过扩展安装实例管理。',
    };
  }
  if (pathname.includes('/backups')) {
    return {
      domain: 'backups',
      extensionId: 'official.backup-advanced',
      title: '备份管理（扩展化）',
      description: '该能力已切换到官方扩展 official.backup-advanced，请通过扩展安装实例管理。',
    };
  }
  return {
    domain: 'notifications',
    extensionId: 'official.notification',
    title: '通知中心（扩展化）',
    description: '该能力已切换到官方扩展 official.notification，请通过扩展安装实例管理。',
  };
}

export default function ExtensionDomainEntryPage() {
  const { message } = App.useApp();
  const location = useLocation();
  const meta = useMemo(() => resolveDomainMeta(location.pathname), [location.pathname]);

  const [loading, setLoading] = useState(false);
  const [installedCount, setInstalledCount] = useState(0);
  const [pages, setPages] = useState<Array<{ title?: string; path?: string }>>([]);

  const load = async () => {
    setLoading(true);
    try {
      const [installationsResp, pagesResp] = await Promise.all([
        listExtensionInstallations({ extensionId: meta.extensionId, page: 1, pageSize: 20 }),
        listExtensionPages(meta.extensionId).catch(() => ({ items: [] as any[] })),
      ]);
      setInstalledCount(installationsResp?.total || 0);
      setPages((pagesResp?.items || []).map((x) => ({ title: x.title, path: x.path })));
    } catch {
      message.error('加载扩展入口信息失败');
      setInstalledCount(0);
      setPages([]);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    load();
  }, [meta.extensionId]);

  return (
    <PageContainer title={meta.title} subTitle={meta.description}>
      <Card loading={loading}>
        <Space direction="vertical" size="middle" style={{ width: '100%' }}>
          <Space wrap>
            <Tag color={installedCount > 0 ? 'green' : 'default'}>扩展: {meta.extensionId}</Tag>
            <Tag color={installedCount > 0 ? 'blue' : 'default'}>安装实例: {installedCount}</Tag>
          </Space>

          {pages.length > 0 ? (
            <Card size="small" title="扩展页面入口">
              <Space direction="vertical" style={{ width: '100%' }}>
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
              description="尚未发现扩展页面绑定，请先安装扩展或完成绑定。"
            />
          )}

          <Space wrap>
            <Button type="primary" onClick={() => history.push('/system/extensions/store')}>
              前往扩展商店
            </Button>
            <Button onClick={() => history.push('/system/extensions/installations')}>
              前往安装管理
            </Button>
            <Button onClick={load}>刷新</Button>
          </Space>
        </Space>
      </Card>
    </PageContainer>
  );
}
