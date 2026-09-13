/** 内置画布组件 Preview 渲染与 shared schema 片段覆盖。
 *
 * 覆盖路径：StaticForm（JSON 无效/空字段/枚举下拉/布尔开关/普通输入/对象
 * 形态 schema/无 title 字段）、Modal（无子提示/标题缺省/子节点渲染）、
 * Container（标题有无/子节点与空提示/publishAs）、FnFields（无输出 schema/
 * 字段卡渲染/scaffold 标题回退）、FnForm（无参数/必填星标/超 8 参数截断/
 * 弹窗·行内文案）、FnTable（columns 覆盖/scheme 兜底/空态）、Button（三样式/
 * 文案缺省）、Text（h2/h3/p 三级/内容缺省）、builtin 辅助与 shared 三态。 */
import { render, screen } from '@testing-library/react';
import { App } from 'antd';
import { resetRegistryForTest, getComponent, allComponents } from '../../registry';
import { registerBuiltinComponents, viewTypeToComponent, TextPreviewInput } from '../builtin';
import { commonFnSchema, spanSchema, visibleWhenSchema, cascadePolicySchema } from '../shared';
import type { PageNode } from '../../model';
import type { FunctionDescriptor } from '@/services/api/functions';

beforeAll(() => {
  resetRegistryForTest();
  registerBuiltinComponents();
});

const PreviewOf = (type: PageNode['type']) => {
  const def = getComponent(type);
  if (!def) throw new Error(`missing ${type}`);
  return def.Preview;
};

const renderPreview = (n: PageNode, fn?: FunctionDescriptor) => {
  const Comp = PreviewOf(n.type);
  return render(
    <App>
      <Comp node={n} fn={fn} />
    </App>,
  );
};

/** 指定类型组件的 rerender 片段 */
const fragmentOf = (type: PageNode['type'], n: PageNode, fn?: FunctionDescriptor) => {
  const Comp = PreviewOf(type);
  return (
    <App>
      <Comp node={n} fn={fn} />
    </App>
  );
};

const node = (type: PageNode['type'], props: Record<string, unknown>): PageNode => ({
  id: `${type}-1`,
  type,
  props,
});

const listFn: FunctionDescriptor = {
  id: 'inventory.list',
  operation: 'list',
  resource: 'inventory',
  inputSchema: {
    type: 'object',
    required: ['playerId'],
    properties: { playerId: { type: 'string' }, limit: { type: 'integer' } },
  },
  outputSchema: {
    type: 'object',
    properties: { id: { type: 'string' }, name: { type: 'string' }, quantity: { type: 'integer' } },
  },
};

const noSchemaFn: FunctionDescriptor = {
  id: 'bare.fn',
  operation: 'list',
  resource: 'bare',
  inputSchema: undefined,
  outputSchema: undefined,
};

describe('StaticForm Preview', () => {
  it('staticSchema 非法 JSON：警告提示', () => {
    renderPreview(node('staticForm', { staticSchema: '{oops' }));
    expect(screen.getByText('字段定义 JSON 无效')).toBeInTheDocument();
  });

  it('properties 为空：提示在属性面板定义', () => {
    renderPreview(node('staticForm', { staticSchema: '{"type":"object"}' }));
    expect(screen.getByText('暂无字段——在属性面板定义 JSON')).toBeInTheDocument();
  });

  it('枚举→下拉 / 布尔→开关 / 普通→输入框；对象形态 schema 直取；无 title 用 key', () => {
    // raw 为对象（非字符串）分支 + 字段 title 缺省回退 key
    renderPreview(
      node('staticForm', {
        staticSchema: {
          type: 'object',
          properties: {
            env: { type: 'string', enum: ['prod', 'dev'] },
            vip: { type: 'boolean', title: 'VIP' },
            remark: { type: 'string', title: '备注' },
            plain: { type: 'integer' },
          },
        },
      }),
    );
    const selects = screen.getAllByRole('combobox');
    expect(selects).toHaveLength(1);
    expect(screen.getByRole('option', { name: 'prod' })).toBeInTheDocument();
    expect(screen.getByRole('option', { name: 'dev' })).toBeInTheDocument();
    expect(screen.getByRole('checkbox')).toBeInTheDocument();
    expect(screen.getByText('VIP')).toBeInTheDocument();
    expect(screen.getByPlaceholderText('string')).toBeInTheDocument();
    // 无 title 字段用 key 作 label；integer 类型占位
    expect(screen.getByText('plain', { exact: false })).toBeInTheDocument();
    expect(screen.getByPlaceholderText('integer')).toBeInTheDocument();
  });

  it('scaffold 默认值与 propSchema 约定', () => {
    const def = getComponent('staticForm')!;
    expect(def.scaffold()).toMatchObject({ title: '常量表单', span: 12 });
    const schema = def.propSchema({ nodes: [], fnById: new Map(), fn: undefined, allFns: [] });
    expect(schema.properties?.staticSchema).toMatchObject({ format: 'staticSchema' });
    expect(schema.properties?.span).toMatchObject({ default: 12 });
    expect(schema.properties?.visibleWhen).toMatchObject({ format: 'condition' });
  });
});

