package io.github.cuihairu.croupier.sdk;

import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertNull;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

/**
 * F：x-ui 呈现 hints 便捷层测试。
 */
public class FieldHintsTest {

    @Test
    public void emptySchemaCreatesObjectSkeleton() {
        FunctionDescriptor descriptor = FieldHints.setFieldWidget(
                new FunctionDescriptor("player.ban", "1.0.0"), "id", "Select");
        assertNotNull(descriptor.getInputSchema());
        assertTrue(descriptor.getInputSchema().contains("\"type\":\"object\""));
        assertTrue(descriptor.getInputSchema().contains("\"x-widget\":\"Select\""));
    }

    @Test
    public void preservesExistingAttributesAndOverrides() {
        FunctionDescriptor descriptor = new FunctionDescriptor("player.ban", "1.0.0");
        descriptor.setInputSchema(
                "{\"type\":\"object\",\"properties\":{\"id\":{\"type\":\"string\",\"title\":\"玩家 ID\",\"x-widget\":\"Input\"}}}");
        FunctionDescriptor updated = FieldHints.setFieldWidget(descriptor, "id", "TreeSelect");
        // 不可变风格：原描述符不变
        assertTrue(descriptor.getInputSchema().contains("\"x-widget\":\"Input\""));
        assertTrue(updated.getInputSchema().contains("\"x-widget\":\"TreeSelect\""));
        assertTrue(updated.getInputSchema().contains("玩家 ID"));
    }

    @Test
    public void optionsSourceObject() {
        FunctionDescriptor descriptor = FieldHints.setFieldHint(
                new FunctionDescriptor("player.ban", "1.0.0"), "id", "x-options-source",
                java.util.Map.of("functionId", "player.list",
                        "labelPath", "/items/*/name",
                        "valuePath", "/items/*/id"));
        assertTrue(descriptor.getInputSchema().contains("player.list"));
        assertTrue(descriptor.getInputSchema().contains("/items/*/name"));
    }

    @Test
    public void xUnderscoreNormalizedToXDash() {
        FunctionDescriptor descriptor = FieldHints.setFieldHint(
                new FunctionDescriptor("f", "1.0.0"), "a", "x_widget", "Input");
        assertTrue(descriptor.getInputSchema().contains("x-widget"));
        assertFalse(descriptor.getInputSchema().contains("x_widget"));
    }

    @Test
    public void invalidHintRejected() {
        assertThrows(IllegalArgumentException.class,
                () -> FieldHints.setFieldHint(new FunctionDescriptor("f", "1.0.0"), "a", "widget", "Input"));
    }

    @Test
    public void emptyFieldRejected() {
        assertThrows(IllegalArgumentException.class,
                () -> FieldHints.setFieldHint(new FunctionDescriptor("f", "1.0.0"), " ", "x-widget", "Input"));
    }

    @Test
    public void emptyWidgetRejected() {
        assertThrows(IllegalArgumentException.class,
                () -> FieldHints.setFieldWidget(new FunctionDescriptor("f", "1.0.0"), "a", " "));
    }
}
