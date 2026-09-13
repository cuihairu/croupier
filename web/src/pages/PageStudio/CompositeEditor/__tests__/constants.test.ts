/** constants（常量共享模型）覆盖：schemaToFields（空/坏 JSON/无 properties/
 * title 缺省回退/enum·enumNames 映射与 label 兜底）、fieldsToSchemaJson
 * （空 key 跳过/重复 key 首个生效/trim/无 options 无 enum/label 与 value
 * 全等无 enumNames·不等带 enumNames）、rowsToFields（长表聚合按名去空保
 * 序·label 第三列/宽表逐行·无名跳过·空值过滤·全空跳过）、jsonToFields
 * 四形（对象元素数组·纯值数组 field1·名→选项对象·其他空）、
 * staticFormNodeFromFields（span 缺省与显式/schema 回填）。 */
import {
  fieldsToSchemaJson,
  jsonToFields,
  rowsToFields,
  schemaToFields,
  staticFormNodeFromFields,
  type ConstantField,
} from '../constants';

describe('schemaToFields', () => {
  it('空/空白/坏 JSON/无 properties：返回空', () => {
    expect(schemaToFields(undefined)).toEqual([]);
    expect(schemaToFields('')).toEqual([]);
    expect(schemaToFields('   ')).toEqual([]);
    expect(schemaToFields('not-json')).toEqual([]);
    expect(schemaToFields('{"type":"object"}')).toEqual([]);
  });

  it('完整解析：enum 映射、enumNames 对齐、title 缺省回退 key', () => {
    const schema = JSON.stringify({
      type: 'object',
      properties: {
        level: { type: 'string', title: '等级', enum: ['1', '2'], enumNames: ['初级', '高级'] },
        zone: { type: 'string', enum: ['a'] },
      },
    });
    expect(schemaToFields(schema)).toEqual([
      {
        key: 'level',
        title: '等级',
        options: [
          { value: '1', label: '初级' },
          { value: '2', label: '高级' },
        ],
      },
      // 无 title → key；enumNames 缺位 → label 兜底 value
      { key: 'zone', title: 'zone', options: [{ value: 'a', label: 'a' }] },
    ]);
  });

  it('enum/enumNames 非数组：options 为空', () => {
    const schema = JSON.stringify({
      properties: { bad: { type: 'string', enum: 'oops', enumNames: 42 } },
    });
    expect(schemaToFields(schema)).toEqual([{ key: 'bad', title: 'bad', options: [] }]);
  });
});

describe('fieldsToSchemaJson', () => {
  it('空 key 与重复 key 首个生效；key 做 trim', () => {
    const json = fieldsToSchemaJson([
      { key: ' a ', title: 'A', options: [] },
      { key: '', title: 'B', options: [] },
      { key: 'a', title: 'A2', options: [] },
    ]);
    const parsed = JSON.parse(json) as { properties: Record<string, { title: string }> };
    expect(Object.keys(parsed.properties)).toEqual(['a']);
    expect(parsed.properties.a.title).toBe('A');
  });

  it('无 options 不带 enum；label=value 全等不带 enumNames；不等才带', () => {
    const plain = JSON.parse(fieldsToSchemaJson([{ key: 'k', title: 'K', options: [] }])) as {
      properties: Record<string, Record<string, unknown>>;
    };
    expect(plain.properties.k.enum).toBeUndefined();
    expect(plain.properties.k.enumNames).toBeUndefined();

    const same = JSON.parse(
      fieldsToSchemaJson([{ key: 'k', title: 'K', options: [{ value: 'v', label: 'v' }] }]),
    ) as { properties: Record<string, Record<string, unknown>> };
    expect(same.properties.k.enum).toEqual(['v']);
    expect(same.properties.k.enumNames).toBeUndefined();

    const diff = JSON.parse(
      fieldsToSchemaJson([
        { key: 'k', title: 'K', options: [{ value: 'v', label: '显名' }, { value: 'w' }] },
      ]),
    ) as { properties: Record<string, Record<string, unknown>> };
    expect(diff.properties.k.enum).toEqual(['v', 'w']);
    // label 缺位兜底 value
    expect(diff.properties.k.enumNames).toEqual(['显名', 'w']);
  });
});