describe('Modal Preview', () => {
  it('无子节点：提示拖入函数表单；标题缺省回退「操作」', () => {
    renderPreview(node('modal', {}));
    expect(screen.getByText('操作')).toBeInTheDocument();
    expect(screen.getByText(/拖入一个函数表单作为弹窗内容/)).toBeInTheDocument();
  });

  it('子节点渲染 fnForm Preview（参数 Tag 透出）', () => {
    renderPreview(
      {
        ...node('modal', { title: '封禁弹窗' }),
        children: [node('fnForm', { functionId: 'player.ban' })],
      },
      listFn,
    );
    expect(screen.getByText('封禁弹窗')).toBeInTheDocument();
    // fnForm 子 Preview 渲染（Modal 子渲染只传 node 不传 fn → 无参数提示）
    expect(screen.getByText('该函数无输入参数')).toBeInTheDocument();
  });

  it('propSchema：width 三档枚举默认 medium', () => {
    const schema = getComponent('modal')!.propSchema({
      nodes: [],
      fnById: new Map(),
      fn: undefined,
      allFns: [],
    });
    expect(schema.properties?.width).toMatchObject({
      enum: ['narrow', 'medium', 'wide'],
      default: 'medium',
    });
  });
});

describe('Container Preview', () => {
  it('无标题无子节点：空提示', () => {
    renderPreview(node('container', {}));
    expect(screen.getByText('容器（V1 单层）——拖入表格/字段卡/按钮/文本')).toBeInTheDocument();
  });

  it('标题渲染 + 子节点（button）渲染', () => {
    renderPreview({
      ...node('container', { title: '玩家分组' }),
      children: [node('button', { title: '同步', btnStyle: 'primary' })],
    });
    expect(screen.getByText('玩家分组')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '同步' })).toBeInTheDocument();
  });

  it('propSchema：publishAs flat/card 枚举', () => {
    const schema = getComponent('container')!.propSchema({
      nodes: [],
      fnById: new Map(),
      fn: undefined,
      allFns: [],
    });
    expect(schema.properties?.publishAs).toMatchObject({
      enum: ['flat', 'card'],
      default: 'flat',
    });
  });
});

describe('FnFields Preview', () => {
  it('函数无输出 schema：Empty 提示', () => {
    renderPreview(node('fnFields', {}), noSchemaFn);
    expect(screen.getByText('无输出 schema')).toBeInTheDocument();
  });

  it('输出字段渲染为字段卡行', () => {
    renderPreview(node('fnFields', {}), listFn);
    expect(screen.getByText('id')).toBeInTheDocument();
    expect(screen.getByText('name')).toBeInTheDocument();
    expect(screen.getByText('quantity')).toBeInTheDocument();
  });

  it('scaffold 标题回退：summary 缺失用 fn.id；无函数用「详情」', () => {
    const def = getComponent('fnFields')!;
    expect(def.scaffold(noSchemaFn)).toMatchObject({ functionId: 'bare.fn', title: 'bare.fn' });
    expect(def.scaffold(undefined)).toMatchObject({ title: '详情', span: 12, autoRun: true });
  });
});

describe('FnForm Preview', () => {
  it('函数无输入参数：提示', () => {
    renderPreview(node('fnForm', {}), noSchemaFn);
    expect(screen.getByText('该函数无输入参数')).toBeInTheDocument();
  });

  it('参数 Tag：必填星标；display 文案区分弹窗/行内', () => {
    const { rerender } = renderPreview(node('fnForm', { display: 'dialog' }), listFn);
    expect(screen.getByText('playerId *')).toBeInTheDocument();
    expect(screen.getByText('弹窗形态')).toBeInTheDocument();
    rerender(<App>{fragmentOf('fnForm', node('fnForm', { display: 'inline' }), listFn)}</App>);
    expect(screen.getByText('行内表单')).toBeInTheDocument();
  });

  it('超过 8 个参数截断展示 +N', () => {
    const manyProps: Record<string, unknown> = {};
    for (let i = 1; i <= 10; i += 1) manyProps[`p${i}`] = { type: 'string' };
    const manyFn: FunctionDescriptor = {
      ...listFn,
      inputSchema: { type: 'object', properties: manyProps },
    };
    renderPreview(node('fnForm', {}), manyFn);
    expect(screen.getByText('p1')).toBeInTheDocument();
    expect(screen.getByText('p8')).toBeInTheDocument();
    expect(screen.queryByText('p9')).not.toBeInTheDocument();
    expect(screen.getByText('+2')).toBeInTheDocument();
  });
});

