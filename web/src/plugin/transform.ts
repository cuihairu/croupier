// Tiny, safe transform helpers for outputs.views
// Supports:
// - transform.expr: string JSON-path like '$.a.b.c' (CEL-lite path)
// - transform.template: object/array template where string leaves starting with '$.' are resolved against context
//   Special form: { forEach: { path: '$.items', template: { ... } } } to map arrays

import type { JSONValue } from '@/types/dashboard';

type JSONObject = Record<string, JSONValue>;

interface TransformForEach {
  path: string;
  template: TransformInput;
  where?: WhereClause;
}

interface TransformMap {
  path: string;
  template: TransformInput;
}

interface TransformPluck {
  path: string;
  value: TransformInput;
}

interface TransformAgg {
  path: string;
  value?: TransformInput;
}

interface DirectiveNode {
  forEach?: TransformForEach;
  map?: TransformMap;
  pluck?: TransformPluck;
  sum?: TransformAgg;
  avg?: TransformAgg;
  number?: TransformInput;
  toFixed?: { value?: TransformInput; digits?: number };
  msFromSec?: TransformInput;
  isoFromMs?: TransformInput;
  isoFromSec?: TransformInput;
  mul?: { value?: TransformInput; by?: number };
  div?: { value?: TransformInput; by?: number };
  add?: { a?: TransformInput; b?: TransformInput };
  sub?: { a?: TransformInput; b?: TransformInput };
}

type WhereClause = Record<string, JSONValue[] | JSONValue>;
type TransformInput = JSONValue | DirectiveNode;

export interface Transform {
  lang?: string;
  expr?: string;
  template?: TransformInput;
}

function isDirectiveNode(value: TransformInput): value is DirectiveNode {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) return false;
  const keys = Object.keys(value);
  if (keys.length !== 1) return false;
  const directiveKeys = ['forEach', 'map', 'pluck', 'sum', 'avg', 'number', 'toFixed', 'msFromSec', 'isoFromMs', 'isoFromSec', 'mul', 'div', 'add', 'sub'];
  return directiveKeys.includes(keys[0]);
}

function isObject(value: TransformInput): value is JSONObject {
  return typeof value === 'object' && value !== null && !Array.isArray(value) && !isDirectiveNode(value);
}

export function applyTransform(root: JSONValue, t: Transform | undefined): JSONValue | undefined {
  if (!t) return root;
  // expr (path extraction)
  if (typeof t.expr === 'string' && t.expr.trim()) {
    return getByPath(root, t.expr);
  }
  // template mapping
  if (t.template !== undefined) {
    return applyTemplate(root, root, t.template);
  }
  return root;
}

export function getByPath(obj: JSONValue, expr: string): JSONValue | undefined {
  if (!expr) return undefined;
  // normalize: allow '$.a.b', 'a.b', or '$'
  let p = expr.trim();
  if (p === '$' || p === '$.') return obj;
  if (!p.startsWith('$.')) p = '$.' + p;
  const parts = p
    .replace(/^\$\.?/, '')
    .split('.')
    .filter(Boolean);
  let cur: JSONValue | undefined = obj;
  for (const k of parts) {
    if (cur == null || typeof cur !== 'object') return undefined;
    // basic bracket index support like 'arr[0]'
    const m = k.match(/^(\w+)(\[(\d+)\])?$/);
    if (m) {
      const key = m[1];
      const idx = m[3] !== undefined ? parseInt(m[3], 10) : undefined;
      if (key) {
        if (!isObject(cur)) return undefined;
        cur = cur[key];
      }
      if (idx !== undefined) {
        if (!Array.isArray(cur)) return undefined;
        cur = cur[idx];
      }
    } else {
      if (!isObject(cur)) return undefined;
      cur = cur[k];
    }
  }
  return cur;
}

function applyTemplate(root: JSONValue, ctx: JSONValue, tmpl: TransformInput): JSONValue | undefined {
  if (typeof tmpl === 'string') {
    if (tmpl.startsWith('$$.')) return getByPath(root, tmpl.slice(1)); // '$$.' means root
    if (tmpl.startsWith('$.')) return getByPath(ctx, tmpl);
    return tmpl;
  }
  if (typeof tmpl === 'number' || typeof tmpl === 'boolean' || tmpl === null) {
    return tmpl;
  }
  if (Array.isArray(tmpl)) {
    const results = tmpl.map((it) => applyTemplate(root, ctx, it));
    return results.filter((r): r is JSONValue => r !== undefined) as JSONValue;
  }
  if (isDirectiveNode(tmpl)) {
    return applyDirective(root, ctx, tmpl);
  }
  if (isObject(tmpl)) {
    const out: JSONObject = {};
    for (const [k, v] of Object.entries(tmpl)) {
      const result = applyTemplate(root, ctx, v as TransformInput);
      if (result !== undefined) out[k] = result;
    }
    return out;
  }
  return tmpl;
}

