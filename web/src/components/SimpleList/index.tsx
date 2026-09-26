import { Empty, Pagination, Spin } from 'antd';
import { createStyles } from 'antd-style';
import type { ReactNode } from 'react';
import React from 'react';

// antd 6 整体废弃了 List 组件（v7 移除，运行时告警见 docs/BUGS.md BUG-014）。
// 本仓库对列表的全部用法都是「只读条目 + Meta + 少量操作」，用这个内部替身承载，
// 视觉契约对齐 antd List 默认形态（分隔线 / 标题 / 次要描述 / 底部分页）。
// 迁移完成后由 web/tests/antd6Deprecations.test.ts 的组件级守卫禁止再引入 antd List。

const useStyles = createStyles(({ token }) => ({
  list: {
    background: token.colorBgContainer,
  },
  item: {
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'center',
    gap: token.sizeUnit * 2,
    padding: `${token.paddingContentVerticalSM}px 0`,
    borderTop: `1px solid ${token.colorSplit}`,
    '&:first-child': {
      borderTop: 'none',
    },
  },
  content: {
    flex: 1,
    minWidth: 0,
  },
  actions: {
    display: 'flex',
    alignItems: 'center',
    gap: token.sizeUnit,
    flexShrink: 0,
  },
  metaTitle: {
    marginBottom: token.marginXXS,
    color: token.colorText,
    fontSize: token.fontSize,
    lineHeight: token.lineHeight,
  },
  metaDescription: {
    color: token.colorTextDescription,
    fontSize: token.fontSize,
    lineHeight: token.lineHeight,
  },
  pagination: {
    marginTop: token.marginLG,
    textAlign: 'right',
  },
  empty: {
    padding: `${token.paddingContentVerticalLG}px 0`,
    textAlign: 'center',
  },
}));

export interface SimpleListItemProps extends Omit<React.LiHTMLAttributes<HTMLLIElement>, 'style'> {
  children?: ReactNode;
  /** 条目右侧操作区（按钮/链接），对齐 antd List.Item 的 actions。 */
  actions?: ReactNode[];
  style?: React.CSSProperties;
  onClick?: React.MouseEventHandler<HTMLLIElement>;
}

function ItemBase({ children, actions, style, onClick, ...rest }: SimpleListItemProps) {
  const { styles } = useStyles();
  return (
    // 其余 DOM 属性（data-testid / data-* / aria-*）透传到根节点：
    // 调用方需要按条目 id 定位并断言状态（如通知列表的未读标记），
    // antd 的 List.Item 本身也是透传的，此前的实现把它们静默丢弃了。
    <li className={styles.item} style={style} onClick={onClick} {...rest}>
      <div className={styles.content}>{children}</div>
      {actions && actions.length > 0 && (
        <div className={styles.actions}>
          {actions.map((action, i) => (
            // 操作元素由调用方提供 key；此处以位置兜底
            <React.Fragment key={i}>{action}</React.Fragment>
          ))}
        </div>
      )}
    </li>
  );
}

export interface SimpleListItemMetaProps {
  title?: ReactNode;
  description?: ReactNode;
  /** 预留对齐 antd List.Item.Meta 的视觉位；当前调用方均未使用头像。 */
  avatar?: ReactNode;
}

function MetaBase({ title, description, avatar }: SimpleListItemMetaProps) {
  const { styles } = useStyles();
  return (
    <div style={{ display: 'flex', alignItems: 'flex-start', gap: 16 }}>
      {avatar}
      <div style={{ flex: 1, minWidth: 0 }}>
        {title != null && <div className={styles.metaTitle}>{title}</div>}
        {description != null && <div className={styles.metaDescription}>{description}</div>}
      </div>
    </div>
  );
}

export interface SimpleListPagination {
  current: number;
  pageSize: number;
  total: number;
  showSizeChanger?: boolean;
  onChange: (page: number, pageSize: number) => void;
}

export interface SimpleListProps<T> {
  dataSource?: T[];
  renderItem: (item: T, index: number) => ReactNode;
  rowKey?: (item: T, index: number) => React.Key;
  locale?: { emptyText?: ReactNode };
  loading?: boolean;
  pagination?: SimpleListPagination;
  style?: React.CSSProperties;
}

function SimpleListBase<T>({
  dataSource,
  renderItem,
  rowKey,
  locale,
  loading = false,
  pagination,
  style,
}: SimpleListProps<T>) {
  const { styles } = useStyles();
  const items = dataSource ?? [];
  const keyOf = (item: T, index: number): React.Key =>
    rowKey ? rowKey(item, index) : index;

  let body: ReactNode;
  if (items.length === 0) {
    body = (
      <div className={styles.empty}>
        <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={locale?.emptyText} />
      </div>
    );
  } else {
    body = (
      <ul style={{ margin: 0, padding: 0, listStyle: 'none' }}>
        {items.map((item, index) => (
          <React.Fragment key={keyOf(item, index)}>
            {renderItem(item, index)}
          </React.Fragment>
        ))}
      </ul>
    );
  }

  return (
    <div className={styles.list} style={style}>
      {loading ? <Spin spinning>{body}</Spin> : body}
      {pagination && items.length > 0 && (
        <div className={styles.pagination}>
          <Pagination
            current={pagination.current}
            pageSize={pagination.pageSize}
            total={pagination.total}
            showSizeChanger={pagination.showSizeChanger}
            onChange={pagination.onChange}
            size="small"
          />
        </div>
      )}
    </div>
  );
}

// 泛型函数组件不能直接挂命名空间属性，用类型断言补上 .Item / .Item.Meta，
// 调用侧写法与原 antd List 一致（<SimpleList<T> renderItem={...}> +
// <SimpleList.Item><SimpleList.Item.Meta /></SimpleList.Item>）。
type ItemWithMeta = typeof ItemBase & { Meta: typeof MetaBase };

type SimpleListComponent = (<T>(props: SimpleListProps<T>) => ReactNode) & {
  Item: ItemWithMeta;
};

const ItemWithMetaHost = ItemBase as ItemWithMeta;
ItemWithMetaHost.Meta = MetaBase;

export const SimpleList = SimpleListBase as unknown as SimpleListComponent;

SimpleList.Item = ItemWithMetaHost;

export default SimpleList;
