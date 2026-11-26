/** FormVariantsEditor（SUI-034）：命名展示风格的新增（key 唯一顺延）、改名
 * 同步 defaultVariant、删除清理默认项、设/取消默认、逐字段列宽与控件覆盖
 * （清空还原整条移除）、预览走 resolveVariantPresentation 合并结果、readonly
 * 隐藏管理操作。风格与绑定的边界在提示文案中区分（本文件不涉及 selector）。 */
import React from 'react';
import { fireEvent, render, screen, within } from '@testing-library/react';
import FormVariantsEditor from '../FormVariantsEditor';
import type { FormPresentationSpec } from '@/types/dashboard';

jest.setTimeout(15000);

jest.mock('@umijs/max', () => ({
  __esModule: true,
  FormattedMessage: ({ defaultMessage }: { defaultMessage?: string }) => <>{defaultMessage}</>,
  useIntl: () => ({
    locale: 'zh-CN',
    formatMessage: (
      { defaultMessage }: { defaultMessage?: string },
      values?: Record<string, unknown>,
    ) =>
      Object.entries(values || {}).reduce(
        (msg, [key, val]) => msg.split(`{${key}}`).join(String(val)),
        defaultMessage ?? '',
      ),
  }),
}));

// SchemaFormRenderer 替身：把收到的 spec 挂到 DOM 供断言预览合并结果
jest.mock('@/components/SchemaFormRenderer', () => {
  const R = require('react') as typeof import('react');
  return {
    __esModule: true,
    default: (props: { spec?: unknown }) =>
      R.createElement('div', {
        'data-testid': 'schema-renderer-stub',
        'data-spec': JSON.stringify(props.spec),
      }),
  };
});

/** 打开 Select 下拉并点击匹配文案的选项 */
const pickOption = async (combobox: HTMLElement, label: string): Promise<void> => {
  fireEvent.mouseDown(combobox);
  let found: HTMLElement | undefined;
  for (let i = 0; i < 60 && !found; i += 1) {
    await new Promise((resolve) => setTimeout(resolve, 25));
    const dropdowns = document.querySelectorAll<HTMLElement>(
      '.ant-select-dropdown:not(.ant-select-dropdown-hidden)',
    );
    for (const dropdown of dropdowns) {
      const hit = Array.from(
        dropdown.querySelectorAll<HTMLElement>('.ant-select-item-option'),
      ).find((item) => item.textContent?.includes(label));
      if (hit) {
        found = hit;
        break;
      }
    }
  }
  if (!found) throw new Error(`下拉选项未出现：${label}`);
  fireEvent.click(found);
};

const baseWithVariants = (): FormPresentationSpec => ({
  jsonSchema: { type: 'object' },
  layout: 'vertical',
  fields: [
    { key: 'playerId', widget: 'Input', label: { 'zh-CN': '玩家' }, width: 12 },
    { key: 'reason', widget: 'TextArea' },
  ],
  variants: [
    {
      key: 'standard',
      title: { 'zh-CN': '标准', 'en-US': 'Standard' },
      fields: [{ key: 'playerId', width: 12 }],
    },
    {
      key: 'compact',
      title: { 'zh-CN': '紧凑' },
      layout: 'grid',
      fields: [{ key: 'playerId', width: 6 }],
    },
  ],
  defaultVariant: 'standard',
});

