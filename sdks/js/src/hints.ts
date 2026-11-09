// F14 presentation-hints helper layer (contract: docs/architecture/presentation-hints.md).
// Extracted from index.ts (LAY-009); re-exported via the barrel.

import type { FunctionDescriptor } from "./types";

/**
 * F14：向函数描述符的 input schema 注入 x-ui 呈现 hints 的便捷层
 * （契约见 docs/architecture/presentation-hints.md）。
 */
export function setFieldHint(
  descriptor: FunctionDescriptor,
  field: string,
  hint: string,
  value: unknown,
): FunctionDescriptor {
  if (!field || !field.trim()) {
    throw new Error("field key is required for setFieldHint");
  }
  const normalized = normalizeHintKey(hint);
  if (!normalized) {
    throw new Error(
      `hint "${hint}" must be an x- extension key (e.g. x-widget)`,
    );
  }
  const schema = {
    type: "object",
    ...(descriptor.inputSchema ?? {}),
  } as Record<string, unknown>;
  const properties = {
    ...((schema.properties as Record<string, unknown>) ?? {}),
  };
  const property = {
    ...((properties[field] as Record<string, unknown>) ?? {}),
  };
  property[normalized] = value;
  properties[field] = property;
  schema.properties = properties;
  return { ...descriptor, inputSchema: schema };
}

export function setFieldWidget(
  descriptor: FunctionDescriptor,
  field: string,
  widget: string,
): FunctionDescriptor {
  if (!widget || !widget.trim()) {
    throw new Error("widget is required for setFieldWidget");
  }
  return setFieldHint(descriptor, field, "x-widget", widget);
}

function normalizeHintKey(hint: string): string | null {
  const trimmed = hint.trim();
  if (!trimmed) return null;
  const lower = trimmed.toLowerCase();
  if (lower.startsWith("x_")) return `x-${trimmed.slice(2)}`;
  if (lower.startsWith("x-")) return trimmed;
  return null;
}
