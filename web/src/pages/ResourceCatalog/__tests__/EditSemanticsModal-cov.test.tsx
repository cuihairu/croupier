/**
 * EditSemanticsModal 覆盖补充：动作/任务/报表三组 Form.List 的增删行与预填默认值、
 * 函数下拉按 capability 过滤与 enabled=false 禁用、未知 capability 回退、必填校验文案、
 * functions 为 undefined 的兜底，以及 onOk/onCancel 传播。
 */
import { act, configure, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { Form } from 'antd';
import type { FormInstance } from 'antd';
import EditSemanticsModal from '../EditSemanticsModal';
import type {
  CapabilityKind,
  FunctionInfo,
  UpdateResourceSemanticsRequest,
} from '@/types/dashboard';

configure({ asyncUtilTimeout: 5000 });
jest.setTimeout(20000);

jest.mock('@umijs/max', () => ({
  FormattedMessage: ({
    defaultMessage,
  }: {
    id: string;
    defaultMessage?: string;
    values?: Record<string, string>;
  }) => <>{defaultMessage ?? ''}</>,
  useIntl: () => ({
    formatMessage: (opts: { defaultMessage?: string }, values?: Record<string, string>) => {
      let text = opts.defaultMessage ?? '';
      if (values) {
        for (const [k, v] of Object.entries(values)) text = text.split(`{${k}}`).join(v);
      }
      return text;
    },
  }),
}));

let formRef: FormInstance<UpdateResourceSemanticsRequest> | null = null;

function Wrapper({
  functions,
  onOk,
  onCancel,
}: {
  functions: FunctionInfo[];
  onOk: () => void;
  onCancel: () => void;
}) {
  const [form] = Form.useForm<UpdateResourceSemanticsRequest>();
  formRef = form;
  return (
    <EditSemanticsModal open form={form} functions={functions} onOk={onOk} onCancel={onCancel} />
  );
}

const baseFns: FunctionInfo[] = [
  {
    id: 1,
    functionId: 'player.list',
    version: '1.0',
    capability: 'collection_query',
    execution: 'sync',
    risk: 'safe',
    enabled: true,
    source: 'sdk',
  },
  {
    id: 2,
    functionId: 'player.ban',
    version: '1.0',
    capability: 'action',
    execution: 'sync',
    risk: 'high',
    enabled: false,
    source: 'sdk',
  },
  {
    id: 4,
    functionId: 'task.start',
    version: '1.0',
    capability: 'task',
    execution: 'task',
    risk: 'safe',
    enabled: true,
    source: 'sdk',
  },
  {
    id: 5,
    functionId: 'report.daily',
    version: '1.0',
    capability: 'report',
    execution: 'sync',
    risk: 'safe',
    enabled: true,
    source: 'sdk',
  },
];

const formItemOf = (labelText: string): HTMLElement => {
  const label = screen.getByText(labelText);
  const item = label.closest('.ant-form-item');
  if (!(item instanceof HTMLElement)) throw new Error(`form item not found: ${labelText}`);
  return item;
};

const openSelect = (labelText: string): HTMLElement => {
  const select = formItemOf(labelText).querySelector('.ant-select');
  if (!(select instanceof HTMLElement)) throw new Error(`select not found: ${labelText}`);
  fireEvent.mouseDown(select);
  fireEvent.click(select);
  return select;
};

const inputValueOf = (labelText: string): string => {
  const input = formItemOf(labelText).querySelector('input');
  return input instanceof HTMLInputElement ? input.value : '';
};

const validateAll = async (): Promise<void> => {
  await act(async () => {
    await formRef?.validateFields().catch(() => undefined);
  });
};

describe('EditSemanticsModal 动作语义', () => {
  it('添加动作行渲染上下文字段、预填 subject，可删除', async () => {
    render(<Wrapper functions={baseFns} onOk={jest.fn()} onCancel={jest.fn()} />);
    fireEvent.click(screen.getByText('添加动作'));
    expect(await screen.findByText('Subject')).toBeInTheDocument();
    expect(screen.getByText('Identity Input')).toBeInTheDocument();
    expect(await screen.findByText('单个资源对象')).toBeInTheDocument();
    expect(screen.queryByText('请选择 subject')).not.toBeInTheDocument();

    fireEvent.click(screen.getByText('删除'));
    await waitFor(() => expect(screen.queryByText('Subject')).not.toBeInTheDocument());
  });

  it('动作函数下拉只列 action 能力，enabled=false 的项禁用', async () => {
    render(<Wrapper functions={baseFns} onOk={jest.fn()} onCancel={jest.fn()} />);
    fireEvent.click(screen.getByText('添加动作'));
    await screen.findByText('Subject');
    openSelect('函数');

    const option = await screen.findByText('player.ban #2');
    const optionRoot = option.closest('.ant-select-item-option');
    expect(optionRoot).not.toBeNull();
    expect((optionRoot as HTMLElement).className).toContain('disabled');
    expect(screen.queryByText('task.start #4')).not.toBeInTheDocument();
    expect(screen.queryByText('report.daily #5')).not.toBeInTheDocument();
  });

  it('Subject 下拉提供三种动作上下文', async () => {
    render(<Wrapper functions={baseFns} onOk={jest.fn()} onCancel={jest.fn()} />);
    fireEvent.click(screen.getByText('添加动作'));
    await screen.findByText('Subject');
    openSelect('Subject');

    expect(await screen.findByText('选中资源集合')).toBeInTheDocument();
    expect(await screen.findByText('整个资源')).toBeInTheDocument();
    expect(screen.getAllByText('单个资源对象').length).toBeGreaterThan(0);
  });
});

describe('EditSemanticsModal 任务语义', () => {
  it('添加任务行落位默认路径，Start/Status 下拉按能力过滤', async () => {
    render(<Wrapper functions={baseFns} onOk={jest.fn()} onCancel={jest.fn()} />);
    fireEvent.click(screen.getByText('添加任务'));
    expect(await screen.findByText('Start 函数')).toBeInTheDocument();

    expect(inputValueOf('TaskID Result Path')).toBe('/taskId');
    expect(inputValueOf('Status TaskID Input')).toBe('/taskId');
    expect(inputValueOf('State Path')).toBe('/status');
    const valueTypeSelect = screen.getByText('string');
    expect(valueTypeSelect).toHaveAttribute('title', 'string');
    expect(valueTypeSelect.closest('.ant-select')).not.toBeNull();

    openSelect('Start 函数');
    expect(await screen.findByText('task.start #4 / 任务')).toBeInTheDocument();
    expect(screen.queryByText('player.list #1 / 列表查询')).not.toBeInTheDocument();

    openSelect('Status 函数');
    expect(await screen.findByText('player.list #1 / 列表查询')).toBeInTheDocument();
    expect(await screen.findByText('report.daily #5 / 报表')).toBeInTheDocument();

    fireEvent.click(screen.getByText('删除任务语义'));
    await waitFor(() => expect(screen.queryByText('Start 函数')).not.toBeInTheDocument());
  });

  it('未知 capability 回退显示原始能力名', async () => {
    const customFn: FunctionInfo = {
      id: 9,
      functionId: 'weird.fn',
      version: '1.0',
      capability: 'custom' as CapabilityKind,
      execution: 'sync',
      risk: 'safe',
      enabled: true,
      source: 'sdk',
    };
    render(<Wrapper functions={[customFn, ...baseFns]} onOk={jest.fn()} onCancel={jest.fn()} />);
    fireEvent.click(screen.getByText('添加任务'));
    await screen.findByText('Start 函数');
    openSelect('Status 函数');

    expect(await screen.findByText('weird.fn #9 / custom')).toBeInTheDocument();
    expect(await screen.findByText('player.list #1 / 列表查询')).toBeInTheDocument();
  });
});

describe('EditSemanticsModal 报表语义', () => {
  it('添加报表行落位 datasetPath，Query 下拉只列 report 能力', async () => {
    render(<Wrapper functions={baseFns} onOk={jest.fn()} onCancel={jest.fn()} />);
    fireEvent.click(screen.getByText('添加报表'));
    expect(await screen.findByText('Query 函数')).toBeInTheDocument();
    expect(inputValueOf('Dataset Path')).toBe('/dataset');

    openSelect('Query 函数');
    expect(await screen.findByText('report.daily #5 / 报表')).toBeInTheDocument();
    expect(screen.queryByText('task.start #4 / 任务')).not.toBeInTheDocument();

    fireEvent.click(screen.getByText('删除'));
    await waitFor(() => expect(screen.queryByText('Query 函数')).not.toBeInTheDocument());
  });

  it('三组列表必填校验文案齐全，预填字段不误报', async () => {
    render(<Wrapper functions={baseFns} onOk={jest.fn()} onCancel={jest.fn()} />);
    fireEvent.click(screen.getByText('添加动作'));
    fireEvent.click(screen.getByText('添加任务'));
    fireEvent.click(screen.getByText('添加报表'));
    await screen.findByText('Query 函数');
    await validateAll();

    for (const message of [
      '请选择 action 函数',
      '请选择 task start 函数',
      '请选择 status 函数',
      '请选择 report 函数',
      '至少填写一个维度指针',
      '至少填写一个指标指针',
    ]) {
      expect(await screen.findByText(message)).toBeInTheDocument();
    }
    expect(screen.queryByText('请选择 subject')).not.toBeInTheDocument();
    expect(screen.queryByText('请输入 taskId 输出路径')).not.toBeInTheDocument();
    expect(screen.queryByText('请输入 status taskId 输入路径')).not.toBeInTheDocument();
  });
});

describe('EditSemanticsModal 边界与回调', () => {
  it('functions 为 undefined 时兜底空数组，仍可加行并校验', async () => {
    render(
      <Wrapper
        functions={undefined as unknown as FunctionInfo[]}
        onOk={jest.fn()}
        onCancel={jest.fn()}
      />,
    );
    expect(screen.getByText('编辑语义')).toBeInTheDocument();

    fireEvent.click(screen.getByText('添加动作'));
    expect(await screen.findByText('Subject')).toBeInTheDocument();

    fireEvent.click(screen.getByText('添加报表'));
    expect(await screen.findByText('Query 函数')).toBeInTheDocument();

    await validateAll();
    expect(await screen.findByText('请选择 action 函数')).toBeInTheDocument();
  });

  it('保存/取消按钮传播到 onOk/onCancel', () => {
    const onOk = jest.fn();
    const onCancel = jest.fn();
    render(<Wrapper functions={baseFns} onOk={onOk} onCancel={onCancel} />);

    fireEvent.click(screen.getByRole('button', { name: /保\s*存/ }));
    expect(onOk).toHaveBeenCalledTimes(1);

    fireEvent.click(screen.getByRole('button', { name: /取\s*消/ }));
    expect(onCancel).toHaveBeenCalledTimes(1);
  });
});
