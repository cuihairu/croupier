/**
 * SimpleList（antd 6 废弃 List 的内部替身，docs/BUGS.md BUG-014）组件行为测试。
 *
 * 覆盖六个迁移调用方用到的全部 API 面：
 * dataSource/renderItem/rowKey/locale.emptyText/loading/pagination、
 * Item(actions/style/onClick)、Item.Meta(title/description)。
 */
import { render, screen, fireEvent } from '@testing-library/react';
import SimpleList from '@/components/SimpleList';

describe('SimpleList', () => {
  it('渲染 dataSource 条目并展示 Meta 标题与描述', () => {
    render(
      <SimpleList
        dataSource={[
          { id: 1, name: '条目一', desc: '描述一' },
          { id: 2, name: '条目二', desc: '描述二' },
        ]}
        renderItem={(item) => (
          <SimpleList.Item>
            <SimpleList.Item.Meta title={item.name} description={item.desc} />
          </SimpleList.Item>
        )}
      />,
    );
    expect(screen.getByText('条目一')).toBeInTheDocument();
    expect(screen.getByText('描述一')).toBeInTheDocument();
    expect(screen.getByText('条目二')).toBeInTheDocument();
    expect(screen.getByText('描述二')).toBeInTheDocument();
  });

  it('空数据时渲染 locale.emptyText', () => {
    render(
      <SimpleList
        dataSource={[]}
        locale={{ emptyText: '暂无游戏' }}
        renderItem={() => <SimpleList.Item>x</SimpleList.Item>}
      />,
    );
    expect(screen.getByText('暂无游戏')).toBeInTheDocument();
  });

  it('Item.actions 渲染在条目上且可点击', () => {
    const onClick = jest.fn();
    render(
      <SimpleList
        dataSource={[{ id: 'a' }]}
        renderItem={(item) => (
          <SimpleList.Item
            actions={[
              <button key="act" onClick={onClick} type="button">
                申请
              </button>,
            ]}
          >
            <span>{item.id}</span>
          </SimpleList.Item>
        )}
      />,
    );
    fireEvent.click(screen.getByRole('button', { name: '申请' }));
    expect(onClick).toHaveBeenCalledTimes(1);
  });

  it('Item.onClick 透传（通知中心点击整行打开消息）', () => {
    const onRowClick = jest.fn();
    render(
      <SimpleList
        dataSource={[{ id: 'row' }]}
        renderItem={(item) => (
          <SimpleList.Item onClick={onRowClick}>
            <span>{item.id}</span>
          </SimpleList.Item>
        )}
      />,
    );
    fireEvent.click(screen.getByText('row'));
    expect(onRowClick).toHaveBeenCalledTimes(1);
  });

  it('rowKey 参与 React key 计算，不回退到索引', () => {
    const rowKey = jest.fn((item: { id: string }) => item.id);
    const { container } = render(
      <SimpleList
        dataSource={[{ id: 'k1' }, { id: 'k2' }]}
        rowKey={rowKey}
        renderItem={(item) => (
          <SimpleList.Item>
            <span>{item.id}</span>
          </SimpleList.Item>
        )}
      />,
    );
    expect(rowKey).toHaveBeenCalledTimes(2);
    const lis = container.querySelectorAll('ul > li');
    // Fragment 的 key 不直接落到 DOM，但至少保证条目数与顺序正确；
    // key 唯一性由 React 在 dev 下告警兜底，此处断言 rowKey 确实被逐条调用。
    expect(lis).toHaveLength(2);
    expect(lis[0]).toHaveTextContent('k1');
    expect(lis[1]).toHaveTextContent('k2');
  });

  it('loading 时条目包在 Spin 里', () => {
    const { container } = render(
      <SimpleList
        loading
        dataSource={[{ id: 'a' }]}
        renderItem={(item) => (
          <SimpleList.Item>
            <span>{item.id}</span>
          </SimpleList.Item>
        )}
      />,
    );
    expect(container.querySelector('.ant-spin-spinning')).toBeInTheDocument();
    expect(screen.getByText('a')).toBeInTheDocument();
  });

  it('pagination 透传给 antd Pagination，翻页回调带页码', () => {
    const onChange = jest.fn();
    render(
      <SimpleList
        dataSource={[{ id: 'a' }]}
        pagination={{ current: 1, pageSize: 10, total: 25, onChange }}
        renderItem={(item) => (
          <SimpleList.Item>
            <span>{item.id}</span>
          </SimpleList.Item>
        )}
      />,
    );
    const two = screen.getByTitle('2');
    fireEvent.click(two);
    expect(onChange).toHaveBeenCalledWith(2, 10);
  });

  it('泛型调用形态（工单详情 <SimpleList<T>>）类型与渲染一致', () => {
    interface Comment {
      author: string;
    }
    const comments: Comment[] = [{ author: 'alice' }];
    render(
      <SimpleList<Comment>
        dataSource={comments}
        rowKey={(c) => c.author}
        renderItem={(c) => (
          <SimpleList.Item>
            <SimpleList.Item.Meta title={c.author} />
          </SimpleList.Item>
        )}
      />,
    );
    expect(screen.getByText('alice')).toBeInTheDocument();
  });
});