function applyDirective(root: JSONValue, ctx: JSONValue, dir: DirectiveNode): JSONValue | undefined {
  // forEach mapping
  if (dir.forEach) {
    const spec = dir.forEach;
    const arr = getByPath(ctx, spec.path);
    if (!Array.isArray(arr)) return [];
    const pred = buildPredicate(root, spec.where);
    const out: JSONValue[] = [];
    for (const item of arr) {
      if (!pred || pred(item as JSONValue)) {
        const result = applyTemplate(root, item as JSONValue, spec.template);
        if (result !== undefined) out.push(result);
      }
    }
    return out;
  }
  // map alias
  if (dir.map) {
    const spec = dir.map;
    const arr = getByPath(ctx, spec.path);
    if (!Array.isArray(arr)) return [];
    return arr.map((item) => applyTemplate(root, item as JSONValue, spec.template)).filter((r): r is JSONValue => r !== undefined) as JSONValue;
  }
  // pluck values
  if (dir.pluck) {
    const spec = dir.pluck;
    const arr = getByPath(ctx, spec.path);
    if (!Array.isArray(arr)) return [];
    return arr.map((item) => resolveValue(root, item as JSONValue, spec.value)).filter((r): r is JSONValue => r !== undefined) as JSONValue;
  }
  // sum/avg over array
  if (dir.sum) {
    return aggregate(root, ctx, dir.sum, 'sum');
  }
  if (dir.avg) {
    return aggregate(root, ctx, dir.avg, 'avg');
  }
  // numeric directives
  const asNumber = (val: TransformInput | undefined): number | undefined => {
    if (val === undefined) return undefined;
    const resolved = resolveValue(root, ctx, val);
    const n = Number(resolved);
    return Number.isNaN(n) ? undefined : n;
  };
  if (dir.number !== undefined) {
    return asNumber(dir.number);
  }
  if (dir.toFixed) {
    const n = asNumber(dir.toFixed.value);
    const d = Number(dir.toFixed.digits);
    const digits = Number.isNaN(d) ? 0 : d;
    return n !== undefined ? Number(n.toFixed(digits)) : undefined;
  }
  if (dir.msFromSec !== undefined) {
    const n = asNumber(dir.msFromSec);
    return n !== undefined ? n * 1000 : undefined;
  }
  if (dir.isoFromMs !== undefined) {
    const n = asNumber(dir.isoFromMs);
    return n !== undefined ? new Date(n).toISOString() : undefined;
  }
  if (dir.isoFromSec !== undefined) {
    const n = asNumber(dir.isoFromSec);
    return n !== undefined ? new Date(n * 1000).toISOString() : undefined;
  }
  if (dir.mul) {
    const n = asNumber(dir.mul.value);
    const by = Number(dir.mul.by);
    return n !== undefined && !Number.isNaN(by) ? n * by : undefined;
  }
  if (dir.div) {
    const n = asNumber(dir.div.value);
    const by = Number(dir.div.by);
    return n !== undefined && !Number.isNaN(by) && by !== 0 ? n / by : undefined;
  }
  if (dir.add) {
    const n = asNumber(dir.add.a);
    const m = asNumber(dir.add.b);
    return (n ?? 0) + (m ?? 0);
  }
  if (dir.sub) {
    const n = asNumber(dir.sub.a);
    const m = asNumber(dir.sub.b);
    return (n ?? 0) - (m ?? 0);
  }
  return undefined;
}

// Build a simple predicate function from an object like { eq: ['$.x', 1] }
function buildPredicate(root: JSONValue, where: WhereClause | undefined): ((item: JSONValue) => boolean) | null {
  if (!where || typeof where !== 'object') return null;
  const ops = Object.keys(where);
  if (!ops.length) return null;
  const [op] = ops;
  const rawArgs = where[op];
  const args: JSONValue[] = Array.isArray(rawArgs) ? rawArgs : [rawArgs];
  const evalArg = (item: JSONValue, v: JSONValue): JSONValue | undefined => {
    if (typeof v === 'string') {
      if (v.startsWith('$$.')) return getByPath(root, v.slice(1));
      if (v.startsWith('$.')) return getByPath(item, v);
    }
    return v;
  };
  switch (op) {
    case 'eq':
      return (item) => evalArg(item, args[0]) === evalArg(item, args[1]);
    case 'ne':
      return (item) => evalArg(item, args[0]) !== evalArg(item, args[1]);
    case 'gt':
      return (item) => Number(evalArg(item, args[0])) > Number(evalArg(item, args[1]));
    case 'lt':
      return (item) => Number(evalArg(item, args[0])) < Number(evalArg(item, args[1]));
    case 'contains':
      return (item) => {
        const a = String(evalArg(item, args[0]) ?? '');
        const b = String(evalArg(item, args[1]) ?? '');
        return a.includes(b);
      };
    case 'match':
      return (item) => {
        const a = String(evalArg(item, args[0]) ?? '');
        const pattern = args[1];
        const r = pattern instanceof RegExp ? pattern : new RegExp(String(pattern ?? ''));
        return r.test(a);
      };
    default:
      return null;
  }
}

function resolveValue(root: JSONValue, ctx: JSONValue, v: TransformInput): JSONValue | undefined {
  if (typeof v === 'string') {
    if (v.startsWith('$$.')) return getByPath(root, v.slice(1));
    if (v.startsWith('$.')) return getByPath(ctx, v);
    return v;
  }
  if (typeof v === 'number' || typeof v === 'boolean' || v === null) {
    return v;
  }
  if (Array.isArray(v)) {
    return v;
  }
  if (isDirectiveNode(v)) {
    return applyDirective(root, ctx, v);
  }
  if (isObject(v)) {
    return v;
  }
  return undefined;
}

function aggregate(root: JSONValue, ctx: JSONValue, spec: TransformAgg, kind: 'sum' | 'avg'): number {
  const path = spec.path;
  const value = spec.value;
  const arr = getByPath(ctx, path);
  if (!Array.isArray(arr)) return 0;
  let total = 0;
  let count = 0;
  for (const item of arr) {
    const resolved = value ? resolveValue(root, item as JSONValue, value) : item;
    const x = Number(resolved);
    if (!Number.isNaN(x)) {
      total += x;
      count++;
    }
  }
  return kind === 'avg' ? (count ? total / count : 0) : total;
}