describe('FormVariantsEditor', () => {
  it('回读：风格卡片按数组顺序呈现 key/标题/默认标记', async () => {
    render(<FormVariantsEditor value={baseWithVariants()} onChange={jest.fn()} />);
    expect(await screen.findByText('命名展示风格')).toBeInTheDocument();
    expect(screen.getByTestId('variant-key-0')).toHaveTextContent('standard');
    expect(screen.getByTestId('variant-key-1')).toHaveTextContent('compact');
    expect(screen.getByDisplayValue('标准')).toBeInTheDocument();
    expect(screen.getByDisplayValue('紧凑')).toBeInTheDocument();
    expect(screen.getByTestId('variant-default-tag-0')).toBeInTheDocument();
    expect(screen.queryByTestId('variant-default-tag-1')).not.toBeInTheDocument();
  });

  it('新增：key 唯一顺延，空覆盖写入 variants', async () => {
    const onChange = jest.fn();
    const value = baseWithVariants();
    render(<FormVariantsEditor value={value} onChange={onChange} />);
    await screen.findByText('命名展示风格');
    fireEvent.click(screen.getByTestId('variant-add'));
    expect(onChange).toHaveBeenLastCalledWith({
      ...value,
      variants: [...(value.variants || []), { key: 'variant-3', fields: [] }],
    });
  });

  it('改名为默认风格：variants 内 key 更新且 defaultVariant 同步', async () => {
    const onChange = jest.fn();
    const value = baseWithVariants();
    render(<FormVariantsEditor value={value} onChange={onChange} />);
    await screen.findByText('命名展示风格');
    fireEvent.change(screen.getByTestId('variant-key-input-0'), {
      target: { value: 'classic' },
    });
    expect(onChange).toHaveBeenLastCalledWith({
      ...value,
      variants: [{ ...(value.variants || [])[0], key: 'classic' }, (value.variants || [])[1]],
      defaultVariant: 'classic',
    });
  });

  it('改名非默认风格：defaultVariant 不动', async () => {
    const onChange = jest.fn();
    const value = baseWithVariants();
    render(<FormVariantsEditor value={value} onChange={onChange} />);
    await screen.findByText('命名展示风格');
    fireEvent.change(screen.getByTestId('variant-key-input-1'), {
      target: { value: 'dense' },
    });
    expect(onChange).toHaveBeenLastCalledWith({
      ...value,
      variants: [(value.variants || [])[0], { ...(value.variants || [])[1], key: 'dense' }],
      defaultVariant: 'standard',
    });
  });

  it('删除默认风格：defaultVariant 清空；删除非默认：保留', async () => {
    const onChange = jest.fn();
    const value = baseWithVariants();
    render(<FormVariantsEditor value={value} onChange={onChange} />);
    await screen.findByText('命名展示风格');
    fireEvent.click(screen.getByTestId('variant-delete-1'));
    expect(onChange).toHaveBeenLastCalledWith({
      ...value,
      variants: [(value.variants || [])[0]],
      defaultVariant: 'standard',
    });
    fireEvent.click(screen.getByTestId('variant-delete-0'));
    expect(onChange).toHaveBeenLastCalledWith({
      ...value,
      variants: [(value.variants || [])[1]],
      defaultVariant: undefined,
    });
  });

  it('设为默认/取消默认：defaultVariant 翻转', async () => {
    const onChange = jest.fn();
    const value = baseWithVariants();
    render(<FormVariantsEditor value={value} onChange={onChange} />);
    await screen.findByText('命名展示风格');
    fireEvent.click(screen.getByTestId('variant-set-default-1'));
    expect(onChange).toHaveBeenLastCalledWith({ ...value, defaultVariant: 'compact' });
    fireEvent.click(screen.getByTestId('variant-set-default-0'));
    expect(onChange).toHaveBeenLastCalledWith({ ...value, defaultVariant: undefined });
  });

  it('列宽覆盖：写入变体字段；清空还原整条移除', async () => {
    // 受控回环 harness：onChange 回写 state（模拟 PageEditor 持有 spec），
    // 才能让「还原」按钮（依赖覆盖条目存在）出现在重渲染后的树里
    const calls: FormPresentationSpec[] = [];
    const Harness = () => {
      const [value, setValue] = React.useState<FormPresentationSpec>(baseWithVariants());
      return (
        <FormVariantsEditor
          value={value}
          onChange={(next) => {
            calls.push(next);
            setValue(next);
          }}
        />
      );
    };
    render(<Harness />);
    await screen.findByText('命名展示风格');
    // reason 无覆盖 → 输入列宽 4 后 compact.fields 变两条
    fireEvent.change(screen.getByTestId('variant-field-width-1-reason'), {
      target: { value: '4' },
    });
    expect(calls[0]?.variants?.[1]?.fields).toEqual([
      { key: 'playerId', width: 6 },
      { key: 'reason', width: 4 },
    ]);
    // 还原 reason：条目移除、playerId 覆盖保留
    fireEvent.click(screen.getByTestId('variant-field-clear-1-reason'));
    expect(calls[1]?.variants?.[1]?.fields).toEqual([{ key: 'playerId', width: 6 }]);
    expect(calls[1]?.variants?.[0]?.fields).toEqual([{ key: 'playerId', width: 12 }]);
  });

  it('控件覆盖：选 TextArea 写入 widget，未声明属性不被触碰', async () => {
    const onChange = jest.fn();
    const value = baseWithVariants();
    render(<FormVariantsEditor value={value} onChange={onChange} />);
    await screen.findByText('命名展示风格');
    await pickOption(screen.getByTestId('variant-field-widget-0-reason'), '多行文本');
    const [lastCall] = onChange.mock.calls.slice(-1);
    const standard = (lastCall[0] as FormPresentationSpec).variants?.[0];
    expect(standard?.fields).toEqual([
      { key: 'playerId', width: 12 },
      { key: 'reason', widget: 'TextArea' },
    ]);
  });

  it('预览：SchemaFormRenderer 收到合并呈现（布局/列宽/控件），基础不变', async () => {
    const onChange = jest.fn();
    const value = baseWithVariants();
    render(<FormVariantsEditor value={value} onChange={onChange} />);
    await screen.findByText('命名展示风格');
    expect(screen.queryByTestId('schema-renderer-stub')).not.toBeInTheDocument();
    fireEvent.click(screen.getByTestId('variant-preview-1'));
    const stub = screen.getByTestId('schema-renderer-stub');
    const merged = JSON.parse(stub.getAttribute('data-spec') || '{}') as FormPresentationSpec;
    expect(merged.layout).toBe('grid');
    expect(merged.fields?.[0]).toEqual({
      key: 'playerId',
      widget: 'Input',
      label: { 'zh-CN': '玩家' },
      width: 6,
    });
    // 预览是显式风格选择：剥离 variants/defaultVariant，渲染器解析链
    // （SUI-035）不得在已合并呈现上二次解析
    expect(merged.variants).toBeUndefined();
    expect(merged.defaultVariant).toBeUndefined();
    // 合并结果不改源
    expect(onChange).not.toHaveBeenCalled();
    expect(value.variants?.[1]?.fields).toEqual([{ key: 'playerId', width: 6 }]);
    // 再点收起
    fireEvent.click(screen.getByTestId('variant-preview-1'));
    expect(screen.queryByTestId('schema-renderer-stub')).not.toBeInTheDocument();
  });

  it('无风格：空态文案，新增入口仍在', async () => {
    render(
      <FormVariantsEditor
        value={{ jsonSchema: {}, fields: [{ key: 'playerId' }] }}
        onChange={jest.fn()}
      />,
    );
    expect(await screen.findByText('未配置风格；表单按基础呈现渲染')).toBeInTheDocument();
    expect(screen.getByTestId('variant-add')).toBeInTheDocument();
  });

  it('风格与绑定边界：提示文案区分风格与 selector 操作', async () => {
    render(<FormVariantsEditor value={baseWithVariants()} onChange={jest.fn()} />);
    expect(await screen.findByText(/风格只调整展示/)).toBeInTheDocument();
    expect(screen.getByText(/从选中行取 playerId/)).toBeInTheDocument();
  });

  it('readonly：新增/设默认/删除隐藏，预览仍可用', async () => {
    render(<FormVariantsEditor value={baseWithVariants()} onChange={jest.fn()} readonly />);
    await screen.findByText('命名展示风格');
    expect(screen.queryByTestId('variant-add')).not.toBeInTheDocument();
    expect(screen.queryByTestId('variant-set-default-0')).not.toBeInTheDocument();
    expect(screen.queryByTestId('variant-delete-0')).not.toBeInTheDocument();
    fireEvent.click(screen.getByTestId('variant-preview-0'));
    expect(screen.getByTestId('schema-renderer-stub')).toBeInTheDocument();
  });
});