describe('FnTable Preview', () => {
  it('props.columns 覆盖契约列；空态提示', () => {
    renderPreview(node('fnTable', { columns: ['id', 'quantity'] }), listFn);
    expect(screen.getByText('id')).toBeInTheDocument();
    expect(screen.getByText('quantity')).toBeInTheDocument();
    expect(screen.queryByText('name')).not.toBeInTheDocument();
    expect(screen.getByText('列来自输出 schema；试跑/预览后显示真实数据')).toBeInTheDocument();
  });

  it('columns 非数组时回退输出 schema 列', () => {
    renderPreview(node('fnTable', {}), listFn);
    expect(screen.getByText('name')).toBeInTheDocument();
  });

  it('propSchema：输出 schema 为空时无 columns 键', () => {
    const schema = getComponent('fnTable')!.propSchema({
      nodes: [],
      fnById: new Map(),
      fn: noSchemaFn,
      allFns: [],
    });
    expect(schema.properties?.columns).toBeUndefined();
    expect(schema.properties?.rowActions).toMatchObject({ format: 'rowActions' });
  });
});

describe('Button Preview', () => {
  it('primary/danger/default 样式与文案缺省', () => {
    const { rerender } = renderPreview(node('button', { title: '主按钮', btnStyle: 'primary' }));
    expect(screen.getByRole('button', { name: '主按钮' }).className).toContain('ant-btn-primary');

    rerender(
      <App>{fragmentOf('button', node('button', { btnStyle: 'danger', title: '危险' }))}</App>,
    );
    const dangerBtn = screen.getByRole('button', { name: '危险' });
    expect(dangerBtn.className).toContain('ant-btn-dangerous');

    rerender(<App>{fragmentOf('button', node('button', {}))}</App>);
    const plain = screen.getByRole('button', { name: '按钮' });
    expect(plain.className).not.toContain('ant-btn-primary');
  });
});

describe('Text Preview', () => {
  it('h2/h3/p 三级渲染与内容缺省', () => {
    const { rerender } = renderPreview(node('text', { content: '大标题', level: 'h2' }));
    expect(screen.getByRole('heading', { level: 4, name: '大标题' })).toBeInTheDocument();

    rerender(<App>{fragmentOf('text', node('text', { content: '小标题', level: 'h3' }))}</App>);
    expect(screen.getByRole('heading', { level: 5, name: '小标题' })).toBeInTheDocument();

    rerender(<App>{fragmentOf('text', node('text', {}))}</App>);
    // level 缺省 p + content 缺省 ''：无 heading，渲染 Typography 文本节点
    expect(screen.queryByRole('heading')).not.toBeInTheDocument();
    expect(document.querySelector('.ant-typography')).toBeInTheDocument();
  });
});

describe('builtin 辅助', () => {
  it('registerBuiltinComponents 注册全部九类', () => {
    expect(allComponents()).toHaveLength(9);
  });

  it('viewTypeToComponent：table/fields/其余→fnForm', () => {
    expect(viewTypeToComponent('table')).toBe('fnTable');
    expect(viewTypeToComponent('fields')).toBe('fnFields');
    expect(viewTypeToComponent('form' as never)).toBe('fnForm');
  });

  it('TextPreviewInput：只读受控输入', () => {
    render(<TextPreviewInput value="静态文案" />);
    const input = screen.getByDisplayValue('静态文案') as HTMLInputElement;
    expect(input.readOnly).toBe(true);
  });
});

