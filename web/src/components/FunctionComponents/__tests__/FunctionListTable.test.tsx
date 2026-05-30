import React from 'react';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import '@testing-library/jest-dom';
import FunctionListTable from '../../FunctionListTable';
import type { FunctionItem } from '../../FunctionListTable';

// Mock ProTable to a simple renderer for unit testing
jest.mock('@ant-design/pro-components', () => ({
  ProTable: ({ columns, dataSource = [], loading, pagination, toolBarRender }: any) => {
    const tools = toolBarRender ? toolBarRender() : [];
    return (
      <div>
        {loading ? <div>加载中</div> : null}
        {Array.isArray(tools) && tools.length > 0 ? <div>{tools}</div> : null}
        {dataSource.length === 0 ? <div>暂无数据</div> : null}
        {dataSource.map((row: any, rowIndex: number) => (
          <div key={row.id || rowIndex}>
            {columns.map((col: any, colIndex: number) => (
              <div key={col.dataIndex || col.title || colIndex}>
                {col.render ? col.render(row[col.dataIndex], row, rowIndex) : row[col.dataIndex]}
              </div>
            ))}
          </div>
        ))}
        {pagination?.total ? <div>{`共 ${pagination.total} 个函数`}</div> : null}
      </div>
    );
  },
}));

// Mock antd components
jest.mock('antd', () => {
  const React = require('react');
  return {
    ...jest.requireActual('antd'),
    Table: ({ children, ...props }: any) => <table {...props}>{children}</table>,
    Button: ({ children, onClick, title }: any) => (
      <button onClick={onClick} title={title}>
        {children}
      </button>
    ),
    Space: ({ children }: any) => <div>{children}</div>,
    Tag: ({ children }: any) => <span>{children}</span>,
    Badge: ({ status, text }: any) => <span>{text}</span>,
    Tooltip: ({ children, title, onClick }: any) => {
      const childOnClick = children.props?.onClick;
      const mergedOnClick = onClick
        ? (event: any) => {
            if (childOnClick) childOnClick(event);
            onClick(event);
          }
        : childOnClick;
      return React.cloneElement(children, {
        title,
        onClick: mergedOnClick,
      });
    },
    Popconfirm: ({ children, onConfirm }: any) => {
      const existing = children.props?.onClick;
      return React.cloneElement(children, {
        onClick: (event: any) => {
          if (existing) {
            existing(event);
          }
          if (onConfirm) {
            onConfirm(event);
          }
        },
      });
    },
  };
});

// Mock history
jest.mock('@umijs/max', () => ({
  history: {
    push: jest.fn(),
  },
}));

const mockFunctions: FunctionItem[] = [
  {
    id: 'test-function-1',
    version: '1.0.0',
    enabled: true,
    displayName: { zh: '测试函数1', en: 'Test Function 1' },
    summary: { zh: '这是一个测试函数', en: 'This is a test function' },
    tags: ['test', 'demo'],
    category: 'utility',
  },
  {
    id: 'test-function-2',
    version: '2.0.0',
    enabled: false,
    displayName: { zh: '测试函数2', en: 'Test Function 2' },
    summary: { zh: '这是另一个测试函数', en: 'This is another test function' },
    tags: ['test'],
    category: 'admin',
  },
];

