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
import { resolveConsolePageRoute, resolveLocalizedText } from '@/utils/consoleMenu';
import { getScope, subscribeScope } from '@/stores/scope';

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

  // scope 切换后重新加载发布页面：旧 scope 的数据与菜单必须立即失效。
  useEffect(() => {
    // 仅当 scope 实际变化（game/env 任一改变）时才整页刷新；
    // 等值 emit（登录流程会 setScope 两次）不得触发 reload 循环。
    let last = JSON.stringify(getScope());
    const unsubscribe = subscribeScope((next) => {
      const serialized = JSON.stringify(next);
      if (serialized === last) return;
      last = serialized;
      window.location.reload();
    });
    return unsubscribe;
  }, []);

  // 面包屑分类 key：以发布分类为准，缺失时回退路由参数
  const breadcrumbCategoryKey = page?.category?.key || categoryKey;
  // 页面标题：按当前语言解析发布 PageSpec 的 LocalizedText
  const pageTitle = resolveLocalizedText(page?.title, intl.locale, pageKey);
  // 面包屑分类：优先用发布分类的本地化 labels，缺失时回退原始 key
  const breadcrumbCategoryTitle = resolveLocalizedText(
    page?.category?.labels,
    intl.locale,
    breadcrumbCategoryKey,
  );

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
  const bindingFreshness = page?.bindingFreshness || [];

  return (
    <PageContainer
      title={pageTitle}
      subTitle={pageKey}
      breadcrumb={{
        items: [
          { title: intl.formatMessage({ id: 'menu.ControlConsole' }), href: '/console' },
          ...(breadcrumbCategoryKey
            ? [
                {
                  title: breadcrumbCategoryTitle,
                  href: `/console/${encodeURIComponent(breadcrumbCategoryKey)}`,
                },
              ]
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
                  {item.functionId ? (
                    <Typography.Text code>{item.functionId}</Typography.Text>
                  ) : null}
                  <Typography.Text>{item.diagnostic.message}</Typography.Text>
                </Space>
              ))}
              <Space size={8} style={{ marginTop: 8 }} wrap>
                <Button
                  type="primary"
                  size="small"
                  onClick={() =>
                    history.push(`/functions/pages?focus=${encodeURIComponent(pageKey)}`)
                  }
                >
                  前往处理（diff / 合并 / 重新发布）
                </Button>
                <Button
                  size="small"
                  onClick={() =>
                    history.push(`/functions/pages?focus=${encodeURIComponent(pageKey)}`)
                  }
                >
                  打开 Proposal Inbox
                </Button>
              </Space>
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
