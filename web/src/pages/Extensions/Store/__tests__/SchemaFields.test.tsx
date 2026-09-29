/**
 * 扩展商店 SchemaFields 渲染器单测（覆盖率巡检：Extensions 簇余量第六批，
 * SchemaFields.tsx 174 行分支 64% → 收口）。
 *
 * 锁定契约：守卫翼（schema 缺省经防御 `?.`、properties 缺省/非对象 → null
 * 不渲染）；卡片标题；六路类型分派矩阵——enum 字段（Select + enum 选项 +
 * required 规则与「请选择 label」消息文案）、boolean（true/false 二值
 * Select）、number/integer（InputNumber + integer precision=0）、array/object
 * （TextArea + placeholder '[]'/'{}' + extra 双态：description 优先否则
 * 「{type} 类型，支持 JSON 文本」兜底）、default 翼（Input）；五处 fallback
 * ——type 缺省 'string'、title 缺省回 key、description 缺省 ''、enum 非数组
 * → 穿过 enum 翼落类型分派、required 非数组 → 全部非必填、prop null →
 * 空对象形态走 default 翼；required 成员标记（ant-form-item-required）。
 *
 * mock 口径：@umijs/max 本地 mock（useIntl values 插值）；antd 真实渲染，
 * 外包一层 Form 提供 Form.Item 上下文。
 *
 * 边界（诚实）：
 * 1. default 翼 Input 的 placeholder 三元 `'0'` 支——number/integer 已在
 *    L86 提前 return，此处条件恒 false，结构上不可达，登记不造假。
 * 2. required 规则的 message 文案仅在表单提交校验时呈现——本渲染器单测
 *    不构造提交链（InstallModal 提交链已由 index.test.tsx 覆盖），断言落
 *    rules 配置驱动的必填标记。
 */
import React from 'react';
import { configure, fireEvent, render, screen, within } from '@testing-library/react';
import { Form } from 'antd';
import SchemaFields from '../SchemaFields';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

jest.mock('@umijs/max', () => ({
  useIntl: () => ({
    formatMessage: (opts: { defaultMessage?: string }, values?: Record<string, unknown>) => {
      let text = opts.defaultMessage ?? '';
      if (values) {
        for (const [k, v] of Object.entries(values)) text = text.split(`{${k}}`).join(String(v));
      }
      return text;
    },
  }),
}));

/** 取指定 label 的 Form.Item 行（antd label 渲染进 .ant-form-item） */
const rowOf = (label: string): HTMLElement => {
  const row = screen.getByText(label).closest('.ant-form-item') as HTMLElement;
  expect(row).not.toBeNull();
  return row;
};

/** 打开行内 Select 并返回可见下拉层 */
function openSelect(row: HTMLElement): HTMLElement {
  fireEvent.mouseDown(row.querySelector('.ant-select') as HTMLElement);
  const visible = Array.from(document.querySelectorAll('.ant-select-dropdown')).find(
    (d) => !d.classList.contains('ant-select-dropdown-hidden'),
  );
  expect(visible).toBeDefined();
  return visible as HTMLElement;
}

describe('SchemaFields 守卫翼', () => {
  it('schema 缺省（防御 ?.）/ properties 缺省 / properties 非对象 → 不渲染', () => {
    const { container } = render(
      <Form>
        <SchemaFields schema={undefined as unknown as Record<string, never>} />
      </Form>,
    );
    expect(container.querySelector('.ant-card')).toBeNull();
    expect(screen.queryByText('配置字段（来自 manifest.configSchema）')).not.toBeInTheDocument();

    const { container: c2 } = render(
      <Form>
        <SchemaFields schema={{} as unknown as Record<string, never>} />
      </Form>,
    );
    expect(c2.querySelector('.ant-card')).toBeNull();

    const { container: c3 } = render(
      <Form>
        <SchemaFields schema={{ properties: 'x' } as unknown as Record<string, never>} />
      </Form>,
    );
    expect(c3.querySelector('.ant-card')).toBeNull();
  });
});

