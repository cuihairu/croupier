/** widgets-upload.tsx 直渲（UploadWidget / KeyValueField 不经 RJSF 集成链）：
 * 覆盖 toValueArray/urlToName/valueToFileList 的值归一分支（数组含空串被过滤、
 * 尾斜杠 URL 无文件名回退整 URL、非字符串非数组值兜底空列表）、UploadWidget
 * options 透传（缺省 {} 兜底、listType 三态、multiple 两路判定、类型卫兵、
 * handleChange 值提取）、KeyValueField 的 idSchema 兜底、非字符串行值 JSON
 * 序列化、placeholder 覆盖、外部 formData 变更同步 effect、onChange 第二参
 * path 语义。测试手法：mock antd Upload 为记录 props 的桩（渲染 children 以
 * 断言按钮文案），Input/Button/Space 用真实组件。 */
import React from 'react';
import { fireEvent, render, screen } from '@testing-library/react';
import { KeyValueField, UploadWidget } from '../widgets-upload';
import type { FieldProps, RJSFSchema, WidgetProps } from '@rjsf/utils';

type StubProps = Record<string, unknown>;

type UploadStub = ((props: StubProps) => React.ReactElement) & {
  __registry: StubProps[];
};

// antd 桩：Upload 记录每次渲染收到的 props，其余导出保持真实
jest.mock('antd', () => {
  const actual = jest.requireActual('antd');
  const registry: StubProps[] = [];
  const Upload = (props: StubProps) => {
    registry.push(props);
    const testid = typeof props['data-testid'] === 'string' ? props['data-testid'] : 'upload-stub';
    return <div data-testid={testid}>{props.children as React.ReactNode}</div>;
  };
  Upload.__registry = registry;
  return { ...actual, Upload };
});

const antdMock = jest.requireMock('antd') as { Upload: UploadStub };

/** 取指定 testid 的最近一次 Upload 桩渲染 props。 */
function uploadProps(testid: string): StubProps {
  const found = antdMock.Upload.__registry.filter((p) => p['data-testid'] === testid);
  if (!found.length) throw new Error(`no upload stub render captured for ${testid}`);
  return found[found.length - 1];
}

type WidgetOverrides = Partial<WidgetProps> & Record<string, unknown>;

function makeWidgetProps(overrides: WidgetOverrides = {}): WidgetProps {
  return {
    id: 'up',
    schema: { type: 'string' } as RJSFSchema,
    uiSchema: {},
    options: {},
    value: undefined,
    required: false,
    disabled: false,
    readonly: false,
    autofocus: false,
    placeholder: '',
    registry: { formContext: {} } as never,
    formContext: {},
    rawErrors: [],
    hideError: false,
    label: '字段',
    multiple: undefined,
    onChange: jest.fn(),
    onBlur: jest.fn(),
    onFocus: jest.fn(),
    ...overrides,
  } as WidgetProps;
}

interface KVOverrides {
  formData?: unknown;
  idSchema?: { $id?: string };
  uiSchema?: Record<string, unknown>;
  fieldPathId?: { $id: string; path: string[] };
  disabled?: boolean;
  readonly?: boolean;
  onChange?: jest.Mock;
}

function kvProps(overrides: KVOverrides = {}): FieldProps {
  const onChange = overrides.onChange ?? jest.fn();
  return {
    disabled: false,
    readonly: false,
    schema: { type: 'object' },
    uiSchema: {},
    idSchema: {},
    formData: undefined,
    onChange,
    ...overrides,
  } as unknown as FieldProps;
}

beforeEach(() => {
  antdMock.Upload.__registry.length = 0;
});

