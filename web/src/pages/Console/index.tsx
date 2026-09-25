/**
 * Console/index - 运行控制台首页
 *
 * 从 ConsoleMenuSpec 动态读取已发布页面。
 * 路由：/console/home 或 /console/:categoryKey
 */

import { FormattedMessage, history, useAccess, useIntl, useParams } from '@umijs/max';
import { Alert, Button, Card, Empty, Space, Spin, Tag, Typography } from 'antd';
import { useEffect, useMemo, useRef, useState } from 'react';
import { AppstoreOutlined, CheckCircleOutlined, FolderOutlined } from '@ant-design/icons';
import { getConsoleMenu, listPublishedPages } from '@/services/console';
import type { ConsoleMenuItem, ConsoleMenuSpec, PublishedPageSpec } from '@/types/dashboard';
import { localizedText } from '@/utils/localizedText';

type ConsoleAccess = {
  canConsoleRead?: boolean;
};

// findMenuItem 在菜单树中递归查找指定 key 的节点（菜单可任意层级嵌套）。
const findMenuItem = (items: ConsoleMenuItem[], key: string): ConsoleMenuItem | undefined => {
  for (const item of items) {
    if (item.key === key) return item;
    if (item.children?.length) {
      const found = findMenuItem(item.children, key);
      if (found) return found;
    }
  }
  return undefined;
};

