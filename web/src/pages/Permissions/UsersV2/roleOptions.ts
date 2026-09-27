/**
 * 用户表单角色下拉的选项构造与过滤（OPEN-ISSUES #19）。
 *
 * 后端 roles 表带简短中文 description（roles.json 引导落库），此前下拉只渲染
 * 英文角色码（全英文选项不可理解）。这里把描述拆到下拉行的副行展示：
 * 选中 tag 仍保持角色名不变（避免 tag 过长），搜索同时命中名称与描述。
 */

/** 角色下拉选项（label=角色名用于选中 tag；description 走 optionRender 副行）。 */
export interface RoleSelectOption {
  value: string;
  label: string;
  description: string;
}

export function buildRoleOptions(
  roles: ReadonlyArray<{ name: string; description?: string }>,
): RoleSelectOption[] {
  return roles.map((r) => ({
    value: r.name,
    label: r.name,
    description: (r.description ?? '').trim(),
  }));
}

/** 下拉搜索过滤：角色名或描述任一命中即保留（不区分大小写）。 */
export function roleOptionFilter(input: string, option?: unknown): boolean {
  const q = input.trim().toLowerCase();
  if (!q) return true;
  // antd BaseOptionType 为索引签名 any 形状；按已知键收窄
  const o = (option ?? {}) as { value?: unknown; description?: unknown };
  const value = String(o.value ?? '').toLowerCase();
  const description = String(o.description ?? '').toLowerCase();
  return value.includes(q) || description.includes(q);
}