describe('FunctionListTable', () => {
  const defaultProps = {
    data: mockFunctions,
    loading: false,
  };

  beforeEach(() => {
    jest.clearAllMocks();
  });

  it('renders function list correctly', () => {
    render(<FunctionListTable {...defaultProps} />);

    expect(screen.getByText('test-function-1')).toBeInTheDocument();
    expect(screen.getByText('test-function-2')).toBeInTheDocument();
  });

  it('displays loading state', () => {
    render(<FunctionListTable {...defaultProps} loading={true} />);

    // Should show loading indicator
    expect(screen.getByText('加载中')).toBeInTheDocument();
  });

  it('calls onInvoke when invoke button is clicked', async () => {
    const mockOnInvoke = jest.fn();

    render(<FunctionListTable {...defaultProps} onInvoke={mockOnInvoke} />);

    const invokeButton = screen.getAllByTitle('调用函数')[0];
    fireEvent.click(invokeButton);

    await waitFor(() => {
      expect(mockOnInvoke).toHaveBeenCalledWith(expect.objectContaining({ id: 'test-function-1' }));
    });
  });

  it('calls onViewDetail when detail button is clicked', async () => {
    const mockOnViewDetail = jest.fn();

    render(<FunctionListTable {...defaultProps} onViewDetail={mockOnViewDetail} />);

    const detailButton = screen.getAllByTitle('查看详情')[0];
    fireEvent.click(detailButton);

    await waitFor(() => {
      expect(mockOnViewDetail).toHaveBeenCalledWith(
        expect.objectContaining({ id: 'test-function-1' }),
      );
    });
  });

  it('displays function status correctly', () => {
    render(<FunctionListTable {...defaultProps} />);

    // Check status badges
    const statusBadges = screen.getAllByText('启用');
    expect(statusBadges).toHaveLength(1);
    expect(screen.getByText('禁用')).toBeInTheDocument();
  });

  it('shows version tags correctly', () => {
    render(<FunctionListTable {...defaultProps} />);

    expect(screen.getByText('v1.0.0')).toBeInTheDocument();
    expect(screen.getByText('v2.0.0')).toBeInTheDocument();
  });

  it('handles empty data correctly', () => {
    render(<FunctionListTable {...defaultProps} data={[]} />);

    expect(screen.getByText('暂无数据')).toBeInTheDocument();
  });

  it('calls onRefresh when refresh button is clicked', async () => {
    const mockOnRefresh = jest.fn();

    render(<FunctionListTable {...defaultProps} onRefresh={mockOnRefresh} />);

    const refreshButton = screen.getByText('刷新');
    fireEvent.click(refreshButton);

    await waitFor(() => {
      expect(mockOnRefresh).toHaveBeenCalled();
    });
  });

  it('displays correct pagination', () => {
    const paginationProps = {
      current: 1,
      pageSize: 10,
      total: 25,
    };

    render(<FunctionListTable {...defaultProps} pagination={paginationProps} />);

    // Should show pagination info
    expect(screen.getByText('共 25 个函数')).toBeInTheDocument();
  });

  it('handles edit actions when enabled', async () => {
    const mockOnEdit = jest.fn();

    render(
      <FunctionListTable {...defaultProps} showActions={{ edit: true }} onEdit={mockOnEdit} />,
    );

    const editButton = screen.getAllByTitle('编辑')[0];
    fireEvent.click(editButton);

    await waitFor(() => {
      expect(mockOnEdit).toHaveBeenCalledWith(expect.objectContaining({ id: 'test-function-1' }));
    });
  });

  it('handles delete actions', async () => {
    const mockOnDelete = jest.fn();

    render(
      <FunctionListTable
        {...defaultProps}
        showActions={{ delete: true }}
        onDelete={mockOnDelete}
      />,
    );

    const deleteButton = screen.getAllByTitle('删除')[0];
    fireEvent.click(deleteButton);

    await waitFor(() => {
      expect(mockOnDelete).toHaveBeenCalledWith(expect.objectContaining({ id: 'test-function-1' }));
    });
  });

  it('handles toggle status actions', async () => {
    const mockOnToggleStatus = jest.fn();

    render(
      <FunctionListTable
        {...defaultProps}
        showActions={{ toggle: true }}
        onToggleStatus={mockOnToggleStatus}
      />,
    );

    const toggleButton = screen.getAllByTitle('禁用')[0];
    fireEvent.click(toggleButton);

    await waitFor(() => {
      expect(mockOnToggleStatus).toHaveBeenCalledWith(
        expect.objectContaining({ id: 'test-function-1' }),
      );
    });
  });

  it('displays tags correctly', () => {
    render(<FunctionListTable {...defaultProps} />);

    expect(screen.getAllByText('test').length).toBeGreaterThan(0);
    expect(screen.getByText('demo')).toBeInTheDocument();
  });
});