export default function ConsoleIndex() {
  const access = useAccess() as ConsoleAccess;
  const intl = useIntl();
  // useIntl 的 mock 每渲染返回新实例；effect 内取文案走 ref，避免 intl 进依赖引发重复加载
  const intlRef = useRef(intl);
  intlRef.current = intl;
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
        const [menuData, pagesData] = await Promise.all([getConsoleMenu(), listPublishedPages()]);

        if (!mounted) return;
        setMenu(menuData);
        setPages(Array.isArray(pagesData) ? pagesData : []);
      } catch (err: unknown) {
        if (!mounted) return;
        setError(
          err instanceof Error
            ? err.message
            : intlRef.current.formatMessage({
                id: 'pages.console.home.error.loadConsoleFailed',
                defaultMessage: '加载控制台失败',
              }),
        );
      } finally {
        if (mounted) setLoading(false);
      }
    };

    loadData();
    return () => {
      mounted = false;
    };
  }, []);

  // 获取页面分类标题
  const getCategoryTitle = (item: ConsoleMenuItem): string => {
    if (!item.title) return item.key;
    if (typeof item.title === 'string') return item.title;
    return item.title[intl.locale] || localizedText(item.title, intl.locale, item.key);
  };

  // 菜单树递归查找：子菜单（任意层级）的 categoryKey 路由也能命中
  const visibleCategories = useMemo(() => {
    const items = menu?.items || [];
    if (!categoryKey) return items;
    const found = findMenuItem(items, categoryKey);
    return found ? [found] : [];
  }, [categoryKey, menu?.items]);

  const activeCategory = categoryKey ? visibleCategories[0] : undefined;
  const pagesByKey = useMemo(() => {
    const next = new Map<string, PublishedPageSpec>();
    pages.forEach((page) => next.set(page.pageKey, page));
    return next;
  }, [pages]);

  const renderPageCard = (item: ConsoleMenuItem) => {
    const page = pagesByKey.get(item.key);
    const staleCount = page?.bindingFreshness?.length || 0;
    // 子菜单组：非页面节点（pagesByKey 未命中且带 children）渲染为分组卡，
    // 点击进入该子菜单的 categoryKey 页
    const isMenuGroup = !page && (item.children?.length || 0) > 0;
    return (
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
                background: isMenuGroup
                  ? 'linear-gradient(135deg, #722ed1 0%, #b37feb 100%)'
                  : 'linear-gradient(135deg, #1677ff 0%, #69b1ff 100%)',
                color: '#fff',
              }}
            >
              {isMenuGroup ? (
                <FolderOutlined style={{ fontSize: 18 }} />
              ) : (
                <AppstoreOutlined style={{ fontSize: 18 }} />
              )}
            </div>
          }
          title={
            <Space wrap size={[8, 8]}>
              <Typography.Text strong>
                {localizedText(item.title, intl.locale, item.key)}
              </Typography.Text>
              {isMenuGroup ? (
                <Tag color="purple">
                  <FormattedMessage
                    id="pages.console.home.tag.menuGroup"
                    defaultMessage="菜单组 · {count} 项"
                    values={{ count: item.children?.length || 0 }}
                  />
                </Tag>
              ) : staleCount > 0 ? (
                <Tag color="error">
                  <FormattedMessage
                    id="pages.console.home.tag.stale"
                    defaultMessage="契约失效 {count}"
                    values={{ count: staleCount }}
                  />
                </Tag>
              ) : (
                <Tag color="success" icon={<CheckCircleOutlined />}>
                  <FormattedMessage id="pages.console.home.tag.published" defaultMessage="已发布" />
                </Tag>
              )}
            </Space>
          }
          description={
            <Typography.Text code style={{ fontSize: 12 }}>
              {item.key}
            </Typography.Text>
          }
        />
      </Card>
    );
  };

  // 权限检查
  if (!access?.canConsoleRead) {
    return (
      <Card>
        <Typography.Title level={4}>
          {intl.formatMessage({
            id: 'pages.console.home.permission.title',
            defaultMessage: '权限受限',
          })}
        </Typography.Title>
        <Typography.Text>
          {intl.formatMessage({
            id: 'pages.console.home.permission.deniedText',
            defaultMessage: '你没有查看运行控制台的权限。',
          })}
        </Typography.Text>
      </Card>
    );
  }

  // 加载中
  if (loading) {
    return (
      <div style={{ textAlign: 'center', padding: '100px 0' }}>
        <Spin
          size="large"
          description={intl.formatMessage({
            id: 'pages.console.home.loading',
            defaultMessage: '加载控制台...',
          })}
        />
      </div>
    );
  }

  // 错误状态
  if (error) {
    return (
      <Card>
        <Alert
          type="error"
          title={intl.formatMessage({
            id: 'pages.console.home.error.load',
            defaultMessage: '加载失败',
          })}
          description={error}
          showIcon
        />
      </Card>
    );
  }

  // 页面标题
  const pageTitle = categoryKey
    ? intl.formatMessage(
        {
          id: 'pages.console.home.title.category',
          defaultMessage: '运行控制台 / {category}',
        },
        { category: activeCategory ? getCategoryTitle(activeCategory) : categoryKey },
      )
    : intl.formatMessage({ id: 'pages.console.home.title.root', defaultMessage: '运行控制台' });

  return (
    <Space orientation="vertical" size={16} style={{ width: '100%' }}>
      {/* 概览信息 */}
      <Card>
        <Space orientation="vertical" size={8}>
          <Typography.Title level={4} style={{ margin: 0 }}>
            {pageTitle}
          </Typography.Title>
          <Typography.Text type="secondary">
            {intl.formatMessage({
              id: 'pages.console.home.description',
              defaultMessage:
                '运行控制台的导航由菜单管理（menu_items）驱动：菜单节点分组展示，挂载到菜单且已发布的页面才会出现在这里。',
            })}
          </Typography.Text>
          <Space wrap size={[8, 8]}>
            <Tag color="blue">
              <FormattedMessage
                id="pages.console.home.tag.pages"
                defaultMessage="已发布 {count} 个页面"
                values={{ count: pages.length }}
              />
            </Tag>
            {menu?.items && (
              <Tag color="green">
                <FormattedMessage
                  id="pages.console.home.tag.categories"
                  defaultMessage="{count} 个菜单"
                  values={{ count: menu.items.length }}
                />
              </Tag>
            )}
          </Space>
        </Space>
      </Card>

      {/* 分类列表 */}
      {menu?.items && menu.items.length > 0 ? (
        <Space orientation="vertical" size={16} style={{ width: '100%' }}>
          {visibleCategories.map((category) => (
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
                  {category.children.map(renderPageCard)}
                </Space>
              ) : (
                <Typography.Text type="secondary">
                  <FormattedMessage
                    id="pages.console.home.empty.category"
                    defaultMessage="该菜单下暂无页面"
                  />
                </Typography.Text>
              )}
            </Card>
          ))}
          {categoryKey && visibleCategories.length === 0 ? (
            <Card>
              <Empty
                description={intl.formatMessage(
                  {
                    id: 'pages.console.home.empty.categoryKey',
                    defaultMessage: '菜单 "{categoryKey}" 下暂无已发布页面',
                  },
                  { categoryKey },
                )}
              />
            </Card>
          ) : null}
        </Space>
      ) : (
        <Card>
          <Empty
            description={intl.formatMessage({
              id: 'pages.console.home.empty.pages',
              defaultMessage: '暂无菜单',
            })}
          >
            <Space orientation="vertical" size={8}>
              <Typography.Text type="secondary">
                <FormattedMessage
                  id="pages.console.home.empty.hint"
                  defaultMessage="请先在菜单管理中创建菜单并挂载已发布页面（页面工作台发布），然后在这里查看。"
                />
              </Typography.Text>
              <Button type="primary" onClick={() => history.push('/functions/menus')}>
                <FormattedMessage
                  id="pages.console.home.empty.goMenus"
                  defaultMessage="去菜单管理"
                />
              </Button>
            </Space>
          </Empty>
        </Card>
      )}
    </Space>
  );
}
