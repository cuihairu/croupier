package io.github.cuihairu.croupier.sdk;

import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.lang.reflect.Method;
import java.util.List;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.*;

/**
 * OpenAPIImporter jsonContentSchema / extractExtension / ImportOptions 边界补测。
 */
@DisplayName("OpenAPIImporter schema conversion edges")
class OpenAPIImporterSchemaEdgeTest {

    private static String jsonContentSchema(Object holder) throws Exception {
        Method method = OpenAPIImporter.class.getDeclaredMethod("jsonContentSchema", Object.class);
        method.setAccessible(true);
        return (String) method.invoke(null, holder);
    }

    @Test
    @DisplayName("jsonContentSchema：缺 content / 缺 application/json / 缺 schema / 空 schema 返回 null")
    void jsonContentSchemaMissingPartsReturnNull() throws Exception {
        // holder 非 Map
        assertNull(jsonContentSchema("not-a-map"));
        // 无 content
        assertNull(jsonContentSchema(Map.of("summary", "s")));
        // content 无 application/json
        assertNull(jsonContentSchema(Map.of("content", Map.of("text/plain", Map.of()))));
        // application/json 无 schema
        assertNull(jsonContentSchema(Map.of("content",
            Map.of("application/json", Map.of("examples", Map.of())))));
        // schema 为空 Map
        assertNull(jsonContentSchema(Map.of("content",
            Map.of("application/json", Map.of("schema", Map.of())))));
        // schema 只含未知关键字 → 结果为空 → null
        assertNull(jsonContentSchema(Map.of("content",
            Map.of("application/json", Map.of("schema", Map.of("x-vendor", "1"))))));
    }

    @Test
    @DisplayName("jsonContentSchema：type/description/properties/required 全量转换")
    void jsonContentSchemaFullConversion() throws Exception {
        Map<String, Object> schema = Map.of(
            "type", "object",
            "description", "a request body",
            "properties", Map.of(
                "name", Map.of("type", "string", "description", "player name"),
                "meta", Map.of("description", "untyped property")),
            "required", List.of("name"));
        String json = jsonContentSchema(Map.of("content",
            Map.of("application/json", Map.of("schema", schema))));
        assertNotNull(json);
        assertTrue(json.contains("\"type\":\"object\""));
        assertTrue(json.contains("a request body"));
        assertTrue(json.contains("\"name\""));
        // 未声明 type 的属性默认 object
        assertTrue(json.contains("\"meta\""));
        assertTrue(json.contains("untyped property"));
        assertTrue(json.contains("player name"));
        assertTrue(json.contains("\"required\":[\"name\"]"));
    }

    @Test
    @DisplayName("extractExtension：字符串 / 布尔 / 复杂值 / 缺失")
    void extractExtensionValueKinds() {
        assertEquals("", OpenAPIImporter.extractExtension(Map.of(), "x-risk"));
        assertEquals("high", OpenAPIImporter.extractExtension(
            Map.of("x-risk", "high"), "x-risk"));
        assertEquals("true", OpenAPIImporter.extractExtension(
            Map.of("x-approval", Boolean.TRUE), "x-approval"));
        assertEquals("false", OpenAPIImporter.extractExtension(
            Map.of("x-approval", Boolean.FALSE), "x-approval"));
        String complex = OpenAPIImporter.extractExtension(
            Map.of("x-scope", List.of("a", "b")), "x-scope");
        assertEquals("[\"a\",\"b\"]", complex);
        String numberValue = OpenAPIImporter.extractExtension(
            Map.of("x-level", 7), "x-level");
        assertEquals("7", numberValue);
    }

    @Test
    @DisplayName("ImportOptions null 前缀归一化为空串")
    void importOptionsNullPrefixes() {
        OpenAPIImporter.ImportOptions options = new OpenAPIImporter.ImportOptions()
            .resourcePrefix(null)
            .tagPrefix(null)
            .defaultTimeoutMs(1500)
            .continueOnError(true);
        assertNotNull(options);
    }
}
