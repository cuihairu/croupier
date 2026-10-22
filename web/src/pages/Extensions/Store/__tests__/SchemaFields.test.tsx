/**
 * SchemaFields 单测（Extensions 簇余量真缺口收口：64.36% → 100%）。
 *
 * 背景：此前该组件仅经 Store 安装弹窗间接覆盖（enum/number/string 三路），
 * boolean/array/object 三类控件与无 properties 守卫、title 缺省回退、
 * description 空兜底、raw null 防御全部未触达。
 *
 * 锁定契约：schema 无 properties/非对象 → null；字段类型分派矩阵
 * （enum→Select、boolean→true/false Select、number/integer→InputNumber、
 * array/object→TextArea（placeholder '[]'/'{}'）、string→Input）；label
 * title 缺省回退 key；extra=description、空则 array/object 兜底
 * 「{type} 类型，支持 JSON 文本」；required 星标两态；Form initialValues
 * 经 ['config', key] 挂载的默认值回显。
 *
 * mock 口径：@umijs/max 本地 mock（对齐 Store 套件先例）；antd 真实渲染。
 *
 * 边界（诚实）：默认路 Input 的 placeholder `type === 'number' || type ===
 * 'integer' ? '0' : ''`（167 行）'0' 翼构造性不可达——number/integer 在
 * 86 行已被 InputNumber 分派拦截，到不了默认路；不造假用例。
 */
import React from 'react';
import { Form } from 'antd';
import { fireEvent, render, screen, within } from '@testing-library/react';
import SchemaFields from '../SchemaFields';

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

function renderFields(schema: Record<string, unknown>, initialValues?: object) {
  return render(
    <Form initialValues={initialValues}>
      <SchemaFields schema={schema as never} />
    </Form>,
  );
}

describe('SchemaFields 守卫与类型分派', () => {
  it('schema 无 properties / properties 非对象 → null（卡片不渲染）', () => {
    const { container, rerender } = renderFields({});
    expect(container.querySelector('.ant-card')).toBeNull();
    rerender(
      <Form>
        <SchemaFields schema={{ properties: 'corrupted' } as never} />
      </Form>,
    );
    expect(container.querySelector('.ant-card')).toBeNull();
  });

  it('全类型矩阵：enum/boolean→Select、number/integer→InputNumber、array/object→TextArea、string→Input', () => {
    renderFields({
      properties: {
        mode: { type: 'string', title: '模式', enum: ['fast', 'safe'] },
        enabled: { type: 'boolean', title: '启用' },
        retries: { type: 'number', title: '重试' },
        workers: { type: 'integer', title: '并发' },
        hosts: { type: 'array', title: '主机', description: '逗号分隔' },
        extra: { type: 'object', title: '附加' },
        note: { type: 'string', title: '备注' },
      },
      required: ['mode'],
    });

    expect(screen.getByText('配置字段（来自 manifest.configSchema）')).toBeInTheDocument();
    // 控件分派：2 Select + 2 InputNumber + 2 TextArea + 1 Input
    expect(document.querySelectorAll('.ant-select').length).toBeGreaterThanOrEqual(2);
    expect(document.querySelectorAll('.ant-input-number').length).toBe(2);
    expect(screen.getByPlaceholderText('[]')).toBeInTheDocument();
    expect(screen.getByPlaceholderText('{}')).toBeInTheDocument();
    // label 全渲染
    for (const label of ['模式', '启用', '重试', '并发', '主机', '附加', '备注']) {
      expect(screen.getByText(label)).toBeInTheDocument();
    }
    // description 透传为 extra
    expect(screen.getByText('逗号分隔')).toBeInTheDocument();
  });

  it('enum 选项渲染：下拉可见 fast/safe', async () => {
    renderFields({
      properties: { mode: { type: 'string', title: '模式', enum: ['fast', 'safe'] } },
    });
    fireEvent.mouseDown(document.querySelector('.ant-select') as HTMLElement);
    const dropdown = document.querySelector('.ant-select-dropdown') as HTMLElement;
    expect(dropdown).not.toBeNull();
    fireEvent.click(
      within(dropdown).getByText('fast', { selector: '.ant-select-item-option-content' }),
    );
    // antd6 Select 选中态渲染在 .ant-select-content（v6 DOM 变更，旧版
    // .ant-select-selection-item 不存在；input value 恒空）
    const selected = await screen.findByText('fast', { selector: '.ant-select-content' });
    expect(selected).toBeInTheDocument();
  });

  it('boolean 选项：下拉 true/false', () => {
    renderFields({ properties: { flag: { type: 'boolean', title: '开关' } } });
    fireEvent.mouseDown(document.querySelector('.ant-select') as HTMLElement);
    const dropdown = document.querySelector('.ant-select-dropdown') as HTMLElement;
    expect(within(dropdown).getByText('true')).toBeInTheDocument();
    expect(within(dropdown).getByText('false')).toBeInTheDocument();
  });

  it('title 缺省回退 key；raw null 防御走 string 默认路', () => {
    renderFields({
      properties: {
        bare_key: { type: 'string' },
        bad: null,
      },
    });
    // title 缺省 → label = key
    expect(screen.getByText('bare_key')).toBeInTheDocument();
    // raw null → (raw || {}) → type 缺省 'string' → Input 而非崩溃
    expect(screen.getByText('bad')).toBeInTheDocument();
    expect(document.querySelector('input.ant-input')).not.toBeNull();
  });

  it('array/object 无 description → extra 兜底「{type} 类型，支持 JSON 文本」', () => {
    renderFields({
      properties: {
        hosts: { type: 'array', title: '主机' },
        extra: { type: 'object', title: '附加' },
      },
    });
    expect(screen.getByText('array 类型，支持 JSON 文本')).toBeInTheDocument();
    expect(screen.getByText('object 类型，支持 JSON 文本')).toBeInTheDocument();
  });

  it('required 星标：required 字段带必填标记、可选字段无', () => {
    renderFields({
      properties: {
        a: { type: 'string', title: '必填项' },
        b: { type: 'string', title: '可选项' },
      },
      required: ['a'],
    });
    const labelA = screen.getByText('必填项').closest('label');
    const labelB = screen.getByText('可选项').closest('label');
    expect(labelA?.className).toContain('ant-form-item-required');
    expect(labelB?.className).not.toContain('ant-form-item-required');
  });

  it('默认值回显：initialValues 经 [config, key] 挂载', async () => {
    renderFields(
      {
        properties: {
          endpoint: { type: 'string', title: '端点' },
          retries: { type: 'number', title: '重试' },
        },
      },
      { config: { endpoint: 'http://localhost:9', retries: 3 } },
    );
    expect(await screen.findByDisplayValue('http://localhost:9')).toBeInTheDocument();
    expect(screen.getByDisplayValue('3')).toBeInTheDocument();
  });
});
