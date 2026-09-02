package io.github.cuihairu.croupier.sdk;

import io.github.cuihairu.croupier.sdk.invoker.Json;

/**
 * F：x-ui 呈现 hints 便捷层（契约见 docs/architecture/presentation-hints.md）。
 *
 * 向函数描述符的 input schema 合并 x-* 呈现意图，供 Dashboard 生成更友好的表单。
 */
public final class FieldHints {

    private FieldHints() {
    }

    /**
     * 向 input_schema 的 properties[field] 合并单个 x-* hint。
     *
     * @return 合并后的新描述符（不可变风格，原描述符不变）
     * @throws IllegalArgumentException field 为空或 hint 不是 x-/x_ 扩展键
     */
    public static FunctionDescriptor setFieldHint(FunctionDescriptor descriptor, String field,
                                                  String hint, Object value) {
        if (descriptor == null) {
            throw new IllegalArgumentException("descriptor is required");
        }
        if (field == null || field.trim().isEmpty()) {
            throw new IllegalArgumentException("field key is required for setFieldHint");
        }
        String normalized = normalizeHintKey(hint);
        if (normalized == null) {
            throw new IllegalArgumentException(
                    "hint \"" + hint + "\" must be an x- extension key (e.g. x-widget)");
        }

        java.util.Map<String, Object> schema = parseSchema(descriptor.getInputSchema());
        schema.putIfAbsent("type", "object");
        @SuppressWarnings("unchecked")
        java.util.Map<String, Object> properties = schema.containsKey("properties")
                && schema.get("properties") instanceof java.util.Map<?, ?> existingProperties
                        ? new java.util.HashMap<>((java.util.Map<String, Object>) existingProperties)
                        : new java.util.HashMap<>();
        schema.put("properties", properties);
        @SuppressWarnings("unchecked")
        java.util.Map<String, Object> property = properties.containsKey(field)
                && properties.get(field) instanceof java.util.Map<?, ?> existing
                        ? new java.util.HashMap<>((java.util.Map<String, Object>) existing)
                        : new java.util.HashMap<>();
        property.put(normalized, value);
        properties.put(field, property);
        schema.put("properties", properties);

        FunctionDescriptor updated = new FunctionDescriptor(descriptor);
        updated.setInputSchema(Json.stringify(schema));
        return updated;
    }

    /** 等价于 {@code setFieldHint(descriptor, field, "x-widget", widget)}。 */
    public static FunctionDescriptor setFieldWidget(FunctionDescriptor descriptor, String field,
                                                    String widget) {
        if (widget == null || widget.trim().isEmpty()) {
            throw new IllegalArgumentException("widget is required for setFieldWidget");
        }
        return setFieldHint(descriptor, field, "x-widget", widget);
    }

    private static String normalizeHintKey(String hint) {
        if (hint == null) {
            return null;
        }
        String trimmed = hint.trim();
        if (trimmed.length() < 3) {
            return null;
        }
        char first = Character.toLowerCase(trimmed.charAt(0));
        char second = trimmed.charAt(1);
        if (first != 'x' || (second != '-' && second != '_')) {
            return null;
        }
        return "x-" + trimmed.substring(2);
    }

    @SuppressWarnings("unchecked")
    private static java.util.Map<String, Object> parseSchema(String raw) {
        if (raw == null || raw.trim().isEmpty()) {
            return new java.util.HashMap<>();
        }
        Object parsed = Json.parse(raw);
        if (parsed instanceof java.util.Map<?, ?> map) {
            return new java.util.HashMap<>((java.util.Map<String, Object>) map);
        }
        throw new IllegalArgumentException("input schema must be a JSON object");
    }
}
