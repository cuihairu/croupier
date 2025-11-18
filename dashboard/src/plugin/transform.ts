// Tiny, safe transform helpers for outputs.views
// Supports:
// - transform.expr: string JSON-path like '$.a.b.c' (CEL-lite path)
// - transform.template: object/array template where string leaves starting with '$.' are resolved against context
//   Special form: { forEach: { path: '$.items', template: { ... } } } to map arrays

export type Transform = { lang?: string; expr?: string; template?: any } | undefined;

export function applyTransform(root: any, t: Transform): any {
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

export function getByPath(obj: any, expr: string): any {
  if (!expr) return undefined;
  // normalize: allow '$.a.b', 'a.b', or '$'
  let p = expr.trim();
  if (p === '$' || p === '$.') return obj;
  if (!p.startsWith('$.')) p = '$.' + p;
  const parts = p.replace(/^\$\.?/, '').split('.').filter(Boolean);
  let cur = obj;
  for (const k of parts) {
    if (cur == null) return undefined;
    // basic bracket index support like 'arr[0]'
    const m = k.match(/^(\w+)(\[(\d+)\])?$/);
    if (m) {
      const key = m[1];
      const idx = m[3] !== undefined ? parseInt(m[3], 10) : undefined;
      cur = key ? cur[key] : cur;
      if (idx !== undefined) {
        if (!Array.isArray(cur)) return undefined;
        cur = cur[idx];
      }
    } else {
      cur = (cur as any)[k];
    }
  }
  return cur;
}

function applyTemplate(root: any, ctx: any, tmpl: any): any {
  if (typeof tmpl === 'string') {
    if (tmpl.startsWith('$$.')) return getByPath(root, tmpl.slice(1)); // '$$.' means root
    if (tmpl.startsWith('$.')) return getByPath(ctx, tmpl);
    return tmpl;
  }
  if (Array.isArray(tmpl)) {
    return tmpl.map((it) => applyTemplate(root, ctx, it));
  }
  if (tmpl && typeof tmpl === 'object') {
    // forEach mapping
    if (tmpl.forEach && typeof tmpl.forEach === 'object') {
      const spec = tmpl.forEach as { path: string; template: any; where?: any };
      const arr = getByPath(ctx, spec.path) || [];
      if (!Array.isArray(arr)) return [];
      const pred = buildPredicate(root, spec.where);
      const out: any[] = [];
      for (const item of arr) {
        if (!pred || pred(item)) out.push(applyTemplate(root, item, spec.template));
      }
      return out;
    }
    // map alias
    if (tmpl.map && typeof tmpl.map === 'object') {
      const spec = tmpl.map as { path: string; template: any };
      const arr = getByPath(ctx, spec.path) || [];
      if (!Array.isArray(arr)) return [];
      return arr.map((item) => applyTemplate(root, item, spec.template));
    }
    // pluck values
    if (tmpl.pluck && typeof tmpl.pluck === 'object') {
      const spec = tmpl.pluck as { path: string; value: any };
      const arr = getByPath(ctx, spec.path) || [];
      if (!Array.isArray(arr)) return [];
      return arr.map((item) => resolveValue(root, item, spec.value));
    }
    // sum/avg over array
    if (tmpl.sum && typeof tmpl.sum === 'object') { return aggregate(root, ctx, tmpl.sum, 'sum'); }
    if (tmpl.avg && typeof tmpl.avg === 'object') { return aggregate(root, ctx, tmpl.avg, 'avg'); }
    // directive nodes: number/msFromSec/mul/div/add/sub
    const dir = tryDirective(root, ctx, tmpl);
    if (dir.__isDirective) return dir.value;
    const out: any = {};
    for (const [k, v] of Object.entries(tmpl)) {
      out[k] = applyTemplate(root, ctx, v);
    }
    return out;
  }
  return tmpl;
}

// Build a simple predicate function from an object like { eq: ['$.x', 1] }
function buildPredicate(root: any, where: any): ((item: any) => boolean) | null {
  if (!where || typeof where !== 'object') return null;
  const ops = Object.keys(where);
  if (!ops.length) return null;
  const [op] = ops;
  const args: any[] = Array.isArray(where[op]) ? where[op] : [where[op]];
  const evalArg = (item: any, v: any) => {
    if (typeof v === 'string') {
      if (v.startsWith('$$.')) return getByPath(root, v.slice(1));
      if (v.startsWith('$.')) return getByPath(item, v);
    }
    return v;
  };
  switch (op) {
    case 'eq': return (item) => evalArg(item, args[0]) === evalArg(item, args[1]);
    case 'ne': return (item) => evalArg(item, args[0]) !== evalArg(item, args[1]);
    case 'gt': return (item) => Number(evalArg(item, args[0])) > Number(evalArg(item, args[1]));
    case 'lt': return (item) => Number(evalArg(item, args[0])) < Number(evalArg(item, args[1]));
    case 'contains':
      return (item) => {
        const a = String(evalArg(item, args[0]) ?? '');
        const b = String(evalArg(item, args[1]) ?? '');
        return a.includes(b);
      };
    case 'match':
      return (item) => {
        const a = String(evalArg(item, args[0]) ?? '');
        const r = args[1] instanceof RegExp ? args[1] : new RegExp(String(args[1] ?? ''));
        return r.test(a);
      };
    default:
      return null;
  }
}

function resolveValue(root: any, ctx: any, v: any): any {
  if (typeof v === 'string') {
    if (v.startsWith('$$.')) return getByPath(root, v.slice(1));
    if (v.startsWith('$.')) return getByPath(ctx, v);
    return v;
  }
  if (v && typeof v === 'object') {
    const dir = tryDirective(root, ctx, v);
    if (dir.__isDirective) return dir.value;
  }
  return v;
}

function aggregate(root: any, ctx: any, spec: any, kind: 'sum'|'avg') {
  const path = spec?.path as string;
  const value = spec?.value;
  const arr = getByPath(ctx, path) || [];
  if (!Array.isArray(arr)) return 0;
  let total = 0, count = 0;
  for (const item of arr) {
    const x = Number(resolveValue(root, item, value));
    if (!Number.isNaN(x)) { total += x; count++; }
  }
  return kind === 'avg' ? (count ? total / count : 0) : total;
}

function tryDirective(root: any, ctx: any, obj: any): { __isDirective: boolean; value?: any } {
  const out = { __isDirective: false as boolean, value: undefined as any };
  if (!obj || typeof obj !== 'object') return out;
  const keys = Object.keys(obj);
  if (keys.length !== 1) return out;
  const k = keys[0];
  const arg = obj[k];
  const asNumber = (val: any) => {
    const n = Number(resolveValue(root, ctx, val));
    return Number.isNaN(n) ? undefined : n;
  };
  switch (k) {
    case 'number':
      out.value = asNumber(arg);
      out.__isDirective = true; return out;
    case 'toFixed': {
      const n = asNumber(arg?.value); const d = Number(arg?.digits);
      const digits = Number.isNaN(d) ? 0 : d;
      out.value = n !== undefined ? Number(n.toFixed(digits)) : undefined; out.__isDirective = true; return out;
    }
    case 'msFromSec': {
      const n = asNumber(arg); out.value = n !== undefined ? n * 1000 : undefined; out.__isDirective = true; return out;
    }
    case 'isoFromMs': {
      const n = asNumber(arg); out.value = n !== undefined ? new Date(n).toISOString() : undefined; out.__isDirective = true; return out;
    }
    case 'isoFromSec': {
      const n = asNumber(arg); out.value = n !== undefined ? new Date(n * 1000).toISOString() : undefined; out.__isDirective = true; return out;
    }
    case 'mul': {
      const n = asNumber(arg?.value); const by = Number(arg?.by);
      out.value = n !== undefined && !Number.isNaN(by) ? n * by : undefined; out.__isDirective = true; return out;
    }
    case 'div': {
      const n = asNumber(arg?.value); const by = Number(arg?.by);
      out.value = n !== undefined && !Number.isNaN(by) && by !== 0 ? n / by : undefined; out.__isDirective = true; return out;
    }
    case 'add': {
      const n = asNumber(arg?.a); const m = asNumber(arg?.b);
      out.value = (n ?? 0) + (m ?? 0); out.__isDirective = true; return out;
    }
    case 'sub': {
      const n = asNumber(arg?.a); const m = asNumber(arg?.b);
      out.value = (n ?? 0) - (m ?? 0); out.__isDirective = true; return out;
    }
    default:
      return out;
  }
}