describe('UploadWidget 值归一（value → fileList）', () => {
  it('数组值含空串被过滤，仅保留非空 URL（map(String)+filter(Boolean)）', () => {
    render(
      <UploadWidget
        {...makeWidgetProps({
          id: 'up-arr',
          value: ['http://x/a.png', ''],
          schema: { type: 'array' } as RJSFSchema,
        })}
      />,
    );
    const fileList = uploadProps('up-arr').fileList as Array<{
      name: string;
      url: string;
      status: string;
      uid: string;
    }>;
    expect(fileList).toHaveLength(1);
    expect(fileList[0]).toMatchObject({
      name: 'a.png',
      url: 'http://x/a.png',
      status: 'done',
      uid: '0-http://x/a.png',
    });
  });

  it('单值字符串 → 单元素列表；非字符串非数组（数字）→ 空列表', () => {
    render(<UploadWidget {...makeWidgetProps({ id: 'up-solo', value: 'http://x/solo.png' })} />);
    expect(uploadProps('up-solo').fileList).toHaveLength(1);

    render(<UploadWidget {...makeWidgetProps({ id: 'up-num', value: 42 })} />);
    expect(uploadProps('up-num').fileList).toEqual([]);
  });

  it('urlToName：query/hash 剥离、百分号解码；尾斜杠无文件名回退整 URL', () => {
    render(
      <UploadWidget
        {...makeWidgetProps({
          id: 'up-name',
          value: ['http://x/a.png?w=1#f', 'http://x/%E4%B8%AD%E6%96%87.png', 'http://x/dir/'],
          schema: { type: 'array' } as RJSFSchema,
        })}
      />,
    );
    const names = (uploadProps('up-name').fileList as Array<{ name: string }>).map((f) => f.name);
    expect(names).toEqual(['a.png', '中文.png', 'http://x/dir/']);
  });
});

describe('UploadWidget options 透传', () => {
  it('options 缺省走 {} 兜底：action/accept/maxCount 未定、listType 回落 text、multiple 按 schema', () => {
    render(<UploadWidget {...makeWidgetProps({ id: 'up-bare', options: undefined })} />);
    const p = uploadProps('up-bare');
    expect(p.action).toBeUndefined();
    expect(p.accept).toBeUndefined();
    expect(p.maxCount).toBeUndefined();
    expect(p.listType).toBe('text');
    expect(p.multiple).toBe(false);
    expect(p.disabled).toBe(false);
  });

  it('options 全量透传：action/accept/maxCount/listType picture-card/multiple 显式 true', () => {
    render(
      <UploadWidget
        {...makeWidgetProps({
          id: 'up-full',
          options: {
            action: '/upload',
            accept: '.png',
            maxCount: 3,
            listType: 'picture-card',
            multiple: true,
          },
        })}
      />,
    );
    const p = uploadProps('up-full');
    expect(p.action).toBe('/upload');
    expect(p.accept).toBe('.png');
    expect(p.maxCount).toBe(3);
    expect(p.listType).toBe('picture-card');
    expect(p.multiple).toBe(true);
  });

  it('listType 非法值（custom/数字）回落 text；schema.type=array → multiple=true', () => {
    render(
      <UploadWidget
        {...makeWidgetProps({
          id: 'up-arrtype',
          options: { listType: 'custom' },
          schema: { type: 'array' } as RJSFSchema,
        })}
      />,
    );
    expect(uploadProps('up-arrtype').listType).toBe('text');
    expect(uploadProps('up-arrtype').multiple).toBe(true);

    render(
      <UploadWidget
        {...makeWidgetProps({
          id: 'up-picture',
          options: { listType: 'picture' },
        })}
      />,
    );
    expect(uploadProps('up-picture').listType).toBe('picture');
  });

  it('非字符串 action/listType/accept 与非数字 maxCount 被类型卫兵过滤', () => {
    render(
      <UploadWidget
        {...makeWidgetProps({
          id: 'up-types',
          options: { action: 123, listType: 42, maxCount: '3', accept: false },
        })}
      />,
    );
    const p = uploadProps('up-types');
    expect(p.action).toBeUndefined();
    expect(p.listType).toBe('text');
    expect(p.maxCount).toBeUndefined();
    expect(p.accept).toBeUndefined();
  });

  it('placeholder 覆盖按钮文案；缺省回退「上传」', () => {
    const { unmount } = render(
      <UploadWidget {...makeWidgetProps({ id: 'up-ph', placeholder: '点击上传' })} />,
    );
    expect(screen.getByText('点击上传')).toBeTruthy();
    unmount();
    render(<UploadWidget {...makeWidgetProps({ id: 'up-noph' })} />);
    // antd 双字按钮自动插空格
    expect(screen.getByText(/上\s*传/)).toBeTruthy();
  });

  it('disabled/readonly 任一为真：Upload 与触发按钮均禁用', () => {
    render(<UploadWidget {...makeWidgetProps({ id: 'up-dis', disabled: true })} />);
    expect(uploadProps('up-dis').disabled).toBe(true);
    expect((screen.getByText(/上\s*传/).closest('button') as HTMLButtonElement).disabled).toBe(
      true,
    );

    render(<UploadWidget {...makeWidgetProps({ id: 'up-ro', readonly: true })} />);
    expect(uploadProps('up-ro').disabled).toBe(true);
  });

  it('handleChange：done 文件提取 URL —— 单值收 string、多值收数组（uploading 跳过）', () => {
    const single = makeWidgetProps({ id: 'up-chg1' });
    render(<UploadWidget {...single} />);
    const chg1 = uploadProps('up-chg1').onChange as (info: { fileList: unknown[] }) => void;
    chg1({ fileList: [{ status: 'done', url: 'http://x/new.png' }] });
    expect(single.onChange).toHaveBeenCalledWith('http://x/new.png');

    const multi = makeWidgetProps({ id: 'up-chg2', schema: { type: 'array' } as RJSFSchema });
    render(<UploadWidget {...multi} />);
    const chg2 = uploadProps('up-chg2').onChange as (info: { fileList: unknown[] }) => void;
    chg2({
      fileList: [
        { status: 'uploading' },
        { status: 'done', url: 'http://x/1.png' },
        { status: 'done', response: { url: 'http://x/2.png' } },
      ],
    });
    expect(multi.onChange).toHaveBeenCalledWith(['http://x/1.png', 'http://x/2.png']);
  });
});

