package io.github.cuihairu.croupier.sdk;

import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.*;

/**
 * FieldHints 参数守卫与 schema 解析异常补测。
 */
@DisplayName("FieldHints guard branches")
class FieldHintsEdgeTest {

    private FunctionDescriptor descriptor() {
        FunctionDescriptor descriptor = new FunctionDescriptor("demo.fn", "1.0.0");
        descriptor.setInputSchema("{\"type\":\"object\",\"properties\":{\"name\":{\"type\":\"string\"}}}");
        return descriptor;
    }

    @Test
    @DisplayName("descriptor/field/hint 守卫全部抛 IllegalArgumentException")
    void guardsThrow() {
        assertThrows(IllegalArgumentException.class,
            () -> FieldHints.setFieldHint(null, "name", "x-widget", "text"));
        assertThrows(IllegalArgumentException.class,
            () -> FieldHints.setFieldHint(descriptor(), "  ", "x-widget", "text"));
        assertThrows(IllegalArgumentException.class,
            () -> FieldHints.setFieldHint(descriptor(), null, "x-widget", "text"));
        // hint 为 null / 过短 / 前缀非法
        assertThrows(IllegalArgumentException.class,
            () -> FieldHints.setFieldHint(descriptor(), "name", null, "text"));
        assertThrows(IllegalArgumentException.class,
            () -> FieldHints.setFieldHint(descriptor(), "name", "x-", "text"));
        assertThrows(IllegalArgumentException.class,
            () -> FieldHints.setFieldHint(descriptor(), "name", "y-widget", "text"));
        // setFieldWidget 空白 widget
        assertThrows(IllegalArgumentException.class,
            () -> FieldHints.setFieldWidget(descriptor(), "name", " "));
        assertThrows(IllegalArgumentException.class,
            () -> FieldHints.setFieldWidget(descriptor(), "name", null));
    }

    @Test
    @DisplayName("input schema 非 JSON object 时抛 IllegalArgumentException")
    void nonObjectSchemaRejected() {
        FunctionDescriptor arraySchema = new FunctionDescriptor("demo.fn2", "1.0.0");
        arraySchema.setInputSchema("[1,2]");
        assertThrows(IllegalArgumentException.class,
            () -> FieldHints.setFieldHint(arraySchema, "name", "x-widget", "text"));

        FunctionDescriptor scalarSchema = new FunctionDescriptor("demo.fn3", "1.0.0");
        scalarSchema.setInputSchema("42");
        assertThrows(IllegalArgumentException.class,
            () -> FieldHints.setFieldHint(scalarSchema, "name", "x-widget", "text"));
    }

    @Test
    @DisplayName("x_ 下划线 hint 键归一为 x-，未知字段自动并入 schema")
    void underscoreHintKeyNormalized() {
        FunctionDescriptor updated = FieldHints.setFieldHint(
            descriptor(), "name", "x_widget", "text");
        assertTrue(updated.getInputSchema().contains("x-widget"));
        assertTrue(updated.getInputSchema().contains("text"));

        FunctionDescriptor newField = FieldHints.setFieldHint(
            descriptor(), "level", "x-enum", java.util.List.of("a", "b"));
        assertTrue(newField.getInputSchema().contains("x-enum"));
        assertTrue(newField.getInputSchema().contains("level"));

        FunctionDescriptor widget = FieldHints.setFieldWidget(descriptor(), "name", "slider");
        assertTrue(widget.getInputSchema().contains("x-widget"));
        assertTrue(widget.getInputSchema().contains("slider"));
    }
}
