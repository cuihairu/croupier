import React from 'react';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import '@testing-library/jest-dom';
import FunctionListTable from '../FunctionListTable';
import type { FunctionItem } from '../FunctionListTable';

// Mock antd components
jest.mock('antd', () => ({
  ...jest.requireActual('antd'),
  Table: ({ children, ...props }: any) => <table {...props}>{children}</table>,
  Button: ({ children, onClick, ...props }: any) => (
    <button onClick={onClick} {...props}>{children}</button>
  ),
  Space: ({ children }: any) => <div>{children}</div>,
  Tag: ({ children }: any) => <span>{children}</span>,
  Badge: ({ status, text }: any) => <span>{text}</span>,
  Tooltip: ({ children, title }: any) => <span title={title}>{children}</span>,
}));

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
    display_name: { zh: '测试函数1', en: 'Test Function 1' },
    summary: { zh: '这是一个测试函数', en: 'This is a test function' },
    tags: ['test', 'demo'],
    category: 'utility',
  },
  {
    id: 'test-function-2',
    version: '2.0.0',
    enabled: false,
    display_name: { zh: '测试函数2', en: 'Test Function 2' },
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

    const invokeButton = screen.getAllByText('调用函数')[0];
    fireEvent.click(invokeButton);

    await waitFor(() => {
      expect(mockOnInvoke).toHaveBeenCalledWith(mockFunctions[0]);
    });
  });

  it('calls onViewDetail when detail button is clicked', async () => {
    const mockOnViewDetail = jest.fn();

    render(<FunctionListTable {...defaultProps} onViewDetail={mockOnViewDetail} />);

    const detailButton = screen.getAllByText('查看详情')[0];
    fireEvent.click(detailButton);

    await waitFor(() => {
      expect(mockOnViewDetail).toHaveBeenCalledWith(mockFunctions[0]);
    });
  });

  it('filters functions correctly', () => {
    render(<FunctionListTable {...defaultProps} searchable={true} />);

    // Search for specific function
    const searchInput = screen.getByPlaceholderText('搜索函数');
    fireEvent.change(searchInput, { target: { value: 'test-function-1' } });

    // Should only show matching function
    expect(screen.getByText('test-function-1')).toBeInTheDocument();
    expect(screen.queryByText('test-function-2')).not.toBeInTheDocument();
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

  it('respects compact mode', () => {
    render(<FunctionListTable {...defaultProps} compact={true} />);

    // Should have compact class or style
    const table = screen.getByRole('table');
    expect(table).toHaveClass('compact');
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

  it('handles selection correctly', async () => {
    const mockOnSelectionChange = jest.fn();

    render(
      <FunctionListTable
        {...defaultProps}
        selectable={true}
        onSelectionChange={mockOnSelectionChange}
      />
    );

    const checkboxes = screen.getAllByRole('checkbox');
    fireEvent.click(checkboxes[0]);

    await waitFor(() => {
      expect(mockOnSelectionChange).toHaveBeenCalledWith([mockFunctions[0]]);
    });
  });

  it('applies filters correctly', () => {
    render(<FunctionListTable {...defaultProps} filters={true} />);

    // Should have filter controls
    expect(screen.getByText('筛选')).toBeInTheDocument();
  });

  it('displays correct pagination', () => {
    const paginationProps = {
      current: 1,
      pageSize: 10,
      total: 25,
    };

    render(
      <FunctionListTable
        {...defaultProps}
        pagination={paginationProps}
      />
    );

    // Should show pagination info
    expect(screen.getByText('共 25 个函数')).toBeInTheDocument();
  });

  it('handles edit actions when enabled', async () => {
    const mockOnEdit = jest.fn();

    render(
      <FunctionListTable
        {...defaultProps}
        showActions={{ edit: true }}
        onEdit={mockOnEdit}
      />
    );

    const editButton = screen.getAllByText('编辑')[0];
    fireEvent.click(editButton);

    await waitFor(() => {
      expect(mockOnEdit).toHaveBeenCalledWith(mockFunctions[0]);
    });
  });

  it('handles delete actions with confirmation', async () => {
    const mockOnDelete = jest.fn();

    render(
      <FunctionListTable
        {...defaultProps}
        showActions={{ delete: true }}
        onDelete={mockOnDelete}
      />
    );

    const deleteButton = screen.getAllByText('删除')[0];
    fireEvent.click(deleteButton);

    // Should show confirmation dialog
    expect(screen.getByText('确定要删除此函数吗？')).toBeInTheDocument();

    const confirmButton = screen.getByText('确定');
    fireEvent.click(confirmButton);

    await waitFor(() => {
      expect(mockOnDelete).toHaveBeenCalledWith(mockFunctions[0]);
    });
  });

  it('handles toggle status actions', async () => {
    const mockOnToggleStatus = jest.fn();

    render(
      <FunctionListTable
        {...defaultProps}
        showActions={{ toggle: true }}
        onToggleStatus={mockOnToggleStatus}
      />
    );

    const toggleButton = screen.getAllByText('禁用')[0];
    fireEvent.click(toggleButton);

    await waitFor(() => {
      expect(mockOnToggleStatus).toHaveBeenCalledWith(mockFunctions[0]);
    });
  });

  it('displays tags correctly', () => {
    render(<FunctionListTable {...defaultProps} />);

    expect(screen.getByText('test')).toBeInTheDocument();
    expect(screen.getByText('demo')).toBeInTheDocument();
  });

  it('copies function ID to clipboard', async () => {
    const mockWriteText = jest.fn();
    Object.assign(navigator, {
      clipboard: {
        writeText: mockWriteText.mockResolvedValue(undefined),
      },
    });

    render(<FunctionListTable {...defaultProps} />);

    const copyButton = screen.getAllByTitle('复制函数ID')[0];
    fireEvent.click(copyButton);

    await waitFor(() => {
      expect(mockWriteText).toHaveBeenCalledWith('test-function-1');
    });
  });
});