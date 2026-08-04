/**
 * Console/Page - 运行控制台页面渲染器
 *
 * 使用 PageRenderer 渲染已发布 PageSpec 页面。
 * 路由：/console/:categoryKey/:pageKey
 */

import { useParams, history, useIntl } from '@umijs/max';
import { Alert, Button, Result, Space, Spin, Tag, Typography } from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import { useEffect, useState } from 'react';
import PageRenderer from '@/components/PageRenderer';
import {
  cancelTask,
  executePageBinding,
  getPublishedPage,
  queryApprovalStatus,
  queryTaskStatus,
} from '@/services/console';
import type { PublishedPageSpec } from '@/types/dashboard';
import { resolveConsolePageRoute } from '@/utils/consoleMenu';

export default function ConsolePage() {
  const params = useParams<{ categoryKey?: string; pageKey: string }>();
  const categoryKey = decodeURIComponent(params?.categoryKey || '');
  const pageKey = decodeURIComponent(params?.pageKey || '');
  const intl = useIntl();

  const [page, setPage] = useState<PublishedPageSpec | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string>('');
  const [errorCode, setErrorCode] = useState<string>('');
  const { canonicalPath, shouldRedirect } = resolveConsolePageRoute(page, categoryKey);

  useEffect(() => {
    if (!pageKey) return;

    let mounted = true;
    const loadPage = async () => {
      setLoading(true);
      setError('');
      setErrorCode('');

      try {
        const data = await getPublishedPage(pageKey);
        if (!mounted) return;
        setPage(data);
      } catch (err: unknown) {
        if (!mounted) return;
        const message = err instanceof Error ? err.message : String(err);
        setError(message);

        // 解析错误码
        if (message.includes('404') || message.includes('not found')) {
          setErrorCode('not_found');
        } else if (message.includes('403') || message.includes('forbidden')) {
          setErrorCode('forbidden');
        } else {
          setErrorCode('error');
        }
      } finally {
        if (mounted) setLoading(false);
      }
    };

    loadPage();
    return () => {
      mounted = false;
    };
  }, [pageKey]);

  useEffect(() => {
    if (!page) return;
    if (!shouldRedirect) return;
    history.replace(canonicalPath);
  }, [canonicalPath, page, shouldRedirect]);

  // 页面标题
  const pageTitle = page?.title
    ? (typeof page.title === 'string'
        ? page.title
        : page.title[intl.locale] || page.title['zh-CN'] || page.title['en-US'] || pageKey)
    : pageKey;

  // 404 状态
  if (!loading && errorCode === 'not_found') {
    return (
      <PageContainer title="页面不存在">
        <Result
          status="404"
          title="页面不存在"
          subTitle={`已发布页面 "${pageKey}" 未找到`}
          extra={
            <Button type="primary" onClick={() => history.push('/console')}>
              返回控制台
            </Button>
          }
        />
      </PageContainer>
    );
  }

  // 403 状态
  if (!loading && errorCode === 'forbidden') {
    return (
      <PageContainer title="无权限">
        <Result
          status="403"
          title="无访问权限"
          subTitle="您没有权限访问此页面"
          extra={
            <Button type="primary" onClick={() => history.push('/console')}>
              返回控制台
            </Button>
          }
        />
      </PageContainer>
    );
  }

  // 加载中状态
  if (loading) {
    return (
      <PageContainer title="加载中...">
        <div style={{ textAlign: 'center', padding: '100px 0' }}>
          <Spin size="large" tip="加载页面中..." />
        </div>
      </PageContainer>
    );
  }

  // 错误状态
  if (error) {
    return (
      <PageContainer title="加载失败">
        <Result
          status="error"
          title="加载失败"
          subTitle={error}
          extra={[
            <Button key="retry" type="primary" onClick={() => window.location.reload()}>
              重试
            </Button>,
            <Button key="back" onClick={() => history.push('/console')}>
              返回控制台
            </Button>,
          ]}
        />
      </PageContainer>
    );
  }

  if (shouldRedirect) {
    return (
      <PageContainer title="正在跳转...">
        <div style={{ textAlign: 'center', padding: '100px 0' }}>
          <Spin size="large" tip="正在跳转到页面发布分类..." />
        </div>
      </PageContainer>
    );
  }

  // 渲染页面
  const breadcrumbCategoryKey = page?.category?.key || categoryKey;
  const bindingFreshness = page?.bindingFreshness || [];

  return (
    <PageContainer
      title={pageTitle}
      subTitle={pageKey}
      breadcrumb={{
        items: [
          { title: '运行控制台', href: '/console' },
          ...(breadcrumbCategoryKey
            ? [{ title: breadcrumbCategoryKey, href: `/console/${encodeURIComponent(breadcrumbCategoryKey)}` }]
            : []),
          { title: pageTitle },
        ],
      }}
    >
      {bindingFreshness.length > 0 ? (
        <Alert
          type="error"
          showIcon
          style={{ marginBottom: 16 }}
          message="页面绑定的函数契约已变化，执行已被阻断"
          description={
            <Space direction="vertical" size={4}>
              {bindingFreshness.map((item) => (
                <Space key={`${item.bindingId}:${item.status}:${item.diagnostic.code}`} wrap>
                  <Tag color="red">{item.status}</Tag>
                  <Typography.Text code>{item.bindingId}</Typography.Text>
                  {item.functionId ? <Typography.Text code>{item.functionId}</Typography.Text> : null}
                  <Typography.Text>{item.diagnostic.message}</Typography.Text>
                </Space>
              ))}
            </Space>
          }
        />
      ) : null}
      {page && (
        <PageRenderer
          pageSpec={page}
          onExecute={async (bindingId, context) => {
            return executePageBinding(page.pageKey, bindingId, context);
          }}
          onQueryStatus={queryTaskStatus}
          onCancelTask={cancelTask}
          onQueryApprovalStatus={queryApprovalStatus}
        />
      )}
    </PageContainer>
  );
}
