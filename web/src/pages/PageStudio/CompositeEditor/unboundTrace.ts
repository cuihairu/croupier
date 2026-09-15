/** T9：unbound functionId 前端溯源——复刻服务端确定性映射
 * （internal/function/openapi.DeriveFunctionID + internal/api/openapi.unboundFunctionID），
 * 两处必须逐字符对齐（锁定用例见 __tests__/bindingDrawer.test.tsx）：
 * operationId 优先；否则 path 段以 "." 拼接；再归一到 ^[a-z0-9][a-z0-9._-]*$。
 * 上传管线以此生成 unbound 契约 functionId，编辑器抽屉以此把画布组件的
 * unbound 函数回溯到 (sourceId, operationId)，供 CreateBinding 就地绑定。 */

export function unboundFunctionIdForOperation(
  operationId: string | undefined,
  path: string | undefined,
): string {
  const opId = (operationId ?? '').trim();
  let derived: string;
  if (opId !== '') {
    derived = opId;
  } else if (path !== undefined && path !== '') {
    // Go: strings.Split(strings.Trim(path, "/"), "/") —— 仅去首尾 "/"，空段保留
    derived = path
      .replace(/^\/+|\/+$/g, '')
      .split('/')
      .join('.');
  } else {
    derived = 'unknown.function';
  }
  return normalizeUnboundFunctionId(derived);
}

/** 归一：小写折叠 → 非 [a-z0-9._-] 字符替换 "-" → 去首尾 .-_ →
 * 首字符非小写字母/数字加 "fn-" 前缀；空串原样返回（服务端跳过建契约）。 */
function normalizeUnboundFunctionId(derived: string): string {
  const lowered = derived.trim().toLowerCase();
  let out = '';
  for (const ch of lowered) {
    out += /[a-z0-9._-]/.test(ch) ? ch : '-';
  }
  out = out.replace(/^[._-]+|[._-]+$/g, '');
  if (out === '') return '';
  return /^[a-z0-9]/.test(out) ? out : `fn-${out}`;
}
