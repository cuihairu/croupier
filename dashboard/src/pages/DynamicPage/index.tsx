/**
 * 动态页面容器
 *
 * 根据路由路径加载对应的页面配置，并使用 PageGenerator 渲染
 */

import React, { useEffect, useState } from 'react';
import { PageContainer } from '@ant-design/pro-components';
import { useLocation } from '@umijs/max';
import { Button, Card, Space, Spin, Tag, Typography } from 'antd';
import { DASHBOARD_PAGE_TOKENS, PageStatePanel } from '@/components';
import PageGenerator from '@/components/PageGenerator';
import type { PageConfig } from '@/components/PageGenerator/types';
import { getPageConfig } from '@/services/pageConfig';

const DynamicPage: React.FC = () => {
  const location = useLocation();
  const [config, setConfig] = useState<PageConfig | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    loadConfig();
  }, [location.pathname]);

  const loadConfig = async () => {
    setLoading(true);
    setError(null);

    try {
      // 根据路径加载配置
      const pageConfig = await getPageConfig(location.pathname);

      if (!pageConfig) {
        setError('页面配置不存在');
        return;
      }

      setConfig(pageConfig);
    } catch (err: any) {
      setError(err.message || '加载页面配置失败');
      console.error('Failed to load page config:', err);
    } finally {
      setLoading(false);
    }
  };

  if (loading) {
    return (
      <PageContainer title="动态运行页" subTitle="按路由加载页面配置并进入正式运行容器">
        <Card
          styles={{
            body: {
              padding: DASHBOARD_PAGE_TOKENS.heroCardPadding,
              background:
                'linear-gradient(135deg, rgba(22,119,255,0.08) 0%, rgba(82,196,26,0.03) 100%)',
            },
          }}
        >
          <Space direction="vertical" size={16} style={{ width: '100%' }}>
            <Space wrap size={[8, 8]}>
              <Tag color="processing">运行态加载中</Tag>
              <Typography.Text code>{location.pathname}</Typography.Text>
            </Space>
            <Typography.Title level={4} style={{ margin: 0 }}>
              正在解析动态页面配置
            </Typography.Title>
            <Typography.Text type="secondary">
              系统会按当前路由加载对应配置，并以运行态容器渲染页面。这里不再展示编辑器式示例骨架。
            </Typography.Text>
            <div style={{ padding: '16px 0' }}>
              <Spin size="large" tip="加载页面配置中..." />
            </div>
          </Space>
        </Card>
      </PageContainer>
    );
  }

  if (error || !config) {
    return (
      <PageContainer title="动态运行页" subTitle="按路由加载页面配置并进入正式运行容器">
        <PageStatePanel
          tone="warning"
          badgeText="未找到页面配置"
          title="当前页面无法渲染"
          description={error || '未找到页面配置'}
          extra={location.pathname}
          actions={
            <>
              <Button type="primary" onClick={() => window.history.back()}>
                返回上一页
              </Button>
              <Button onClick={loadConfig}>重新加载</Button>
            </>
          }
        />
      </PageContainer>
    );
  }

  return (
    <PageContainer
      title={config.title || '动态运行页'}
      subTitle="当前页面已按动态配置进入正式运行态"
    >
      <Space direction="vertical" size={16} style={{ width: '100%' }}>
        <Card
          size="small"
          styles={{
            body: {
              padding: DASHBOARD_PAGE_TOKENS.cardPadding,
              background:
                'linear-gradient(135deg, rgba(82,196,26,0.08) 0%, rgba(22,119,255,0.03) 100%)',
            },
          }}
        >
          <Space direction="vertical" size={10} style={{ width: '100%' }}>
            <Space wrap size={[8, 8]}>
              <Tag color="success">动态运行态</Tag>
              <Typography.Text code>{location.pathname}</Typography.Text>
            </Space>
            <Typography.Text type="secondary">
              当前页面已加载路由配置并进入正式容器。重点核对入口可见性、页面层级和真实数据表达是否达到交付标准。
            </Typography.Text>
          </Space>
        </Card>
        <PageGenerator config={config} />
      </Space>
    </PageContainer>
  );
};

export default DynamicPage;