describe('KeyValueField 直渲', () => {
  it('idSchema 缺 $id → data-testid 兜底 keyValue；非字符串行值 JSON 序列化', () => {
    render(<KeyValueField {...kvProps({ formData: { n: 5, obj: { x: 1 }, s: 'str' } })} />);
    expect(screen.getByTestId('keyValue')).toBeTruthy();
    expect(screen.getByTestId('keyValue-row-0')).toBeTruthy();
    // typeof v === 'string' ? v : JSON.stringify(v) 两路
    expect(screen.getByDisplayValue('5')).toBeTruthy();
    expect(screen.getByDisplayValue('{"x":1}')).toBeTruthy();
    expect(screen.getByDisplayValue('str')).toBeTruthy();
  });

  it('idSchema.$id 存在 → testid 取 $id', () => {
    render(<KeyValueField {...kvProps({ idSchema: { $id: 'root_extra' }, formData: {} })} />);
    expect(screen.getByTestId('root_extra')).toBeTruthy();
    expect(screen.getByTestId('root_extra-add')).toBeTruthy();
  });

  it('formData 非对象（undefined/数组）→ 无行', () => {
    const { container } = render(<KeyValueField {...kvProps({ formData: undefined })} />);
    expect(container.querySelectorAll('[data-testid^="keyValue-row-"]')).toHaveLength(0);

    const second = render(<KeyValueField {...kvProps({ formData: ['a'] })} />);
    expect(second.container.querySelectorAll('[data-testid^="keyValue-row-"]')).toHaveLength(0);
  });

  it('ui:options.placeholder 覆盖值输入 placeholder；缺省回退「值」', () => {
    const { unmount } = render(
      <KeyValueField
        {...kvProps({
          formData: { a: '1' },
          uiSchema: { 'ui:options': { placeholder: '输入值' } },
        })}
      />,
    );
    expect(screen.getByPlaceholderText('输入值')).toBeTruthy();
    expect(screen.queryByPlaceholderText('值')).toBeNull();
    unmount();

    render(<KeyValueField {...kvProps({ formData: { a: '1' } })} />);
    expect(screen.getByPlaceholderText('值')).toBeTruthy();
    expect(screen.getByPlaceholderText('键')).toBeTruthy();
  });

  it('外部 formData 变更 → 行重建同步（effect 对比 committed）；同值重渲不重建', () => {
    const { rerender } = render(<KeyValueField {...kvProps({ formData: { a: '1' } })} />);
    expect(screen.getByDisplayValue('a')).toBeTruthy();
    rerender(<KeyValueField {...kvProps({ formData: { b: '2' } })} />);
    expect(screen.getByDisplayValue('b')).toBeTruthy();
    expect(screen.queryByDisplayValue('a')).toBeNull();
    // 新对象同值 → external === committed，行保持
    rerender(<KeyValueField {...kvProps({ formData: { b: '2' } })} />);
    expect(screen.getByDisplayValue('b')).toBeTruthy();
  });

  it('onChange 第二参：fieldPathId 缺省 → []；有 path → 原样 path', () => {
    const onChange1 = jest.fn();
    render(<KeyValueField {...kvProps({ formData: { a: '1' }, onChange: onChange1 })} />);
    fireEvent.change(screen.getByDisplayValue('1'), { target: { value: 'x' } });
    expect(onChange1).toHaveBeenCalledWith({ a: 'x' }, []);

    const onChange2 = jest.fn();
    render(
      <KeyValueField
        {...kvProps({
          formData: { a: '1' },
          fieldPathId: { $id: 'root_extra', path: ['root', 'extra'] },
          onChange: onChange2,
        })}
      />,
    );
    fireEvent.change(screen.getByDisplayValue('1'), { target: { value: 'x' } });
    expect(onChange2).toHaveBeenCalledWith({ a: 'x' }, ['root', 'extra']);
  });

  it('key trim 后空 key 行不计入提交对象', () => {
    const onChange = jest.fn();
    render(<KeyValueField {...kvProps({ formData: { ' k ': 'v', '': 'drop' }, onChange })} />);
    fireEvent.change(screen.getByDisplayValue('v'), { target: { value: 'v2' } });
    expect(onChange).toHaveBeenCalledWith({ k: 'v2' }, []);
  });

  it('删除行：rows 过滤后提交；添加行：空行追加并 commit 空对象', () => {
    const onChangeDel = jest.fn();
    const del = render(
      <KeyValueField {...kvProps({ formData: { a: '1', b: '2' }, onChange: onChangeDel })} />,
    );
    fireEvent.click(del.getByTestId('keyValue-row-0').querySelector('button') as Element);
    expect(onChangeDel).toHaveBeenCalledWith({ b: '2' }, []);
    del.unmount();

    const onChangeAdd = jest.fn();
    const add = render(
      <KeyValueField {...kvProps({ formData: undefined, onChange: onChangeAdd })} />,
    );
    fireEvent.click(add.getByTestId('keyValue-add'));
    expect(onChangeAdd).toHaveBeenCalledWith({}, []);
    expect(add.container.querySelectorAll('[data-testid^="keyValue-row-"]')).toHaveLength(1);
  });

  it('disabled：键值输入与删除/添加按钮禁用', () => {
    render(<KeyValueField {...kvProps({ formData: { a: '1' }, disabled: true })} />);
    const row = screen.getByTestId('keyValue-row-0');
    const inputs = row.querySelectorAll('input');
    expect((inputs[0] as HTMLInputElement).disabled).toBe(true);
    expect((inputs[1] as HTMLInputElement).disabled).toBe(true);
    expect((row.querySelector('button') as HTMLButtonElement).disabled).toBe(true);
    expect((screen.getByTestId('keyValue-add') as HTMLButtonElement).disabled).toBe(true);
  });
});