describe('SchemaFields 类型分派矩阵', () => {
  const schema = {
    properties: {
      mode: { type: 'string', enum: ['fast', 'slow'], title: '模式', description: '选一个' },
      verbose: { type: 'boolean', title: '详细日志' },
      rate: { type: 'number', title: '速率' },
      shards: { type: 'integer' },
      tags: { type: 'array' },
      meta: { type: 'object', description: '对象说明' },
      note: { type: 'string', title: '备注' },
      legacy: { enum: 'not-array', type: 'string', title: '旧形态' },
      ghost: null,
    },
    required: ['mode'],
  } as unknown as Record<string, never>;

  function renderFields() {
    return render(
      <Form>
        <SchemaFields schema={schema} />
      </Form>,
    );
  }

  it('卡片标题 + required 成员标记（mode 必填、其余非必填）', () => {
    renderFields();
    expect(screen.getByText('配置字段（来自 manifest.configSchema）')).toBeInTheDocument();
    expect(rowOf('模式').querySelector('.ant-form-item-required')).not.toBeNull();
    expect(rowOf('速率').querySelector('.ant-form-item-required')).toBeNull();
  });

  it('enum 翼：Select 下拉枚举选项 + extra=description', () => {
    renderFields();
    const row = rowOf('模式');
    expect(within(row).getByText('选一个')).toBeInTheDocument();
    const dropdown = openSelect(row);
    expect(
      within(dropdown).getAllByText('fast', { selector: '.ant-select-item-option-content' }),
    ).toHaveLength(1);
    expect(
      within(dropdown).getAllByText('slow', { selector: '.ant-select-item-option-content' }),
    ).toHaveLength(1);
  });

  it('boolean 翼：true/false 二值 Select', () => {
    renderFields();
    const dropdown = openSelect(rowOf('详细日志'));
    expect(
      within(dropdown).getAllByText('true', { selector: '.ant-select-item-option-content' }),
    ).toHaveLength(1);
    expect(
      within(dropdown).getAllByText('false', { selector: '.ant-select-item-option-content' }),
    ).toHaveLength(1);
  });

  it('number/integer 翼：InputNumber（spinbutton）', () => {
    renderFields();
    expect(rowOf('速率').querySelector('input[class*="ant-input-number-input"]')).not.toBeNull();
    // integer 无 title → label 回退 key
    expect(rowOf('shards').querySelector('input[class*="ant-input-number-input"]')).not.toBeNull();
  });

  it('array/object 翼：TextArea + placeholder「[]」「{}」+ extra 双态', () => {
    renderFields();
    const tags = rowOf('tags').querySelector('textarea.ant-input') as HTMLTextAreaElement;
    expect(tags).not.toBeNull();
    expect(tags).toHaveAttribute('placeholder', '[]');
    // 无 description → 兜底提示（help || hint 右翼）
    expect(rowOf('tags').textContent).toContain('array 类型，支持 JSON 文本');

    const meta = rowOf('meta').querySelector('textarea.ant-input') as HTMLTextAreaElement;
    expect(meta).not.toBeNull();
    expect(meta).toHaveAttribute('placeholder', '{}');
    // 有 description → 优先（左翼）
    expect(rowOf('meta').textContent).toContain('对象说明');
    expect(rowOf('meta').textContent).not.toContain('object 类型，支持 JSON 文本');
  });

  it('default 翼（string/无 type/enum 非数组/pro null 四形态均落 Input）', () => {
    renderFields();
    for (const label of ['备注', '旧形态', 'ghost']) {
      const input = rowOf(label).querySelector('input.ant-input') as HTMLInputElement;
      expect(input).not.toBeNull();
      expect(input).toHaveAttribute('placeholder', '');
    }
  });
});

describe('SchemaFields required 非数组翼', () => {
  it('schema.required 为字符串 → 全部字段非必填', () => {
    const schema = {
      properties: { a: { type: 'string', title: '甲' } },
      required: 'x',
    } as unknown as Record<string, never>;
    render(
      <Form>
        <SchemaFields schema={schema} />
      </Form>,
    );
    expect(rowOf('甲').querySelector('.ant-form-item-required')).toBeNull();
  });
});