describe('rowsToFields', () => {
  it('长表：按名称聚合保序；空名/空值跳过；第三列 label 空则省略', () => {
    const fields = rowsToFields(
      [
        ['等级', '1', '初级'],
        ['等级', '2', ''],
        ['  ', 'x'],
        ['等级', '  '],
        ['区服', 's1'],
        [undefined, undefined],
      ],
      'long',
    );
    expect(fields).toEqual([
      {
        key: '等级',
        title: '等级',
        options: [
          { value: '1', label: '初级' },
          { value: '2', label: undefined },
        ],
      },
      { key: '区服', title: '区服', options: [{ value: 's1', label: undefined }] },
    ]);
  });

  it('宽表：每行一个常量；无名跳过；空值过滤；全空跳过', () => {
    const fields = rowsToFields(
      [
        ['等级', '1', ' ', '2'],
        ['', 'x'],
        ['空', undefined, null],
        [undefined, 'y'],
      ],
      'wide',
    );
    expect(fields).toEqual([
      {
        key: '等级',
        title: '等级',
        options: [
          { value: '1', label: undefined },
          { value: '2', label: undefined },
        ],
      },
    ]);
  });
});

describe('jsonToFields', () => {
  it('数组·对象元素：name/options 取 value·label（纯值元素无 label 键）；空名或空 options 过滤', () => {
    expect(
      jsonToFields([
        { name: '等级', options: [{ value: '1', label: '初级' }, '2'] },
        { name: '', options: ['x'] },
        { name: '空', options: [] },
      ]),
    ).toEqual([
      {
        key: '等级',
        title: '等级',
        options: [{ value: '1', label: '初级' }, { value: '2' }],
      },
    ]);
  });

  it('数组·纯值：收拢为 field1；对象元素 value 缺省兜底空 label', () => {
    expect(jsonToFields(['a', 1])).toEqual([
      { key: 'field1', title: 'field1', options: [{ value: 'a' }, { value: '1' }] },
    ]);
    // 首元素是对象 → 走对象元素分支：name 空 → 过滤为空（不收拢 field1）
    expect(jsonToFields([{ value: 'v' }])).toEqual([]);
    expect(jsonToFields([{ name: 'f', options: [{ value: 'v' }] }])).toEqual([
      { key: 'f', title: 'f', options: [{ value: 'v', label: '' }] },
    ]);
    // 仅有 label 无 value：value 兜底空串
    expect(jsonToFields([{ name: 'f', options: [{ label: 'x' }] }])).toEqual([
      { key: 'f', title: 'f', options: [{ value: '', label: 'x' }] },
    ]);
  });

  it('对象 {名:[选项]}：纯值与对象元素两形；空数组过滤；无 value 兜底空串', () => {
    expect(
      jsonToFields({
        等级: ['1', { value: '2', label: '高级' }, { label: '仅标签' }, { value: '3' }, null],
        空: [],
        非数组: 'x',
      }),
    ).toEqual([
      {
        key: '等级',
        title: '等级',
        options: [
          { value: '1' },
          { value: '2', label: '高级' },
          { value: '', label: '仅标签' },
          // 无 label 键 → label 兜底空串
          { value: '3', label: '' },
          // null 的 typeof 是 object 但按纯值收拢
          { value: 'null' },
        ],
      },
    ]);
  });

  it('原始值/null：返回空', () => {
    expect(jsonToFields('str')).toEqual([]);
    expect(jsonToFields(null)).toEqual([]);
    expect(jsonToFields(42)).toEqual([]);
  });
});

describe('staticFormNodeFromFields', () => {
  const fields: ConstantField[] = [
    { key: 'level', title: '等级', options: [{ value: '1', label: '初级' }] },
  ];

  it('构造 staticForm 节点：span 缺省 24、schema 回填', () => {
    const node = staticFormNodeFromFields(fields, '筛选条件');
    expect(node.type).toBe('staticForm');
    expect(node.id).toMatch(/^staticForm-/);
    expect(node.props).toMatchObject({ title: '筛选条件', span: 24 });
    const schema = JSON.parse(String(node.props.staticSchema)) as {
      properties: Record<string, Record<string, unknown>>;
    };
    expect(schema.properties.level.enum).toEqual(['1']);
  });

  it('span 显式传入', () => {
    expect(staticFormNodeFromFields(fields, 't', 12).props.span).toBe(12);
  });
});
