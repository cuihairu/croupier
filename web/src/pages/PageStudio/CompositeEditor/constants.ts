import type { PageNode } from './model';

/**
 * 常量（导入）共享模型与解析：常量表单的选项来自游戏配置表，
 * 导入一次存为组件模板，之后在组合页组件库中拖选即用。
 */

export type FieldOption = { value: string; label?: string };

export type ConstantField = {
  /** 变量名（下游 refreshOn/参数引用）；实例可改。 */
  key: string;
  /** 显示名（下拉标签），= 导入配置里的常量名。 */
  title: string;
  options: FieldOption[];
};

export type ImportMode = 'long' | 'wide';

/** schema JSON → 常量字段列表（解析失败返回空）。 */
export function schemaToFields(value: string | undefined): ConstantField[] {
  if (!value || !value.trim()) return [];
  try {
    const obj = JSON.parse(value) as { properties?: Record<string, Record<string, unknown>> };
    const props = obj.properties ?? {};
    return Object.entries(props).map(([key, p]) => {
      const enumArr = Array.isArray(p.enum) ? (p.enum as unknown[]).map(String) : [];
      const namesArr = Array.isArray(p.enumNames) ? (p.enumNames as unknown[]).map(String) : [];
      return {
        key,
        title: typeof p.title === 'string' && p.title ? p.title : key,
        options: enumArr.map((v, i) => ({ value: v, label: namesArr[i] ?? v })),
      };
    });
  } catch {
    return [];
  }
}

/** 常量字段列表 → schema JSON（编辑器双向同步用）。 */
export function fieldsToSchemaJson(fields: ConstantField[]): string {
  const properties = Object.fromEntries(
    fields.map((f) => [
      f.key,
      {
        type: 'string',
        title: f.title,
        ...(f.options.length > 0
          ? {
              enum: f.options.map((o) => o.value),
              ...(f.options.some((o) => o.label && o.label !== o.value)
                ? { enumNames: f.options.map((o) => o.label ?? o.value) }
                : {}),
            }
          : {}),
      },
    ]),
  );
  return JSON.stringify({ type: 'object', properties }, null, 2);
}

/** 行 → 常量定义：长表按名称聚合，宽表每行一个常量。 */
export function rowsToFields(rows: unknown[][], mode: ImportMode): ConstantField[] {
  const out: ConstantField[] = [];
  if (mode === 'long') {
    const byName = new Map<string, FieldOption[]>();
    const order: string[] = [];
    for (const row of rows) {
      const name = String(row[0] ?? '').trim();
      const value = String(row[1] ?? '').trim();
      if (!name || !value) continue;
      const label =
        row[2] !== undefined && String(row[2]).trim() !== '' ? String(row[2]).trim() : undefined;
      if (!byName.has(name)) {
        byName.set(name, []);
        order.push(name);
      }
      byName.get(name)!.push({ value, label });
    }
    for (const name of order) {
      out.push({ key: name, title: name, options: byName.get(name)! });
    }
    return out;
  }
  for (const row of rows) {
    const name = String(row[0] ?? '').trim();
    if (!name) continue;
    const options = row
      .slice(1)
      .map((v) => String(v ?? '').trim())
      .filter(Boolean)
      .map((v) => ({ value: v }));
    if (options.length === 0) continue;
    out.push({ key: name, title: name, options });
  }
  return out;
}

/** JSON → 常量字段列表（对象 {名:[选项]} / 数组 [{name,options}] / 纯值数组）。 */
export function jsonToFields(parsed: unknown): ConstantField[] {
  if (Array.isArray(parsed)) {
    const first = parsed[0];
    if (first !== null && typeof first === 'object') {
      return (parsed as Array<Record<string, unknown>>)
        .map((item) => {
          const name = String(item.name ?? '').trim();
          const options = Array.isArray(item.options)
            ? (item.options as unknown[]).map((v) =>
                typeof v === 'object' && v !== null
                  ? {
                      value: String((v as Record<string, unknown>).value ?? ''),
                      label: String((v as Record<string, unknown>).label ?? ''),
                    }
                  : { value: String(v) },
              )
            : [];
          return { key: name, title: name, options };
        })
        .filter((f) => f.key && f.options.length > 0);
    }
    return [
      {
        key: 'field1',
        title: 'field1',
        options: (parsed as unknown[]).map((v) => ({ value: String(v) })),
      },
    ];
  }
  if (parsed && typeof parsed === 'object') {
    return Object.entries(parsed as Record<string, unknown>)
      .filter(([, v]) => Array.isArray(v))
      .map(([name, opts]) => ({
        key: name,
        title: name,
        options: (opts as unknown[]).map((v) =>
          typeof v === 'object' && v !== null
            ? {
                value: String((v as Record<string, unknown>).value ?? ''),
                label: String((v as Record<string, unknown>).label ?? ''),
              }
            : { value: String(v) },
        ),
      }))
      .filter((f) => f.options.length > 0);
  }
  return [];
}

/** 由常量字段构造 staticForm 节点（导入存模板用）。 */
export function staticFormNodeFromFields(
  fields: ConstantField[],
  title: string,
  span = 24,
): PageNode {
  return {
    id: `staticForm-${Date.now().toString(36)}`,
    type: 'staticForm',
    props: {
      title,
      span,
      staticSchema: fieldsToSchemaJson(fields),
    },
  };
}
