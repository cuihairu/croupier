/**
 * Console/index - 运行控制台首页
 *
 * 从 ConsoleMenuSpec 动态读取已发布页面，不读取旧 WorkspaceConfig。
 * 路由：/console/home 或 /console/:categoryKey
 */

import { history, useAccess, useIntl, useParams } from '@umijs/max';
import { Alert, Card, Empty, Space, Spin, Tag, Typography } from 'antd';
import { useEffect, useMemo, useState } from 'react';
import { AppstoreOutlined, CheckCircleOutlined } from '@ant-design/icons';
import { getConsoleMenu, listPublishedPages } from '@/services/console';
import type { ConsoleMenuSpec, PublishedPageSpec } from '@/types/dashboard';

type ConsoleAccess = {
  canConsoleRead?: boolean;
};

export default function ConsoleIndex() {
  const access = useAccess() as ConsoleAccess;
  const intl = useIntl();
  const params = useParams<{ categoryKey?: string }>();
  const categoryKey = decodeURIComponent(params?.categoryKey || '');

  const [loading, setLoading] = useState(true);
  const [menu, setMenu] = useState<ConsoleMenuSpec | null>(null);
  const [pages, setPages] = useState<PublishedPageSpec[]>([]);
  const [error, setError] = useState('');

  // 加载菜单和页面数据
  useEffect(() => {
    let mounted = true;

    const loadData = async () => {
      setLoading(true);
      setError('');

      try {
        const [menuData, pagesData] = await Promise.all([
          getConsoleMenu(),
          listPublishedPages(),
        ]);

        if (!mounted) return;
        setMenu(menuData);
        setPages(Array.isArray(pagesData) ? pagesData : []);
      } catch (err: unknown) {
        if (!mounted) return;
        setError(err instanceof Error ? err.message : '加载控制台失败');
      } finally {
        if (mounted) setLoading(false);
      }
    };

    loadData();
    return () => {
      mounted = false;
    };
  }, []);

  // 按分类过滤页面
  const filteredPages = useMemo(() => {
    if (!categoryKey) return pages;
    return pages.filter((page) => {
      const pageCategory = page.category?.key || '';
      return pageCategory === categoryKey;
    });
  }, [categoryKey, pages]);

  // 获取页面标题（多语言）
  const getPageTitle = (page: PublishedPageSpec): string => {
    if (!page.title) return page.pageKey;
    if (typeof page.title === 'string') return page.title;
    return page.title[intl.locale] || page.title['zh-CN'] || page.title['en-US'] || page.pageKey;
  };

  // 获取页面分类标题
  const getCategoryTitle = (item: ConsoleMenuSpec['items'][0]): string => {
    if (!item.title) return item.key;
    if (typeof item.title === 'string') return item.title;
    return item.title[intl.locale] || item.title['zh-CN'] || item.title['en-US'] || item.key;
  };

  // 权限检查
  if (!access?.canConsoleRead) {
    return (
      <Card>
        <Typography.Title level={4}>权限受限</Typography.Title>
        <Typography.Text>你没有查看运行控制台的权限。</Typography.Text>
      </Card>
    );
  }

  // 加载中
  if (loading) {
    return (
      <div style={{ textAlign: 'center', padding: '100px 0' }}>
        <Spin size="large" tip="加载控制台..." />
      </div>
    );
  }

  // 错误状态
  if (error) {
    return (
      <Card>
        <Alert type="error" message="加载失败" description={error} showIcon />
      </Card>
    );
  }

  // 页面标题
  const pageTitle = categoryKey
    ? `运行控制台 / ${categoryKey}`
    : '运行控制台';

  return (
    <Space direction="vertical" size={16} style={{ width: '100%' }}>
      {/* 概览信息 */}
      <Card>
        <Space direction="vertical" size={8}>
          <Typography.Title level={4} style={{ margin: 0 }}>
            {pageTitle}
          </Typography.Title>
          <Typography.Text type="secondary">
            运行控制台展示已发布的页面。页面由 PageSpec 定义，通过 Formily 渲染。
          </Typography.Text>
          <Space wrap size={[8, 8]}>
            <Tag color="blue">{`已发布 ${pages.length} 个页面`}</Tag>
            {menu?.items && (
              <Tag color="green">{`${menu.items.length} 个分类`}</Tag>
            )}
          </Space>
        </Space>
      </Card>

      {/* 分类列表 */}
      {menu?.items && menu.items.length > 0 ? (
        <Space direction="vertical" size={16} style={{ width: '100%' }}>
          {menu.items.map((category) => (
            <Card
              key={category.key}
              title={
                <Space wrap size={[8, 8]}>
                  <AppstoreOutlined />
                  <span>{getCategoryTitle(category)}</span>
                  <Typography.Text code>{category.key}</Typography.Text>
                </Space>
              }
            >
              {category.children && category.children.length > 0 ? (
                <Space wrap size={[12, 12]}>
                  {category.children.map((item) => (
                    <Card
                      key={item.key}
                      hoverable
                      size="small"
                      style={{ width: 280 }}
                      onClick={() => history.push(item.path)}
                    >
                      <Card.Meta
                        avatar={
                          <div
                            style={{
                              width: 40,
                              height: 40,
                              borderRadius: 12,
                              display: 'grid',
                              placeItems: 'center',
                              background: 'linear-gradient(135deg, #1677ff 0%, #69b1ff 100%)',
                              color: '#fff',
                            }}
                          >
                            <AppstoreOutlined style={{ fontSize: 18 }} />
                          </div>
                        }
                        title={
                          <Space wrap size={[8, 8]}>
                            <Typography.Text strong>
                              {item.title
                                ? (typeof item.title === 'string'
                                    ? item.title
                                    : item.title[intl.locale] || item.title['zh-CN'] || item.key)
                                : item.key}
                            </Typography.Text>
                            <Tag color="success" icon={<CheckCircleOutlined />}>
                              已发布
                            </Tag>
                          </Space>
                        }
                        description={
                          <Typography.Text code style={{ fontSize: 12 }}>
                            {item.key}
                          </Typography.Text>
                        }
                      />
                    </Card>
                  ))}
                </Space>
              ) : (
                <Typography.Text type="secondary">该分类下暂无页面</Typography.Text>
              )}
            </Card>
          ))}
        </Space>
      ) : (
        <Card>
          <Empty description="暂无已发布页面">
            <Typography.Text type="secondary">
              请先在 Page 工作台发布页面，然后在这里查看。
            </Typography.Text>
          </Empty>
        </Card>
      )}

      {/* 所有页面列表（当有分类过滤时显示） */}
      {categoryKey && filteredPages.length > 0 && (
        <Card title={`分类 "${categoryKey}" 下的页面`}>
          <Space wrap size={[12, 12]}>
            {filteredPages.map((page) => (
              <Card
                key={page.pageKey}
                hoverable
                size="small"
                style={{ width: 280 }}
                onClick={() =>
                  history.push(
                    `/console/${encodeURIComponent(categoryKey)}/${encodeURIComponent(page.pageKey)}`,
                  )
                }
              >
                <Card.Meta
                  title={getPageTitle(page)}
                  description={
                    <Space direction="vertical" size={4}>
                      <Typography.Text code style={{ fontSize: 12 }}>
                        {page.pageKey}
                      </Typography.Text>
                      {page.version && (
                        <Tag>{`v${page.version}`}</Tag>
                      )}
                    </Space>
                  }
                />
              </Card>
            ))}
          </Space>
        </Card>
      )}
    </Space>
  );
}