describe('shared schema 片段', () => {
  it('commonFnSchema：allFns 优先 / 空 pool 回退 fn / 均空；fn 存在时 default', () => {
    const otherFn: FunctionDescriptor = {
      ...listFn,
      id: 'other.fn',
      summary: { 'zh-CN': '其他函数' },
    };
    const rich = commonFnSchema(listFn, [listFn, otherFn]);
    const fnId = rich.properties?.functionId as { enum?: string[]; enumNames?: string[] };
    expect(fnId.enum).toEqual(['inventory.list', 'other.fn']);
    expect(fnId.enumNames).toEqual(['inventory.list', 'other.fn（其他函数）']);
    expect(fnId.default).toBe('inventory.list');

    // pool 空 → 回退当前 fn
    const lone = commonFnSchema(listFn, []);
    const loneId = lone.properties?.functionId as { enum?: string[] };
    expect(loneId.enum).toEqual(['inventory.list']);

    // 两者皆无 → 空枚举，无 default
    const none = commonFnSchema(undefined, []);
    const noneId = none.properties?.functionId as { enum?: string[]; default?: string };
    expect(noneId.enum).toEqual([]);
    expect(noneId.default).toBeUndefined();
  });

  it('span/visibleWhen/cascadePolicy 片段约定', () => {
    expect(spanSchema()).toMatchObject({ type: 'integer', minimum: 4, maximum: 24, default: 24 });
    expect(visibleWhenSchema()).toMatchObject({ format: 'condition' });
    const cp = cascadePolicySchema() as unknown as {
      enum: string[];
      enumNames: string[];
    };
    expect(cp.enum).toEqual(['pause', 'clear', 'keep']);
    expect(cp.enumNames).toHaveLength(3);
  });
});

describe('各组件 propSchema 声明（属性面板渲染约定）', () => {
  it('button：btnStyle 三档默认 default', () => {
    const schema = getComponent('button')!.propSchema({
      nodes: [],
      fnById: new Map(),
      fn: undefined,
      allFns: [],
    });
    expect(schema.properties?.btnStyle).toMatchObject({
      enum: ['default', 'primary', 'danger'],
      default: 'default',
    });
    expect(schema.properties?.span).toMatchObject({ default: 24 });
  });

  it('text：level h2/h3/p 默认 p', () => {
    const schema = getComponent('text')!.propSchema({
      nodes: [],
      fnById: new Map(),
      fn: undefined,
      allFns: [],
    });
    expect(schema.properties?.level).toMatchObject({
      enum: ['h2', 'h3', 'p'],
      default: 'p',
    });
  });

  it('fnForm：display inline/dialog 默认 inline；换绑下拉含 allFns', () => {
    const schema = getComponent('fnForm')!.propSchema({
      nodes: [],
      fnById: new Map(),
      fn: listFn,
      allFns: [listFn],
    });
    expect(schema.properties?.display).toMatchObject({
      enum: ['inline', 'dialog'],
      default: 'inline',
    });
    expect(schema.properties?.functionId).toMatchObject({ enum: ['inventory.list'] });
    expect(schema.properties?.visibleWhen).toMatchObject({ format: 'condition' });
    expect(schema.properties?.cascadePolicy).toMatchObject({ enum: ['pause', 'clear', 'keep'] });
  });

  it('fnFields：autoRun 默认开 + visibleWhen/cascadePolicy', () => {
    const schema = getComponent('fnFields')!.propSchema({
      nodes: [],
      fnById: new Map(),
      fn: listFn,
      allFns: [listFn],
    });
    expect(schema.properties?.autoRun).toMatchObject({ type: 'boolean', default: true });
    expect(schema.properties?.visibleWhen).toMatchObject({ format: 'condition' });
    expect(schema.properties?.cascadePolicy).toMatchObject({ enum: ['pause', 'clear', 'keep'] });
  });

  it('fnTable：有输出 schema 时 columns 全选默认；span/autoRun', () => {
    const schema = getComponent('fnTable')!.propSchema({
      nodes: [],
      fnById: new Map(),
      fn: listFn,
      allFns: [listFn],
    });
    expect(schema.properties?.columns).toMatchObject({
      format: 'columns',
      default: ['id', 'name', 'quantity'],
    });
    expect(schema.properties?.autoRun).toMatchObject({ default: true });
  });

  it('tabs：sectionKey 可选 + span 默认 24', () => {
    const schema = getComponent('tabs')!.propSchema({
      nodes: [],
      fnById: new Map(),
      fn: undefined,
      allFns: [],
    });
    expect(schema.properties?.sectionKey).toMatchObject({ type: 'string' });
    expect(schema.properties?.span).toMatchObject({ default: 24 });
  });
});

describe('Tabs Preview 补充', () => {
  it('页签 title 为空时标签回退「页签 N」，非空用自定义', () => {
    const Comp = PreviewOf('tabs');
    render(
      <App>
        <Comp
          node={{
            id: 'tabs-x',
            type: 'tabs',
            props: {},
            children: [
              { id: 'pg1', type: 'container', props: { title: '   ', span: 24 }, children: [] },
              {
                id: 'pg2',
                type: 'container',
                props: { title: '自定义页', span: 24 },
                children: [],
              },
            ],
          }}
        />
      </App>,
    );
    expect(screen.getByRole('tab', { name: '页签 1' })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: '自定义页' })).toBeInTheDocument();
  });
});
