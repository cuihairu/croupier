/**
 * DetailSections 覆盖补充：
 * - JsonViewer：pretty JSON 输出、data 为空兜底、复制成功/失败回调、beforeMount 主题分支；
 * - BasicInfoTab：null detail 兜底默认值、完整字段/异常健康标签、编辑态表单必填校验、
 *   标签分区显隐、状态开关回调；
 * - PermissionsTab：permError 提示、functionId 缺省禁用保存、permSaving loading、
 *   规则增删与必填校验、角色接口返回缺 items 的降级。
 */
import type { ComponentProps } from 'react';
import { act, configure, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { Form } from 'antd';
import type { FormInstance } from 'antd';
import { BasicInfoTab, JsonViewer, PermissionsTab } from '../DetailSections';
import type { FunctionDetailData } from '../DetailSections';
import { listRoles } from '@/services/api/permissions';
import { formatDateTime } from '@/utils/format';

configure({ asyncUtilTimeout: 5000 });
jest.setTimeout(20000);

jest.mock('@/services/api/permissions', () => ({ listRoles: jest.fn() }));
const mockListRoles = jest.mocked(listRoles);

type FakeEditorProps = { value?: unknown; beforeMount?: (monaco: unknown) => void };
const mockEditorState: { current: FakeEditorProps | null } = { current: null };
jest.mock('@monaco-editor/react', () => ({
  __esModule: true,
  default: (props: FakeEditorProps) => {
    mockEditorState.current = props;
    return (
      <div data-testid="fake-monaco">{typeof props.value === 'string' ? props.value : ''}</div>
    );
  },
}));

type BasicProps = ComponentProps<typeof BasicInfoTab>;
let basicForm: FormInstance | null = null;
function BasicHarness(props: BasicProps) {
  const [form] = Form.useForm();
  basicForm = form;
  return (
    <Form form={form}>
      <BasicInfoTab {...props} />
    </Form>
  );
}

type PermProps = Omit<ComponentProps<typeof PermissionsTab>, 'permForm'>;
let permForm: FormInstance | null = null;
function PermHarness(props: PermProps) {
  const [form] = Form.useForm();
  permForm = form;
  return <PermissionsTab {...props} permForm={form} />;
}

const detail: FunctionDetailData = {
  id: 'inventory.consume',
  description: '消耗库存并回写余量',
  resource: 'inventory',
  operation: 'consume',
  version: '2.1.0',
  enabled: true,
  tags: ['core', 'economy'],
  createdAt: '2024-03-01T08:30:00.000Z',
  updatedAt: '2024-03-02T10:00:00.000Z',
  provider: 'local',
  agentCount: 3,
  health: 'healthy',
};

const noop = () => undefined;

const formItemOf = (labelText: string): HTMLElement => {
  const label = screen.getByText(labelText);
  const item = label.closest('.ant-form-item');
  if (!(item instanceof HTMLElement)) throw new Error(`form item not found: ${labelText}`);
  return item;
};

const inputValueOf = (labelText: string): string => {
  const input = formItemOf(labelText).querySelector('input');
  return input instanceof HTMLInputElement ? input.value : '';
};

const tagClassOf = (text: string): string => {
  const tag = screen.getByText(text).closest('.ant-tag');
  return tag instanceof HTMLElement ? tag.className : '';
};

const setClipboard = (writeText: jest.Mock): void => {
  Object.defineProperty(navigator, 'clipboard', {
    value: { writeText },
    configurable: true,
    writable: true,
  });
};

describe('JsonViewer', () => {
  beforeEach(() => {
    mockEditorState.current = null;
  });

  it('把数据渲染成 pretty JSON', async () => {
    render(<JsonViewer data={{ a: 1 }} onCopySuccess={jest.fn()} onCopyError={jest.fn()} />);
    const editor = await screen.findByTestId('fake-monaco');
    expect(editor.textContent).toBe(JSON.stringify({ a: 1 }, null, 2));
  });

  it('data 为空时兜底 {}', async () => {
    render(<JsonViewer data={null} onCopySuccess={jest.fn()} onCopyError={jest.fn()} />);
    const editor = await screen.findByTestId('fake-monaco');
    expect(editor.textContent).toBe('{}');
  });

  it('复制成功回调 onCopySuccess 并写入 pretty JSON', async () => {
    const writeText = jest.fn().mockResolvedValue(undefined);
    setClipboard(writeText);
    const onCopySuccess = jest.fn();
    const onCopyError = jest.fn();
    render(<JsonViewer data={{ a: 1 }} onCopySuccess={onCopySuccess} onCopyError={onCopyError} />);

    fireEvent.click(screen.getByRole('button', { name: /复\s*制/ }));
    await waitFor(() => expect(onCopySuccess).toHaveBeenCalledTimes(1));
    expect(writeText).toHaveBeenCalledWith(JSON.stringify({ a: 1 }, null, 2));
    expect(onCopyError).not.toHaveBeenCalled();
  });

  it('剪贴板写入失败时回调 onCopyError', async () => {
    const writeText = jest.fn().mockRejectedValue(new Error('denied'));
    setClipboard(writeText);
    const onCopySuccess = jest.fn();
    const onCopyError = jest.fn();
    render(<JsonViewer data={{ a: 1 }} onCopySuccess={onCopySuccess} onCopyError={onCopyError} />);

    fireEvent.click(screen.getByRole('button', { name: /复\s*制/ }));
    await waitFor(() => expect(onCopyError).toHaveBeenCalledTimes(1));
    expect(onCopySuccess).not.toHaveBeenCalled();
  });

  it('beforeMount 按主题状态决定是否注册 sublime-monokai', async () => {
    render(<JsonViewer data={{ a: 1 }} onCopySuccess={jest.fn()} onCopyError={jest.fn()} />);
    await screen.findByTestId('fake-monaco');
    const beforeMount = mockEditorState.current?.beforeMount;
    expect(beforeMount).toEqual(expect.any(Function));

    const defineTheme = jest.fn();
    beforeMount?.({ editor: { getTheme: () => 'vs-dark', defineTheme } });
    expect(defineTheme).toHaveBeenCalledTimes(1);
    expect(defineTheme).toHaveBeenCalledWith(
      'sublime-monokai',
      expect.objectContaining({ base: 'vs-dark' }),
    );

    beforeMount?.({ editor: { getTheme: () => 'sublime-monokai', defineTheme } });
    expect(defineTheme).toHaveBeenCalledTimes(1);

    expect(() => beforeMount?.({})).not.toThrow();
    expect(() => beforeMount?.({ editor: {} })).not.toThrow();
  });
});

describe('BasicInfoTab', () => {
  it('完整字段渲染标签、时间与描述', () => {
    render(
      <BasicHarness
        functionDetail={detail}
        effectiveResource="inventory"
        editing={false}
        onStatusToggle={noop}
      />,
    );

    expect(screen.getByText('inventory.consume').tagName).toBe('CODE');
    expect(screen.getByText('2.1.0')).toBeInTheDocument();
    expect(tagClassOf('inventory')).toContain('ant-tag-blue');
    expect(tagClassOf('consume')).toContain('ant-tag-purple');
    expect(screen.getByRole('switch')).toBeChecked();
    expect(screen.getByText('已启用')).toBeInTheDocument();
    expect(screen.getByText('local')).toBeInTheDocument();
    expect(tagClassOf('健康')).toContain('ant-tag-green');
    expect(screen.getByText('3')).toBeInTheDocument();
    expect(screen.getByText(formatDateTime(detail.createdAt))).toBeInTheDocument();
    expect(screen.getByText(formatDateTime(detail.updatedAt))).toBeInTheDocument();
    expect(screen.getByText('消耗库存并回写余量')).toBeInTheDocument();
    expect(screen.getByText('标签')).toBeInTheDocument();
    expect(tagClassOf('core')).toContain('ant-tag-geekblue');
    expect(screen.getByText('economy')).toBeInTheDocument();
    expect(screen.queryByText('编辑信息')).not.toBeInTheDocument();
  });

  it('functionDetail 为空时兜底默认值', () => {
    render(
      <BasicHarness
        functionDetail={null}
        effectiveResource=""
        editing={false}
        onStatusToggle={noop}
      />,
    );

    expect(screen.getByText('1.0.0')).toBeInTheDocument();
    expect(screen.getAllByText('未声明').length).toBe(2);
    expect(screen.getByText('已禁用')).toBeInTheDocument();
    expect(screen.getByRole('switch')).not.toBeChecked();
    const unknownTag = screen.getByText('未知').closest('.ant-tag');
    expect(unknownTag).not.toBeNull();
    expect(unknownTag).toHaveStyle({ color: 'rgb(128, 128, 128)' });
    expect(screen.getByText('0')).toBeInTheDocument();
    expect(screen.getAllByText('-').length).toBeGreaterThanOrEqual(3);
    expect(screen.getByText('暂无描述')).toBeInTheDocument();
    expect(screen.queryByText('标签')).not.toBeInTheDocument();
    expect(screen.queryByText('编辑信息')).not.toBeInTheDocument();
  });

  it('unhealthy 显示红色异常标签并回落为已禁用', () => {
    render(
      <BasicHarness
        functionDetail={{ ...detail, health: 'unhealthy', enabled: false }}
        effectiveResource="inventory"
        editing={false}
        onStatusToggle={noop}
      />,
    );

    expect(tagClassOf('异常')).toContain('ant-tag-red');
    expect(screen.getByText('已禁用')).toBeInTheDocument();
  });

  it('编辑态渲染表单，必填校验报「请输入函数名称」', async () => {
    render(
      <BasicHarness
        functionDetail={detail}
        effectiveResource="inventory"
        editing
        onStatusToggle={noop}
      />,
    );
    expect(screen.getByText('编辑信息')).toBeInTheDocument();
    expect(screen.queryByText('暂无描述')).not.toBeInTheDocument();

    await act(async () => {
      await basicForm?.validateFields().catch(() => undefined);
    });
    expect(await screen.findByText('请输入函数名称')).toBeInTheDocument();
  });

  it('状态开关点击回调 onStatusToggle', () => {
    const onStatusToggle = jest.fn();
    render(
      <BasicHarness
        functionDetail={detail}
        effectiveResource="inventory"
        editing={false}
        onStatusToggle={onStatusToggle}
      />,
    );

    const toggle = screen.getByRole('switch');
    expect(toggle).toBeChecked();
    fireEvent.click(toggle);
    expect(onStatusToggle).toHaveBeenCalledTimes(1);
    expect(onStatusToggle.mock.calls[0]?.[0]).toBe(false);
  });
});

describe('PermissionsTab', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockListRoles.mockResolvedValue({ items: [], total: 0, page: 1, pageSize: 200 });
  });

  it('permError 非空时渲染错误提示', async () => {
    render(
      <PermHarness
        functionId="inventory.consume"
        permError="网络异常"
        permLoading={false}
        permSaving={false}
        onSave={jest.fn()}
      />,
    );

    expect(await screen.findByText('无法读取权限')).toBeInTheDocument();
    expect(screen.getByText('网络异常')).toBeInTheDocument();
    expect(screen.getByText('权限配置')).toBeInTheDocument();
  });

  it('functionId 缺省时保存禁用，permSaving 时进入 loading', () => {
    const { rerender } = render(
      <PermHarness permError="" permLoading={false} permSaving={false} onSave={jest.fn()} />,
    );
    expect(screen.getByRole('button', { name: '保存权限' })).toBeDisabled();

    rerender(
      <PermHarness
        functionId="inventory.consume"
        permError=""
        permLoading={false}
        permSaving
        onSave={jest.fn()}
      />,
    );
    const loadingButton = screen.getByText('保存权限').closest('button');
    expect(loadingButton).not.toBeNull();
    expect((loadingButton as HTMLElement).className).toContain('ant-btn-loading');
  });

  it('functionId 就绪时点击保存触发 onSave', () => {
    const onSave = jest.fn().mockResolvedValue(undefined);
    render(
      <PermHarness
        functionId="inventory.consume"
        permError=""
        permLoading={false}
        permSaving={false}
        onSave={onSave}
      />,
    );

    fireEvent.click(screen.getByRole('button', { name: '保存权限' }));
    expect(onSave).toHaveBeenCalledTimes(1);
  });

  it('添加规则落位默认值，roles 为空触发必填校验', async () => {
    render(
      <PermHarness
        functionId="inventory.consume"
        permError=""
        permLoading={false}
        permSaving={false}
        onSave={jest.fn()}
      />,
    );
    fireEvent.click(await screen.findByRole('button', { name: '添加规则' }));
    expect(await screen.findByText('规则 #1')).toBeInTheDocument();
    await waitFor(() => expect(inputValueOf('resource')).toBe('function'));
    expect(await screen.findByText('invoke')).toBeInTheDocument();

    await act(async () => {
      await permForm?.validateFields().catch(() => undefined);
    });
    expect(await screen.findByText('roles 必填（至少 1 个）')).toBeInTheDocument();
    expect(screen.queryByText('resource 必填')).not.toBeInTheDocument();
    expect(screen.queryByText('actions 必填')).not.toBeInTheDocument();
  });

  it('resource/actions 为空触发必填校验，roles 已填不误报', async () => {
    render(
      <PermHarness
        functionId="inventory.consume"
        permError=""
        permLoading={false}
        permSaving={false}
        onSave={jest.fn()}
      />,
    );
    await act(async () => {
      permForm?.setFieldsValue({ items: [{ resource: '', actions: [], roles: ['ops'] }] });
    });
    await act(async () => {
      await permForm?.validateFields().catch(() => undefined);
    });

    expect(await screen.findByText('resource 必填')).toBeInTheDocument();
    expect(await screen.findByText('actions 必填')).toBeInTheDocument();
    expect(screen.queryByText('roles 必填（至少 1 个）')).not.toBeInTheDocument();
  });

  it('删除规则移除该行', async () => {
    render(
      <PermHarness
        functionId="inventory.consume"
        permError=""
        permLoading={false}
        permSaving={false}
        onSave={jest.fn()}
      />,
    );
    fireEvent.click(await screen.findByRole('button', { name: '添加规则' }));
    expect(await screen.findByText('规则 #1')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: '删除' }));
    await waitFor(() => expect(screen.queryByText('规则 #1')).not.toBeInTheDocument());
  });

  it('角色接口返回缺 items 时降级为空选项', async () => {
    mockListRoles.mockResolvedValue({} as unknown as Awaited<ReturnType<typeof listRoles>>);
    render(
      <PermHarness
        functionId="inventory.consume"
        permError=""
        permLoading={false}
        permSaving={false}
        onSave={jest.fn()}
      />,
    );

    await waitFor(() => expect(mockListRoles).toHaveBeenCalled());
    expect(await screen.findByRole('button', { name: '添加规则' })).toBeInTheDocument();
  });
});
